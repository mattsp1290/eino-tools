package beads

import (
	"context"
	"errors"

	beadssdk "github.com/mattsp1290/beads-go/beads"
)

// Client is the beads-go surface required by this adapter.
type Client interface {
	Close(ctx context.Context, id, reason string) error
}

// Adapter exposes a beads-go client as a tracker.CloseWriter.
type Adapter struct {
	client Client
}

// New constructs an Adapter around a beads-go-compatible client.
func New(client Client) (*Adapter, error) {
	if client == nil {
		return nil, errors.New("tracker/beads: client is required")
	}
	return &Adapter{client: client}, nil
}

// NewClient constructs a beads-go client and wraps it as an Adapter.
func NewClient(opts ...beadssdk.Option) (*Adapter, error) {
	client, err := beadssdk.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return New(client)
}

// Close closes the tracker issue through the wrapped beads-go client.
func (a *Adapter) Close(ctx context.Context, id, reason string) error {
	if a == nil || a.client == nil {
		return errors.New("tracker/beads: client is required")
	}
	return a.client.Close(ctx, id, reason)
}
