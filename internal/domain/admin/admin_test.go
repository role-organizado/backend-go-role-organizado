package admin

import "testing"

func TestCancellationTier_Valid(t *testing.T) {
	cases := []struct {
		name string
		tier CancellationTier
		want bool
	}{
		{"valid days", CancellationTier{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 7, RefundPercent: 100, Label: "ok"}, true},
		{"valid phase", CancellationTier{TriggerType: "PHASE_AT_OR_AFTER", Threshold: 0, RefundPercent: 0, Label: "ok"}, true},
		{"bad trigger", CancellationTier{TriggerType: "WAT", Threshold: 1, RefundPercent: 50, Label: "x"}, false},
		{"negative threshold", CancellationTier{TriggerType: "DAYS_BEFORE_EVENT", Threshold: -1, RefundPercent: 50, Label: "x"}, false},
		{"refund over 100", CancellationTier{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 1, RefundPercent: 101, Label: "x"}, false},
		{"refund negative", CancellationTier{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 1, RefundPercent: -1, Label: "x"}, false},
		{"blank label", CancellationTier{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 1, RefundPercent: 50, Label: "  "}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tier.Valid(); got != tc.want {
				t.Fatalf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}
