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

func TestPlanned_Create_TC_PV_001_002(t *testing.T) {
	w := newWorld(t)
	r := w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"expectedArrival": "09:30"})).Expect(201)
	assert.Equal(t, "SCHEDULED", r.Str("status"))
	assert.Equal(t, "PLANNED", r.Str("type"))
	assert.Equal(t, "GP-20261103-0001", r.Str("code"))
	assert.Equal(t, "Anita Desai", r.Str("host.name"))
	assert.Equal(t, "09:30", r.Str("expectedArrival"))
	assert.Equal(t, "+919876543210", r.Str("visitor.mobile"))
	assert.ElementsMatch(t, []string{"EDIT", "CANCEL"}, r.Strings("allowedActions"))
	assert.Equal(t, []string{"VISIT_CREATED"}, w.auditActions(r.Str("id")))

	// the host cannot be chosen by the client (FR-2.2)
	r2 := w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"hostId": w.UserID("empB")})).Expect(201)
	assert.Equal(t, "Anita Desai", r2.Str("host.name"))
	// an admin can host too
	w.admin.Post("/api/visits/planned", testutil.PlannedBody(nil)).Expect(201)
	// security cannot create planned visits
	w.guard.Post("/api/visits/planned", testutil.PlannedBody(nil)).Expect(403)
}

func TestPlanned_Validation_TC_PV_003_004(t *testing.T) {
	w := newWorld(t)
	r := w.empA.Post("/api/visits/planned", map[string]any{}).Expect(400)
	for _, f := range []string{"visitorName", "visitorMobile", "purpose", "visitDate", "expectedDurationMin"} {
		assert.NotEmpty(t, r.Path("error.details.fields."+f), f)
	}
	for _, d := range []string{"2026-11-02", "2026-12-04"} { // yesterday, today + 31
		r := w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"visitDate": d})).Expect(400)
		assert.NotEmpty(t, r.Path("error.details.fields.visitDate"), d)
	}
	w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"visitDate": "2026-12-03"})).Expect(201) // today + 30
	w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"expectedDurationMin": 50})).Expect(400)
	w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"visitorMobile": "12345"})).Expect(400)
}

func TestPlanned_EditAndCancel_TC_PV_006_to_009(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)

	r := w.empA.Patch("/api/visits/"+id, map[string]any{"visitDate": "2026-11-05", "expectedDurationMin": 90}).Expect(200)
	assert.Equal(t, "2026-11-05", r.Str("visitDate"))
	assert.EqualValues(t, 90, r.Path("expectedDurationMin"))
	assert.EqualValues(t, 2, r.Path("version"))
	assert.Equal(t, []string{"VISIT_CREATED", "VISIT_UPDATED"}, w.auditActions(id))

	w.empA.Patch("/api/visits/"+id, map[string]any{"purpose": "x"}).Expect(400)
	w.empB.Patch("/api/visits/"+id, map[string]any{"purpose": "Hijack attempt"}).Expect(403)

	w.empA.Post("/api/visits/"+id+"/cancel", nil).Expect(200)
	assert.Equal(t, "CANCELLED", w.status(w.empA, id))
	w.empA.Post("/api/visits/"+id+"/cancel", nil).Expect(409)
	w.empA.Patch("/api/visits/"+id, map[string]any{"purpose": "Too late to edit"}).Expect(409)

	// a cancelled visit can never be checked in
	w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", testutil.JPEG(t, 200, 200), nil).Expect(409)
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(409)

	// the host cannot edit a visit that is already in progress
	in := w.checkedIn(map[string]any{"visitorMobile": "9123456789"})
	w.empA.Patch("/api/visits/"+in, map[string]any{"purpose": "Changed my mind"}).Expect(409)
}

func TestVisitCodes_TC_PV_010(t *testing.T) {
	w := newWorld(t)
	for i, want := range []string{"GP-20261103-0001", "GP-20261103-0002", "GP-20261103-0003"} {
		r := w.empA.Post("/api/visits/planned", testutil.PlannedBody(nil)).Expect(201)
		assert.Equal(t, want, r.Str("code"), "visit %d", i)
	}
	r := w.empA.Post("/api/visits/planned", testutil.PlannedBody(map[string]any{"visitDate": "2026-11-04"})).Expect(201)
	assert.Equal(t, "GP-20261104-0001", r.Str("code"), "the counter is per date")

	// 20 creates at once: all codes unique
	var mu sync.Mutex
	codes := map[string]bool{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, raw, err := w.empA.Raw("POST", "/api/visits/planned", testutil.PlannedBody(nil))
			assert.NoError(t, err)
			assert.Equal(t, 201, status, string(raw))
			mu.Lock()
			defer mu.Unlock()
			codes[string(raw[:0])+extractCode(raw)] = true
		}()
	}
	wg.Wait()
	assert.Len(t, codes, 20)
}

