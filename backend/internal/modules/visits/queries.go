package visits

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
)

// present builds the response for one visit as seen by p.
func (s *Service) present(ctx context.Context, p authctx.Principal, v *Visit, timeline bool) (*VisitDTO, error) {
	dtos, err := s.presentAll(ctx, p, []Visit{*v})
	if err != nil {
		return nil, err
	}
	d := dtos[0]
	if timeline {
		logs, err := s.d.Audit.ForVisit(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		d.Timeline = make([]TimelineItem, 0, len(logs))
		for _, l := range logs {
			d.Timeline = append(d.Timeline, TimelineItem{l.Action, l.ActorRole, l.FromStatus, l.ToStatus, l.Reason, l.CreatedAt})
		}
	}
	return &d, nil
}

func (s *Service) presentAll(ctx context.Context, p authctx.Principal, vs []Visit) ([]VisitDTO, error) {
	ids := make([]uuid.UUID, len(vs))
	for i := range vs {
		ids[i] = vs[i].ID
	}
	photos := s.photoSummaries(ctx, ids)
	now, today := s.d.Clock.Now(), s.d.Clock.Today()
	actor := ActorOf(p)
	out := make([]VisitDTO, 0, len(vs))
	for i := range vs {
		v := &vs[i]
		ps := photos[v.ID]
		d := VisitDTO{ID: v.ID, Code: v.Code, Type: v.Type, Status: v.Status, Purpose: v.Purpose, VisitDate: v.VisitDate,
			ExpectedArrival: v.ExpectedArrival, ExpectedDurationMin: v.ExpectedDurationMin, ApprovalDeadline: v.ApprovalDeadline,
			DecidedAt: v.DecidedAt, DecisionReason: v.DecisionReason, CheckedInAt: v.CheckedInAt, MeetingOverAt: v.MeetingOverAt,
			CheckedOutAt: v.CheckedOutAt, ClosedReason: v.ClosedReason, IsOverstay: v.IsOverstay(now),
			HasVisitorPhoto: ps.HasVisitor, HasIDPhoto: ps.HasID, Version: v.Version, CreatedAt: v.CreatedAt,
			AllowedActions: AllowedActions(v, actor, Facts{Now: now, Today: today, HasVisitorPhoto: ps.HasVisitor, HasIDPhoto: ps.HasID})}
		if ps.IDType != "" {
			t := ps.IDType
			d.IDType = &t
		}
		if v.Visitor != nil {
			d.Visitor = VisitorDTO{Name: v.VisitorName, Mobile: v.Visitor.Mobile, Email: v.Visitor.Email, Company: v.Visitor.Company}
		} else {
			d.Visitor = VisitorDTO{Name: v.VisitorName}
		}
		if v.Host != nil {
			d.Host = HostDTO{ID: v.Host.ID, Name: v.Host.Name, Department: v.Host.Department}
		}
		if v.Status == StatusPendingApproval && v.ApprovalDeadline != nil {
			left := int(v.ApprovalDeadline.Sub(now).Seconds())
			if left < 0 {
				left = 0
			}
			d.ApprovalSecondsLeft = &left
		}
		out = append(out, d)
	}
	return out, nil
}

// Get returns one visit; an employee may only see their own (BR-9). A pending walk-in whose
// window has passed is settled as expired first (lazy expiry, spec §6.3).
func (s *Service) Get(ctx context.Context, p authctx.Principal, id uuid.UUID) (*VisitDTO, error) {
	v, err := s.load(ctx, s.d.DB, id)
	if err != nil {
		return nil, err
	}
	if p.Role == authctx.RoleEmployee && v.HostID != p.ID {
		return nil, apperr.ErrForbidden
	}
	if v.Status == StatusPendingApproval && v.ApprovalDeadline != nil && !s.d.Clock.Now().Before(*v.ApprovalDeadline) {
		if err := s.expire(ctx, id); err != nil {
			return nil, err
		}
		if v, err = s.load(ctx, s.d.DB, id); err != nil {
			return nil, err
		}
	}
	return s.present(ctx, p, v, true)
}

type ListFilter struct {
	Statuses   []Status
	Type       Type
	HostID     *uuid.UUID
	From, To   clock.Date
	Q          string
	OnPremises bool
	Overstay   bool
	Mine       bool
	Page       int
	PageSize   int
}

var nonDigits = regexp.MustCompile(`\D`)

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Service) scope(p authctx.Principal, f ListFilter) func(*gorm.DB) *gorm.DB {
	now := s.d.Clock.Now()
	return func(q *gorm.DB) *gorm.DB {
		if p.Role == authctx.RoleEmployee || f.Mine {
			q = q.Where("visits.host_id = ?", p.ID)
		} else if f.HostID != nil {
			q = q.Where("visits.host_id = ?", *f.HostID)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("visits.status IN ?", f.Statuses)
		}
		if f.OnPremises {
			q = q.Where("visits.status IN ?", []Status{StatusCheckedIn, StatusMeetingOver})
		}
		if f.Overstay {
			q = q.Where("visits.status = ? AND visits.checked_in_at + (visits.expected_duration_min * interval '1 minute') < ?", StatusCheckedIn, now)
		}
		if f.Type != "" {
			q = q.Where("visits.type = ?", f.Type)
		}
		if f.From != "" {
			q = q.Where("visits.visit_date >= ?", f.From)
		}
		if f.To != "" {
			q = q.Where("visits.visit_date <= ?", f.To)
		}
		if t := strings.TrimSpace(f.Q); t != "" {
			like := "%" + escapeLike(strings.ToLower(t)) + "%"
			digits := nonDigits.ReplaceAllString(t, "")
			if len(digits) >= 3 {
				q = q.Where(`lower(visits.code) LIKE ? ESCAPE '\' OR lower(visits.visitor_name) LIKE ? ESCAPE '\' OR visits.visitor_id IN (SELECT id FROM visitors WHERE mobile LIKE ?)`,
					like, like, "%"+digits+"%")
			} else {
				q = q.Where(`lower(visits.code) LIKE ? ESCAPE '\' OR lower(visits.visitor_name) LIKE ? ESCAPE '\'`, like, like)
			}
		}
		return q
	}
}

