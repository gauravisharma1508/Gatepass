//go:build integration

package app_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

func TestPhotos_Upload_TC_PH_001_to_009(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)

	// metadata is stripped and the file is small (FR-4.2, NFR-8)
	secret := "GPS-SECRET-LAT-12.97"
	big := testutil.WithEXIF(testutil.JPEG(t, 2000, 1500), secret)
	w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", big, nil).Expect(204)
	keys := w.Store.Keys()
	require.Len(t, keys, 1)
	r := w.guard.Get("/api/visits/" + id + "/photos/VISITOR")
	r.Expect(200)
	assert.Equal(t, "image/jpeg", r.Header.Get("Content-Type"))
	assert.Equal(t, "private, no-store", r.Header.Get("Cache-Control"))
	assert.LessOrEqual(t, len(r.Raw), 500_000)
	assert.False(t, bytes.Contains(r.Raw, []byte(secret)), "EXIF/GPS must not survive")
	assert.Contains(t, w.auditActions(id), "PHOTO_CAPTURED")

	// retake replaces: still one stored file for that kind, and the old one is deleted
	w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", testutil.JPEG(t, 400, 300), nil).Expect(204)
	assert.Len(t, w.Store.Keys(), 1)

	// ID photo needs a valid type; last4 is optional but validated
	id2 := w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", testutil.JPEG(t, 400, 300), nil).Expect(400)
	assert.NotEmpty(t, id2.Path("error.details.fields.idType"))
	w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", testutil.JPEG(t, 400, 300), map[string]string{"idType": "CARD"}).Expect(400)
	w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", testutil.JPEG(t, 400, 300), map[string]string{"idType": "PAN", "idLast4": "12-4"}).Expect(400)
	w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", testutil.JPEG(t, 400, 300), map[string]string{"idType": "PAN", "idLast4": "AB12"}).Expect(204)
	assert.Equal(t, "PAN", w.empA.Get("/api/visits/"+id).Str("idType"))
	w.guard.Upload("/api/visits/"+id+"/photos/SELFIE", testutil.JPEG(t, 100, 100), nil).Expect(404)

	// only security may upload
	w.empA.Upload("/api/visits/"+id+"/photos/VISITOR", testutil.JPEG(t, 100, 100), nil).Expect(403)
	w.admin.Upload("/api/visits/"+id+"/photos/VISITOR", testutil.JPEG(t, 100, 100), nil).Expect(403)
}

func TestPhotos_RejectedContent_TC_PH_003_004(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	up := func(data []byte) *testutil.Resp {
		return w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", data, nil)
	}
	assert.Equal(t, "FILE_TOO_LARGE", up(bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 2_200_000)).Expect(413).Code(), "6.6 MB")
	assert.Equal(t, "UNSUPPORTED_MEDIA_TYPE", up([]byte("plain text named .jpg")).Expect(415).Code())
	assert.Equal(t, "UNSUPPORTED_MEDIA_TYPE", up([]byte("GIF89a\x01\x00\x01\x00")).Expect(415).Code())
	assert.Empty(t, w.Store.Keys(), "nothing is stored for rejected uploads")

	// TC-SEC-006: a PNG claiming 20 000 x 20 000 px is refused before decoding
	chunk := func(kind string, data []byte) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(kind)
		b.Write(data)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 20000)
	binary.BigEndian.PutUint32(ihdr[4:], 20000)
	ihdr[8] = 8
	bomb := append([]byte("\x89PNG\r\n\x1a\n"), chunk("IHDR", ihdr)...)
	bomb = append(bomb, chunk("IDAT", nil)...)
	bomb = append(bomb, chunk("IEND", nil)...)
	up(bomb).Expect(413)
	assert.Empty(t, w.Store.Keys())
}

