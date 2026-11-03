//go:build integration

package app_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

func TestWalkIn_Create_TC_WI_001_002(t *testing.T) {
	w := newWorld(t)
	r := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empA"), map[string]any{"visitDate": "2026-11-09"})).Expect(201)
	assert.Equal(t, "PENDING_APPROVAL", r.Str("status"))
	assert.Equal(t, "UNPLANNED", r.Str("type"))
	assert.Equal(t, "2026-11-03", r.Str("visitDate"), "a walk-in is always for today")
	assert.Equal(t, "2026-11-03T05:00:00Z", r.Str("approvalDeadline"), "10:00 IST + 30 minutes")
	assert.EqualValues(t, 1800, r.Path("approvalSecondsLeft"))
	assert.Equal(t, "Anita Desai", r.Str("host.name"))
	assert.ElementsMatch(t, []string{"CANCEL"}, r.Strings("allowedActions"), "security can cancel the request")

	// the host is told in the app and by email
	assert.Contains(t, w.notifications(w.empA), "APPROVAL_REQUESTED")
	mails := w.Mail.To(testutil.Email("empA"))
	require.Len(t, mails, 1)
	assert.Contains(t, mails[0].Body, "Ravi Kumar from Acme")
	assert.Contains(t, mails[0].Body, "/visits/"+r.Str("id"))
	assert.Empty(t, w.notifications(w.empB), "nobody else is told")
	assert.Empty(t, w.notifications(w.guard), "security is told about decisions, not about their own request")
}

func TestWalkIn_InvalidHostAndRole_TC_WI_003_to_006(t *testing.T) {
	w := newWorld(t)
	for name, host := range map[string]string{"inactive": w.UserID("exEmp"), "security user": w.UserID("guard2"),
		"unknown": "00000000-0000-0000-0000-000000000000", "not a uuid": "nope", "empty": ""} {
		r := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(host, nil)).Expect(400)
		assert.NotEmpty(t, r.Path("error.details.fields.hostId"), name)
	}
	w.empA.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empB"), nil)).Expect(403)
	w.admin.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empB"), nil)).Expect(403)

	first := w.walkIn("empA", nil)
	r := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empB"), nil)).Expect(409)
	assert.Equal(t, "VISITOR_ALREADY_ACTIVE", r.Code(), "same mobile, first request still pending")
	_ = first

	// a visitor who is inside cannot be registered again either
	inside := w.checkedIn(map[string]any{"visitorMobile": "9123456780", "visitorName": "Inside Person"})
	_ = inside
	r = w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empB"), map[string]any{"visitorMobile": "9123456780"})).Expect(409)
	assert.Equal(t, "VISITOR_ALREADY_ACTIVE", r.Code())
}

func TestWalkIn_AcceptAndReject_TC_WI_007_to_009(t *testing.T) {
	w := newWorld(t)
	id := w.walkIn("empA", nil)
	w.Clock.Advance(15 * time.Minute)
	r := w.empA.Post("/api/visits/"+id+"/approve", nil).Expect(200)
	assert.Equal(t, "APPROVED", r.Str("status"))
	assert.NotEmpty(t, r.Str("decidedAt"))
	assert.Contains(t, w.notifications(w.guard), "APPROVAL_DECIDED")
	assert.Contains(t, w.notifications(w.guard2), "APPROVAL_DECIDED", "every security user is told")
	assert.Contains(t, w.guard.Get("/api/visits/"+id).Strings("allowedActions"), "UPLOAD_PHOTOS")

	// reject with a reason
	id2 := w.walkIn("empA", map[string]any{"visitorMobile": "9111111111"})
	rej := w.empA.Post("/api/visits/"+id2+"/reject", map[string]any{"reason": "Not expecting anyone"}).Expect(200)
	assert.Equal(t, "REJECTED", rej.Str("status"))
	assert.Equal(t, "Not expecting anyone", rej.Str("decisionReason"))
	assert.Empty(t, w.guard.Get("/api/visits/"+id2).Strings("allowedActions"), "no check-in after a rejection")
	// reject without a reason is fine
	id3 := w.walkIn("empA", map[string]any{"visitorMobile": "9222222222"})
	w.empA.Post("/api/visits/"+id3+"/reject", nil).Expect(200)
}

