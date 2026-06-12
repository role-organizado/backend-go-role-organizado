package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// faseAlterer advances/rolls back an event's operational fase. Satisfied by
// event.AlterarFase (AlterarFaseUseCase).
type faseAlterer interface {
	Execute(ctx context.Context, in portin.AlterarFaseInput) (*portin.AlterarFaseResult, error)
}

// EventLifecycleActivityInput is the input for AlterarFaseEvento.
type EventLifecycleActivityInput struct {
	EventoID string `json:"eventoId"`
	NovaFase string `json:"novaFase"`
	UserID   string `json:"userId"`
}

// EventLifecycleActivityResult is the outcome of a fase transition.
type EventLifecycleActivityResult struct {
	FaseAnterior string `json:"faseAnterior"`
	FaseAtual    string `json:"faseAtual"`
	Mensagem     string `json:"mensagem"`
}

// EventLifecycleActivities holds dependencies for event lifecycle activities.
type EventLifecycleActivities struct {
	alterer faseAlterer
}

// NewEventLifecycleActivities creates a new EventLifecycleActivities instance.
func NewEventLifecycleActivities(alterer faseAlterer) *EventLifecycleActivities {
	return &EventLifecycleActivities{alterer: alterer}
}

// AlterarFaseEvento transitions an event to a new operational fase via the
// native AlterarFaseUseCase. The use case enforces the legal transition graph.
func (a *EventLifecycleActivities) AlterarFaseEvento(ctx context.Context, input EventLifecycleActivityInput) (EventLifecycleActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("altering event fase (native)",
		"eventoId", input.EventoID,
		"novaFase", input.NovaFase,
	)

	res, err := a.alterer.Execute(ctx, portin.AlterarFaseInput{
		EventoID:    input.EventoID,
		RequesterID: input.UserID,
		FaseDestino: input.NovaFase,
	})
	if err != nil {
		return EventLifecycleActivityResult{}, fmt.Errorf("alterar fase evento %s -> %s: %w", input.EventoID, input.NovaFase, err)
	}

	return EventLifecycleActivityResult{
		FaseAnterior: res.FaseAnterior,
		FaseAtual:    res.FaseAtual,
		Mensagem:     res.Mensagem,
	}, nil
}
