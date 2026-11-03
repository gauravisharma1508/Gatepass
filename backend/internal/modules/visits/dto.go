package visits

import (
	"time"

	"github.com/google/uuid"

	"gatepass/internal/platform/clock"
)

type VisitorDTO struct {
	Name    string  `json:"name"`
	Mobile  string  `json:"mobile"`
	Email   *string `json:"email"`
	Company *string `json:"company"`
}

type HostDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Department *string   `json:"department"`
}

type TimelineItem struct {
	Action     string    `json:"action"`
	ActorRole  string    `json:"actorRole"`
	FromStatus *string   `json:"fromStatus"`
	ToStatus   *string   `json:"toStatus"`
	Reason     *string   `json:"reason"`
	CreatedAt  time.Time `json:"createdAt"`
}

// VisitDTO is the response shape of spec §8.3.
type VisitDTO struct {
	ID                  uuid.UUID        `json:"id"`
	Code                string           `json:"code"`
	Type                Type             `json:"type"`
	Status              Status           `json:"status"`
	Visitor             VisitorDTO       `json:"visitor"`
	Host                HostDTO          `json:"host"`
	Purpose             string           `json:"purpose"`
	VisitDate           clock.Date       `json:"visitDate"`
	ExpectedArrival     *clock.TimeOfDay `json:"expectedArrival"`
	ExpectedDurationMin int              `json:"expectedDurationMin"`
	ApprovalDeadline    *time.Time       `json:"approvalDeadline"`
	ApprovalSecondsLeft *int             `json:"approvalSecondsLeft"`
	DecidedAt           *time.Time       `json:"decidedAt"`
	DecisionReason      *string          `json:"decisionReason"`
	CheckedInAt         *time.Time       `json:"checkedInAt"`
	MeetingOverAt       *time.Time       `json:"meetingOverAt"`
	CheckedOutAt        *time.Time       `json:"checkedOutAt"`
	ClosedReason        *string          `json:"closedReason"`
	IsOverstay          bool             `json:"isOverstay"`
	HasVisitorPhoto     bool             `json:"hasVisitorPhoto"`
	HasIDPhoto          bool             `json:"hasIdPhoto"`
	IDType              *string          `json:"idType"`
	AllowedActions      []Action         `json:"allowedActions"`
	Version             int              `json:"version"`
	CreatedAt           time.Time        `json:"createdAt"`
	Timeline            []TimelineItem   `json:"timeline,omitempty"`
}

// SecurityDashboard / EmployeeDashboard are the dashboard responses (spec §8.2).
type SecurityDashboard struct {
	ExpectedToday    []VisitDTO     `json:"expectedToday"`
	PendingApprovals []VisitDTO     `json:"pendingApprovals"`
	OnPremises       []VisitDTO     `json:"onPremises"`
	ReadyForCheckout []VisitDTO     `json:"readyForCheckout"`
	Overstays        []VisitDTO     `json:"overstays"`
	Counts           map[string]int `json:"counts"`
}

type EmployeeDashboard struct {
	PendingApprovals    []VisitDTO `json:"pendingApprovals"`
	TodaysVisits        []VisitDTO `json:"todaysVisits"`
	Upcoming            []VisitDTO `json:"upcoming"`
	AwaitingMeetingOver []VisitDTO `json:"awaitingMeetingOver"`
}
