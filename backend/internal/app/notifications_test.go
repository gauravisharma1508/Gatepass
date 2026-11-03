//go:build integration

package app_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

func TestNotifications_Lifecycle_TC_NT_001_to_010(t *testing.T) {
	w := newWorld(t)
	id := w.walkIn("empA", nil)
	w.empA.Post("/api/visits/"+id+"/reject", map[string]any{"reason": "Busy"}).Expect(200)

	// security is told about decisions (FR-7.3), including the reason
	g := w.guard.Get("/api/notifications").Expect(200)
	assert.EqualValues(t, 1, g.Path("unreadCount"))
	assert.Equal(t, "APPROVAL_DECIDED", g.Str("items.0.type"))
	assert.Contains(t, g.Str("items.0.body"), "Busy")
	assert.Equal(t, id, g.Str("items.0.visitId"))

	// the host's own list: only theirs
	r := w.empA.Get("/api/notifications").Expect(200)
	assert.EqualValues(t, 1, r.Path("unreadCount"))
	assert.Equal(t, "APPROVAL_REQUESTED", r.Str("items.0.type"))
	assert.Empty(t, w.notifications(w.empB))

	nid := r.Str("items.0.id")
	assert.Equal(t, 404, w.empB.Post("/api/notifications/"+nid+"/read", nil).Status, "cannot touch someone else's notification")
	w.empA.Post("/api/notifications/"+nid+"/read", nil).Expect(204)
	w.empA.Post("/api/notifications/"+nid+"/read", nil).Expect(204) // idempotent
	assert.EqualValues(t, 0, w.empA.Get("/api/notifications").Expect(200).Path("unreadCount"))
	assert.Equal(t, 0, w.empA.Get("/api/notifications?unread=true").Expect(200).Len("items"))

	w.guard.Post("/api/notifications/read-all", nil).Expect(204)
	assert.EqualValues(t, 0, w.guard.Get("/api/notifications").Expect(200).Path("unreadCount"))
	assert.Equal(t, 1, w.guard.Get("/api/notifications").Expect(200).Len("items"), "read notifications stay in the list")
}

func TestNotifications_DecisionOutcomes_TC_NT_004(t *testing.T) {
	w := newWorld(t)
	approve := w.walkIn("empA", map[string]any{"visitorMobile": "9100000001"})
	w.empA.Post("/api/visits/"+approve+"/approve", nil).Expect(200)
	reject := w.walkIn("empA", map[string]any{"visitorMobile": "9100000002"})
	w.empA.Post("/api/visits/"+reject+"/reject", nil).Expect(200)
	cancel := w.walkIn("empA", map[string]any{"visitorMobile": "9100000003"})
	w.guard.Post("/api/visits/"+cancel+"/cancel", nil).Expect(200)
	expire := w.walkIn("empA", map[string]any{"visitorMobile": "9100000004"})
	w.Clock.Advance(31 * time.Minute)
	_, err := w.App.Visits.ExpireDue(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "EXPIRED", w.dbStatus(expire))

	assert.Equal(t, 4, count(w.notifications(w.guard), "APPROVAL_DECIDED"), "security hears about all four outcomes")
	assert.Equal(t, 4, count(w.notifications(w.guard2), "APPROVAL_DECIDED"))
	// the host is told only when a request expires (plus the 4 original requests)
	own := w.notifications(w.empA)
	assert.Equal(t, 4, count(own, "APPROVAL_REQUESTED"))
	assert.Equal(t, 1, count(own, "APPROVAL_DECIDED"))
}

func TestNotifications_InactiveRecipientsSkipped_TC_NT_006(t *testing.T) {
	w := newWorld(t)
	w.admin.Post("/api/users/"+w.UserID("guard2")+"/deactivate", nil).Expect(200)
	id := w.checkedIn(nil)
	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	assert.Contains(t, w.notifications(w.guard), "MEETING_OVER")
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'MEETING_OVER'", w.UserID("guard2")).Scan(&n))
	assert.Zero(t, n, "a deactivated security user is not notified")
}

func TestNotifications_EmailFailureNeverBreaksTheAction_TC_NT_007(t *testing.T) {
	w := newWorld(t)
	w.Mail.Err = errors.New("smtp is down")
	id := w.walkIn("empA", nil) // would fail with 500 if mail errors bubbled up
	assert.Contains(t, w.notifications(w.empA), "APPROVAL_REQUESTED", "the in-app notification is still created")
	assert.Equal(t, "PENDING_APPROVAL", w.status(w.guard, id))
	assert.Empty(t, w.Mail.Messages())
}

// ---- real SSE streams (spec §9.2) ----

type stream struct {
	status int
	lines  chan string
	cancel context.CancelFunc
}

func openStream(t *testing.T, c *testutil.Client) *stream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, "GET", c.Base+"/api/events", nil)
	require.NoError(t, err)
	hc := *c.HC
	hc.Timeout = 0
	resp, err := hc.Do(req)
	require.NoError(t, err)
	s := &stream{status: resp.StatusCode, lines: make(chan string, 256), cancel: cancel}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		close(s.lines)
		return s
	}
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	go func() {
		defer resp.Body.Close()
		defer close(s.lines)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
	}()
	s.waitFor(t, ": connected", 3*time.Second)
	return s
}

// waitFor returns true when a line containing want arrives in time.
func (s *stream) wait(want string, d time.Duration) bool {
	deadline := time.After(d)
	for {
		select {
		case l, ok := <-s.lines:
			if !ok {
				return false
			}
			if strings.Contains(l, want) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func (s *stream) waitFor(t *testing.T, want string, d time.Duration) {
	t.Helper()
	require.True(t, s.wait(want, d), "expected %q on the event stream", want)
}

func TestEventStream_AuthAndDelivery_TC_NT_011_to_013(t *testing.T) {
	w := newWorld(t)
	assert.Equal(t, 401, openStream(t, w.Anon()).status, "no session, no stream")

	empA, empB, guard := openStream(t, w.empA), openStream(t, w.empB), openStream(t, w.guard)
	id := w.walkIn("empA", nil)

	empA.waitFor(t, "event: visit-updated", 5*time.Second) // sent first, then the notification
	empA.waitFor(t, "event: notification", 5*time.Second)
	guard.waitFor(t, "event: visit-updated", 5*time.Second)
	assert.False(t, empB.wait("event:", 700*time.Millisecond), "an unrelated employee receives nothing")

	// a failed action emits nothing (events are sent after commit only)
	w.empB.Post("/api/visits/"+id+"/approve", nil).Expect(403)
	assert.False(t, guard.wait("event: visit-updated", 700*time.Millisecond))

	// a successful one does, and the payload carries the new status
	w.empA.Post("/api/visits/"+id+"/approve", nil).Expect(200)
	guard.waitFor(t, `"status":"APPROVED"`, 5*time.Second)
}

func TestEventStream_EndsWhenUserIsDeactivated(t *testing.T) {
	w := newWorld(t)
	s := openStream(t, w.empB)
	w.admin.Post("/api/users/"+w.UserID("empB")+"/deactivate", nil).Expect(200)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-s.lines:
			if !ok {
				return // stream closed by the server
			}
		case <-deadline:
			t.Fatal("the stream of a deactivated user must be closed")
		}
	}
}
