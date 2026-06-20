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
			args := Args{Op: op, ID: "T-1"}
			if op == OpTransition {
				args.ToState = "accepted"
			}
			res := tool.Run(context.Background(), args)
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
