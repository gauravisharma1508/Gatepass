//go:build integration

package app_test

import (
	"database/sql"
	"encoding/csv"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"gatepass/internal/platform/db"
	"gatepass/internal/testutil"
)

func TestDashboards_TC_DB_001_to_004(t *testing.T) {
	w := newWorld(t)
	w.planned(w.empA, map[string]any{"visitorMobile": "9600000001"})    // expected today
	w.walkIn("empA", map[string]any{"visitorMobile": "9600000002"})     // pending
	over := w.checkedIn(map[string]any{"visitorMobile": "9600000003"})  // on premises (will overstay)
	ready := w.checkedIn(map[string]any{"visitorMobile": "9600000004"}) // on premises + ready
	w.empA.Post("/api/visits/"+ready+"/meeting-over", nil).Expect(200)
	cancelled := w.planned(w.empA, map[string]any{"visitorMobile": "9600000005"})
	w.empA.Post("/api/visits/"+cancelled+"/cancel", nil).Expect(200)
	w.planned(w.empB, map[string]any{"visitorMobile": "9600000006", "visitDate": "2026-11-05"}) // empB, later

	d := w.guard.Get("/api/dashboard/security").Expect(200)
	assert.EqualValues(t, 1, d.Path("counts.expectedToday"))
	assert.EqualValues(t, 1, d.Path("counts.pendingApprovals"))
	assert.EqualValues(t, 2, d.Path("counts.onPremises"))
	assert.EqualValues(t, 1, d.Path("counts.readyForCheckout"))
	assert.EqualValues(t, 0, d.Path("counts.overstays"))
	assert.Equal(t, ready, d.Str("readyForCheckout.0.id"))
	assert.Positive(t, d.Path("pendingApprovals.0.approvalSecondsLeft"))

	// 61 minutes later: the pending request has expired, one visitor has overstayed, the one in "Meeting Over" has not
	w.Clock.Advance(61 * time.Minute)
	d = w.guard.Get("/api/dashboard/security").Expect(200)
	assert.EqualValues(t, 0, d.Path("counts.pendingApprovals"))
	assert.EqualValues(t, 1, d.Path("counts.overstays"))
	assert.Equal(t, over, d.Str("overstays.0.id"))
	assert.Equal(t, true, d.Path("overstays.0.isOverstay"))

	// employee dashboard: only own visits
	w.Clock.Set(testutil.Start)
	w.walkIn("empA", map[string]any{"visitorMobile": "9600000007"})
	w.planned(w.empA, map[string]any{"visitorMobile": "9600000008", "visitDate": "2026-11-06"})
	e := w.empA.Get("/api/dashboard/employee").Expect(200)
	assert.Equal(t, 1, e.Len("pendingApprovals"))
	assert.Equal(t, 1, e.Len("awaitingMeetingOver"))
	assert.Equal(t, 1, e.Len("upcoming"))
	assert.GreaterOrEqual(t, e.Len("todaysVisits"), 2)
	for i := 0; i < e.Len("todaysVisits"); i++ {
		assert.Equal(t, "Anita Desai", e.Str("todaysVisits."+itoa(i)+".host.name"))
	}
	assert.Equal(t, 1, w.empB.Get("/api/dashboard/employee").Expect(200).Len("upcoming"))

	// who may open which dashboard
	w.empA.Get("/api/dashboard/security").Expect(403)
	w.guard.Get("/api/dashboard/employee").Expect(403)
	w.admin.Get("/api/dashboard/security").Expect(200)
	w.admin.Get("/api/dashboard/employee").Expect(200)
}

