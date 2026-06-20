package tracker

import "context"

// CloseWriter is the v0.1 tracker mutation surface required by tracker_write.
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}

// TransitionWriter is an optional tracker mutation surface for writers that can
// close issues and move them to a target state.
type TransitionWriter interface {
	CloseWriter
	Transition(ctx context.Context, id, toState string) error
}
