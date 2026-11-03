//go:build integration

package app_test

import (
	"github.com/google/uuid"
	"testing"

	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

type world struct {
	*testutil.Env
	admin, empA, empB, guard, guard2 *testutil.Client
}

// newWorld starts an app and signs in every seed user.
func newWorld(t *testing.T, opts ...testutil.Option) *world {
	t.Helper()
	e := testutil.New(t, opts...)
	return &world{Env: e, admin: e.Login("admin"), empA: e.Login("empA"), empB: e.Login("empB"), guard: e.Login("guard"), guard2: e.Login("guard2")}
}

func (w *world) planned(c *testutil.Client, over map[string]any) string {
	w.T.Helper()
	r := c.Post("/api/visits/planned", testutil.PlannedBody(over)).Expect(201)
	return r.Str("id")
}

func (w *world) walkIn(hostKey string, over map[string]any) string {
	w.T.Helper()
	r := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID(hostKey), over)).Expect(201)
	return r.Str("id")
}

func (w *world) photos(id string) {
	w.T.Helper()
	img := testutil.JPEG(w.T, 640, 480)
	w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", img, nil).Expect(204)
	w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", img, map[string]string{"idType": "AADHAAR", "idLast4": "1234"}).Expect(204)
}

// checkedIn returns a planned visit of empA that is checked in.
func (w *world) checkedIn(over map[string]any) string {
	w.T.Helper()
	id := w.planned(w.empA, over)
	w.photos(id)
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(200)
	return id
}

func (w *world) status(c interface{ Get(string) *testutil.Resp }, id string) string {
	w.T.Helper()
	r := c.Get("/api/visits/" + id)
	require.Equal(w.T, 200, r.Status, string(r.Raw))
	return r.Str("status")
}

func (w *world) auditActions(id string) []string {
	w.T.Helper()
	r := w.admin.Get("/api/visits/" + id + "/audit").Expect(200)
	var out []string
	for i := 0; i < r.Len("items"); i++ {
		out = append(out, r.Str("items."+itoa(i)+".action"))
	}
	return out
}

func (w *world) countAudit(id, action string) int {
	n := 0
	for _, a := range w.auditActions(id) {
		if a == action {
			n++
		}
	}
	return n
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func (w *world) notifications(c *testutil.Client) []string {
	w.T.Helper()
	r := c.Get("/api/notifications?pageSize=100").Expect(200)
	var out []string
	for i := 0; i < r.Len("items"); i++ {
		out = append(out, r.Str("items."+itoa(i)+".type"))
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func count(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// dbStatus reads the status straight from the database (no lazy expiry, unlike the API).
func (w *world) dbStatus(id string) string {
	w.T.Helper()
	var s string
	require.NoError(w.T, w.Owner.QueryRow("SELECT status FROM visits WHERE id = $1", id).Scan(&s))
	return s
}

func mustUUID(s string) uuid.UUID { return uuid.MustParse(s) }
