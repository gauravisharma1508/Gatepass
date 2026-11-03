// Package settings owns the single admin-editable settings row.
package settings

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"gatepass/internal/modules/audit"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/httpx"
	"gatepass/internal/platform/validation"
)

type Settings struct {
	ID                  int16 `gorm:"primaryKey"`
	ApprovalWindowMin   int
	ApprovalReminderMin int
	MaxAdvanceDays      int
	PhotoRetentionDays  int
	RecordRetentionDays int
	UpdatedAt           time.Time
}

func (Settings) TableName() string { return "settings" }

type DTO struct {
	ApprovalWindowMin   int `json:"approvalWindowMin"`
	ApprovalReminderMin int `json:"approvalReminderMin"`
	MaxAdvanceDays      int `json:"maxAdvanceDays"`
	PhotoRetentionDays  int `json:"photoRetentionDays"`
	RecordRetentionDays int `json:"recordRetentionDays"`
}

type Patch struct {
	ApprovalWindowMin   *int `json:"approvalWindowMin"`
	ApprovalReminderMin *int `json:"approvalReminderMin"`
	MaxAdvanceDays      *int `json:"maxAdvanceDays"`
	PhotoRetentionDays  *int `json:"photoRetentionDays"`
	RecordRetentionDays *int `json:"recordRetentionDays"`
}

type Service struct {
	db    *gorm.DB
	audit *audit.Service
	clock clock.Clock
}

func New(g *gorm.DB, a *audit.Service, clk clock.Clock) *Service {
	return &Service{db: g, audit: a, clock: clk}
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	var v Settings
	err := s.db.WithContext(ctx).First(&v, 1).Error
	return v, db.MapError(err)
}

func (s *Service) Update(ctx context.Context, actor authctx.Principal, p Patch) (Settings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return cur, err
	}
	next := cur
	set := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	set(&next.ApprovalWindowMin, p.ApprovalWindowMin)
	set(&next.ApprovalReminderMin, p.ApprovalReminderMin)
	set(&next.MaxAdvanceDays, p.MaxAdvanceDays)
	set(&next.PhotoRetentionDays, p.PhotoRetentionDays)
	set(&next.RecordRetentionDays, p.RecordRetentionDays)

	f := validation.New()
	rng := func(field string, v, lo, hi int) {
		if v < lo || v > hi {
			f.Set(field, "Must be between "+itoa(lo)+" and "+itoa(hi))
		}
	}
	rng("approvalWindowMin", next.ApprovalWindowMin, 10, 120)
	rng("approvalReminderMin", next.ApprovalReminderMin, 1, max(1, next.ApprovalWindowMin-1))
	rng("maxAdvanceDays", next.MaxAdvanceDays, 1, 90)
	rng("photoRetentionDays", next.PhotoRetentionDays, 7, 365)
	if next.RecordRetentionDays < 90 {
		f.Set("recordRetentionDays", "Must be at least 90")
	}
	if err := f.Err(); err != nil {
		return cur, err
	}
	next.UpdatedAt = s.clock.Now()
	err = db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		if err := tx.Save(&next).Error; err != nil {
			return err
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{ActorID: &actor.ID, ActorRole: string(actor.Role), Action: "SETTINGS_UPDATED",
			Metadata: map[string]any{"from": toDTO(cur), "to": toDTO(next)}})
	})
	return next, err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func toDTO(s Settings) DTO {
	return DTO{s.ApprovalWindowMin, s.ApprovalReminderMin, s.MaxAdvanceDays, s.PhotoRetentionDays, s.RecordRetentionDays}
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	admin := r.With(httpx.RequireRoles(authctx.RoleAdmin))
	admin.Get("/settings", httpx.H(h.get))
	admin.Patch("/settings", httpx.H(h.patch))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	s, err := h.svc.Get(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, toDTO(s))
	return nil
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	var in Patch
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	s, err := h.svc.Update(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, toDTO(s))
	return nil
}
