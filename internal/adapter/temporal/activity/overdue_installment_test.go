package activity

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/testsuite"
)

// stubFinder satisfies overdueInstallmentFinder for unit tests.
type stubFinder struct {
	calls         int
	lastReference string
	count         int
	err           error
}

func (s *stubFinder) Execute(_ context.Context, ref string) (int, error) {
	s.calls++
	s.lastReference = ref
	return s.count, s.err
}

// stubDispatcher satisfies overdueNotificationDispatcher for unit tests.
type stubDispatcher struct {
	calls         int
	lastReference string
	lastCount     int
	err           error
}

func (s *stubDispatcher) Execute(_ context.Context, ref string, count int) error {
	s.calls++
	s.lastReference = ref
	s.lastCount = count
	return s.err
}

func TestOverdueInstallmentActivities_FindAndMark(t *testing.T) {
	tests := []struct {
		name           string
		reference      string
		ucCount        int
		ucErr          error
		wantErr        bool
		wantCount      int
		wantRefPassed  string
	}{
		{
			name:          "happy path returns count and reference",
			reference:     "2026-06-01",
			ucCount:       7,
			ucErr:         nil,
			wantErr:       false,
			wantCount:     7,
			wantRefPassed: "2026-06-01",
		},
		{
			name:          "zero installments still succeeds",
			reference:     "2026-06-02",
			ucCount:       0,
			ucErr:         nil,
			wantErr:       false,
			wantCount:     0,
			wantRefPassed: "2026-06-02",
		},
		{
			name:          "finder error wrapped",
			reference:     "2026-06-03",
			ucCount:       0,
			ucErr:         errors.New("mongo unreachable"),
			wantErr:       true,
			wantCount:     0,
			wantRefPassed: "2026-06-03",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := &testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			finder := &stubFinder{count: tc.ucCount, err: tc.ucErr}
			dispatcher := &stubDispatcher{}
			acts := NewOverdueInstallmentActivities(finder, dispatcher)
			env.RegisterActivity(acts.FindAndMarkOverdueInstallments)

			val, err := env.ExecuteActivity(acts.FindAndMarkOverdueInstallments, tc.reference)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr {
				var got int
				if dErr := val.Get(&got); dErr != nil {
					t.Fatalf("decode result: %v", dErr)
				}
				if got != tc.wantCount {
					t.Errorf("count=%d want=%d", got, tc.wantCount)
				}
			}
			if finder.lastReference != tc.wantRefPassed {
				t.Errorf("lastReference=%q want=%q", finder.lastReference, tc.wantRefPassed)
			}
			// The finder activity must never trigger the dispatcher.
			if dispatcher.calls != 0 {
				t.Errorf("dispatcher unexpectedly called: %d times", dispatcher.calls)
			}
		})
	}
}

func TestOverdueInstallmentActivities_Dispatch(t *testing.T) {
	tests := []struct {
		name          string
		reference     string
		count         int
		ucErr         error
		wantErr       bool
		wantCallPassed int
	}{
		{
			name:           "dispatch happy path",
			reference:      "2026-06-01",
			count:          5,
			ucErr:          nil,
			wantErr:        false,
			wantCallPassed: 5,
		},
		{
			name:           "dispatcher error wrapped",
			reference:      "2026-06-02",
			count:          3,
			ucErr:          errors.New("notification svc down"),
			wantErr:        true,
			wantCallPassed: 3,
		},
		{
			name:           "zero count still forwarded (workflow already guards)",
			reference:      "2026-06-03",
			count:          0,
			ucErr:          nil,
			wantErr:        false,
			wantCallPassed: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := &testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			finder := &stubFinder{}
			dispatcher := &stubDispatcher{err: tc.ucErr}
			acts := NewOverdueInstallmentActivities(finder, dispatcher)
			env.RegisterActivity(acts.DispatchNotifications)

			_, err := env.ExecuteActivity(acts.DispatchNotifications, tc.reference, tc.count)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if dispatcher.lastReference != tc.reference {
				t.Errorf("lastReference=%q want=%q", dispatcher.lastReference, tc.reference)
			}
			if dispatcher.lastCount != tc.wantCallPassed {
				t.Errorf("lastCount=%d want=%d", dispatcher.lastCount, tc.wantCallPassed)
			}
		})
	}
}
