package activity

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/testsuite"
)

// stubPspReviewUC implements in.RunPspCostReviewUseCase for tests.
type stubPspReviewUC struct {
	calls         int
	lastReference string
	err           error
}

func (s *stubPspReviewUC) Execute(_ context.Context, ref string) error {
	s.calls++
	s.lastReference = ref
	return s.err
}

func TestPricingPspReviewActivity_Run(t *testing.T) {
	tests := []struct {
		name           string
		reference      string
		ucErr          error
		wantErr        bool
		wantCalls      int
		wantRefPassed  string
	}{
		{
			name:          "happy path forwards reference date to UC",
			reference:     "2026-06-01",
			ucErr:         nil,
			wantErr:       false,
			wantCalls:     1,
			wantRefPassed: "2026-06-01",
		},
		{
			name:          "empty reference still forwarded as-is",
			reference:     "",
			ucErr:         nil,
			wantErr:       false,
			wantCalls:     1,
			wantRefPassed: "",
		},
		{
			name:          "use case error bubbles up",
			reference:     "2026-06-02",
			ucErr:         errors.New("boom"),
			wantErr:       true,
			wantCalls:     1,
			wantRefPassed: "2026-06-02",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := &testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			uc := &stubPspReviewUC{err: tc.ucErr}
			act := NewPricingPspReviewActivity(uc)
			env.RegisterActivity(act.RunPspCostReview)

			_, err := env.ExecuteActivity(act.RunPspCostReview, tc.reference)
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
