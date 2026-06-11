package activity

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/testsuite"
)

// stubFinanceReconciler implements financeReconciler for unit tests.
type stubFinanceReconciler struct {
	calls         int
	lastReference string
	err           error
}

func (s *stubFinanceReconciler) Execute(_ context.Context, ref string) error {
	s.calls++
	s.lastReference = ref
	return s.err
}

func TestFinanceReconciliationActivities_RunReconciliation(t *testing.T) {
	tests := []struct {
		name          string
		reference     string
		ucErr         error
		wantErr       bool
		wantCalls     int
		wantRefPassed string
	}{
		{
			name:          "single pass succeeds and forwards reference",
			reference:     "2026-06-01",
			ucErr:         nil,
			wantErr:       false,
			wantCalls:     1,
			wantRefPassed: "2026-06-01",
		},
		{
			name:          "use case error is wrapped and surfaced",
			reference:     "2026-06-02",
			ucErr:         errors.New("ledger unreachable"),
			wantErr:       true,
			wantCalls:     1,
			wantRefPassed: "2026-06-02",
		},
		{
			name:          "empty reference still forwarded",
			reference:     "",
			ucErr:         nil,
			wantErr:       false,
			wantCalls:     1,
			wantRefPassed: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := &testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			uc := &stubFinanceReconciler{err: tc.ucErr}
			acts := NewFinanceReconciliationActivities(uc)
			env.RegisterActivity(acts.RunReconciliation)

			_, err := env.ExecuteActivity(acts.RunReconciliation, tc.reference)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if uc.calls != tc.wantCalls {
				t.Errorf("calls=%d want=%d", uc.calls, tc.wantCalls)
			}
			if uc.lastReference != tc.wantRefPassed {
				t.Errorf("lastReference=%q want=%q", uc.lastReference, tc.wantRefPassed)
			}
		})
	}
}

// Triple-check pattern test: the same activity instance can be invoked multiple times
// independently (the workflow drives the cardinality, not the activity).
func TestFinanceReconciliationActivities_TriplePassInvariant(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	uc := &stubFinanceReconciler{}
	acts := NewFinanceReconciliationActivities(uc)
	env.RegisterActivity(acts.RunReconciliation)

	for i := 0; i < 3; i++ {
		_, err := env.ExecuteActivity(acts.RunReconciliation, "2026-06-01")
		if err != nil {
			t.Fatalf("pass %d unexpected err: %v", i+1, err)
		}
	}
	if uc.calls != 3 {
		t.Errorf("calls=%d want=3", uc.calls)
	}
}
