//go:build unix

// Package shelltest contains shared leaf and mounted execution acceptance fixtures.
package shelltest

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mattsp1290/eino-tools/result"
	"github.com/mattsp1290/eino-tools/shell"
)

type Run func(context.Context, shell.Args) shell.Result
type Factory func(*testing.T, string, *shell.Options) Run

func Modes() []shell.StartupMode {
	modes := []shell.StartupMode{shell.StartupModeNonLogin}
	if os.Getenv("EINO_TOOLS_TEST_LOGIN") == "1" {
		modes = append(modes, shell.StartupModeLogin)
	}
	return modes
}

func Root(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func Write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func Success(t *testing.T, got shell.Result, stdout, stderr string, exit int) {
	t.Helper()
	if got.Outcome != result.OutcomeSucceeded || got.ExitCode != exit || got.Error != nil || got.Stdout != stdout || got.Stderr != stderr {
		t.Fatalf("result = %+v, want stdout=%q stderr=%q exit=%d", got, stdout, stderr, exit)
	}
}

func Absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected absent %s, stat: %v", path, err)
	}
}

// Contract runs identical observable regressions through either public execution path.
func Contract(t *testing.T, factory Factory) {
	t.Helper()
	t.Run("profile-and-capture", func(t *testing.T) { profile(t, factory) })
	for _, mode := range Modes() {
		t.Run(string(mode), func(t *testing.T) {
			t.Run("syntax-env-stdin", func(t *testing.T) {
				root := Root(t)
				options := &shell.Options{ShellBinary: "/bin/sh", StartupMode: mode, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + Root(t), "VALUE=host"}}
				run := factory(t, root, options)
				command := "printf '%s\\n' \"$VALUE\"; printf 'a\\nb\\n' | tr a-z A-Z > relative; cat relative\nprintf 'quoted: %s' \"a'b\\\"c\"; printf error >&2; if read -r line; then exit 90; fi; case $- in *i*) exit 91;; esac; exit 7"
				Success(t, run(Context(t), shell.Args{Cmd: command}), "host\nA\nB\nquoted: a'b\"c", "error", 7)
				content, err := os.ReadFile(filepath.Join(root, "relative")) //nolint:gosec // Test-owned temporary fixture.
				if err != nil || string(content) != "A\nB\n" {
					t.Fatalf("redirect = %q, %v", content, err)
				}
			})
			t.Run("caps", func(t *testing.T) {
				options := &shell.Options{ShellBinary: "/bin/sh", StartupMode: mode, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + Root(t)}, OutputCapBytes: 8}
				run := factory(t, Root(t), options)
				options.OutputCapBytes = 100
				got := run(Context(t), shell.Args{Cmd: "printf abcdefghijk; printf lmnopqrstuv >&2"})
				Success(t, got, "abcdefgh", "lmnopqrs", 0)
				if !got.StdoutTruncated || !got.StderrTruncated {
					t.Fatalf("missing truncation: %+v", got)
				}
			})
			for _, cancel := range []bool{false, true} {
				name := "timeout"
				if cancel {
					name = "cancellation"
				}
				t.Run(name, func(t *testing.T) { descendants(t, factory, mode, cancel) })
			}
		})
	}
}

