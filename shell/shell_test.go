//go:build unix

package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattsp1290/eino-tools/result"
)

func newToolInTempDir(t *testing.T, opts ...Options) (*Tool, string) {
	t.Helper()

	dir := t.TempDir()
	if len(opts) == 0 {
		opts = []Options{{Env: hermeticShellEnv(t)}}
	} else if opts[0].Env == nil {
		opts[0].Env = hermeticShellEnv(t)
	}
	tool, err := New(dir, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tool, dir
}

func hermeticShellEnv(t *testing.T) []string {
	t.Helper()

	home := t.TempDir()
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	haveHome := false
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") {
			if !haveHome {
				out = append(out, "HOME="+home)
				haveHome = true
			}
			continue
		}
		out = append(out, e)
	}
	if !haveHome {
		out = append(out, "HOME="+home)
	}
	return out
}

func TestNewRejectsInvalidWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := New(""); err == nil {
		t.Fatal("New(empty) returned nil error")
	}
	if _, err := New("relative/path"); err == nil {
		t.Fatal("New(relative) returned nil error")
	}
}

func TestNewAppliesOptions(t *testing.T) {
	t.Parallel()

	env := []string{"A=B"}
	tool, _ := newToolInTempDir(t, Options{
		Env:            env,
		ShellBinary:    "/bin/sh",
		OutputCapBytes: 12,
	})

	if tool.shellBinary != "/bin/sh" {
		t.Fatalf("shellBinary = %q, want /bin/sh", tool.shellBinary)
	}
	if tool.outputCapBytes != 12 {
		t.Fatalf("outputCapBytes = %d, want 12", tool.outputCapBytes)
	}
	env[0] = "A=C"
	if tool.env[0] != "A=B" {
		t.Fatalf("tool env aliases caller env slice: %#v", tool.env)
	}
}

func TestRunSuccessfulCommand(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	res := tool.Run(context.Background(), Args{Cmd: "printf hello"})

	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Stdout != "hello" {
		t.Fatalf("Stdout = %q, want hello", res.Stdout)
	}
	if res.Error != nil {
		t.Fatalf("Error = %+v, want nil", res.Error)
	}
}

func TestRunCwdIsWorkspace(t *testing.T) {
	t.Parallel()

	tool, dir := newToolInTempDir(t)
	res := tool.Run(context.Background(), Args{Cmd: "pwd"})
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded", res.Outcome)
	}

	got := strings.TrimSpace(res.Stdout)
	resolved, _ := filepath.EvalSymlinks(dir)
	if got != dir && got != resolved {
		t.Fatalf("pwd output = %q, want %q or %q", got, dir, resolved)
	}
}

func TestRunNonzeroExitIsSucceededOutcome(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	res := tool.Run(context.Background(), Args{Cmd: "exit 7"})

	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded", res.Outcome)
	}
	if res.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", res.ExitCode)
	}
	if res.Error != nil {
		t.Fatalf("Error = %+v, want nil", res.Error)
	}
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	start := time.Now()
	res := tool.Run(context.Background(), Args{Cmd: "sleep 5", TimeoutSeconds: 1})

	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryTimeout {
		t.Fatalf("Error = %+v, want timeout", res.Error)
	}
	if !res.TimedOut {
		t.Fatal("TimedOut = false, want true")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("timeout did not kill command promptly")
	}
}

func TestRunOutputCapTruncatesStreams(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t, Options{OutputCapBytes: 8})
	res := tool.Run(context.Background(), Args{
		Cmd: "printf abcdefghijklmnop; printf qrstuvwxyz 1>&2",
	})

	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded (err=%+v)", res.Outcome, res.Error)
	}
	if res.Stdout != "abcdefgh" || !res.StdoutTruncated {
		t.Fatalf("stdout = %q truncated=%t, want abcdefgh true", res.Stdout, res.StdoutTruncated)
	}
	if res.Stderr != "qrstuvwx" || !res.StderrTruncated {
		t.Fatalf("stderr = %q truncated=%t, want qrstuvwx true", res.Stderr, res.StderrTruncated)
	}
}

func TestRunEnvOption(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t, Options{Env: []string{"EINO_TOOLS_PROBE=ok"}})
	res := tool.Run(context.Background(), Args{Cmd: "printf $EINO_TOOLS_PROBE"})

	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want succeeded", res.Outcome)
	}
	if res.Stdout != "ok" {
		t.Fatalf("Stdout = %q, want ok", res.Stdout)
	}
}

func TestRunShellBinaryOptionExecFailed(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t, Options{ShellBinary: "/path/to/no/such/sh"})
	res := tool.Run(context.Background(), Args{Cmd: "printf nope"})

	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryExecFailed {
		t.Fatalf("Error = %+v, want exec_failed", res.Error)
	}
	if res.ExitCode != -1 {
		t.Fatalf("ExitCode = %d, want -1", res.ExitCode)
	}
}

