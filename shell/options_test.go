package shell

import "testing"

func TestOptionsWithDefaultsPreservesCurrentBehavior(t *testing.T) {
	got := (Options{}).withDefaults()

	if got.ShellBinary != DefaultShellBinary {
		t.Fatalf("ShellBinary = %q, want %q", got.ShellBinary, DefaultShellBinary)
	}
	if got.OutputCapBytes != DefaultOutputCapBytes {
		t.Fatalf("OutputCapBytes = %d, want %d", got.OutputCapBytes, DefaultOutputCapBytes)
	}
	if got.Env != nil {
		t.Fatalf("Env = %#v, want nil to inherit parent environment", got.Env)
	}
}

func TestOptionsWithDefaultsPreservesOverrides(t *testing.T) {
	in := Options{
		Env:            []string{"A=B"},
		ShellBinary:    "/bin/sh",
		OutputCapBytes: 1024,
	}

	got := in.withDefaults()

	if got.ShellBinary != in.ShellBinary {
		t.Fatalf("ShellBinary = %q, want %q", got.ShellBinary, in.ShellBinary)
	}
	if got.OutputCapBytes != in.OutputCapBytes {
		t.Fatalf("OutputCapBytes = %d, want %d", got.OutputCapBytes, in.OutputCapBytes)
	}
	if len(got.Env) != 1 || got.Env[0] != "A=B" {
		t.Fatalf("Env = %#v, want copy of %#v", got.Env, in.Env)
	}

	in.Env[0] = "A=C"
	if got.Env[0] != "A=B" {
		t.Fatalf("Env aliases caller slice: got %#v", got.Env)
	}
}

func TestOptionsWithDefaultsKeepsExplicitEmptyEnvironment(t *testing.T) {
	got := (Options{Env: []string{}}).withDefaults()

	if got.Env == nil {
		t.Fatal("Env is nil, want explicit empty environment preserved")
	}
	if len(got.Env) != 0 {
		t.Fatalf("Env length = %d, want 0", len(got.Env))
	}
}
