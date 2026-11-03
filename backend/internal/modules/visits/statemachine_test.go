package visits

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
)

var (
	now    = time.Date(2026, 11, 3, 4, 30, 0, 0, time.UTC) // 10:00 IST
	today  = clock.Date("2026-11-03")
	hostID = uuid.New()
	host   = Actor{ID: hostID, Role: authctx.RoleEmployee}
	other  = Actor{ID: uuid.New(), Role: authctx.RoleEmployee}
	guard  = Actor{ID: uuid.New(), Role: authctx.RoleSecurity}
	admin  = Actor{ID: uuid.New(), Role: authctx.RoleAdmin}
	sys    = SystemActor()
)

func facts(photos bool) Facts {
	return Facts{Now: now, Today: today, HasVisitorPhoto: photos, HasIDPhoto: photos}
}

func visit(st Status) *Visit {
	d := now.Add(30 * time.Minute)
	return &Visit{ID: uuid.New(), Status: st, HostID: hostID, VisitDate: today, ApprovalDeadline: &d, ExpectedDurationMin: 60}
}

func actorsByName() map[string]Actor {
	return map[string]Actor{"host": host, "other": other, "guard": guard, "admin": admin, "system": sys}
}

// TC-SM-001: every row of the transition table works for the right actor.
func TestAllowedTransitions_TC_SM_001(t *testing.T) {
	cases := []struct {
		from   Status
		action Action
		actor  Actor
		to     Status
		facts  Facts
	}{
		{StatusPendingApproval, ActionApprove, host, StatusApproved, facts(false)},
		{StatusPendingApproval, ActionReject, host, StatusRejected, facts(false)},
		{StatusPendingApproval, ActionExpire, sys, StatusExpired, Facts{Now: now.Add(31 * time.Minute), Today: today}},
		{StatusPendingApproval, ActionCancel, guard, StatusCancelled, facts(false)},
		{StatusScheduled, ActionCancel, host, StatusCancelled, facts(false)},
		{StatusScheduled, ActionNoShow, sys, StatusNoShow, Facts{Now: now, Today: today.AddDays(1)}},
		{StatusApproved, ActionNoShow, sys, StatusNoShow, Facts{Now: now, Today: today.AddDays(1)}},
		{StatusScheduled, ActionCheckIn, guard, StatusCheckedIn, facts(true)},
		{StatusApproved, ActionCheckIn, guard, StatusCheckedIn, facts(true)},
		{StatusScheduled, ActionDeny, guard, StatusDenied, facts(false)},
		{StatusApproved, ActionDeny, guard, StatusDenied, facts(false)},
		{StatusCheckedIn, ActionMeetingOver, host, StatusMeetingOver, facts(true)},
		{StatusCheckedIn, ActionMeetingOver, Actor{ID: hostID, Role: authctx.RoleAdmin}, StatusMeetingOver, facts(true)}, // admin hosting
		{StatusMeetingOver, ActionCheckOut, guard, StatusCheckedOut, facts(true)},
		{StatusCheckedIn, ActionForceClose, admin, StatusCheckedOut, facts(true)},
		{StatusMeetingOver, ActionForceClose, admin, StatusCheckedOut, facts(true)},
	}
	for _, c := range cases {
		t.Run(string(c.from)+"/"+string(c.action), func(t *testing.T) {
			to, err := Check(visit(c.from), c.action, c.actor, c.facts)
			require.NoError(t, err)
			assert.Equal(t, c.to, to)
		})
	}
}