func TestSettings_TC_SET_001_to_004(t *testing.T) {
	w := newWorld(t)
	g := w.admin.Get("/api/settings").Expect(200)
	assert.EqualValues(t, 30, g.Path("approvalWindowMin"))
	assert.EqualValues(t, 90, g.Path("photoRetentionDays"))

	p := w.admin.Patch("/api/settings", map[string]any{"approvalWindowMin": 20, "approvalReminderMin": 5}).Expect(200)
	assert.EqualValues(t, 20, p.Path("approvalWindowMin"))
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM audit_logs WHERE action = 'SETTINGS_UPDATED'").Scan(&n))
	assert.Equal(t, 1, n)
	r := w.guard.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empA"), nil)).Expect(201)
	assert.Equal(t, "2026-11-03T04:50:00Z", r.Str("approvalDeadline"), "10:00 + 20 min")

	for name, body := range map[string]map[string]any{
		"window too small":          {"approvalWindowMin": 5},
		"reminder not below window": {"approvalWindowMin": 20, "approvalReminderMin": 20},
		"photo retention too low":   {"photoRetentionDays": 3},
		"record retention < 90":     {"recordRetentionDays": 30},
		"advance days too large":    {"maxAdvanceDays": 91},
	} {
		assert.Equal(t, 400, w.admin.Patch("/api/settings", body).Status, name)
	}
	w.empA.Get("/api/settings").Expect(403)
	w.guard.Patch("/api/settings", map[string]any{"approvalWindowMin": 15}).Expect(403)
}

func TestReports_CSV_TC_RPT_001_to_004(t *testing.T) {
	w := newWorld(t)
	id := w.checkedIn(map[string]any{"visitorCompany": `=HYPERLINK("http://evil.example")`, "purpose": "+cmd|' /C calc'!A0"})
	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(200)
	w.planned(w.empA, map[string]any{"visitorMobile": "9700000001"}) // SCHEDULED: must be filtered out

	r := w.admin.Get("/api/reports/visits.csv?status=CHECKED_OUT").Expect(200)
	assert.Contains(t, r.Header.Get("Content-Type"), "text/csv")
	rows, err := csv.NewReader(strings.NewReader(string(r.Raw))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2, "header + the one checked-out visit")
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	row := rows[1]
	assert.Equal(t, "CHECKED_OUT", row[col["Status"]])
	assert.Equal(t, `'=HYPERLINK("http://evil.example")`, row[col["Company"]], "formula injection is neutralised")
	assert.True(t, strings.HasPrefix(row[col["Purpose"]], "'+"))
	assert.Equal(t, "2026-11-03 10:00", row[col["Checked in (IST)"]], "times are in the office time zone")
	body := string(r.Raw)
	for _, forbidden := range []string{"AADHAAR", "1234", ".jpg", "visits/"} {
		assert.NotContains(t, body, forbidden, "no photo or ID details in the report")
	}

	w.empA.Get("/api/reports/visits.csv").Expect(403)
	w.guard.Get("/api/reports/visits.csv").Expect(403)
	w.admin.Get("/api/reports/visits.csv?status=NOPE").Expect(400)
}

func TestAudit_TC_AUD_001_to_003(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	w.admin.Get("/api/visits/" + id + "/audit").Expect(200)
	w.empA.Get("/api/visits/" + id + "/audit").Expect(403)
	w.guard.Get("/api/visits/" + id + "/audit").Expect(403)

	// the application's own database role can add to the log but never change or remove from it (FR-9.2)
	appDB, err := sql.Open("pgx", testutil.AppURL())
	require.NoError(t, err)
	defer appDB.Close()
	_, err = appDB.Exec(`INSERT INTO audit_logs (id, actor_role, action, created_at) VALUES (gen_random_uuid(), 'SYSTEM', 'PROBE', now())`)
	assert.NoError(t, err, "INSERT is allowed")
	for name, stmt := range map[string]string{
		"update":   `UPDATE audit_logs SET action = 'TAMPERED'`,
		"delete":   `DELETE FROM audit_logs`,
		"truncate": `TRUNCATE audit_logs`,
	} {
		_, err := appDB.Exec(stmt)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "permission denied", name)
	}
}

func TestDatabase_AppRolePrivileges_TC_DBS_003(t *testing.T) {
	e := testutil.New(t)
	appDB, err := sql.Open("pgx", testutil.AppURL())
	require.NoError(t, err)
	defer appDB.Close()

	_, err = appDB.Exec(`CREATE TABLE should_not_exist (i int)`)
	require.Error(t, err, "the app role cannot change the schema")
	_, err = appDB.Exec(`DROP TABLE visits`)
	require.Error(t, err)
	var n int
	require.NoError(t, appDB.QueryRow(`SELECT count(*) FROM visits`).Scan(&n), "but it can read and write data")

	// a table added by a later migration is usable straight away (default privileges)
	_, err = e.Owner.Exec(`CREATE TABLE priv_probe (i int)`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = e.Owner.Exec(`DROP TABLE IF EXISTS priv_probe`) })
	_, err = appDB.Exec(`INSERT INTO priv_probe VALUES (1)`)
	assert.NoError(t, err)
	require.NoError(t, appDB.QueryRow(`SELECT count(*) FROM priv_probe`).Scan(&n))
	assert.Equal(t, 1, n)
}

