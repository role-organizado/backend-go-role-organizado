package outbound

import "time"

// AuditAction enumerates structured audit-log actions on an outbound request.
type AuditAction string

const (
	AuditActionCreated  AuditAction = "CREATED"
	AuditActionApproved AuditAction = "APPROVED"
	AuditActionRejected AuditAction = "REJECTED"
	AuditActionCancelled AuditAction = "CANCELLED"
	AuditActionVoted    AuditAction = "VOTED"
	AuditActionExecuted AuditAction = "EXECUTED"
	AuditActionCompleted AuditAction = "COMPLETED"
	AuditActionFailed   AuditAction = "FAILED"
	AuditActionExpired  AuditAction = "EXPIRED"
)

// AuditLog is a single audit event attached to an outbound request.
type AuditLog struct {
	ID         string
	RequestID  string
	EventID    string
	ActorID    string
	Action     AuditAction
	Details    string
	OccurredAt time.Time
}
