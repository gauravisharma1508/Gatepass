// Package notifications turns visit events into in-app notifications, emails and live
// updates (SSE). It implements visits.EventSink.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/modules/users"
	"gatepass/internal/modules/visits"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/mailer"
)

type Notification struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	UserID    uuid.UUID
	VisitID   *uuid.UUID
	Type      string
	Title     string
	Body      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

func (Notification) TableName() string { return "notifications" }

type Deps struct {
	DB           *gorm.DB
	Users        *users.Service
	Mailer       mailer.Mailer
	Clock        clock.Clock
	Hub          *Hub
	Log          *slog.Logger
	WebOrigin    string
	SyncEmail    bool // send email in the caller's goroutine (tests)
	EmailBackoff time.Duration
}

type Service struct{ d Deps }

func New(d Deps) *Service {
	if d.EmailBackoff == 0 {
		d.EmailBackoff = 2 * time.Second
	}
	return &Service{d: d}
}

func (s *Service) Hub() *Hub { return s.d.Hub }

type visitEventPayload struct {
	ID      uuid.UUID     `json:"id"`
	Status  visits.Status `json:"status"`
	Version int           `json:"version"`
}

// HandleVisitEvent runs after the visit transaction committed. Failures are logged and never
// bubble up: telling people must not fail the business action.
func (s *Service) HandleVisitEvent(ctx context.Context, e visits.Event) {
	ctx = context.WithoutCancel(ctx)
	// 1. live refresh for the host, security and admin screens
	payload, _ := json.Marshal(visitEventPayload{e.VisitID, e.Status, e.Version})
	s.d.Hub.ToUsersAndRoles([]uuid.UUID{e.HostID}, []authctx.Role{authctx.RoleSecurity, authctx.RoleAdmin},
		Message{Event: "visit-updated", Data: payload})

	// 2. notifications per spec §9.1
	var host, security bool
	var title, body string
	mailHost := false
	who := e.VisitorName
	if e.Company != "" {
		who += " from " + e.Company
	}
	switch e.Type {
	case visits.EventApprovalRequested:
		host, mailHost = true, true
		title = "Visitor waiting for you"
		body = fmt.Sprintf("%s is at reception. Please respond within %d minutes.", who, e.WindowMin)
	case visits.EventApprovalReminder:
		host = true
		title = "Please respond to your visitor"
		body = fmt.Sprintf("%s is still waiting. The request expires soon.", who)
	case visits.EventApprovalDecided:
		security = true
		switch e.Status {
		case visits.StatusApproved:
			title, body = "Walk-in approved", fmt.Sprintf("%s was approved by %s.", e.VisitorName, e.HostName)
		case visits.StatusRejected:
			title, body = "Walk-in rejected", fmt.Sprintf("%s was rejected by %s.", e.VisitorName, e.HostName)
			if e.Reason != "" {
				body += " Reason: " + e.Reason
			}
		case visits.StatusExpired:
			host = true
			title, body = "Walk-in request expired", fmt.Sprintf("%s was not answered in time by %s.", e.VisitorName, e.HostName)
		case visits.StatusCancelled:
			title, body = "Walk-in request cancelled", fmt.Sprintf("The request for %s was cancelled.", e.VisitorName)
		}
	case visits.EventVisitorCheckedIn:
		host, mailHost = true, true
		title, body = "Your visitor has arrived", fmt.Sprintf("%s has checked in at the gate.", who)
	case visits.EventVisitorDenied:
		host = true
		title, body = "Visitor denied entry", fmt.Sprintf("Security denied entry to %s. Reason: %s", e.VisitorName, e.Reason)
	case visits.EventOverstay:
		host, security = true, true
		title, body = "Visitor overstay", fmt.Sprintf("%s is still checked in past the expected duration.", e.VisitorName)
	case visits.EventMeetingOver:
		security = true
		title, body = "Visitor ready for check-out", fmt.Sprintf("%s's meeting is over (host: %s).", e.VisitorName, e.HostName)
	case visits.EventVisitorCheckedOut:
		host = true
		title, body = "Visitor checked out", fmt.Sprintf("%s has left the premises.", e.VisitorName)
	default:
		return
	}

	recipients := map[uuid.UUID]bool{}
	if host {
		recipients[e.HostID] = true
	}
	if security {
		ids, err := s.d.Users.IDsByRole(ctx, authctx.RoleSecurity)
		if err != nil {
			s.d.Log.Error("notify: list security users", "err", err)
		}
		for _, id := range ids {
			recipients[id] = true
		}
	}
	for uid := range recipients {
		s.create(ctx, uid, e, title, body)
	}
	if mailHost && e.HostEmail != "" {
		link := fmt.Sprintf("%s/visits/%s", strings.TrimRight(s.d.WebOrigin, "/"), e.VisitID)
		msg := mailer.Message{To: e.HostEmail, Subject: title + " – " + e.Code, Body: fmt.Sprintf("Hello %s,\n\n%s\n\nOpen the visit: %s\n", e.HostName, body, link)}
		if s.d.SyncEmail {
			s.sendWithRetry(ctx, msg)
		} else {
			go s.sendWithRetry(ctx, msg)
		}
	}
}

func (s *Service) create(ctx context.Context, userID uuid.UUID, e visits.Event, title, body string) {
	vid := e.VisitID
	n := Notification{ID: uuid.New(), UserID: userID, VisitID: &vid, Type: string(e.Type), Title: title, Body: body, CreatedAt: s.d.Clock.Now()}
	if err := s.d.DB.WithContext(ctx).Create(&n).Error; err != nil {
		s.d.Log.Error("notify: create", "err", err)
		return
	}
	unread, _ := s.UnreadCount(ctx, userID)
	data, _ := json.Marshal(map[string]any{"notification": toDTO(n), "unreadCount": unread})
	s.d.Hub.ToUser(userID, Message{Event: "notification", Data: data})
}

// sendWithRetry tries up to 3 times with back-off; the last failure is only logged.
func (s *Service) sendWithRetry(ctx context.Context, m mailer.Message) {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = s.d.Mailer.Send(ctx, m); err == nil {
			return
		}
		s.d.Log.Warn("email failed", "attempt", attempt, "err", err)
		if attempt < 3 {
			time.Sleep(s.d.EmailBackoff * time.Duration(attempt))
		}
	}
	s.d.Log.Error("email gave up", "to", "(host)", "err", err)
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int64
	err := s.d.DB.WithContext(ctx).Model(&Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return int(n), err
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, page, size int, unreadOnly bool) ([]Notification, int, error) {
	q := s.d.DB.WithContext(ctx).Model(&Notification{}).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []Notification
	err := q.Order("created_at DESC, id").Limit(size).Offset((page - 1) * size).Find(&out).Error
	return out, int(total), err
}

func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	res := s.d.DB.WithContext(ctx).Model(&Notification{}).Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", gorm.Expr("COALESCE(read_at, ?)", s.d.Clock.Now()))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return db.MapError(gorm.ErrRecordNotFound)
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.d.DB.WithContext(ctx).Model(&Notification{}).Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", s.d.Clock.Now()).Error
}
