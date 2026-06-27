package trackerwrite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
)

type fakeCloseWriter struct {
	mu     sync.Mutex
	id     string
	reason string
	err    error
	calls  int
}

func (f *fakeCloseWriter) Close(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.id = id
	f.reason = reason
	return f.err
}

type fakeTransitionWriter struct {
	mu              sync.Mutex
	closeCalls      int
	transitionID    string
	transitionState string
	transitionErr   error
	transitionCalls int
}

func (f *fakeTransitionWriter) Close(_ context.Context, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	return nil
}

func (f *fakeTransitionWriter) Transition(_ context.Context, id, toState string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitionCalls++
	f.transitionID = id
	f.transitionState = toState
	return f.transitionErr
}

type fakeCommentWriter struct {
	mu           sync.Mutex
	closeID      string
	closeReason  string
	closeCalls   int
	commentID    string
	commentBody  string
	commentErr   error
	commentCalls int
}

func (f *fakeCommentWriter) Close(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	f.closeID = id
	f.closeReason = reason
	return nil
}

func (f *fakeCommentWriter) Comment(_ context.Context, id, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commentCalls++
	f.commentID = id
	f.commentBody = body
	return f.commentErr
}

type fakeFullWriter struct {
	mu              sync.Mutex
	closeCalls      int
	transitionCalls int
	commentCalls    int
	closeID         string
	closeReason     string
	transitionID    string
	transitionState string
	commentID       string
	commentBody     string
}

func (f *fakeFullWriter) Close(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	f.closeID = id
	f.closeReason = reason
	return nil
}

func (f *fakeFullWriter) Transition(_ context.Context, id, toState string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitionCalls++
	f.transitionID = id
	f.transitionState = toState
	return nil
}

func (f *fakeFullWriter) Comment(_ context.Context, id, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commentCalls++
	f.commentID = id
	f.commentBody = body
	return nil
}

func TestRunCloseSucceeds(t *testing.T) {
	t.Parallel()

	writer := &fakeCloseWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpClose, ID: "T-1", Reason: "done"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded; error=%+v", res.Outcome, res.Error)
	}
	if res.Op != OpClose || res.ID != "T-1" {
		t.Fatalf("res = %+v", res)
	}
	if writer.calls != 1 || writer.id != "T-1" || writer.reason != "done" {
		t.Fatalf("writer = calls:%d id:%q reason:%q", writer.calls, writer.id, writer.reason)
	}
}

func TestRunTransitionSucceedsWhenWriterSupportsIt(t *testing.T) {
	t.Parallel()

	writer := &fakeTransitionWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpTransition, ID: "T-1", ToState: "accepted"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded; error=%+v", res.Outcome, res.Error)
	}
	if res.Op != OpTransition || res.ID != "T-1" {
		t.Fatalf("res = %+v", res)
	}
	if writer.transitionCalls != 1 || writer.transitionID != "T-1" || writer.transitionState != "accepted" {
		t.Fatalf("transition = calls:%d id:%q state:%q", writer.transitionCalls, writer.transitionID, writer.transitionState)
	}
	if writer.closeCalls != 0 {
		t.Fatalf("close called %d times", writer.closeCalls)
	}
}

func TestRunCommentSucceedsWhenWriterSupportsIt(t *testing.T) {
	t.Parallel()

	writer := &fakeCommentWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpComment, ID: "T-1", Body: "verdict: pass"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded; error=%+v", res.Outcome, res.Error)
	}
	if res.Op != OpComment || res.ID != "T-1" {
		t.Fatalf("res = %+v", res)
	}
	if writer.commentCalls != 1 || writer.commentID != "T-1" || writer.commentBody != "verdict: pass" {
		t.Fatalf("comment = calls:%d id:%q body:%q", writer.commentCalls, writer.commentID, writer.commentBody)
	}
	if writer.closeCalls != 0 {
		t.Fatalf("close called %d times", writer.closeCalls)
	}
}

func TestRunCommentPreservesRawBody(t *testing.T) {
	t.Parallel()

	const rawBody = "  verdict: pass\n"
	writer := &fakeCommentWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpComment, ID: "T-1", Body: rawBody})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded; error=%+v", res.Outcome, res.Error)
	}
	if writer.commentBody != rawBody {
		t.Fatalf("commentBody = %q, want raw %q", writer.commentBody, rawBody)
	}
}