func profile(t *testing.T, factory Factory) {
	root, home := Root(t), Root(t)
	marker := filepath.Join(home, "profile-marker")
	canary := "canary-" + rand.Text()
	Write(t, filepath.Join(home, ".profile"), "export STARTUP_CANARY='"+canary+"'\nprintf marker > \"$MARKER\"\n")
	Write(t, filepath.Join(root, "workspace-file"), "workspace\n")
	options := &shell.Options{ShellBinary: "/bin/sh", StartupMode: shell.StartupModeNonLogin, OutputCapBytes: 4096, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + home, "MARKER=" + marker}}
	run := factory(t, root, options)
	options.Env[1] = "HOME=/missing"
	options.ShellBinary = "/missing"
	options.StartupMode = shell.StartupModeLogin
	options.OutputCapBytes = 1
	command := "pwd; printf '%s\\n' \"$HOME\" \"${STARTUP_CANARY-unset}\"; cat workspace-file"
	check := func() {
		got := run(Context(t), shell.Args{Cmd: command})
		Success(t, got, root+"\n"+home+"\nunset\nworkspace\n", "", 0)
		if strings.Contains(got.Stdout+got.Stderr, canary) {
			t.Fatal("startup canary leaked")
		}
		Absent(t, marker)
	}
	check()
	control := run(Context(t), shell.Args{Cmd: ". \"$HOME/.profile\"; printf '%s' \"$STARTUP_CANARY\""})
	Success(t, control, canary, "", 0)
	content, err := os.ReadFile(marker) //nolint:gosec // Test-owned temporary fixture.
	if err != nil || string(content) != "marker" {
		t.Fatalf("positive control marker: %q %v", content, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	check()
	if os.Getenv("EINO_TOOLS_TEST_LOGIN") == "1" {
		t.Run("login-profile-control", func(t *testing.T) {
			login := factory(t, root, &shell.Options{ShellBinary: "/bin/sh", StartupMode: shell.StartupModeLogin, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + home, "MARKER=" + marker}})
			Success(t, login(Context(t), shell.Args{Cmd: "printf '%s' \"$STARTUP_CANARY\""}), canary, "", 0)
			content, err := os.ReadFile(marker) //nolint:gosec // Test-owned temporary fixture.
			if err != nil || string(content) != "marker" {
				t.Fatalf("login control marker: %q %v", content, err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			check()
		})
	}
}

func descendants(t *testing.T, factory Factory, mode shell.StartupMode, cancelParent bool) {
	root := Root(t)
	options := &shell.Options{ShellBinary: "/bin/sh", StartupMode: mode, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + Root(t)}}
	run := factory(t, root, options)
	ctx, cancel := context.WithCancel(Context(t))
	defer cancel()
	// The shell records its process group and the child's PID before readiness.
	// The child is intentionally non-detached; detached descendants are out of scope.
	command := "printf '%s' $$ > group; (printf ready > child-ready; sleep 3; printf survived > sentinel) & child=$!; printf '%s' \"$child\" > child; while [ ! -f child-ready ]; do sleep 0.01; done; printf ready > ready; wait"
	done := make(chan shell.Result, 1)
	go func() { done <- run(ctx, shell.Args{Cmd: command, TimeoutSeconds: 2}) }()
	t.Cleanup(func() {
		cancel()
		raw, err := os.ReadFile(filepath.Join(root, "group")) //nolint:gosec // Test-owned temporary fixture.
		if err == nil {
			pid, err := strconv.Atoi(string(raw))
			if err == nil && pid > 1 {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})
	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("descendant readiness timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	child, err := os.ReadFile(filepath.Join(root, "child")) //nolint:gosec // Test-owned temporary fixture.
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(child))
	if err != nil || pid <= 1 {
		t.Fatalf("invalid child PID: %q", child)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("child was not live at readiness: %v", err)
	}
	ready := time.Now()
	if cancelParent {
		cancel()
	}
	var got shell.Result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("execution failed to return promptly")
	}
	category := shell.ErrCategoryTimeout
	if cancelParent {
		category = shell.ErrCategoryCanceled
	}
	if got.Outcome != result.OutcomeFailed || got.Error == nil || got.Error.Category != category || got.TimedOut == cancelParent {
		t.Fatalf("cleanup result = %+v, want %s", got, category)
	}
	// Wait past the child's deadline; zombies cannot write a sentinel and need not
	// disappear immediately on systems whose init reaps asynchronously.
	if remaining := time.Until(ready.Add(3500 * time.Millisecond)); remaining > 0 {
		time.Sleep(remaining)
	}
	Absent(t, filepath.Join(root, "sentinel"))
	t.Logf("verified %s process group cleanup for child %d", category, pid)
}
