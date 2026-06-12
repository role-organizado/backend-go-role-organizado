package event_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	usecase "github.com/role-organizado/backend-go-role-organizado/internal/usecase/event"
)

func TestFindStuckExecutions_ReportsOldestFirstAndCaps(t *testing.T) {
	now := time.Now()
	txs := []*paymentdomain.PaymentTransaction{
		{ID: "tx-new", EventID: "e1", Status: paymentdomain.TransactionStatusPending, CreatedAt: now.Add(-40 * time.Minute)},
		{ID: "tx-oldest", EventID: "e2", Status: paymentdomain.TransactionStatusProcessing, CreatedAt: now.Add(-90 * time.Minute)},
		{ID: "tx-mid", EventID: "e3", Status: paymentdomain.TransactionStatusPending, CreatedAt: now.Add(-60 * time.Minute)},
	}

	repo := &mockPaymentTxRepo{}
	repo.On("FindPendingOlderThan", mock.Anything, mock.Anything).Return(txs, nil)

	uc := usecase.NewFindStuckExecutions(repo)
	res, err := uc.Execute(context.Background(), portin.FindStuckExecutionsInput{
		StuckThresholdMinutes: 30,
		MaxResults:            2, // caps to the 2 oldest
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 2, res.StuckCount)
	require.Len(t, res.Executions, 2)
	assert.Equal(t, "tx-oldest", res.Executions[0].TransactionID)
	assert.Equal(t, "tx-mid", res.Executions[1].TransactionID)
	assert.GreaterOrEqual(t, res.Executions[0].AgeMinutes, res.Executions[1].AgeMinutes)
	repo.AssertExpectations(t)
}

func TestFindStuckExecutions_NoneStuck(t *testing.T) {
	repo := &mockPaymentTxRepo{}
	repo.On("FindPendingOlderThan", mock.Anything, mock.Anything).Return([]*paymentdomain.PaymentTransaction{}, nil)

	uc := usecase.NewFindStuckExecutions(repo)
	res, err := uc.Execute(context.Background(), portin.FindStuckExecutionsInput{})

	require.NoError(t, err)
	assert.Equal(t, 0, res.StuckCount)
	assert.Empty(t, res.Executions)
}

func TestFindStuckExecutions_PropagatesRepoError(t *testing.T) {
	repo := &mockPaymentTxRepo{}
	repo.On("FindPendingOlderThan", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	uc := usecase.NewFindStuckExecutions(repo)
	_, err := uc.Execute(context.Background(), portin.FindStuckExecutionsInput{})
	require.Error(t, err)
}
