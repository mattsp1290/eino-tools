package shell

import "fmt"

// StartupMode selects the host-owned, noninteractive shell invocation policy.
type StartupMode string

const (
	// StartupModeLogin invokes the shell with -lc (the default).
	StartupModeLogin StartupMode = "login"
	// StartupModeNonLogin invokes the shell with -c.
	StartupModeNonLogin StartupMode = "non-login"
)

const (
	// DefaultShellBinary preserves the current local-symphony shell behavior.
	DefaultShellBinary = "sh"

	// OutputCapBytes is the default per-stream stdout/stderr output cap.
	OutputCapBytes = 256 * 1024

	// DefaultOutputCapBytes aliases OutputCapBytes for constructor option
	// defaulting.
	DefaultOutputCapBytes = OutputCapBytes
)

// Options configures shell tool behavior that is intentionally owned by the
// caller rather than hidden inside the tool.
type Options struct {
	// Env is the process environment for commands. A nil slice inherits the
	// parent process environment at execution. A nonnil slice replaces it.
	Env []string

	// ShellBinary must accept -lc and -c for the selected StartupMode. Empty uses
	// DefaultShellBinary.
	ShellBinary string

	// StartupMode defaults to StartupModeLogin when empty.
	StartupMode StartupMode

	// OutputCapBytes caps stdout and stderr independently. Zero uses
	// DefaultOutputCapBytes.
	OutputCapBytes int
}

// Validate rejects unsupported configuration without resolving or executing a binary.
func (o Options) Validate() error {
	switch o.StartupMode {
	case "", StartupModeLogin, StartupModeNonLogin:
	default:
		return fmt.Errorf("shell: unsupported startup mode %q", o.StartupMode)
	}
	if o.OutputCapBytes < 0 {
		return fmt.Errorf("shell: output cap bytes must be non-negative, got %d", o.OutputCapBytes)
	}
	return nil
}

func (o Options) withDefaults() Options {
	if o.StartupMode == "" {
		o.StartupMode = StartupModeLogin
	}
	if o.ShellBinary == "" {
		o.ShellBinary = DefaultShellBinary
	}
	if o.OutputCapBytes == 0 {
		o.OutputCapBytes = DefaultOutputCapBytes
	}
	if o.Env != nil {
		o.Env = append(make([]string, 0, len(o.Env)), o.Env...)
	}
	return o
}