func TestWalkIn_WindowRules_TC_WI_010_to_013(t *testing.T) {
	w := newWorld(t)

	// answer after 30 minutes: refused, and the visit is settled as expired
	id := w.walkIn("empA", nil)
	w.Clock.Advance(30*time.Minute + time.Second)
	r := w.empA.Post("/api/visits/"+id+"/approve", nil).Expect(409)
	assert.Equal(t, "APPROVAL_WINDOW_CLOSED", r.Code())
	assert.Equal(t, "EXPIRED", w.status(w.guard, id))
	assert.Contains(t, w.notifications(w.empA), "APPROVAL_DECIDED", "the host is told it expired")
	w.empA.Post("/api/visits/"+id+"/reject", nil).Expect(409)

	// only the host may decide
	w.Clock.Set(testutil.Start)
	id2 := w.walkIn("empA", map[string]any{"visitorMobile": "9333333333"})
	w.empB.Post("/api/visits/"+id2+"/approve", nil).Expect(403)
	w.guard.Post("/api/visits/"+id2+"/approve", nil).Expect(403)
	w.admin.Post("/api/visits/"+id2+"/approve", nil).Expect(403)

	// a decision is final
	w.empA.Post("/api/visits/"+id2+"/approve", nil).Expect(200)
	w.empA.Post("/api/visits/"+id2+"/reject", nil).Expect(409)

	// the window is fixed when the request is created; a later settings change only affects new requests
	id3 := w.walkIn("empB", map[string]any{"visitorMobile": "9444444444"})
	w.admin.Patch("/api/settings", map[string]any{"approvalWindowMin": 15, "approvalReminderMin": 5}).Expect(200)
	w.Clock.Advance(20 * time.Minute)
	w.empB.Post("/api/visits/"+id3+"/approve", nil).Expect(200)
	w.Clock.Set(testutil.Start)
	r4 := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empA"), map[string]any{"visitorMobile": "9555555555"})).Expect(201)
	assert.Equal(t, "2026-11-03T04:45:00Z", r4.Str("approvalDeadline"), "new walk-ins use the new 15-minute window")
}

func TestWalkIn_CancelAndRetry_TC_WI_014_to_016(t *testing.T) {
	w := newWorld(t)
	id := w.walkIn("empA", nil)
	w.empA.Post("/api/visits/"+id+"/cancel", nil).Expect(403) // the host cannot cancel the walk-in
	w.guard.Post("/api/visits/"+id+"/cancel", map[string]any{"reason": "Visitor left"}).Expect(200)
	assert.Equal(t, "CANCELLED", w.status(w.guard, id))
	w.empA.Post("/api/visits/"+id+"/approve", nil).Expect(409)

	// a rejected visitor may be registered again for another host; the old record stays untouched
	id2 := w.walkIn("empA", map[string]any{"visitorMobile": "9666666666"})
	w.empA.Post("/api/visits/"+id2+"/reject", map[string]any{"reason": "Busy"}).Expect(200)
	id3 := w.walkIn("empB", map[string]any{"visitorMobile": "9666666666"})
	assert.NotEqual(t, id2, id3)
	assert.Equal(t, "REJECTED", w.status(w.guard, id2))
	assert.Equal(t, "PENDING_APPROVAL", w.status(w.guard, id3))
}

// TC-WI-017: the host answers at the very moment the expiry job runs: exactly one outcome.
func TestWalkIn_RaceApproveVsExpiry_TC_WI_017(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 8; i++ {
		id := w.walkIn("empA", map[string]any{"visitorMobile": "9" + pad(i)})
		w.Clock.Set(testutil.Start.Add(30 * time.Minute)) // exactly the deadline
		var wg sync.WaitGroup
		var status int
		wg.Add(2)
		go func() { defer wg.Done(); _, err := w.App.Visits.ExpireDue(t.Context()); assert.NoError(t, err) }()
		go func() { defer wg.Done(); status, _, _ = w.empA.Raw("POST", "/api/visits/"+id+"/approve", nil) }()
		wg.Wait()
		assert.Equal(t, 409, status, "round %d: approving at the deadline must fail", i)
		assert.Equal(t, "EXPIRED", w.status(w.guard, id))
		assert.Equal(t, 1, w.countAudit(id, "EXPIRED"), "round %d: exactly one expiry is recorded", i)
		w.Clock.Set(testutil.Start)
	}
}

// TC-WI-018: accept and reject at once: one wins, the other gets a conflict.
func TestWalkIn_RaceApproveVsReject_TC_WI_018(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 8; i++ {
		id := w.walkIn("empA", map[string]any{"visitorMobile": "9" + pad(100+i)})
		var wg sync.WaitGroup
		var a, b int
		wg.Add(2)
		go func() { defer wg.Done(); a, _, _ = w.empA.Raw("POST", "/api/visits/"+id+"/approve", nil) }()
		go func() { defer wg.Done(); b, _, _ = w.empA.Raw("POST", "/api/visits/"+id+"/reject", nil) }()
		wg.Wait()
		assert.ElementsMatch(t, []int{200, 409}, []int{a, b}, "round %d", i)
		decided := w.countAudit(id, "APPROVED") + w.countAudit(id, "REJECTED")
		assert.Equal(t, 1, decided, "round %d: one decision recorded", i)
	}
}

func pad(i int) string {
	s := itoa(i)
	for len(s) < 9 {
		s = "0" + s
	}
	return s
}
