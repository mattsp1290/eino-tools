package beads

import (
	"context"
	"errors"
	"testing"
)

type fakeClient struct {
	id     string
	reason string
	err    error
	calls  int
}

func (f *fakeClient) Close(_ context.Context, id, reason string) error {
	f.calls++
	f.id = id
	f.reason = reason
	return f.err
}

func TestNewRejectsNilClient(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) returned nil error")
	}
}

func TestCloseForwardsToClient(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	adapter, err := New(client)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := adapter.Close(context.Background(), "issue-1", "done"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if client.calls != 1 || client.id != "issue-1" || client.reason != "done" {
		t.Fatalf("client call = calls:%d id:%q reason:%q", client.calls, client.id, client.reason)
	}
}

func TestClosePropagatesClientError(t *testing.T) {
	t.Parallel()

	want := errors.New("close failed")
	adapter, err := New(&fakeClient{err: want})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := adapter.Close(context.Background(), "issue-1", "done"); !errors.Is(got, want) {
		t.Fatalf("Close error = %v, want %v", got, want)
	}
}

func TestCloseRejectsNilReceiver(t *testing.T) {
	t.Parallel()

	var adapter *Adapter
	if err := adapter.Close(context.Background(), "issue-1", "done"); err == nil {
		t.Fatal("nil adapter Close returned nil error")
	}
}
