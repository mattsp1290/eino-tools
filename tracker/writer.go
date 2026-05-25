package tracker

import "context"

// CloseWriter is the v0.1 tracker mutation surface required by tracker_write.
type CloseWriter interface {
	Close(ctx context.Context, id, reason string) error
}
