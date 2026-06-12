// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	accountingdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// accountingSnapshotGenerator is satisfied by *accounting.GenerateSnapshot.
// Defined as an interface so the activity can be unit-tested with a stub.
type accountingSnapshotGenerator interface {
	Execute(ctx context.Context, in portin.GenerateAccountingSnapshotInput) (*accountingdomain.Snapshot, error)
}

// AccountingSnapshotActivities groups the activities for the accounting snapshot
// workflow. Register the struct with the worker.
type AccountingSnapshotActivities struct {
	useCase accountingSnapshotGenerator
}

// NewAccountingSnapshotActivities constructs AccountingSnapshotActivities.
func NewAccountingSnapshotActivities(uc accountingSnapshotGenerator) *AccountingSnapshotActivities {
	return &AccountingSnapshotActivities{useCase: uc}
}

// AccountingSnapshotInput carries parameters for the aggregation activity.
type AccountingSnapshotInput struct {
	DataInicio    string `json:"dataInicio"`
	DataFim       string `json:"dataFim"`
	CorrelationID string `json:"correlationId"`
}

// AccountingSnapshotResult is the serialisable outcome of the aggregation activity.
type AccountingSnapshotResult struct {
	SnapshotID      string `json:"snapshotId"`
	TotalEventos    int64  `json:"totalEventos"`
	TotalArrecadado int64  `json:"totalArrecadado"`
	TotalRepassado  int64  `json:"totalRepassado"`
	TotalTaxas      int64  `json:"totalTaxas"`
}

// GenerateAccountingSnapshot aggregates platform accounting totals and persists a
// snapshot. Registered as activity "GenerateAccountingSnapshot".
func (a *AccountingSnapshotActivities) GenerateAccountingSnapshot(ctx context.Context, input AccountingSnapshotInput) (*AccountingSnapshotResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("generating accounting snapshot",
		"dataInicio", input.DataInicio,
		"dataFim", input.DataFim,
		"correlationId", input.CorrelationID,
	)

	snapshot, err := a.useCase.Execute(ctx, portin.GenerateAccountingSnapshotInput{
		DataInicio:    input.DataInicio,
		DataFim:       input.DataFim,
		CorrelationID: input.CorrelationID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate accounting snapshot: %w", err)
	}

	logger.Info("accounting snapshot generated",
		"snapshotId", snapshot.ID,
		"totalEventos", snapshot.TotalEventos,
	)
	return &AccountingSnapshotResult{
		SnapshotID:      snapshot.ID,
		TotalEventos:    snapshot.TotalEventos,
		TotalArrecadado: snapshot.TotalArrecadado,
		TotalRepassado:  snapshot.TotalRepassado,
		TotalTaxas:      snapshot.TotalTaxas,
	}, nil
}
