// Package audit owns the append-only audit log (FR-9). Other modules call Record
// inside their own transactions so the entry commits (or rolls back) with the change.
package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/httpx"
)

type Log struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	VisitID    *uuid.UUID
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	FromStatus *string
	ToStatus   *string
	Reason     *string
	Metadata   json.RawMessage `gorm:"type:jsonb"`
	CreatedAt  time.Time
}

func (Log) TableName() string { return "audit_logs" }

// Entry is what callers record. ActorRole "SYSTEM" means a background job.
type Entry struct {
	VisitID   *uuid.UUID
	ActorID   *uuid.UUID
	ActorRole string
	Action    string
	From, To  string
	Reason    string
	Metadata  map[string]any
}

type Service struct {
	db    *gorm.DB
	clock clock.Clock
}

func New(g *gorm.DB, clk clock.Clock) *Service { return &Service{db: g, clock: clk} }

// Record inserts an entry using tx (a transaction of the calling module).
func (s *Service) Record(ctx context.Context, tx *gorm.DB, e Entry) error {
	l := Log{ID: uuid.New(), VisitID: e.VisitID, ActorID: e.ActorID, ActorRole: e.ActorRole, Action: e.Action,
		FromStatus: ptr(e.From), ToStatus: ptr(e.To), Reason: ptr(e.Reason), CreatedAt: s.clock.Now()}
	if len(e.Metadata) > 0 {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
		l.Metadata = b
	}
	return tx.WithContext(ctx).Create(&l).Error
}

func (s *Service) ForVisit(ctx context.Context, visitID uuid.UUID) ([]Log, error) {
	var out []Log
	err := s.db.WithContext(ctx).Where("visit_id = ?", visitID).Order("seq").Find(&out).Error
	return out, err
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---- HTTP ----

type EntryDTO struct {
	ID         uuid.UUID       `json:"id"`
	Action     string          `json:"action"`
	ActorID    *uuid.UUID      `json:"actorId"`
	ActorRole  string          `json:"actorRole"`
	FromStatus *string         `json:"fromStatus"`
	ToStatus   *string         `json:"toStatus"`
	Reason     *string         `json:"reason"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	r.With(httpx.RequireRoles(authctx.RoleAdmin)).Get("/visits/{id}/audit", httpx.H(h.list))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	rows, err := h.svc.ForVisit(r.Context(), id)
	if err != nil {
		return db.MapError(err)
	}
	out := make([]EntryDTO, 0, len(rows))
	for _, l := range rows {
		out = append(out, EntryDTO{l.ID, l.Action, l.ActorID, l.ActorRole, l.FromStatus, l.ToStatus, l.Reason, l.Metadata, l.CreatedAt})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}
