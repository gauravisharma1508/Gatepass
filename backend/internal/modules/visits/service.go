package visits

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/modules/audit"
	"gatepass/internal/modules/settings"
	"gatepass/internal/modules/users"
	"gatepass/internal/modules/visitors"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/validation"
)

type Deps struct {
	DB       *gorm.DB
	Clock    clock.Clock
	Users    *users.Service
	Visitors *visitors.Service
	Audit    *audit.Service
	Settings *settings.Service
	Photos   PhotoChecker // optional until photos is wired (reverse dependency)
	Events   EventSink    // optional
	Log      *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// SetPhotos injects the photos module (it needs this service, so it is wired second).
func (s *Service) SetPhotos(p PhotoChecker) { s.d.Photos = p }

func (s *Service) facts(ps PhotoSummary) Facts {
	return Facts{Now: s.d.Clock.Now(), Today: s.d.Clock.Today(), HasVisitorPhoto: ps.HasVisitor, HasIDPhoto: ps.HasID}
}

func (s *Service) photoSummaries(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]PhotoSummary {
	if s.d.Photos == nil || len(ids) == 0 {
		return map[uuid.UUID]PhotoSummary{}
	}
	m, err := s.d.Photos.Summaries(ctx, ids)
	if err != nil {
		s.d.Log.Error("photo summaries", "err", err)
		return map[uuid.UUID]PhotoSummary{}
	}
	return m
}

func (s *Service) load(ctx context.Context, q *gorm.DB, id uuid.UUID) (*Visit, error) {
	var v Visit
	if err := q.WithContext(ctx).Preload("Visitor").Preload("Host").First(&v, "id = ?", id).Error; err != nil {
		return nil, db.MapError(err)
	}
	return &v, nil
}

// ---- creating and editing ----

type PlannedInput struct {
	VisitorName         string `json:"visitorName"`
	VisitorMobile       string `json:"visitorMobile"`
	VisitorEmail        string `json:"visitorEmail"`
	VisitorCompany      string `json:"visitorCompany"`
	Purpose             string `json:"purpose"`
	VisitDate           string `json:"visitDate"`
	ExpectedArrival     string `json:"expectedArrival"`
	ExpectedDurationMin int    `json:"expectedDurationMin"`
}

type WalkInInput struct {
	VisitorName         string `json:"visitorName"`
	VisitorMobile       string `json:"visitorMobile"`
	VisitorEmail        string `json:"visitorEmail"`
	VisitorCompany      string `json:"visitorCompany"`
	Purpose             string `json:"purpose"`
	ExpectedDurationMin int    `json:"expectedDurationMin"`
	HostID              string `json:"hostId"`
}

type cleaned struct {
	name, mobile, email, company, purpose, arrival string
	duration                                       int
}

func validateCommon(f *validation.Fields, name, mobile, email, company, purpose, arrival string, duration int) cleaned {
	var c cleaned
	var msg string
	c.name, msg = validation.Name(name)
	f.Set("visitorName", msg)
	c.mobile, msg = validation.Mobile(mobile)
	f.Set("visitorMobile", msg)
	c.email, msg = validation.OptionalEmail(email)
	f.Set("visitorEmail", msg)
	c.company, msg = validation.Text(company, 0, 100)
	f.Set("visitorCompany", msg)
	c.purpose, msg = validation.Text(purpose, 3, 250)
	f.Set("purpose", msg)
	c.arrival = arrival
	f.Set("expectedArrival", validation.Arrival(arrival))
	c.duration = duration
	f.Set("expectedDurationMin", validation.Duration(duration))
	return c
}

func (s *Service) nextCode(tx *gorm.DB, date clock.Date) (string, error) {
	var n int
	err := tx.Raw(`INSERT INTO visit_code_counters (date, last_value) VALUES (?, 1)
		ON CONFLICT (date) DO UPDATE SET last_value = visit_code_counters.last_value + 1 RETURNING last_value`, date).Scan(&n).Error
	if err != nil {
		return "", err
	}
	if n > 9999 {
		return "", apperr.New(409, "DAILY_LIMIT", "Too many visits for this day")
	}
	return fmt.Sprintf("GP-%s-%04d", strings.ReplaceAll(string(date), "-", ""), n), nil
}

func auditActor(a Actor) (*uuid.UUID, string) {
	if a.System {
		return nil, "SYSTEM"
	}
	id := a.ID
	return &id, string(a.Role)
}

func (s *Service) event(v *Visit, t EventType) Event {
	e := Event{Type: t, VisitID: v.ID, Code: v.Code, Status: v.Status, Version: v.Version, HostID: v.HostID,
		VisitorName: v.VisitorName, At: s.d.Clock.Now()}
	if v.Host != nil {
		e.HostName, e.HostEmail = v.Host.Name, v.Host.Email
	}
	if v.Visitor != nil && v.Visitor.Company != nil {
		e.Company = *v.Visitor.Company
	}
	if v.ApprovalDeadline != nil {
		e.WindowMin = int(v.ApprovalDeadline.Sub(v.CreatedAt).Round(time.Minute) / time.Minute)
	}
	return e
}

func (s *Service) publish(ctx context.Context, e Event) {
	if s.d.Events != nil {
		s.d.Events.HandleVisitEvent(ctx, e)
	}
}

func (s *Service) CreatePlanned(ctx context.Context, p authctx.Principal, in PlannedInput) (*VisitDTO, error) {
	if !p.CanHost() {
		return nil, apperr.ErrForbidden
	}
	set, err := s.d.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	f := validation.New()
	c := validateCommon(f, in.VisitorName, in.VisitorMobile, in.VisitorEmail, in.VisitorCompany, in.Purpose, in.ExpectedArrival, in.ExpectedDurationMin)
	date, msg := validation.VisitDate(in.VisitDate, s.d.Clock.Today(), set.MaxAdvanceDays)
	f.Set("visitDate", msg)
	if err := f.Err(); err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.d.DB, func(tx *db.Tx) error {
		vis, err := s.d.Visitors.Upsert(ctx, tx.DB, visitors.UpsertInput{Mobile: c.mobile, Name: c.name, Email: c.email, Company: c.company})
		if err != nil {
			return err
		}
		code, err := s.nextCode(tx.DB, date)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		v := Visit{ID: uuid.New(), Code: code, Type: TypePlanned, Status: StatusScheduled, VisitorID: vis.ID, VisitorName: c.name,
			HostID: p.ID, CreatedByID: p.ID, Purpose: c.purpose, VisitDate: date, ExpectedArrival: arrivalPtr(c.arrival),
			ExpectedDurationMin: c.duration, Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Omit("Visitor", "Host").Create(&v).Error; err != nil {
			return db.MapError(err)
		}
		id = v.ID
		if err := s.d.Audit.Record(ctx, tx.DB, audit.Entry{VisitID: &v.ID, ActorID: &p.ID, ActorRole: string(p.Role),
			Action: "VISIT_CREATED", To: string(StatusScheduled)}); err != nil {
			return err
		}
		tx.AfterCommit(func() {
			if nv, err := s.load(context.WithoutCancel(ctx), s.d.DB, v.ID); err == nil {
				s.publish(ctx, s.event(nv, EventNone))
			}
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, p, id)
}

func arrivalPtr(s string) *clock.TimeOfDay {
	if s == "" {
		return nil
	}
	t := clock.TimeOfDay(s)
	return &t
}

type PlannedPatch struct {
	VisitorName         *string `json:"visitorName"`
	VisitorMobile       *string `json:"visitorMobile"`
	VisitorEmail        *string `json:"visitorEmail"`
	VisitorCompany      *string `json:"visitorCompany"`
	Purpose             *string `json:"purpose"`
	VisitDate           *string `json:"visitDate"`
	ExpectedArrival     *string `json:"expectedArrival"`
	ExpectedDurationMin *int    `json:"expectedDurationMin"`
}

// UpdatePlanned edits a SCHEDULED visit; only its host may (FR-2.5).
func (s *Service) UpdatePlanned(ctx context.Context, p authctx.Principal, id uuid.UUID, in PlannedPatch) (*VisitDTO, error) {
	set, err := s.d.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	err = db.WithTx(ctx, s.d.DB, func(tx *db.Tx) error {
		v, err := s.load(ctx, tx.DB, id)
		if err != nil {
			return err
		}
		if !p.CanHost() || v.HostID != p.ID {
			return apperr.ErrForbidden
		}
		if v.Status != StatusScheduled {
			return apperr.ErrInvalidState
		}
		f := validation.New()
		upd := map[string]any{}
		changed := map[string]any{}

		name, mobile := v.VisitorName, v.Visitor.Mobile
		email, company := deref(v.Visitor.Email), deref(v.Visitor.Company)
		visitorChanged := false
		if in.VisitorName != nil {
			var msg string
			name, msg = validation.Name(*in.VisitorName)
			f.Set("visitorName", msg)
			visitorChanged = true
			changed["visitorName"] = name
		}
		if in.VisitorMobile != nil {
			var msg string
			mobile, msg = validation.Mobile(*in.VisitorMobile)
			f.Set("visitorMobile", msg)
			visitorChanged = true
			changed["visitorMobile"] = mobile
		}
		if in.VisitorEmail != nil {
			var msg string
			email, msg = validation.OptionalEmail(*in.VisitorEmail)
			f.Set("visitorEmail", msg)
			visitorChanged = true
		}
		if in.VisitorCompany != nil {
			var msg string
			company, msg = validation.Text(*in.VisitorCompany, 0, 100)
			f.Set("visitorCompany", msg)
			visitorChanged = true
		}
		if in.Purpose != nil {
			pv, msg := validation.Text(*in.Purpose, 3, 250)
			f.Set("purpose", msg)
			upd["purpose"], changed["purpose"] = pv, pv
		}
		if in.VisitDate != nil {
			d, msg := validation.VisitDate(*in.VisitDate, s.d.Clock.Today(), set.MaxAdvanceDays)
			f.Set("visitDate", msg)
			upd["visit_date"], changed["visitDate"] = d, d
		}
		if in.ExpectedArrival != nil {
			f.Set("expectedArrival", validation.Arrival(*in.ExpectedArrival))
			upd["expected_arrival"], changed["expectedArrival"] = arrivalPtr(*in.ExpectedArrival), *in.ExpectedArrival
		}
		if in.ExpectedDurationMin != nil {
			f.Set("expectedDurationMin", validation.Duration(*in.ExpectedDurationMin))
			upd["expected_duration_min"], changed["expectedDurationMin"] = *in.ExpectedDurationMin, *in.ExpectedDurationMin
		}
		if err := f.Err(); err != nil {
			return err
		}
		if visitorChanged {
			vis, err := s.d.Visitors.Upsert(ctx, tx.DB, visitors.UpsertInput{Mobile: mobile, Name: name, Email: email, Company: company})
			if err != nil {
				return err
			}
			upd["visitor_id"], upd["visitor_name"] = vis.ID, name
		}
		if len(upd) == 0 {
			return nil
		}
		upd["version"], upd["updated_at"] = gorm.Expr("version + 1"), s.d.Clock.Now()
		res := tx.Model(&Visit{}).Where("id = ? AND status = ?", id, StatusScheduled).Updates(upd)
		if res.Error != nil {
			return db.MapError(res.Error)
		}
		if res.RowsAffected == 0 {
			return apperr.ErrInvalidState
		}
		if err := s.d.Audit.Record(ctx, tx.DB, audit.Entry{VisitID: &id, ActorID: &p.ID, ActorRole: string(p.Role),
			Action: "VISIT_UPDATED", From: string(StatusScheduled), To: string(StatusScheduled), Metadata: map[string]any{"changed": changed}}); err != nil {
			return err
		}
		tx.AfterCommit(func() {
			if nv, err := s.load(context.WithoutCancel(ctx), s.d.DB, id); err == nil {
				s.publish(ctx, s.event(nv, EventNone))
			}
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, p, id)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// busy statuses: a visitor in one of these cannot be registered or checked in again.
var (
	busyForWalkIn  = []Status{StatusPendingApproval, StatusCheckedIn, StatusMeetingOver}
	busyForCheckIn = []Status{StatusCheckedIn, StatusMeetingOver}
)

func (s *Service) visitorBusy(ctx context.Context, tx *gorm.DB, visitorID, except uuid.UUID, statuses []Status) error {
	var n int64
	err := tx.WithContext(ctx).Model(&Visit{}).Where("visitor_id = ? AND id <> ? AND status IN ?", visitorID, except, statuses).Count(&n).Error
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.ErrVisitorAlreadyActive
	}
	return nil
}

// CreateWalkIn registers an unplanned visitor (security only) and asks the host for approval.
func (s *Service) CreateWalkIn(ctx context.Context, p authctx.Principal, in WalkInInput) (*VisitDTO, error) {
	if p.Role != authctx.RoleSecurity {
		return nil, apperr.ErrForbidden
	}
	set, err := s.d.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	f := validation.New()
	c := validateCommon(f, in.VisitorName, in.VisitorMobile, in.VisitorEmail, in.VisitorCompany, in.Purpose, "", in.ExpectedDurationMin)
	hostID, perr := uuid.Parse(in.HostID)
	if perr != nil {
		f.Set("hostId", "Choose who to meet")
	}
	if err := f.Err(); err != nil {
		return nil, err
	}
	if _, err := s.d.Users.ValidateHost(ctx, hostID); err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.d.DB, func(tx *db.Tx) error {
		vis, err := s.d.Visitors.Upsert(ctx, tx.DB, visitors.UpsertInput{Mobile: c.mobile, Name: c.name, Email: c.email, Company: c.company})
		if err != nil {
			return err
		}
		if err := s.visitorBusy(ctx, tx.DB, vis.ID, uuid.Nil, busyForWalkIn); err != nil {
			return err
		}
		now := s.d.Clock.Now()
		today := s.d.Clock.Today()
		code, err := s.nextCode(tx.DB, today)
		if err != nil {
			return err
		}
		deadline := now.Add(time.Duration(set.ApprovalWindowMin) * time.Minute)
		v := Visit{ID: uuid.New(), Code: code, Type: TypeUnplanned, Status: StatusPendingApproval, VisitorID: vis.ID, VisitorName: c.name,
			HostID: hostID, CreatedByID: p.ID, Purpose: c.purpose, VisitDate: today, ExpectedDurationMin: c.duration,
			ApprovalDeadline: &deadline, Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Omit("Visitor", "Host").Create(&v).Error; err != nil {
			return db.MapError(err)
		}
		id = v.ID
		if err := s.d.Audit.Record(ctx, tx.DB, audit.Entry{VisitID: &v.ID, ActorID: &p.ID, ActorRole: string(p.Role),
			Action: "VISIT_CREATED", To: string(StatusPendingApproval)}); err != nil {
			return err
		}
		tx.AfterCommit(func() {
			if nv, err := s.load(context.WithoutCancel(ctx), s.d.DB, v.ID); err == nil {
				s.publish(ctx, s.event(nv, EventApprovalRequested))
			}
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, p, id)
}

// ---- status changes ----

type tparams struct {
	action Action
	actor  Actor
	reason string
	audit  string                             // audit action; defaults to the new status
	set    func(now time.Time) map[string]any // extra columns to update
	pre    func(ctx context.Context, tx *gorm.DB, v *Visit) error
	event  func(from, to Status) EventType
	extra  func(e *Event)
}

// transition is the only place where a visit's status changes (spec §6.3): check the rules,
// update conditionally on the old status, write the audit row, publish after commit.
func (s *Service) transition(ctx context.Context, id uuid.UUID, tp tparams) (*Visit, error) {
	var out *Visit
	err := db.WithTx(ctx, s.d.DB, func(tx *db.Tx) error {
		v, err := s.load(ctx, tx.DB, id)
		if err != nil {
			return err
		}
		var ps PhotoSummary
		if tp.action == ActionCheckIn {
			ps = s.photoSummaries(ctx, []uuid.UUID{id})[id]
		}
		to, err := Check(v, tp.action, tp.actor, s.facts(ps))
		if err != nil {
			return err
		}
		if tp.pre != nil {
			if err := tp.pre(ctx, tx.DB, v); err != nil {
				return err
			}
		}
		now := s.d.Clock.Now()
		upd := map[string]any{"status": to, "version": gorm.Expr("version + 1"), "updated_at": now}
		if tp.set != nil {
			for k, val := range tp.set(now) {
				upd[k] = val
			}
		}
		res := tx.Model(&Visit{}).Where("id = ? AND status = ?", id, v.Status).Updates(upd)
		if res.Error != nil {
			return db.MapError(res.Error)
		}
		if res.RowsAffected == 0 {
			return apperr.ErrInvalidState // someone else changed it first
		}
		actorID, role := auditActor(tp.actor)
		action := tp.audit
		if action == "" {
			action = string(to)
		}
		if err := s.d.Audit.Record(ctx, tx.DB, audit.Entry{VisitID: &id, ActorID: actorID, ActorRole: role, Action: action,
			From: string(v.Status), To: string(to), Reason: tp.reason}); err != nil {
			return err
		}
		nv, err := s.load(ctx, tx.DB, id)
		if err != nil {
			return err
		}
		out = nv
		from := v.Status
		tx.AfterCommit(func() {
			t := EventNone
			if tp.event != nil {
				t = tp.event(from, to)
			}
			e := s.event(nv, t)
			e.Reason = tp.reason
			if tp.extra != nil {
				tp.extra(&e)
			}
			s.publish(ctx, e)
		})
		return nil
	})
	if errors.Is(err, apperr.ErrApprovalWindowClosed) {
		_ = s.expire(ctx, id) // a late answer also settles the visit as expired
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func always(t EventType) func(from, to Status) EventType {
	return func(_, _ Status) EventType { return t }
}

func (s *Service) expire(ctx context.Context, id uuid.UUID) error {
	_, err := s.transition(ctx, id, tparams{action: ActionExpire, actor: SystemActor(), event: always(EventApprovalDecided)})
	if errors.Is(err, apperr.ErrInvalidState) {
		return nil // already settled by someone else
	}
	return err
}

func (s *Service) Approve(ctx context.Context, p authctx.Principal, id uuid.UUID) (*VisitDTO, error) {
	v, err := s.transition(ctx, id, tparams{action: ActionApprove, actor: ActorOf(p), event: always(EventApprovalDecided),
		set: func(now time.Time) map[string]any { return map[string]any{"decided_at": now} }})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) Reject(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*VisitDTO, error) {
	reason, msg := validation.Reason(reason, false)
	if msg != "" {
		return nil, apperr.Validation(map[string]string{"reason": msg})
	}
	v, err := s.transition(ctx, id, tparams{action: ActionReject, actor: ActorOf(p), reason: reason, event: always(EventApprovalDecided),
		set: func(now time.Time) map[string]any {
			return map[string]any{"decided_at": now, "decision_reason": strPtr(reason)}
		}})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

// Cancel: the host cancels a SCHEDULED visit; security cancels a PENDING_APPROVAL walk-in.
func (s *Service) Cancel(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*VisitDTO, error) {
	reason, msg := validation.Reason(reason, false)
	if msg != "" {
		return nil, apperr.Validation(map[string]string{"reason": msg})
	}
	v, err := s.transition(ctx, id, tparams{action: ActionCancel, actor: ActorOf(p), reason: reason,
		set: func(time.Time) map[string]any { return map[string]any{"closed_reason": strPtr(reason)} },
		event: func(from, _ Status) EventType {
			if from == StatusPendingApproval {
				return EventApprovalDecided
			}
			return EventNone
		}})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) CheckIn(ctx context.Context, p authctx.Principal, id uuid.UUID) (*VisitDTO, error) {
	v, err := s.transition(ctx, id, tparams{action: ActionCheckIn, actor: ActorOf(p), event: always(EventVisitorCheckedIn),
		pre: func(ctx context.Context, tx *gorm.DB, v *Visit) error {
			return s.visitorBusy(ctx, tx, v.VisitorID, v.ID, busyForCheckIn)
		},
		set: func(now time.Time) map[string]any {
			return map[string]any{"checked_in_at": now, "checked_in_by_id": p.ID}
		}})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) Deny(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*VisitDTO, error) {
	reason, msg := validation.Reason(reason, true)
	if msg != "" {
		return nil, apperr.Validation(map[string]string{"reason": msg})
	}
	v, err := s.transition(ctx, id, tparams{action: ActionDeny, actor: ActorOf(p), reason: reason, event: always(EventVisitorDenied),
		set: func(time.Time) map[string]any { return map[string]any{"closed_reason": reason} }})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) MeetingOver(ctx context.Context, p authctx.Principal, id uuid.UUID) (*VisitDTO, error) {
	v, err := s.transition(ctx, id, tparams{action: ActionMeetingOver, actor: ActorOf(p), event: always(EventMeetingOver),
		set: func(now time.Time) map[string]any { return map[string]any{"meeting_over_at": now} }})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) CheckOut(ctx context.Context, p authctx.Principal, id uuid.UUID) (*VisitDTO, error) {
	v, err := s.transition(ctx, id, tparams{action: ActionCheckOut, actor: ActorOf(p), event: always(EventVisitorCheckedOut),
		set: func(now time.Time) map[string]any {
			return map[string]any{"checked_out_at": now, "checked_out_by_id": p.ID}
		}})
	if err != nil {
		if errors.Is(err, apperr.ErrInvalidState) {
			return nil, apperr.ErrInvalidState.With("Waiting for host to mark Meeting Over")
		}
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

func (s *Service) ForceClose(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*VisitDTO, error) {
	reason, msg := validation.Reason(reason, true)
	if msg != "" {
		return nil, apperr.Validation(map[string]string{"reason": msg})
	}
	v, err := s.transition(ctx, id, tparams{action: ActionForceClose, actor: ActorOf(p), reason: reason, audit: "FORCE_CLOSED",
		event: always(EventVisitorCheckedOut),
		set: func(now time.Time) map[string]any {
			return map[string]any{"closed_reason": reason, "checked_out_at": now, "checked_out_by_id": p.ID}
		}})
	if err != nil {
		return nil, err
	}
	return s.present(ctx, p, v, false)
}

// ---- support for the photos module ----

type Brief struct {
	ID     uuid.UUID
	HostID uuid.UUID
	Status Status
}

func (s *Service) Brief(ctx context.Context, id uuid.UUID) (Brief, error) {
	v, err := s.load(ctx, s.d.DB, id)
	if err != nil {
		return Brief{}, err
	}
	return Brief{ID: v.ID, HostID: v.HostID, Status: v.Status}, nil
}

// CanUploadPhotos applies the state machine to photo capture (security, SCHEDULED today or APPROVED).
func (s *Service) CanUploadPhotos(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	v, err := s.load(ctx, s.d.DB, id)
	if err != nil {
		return err
	}
	_, err = Check(v, ActionUploadPhotos, ActorOf(p), s.facts(PhotoSummary{}))
	return err
}

// FinalBefore lists visits that reached a final status before cutoff and still may have photos.
func (s *Service) FinalBefore(ctx context.Context, cutoff time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).
		Where("status IN ? AND updated_at < ? AND photos_purged_at IS NULL", finalStatuses, cutoff).Pluck("id", &ids).Error
	return ids, err
}

func (s *Service) MarkPhotosPurged(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	// UpdateColumn: bookkeeping must not touch updated_at, which retention reads as "when the visit ended"
	return s.d.DB.WithContext(ctx).Model(&Visit{}).Where("id IN ?", ids).UpdateColumn("photos_purged_at", s.d.Clock.Now()).Error
}