func TestCommentWriterStillRoutesClose(t *testing.T) {
	t.Parallel()

	writer := &fakeCommentWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpClose, ID: "T-1", Reason: "done"})
	if res.Outcome != result.OutcomeSucceeded || res.Op != OpClose {
		t.Fatalf("res = %+v", res)
	}
	if writer.closeCalls != 1 || writer.closeID != "T-1" || writer.closeReason != "done" || writer.commentCalls != 0 {
		t.Fatalf("writer called close:%d id:%q reason:%q comment:%d",
			writer.closeCalls, writer.closeID, writer.closeReason, writer.commentCalls)
	}
}

func TestCommentAndTransitionWriterRoutesEachOp(t *testing.T) {
	t.Parallel()

	writer := &fakeFullWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, args := range []Args{
		{Op: OpClose, ID: "T-1", Reason: "done"},
		{Op: OpTransition, ID: "T-2", ToState: "accepted"},
		{Op: OpComment, ID: "T-3", Body: "verdict: pass"},
	} {
		res := tool.Run(context.Background(), args)
		if res.Outcome != result.OutcomeSucceeded {
			t.Fatalf("Run(%+v) = %+v", args, res)
		}
	}

	if writer.closeCalls != 1 || writer.closeID != "T-1" || writer.closeReason != "done" {
		t.Fatalf("close = calls:%d id:%q reason:%q", writer.closeCalls, writer.closeID, writer.closeReason)
	}
	if writer.transitionCalls != 1 || writer.transitionID != "T-2" || writer.transitionState != "accepted" {
		t.Fatalf("transition = calls:%d id:%q state:%q", writer.transitionCalls, writer.transitionID, writer.transitionState)
	}
	if writer.commentCalls != 1 || writer.commentID != "T-3" || writer.commentBody != "verdict: pass" {
		t.Fatalf("comment = calls:%d id:%q body:%q", writer.commentCalls, writer.commentID, writer.commentBody)
	}
}

func TestUnsupportedOpsDoNotCallWriter(t *testing.T) {
	t.Parallel()

	for _, op := range []Op{OpComment, OpTransition, OpLinkPR} {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()

			writer := &fakeCloseWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			// No toState for transition: capability is checked before shape, so a
			// close-only writer reports unsupported_op (terminal), not validation.
			res := tool.Run(context.Background(), Args{Op: op, ID: "T-1"})
			assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
			if writer.calls != 0 {
				t.Fatalf("writer called %d times", writer.calls)
			}
		})
	}
}

func TestCommentAndLinkPRUnsupportedWithTransitionWriter(t *testing.T) {
	t.Parallel()

	for _, op := range []Op{OpComment, OpLinkPR} {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()

			writer := &fakeTransitionWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res := tool.Run(context.Background(), Args{Op: op, ID: "T-1"})
			assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
			if writer.closeCalls != 0 || writer.transitionCalls != 0 {
				t.Fatalf("writer called close:%d transition:%d times", writer.closeCalls, writer.transitionCalls)
			}
		})
	}
}

func TestRunCommentRequiresBodyWhenWriterSupportsIt(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "whitespace", body: " \n\t "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := &fakeCommentWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res := tool.Run(context.Background(), Args{Op: OpComment, ID: "T-1", Body: tc.body})
			assertFailureCategory(t, res, ErrCategoryValidation)
			if res.Error.Message != "body is required and must be non-empty for op comment" {
				t.Fatalf("message = %q", res.Error.Message)
			}
			if writer.commentCalls != 0 || writer.closeCalls != 0 {
				t.Fatalf("writer called close:%d comment:%d times", writer.closeCalls, writer.commentCalls)
			}
		})
	}
}

func TestCommentOnCloseOnlyWriterIsUnsupportedRegardlessOfBody(t *testing.T) {
	t.Parallel()

	const wantMsg = `op "comment" is not supported by the configured writer; supported ops: [close]`

	for _, tc := range []struct {
		name string
		args Args
	}{
		{name: "empty body", args: Args{Op: OpComment, ID: "T-1"}},
		{name: "with body", args: Args{Op: OpComment, ID: "T-1", Body: "verdict: pass"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := &fakeCloseWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res := tool.Run(context.Background(), tc.args)
			assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
			if res.Error.Message != wantMsg {
				t.Fatalf("message = %q, want %q", res.Error.Message, wantMsg)
			}
			if writer.calls != 0 {
				t.Fatalf("writer called %d times", writer.calls)
			}
		})
	}
}

