package result

import "testing"

func TestOutcomeValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Outcome
		want bool
	}{
		{name: "succeeded", in: OutcomeSucceeded, want: true},
		{name: "failed", in: OutcomeFailed, want: true},
		{name: "timed out", in: OutcomeTimedOut, want: true},
		{name: "rejected", in: OutcomeRejected, want: true},
		{name: "zero value", in: Outcome(""), want: false},
		{name: "unknown", in: Outcome("unknown"), want: false},
		{name: "case sensitive", in: Outcome("SUCCEEDED"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.Valid(); got != tt.want {
				t.Fatalf("Valid() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestOutcomeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Outcome
		want string
	}{
		{name: "succeeded", in: OutcomeSucceeded, want: "succeeded"},
		{name: "failed", in: OutcomeFailed, want: "failed"},
		{name: "timed out", in: OutcomeTimedOut, want: "timed_out"},
		{name: "rejected", in: OutcomeRejected, want: "rejected"},
		{name: "zero value", in: Outcome(""), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutcomeConstantsStayStable(t *testing.T) {
	t.Parallel()

	got := []Outcome{
		OutcomeSucceeded,
		OutcomeFailed,
		OutcomeTimedOut,
		OutcomeRejected,
	}
	want := []Outcome{
		"succeeded",
		"failed",
		"timed_out",
		"rejected",
	}

	if len(got) != len(want) {
		t.Fatalf("len(constants) = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("constant %d = %q, want %q", i, got[i], want[i])
		}
	}
}