func extractCode(raw []byte) string {
	s := string(raw)
	i := indexOf(s, `"code":"`)
	if i < 0 {
		return ""
	}
	return s[i+8 : i+8+16]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestVisitorIsReused_TC_PV_011_012(t *testing.T) {
	w := newWorld(t)
	first := w.planned(w.empA, nil)
	second := w.planned(w.empA, map[string]any{"visitorName": "Ravi K.", "visitorEmail": "ravi@acme.com"})

	assert.Equal(t, "Ravi Kumar", w.empA.Get("/api/visits/"+first).Str("visitor.name"), "an earlier visit keeps the name it was booked with")
	assert.Equal(t, "Ravi K.", w.empA.Get("/api/visits/"+second).Str("visitor.name"))
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM visitors").Scan(&n))
	assert.Equal(t, 1, n, "one visitor record per mobile number")

	l := w.empA.Get("/api/visitors/lookup?mobile=9876543210").Expect(200)
	assert.Equal(t, "Ravi K.", l.Str("name"))
	assert.Equal(t, "ravi@acme.com", l.Str("email"))
	assert.NotContains(t, string(l.Raw), "mobile")
	w.empA.Get("/api/visitors/lookup?mobile=9000000000").Expect(404)
	w.empA.Get("/api/visitors/lookup?mobile=123").Expect(400)
}

func TestList_ScopingFiltersAndPaging_TC_PV_014_015(t *testing.T) {
	w := newWorld(t)
	w.planned(w.empA, nil)
	w.planned(w.empA, map[string]any{"visitorName": "Meera Shah", "visitorMobile": "9000011111"})
	w.planned(w.empB, map[string]any{"visitorName": "Karan Joshi", "visitorMobile": "9000022222"})

	assert.EqualValues(t, 2, w.empA.Get("/api/visits").Expect(200).Path("total"))
	assert.EqualValues(t, 1, w.empB.Get("/api/visits").Expect(200).Path("total"))
	assert.EqualValues(t, 3, w.guard.Get("/api/visits").Expect(200).Path("total"))
	assert.EqualValues(t, 3, w.admin.Get("/api/visits").Expect(200).Path("total"))

	// an admin who hosts a visit sees it under mine=true
	w.planned(w.admin, map[string]any{"visitorName": "Nina Rao", "visitorMobile": "9000033333"})
	assert.EqualValues(t, 1, w.admin.Get("/api/visits?mine=true").Expect(200).Path("total"))
	assert.EqualValues(t, 4, w.admin.Get("/api/visits").Expect(200).Path("total"))

	// filters: status, search by name / mobile digits / code, host, type, date range
	assert.EqualValues(t, 4, w.guard.Get("/api/visits?status=SCHEDULED&status=CHECKED_IN").Expect(200).Path("total"))
	assert.EqualValues(t, 0, w.guard.Get("/api/visits?status=CHECKED_IN").Expect(200).Path("total"))
	assert.EqualValues(t, 1, w.guard.Get("/api/visits?q=meera").Expect(200).Path("total"))
	assert.EqualValues(t, 1, w.guard.Get("/api/visits?q=90000+22222").Expect(200).Path("total"))
	assert.EqualValues(t, 1, w.guard.Get("/api/visits?q=9000022").Expect(200).Path("total"))
	code := w.guard.Get("/api/visits?q=karan").Expect(200).Str("items.0.code")
	assert.EqualValues(t, 1, w.guard.Get("/api/visits?q="+code).Expect(200).Path("total"))
	assert.EqualValues(t, 1, w.admin.Get("/api/visits?hostId="+w.UserID("empB")).Expect(200).Path("total"))
	assert.EqualValues(t, 4, w.admin.Get("/api/visits?type=PLANNED&from=2026-11-03&to=2026-11-03").Expect(200).Path("total"))
	assert.EqualValues(t, 0, w.admin.Get("/api/visits?from=2026-11-04").Expect(200).Path("total"))
	w.admin.Get("/api/visits?status=BOGUS").Expect(400)
	w.admin.Get("/api/visits?from=yesterday").Expect(400)

	// paging
	p := w.admin.Get("/api/visits?page=2&pageSize=3").Expect(200)
	assert.EqualValues(t, 4, p.Path("total"))
	assert.Equal(t, 1, p.Len("items"))
	w.admin.Get("/api/visits?pageSize=101").Expect(400)
	w.admin.Get("/api/visits?page=0").Expect(400)
}

func TestDetailAccess_TC_PV_016_017(t *testing.T) {
	w := newWorld(t)
	id := w.checkedIn(nil)

	w.empB.Get("/api/visits/" + id).Expect(403)
	w.Anon().Get("/api/visits/" + id).Expect(401)
	w.empA.Get("/api/visits/00000000-0000-0000-0000-000000000000").Expect(404)
	w.empA.Get("/api/visits/not-a-uuid").Expect(404)

	r := w.empA.Get("/api/visits/" + id).Expect(200)
	assert.Equal(t, true, r.Path("hasVisitorPhoto"))
	assert.Equal(t, true, r.Path("hasIdPhoto"))
	assert.Equal(t, "AADHAAR", r.Str("idType"))
	assert.Equal(t, []string{"MEETING_OVER"}, r.Strings("allowedActions"))
	assert.Equal(t, 4, r.Len("timeline"), "created, 2 photos, checked in => timeline from the audit log")
	assert.NotContains(t, string(r.Raw), "storage", "no storage paths leak (TC-PH-013)")
	assert.NotContains(t, string(r.Raw), ".jpg")
	assert.Equal(t, "VISIT_CREATED", r.Str("timeline.0.action"))
	assert.Equal(t, "CHECKED_IN", r.Str("timeline.3.action"))
}

func TestLazyExpiryOnRead_TC_PV_018(t *testing.T) {
	w := newWorld(t)
	id := w.walkIn("empA", nil)
	assert.Equal(t, "PENDING_APPROVAL", w.status(w.guard, id))
	w.Clock.Advance(31 * time.Minute) // the expiry job has not run
	r := w.empA.Get("/api/visits/" + id).Expect(200)
	assert.Equal(t, "EXPIRED", r.Str("status"))
	assert.Empty(t, r.Strings("allowedActions"))
	acts := w.auditActions(id)
	assert.Equal(t, "EXPIRED", acts[len(acts)-1])
	var role string
	require.NoError(t, w.Owner.QueryRow("SELECT actor_role FROM audit_logs WHERE visit_id = $1 AND action = 'EXPIRED'", id).Scan(&role))
	assert.Equal(t, "SYSTEM", role)
}

func TestPlanned_EditVisitorDetails_TC_PV_006(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	r := w.empA.Patch("/api/visits/"+id, map[string]any{"visitorName": "Ravi Kumar Jr.", "visitorEmail": "ravi@acme.com",
		"visitorCompany": "Acme Ltd", "expectedArrival": "11:15", "purpose": "Contract review"}).Expect(200)
	assert.Equal(t, "Ravi Kumar Jr.", r.Str("visitor.name"))
	assert.Equal(t, "ravi@acme.com", r.Str("visitor.email"))
	assert.Equal(t, "Acme Ltd", r.Str("visitor.company"))
	assert.Equal(t, "11:15", r.Str("expectedArrival"))
	assert.Equal(t, "Contract review", r.Str("purpose"))

	// a new mobile number moves the visit to that visitor record
	r = w.empA.Patch("/api/visits/"+id, map[string]any{"visitorMobile": "9000099999"}).Expect(200)
	assert.Equal(t, "+919000099999", r.Str("visitor.mobile"))
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM visitors").Scan(&n))
	assert.Equal(t, 2, n)
	// clearing the arrival time
	assert.Equal(t, "", w.empA.Patch("/api/visits/"+id, map[string]any{"expectedArrival": ""}).Expect(200).Str("expectedArrival"))
	// invalid values are reported per field
	bad := w.empA.Patch("/api/visits/"+id, map[string]any{"visitorName": "R2", "visitorEmail": "nope", "expectedArrival": "25:00"}).Expect(400)
	for _, f := range []string{"visitorName", "visitorEmail", "expectedArrival"} {
		assert.NotEmpty(t, bad.Path("error.details.fields."+f), f)
	}
	// an empty patch changes nothing
	v := w.empA.Get("/api/visits/" + id).Path("version")
	assert.Equal(t, v, w.empA.Patch("/api/visits/"+id, map[string]any{}).Expect(200).Path("version"))
}
