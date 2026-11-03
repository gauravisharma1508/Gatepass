// Package visitors owns visitor records, reused by mobile number (Q-5).
package visitors

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/httpx"
	"gatepass/internal/platform/validation"
)

type Visitor struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	Mobile    string
	Name      string
	Email     *string
	Company   *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Visitor) TableName() string { return "visitors" }

// UpsertInput must already be validated and normalised.
type UpsertInput struct{ Mobile, Name, Email, Company string }

type Service struct{ db *gorm.DB }

func New(g *gorm.DB) *Service { return &Service{db: g} }

// Upsert creates the visitor or updates the name (and email/company when given).
func (s *Service) Upsert(ctx context.Context, tx *gorm.DB, in UpsertInput) (*Visitor, error) {
	v := Visitor{ID: uuid.New(), Mobile: in.Mobile, Name: in.Name, Email: nilIfEmpty(in.Email), Company: nilIfEmpty(in.Company)}
	err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "mobile"}},
		DoUpdates: clause.Set{
			{Column: clause.Column{Name: "name"}, Value: gorm.Expr("excluded.name")},
			{Column: clause.Column{Name: "email"}, Value: gorm.Expr("COALESCE(excluded.email, visitors.email)")},
			{Column: clause.Column{Name: "company"}, Value: gorm.Expr("COALESCE(excluded.company, visitors.company)")},
			{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("excluded.updated_at")},
		},
	}).Create(&v).Error
	if err != nil {
		return nil, db.MapError(err)
	}
	var out Visitor
	if err := tx.WithContext(ctx).Where("mobile = ?", in.Mobile).First(&out).Error; err != nil {
		return nil, db.MapError(err)
	}
	return &out, nil
}

func (s *Service) Lookup(ctx context.Context, mobile string) (*Visitor, error) {
	var v Visitor
	if err := s.db.WithContext(ctx).Where("mobile = ?", mobile).First(&v).Error; err != nil {
		return nil, db.MapError(err)
	}
	return &v, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) { r.Get("/visitors/lookup", httpx.H(h.lookup)) }

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) error {
	mobile, msg := validation.Mobile(r.URL.Query().Get("mobile"))
	if msg != "" {
		return apperr.Validation(map[string]string{"mobile": msg})
	}
	v, err := h.svc.Lookup(r.Context(), mobile)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"name": v.Name, "email": v.Email, "company": v.Company})
	return nil
}