// TC-SM-002: everything else is rejected, never silently allowed.
func TestAllOtherTransitionsRejected_TC_SM_002(t *testing.T) {
	allowed := map[[2]string]bool{}
	for _, r := range rules {
		allowed[[2]string{string(r.from), string(r.action)}] = true
	}
	actions := []Action{ActionApprove, ActionReject, ActionExpire, ActionCancel, ActionNoShow, ActionUploadPhotos, ActionCheckIn,
		ActionDeny, ActionMeetingOver, ActionCheckOut, ActionForceClose}
	for _, st := range allStatuses {
		for _, a := range actions {
			if allowed[[2]string{string(st), string(a)}] {
				continue
			}
			for name, actor := range actorsByName() {
				_, err := Check(visit(st), a, actor, Facts{Now: now.Add(time.Hour), Today: today.AddDays(1), HasVisitorPhoto: true, HasIDPhoto: true})
				require.Error(t, err, "%s %s by %s must be rejected", st, a, name)
			}
		}
	}
}

// TC-SM-003: final statuses offer no actions to anybody.
func TestFinalStatusesLocked_TC_SM_003(t *testing.T) {
	for _, st := range finalStatuses {
		for name, actor := range actorsByName() {
			assert.Empty(t, AllowedActions(visit(st), actor, facts(true)), "%s as %s", st, name)
		}
	}
}

// TC-SM-004 / 005: the approval window boundary.
func TestApprovalBoundary_TC_SM_004(t *testing.T) {
	v := visit(StatusPendingApproval)
	deadline := *v.ApprovalDeadline
	_, err := Check(v, ActionApprove, host, Facts{Now: deadline.Add(-time.Second), Today: today})
	assert.NoError(t, err)
	for _, a := range []Action{ActionApprove, ActionReject} {
		_, err = Check(v, a, host, Facts{Now: deadline, Today: today})
		assert.ErrorIs(t, err, apperr.ErrApprovalWindowClosed)
		_, err = Check(v, a, host, Facts{Now: deadline.Add(time.Second), Today: today})
		assert.ErrorIs(t, err, apperr.ErrApprovalWindowClosed)
	}
	// the system may only expire once the deadline has passed
	_, err = Check(v, ActionExpire, sys, Facts{Now: deadline.Add(-time.Second), Today: today})
	assert.ErrorIs(t, err, apperr.ErrInvalidState)
	_, err = Check(v, ActionExpire, sys, Facts{Now: deadline, Today: today})
	assert.NoError(t, err)
}

// TC-SM-006 / 011 / 010: only the right person may act.
func TestWhoMayAct(t *testing.T) {
	_, err := Check(visit(StatusPendingApproval), ActionApprove, other, facts(false))
	assert.ErrorIs(t, err, apperr.ErrForbidden, "another employee cannot approve")
	_, err = Check(visit(StatusPendingApproval), ActionApprove, guard, facts(false))
	assert.ErrorIs(t, err, apperr.ErrForbidden, "security cannot approve")
	for _, a := range []Actor{other, guard} {
		_, err = Check(visit(StatusCheckedIn), ActionMeetingOver, a, facts(true))
		assert.ErrorIs(t, err, apperr.ErrForbidden, "only the host marks Meeting Over")
	}
	_, err = Check(visit(StatusCheckedIn), ActionForceClose, guard, facts(true))
	assert.ErrorIs(t, err, apperr.ErrForbidden, "guard cannot force-close")
	_, err = Check(visit(StatusCheckedIn), ActionForceClose, host, facts(true))
	assert.ErrorIs(t, err, apperr.ErrForbidden, "host cannot force-close")
	_, err = Check(visit(StatusScheduled), ActionForceClose, admin, facts(true))
	assert.ErrorIs(t, err, apperr.ErrInvalidState, "admin cannot force-close a visit that has not started")
	// TC-WI-015: the host cannot cancel a pending walk-in; security cannot cancel a planned visit
	_, err = Check(visit(StatusPendingApproval), ActionCancel, host, facts(false))
	assert.ErrorIs(t, err, apperr.ErrForbidden)
	_, err = Check(visit(StatusScheduled), ActionCancel, guard, facts(false))
	assert.ErrorIs(t, err, apperr.ErrForbidden)
}

