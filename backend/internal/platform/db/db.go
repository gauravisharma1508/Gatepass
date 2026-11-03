// Package db opens GORM and provides the transaction helper (spec §5.4).
// GORM is used for data access only: the schema comes from the goose migrations
// and AutoMigrate is never called.
package db

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/clock"
)

func init() {
	// BR-11: timestamps are UTC everywhere. pgx decodes timestamptz into time.Local, which would leak
	// the server's zone (e.g. +05:30) into API responses. The office zone is applied explicitly where
	// "today" is computed (platform/clock), never through Local. Set in init so it happens once,
	// before any goroutine exists (no data race).
	time.Local = time.UTC
}

func Open(url string, clk clock.Clock, lg *slog.Logger) (*gorm.DB, error) {
	g, err := gorm.Open(postgres.Open(url), &gorm.Config{
		TranslateError:         true,
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return clk.Now() },
		Logger: logger.New(log.New(slogWriter{lg}, "", 0), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true, // keep visitor data out of SQL logs
		}),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return g, nil
}

type slogWriter struct{ l *slog.Logger }

func (w slogWriter) Write(p []byte) (int, error) {
	w.l.Warn("gorm", "msg", strings.TrimSpace(string(p)))
	return len(p), nil
}

// Tx is a GORM transaction that can run callbacks after it commits.
type Tx struct {
	*gorm.DB
	after []func()
}

// AfterCommit queues f; it runs only if the transaction commits (events, emails).
func (t *Tx) AfterCommit(f func()) { t.after = append(t.after, f) }

func WithTx(ctx context.Context, g *gorm.DB, fn func(tx *Tx) error) error {
	t := &Tx{}
	err := g.WithContext(ctx).Transaction(func(x *gorm.DB) error {
		t.DB = x
		return fn(t)
	})
	if err != nil {
		return err
	}
	for _, f := range t.after {
		f()
	}
	return nil
}

// MapError turns GORM errors into the shared application errors (rule G-12).
func MapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperr.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return apperr.ErrDuplicate
	}
	return err
}
