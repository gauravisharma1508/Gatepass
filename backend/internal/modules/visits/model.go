// Package visits owns planned visits, walk-ins and every status change (spec §6).
package visits

import (
	"time"

	"github.com/google/uuid"

	"gatepass/internal/platform/clock"
)

type Status string

const (
	StatusScheduled       Status = "SCHEDULED"
	StatusPendingApproval Status = "PENDING_APPROVAL"
	StatusApproved        Status = "APPROVED"
	StatusCheckedIn       Status = "CHECKED_IN"
	StatusMeetingOver     Status = "MEETING_OVER"
	StatusCheckedOut      Status = "CHECKED_OUT"
	StatusRejected        Status = "REJECTED"
	StatusExpired         Status = "EXPIRED"
	StatusCancelled       Status = "CANCELLED"
	StatusNoShow          Status = "NO_SHOW"
	StatusDenied          Status = "DENIED"
)

var allStatuses = []Status{StatusScheduled, StatusPendingApproval, StatusApproved, StatusCheckedIn, StatusMeetingOver,
	StatusCheckedOut, StatusRejected, StatusExpired, StatusCancelled, StatusNoShow, StatusDenied}

var finalStatuses = []Status{StatusCheckedOut, StatusRejected, StatusExpired, StatusCancelled, StatusNoShow, StatusDenied}

func (s Status) Valid() bool {
	for _, x := range allStatuses {
		if x == s {
			return true
		}
	}
	return false
}

func (s Status) Final() bool {
	for _, x := range finalStatuses {
		if x == s {
			return true
		}
	}
	return false
}

type Type string

const (
	TypePlanned   Type = "PLANNED"
	TypeUnplanned Type = "UNPLANNED"
)

type Visit struct {
	ID                  uuid.UUID `gorm:"primaryKey"`
	Code                string
	Type                Type
	Status              Status
	VisitorID           uuid.UUID
	VisitorName         string
	HostID              uuid.UUID
	CreatedByID         uuid.UUID
	Purpose             string
	VisitDate           clock.Date
	ExpectedArrival     *clock.TimeOfDay
	ExpectedDurationMin int
	ApprovalDeadline    *time.Time
	ReminderSentAt      *time.Time
	DecidedAt           *time.Time
	DecisionReason      *string
	CheckedInAt         *time.Time
	CheckedInByID       *uuid.UUID
	MeetingOverAt       *time.Time
	CheckedOutAt        *time.Time
	CheckedOutByID      *uuid.UUID
	OverstayNotifiedAt  *time.Time
	ClosedReason        *string
	PhotosPurgedAt      *time.Time
	Version             int
	CreatedAt           time.Time
	UpdatedAt           time.Time

	// cross-module read: visitors, users (display fields only, spec §3.3 rule M-4)
	Visitor *VisitorRef `gorm:"foreignKey:VisitorID;references:ID"`
	Host    *HostRef    `gorm:"foreignKey:HostID;references:ID"`
}

func (Visit) TableName() string { return "visits" }

// IsOverstay: still CHECKED_IN after the expected duration (a visit marked Meeting Over is not an overstay).
func (v *Visit) IsOverstay(now time.Time) bool {
	return v.Status == StatusCheckedIn && v.CheckedInAt != nil &&
		now.After(v.CheckedInAt.Add(time.Duration(v.ExpectedDurationMin)*time.Minute))
}

// VisitorRef is a read-only view of a visitors row.
type VisitorRef struct {
	ID      uuid.UUID `gorm:"primaryKey"`
	Mobile  string
	Name    string
	Email   *string
	Company *string
}

func (VisitorRef) TableName() string { return "visitors" }

// HostRef is a read-only view of a users row.
type HostRef struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	Name       string
	Email      string
	Department *string
}

func (HostRef) TableName() string { return "users" }
