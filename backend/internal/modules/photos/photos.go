// Package photos owns the visitor and ID photos: upload pipeline, storage, access rules, retention.
package photos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/modules/audit"
	"gatepass/internal/modules/settings"
	"gatepass/internal/modules/visits"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/scheduler"
	"gatepass/internal/platform/storage"
	"gatepass/internal/platform/validation"
)

type Kind string

const (
	KindVisitor    Kind = "VISITOR"
	KindIDDocument Kind = "ID_DOCUMENT"
)

var idTypes = map[string]bool{"AADHAAR": true, "PAN": true, "PASSPORT": true, "DRIVING_LICENCE": true, "VOTER_ID": true, "OTHER": true}

type Photo struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	VisitID      uuid.UUID
	Kind         Kind
	StorageKey   string
	MimeType     string
	SizeBytes    int
	IDType       *string
	IDLast4      *string
	CapturedByID uuid.UUID
	CapturedAt   time.Time
}

func (Photo) TableName() string { return "visit_photos" }

type Service struct {
	db       *gorm.DB
	store    storage.Store
	clock    clock.Clock
	audit    *audit.Service
	settings *settings.Service
	visits   *visits.Service
	log      *slog.Logger
}

func New(g *gorm.DB, st storage.Store, clk clock.Clock, a *audit.Service, set *settings.Service, lg *slog.Logger) *Service {
	return &Service{db: g, store: st, clock: clk, audit: a, settings: set, log: lg}
}

// SetVisits injects the visits service (wired second, because visits calls Summaries).
func (s *Service) SetVisits(v *visits.Service) { s.visits = v }

// Summaries implements visits.PhotoChecker.
func (s *Service) Summaries(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]visits.PhotoSummary, error) {
	out := map[uuid.UUID]visits.PhotoSummary{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []Photo
	if err := s.db.WithContext(ctx).Select("visit_id", "kind", "id_type").Where("visit_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		ps := out[r.VisitID]
		switch r.Kind {
		case KindVisitor:
			ps.HasVisitor = true
		case KindIDDocument:
			ps.HasID = true
			if r.IDType != nil {
				ps.IDType = *r.IDType
			}
		}
		out[r.VisitID] = ps
	}
	return out, nil
}

func parseKind(k string) (Kind, error) {
	switch Kind(k) {
	case KindVisitor, KindIDDocument:
		return Kind(k), nil
	}
	return "", apperr.ErrNotFound
}

type UploadInput struct {
	Kind    string
	Data    []byte
	IDType  string
	IDLast4 string
}

// Upload stores (or replaces, on retake) one photo of a visit. Security only; the visit must be
// SCHEDULED for today or APPROVED.
func (s *Service) Upload(ctx context.Context, p authctx.Principal, visitID uuid.UUID, in UploadInput) error {
	kind, err := parseKind(in.Kind)
	if err != nil {
		return err
	}
	if err := s.visits.CanUploadPhotos(ctx, p, visitID); err != nil {
		return err
	}
	f := validation.New()
	if kind == KindIDDocument && !idTypes[in.IDType] {
		f.Set("idType", "Choose the type of ID")
	}
	f.Set("idLast4", validation.IDLast4(in.IDLast4))
	if err := f.Err(); err != nil {
		return err
	}
	jpg, err := Process(in.Data)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("visits/%s/%s-%s.jpg", visitID, kind, uuid.New())
	if err := s.store.Put(ctx, key, jpg); err != nil {
		return err
	}
	var oldKey string
	err = db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		var old Photo
		res := tx.Where("visit_id = ? AND kind = ?", visitID, kind).Limit(1).Find(&old)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			oldKey = old.StorageKey
			if err := tx.Delete(&Photo{}, "id = ?", old.ID).Error; err != nil {
				return err
			}
		}
		row := Photo{ID: uuid.New(), VisitID: visitID, Kind: kind, StorageKey: key, MimeType: "image/jpeg", SizeBytes: len(jpg),
			CapturedByID: p.ID, CapturedAt: s.clock.Now()}
		if kind == KindIDDocument {
			row.IDType = &in.IDType
			if in.IDLast4 != "" {
				row.IDLast4 = &in.IDLast4
			}
		}
		if err := tx.Create(&row).Error; err != nil {
			return db.MapError(err)
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{VisitID: &visitID, ActorID: &p.ID, ActorRole: string(p.Role), Action: "PHOTO_CAPTURED",
			Metadata: map[string]any{"kind": kind}})
	})
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), key)
		return err
	}
	if oldKey != "" {
		if err := s.store.Delete(ctx, oldKey); err != nil {
			s.log.Warn("delete replaced photo", "err", err)
		}
	}
	return nil
}

// Open returns the photo for a viewer allowed to see it (BR-10): security and admin see both kinds,
// the host sees only the visitor photo of their own visit.
func (s *Service) Open(ctx context.Context, p authctx.Principal, visitID uuid.UUID, kindStr string) (io.ReadCloser, error) {
	kind, err := parseKind(kindStr)
	if err != nil {
		return nil, err
	}
	b, err := s.visits.Brief(ctx, visitID)
	if err != nil {
		return nil, err
	}
	switch {
	case p.Role == authctx.RoleSecurity || p.Role == authctx.RoleAdmin:
	case kind == KindVisitor && b.HostID == p.ID:
	default:
		return nil, apperr.ErrForbidden
	}
	var row Photo
	if err := s.db.WithContext(ctx).Where("visit_id = ? AND kind = ?", visitID, kind).First(&row).Error; err != nil {
		return nil, db.MapError(err)
	}
	rc, err := s.store.Open(ctx, row.StorageKey)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	return rc, nil
}

func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{{Name: "purgePhotos", Spec: "0 2 * * *", Run: func(ctx context.Context) error { _, err := s.PurgeOld(ctx); return err }}}
}

// PurgeOld deletes photos of visits that ended more than photoRetentionDays ago.
func (s *Service) PurgeOld(ctx context.Context) (int, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := s.clock.Now().AddDate(0, 0, -set.PhotoRetentionDays)
	ids, err := s.visits.FinalBefore(ctx, cutoff)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	var rows []Photo
	if err := s.db.WithContext(ctx).Where("visit_id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, err
	}
	var firstErr error
	for _, r := range rows {
		if err := s.store.Delete(ctx, r.StorageKey); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	if firstErr != nil {
		return 0, firstErr
	}
	if err := s.db.WithContext(ctx).Where("visit_id IN ?", ids).Delete(&Photo{}).Error; err != nil {
		return 0, err
	}
	return len(rows), s.visits.MarkPhotosPurged(ctx, ids)
}