func TestCommentOnTransitionWriterIsUnsupportedRegardlessOfBody(t *testing.T) {
	t.Parallel()

	const wantMsg = `op "comment" is not supported by the configured writer; supported ops: [close, transition]`

	for _, tc := range []struct {
		name string
		args Args
	}{
		{name: "empty body", args: Args{Op: OpComment, ID: "T-1"}},
		{name: "with body", args: Args{Op: OpComment, ID: "T-1", Body: "verdict: pass"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := &fakeTransitionWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res := tool.Run(context.Background(), tc.args)
			assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
			if res.Error.Message != wantMsg {
				t.Fatalf("message = %q, want %q", res.Error.Message, wantMsg)
			}
			if writer.closeCalls != 0 || writer.transitionCalls != 0 {
				t.Fatalf("writer called close:%d transition:%d times", writer.closeCalls, writer.transitionCalls)
			}
		})
	}
}

func TestCommentWriterWithoutTransitionAdvertisesCloseComment(t *testing.T) {
	t.Parallel()

	writer := &fakeCommentWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, op := range []Op{OpTransition, OpLinkPR} {
		res := tool.Run(context.Background(), Args{Op: op, ID: "T-1"})
		assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
		if !strings.Contains(res.Error.Message, "supported ops: [close, comment]") {
			t.Fatalf("message = %q, want close/comment supported ops", res.Error.Message)
		}
	}
	if writer.closeCalls != 0 || writer.commentCalls != 0 {
		t.Fatalf("writer called close:%d comment:%d times", writer.closeCalls, writer.commentCalls)
	}
}

func TestWriterWithCommentAndTransitionAdvertisesAllSupportedOps(t *testing.T) {
	t.Parallel()

	writer := &fakeFullWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpLinkPR, ID: "T-1"})
	assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
	if !strings.Contains(res.Error.Message, "supported ops: [close, transition, comment]") {
		t.Fatalf("message = %q, want all supported ops", res.Error.Message)
	}
	if writer.closeCalls != 0 || writer.transitionCalls != 0 || writer.commentCalls != 0 {
		t.Fatalf("writer called close:%d transition:%d comment:%d times",
			writer.closeCalls, writer.transitionCalls, writer.commentCalls)
	}
}

func TestTransitionOnCloseOnlyWriterIsUnsupportedRegardlessOfToState(t *testing.T) {
	t.Parallel()

	const wantMsg = `op "transition" is not supported by the configured writer; supported ops: [close]`

	for _, tc := range []struct {
		name string
		args Args
	}{
		{name: "empty toState", args: Args{Op: OpTransition, ID: "T-1"}},
		{name: "with toState", args: Args{Op: OpTransition, ID: "T-1", ToState: "accepted"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := &fakeCloseWriter{}
			tool, err := New(writer)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res := tool.Run(context.Background(), tc.args)
			assertFailureCategory(t, res, ErrCategoryUnsupportedOp)
			if res.Error.Message != wantMsg {
				t.Fatalf("message = %q, want %q", res.Error.Message, wantMsg)
			}
			if writer.calls != 0 {
				t.Fatalf("writer called %d times", writer.calls)
			}
		})
	}
}

func TestTransitionWriterStillRoutesClose(t *testing.T) {
	t.Parallel()

	writer := &fakeTransitionWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpClose, ID: "T-1", Reason: "done"})
	if res.Outcome != result.OutcomeSucceeded || res.Op != OpClose {
		t.Fatalf("res = %+v", res)
	}
	if writer.closeCalls != 1 || writer.transitionCalls != 0 {
		t.Fatalf("writer called close:%d transition:%d", writer.closeCalls, writer.transitionCalls)
	}
}

func TestRunTransitionTrimsToState(t *testing.T) {
	t.Parallel()

	writer := &fakeTransitionWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := tool.Run(context.Background(), Args{Op: OpTransition, ID: "T-1", ToState: "  accepted  "})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded; error=%+v", res.Outcome, res.Error)
	}
	if writer.transitionState != "accepted" {
		t.Fatalf("transitionState = %q, want trimmed %q", writer.transitionState, "accepted")
	}
}