// expireDue settles every pending walk-in whose window has passed (used before reads).
func (s *Service) expireDue(ctx context.Context) {
	if _, err := s.ExpireDue(ctx); err != nil {
		s.d.Log.Error("expire due", "err", err)
	}
}

func (s *Service) List(ctx context.Context, p authctx.Principal, f ListFilter) ([]VisitDTO, int, error) {
	s.expireDue(ctx)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	var total int64
	if err := s.d.DB.WithContext(ctx).Model(&Visit{}).Scopes(s.scope(p, f)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var vs []Visit
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).Scopes(s.scope(p, f)).Preload("Visitor").Preload("Host").
		Order("visits.visit_date DESC, visits.created_at DESC, visits.id").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&vs).Error
	if err != nil {
		return nil, 0, err
	}
	out, err := s.presentAll(ctx, p, vs)
	return out, int(total), err
}

const dashboardLimit = 200

func (s *Service) find(ctx context.Context, p authctx.Principal, scope func(*gorm.DB) *gorm.DB, order string) ([]VisitDTO, error) {
	var vs []Visit
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).Scopes(scope).Preload("Visitor").Preload("Host").Order(order).Limit(dashboardLimit).Find(&vs).Error
	if err != nil {
		return nil, err
	}
	return s.presentAll(ctx, p, vs)
}

func (s *Service) SecurityDashboard(ctx context.Context, p authctx.Principal) (*SecurityDashboard, error) {
	s.expireDue(ctx)
	today, now := s.d.Clock.Today(), s.d.Clock.Now()
	var d SecurityDashboard
	var err error
	if d.ExpectedToday, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return q.Where("visits.visit_date = ? AND visits.status IN ?", today, []Status{StatusScheduled, StatusApproved})
	}, "visits.expected_arrival NULLS LAST, visits.created_at"); err != nil {
		return nil, err
	}
	if d.PendingApprovals, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return q.Where("visits.status = ?", StatusPendingApproval)
	}, "visits.approval_deadline"); err != nil {
		return nil, err
	}
	if d.OnPremises, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return q.Where("visits.status IN ?", []Status{StatusCheckedIn, StatusMeetingOver})
	}, "visits.checked_in_at"); err != nil {
		return nil, err
	}
	if d.ReadyForCheckout, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return q.Where("visits.status = ?", StatusMeetingOver)
	}, "visits.meeting_over_at"); err != nil {
		return nil, err
	}
	if d.Overstays, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return q.Where("visits.status = ? AND visits.checked_in_at + (visits.expected_duration_min * interval '1 minute') < ?", StatusCheckedIn, now)
	}, "visits.checked_in_at"); err != nil {
		return nil, err
	}
	d.Counts = map[string]int{"expectedToday": len(d.ExpectedToday), "pendingApprovals": len(d.PendingApprovals),
		"onPremises": len(d.OnPremises), "readyForCheckout": len(d.ReadyForCheckout), "overstays": len(d.Overstays)}
	return &d, nil
}

