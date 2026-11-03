package visits

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventNone              EventType = "" // status changed, nobody needs a notification
	EventApprovalRequested EventType = "APPROVAL_REQUESTED"
	EventApprovalReminder  EventType = "APPROVAL_REMINDER"
	EventApprovalDecided   EventType = "APPROVAL_DECIDED"
	EventVisitorCheckedIn  EventType = "VISITOR_CHECKED_IN"
	EventVisitorDenied     EventType = "VISITOR_DENIED"
	EventOverstay          EventType = "OVERSTAY"
	EventMeetingOver       EventType = "MEETING_OVER"
	EventVisitorCheckedOut EventType = "VISITOR_CHECKED_OUT"
)

// Event is published after a visit change commits. The notifications module decides
// who is told, and how (declared here so visits does not import notifications).
type Event struct {
	Type        EventType
	VisitID     uuid.UUID
	Code        string
	Status      Status
	Version     int
	HostID      uuid.UUID
	HostName    string
	HostEmail   string
	VisitorName string
	Company     string
	WindowMin   int
	Reason      string
	At          time.Time
}

type EventSink interface {
	HandleVisitEvent(ctx context.Context, e Event)
}

// PhotoSummary tells which photos a visit has (implemented by the photos module).
type PhotoSummary struct {
	HasVisitor bool
	HasID      bool
	IDType     string
}

type PhotoChecker interface {
	Summaries(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]PhotoSummary, error)
}