func TestRunValidationAndContext(t *testing.T) {
	t.Parallel()

	tool, err := New(&fakeCloseWriter{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	assertFailureCategory(t, tool.Run(context.Background(), Args{ID: "T-1"}), ErrCategoryValidation)
	assertFailureCategory(t, tool.Run(context.Background(), Args{Op: OpClose}), ErrCategoryValidation)

	transitionTool, err := New(&fakeTransitionWriter{})
	if err != nil {
		t.Fatalf("New transition: %v", err)
	}
	assertFailureCategory(t, transitionTool.Run(context.Background(), Args{Op: OpTransition, ID: "T-1"}), ErrCategoryValidation)
	assertFailureCategory(t, transitionTool.Run(context.Background(), Args{Op: OpTransition, ID: "T-1", ToState: "   "}), ErrCategoryValidation)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assertFailureCategory(t, tool.Run(ctx, Args{Op: OpClose, ID: "T-1"}), ErrCategoryCanceled)
}

func TestRunWriterErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: ErrCategoryTimeout},
		{name: "canceled", err: context.Canceled, want: ErrCategoryCanceled},
		{name: "unknown", err: errors.New("boom"), want: ErrCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool, err := New(&fakeCloseWriter{err: tt.err})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			assertFailureCategory(t, tool.Run(context.Background(), Args{Op: OpClose, ID: "T-1"}), tt.want)
		})
	}
}

func TestRunTransitionWriterErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: ErrCategoryTimeout},
		{name: "canceled", err: context.Canceled, want: ErrCategoryCanceled},
		{name: "unknown", err: errors.New("boom"), want: ErrCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool, err := New(&fakeTransitionWriter{transitionErr: tt.err})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			assertFailureCategory(t, tool.Run(context.Background(), Args{Op: OpTransition, ID: "T-1", ToState: "accepted"}), tt.want)
		})
	}
}

func TestRunCommentWriterErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: ErrCategoryTimeout},
		{name: "canceled", err: context.Canceled, want: ErrCategoryCanceled},
		{name: "unknown", err: errors.New("boom"), want: ErrCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool, err := New(&fakeCommentWriter{commentErr: tt.err})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			assertFailureCategory(t, tool.Run(context.Background(), Args{Op: OpComment, ID: "T-1", Body: "verdict: pass"}), tt.want)
		})
	}
}

