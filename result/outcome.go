package result

// Outcome is the model-facing discriminator for a tool result.
type Outcome string

const (
	// OutcomeSucceeded indicates the tool produced a valid result.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomeFailed indicates the tool ran or validated unsuccessfully.
	OutcomeFailed Outcome = "failed"
	// OutcomeTimedOut is reserved for caller or orchestration timeouts.
	OutcomeTimedOut Outcome = "timed_out"
	// OutcomeRejected indicates the caller rejected the tool call before
	// execution.
	OutcomeRejected Outcome = "rejected"
)

// Valid reports whether o is one of the supported outcome values.
func (o Outcome) Valid() bool {
	switch o {
	case OutcomeSucceeded, OutcomeFailed, OutcomeTimedOut, OutcomeRejected:
		return true
	default:
		return false
	}
}

// String returns the JSON/string representation of o.
func (o Outcome) String() string {
	return string(o)
}
