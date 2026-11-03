//go:build integration

package app_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/platform/scheduler"
	"gatepass/internal/testutil"
)

func TestJob_ExpireApprovals_TC_JOB_001_002(t *testing.T) {
	w := newWorld(t)
	first := w.walkIn("empA", nil) // deadline 10:30
	w.Clock.Advance(15 * time.Minute)
	second := w.walkIn("empB", map[string]any{"visitorMobile": "9200000002"}) // deadline 10:45
	w.Clock.Advance(16 * time.Minute)                                         // now 10:31

	n, err := w.App.Visits.ExpireDue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "EXPIRED", w.dbStatus(first))
	assert.Equal(t, "PENDING_APPROVAL", w.dbStatus(second))

	n, err = w.App.Visits.ExpireDue(t.Context())
	require.NoError(t, err)
	assert.Zero(t, n, "idempotent: nothing left to do")
	assert.Equal(t, 1, w.countAudit(first, "EXPIRED"))
	assert.Equal(t, 1, count(w.notifications(w.guard), "APPROVAL_DECIDED"), "security told once")
	assert.Equal(t, 1, count(w.notifications(w.empA), "APPROVAL_DECIDED"), "the host is told it expired, once")
}

func TestJob_ApprovalReminder_TC_JOB_003_004(t *testing.T) {
	w := newWorld(t)
	pending := w.walkIn("empA", nil)
	answered := w.walkIn("empA", map[string]any{"visitorMobile": "9200000003"})
	w.empA.Post("/api/visits/"+answered+"/approve", nil).Expect(200)

	w.Clock.Set(testutil.Start.Add(19 * time.Minute)) // 11 minutes left
	n, err := w.App.Visits.ApprovalReminders(t.Context())
	require.NoError(t, err)
	assert.Zero(t, n)

	w.Clock.Set(testutil.Start.Add(20 * time.Minute)) // 10 minutes left
	n, err = w.App.Visits.ApprovalReminders(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the unanswered request")
	n, _ = w.App.Visits.ApprovalReminders(t.Context())
	assert.Zero(t, n, "once per request")
	assert.Equal(t, 1, count(w.notifications(w.empA), "APPROVAL_REMINDER"))

	// no reminder once the window is over
	w.Clock.Set(testutil.Start.Add(31 * time.Minute))
	n, _ = w.App.Visits.ApprovalReminders(t.Context())
	assert.Zero(t, n)
	_ = pending
}

func TestJob_Overstay_TC_JOB_005_006(t *testing.T) {
	w := newWorld(t)
	over := w.checkedIn(nil) // 60 minutes from 10:00
	done := w.checkedIn(map[string]any{"visitorMobile": "9300000002"})
	w.Clock.Set(testutil.Start.Add(30 * time.Minute))
	w.empA.Post("/api/visits/"+done+"/meeting-over", nil).Expect(200)

	w.Clock.Set(testutil.Start.Add(59 * time.Minute))
	n, err := w.App.Visits.OverstayAlerts(t.Context())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, false, w.admin.Get("/api/visits/"+over).Path("isOverstay"))

	w.Clock.Set(testutil.Start.Add(61 * time.Minute))
	n, err = w.App.Visits.OverstayAlerts(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the visit marked Meeting Over is not an overstay")
	n, _ = w.App.Visits.OverstayAlerts(t.Context())
	assert.Zero(t, n, "once per visit")
	assert.Equal(t, true, w.admin.Get("/api/visits/"+over).Path("isOverstay"))
	assert.Equal(t, 1, count(w.notifications(w.empA), "OVERSTAY"))
	assert.Equal(t, 1, count(w.notifications(w.guard), "OVERSTAY"))
	assert.Equal(t, 1, count(w.notifications(w.guard2), "OVERSTAY"))
	assert.Equal(t, 1, w.guard.Get("/api/dashboard/security").Expect(200).Len("overstays"))
}

func TestJob_NoShow_AtOfficeMidnight_TC_JOB_007_008(t *testing.T) {
	w := newWorld(t)
	planned := w.planned(w.empA, nil)
	walkin := w.walkIn("empA", map[string]any{"visitorMobile": "9400000002"})
	w.empA.Post("/api/visits/"+walkin+"/approve", nil).Expect(200)
	tomorrow := w.planned(w.empA, map[string]any{"visitDate": "2026-11-04", "visitorMobile": "9400000003"})

	w.Clock.Set(time.Date(2026, 11, 3, 18, 29, 0, 0, time.UTC)) // 23:59 IST
	n, err := w.App.Visits.MarkNoShows(t.Context())
	require.NoError(t, err)
	assert.Zero(t, n, "still the visit date in the office time zone")

	w.Clock.Set(time.Date(2026, 11, 3, 18, 35, 0, 0, time.UTC)) // 00:05 IST on 4 Nov
	n, err = w.App.Visits.MarkNoShows(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "NO_SHOW", w.dbStatus(planned))
	assert.Equal(t, "NO_SHOW", w.dbStatus(walkin), "an approved walk-in that never came in")
	assert.Equal(t, "SCHEDULED", w.dbStatus(tomorrow))
}

func TestJob_PhotoAndRecordRetention_TC_JOB_009_010(t *testing.T) {
	w := newWorld(t)
	finish := func(mobile string) string {
		id := w.checkedIn(map[string]any{"visitorMobile": mobile, "visitDate": string(w.Clock.Today())})
		w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
		w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(200)
		return id
	}
	a := finish("9500000001")                                                 // ends on day 0
	stillInside := w.checkedIn(map[string]any{"visitorMobile": "9500000003"}) // never ends
	w.Clock.Advance(40 * 24 * time.Hour)
	b := finish("9500000002") // ends on day 40
	require.Len(t, w.Store.Keys(), 6)

	w.Clock.Advance(51 * 24 * time.Hour) // day 91
	n, err := w.App.Photos.PurgeOld(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, n, "only the visit that ended more than 90 days ago")
	assert.Len(t, w.Store.Keys(), 4)
	w.guard.Get("/api/visits/" + a + "/photos/VISITOR").Expect(404)
	w.guard.Get("/api/visits/" + b + "/photos/VISITOR").Expect(200)
	w.guard.Get("/api/visits/" + stillInside + "/photos/VISITOR").Expect(200)
	var purged *time.Time
	require.NoError(t, w.Owner.QueryRow("SELECT photos_purged_at FROM visits WHERE id = $1", a).Scan(&purged))
	assert.NotNil(t, purged)
	assert.Equal(t, false, w.admin.Get("/api/visits/"+a).Path("hasVisitorPhoto"))

	// records: deleted after 365 days; notifications go with them, audit rows stay
	w.Clock.Advance(275 * 24 * time.Hour) // day 366
	_, err = w.App.Photos.PurgeOld(t.Context())
	require.NoError(t, err)
	deleted, err := w.App.Visits.PurgeRecords(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "visit A only: B ended 326 days ago, the other visitor is still inside")
	var visits, notes, audits int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM visits WHERE id = $1", a).Scan(&visits))
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM notifications WHERE visit_id = $1", a).Scan(&notes))
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM audit_logs WHERE visit_id = $1", a).Scan(&audits))
	assert.Zero(t, visits)
	assert.Zero(t, notes)
	assert.Greater(t, audits, 0, "the audit trail outlives the visit")
}

// TC-JOB-012: two instances run the same job at once; only one does the work.
func TestJob_SingleRunner_TC_JOB_012(t *testing.T) {
	w := newWorld(t)
	var runs atomic.Int32
	job := scheduler.Job{Name: "lock-test", Run: func(ctx context.Context) error { runs.Add(1); time.Sleep(400 * time.Millisecond); return nil }}
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() { w.App.Scheduler.RunOnce(t.Context(), job); done <- struct{}{} }()
	}
	<-done
	<-done
	assert.EqualValues(t, 1, runs.Load())
	w.App.Scheduler.RunOnce(t.Context(), job) // after the first finished, it can run again
	assert.EqualValues(t, 2, runs.Load())
}