func TestInvokableRun(t *testing.T) {
	t.Parallel()

	tool, err := New(&fakeCloseWriter{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := tool.InvokableRun(context.Background(), `{"op":"close","id":"T-1","reason":"done"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	var got Result
	if unmarshalErr := json.Unmarshal([]byte(raw), &got); unmarshalErr != nil {
		t.Fatalf("unmarshal result: %v", unmarshalErr)
	}
	if got.Outcome != result.OutcomeSucceeded || got.Op != OpClose || got.ID != "T-1" {
		t.Fatalf("got = %+v", got)
	}

	raw, err = tool.InvokableRun(context.Background(), `{"op":"comment","id":"T-1"}`)
	if err != nil {
		t.Fatalf("InvokableRun unsupported op: %v", err)
	}
	if unmarshalErr := json.Unmarshal([]byte(raw), &got); unmarshalErr != nil {
		t.Fatalf("unmarshal unsupported result: %v", unmarshalErr)
	}
	assertFailureCategory(t, got, ErrCategoryUnsupportedOp)
}

func TestInvokableRunComment(t *testing.T) {
	t.Parallel()

	writer := &fakeCommentWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := tool.InvokableRun(context.Background(), `{"op":"comment","id":"T-1","body":"verdict: pass"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	var got Result
	if unmarshalErr := json.Unmarshal([]byte(raw), &got); unmarshalErr != nil {
		t.Fatalf("unmarshal result: %v", unmarshalErr)
	}
	if got.Outcome != result.OutcomeSucceeded || got.Op != OpComment || got.ID != "T-1" {
		t.Fatalf("got = %+v", got)
	}
	if writer.commentCalls != 1 || writer.commentID != "T-1" || writer.commentBody != "verdict: pass" {
		t.Fatalf("comment = calls:%d id:%q body:%q", writer.commentCalls, writer.commentID, writer.commentBody)
	}
}

func TestInvokableRunTransition(t *testing.T) {
	t.Parallel()

	writer := &fakeTransitionWriter{}
	tool, err := New(writer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := tool.InvokableRun(context.Background(), `{"op":"transition","id":"T-1","toState":"accepted"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	var got Result
	if unmarshalErr := json.Unmarshal([]byte(raw), &got); unmarshalErr != nil {
		t.Fatalf("unmarshal result: %v", unmarshalErr)
	}
	if got.Outcome != result.OutcomeSucceeded || got.Op != OpTransition || got.ID != "T-1" {
		t.Fatalf("got = %+v", got)
	}
	if writer.transitionCalls != 1 || writer.transitionID != "T-1" || writer.transitionState != "accepted" {
		t.Fatalf("transition = calls:%d id:%q state:%q", writer.transitionCalls, writer.transitionID, writer.transitionState)
	}
}

func TestInvokableRunRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	tool, err := New(&fakeCloseWriter{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, input := range []string{"", "   ", `{not json`} {
		if _, err := tool.InvokableRun(context.Background(), input); err == nil {
			t.Fatalf("InvokableRun(%q) returned nil error", input)
		}
	}
	if _, err := tool.InvokableRun(context.Background(), `{"op":"comment","id":"T-1","op":"close"}`); err == nil ||
		!strings.Contains(err.Error(), "duplicate top-level key") {
		t.Fatalf("duplicate key err = %v, want duplicate top-level key", err)
	}
}

func TestSchemaAndNameABI(t *testing.T) {
	t.Parallel()

	if Name != "tracker_write" {
		t.Fatalf("Name = %q, want tracker_write", Name)
	}
	first := Schema()
	second := Schema()
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("Schema returned empty JSON")
	}
	first[0] = ' '
	if second[0] == ' ' {
		t.Fatal("Schema returned aliased slice")
	}

	var schema struct {
		Type                 string `json:"type"`
		AdditionalProperties bool   `json:"additionalProperties"`
		Required             []string
		Properties           map[string]json.RawMessage
	}
	if err := json.Unmarshal(second, &schema); err != nil {
		t.Fatalf("schema parse: %v", err)
	}
	if schema.Type != "object" || schema.AdditionalProperties {
		t.Fatalf("schema type/additionalProperties = %q/%t", schema.Type, schema.AdditionalProperties)
	}
	for _, required := range []string{"op", "id"} {
		if !contains(schema.Required, required) {
			t.Fatalf("required %q missing from %#v", required, schema.Required)
		}
	}
	for _, property := range []string{"op", "id", "body", "toState", "reason", "prURL"} {
		if _, ok := schema.Properties[property]; !ok {
			t.Fatalf("property %q missing from schema", property)
		}
	}

	opDesc := schemaPropertyDescription(t, schema.Properties["op"])
	for _, want := range []string{"comment", "configured writer supports"} {
		if !strings.Contains(opDesc, want) {
			t.Fatalf("op description = %q, want %q", opDesc, want)
		}
	}
	bodyDesc := schemaPropertyDescription(t, schema.Properties["body"])
	if !strings.Contains(bodyDesc, "Required for op=comment") || strings.Contains(bodyDesc, "post-v1") {
		t.Fatalf("body description = %q", bodyDesc)
	}
}

func TestInfoDescriptionMentionsOptionalCommentSupport(t *testing.T) {
	t.Parallel()

	tool, err := New(&fakeCommentWriter{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	for _, want := range []string{"op=comment", "configured writer supports", "op=link_pr", "unsupported_op"} {
		if !strings.Contains(info.Desc, want) {
			t.Fatalf("Info.Desc = %q, want %q", info.Desc, want)
		}
	}
}

func TestResultIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Result
		want bool
	}{
		{name: "success", in: Result{Outcome: result.OutcomeSucceeded}, want: false},
		{name: "failed no error", in: Result{Outcome: result.OutcomeFailed}, want: false},
		{name: "api request", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryAPIRequest}}, want: true},
		{name: "rate limited", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryRateLimited}}, want: true},
		{name: "timeout", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryTimeout}}, want: true},
		{name: "unknown", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryUnknown}}, want: true},
		{name: "validation", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryValidation}}, want: false},
		{name: "unsupported op", in: Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryUnsupportedOp}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.IsRetryable(); got != tt.want {
				t.Fatalf("IsRetryable() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestResultPreservesUnknownFieldsInRawJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"outcome":"succeeded","op":"close","id":"T-1","future":"field"}`)
	var got Result
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Contains(got.RawJSON, []byte(`"future"`)) {
		t.Fatalf("RawJSON = %s, want future field preserved", got.RawJSON)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(encoded, []byte("RawJSON")) || bytes.Contains(encoded, []byte("future")) {
		t.Fatalf("marshal emitted raw/future field: %s", encoded)
	}
}

func assertFailureCategory(t *testing.T, res Result, want string) {
	t.Helper()

	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil {
		t.Fatalf("error = nil, want category %q", want)
	}
	if res.Error.Category != want {
		t.Fatalf("category = %q, want %q; message=%q", res.Error.Category, want, res.Error.Message)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func schemaPropertyDescription(t *testing.T, raw json.RawMessage) string {
	t.Helper()

	var property struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("property schema parse: %v", err)
	}
	return property.Description
}