func TestRunValidationFailures(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	tests := []Args{
		{Cmd: ""},
		{Cmd: "   "},
		{Cmd: "printf x", TimeoutSeconds: -1},
		{Cmd: "printf x", TimeoutSeconds: MaxTimeoutSeconds + 1},
	}

	for _, args := range tests {
		res := tool.Run(context.Background(), args)
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("args=%+v Outcome = %q, want failed", args, res.Outcome)
		}
		if res.Error == nil || res.Error.Category != ErrCategoryValidation {
			t.Fatalf("args=%+v Error = %+v, want validation", args, res.Error)
		}
	}
}

func TestRunParentCancellation(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := tool.Run(ctx, Args{Cmd: "printf x"})
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryCanceled {
		t.Fatalf("Error = %+v, want canceled", res.Error)
	}
}

func TestInvokableRunParsing(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	tests := []string{
		"",
		"   ",
		"{not-json",
		`{"cmd":"printf a","cmd":"printf b"}`,
	}

	for _, in := range tests {
		if _, err := tool.InvokableRun(context.Background(), in); err == nil {
			t.Fatalf("InvokableRun(%q) returned nil error", in)
		}
	}
}

func TestInvokableRunSuccessSerializes(t *testing.T) {
	t.Parallel()

	tool, _ := newToolInTempDir(t)
	out, err := tool.InvokableRun(context.Background(), `{"cmd":"printf hi"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var got Result
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got.Outcome != result.OutcomeSucceeded || got.Stdout != "hi" {
		t.Fatalf("result = %+v, want succeeded stdout hi", got)
	}
}

func TestSchemaABI(t *testing.T) {
	t.Parallel()

	var got struct {
		Type                 string                     `json:"type"`
		AdditionalProperties bool                       `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
	}
	if err := json.Unmarshal(Schema(), &got); err != nil {
		t.Fatalf("schema parse: %v", err)
	}
	if got.Type != "object" {
		t.Fatalf("type = %q, want object", got.Type)
	}
	if got.AdditionalProperties {
		t.Fatal("additionalProperties = true, want false")
	}
	if _, ok := got.Properties["cmd"]; !ok {
		t.Fatal("schema missing cmd property")
	}
	if _, ok := got.Properties["timeout_seconds"]; !ok {
		t.Fatal("schema missing timeout_seconds property")
	}
	if len(got.Required) != 1 || got.Required[0] != "cmd" {
		t.Fatalf("required = %#v, want [cmd]", got.Required)
	}
}

func TestSchemaReturnsFreshSlice(t *testing.T) {
	t.Parallel()

	a := Schema()
	b := Schema()
	if len(a) > 0 && len(b) > 0 && &a[0] == &b[0] {
		t.Fatal("Schema returned aliased slices")
	}
	a[0] = 'X'
	if Schema()[0] == 'X' {
		t.Fatal("mutating Schema return affected later call")
	}
}

func TestResultJSONOmitEmpty(t *testing.T) {
	t.Parallel()

	out, err := json.Marshal(Result{Outcome: result.OutcomeSucceeded, ExitCode: 0})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), `"error"`) {
		t.Fatalf("success result contains error field: %s", out)
	}
	if strings.Contains(string(out), `"timed_out"`) {
		t.Fatalf("success result contains timed_out field: %s", out)
	}
	if strings.Contains(string(out), `"RawJSON"`) {
		t.Fatalf("success result contains RawJSON field: %s", out)
	}
}

func TestResultPreservesUnknownFieldsInRawJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"outcome":"succeeded","exit_code":0,"stdout":"ok","stderr":"","duration_ms":1,"future":"field"}`)
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

func TestResultIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Result
		want bool
	}{
		{"success", Result{Outcome: result.OutcomeSucceeded}, false},
		{"failed_no_error", Result{Outcome: result.OutcomeFailed}, false},
		{"timeout", Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryTimeout}}, true},
		{"unknown", Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryUnknown}}, true},
		{"validation", Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryValidation}}, false},
		{"canceled", Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryCanceled}}, false},
		{"exec_failed", Result{Outcome: result.OutcomeFailed, Error: &ResultError{Category: ErrCategoryExecFailed}}, false},
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

func TestRunMissingWorkspaceIsExecFailed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	res := tool.Run(context.Background(), Args{Cmd: "printf x"})
	if res.Outcome != result.OutcomeFailed {
		t.Fatalf("Outcome = %q, want failed", res.Outcome)
	}
	if res.Error == nil || res.Error.Category != ErrCategoryExecFailed {
		t.Fatalf("Error = %+v, want exec_failed", res.Error)
	}
}
