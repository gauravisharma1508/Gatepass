package visits

import (
	"time"

	"github.com/google/uuid"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
)

type Action string

const (
	ActionApprove      Action = "APPROVE"
	ActionReject       Action = "REJECT"
	ActionExpire       Action = "EXPIRE"
	ActionCancel       Action = "CANCEL"
	ActionNoShow       Action = "NO_SHOW"
	ActionUploadPhotos Action = "UPLOAD_PHOTOS"
	ActionCheckIn      Action = "CHECK_IN"
	ActionDeny         Action = "DENY"
	ActionMeetingOver  Action = "MEETING_OVER"
	ActionCheckOut     Action = "CHECK_OUT"
	ActionForceClose   Action = "FORCE_CLOSE"
	ActionEdit         Action = "EDIT" // not a status change; listed in allowedActions for the UI
)

// Actor is who performs an action: a signed-in user, or the system (background jobs).
type Actor struct {
	ID     uuid.UUID
	Role   authctx.Role
	System bool
}

func SystemActor() Actor { return Actor{System: true} }

func ActorOf(p authctx.Principal) Actor { return Actor{ID: p.ID, Role: p.Role} }

type who int

const (
	whoHost who = iota
	whoSecurity
	whoAdmin
	whoSystem
)

func (a Actor) allowed(w who, v *Visit) bool {
	switch w {
	case whoSystem:
		return a.System
	case whoSecurity:
		return !a.System && a.Role == authctx.RoleSecurity
	case whoAdmin:
		return !a.System && a.Role == authctx.RoleAdmin
	case whoHost:
		return !a.System && (a.Role == authctx.RoleEmployee || a.Role == authctx.RoleAdmin) && a.ID == v.HostID
	}
	return false
}

type rule struct {
	from   Status
	action Action
	to     Status
	who    who
}

// rules is the transition table of spec §6.2. UPLOAD_PHOTOS keeps the status.
var rules = []rule{
	{StatusPendingApproval, ActionApprove, StatusApproved, whoHost},
	{StatusPendingApproval, ActionReject, StatusRejected, whoHost},
	{StatusPendingApproval, ActionExpire, StatusExpired, whoSystem},
	{StatusPendingApproval, ActionCancel, StatusCancelled, whoSecurity},
	{StatusScheduled, ActionCancel, StatusCancelled, whoHost},
	{StatusScheduled, ActionNoShow, StatusNoShow, whoSystem},
	{StatusApproved, ActionNoShow, StatusNoShow, whoSystem},
	{StatusScheduled, ActionUploadPhotos, StatusScheduled, whoSecurity},
	{StatusApproved, ActionUploadPhotos, StatusApproved, whoSecurity},
	{StatusScheduled, ActionCheckIn, StatusCheckedIn, whoSecurity},
	{StatusApproved, ActionCheckIn, StatusCheckedIn, whoSecurity},
	{StatusScheduled, ActionDeny, StatusDenied, whoSecurity},
	{StatusApproved, ActionDeny, StatusDenied, whoSecurity},
	{StatusCheckedIn, ActionMeetingOver, StatusMeetingOver, whoHost},
	{StatusMeetingOver, ActionCheckOut, StatusCheckedOut, whoSecurity},
	{StatusCheckedIn, ActionForceClose, StatusCheckedOut, whoAdmin},
	{StatusMeetingOver, ActionForceClose, StatusCheckedOut, whoAdmin},
}

// Facts are the time and photo facts the guards need.
type Facts struct {
	Now             time.Time
	Today           clock.Date
	HasVisitorPhoto bool
	HasIDPhoto      bool
}

// Check decides whether actor may perform action on v and returns the resulting status.
// Errors: ErrForbidden (wrong role/not the host) → ErrInvalidState (wrong status) → guard errors.
func Check(v *Visit, action Action, actor Actor, f Facts) (Status, error) {
	var atStatus *rule
	anyAllowed := false
	for i := range rules {
		r := &rules[i]
		if r.action != action {
			continue
		}
		if r.from == v.Status {
			atStatus = r
		}
		if actor.allowed(r.who, v) {
			anyAllowed = true
		}
	}
	if atStatus == nil {
		if anyAllowed {
			return "", apperr.ErrInvalidState
		}
		return "", apperr.ErrForbidden
	}
	if !actor.allowed(atStatus.who, v) {
		return "", apperr.ErrForbidden
	}

	switch action {
	case ActionApprove, ActionReject:
		if v.ApprovalDeadline == nil || !f.Now.Before(*v.ApprovalDeadline) {
			return "", apperr.ErrApprovalWindowClosed
		}
	case ActionExpire:
		if v.ApprovalDeadline != nil && f.Now.Before(*v.ApprovalDeadline) {
			return "", apperr.ErrInvalidState
		}
	case ActionNoShow:
		if !(v.VisitDate < f.Today) {
			return "", apperr.ErrInvalidState
		}
	case ActionUploadPhotos:
		if v.VisitDate != f.Today {
			return "", apperr.ErrInvalidState.With("Photos can only be captured on the visit date")
		}
	case ActionCheckIn:
		if v.VisitDate != f.Today {
			return "", apperr.ErrWrongVisitDate.With("This visit is scheduled for " + string(v.VisitDate))
		}
		if !f.HasVisitorPhoto || !f.HasIDPhoto {
			return "", apperr.ErrPhotosRequired
		}
	}
	return atStatus.to, nil
}

var listedActions = []Action{ActionApprove, ActionReject, ActionUploadPhotos, ActionCheckIn, ActionDeny,
	ActionMeetingOver, ActionCheckOut, ActionCancel, ActionForceClose}

// AllowedActions is what the UI may offer actor on v (so screens never repeat the rules).
func AllowedActions(v *Visit, actor Actor, f Facts) []Action {
	out := []Action{}
	if v.Status == StatusScheduled && actor.allowed(whoHost, v) {
		out = append(out, ActionEdit)
	}
	for _, a := range listedActions {
		if _, err := Check(v, a, actor, f); err == nil {
			out = append(out, a)
		}
	}
	return out
}
