package shell

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
	// parent process environment, matching os/exec behavior.
	Env []string

	// ShellBinary is the executable used with "-lc". Empty uses
	// DefaultShellBinary.
	ShellBinary string

	// OutputCapBytes caps stdout and stderr independently. Zero uses
	// DefaultOutputCapBytes.
	OutputCapBytes int
}

func (o Options) withDefaults() Options {
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