func schemaSnapshot(t *testing.T, d *sql.DB) []string {
	t.Helper()
	rows, err := d.Query(`SELECT table_name || '.' || column_name || ':' || data_type || ':' || is_nullable || ':' || coalesce(column_default,'')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name <> 'goose_db_version' ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	return out
}

// TC-DBS-002: every migration can be rolled back, and re-applying gives the same schema.
func TestDatabase_MigrationsAreReversible_TC_DBS_002(t *testing.T) {
	e := testutil.New(t) // makes sure the schema is migrated
	before := schemaSnapshot(t, e.Owner)
	require.NotEmpty(t, before)
	e.App.Close()

	require.NoError(t, db.Migrate(t.Context(), testutil.OwnerURL(), "reset", io.Discard))
	assert.Empty(t, schemaSnapshot(t, e.Owner), "everything is gone after rolling back to version 0")
	require.NoError(t, db.Migrate(t.Context(), testutil.OwnerURL(), "up", io.Discard))
	assert.Equal(t, before, schemaSnapshot(t, e.Owner))
}

// TC-DBS-005: callbacks registered in a transaction run only after it commits.
func TestDatabase_TransactionHelper_TC_DBS_005(t *testing.T) {
	e := testutil.New(t)
	insert := func(tx *db.Tx, day string) error {
		return tx.Exec(`INSERT INTO visit_code_counters (date, last_value) VALUES (?, 1)`, day).Error
	}
	exists := func(day string) bool {
		var n int
		require.NoError(t, e.Owner.QueryRow(`SELECT count(*) FROM visit_code_counters WHERE date = $1`, day).Scan(&n))
		return n == 1
	}

	ran := 0
	require.NoError(t, db.WithTx(t.Context(), e.App.DB, func(tx *db.Tx) error {
		tx.AfterCommit(func() { ran++ })
		return insert(tx, "2030-01-01")
	}))
	assert.Equal(t, 1, ran)
	assert.True(t, exists("2030-01-01"))

	ran = 0
	err := db.WithTx(t.Context(), e.App.DB, func(tx *db.Tx) error {
		tx.AfterCommit(func() { ran++ })
		if err := insert(tx, "2030-01-02"); err != nil {
			return err
		}
		return gorm.ErrInvalidData
	})
	assert.Error(t, err)
	assert.Zero(t, ran, "no callback after a rollback")
	assert.False(t, exists("2030-01-02"))

	ran = 0
	assert.Panics(t, func() {
		_ = db.WithTx(t.Context(), e.App.DB, func(tx *db.Tx) error {
			tx.AfterCommit(func() { ran++ })
			_ = insert(tx, "2030-01-03")
			panic("boom")
		})
	})
	assert.Zero(t, ran)
	assert.False(t, exists("2030-01-03"), "a panic rolls the transaction back")
}

func TestDatabase_GormErrorMapping_TC_DBS_004(t *testing.T) {
	w := newWorld(t)
	_, err := w.App.Users.Get(t.Context(), mustUUID("00000000-0000-0000-0000-000000000001"))
	assert.ErrorContains(t, err, "NOT_FOUND")
	w.admin.Post("/api/users", newUser("EMP400", "dup@gatepass.local")).Expect(201)
	assert.Equal(t, "DUPLICATE", w.admin.Post("/api/users", newUser("EMP401", "dup@gatepass.local")).Expect(409).Code())
}

// ---- security checks (TC-SEC) ----

func TestSecurity_Headers_TC_SEC_001(t *testing.T) {
	e := testutil.New(t)
	h := e.Anon().Get("/api/health").Expect(200).Header
	assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
	assert.Contains(t, h.Get("Content-Security-Policy"), "img-src 'self' blob:")
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
	assert.Equal(t, "no-store", h.Get("Cache-Control"))
	assert.NotEmpty(t, h.Get("X-Request-ID"))
}

func TestSecurity_IDORSweep_TC_SEC_003(t *testing.T) {
	w := newWorld(t)
	sched := w.planned(w.empA, nil)
	pending := w.walkIn("empA", map[string]any{"visitorMobile": "9800000001"})
	inside := w.checkedIn(map[string]any{"visitorMobile": "9800000002"})
	type call struct{ method, path string }
	var calls []call
	for _, id := range []string{sched, pending, inside} {
		calls = append(calls, call{"GET", "/api/visits/" + id}, call{"PATCH", "/api/visits/" + id},
			call{"POST", "/api/visits/" + id + "/approve"}, call{"POST", "/api/visits/" + id + "/reject"},
			call{"POST", "/api/visits/" + id + "/cancel"}, call{"POST", "/api/visits/" + id + "/meeting-over"},
			call{"POST", "/api/visits/" + id + "/check-in"}, call{"POST", "/api/visits/" + id + "/check-out"},
			call{"POST", "/api/visits/" + id + "/deny"}, call{"POST", "/api/visits/" + id + "/force-close"},
			call{"GET", "/api/visits/" + id + "/photos/VISITOR"}, call{"GET", "/api/visits/" + id + "/photos/ID_DOCUMENT"},
			call{"GET", "/api/visits/" + id + "/audit"}, call{"PUT", "/api/visits/" + id + "/photos/VISITOR"})
	}
	for _, c := range calls {
		var body any
		if c.method != "GET" {
			body = map[string]any{"reason": "an attempt by another employee"}
		}
		r := w.empB.Do(c.method, c.path, body)
		assert.Equal(t, 403, r.Status, "empB %s %s must be forbidden, got %d: %s", c.method, c.path, r.Status, r.Raw)
	}
}

func TestSecurity_InjectionAndXSS_TC_SEC_004_005(t *testing.T) {
	w := newWorld(t)
	xss := `<img src=x onerror=alert(1)>`
	id := w.planned(w.empA, map[string]any{"purpose": xss})
	r := w.empA.Get("/api/visits/" + id).Expect(200)
	assert.Equal(t, xss, r.Str("purpose"), "stored as plain text")
	assert.NotContains(t, string(r.Raw), "<img", "and JSON-escaped on the way out")

	for _, q := range []string{"%27+OR+1%3D1+--", "%27%3B+DROP+TABLE+visits%3B+--", "%25", "%5C"} {
		r := w.admin.Get("/api/visits?q=" + q)
		assert.Equal(t, 200, r.Status, q)
		assert.EqualValues(t, 0, r.Path("total"), q)
	}
	w.admin.Get("/api/visits?hostId=1%27+OR+%271%27%3D%271").Expect(400)
	w.admin.Get("/api/users?q=%27+OR+1%3D1+--").Expect(200)
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM visits").Scan(&n))
	assert.Equal(t, 1, n, "the visits table is intact")
}

func TestSecurity_TestClockOnlyInTestMode_TC_SEC_008(t *testing.T) {
	prod := testutil.New(t, testutil.WithAppEnv("development"))
	c := prod.Login("admin")
	assert.Equal(t, 404, c.Post("/api/test/clock", map[string]any{"now": "2030-01-01T00:00:00Z"}).Status, "must not exist outside test mode")

	e := testutil.New(t)
	c = e.Login("admin")
	r := c.Post("/api/test/clock", map[string]any{"now": "2026-11-03T18:35:00Z"}).Expect(200)
	assert.Equal(t, "2026-11-04", r.Str("today"), "00:05 IST is already the next day")
}

func TestStaticApp_TC_SYS(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<html>gatepass</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	e := testutil.New(t, testutil.WithStatic(static))
	c := e.Anon()
	assert.Contains(t, string(c.Get("/some/client/route").Expect(200).Raw), "gatepass", "unknown paths fall back to the app")
	assert.Contains(t, string(c.Get("/").Expect(200).Raw), "gatepass")
	js := c.Get("/assets/app.js").Expect(200)
	assert.Contains(t, js.Header.Get("Cache-Control"), "immutable")
	assert.Equal(t, "NOT_FOUND", c.Get("/api/missing").Expect(404).Code(), "API misses stay JSON")
}