// TC-SM-007 / 008: check-in guards (BR-1, BR-5).
func TestCheckInGuards_TC_SM_007(t *testing.T) {
	v := visit(StatusScheduled)
	v.VisitDate = today.AddDays(1)
	_, err := Check(v, ActionCheckIn, guard, facts(true))
	assert.ErrorIs(t, err, apperr.ErrWrongVisitDate)

	for name, f := range map[string]Facts{
		"no photos":    {Now: now, Today: today},
		"visitor only": {Now: now, Today: today, HasVisitorPhoto: true},
		"id only":      {Now: now, Today: today, HasIDPhoto: true},
	} {
		_, err := Check(visit(StatusScheduled), ActionCheckIn, guard, f)
		assert.ErrorIs(t, err, apperr.ErrPhotosRequired, name)
	}
}

// TC-SM-009: no check-out before Meeting Over (BR-4).
func TestCheckOutNeedsMeetingOver_TC_SM_009(t *testing.T) {
	_, err := Check(visit(StatusCheckedIn), ActionCheckOut, guard, facts(true))
	assert.ErrorIs(t, err, apperr.ErrInvalidState)
}

func TestPhotoUploadRules(t *testing.T) {
	_, err := Check(visit(StatusScheduled), ActionUploadPhotos, guard, facts(false))
	assert.NoError(t, err)
	v := visit(StatusScheduled)
	v.VisitDate = today.AddDays(1)
	_, err = Check(v, ActionUploadPhotos, guard, facts(false))
	assert.ErrorIs(t, err, apperr.ErrInvalidState, "TC-PH-008: tomorrow's visit")
	for _, st := range []Status{StatusCheckedIn, StatusPendingApproval} {
		_, err = Check(visit(st), ActionUploadPhotos, guard, facts(false))
		assert.ErrorIs(t, err, apperr.ErrInvalidState, string(st))
	}
}

// TC-SM-012 … 014: what the screens may offer.
func TestAllowedActions_TC_SM_012(t *testing.T) {
	pending := visit(StatusPendingApproval)
	assert.Equal(t, []Action{ActionApprove, ActionReject}, AllowedActions(pending, host, facts(false)))
	assert.Equal(t, []Action{ActionCancel}, AllowedActions(pending, guard, facts(false)))
	assert.Empty(t, AllowedActions(pending, other, facts(false)))
	late := Facts{Now: now.Add(time.Hour), Today: today}
	assert.Empty(t, AllowedActions(pending, host, late), "no actions after the window")

	sched := visit(StatusScheduled)
	assert.Equal(t, []Action{ActionUploadPhotos, ActionDeny}, AllowedActions(sched, guard, facts(false)))
	assert.Equal(t, []Action{ActionUploadPhotos, ActionCheckIn, ActionDeny}, AllowedActions(sched, guard, facts(true)))
	assert.Equal(t, []Action{ActionEdit, ActionCancel}, AllowedActions(sched, host, facts(false)))

	assert.Equal(t, []Action{ActionCheckOut}, AllowedActions(visit(StatusMeetingOver), guard, facts(true)))
	assert.Empty(t, AllowedActions(visit(StatusCheckedIn), guard, facts(true)))
	assert.Equal(t, []Action{ActionForceClose}, AllowedActions(visit(StatusCheckedIn), admin, facts(true)))
	adminHost := Actor{ID: hostID, Role: authctx.RoleAdmin}
	assert.Equal(t, []Action{ActionMeetingOver, ActionForceClose}, AllowedActions(visit(StatusCheckedIn), adminHost, facts(true)))
}

func TestOverstay(t *testing.T) {
	in := now.Add(-61 * time.Minute)
	v := &Visit{Status: StatusCheckedIn, CheckedInAt: &in, ExpectedDurationMin: 60}
	assert.True(t, v.IsOverstay(now))
	v.Status = StatusMeetingOver
	assert.False(t, v.IsOverstay(now), "a visit marked Meeting Over is not an overstay")
	in2 := now.Add(-59 * time.Minute)
	v2 := &Visit{Status: StatusCheckedIn, CheckedInAt: &in2, ExpectedDurationMin: 60}
	assert.False(t, v2.IsOverstay(now))
}