func TestPhotos_StateGuard_TC_PH_008(t *testing.T) {
	w := newWorld(t)
	img := testutil.JPEG(t, 200, 200)
	up := func(id string) *testutil.Resp { return w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", img, nil) }

	up(w.walkIn("empA", nil)).Expect(409)                                                                       // still PENDING_APPROVAL
	up(w.planned(w.empA, map[string]any{"visitDate": "2026-11-04", "visitorMobile": "9111100000"})).Expect(409) // tomorrow
	up(w.checkedIn(map[string]any{"visitorMobile": "9111100001"})).Expect(409)                                  // already inside
	up(w.planned(w.empA, map[string]any{"visitorMobile": "9111100002"})).Expect(204)                            // today's planned visit
}

func TestPhotos_Viewing_TC_PH_010_to_013(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	w.photos(id)

	w.guard.Get("/api/visits/" + id + "/photos/VISITOR").Expect(200)
	w.guard.Get("/api/visits/" + id + "/photos/ID_DOCUMENT").Expect(200)
	w.admin.Get("/api/visits/" + id + "/photos/ID_DOCUMENT").Expect(200)
	w.empA.Get("/api/visits/" + id + "/photos/VISITOR").Expect(200)                  // the host sees the visitor photo
	assert.Equal(t, 403, w.empA.Get("/api/visits/"+id+"/photos/ID_DOCUMENT").Status) // but never the ID
	assert.Equal(t, 403, w.empB.Get("/api/visits/"+id+"/photos/VISITOR").Status)     // nor does another employee
	w.Anon().Get("/api/visits/" + id + "/photos/VISITOR").Expect(401)
	id2 := w.planned(w.empA, map[string]any{"visitorMobile": "9222200000"})
	w.guard.Get("/api/visits/" + id2 + "/photos/VISITOR").Expect(404) // none uploaded yet
}

func TestCheckIn_Rules_TC_CI_001_to_008(t *testing.T) {
	w := newWorld(t)

	// planned visit with both photos
	id := w.planned(w.empA, nil)
	r := w.guard.Get("/api/visits/" + id)
	assert.NotContains(t, r.Strings("allowedActions"), "CHECK_IN", "not offered before both photos exist")
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(422) // no photos
	w.guard.Upload("/api/visits/"+id+"/photos/VISITOR", testutil.JPEG(t, 200, 200), nil).Expect(204)
	assert.Equal(t, "PHOTOS_REQUIRED", w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(422).Code(), "ID photo still missing")
	w.guard.Upload("/api/visits/"+id+"/photos/ID_DOCUMENT", testutil.JPEG(t, 200, 200), map[string]string{"idType": "PASSPORT"}).Expect(204)

	w.empA.Post("/api/visits/"+id+"/check-in", nil).Expect(403)
	w.admin.Post("/api/visits/"+id+"/check-in", nil).Expect(403)
	assert.Contains(t, w.guard.Get("/api/visits/"+id).Strings("allowedActions"), "CHECK_IN")

	ci := w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(200)
	assert.Equal(t, "CHECKED_IN", ci.Str("status"))
	assert.NotEmpty(t, ci.Str("checkedInAt"))
	assert.Contains(t, w.notifications(w.empA), "VISITOR_CHECKED_IN")
	require.NotEmpty(t, w.Mail.To(testutil.Email("empA")))
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(409) // already in

	// wrong date (BR-5)
	tomorrow := w.planned(w.empA, map[string]any{"visitDate": "2026-11-04", "visitorMobile": "9333300000"})
	assert.Equal(t, "WRONG_VISIT_DATE", w.guard.Post("/api/visits/"+tomorrow+"/check-in", nil).Expect(422).Code())

	// walk-ins only after approval (BR-2)
	pending := w.walkIn("empA", map[string]any{"visitorMobile": "9444400000"})
	w.guard.Post("/api/visits/"+pending+"/check-in", nil).Expect(409)
	rejected := w.walkIn("empA", map[string]any{"visitorMobile": "9444400001"})
	w.empA.Post("/api/visits/"+rejected+"/reject", nil).Expect(200)
	w.guard.Post("/api/visits/"+rejected+"/check-in", nil).Expect(409)
	approved := w.walkIn("empA", map[string]any{"visitorMobile": "9444400002"})
	w.empA.Post("/api/visits/"+approved+"/approve", nil).Expect(200)
	w.photos(approved)
	w.guard.Post("/api/visits/"+approved+"/check-in", nil).Expect(200)

	// closed visits cannot be checked in
	cancelled := w.planned(w.empA, map[string]any{"visitorMobile": "9555500000"})
	w.empA.Post("/api/visits/"+cancelled+"/cancel", nil).Expect(200)
	w.guard.Post("/api/visits/"+cancelled+"/check-in", nil).Expect(409)
}

func TestCheckIn_VisitorAlreadyInside_TC_CI_007(t *testing.T) {
	w := newWorld(t)
	w.checkedIn(nil) // Ravi Kumar is inside
	second := w.planned(w.empB, nil)
	w.photos(second)
	r := w.guard.Post("/api/visits/"+second+"/check-in", nil).Expect(409)
	assert.Equal(t, "VISITOR_ALREADY_ACTIVE", r.Code())
}

func TestExpectedToday_TC_CI_009(t *testing.T) {
	w := newWorld(t)
	todayPlanned := w.planned(w.empA, map[string]any{"visitorName": "Meera Shah", "visitorMobile": "9000011111"})
	approved := w.walkIn("empA", map[string]any{"visitorName": "Karan Joshi", "visitorMobile": "9000022222"})
	w.empA.Post("/api/visits/"+approved+"/approve", nil).Expect(200)
	w.planned(w.empA, map[string]any{"visitDate": "2026-11-04", "visitorMobile": "9000033333"})
	w.checkedIn(map[string]any{"visitorMobile": "9000044444"})

	d := w.guard.Get("/api/dashboard/security").Expect(200)
	assert.Equal(t, 2, d.Len("expectedToday"), "today's SCHEDULED + APPROVED only")
	assert.EqualValues(t, 2, d.Path("counts.expectedToday"))
	assert.EqualValues(t, 1, d.Path("counts.onPremises"))
	for _, q := range []string{"meera", "9000011111", "90000+11111"} {
		assert.EqualValues(t, 1, w.guard.Get("/api/visits?q="+q).Expect(200).Path("total"), q)
	}
	code := w.guard.Get("/api/visits/" + todayPlanned).Str("code")
	assert.EqualValues(t, 1, w.guard.Get("/api/visits?q="+code).Expect(200).Path("total"))
}

func TestDeny_TC_CI_010_011(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	w.guard.Post("/api/visits/"+id+"/deny", map[string]any{"reason": ""}).Expect(400)
	w.guard.Post("/api/visits/"+id+"/deny", map[string]any{"reason": "ok"}).Expect(400)
	w.empA.Post("/api/visits/"+id+"/deny", map[string]any{"reason": "Not allowed to deny"}).Expect(403)
	r := w.guard.Post("/api/visits/"+id+"/deny", map[string]any{"reason": "Name on ID does not match"}).Expect(200)
	assert.Equal(t, "DENIED", r.Str("status"))
	assert.Equal(t, "Name on ID does not match", r.Str("closedReason"))
	assert.Contains(t, w.notifications(w.empA), "VISITOR_DENIED")
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(409)

	in := w.checkedIn(map[string]any{"visitorMobile": "9777700000"})
	w.guard.Post("/api/visits/"+in+"/deny", map[string]any{"reason": "Too late to deny"}).Expect(409)
}

func TestMeetingOver_TC_MO_001_to_004(t *testing.T) {
	w := newWorld(t)
	id := w.checkedIn(nil)
	w.empB.Post("/api/visits/"+id+"/meeting-over", nil).Expect(403)
	w.guard.Post("/api/visits/"+id+"/meeting-over", nil).Expect(403)
	w.admin.Post("/api/visits/"+id+"/meeting-over", nil).Expect(403) // admin is not this visit's host

	r := w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	assert.Equal(t, "MEETING_OVER", r.Str("status"))
	assert.NotEmpty(t, r.Str("meetingOverAt"))
	assert.Contains(t, w.notifications(w.guard), "MEETING_OVER")
	assert.Contains(t, w.notifications(w.guard2), "MEETING_OVER")
	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(409)
	w.empA.Post("/api/visits/"+w.planned(w.empA, map[string]any{"visitorMobile": "9888800000"})+"/meeting-over", nil).Expect(409) // not started

	// an admin who hosts a visit can close it (TC-MO-004)
	adminVisit := w.planned(w.admin, map[string]any{"visitorMobile": "9888800001"})
	w.photos(adminVisit)
	w.guard.Post("/api/visits/"+adminVisit+"/check-in", nil).Expect(200)
	w.admin.Post("/api/visits/"+adminVisit+"/meeting-over", nil).Expect(200)
}

func TestCheckOut_TC_CO_001_to_008(t *testing.T) {
	w := newWorld(t)
	id := w.checkedIn(nil)

	// blocked before Meeting Over, with a clear message (BR-4)
	r := w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(409)
	assert.Equal(t, "INVALID_STATE", r.Code())
	assert.Equal(t, "Waiting for host to mark Meeting Over", r.Str("error.message"))
	assert.Empty(t, w.guard.Get("/api/visits/"+id).Strings("allowedActions"))
	assert.Equal(t, 1, w.guard.Get("/api/visits?onPremises=true").Expect(200).Len("items"))

	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	assert.Equal(t, []string{"CHECK_OUT"}, w.guard.Get("/api/visits/"+id).Strings("allowedActions"))
	w.empA.Post("/api/visits/"+id+"/check-out", nil).Expect(403)
	w.admin.Post("/api/visits/"+id+"/check-out", nil).Expect(403)
	assert.Equal(t, 1, w.guard.Get("/api/dashboard/security").Expect(200).Len("readyForCheckout"))

	co := w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(200)
	assert.Equal(t, "CHECKED_OUT", co.Str("status"))
	assert.NotEmpty(t, co.Str("checkedOutAt"))
	assert.Contains(t, w.notifications(w.empA), "VISITOR_CHECKED_OUT")
	w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(409)
	assert.Equal(t, 0, w.guard.Get("/api/visits?onPremises=true").Expect(200).Len("items"))
}

func TestForceClose_TC_CO_005_to_007(t *testing.T) {
	w := newWorld(t)
	id := w.checkedIn(nil)
	w.admin.Post("/api/visits/"+id+"/force-close", nil).Expect(400)
	w.admin.Post("/api/visits/"+id+"/force-close", map[string]any{"reason": "ok"}).Expect(400)
	w.guard.Post("/api/visits/"+id+"/force-close", map[string]any{"reason": "Guards may not do this"}).Expect(403)
	w.empA.Post("/api/visits/"+id+"/force-close", map[string]any{"reason": "Hosts may not do this"}).Expect(403)

	r := w.admin.Post("/api/visits/"+id+"/force-close", map[string]any{"reason": "Host on leave, visitor left at 5 pm"}).Expect(200)
	assert.Equal(t, "CHECKED_OUT", r.Str("status"))
	acts := w.auditActions(id)
	assert.Equal(t, "FORCE_CLOSED", acts[len(acts)-1])
	var reason string
	require.NoError(t, w.Owner.QueryRow("SELECT reason FROM audit_logs WHERE visit_id = $1 AND action = 'FORCE_CLOSED'", id).Scan(&reason))
	assert.Equal(t, "Host on leave, visitor left at 5 pm", reason)

	notStarted := w.planned(w.empA, map[string]any{"visitorMobile": "9999900000"})
	w.admin.Post("/api/visits/"+notStarted+"/force-close", map[string]any{"reason": "Not started yet"}).Expect(409)
}

// TC-SM-015: two guards check out at the same moment: one wins.
func TestConcurrentCheckOut_TC_SM_015(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 6; i++ {
		id := w.checkedIn(map[string]any{"visitorMobile": "9" + pad(500+i)})
		w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
		var wg sync.WaitGroup
		var a, b int
		wg.Add(2)
		go func() { defer wg.Done(); a, _, _ = w.guard.Raw("POST", "/api/visits/"+id+"/check-out", nil) }()
		go func() { defer wg.Done(); b, _, _ = w.guard2.Raw("POST", "/api/visits/"+id+"/check-out", nil) }()
		wg.Wait()
		assert.ElementsMatch(t, []int{200, 409}, []int{a, b}, "round %d", i)
		assert.Equal(t, 1, w.countAudit(id, "CHECKED_OUT"), "round %d", i)
		assert.EqualValues(t, 4, w.admin.Get("/api/visits/"+id).Path("version"), "version 1 + check-in + meeting over + ONE check-out")
	}
}

func TestFailedTransitionLeavesNoTrace_TC_SM_016(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	before := w.auditActions(id)
	v := w.admin.Get("/api/visits/" + id).Path("version")
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(422) // guard fails: no photos
	assert.Equal(t, before, w.auditActions(id))
	assert.Equal(t, v, w.admin.Get("/api/visits/"+id).Path("version"))
}

// ---- complete flows ----

func TestFlow_PlannedVisit_AC1_TC_FLOW_001(t *testing.T) {
	w := newWorld(t)
	id := w.planned(w.empA, nil)
	w.photos(id)
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(200)
	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(200)

	r := w.admin.Get("/api/visits/" + id).Expect(200)
	assert.Equal(t, "CHECKED_OUT", r.Str("status"))
	for _, f := range []string{"checkedInAt", "meetingOverAt", "checkedOutAt"} {
		assert.NotEmpty(t, r.Str(f), f)
	}
	assert.Equal(t, []string{"VISIT_CREATED", "PHOTO_CAPTURED", "PHOTO_CAPTURED", "CHECKED_IN", "MEETING_OVER", "CHECKED_OUT"}, w.auditActions(id))
	// every audit row has actor, role and a status change where applicable
	a := w.admin.Get("/api/visits/" + id + "/audit").Expect(200)
	assert.Equal(t, "CHECKED_IN", a.Str("items.3.action"))
	assert.Equal(t, "SCHEDULED", a.Str("items.3.fromStatus"))
	assert.Equal(t, "CHECKED_IN", a.Str("items.3.toStatus"))
	assert.Equal(t, "SECURITY", a.Str("items.3.actorRole"))
	assert.NotEmpty(t, a.Str("items.3.actorId"))
}

func TestFlow_WalkIn_AC2_TC_FLOW_002(t *testing.T) {
	w := newWorld(t)
	id := w.walkIn("empA", nil)
	w.empA.Post("/api/visits/"+id+"/approve", nil).Expect(200)
	w.photos(id)
	w.guard.Post("/api/visits/"+id+"/check-in", nil).Expect(200)
	w.empA.Post("/api/visits/"+id+"/meeting-over", nil).Expect(200)
	w.guard.Post("/api/visits/"+id+"/check-out", nil).Expect(200)
	assert.Equal(t, []string{"VISIT_CREATED", "APPROVED", "PHOTO_CAPTURED", "PHOTO_CAPTURED", "CHECKED_IN", "MEETING_OVER", "CHECKED_OUT"}, w.auditActions(id))
}
