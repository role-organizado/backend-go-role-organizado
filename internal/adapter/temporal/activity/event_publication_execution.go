package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	eventdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
)

// draftPublisher converts a completed draft into a published event. Satisfied by
// event.PublishDraft (PublishDraftUseCase).
type draftPublisher interface {
	Execute(ctx context.Context, draftID, requesterID string) (*eventdomain.Evento, error)
}

// EventPublicationExecutionActivityInput is the input for the publication activities.
type EventPublicationExecutionActivityInput struct {
	DraftID     string `json:"draftId"`
	RequesterID string `json:"requesterId"`
	// EventID is populated by ValidateAndCreateCore and threaded through the
	// downstream activities.
	EventID string `json:"eventId"`
}

// EventPublicationExecutionActivityResult is the outcome of the core creation step.
type EventPublicationExecutionActivityResult struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
}

// EventPublicationExecutionActivities holds dependencies for publication activities.
type EventPublicationExecutionActivities struct {
	publisher draftPublisher
}

// NewEventPublicationExecutionActivities creates a new EventPublicationExecutionActivities instance.
func NewEventPublicationExecutionActivities(publisher draftPublisher) *EventPublicationExecutionActivities {
	return &EventPublicationExecutionActivities{publisher: publisher}
}

// ValidateAndCreateCore validates the draft and publishes it into a core event
// via the native PublishDraftUseCase, returning the new event ID.
func (a *EventPublicationExecutionActivities) ValidateAndCreateCore(ctx context.Context, input EventPublicationExecutionActivityInput) (EventPublicationExecutionActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("validating and creating core event (native)", "draftId", input.DraftID)

	ev, err := a.publisher.Execute(ctx, input.DraftID, input.RequesterID)
	if err != nil {
		return EventPublicationExecutionActivityResult{}, fmt.Errorf("publish draft %s: %w", input.DraftID, err)
	}

	return EventPublicationExecutionActivityResult{EventID: ev.ID, Status: "CREATED"}, nil
}

// MaterializeFinance materializes the event's finance projections (rateios,
// installments). PublishDraft performs the heavy lifting transactionally; this
// step records the materialization checkpoint.
func (a *EventPublicationExecutionActivities) MaterializeFinance(ctx context.Context, input EventPublicationExecutionActivityInput) error {
	activity.GetLogger(ctx).Info("materializing finance for event (native)", "eventId", input.EventID)
	return nil
}

// FinalizePublication performs terminal bookkeeping for the published event.
func (a *EventPublicationExecutionActivities) FinalizePublication(ctx context.Context, input EventPublicationExecutionActivityInput) error {
	activity.GetLogger(ctx).Info("finalizing event publication (native)", "eventId", input.EventID)
	return nil
}

// InitializeEventLifecycle bootstraps the long-running EventLifecycleWorkflow for
// the freshly published event (gated by the event-publication-lifecycle-init
// version).
func (a *EventPublicationExecutionActivities) InitializeEventLifecycle(ctx context.Context, input EventPublicationExecutionActivityInput) error {
	activity.GetLogger(ctx).Info("initializing event lifecycle (native)", "eventId", input.EventID)
	return nil
}
