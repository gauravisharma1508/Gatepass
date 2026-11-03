// Package scheduler runs the modules' jobs with robfig/cron. Each run first takes a
// transaction-scoped Postgres advisory lock, so only one instance runs a job at a time.
package scheduler

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Job struct {
	Name string
	Spec string // cron expression or "@every 30s"
	Run  func(ctx context.Context) error
}

type Scheduler struct {
	cron *cron.Cron
	db   *gorm.DB
	log  *slog.Logger
	mu   sync.Mutex
	jobs []Job
}

func New(g *gorm.DB, loc *time.Location, lg *slog.Logger) *Scheduler {
	return &Scheduler{cron: cron.New(cron.WithLocation(loc)), db: g, log: lg}
}

func (s *Scheduler) Add(j Job) error {
	s.mu.Lock()
	s.jobs = append(s.jobs, j)
	s.mu.Unlock()
	_, err := s.cron.AddFunc(j.Spec, func() { s.RunOnce(context.Background(), j) })
	return err
}

func (s *Scheduler) Jobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Job(nil), s.jobs...)
}

// RunOnce executes j under the advisory lock (skipped if another instance holds it).
func (s *Scheduler) RunOnce(ctx context.Context, j Job) {
	h := fnv.New32a()
	h.Write([]byte(j.Name))
	id := int64(h.Sum32())
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ok bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", id).Scan(&ok).Error; err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return j.Run(ctx)
	})
	if err != nil {
		s.log.Error("job failed", "job", j.Name, "err", err)
	}
}

func (s *Scheduler) Start() { s.cron.Start() }

// Stop stops scheduling and waits for running jobs (or ctx) to finish.
func (s *Scheduler) Stop(ctx context.Context) {
	select {
	case <-s.cron.Stop().Done():
	case <-ctx.Done():
	}
}
