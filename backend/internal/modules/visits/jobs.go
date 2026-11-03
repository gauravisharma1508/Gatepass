package visits

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gatepass/internal/platform/scheduler"
)

// Jobs are registered with the shared scheduler by internal/app (spec §10).
func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{
		{Name: "expireApprovals", Spec: "@every 30s", Run: func(ctx context.Context) error { _, err := s.ExpireDue(ctx); return err }},
		{Name: "approvalReminders", Spec: "@every 30s", Run: func(ctx context.Context) error { _, err := s.ApprovalReminders(ctx); return err }},
		{Name: "overstayAlerts", Spec: "@every 1m", Run: func(ctx context.Context) error { _, err := s.OverstayAlerts(ctx); return err }},
		{Name: "markNoShows", Spec: "5 0 * * *", Run: func(ctx context.Context) error { _, err := s.MarkNoShows(ctx); return err }},
		{Name: "purgeRecords", Spec: "30 2 * * *", Run: func(ctx context.Context) error { _, err := s.PurgeRecords(ctx); return err }},
	}
}

// ExpireDue settles pending walk-ins whose approval window has passed (idempotent).
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	var ids []uuid.UUID
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).
		Where("status = ? AND approval_deadline <= ?", StatusPendingApproval, s.d.Clock.Now()).Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := s.expire(ctx, id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ApprovalReminders warns hosts once when little time is left to answer.
func (s *Service) ApprovalReminders(ctx context.Context) (int, error) {
	set, err := s.d.Settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	now := s.d.Clock.Now()
	var ids []uuid.UUID
	err = s.d.DB.WithContext(ctx).Model(&Visit{}).
		Where("status = ? AND reminder_sent_at IS NULL AND approval_deadline > ? AND approval_deadline <= ?",
			StatusPendingApproval, now, now.Add(time.Duration(set.ApprovalReminderMin)*time.Minute)).Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		res := s.d.DB.WithContext(ctx).Model(&Visit{}).Where("id = ? AND status = ? AND reminder_sent_at IS NULL", id, StatusPendingApproval).
			Update("reminder_sent_at", now)
		if res.Error != nil {
			return n, res.Error
		}
		if res.RowsAffected == 1 {
			if v, err := s.load(ctx, s.d.DB, id); err == nil {
				s.publish(ctx, s.event(v, EventApprovalReminder))
				n++
			}
		}
	}
	return n, nil
}

// OverstayAlerts tells the host and security once when a visitor is still checked in past the expected duration.
func (s *Service) OverstayAlerts(ctx context.Context) (int, error) {
	now := s.d.Clock.Now()
	var ids []uuid.UUID
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).
		Where("status = ? AND overstay_notified_at IS NULL AND checked_in_at + (expected_duration_min * interval '1 minute') < ?", StatusCheckedIn, now).
		Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		res := s.d.DB.WithContext(ctx).Model(&Visit{}).Where("id = ? AND status = ? AND overstay_notified_at IS NULL", id, StatusCheckedIn).
			Update("overstay_notified_at", now)
		if res.Error != nil {
			return n, res.Error
		}
		if res.RowsAffected == 1 {
			if v, err := s.load(ctx, s.d.DB, id); err == nil {
				s.publish(ctx, s.event(v, EventOverstay))
				n++
			}
		}
	}
	return n, nil
}

// MarkNoShows closes SCHEDULED/APPROVED visits whose date has passed (BR-6).
func (s *Service) MarkNoShows(ctx context.Context) (int, error) {
	var ids []uuid.UUID
	err := s.d.DB.WithContext(ctx).Model(&Visit{}).
		Where("status IN ? AND visit_date < ?", []Status{StatusScheduled, StatusApproved}, s.d.Clock.Today()).Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if _, err := s.transition(ctx, id, tparams{action: ActionNoShow, actor: SystemActor()}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// PurgeRecords deletes visits that ended before the record-retention period (notifications cascade;
// audit rows stay). Visits that still have photo rows are kept until the photos are purged.
func (s *Service) PurgeRecords(ctx context.Context) (int, error) {
	set, err := s.d.Settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := s.d.Clock.Now().AddDate(0, 0, -set.RecordRetentionDays)
	res := s.d.DB.WithContext(ctx).Exec(`DELETE FROM visits WHERE status IN ? AND updated_at < ?
		AND NOT EXISTS (SELECT 1 FROM visit_photos p WHERE p.visit_id = visits.id)`, finalStatuses, cutoff)
	return int(res.RowsAffected), res.Error
}