func TestJob_RegisteredJobs(t *testing.T) {
	w := newWorld(t)
	var names []string
	for _, j := range w.App.Scheduler.Jobs() {
		names = append(names, j.Name)
	}
	assert.ElementsMatch(t, []string{"expireApprovals", "approvalReminders", "overstayAlerts", "markNoShows", "purgeRecords", "purgePhotos", "purgeTokens"}, names)
}

func TestJob_PurgeTokens_TC_JOB_011(t *testing.T) {
	w := newWorld(t)
	uid := w.UserID("empA")
	ins := func(hash, expires, used string) {
		_, err := w.Owner.Exec(`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used_at, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3::timestamptz, NULLIF($4,'')::timestamptz, now())`, uid, hash, expires, used)
		require.NoError(t, err)
	}
	ins("expired", "2026-10-30T00:00:00Z", "")                  // expired days ago
	ins("used", "2026-11-04T00:00:00Z", "2026-11-03T03:00:00Z") // already used
	ins("valid", "2026-11-04T00:00:00Z", "")                    // still usable
	for _, j := range w.App.Scheduler.Jobs() {
		if j.Name == "purgeTokens" {
			require.NoError(t, j.Run(t.Context()))
		}
	}
	rows, err := w.Owner.Query(`SELECT token_hash FROM password_reset_tokens`)
	require.NoError(t, err)
	defer rows.Close()
	var left []string
	for rows.Next() {
		var h string
		require.NoError(t, rows.Scan(&h))
		left = append(left, h)
	}
	assert.Equal(t, []string{"valid"}, left)
}