func (s *Service) EmployeeDashboard(ctx context.Context, p authctx.Principal) (*EmployeeDashboard, error) {
	s.expireDue(ctx)
	today := s.d.Clock.Today()
	mine := func(q *gorm.DB) *gorm.DB { return q.Where("visits.host_id = ?", p.ID) }
	var d EmployeeDashboard
	var err error
	if d.PendingApprovals, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return mine(q).Where("visits.status = ?", StatusPendingApproval)
	}, "visits.approval_deadline"); err != nil {
		return nil, err
	}
	if d.TodaysVisits, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return mine(q).Where("visits.visit_date = ? AND visits.status <> ?", today, StatusPendingApproval)
	}, "visits.expected_arrival NULLS LAST, visits.created_at"); err != nil {
		return nil, err
	}
	if d.Upcoming, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return mine(q).Where("visits.visit_date > ? AND visits.status = ?", today, StatusScheduled)
	}, "visits.visit_date, visits.expected_arrival NULLS LAST"); err != nil {
		return nil, err
	}
	if d.AwaitingMeetingOver, err = s.find(ctx, p, func(q *gorm.DB) *gorm.DB {
		return mine(q).Where("visits.status = ?", StatusCheckedIn)
	}, "visits.checked_in_at"); err != nil {
		return nil, err
	}
	return &d, nil
}

// ExportRow is one line of the CSV report (no photos, no ID details).
type ExportRow struct {
	Code, Type, Status, Visitor, Mobile, Company, Host, Department, Purpose string
	VisitDate                                                               clock.Date
	DurationMin                                                             int
	CreatedAt                                                               time.Time
	CheckedInAt, MeetingOverAt, CheckedOutAt                                *time.Time
	ClosedReason                                                            string
}

const exportMaxRows = 50000

// Export streams the filtered visits (admin report), at most 50 000 rows.
func (s *Service) Export(ctx context.Context, p authctx.Principal, f ListFilter, fn func(ExportRow) error) error {
	sent := 0
	for offset := 0; sent < exportMaxRows; offset += 1000 {
		var vs []Visit
		err := s.d.DB.WithContext(ctx).Model(&Visit{}).Scopes(s.scope(p, f)).Preload("Visitor").Preload("Host").
			Order("visits.visit_date, visits.created_at, visits.id").Limit(1000).Offset(offset).Find(&vs).Error
		if err != nil {
			return err
		}
		for _, v := range vs {
			r := ExportRow{Code: v.Code, Type: string(v.Type), Status: string(v.Status), Visitor: v.VisitorName, Purpose: v.Purpose,
				VisitDate: v.VisitDate, DurationMin: v.ExpectedDurationMin, CreatedAt: v.CreatedAt, CheckedInAt: v.CheckedInAt,
				MeetingOverAt: v.MeetingOverAt, CheckedOutAt: v.CheckedOutAt, ClosedReason: deref(v.ClosedReason)}
			if v.Visitor != nil {
				r.Mobile, r.Company = v.Visitor.Mobile, deref(v.Visitor.Company)
			}
			if v.Host != nil {
				r.Host, r.Department = v.Host.Name, deref(v.Host.Department)
			}
			if err := fn(r); err != nil {
				return err
			}
			if sent++; sent >= exportMaxRows {
				return nil
			}
		}
		if len(vs) < 1000 {
			return nil
		}
	}
	return nil
}
