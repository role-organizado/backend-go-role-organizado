package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// EventLifecycleInput is the input for EventLifecycleWorkflow.
type EventLifecycleInput struct {
	EventoID string `json:"eventoId"`
	UserID   string `json:"userId"`
	// TransitionsSoFar carries the cumulative transition count across
	// continueAsNew boundaries.
	TransitionsSoFar int `json:"transitionsSoFar"`
}

// EventLifecycleState is the observable state of an event lifecycle run.
type EventLifecycleState struct {
	// Status is "RUNNING" or "FINALIZED".
	Status          string `json:"status"`
	EventoID        string `json:"eventoId"`
	CurrentFase     string `json:"currentFase"`
	TransitionCount int    `json:"transitionCount"`
}

// eventLifecycleMaxTransitions caps the number of fase transitions handled in a
// single workflow run before continueAsNew is triggered, keeping the event
// history bounded for this long-running workflow.
const eventLifecycleMaxTransitions = 100

// EventLifecycleWorkflow is a long-running workflow that tracks an event through
// its operational phases. It blocks on Workflow.Await for incoming control
// signals, applies the corresponding fase transition via the AlterarFaseEvento
// activity, and continues-as-new after 100 transitions to bound history growth.
//
// Signals:  releasePayments, pausePayments, advanceToPreparacao
// Queries:  getCurrentState, getWorkflowStatus
func EventLifecycleWorkflow(ctx workflow.Context, input EventLifecycleInput) error {
	state := EventLifecycleState{
		Status:          "RUNNING",
		EventoID:        input.EventoID,
		TransitionCount: input.TransitionsSoFar,
	}

	_ = workflow.SetQueryHandler(ctx, "getCurrentState", func() (EventLifecycleState, error) {
		return state, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	// Signal → target fase mapping.
	releaseCh := workflow.GetSignalChannel(ctx, "releasePayments")
	pauseCh := workflow.GetSignalChannel(ctx, "pausePayments")
	advanceCh := workflow.GetSignalChannel(ctx, "advanceToPreparacao")

	// A dedicated goroutine drains the control signals into an ordered queue so the
	// main coroutine can block on Workflow.Await — mirroring the Java
	// Workflow.await(() -> !queue.isEmpty()) pattern.
	var queue []string
	workflow.Go(ctx, func(gctx workflow.Context) {
		for {
			sel := workflow.NewSelector(gctx)
			sel.AddReceive(releaseCh, func(c workflow.ReceiveChannel, _ bool) {
				c.Receive(gctx, nil)
				queue = append(queue, "COLETA_PAGAMENTOS")
			})
			sel.AddReceive(pauseCh, func(c workflow.ReceiveChannel, _ bool) {
				c.Receive(gctx, nil)
				queue = append(queue, "ORGANIZACAO")
			})
			sel.AddReceive(advanceCh, func(c workflow.ReceiveChannel, _ bool) {
				c.Receive(gctx, nil)
				queue = append(queue, "PREPARACAO")
			})
			sel.Select(gctx)
		}
	})

	transitionsThisRun := 0
	for transitionsThisRun < eventLifecycleMaxTransitions {
		// Block until the signal-draining goroutine enqueues a target fase.
		_ = workflow.Await(ctx, func() bool { return len(queue) > 0 })

		target := queue[0]
		queue = queue[1:]

		var res activity.EventLifecycleActivityResult
		err := workflow.ExecuteActivity(actCtx, "AlterarFaseEvento", activity.EventLifecycleActivityInput{
			EventoID: input.EventoID,
			NovaFase: target,
			UserID:   input.UserID,
		}).Get(ctx, &res)

		state.TransitionCount++
		transitionsThisRun++

		if err != nil {
			// A rejected transition (e.g. illegal fase change) is non-fatal for the
			// long-running lifecycle — record nothing and keep awaiting signals.
			continue
		}

		state.CurrentFase = res.FaseAtual
		if res.FaseAtual == "FINALIZADO" {
			state.Status = "FINALIZED"
			return nil
		}
	}

	// History bound reached — continue as new, carrying the cumulative count.
	return workflow.NewContinueAsNewError(ctx, EventLifecycleWorkflow, EventLifecycleInput{
		EventoID:         input.EventoID,
		UserID:           input.UserID,
		TransitionsSoFar: state.TransitionCount,
	})
}
