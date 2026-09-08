//go:build unix

package shellstartup_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mattsp1290/eino-agent/composition"
	"github.com/mattsp1290/eino-agent/extension"
	"github.com/mattsp1290/eino-agent/runtime"
	"github.com/mattsp1290/eino-agent/session"
	"github.com/mattsp1290/eino-agent/tools/einotools"
	"github.com/mattsp1290/eino-tools/catalog"
	"github.com/mattsp1290/eino-tools/internal/shelltest"
	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
)

func mount(t *testing.T, options *shell.Options) (*composition.Registry, error) {
	t.Helper()
	registry, err := composition.NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := exec.LookPath("rg")
	if err != nil {
		t.Fatal("required ripgrep:", err)
	}
	component := extension.Component{InstanceID: "standard", Artifact: extension.Artifact{Name: "shell-startup-fixture", Version: "1", Hash: "fixture-artifact", ConfigHash: "fixture-config", SourceKind: extension.SourceNative}}
	mounted, err := einotools.MountStandard(shelltest.Context(t), registry, component, einotools.Options{
		Scope:   extension.GlobalScope(),
		Catalog: catalog.Options{ShellOptions: options, SearchOptions: &search.Options{RGBinary: rg, Env: []string{"PATH=/usr/bin:/bin"}}},
	})
	if err == nil {
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mounted.Close(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	return registry, err
}

func acquire(t *testing.T, registry *composition.Registry) *runtime.RunPlan {
	t.Helper()
	plan, err := registry.AcquireRunPlan(shelltest.Context(t), runtime.RunPlanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(plan.Release)
	return plan
}

func resolvedShell(t *testing.T, plan *runtime.RunPlan, root string) runtime.Tool {
	t.Helper()
	tools, err := plan.ResolveTools(shelltest.Context(t), runtime.ToolScopeContext{WorkspaceID: "fixture", WorkspaceRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name == shell.Name {
			return tool
		}
	}
	t.Fatal("mounted shell missing")
	return runtime.Tool{}
}

func identity(t *testing.T, plan *runtime.RunPlan) session.ToolPlanIdentity {
	t.Helper()
	for _, component := range plan.Descriptor().Components {
		for _, tool := range component.Tools {
			if tool.RegistrationID == catalog.IDShell {
				return tool
			}
		}
	}
	t.Fatal("shell descriptor missing")
	return session.ToolPlanIdentity{}
}

func TestMountedStartupContract(t *testing.T) {
	shelltest.Contract(t, func(t *testing.T, root string, options *shell.Options) shelltest.Run {
		registry, err := mount(t, options)
		if err != nil {
			t.Fatal(err)
		}
		original := identity(t, acquire(t, registry))
		// Each call obtains a new plan and materializes a fresh executor, proving
		// future factories retain the policy after the contract mutates caller input.
		return func(ctx context.Context, args shell.Args) shell.Result {
			plan, err := registry.AcquireRunPlan(ctx, runtime.RunPlanRequest{})
			if err != nil {
				t.Error(err)
				return shell.Result{}
			}
			defer plan.Release()
			if got := identity(t, plan); got != original {
				t.Error("mounted descriptor changed after caller mutation")
			}
			tool := resolvedShell(t, plan, root)
			raw, err := json.Marshal(args)
			if err != nil {
				t.Error(err)
				return shell.Result{}
			}
			input, err := tool.InputDecoder.DecodeToolInput(ctx, raw)
			if err != nil {
				t.Error(err)
				return shell.Result{}
			}
			output, err := tool.Executor.Execute(ctx, runtime.ToolCall{Input: input})
			if err != nil {
				t.Error(err)
				return shell.Result{}
			}
			var got shell.Result
			if err := json.Unmarshal(output.Structured, &got); err != nil {
				t.Error(err)
			}
			return got
		}
	})
}

func TestMountedPolicyIdentity(t *testing.T) {
	environment := []string{"PATH=/usr/bin:/bin", "HOME=" + shelltest.Root(t)}
	var identities []session.ToolPlanIdentity
	for _, mode := range []shell.StartupMode{"", shell.StartupModeLogin, shell.StartupModeNonLogin} {
		registry, err := mount(t, &shell.Options{ShellBinary: "/bin/sh", StartupMode: mode, Env: environment})
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, identity(t, acquire(t, registry)))
	}
	if identities[0] != identities[1] {
		t.Fatal("default/login descriptors differ")
	}
	if identities[1].ExecutorHash == identities[2].ExecutorHash || identities[1].SchemaHash != identities[2].SchemaHash {
		t.Fatal("mode not isolated in executor identity")
	}
}

func TestMountedRejectsPolicyAuthority(t *testing.T) {
	registry, err := mount(t, &shell.Options{ShellBinary: "/bin/sh", StartupMode: "unsupported"})
	if err == nil {
		t.Fatal("invalid mode mounted")
	}
	if len(acquire(t, registry).Descriptor().Components) != 0 {
		t.Fatal("failed mount published capabilities")
	}
	home := shelltest.Root(t)
	marker := filepath.Join(home, "marker")
	shelltest.Write(t, filepath.Join(home, ".profile"), "export VALUE=profile; printf marker > \"$MARKER\"\n")
	registry, err = mount(t, &shell.Options{ShellBinary: "/bin/sh", StartupMode: shell.StartupModeNonLogin, Env: []string{"HOME=" + home, "MARKER=" + marker, "VALUE=host"}})
	if err != nil {
		t.Fatal(err)
	}
	tool := resolvedShell(t, acquire(t, registry), shelltest.Root(t))
	for _, field := range []string{`"startup_mode":"login"`, `"shell_binary":"/missing"`, `"env":["VALUE=model"]`} {
		ctx := shelltest.Context(t)
		input, err := tool.InputDecoder.DecodeToolInput(ctx, json.RawMessage(`{"cmd":"printf '%s' \"$VALUE\"",`+field+`}`))
		if err != nil {
			continue
		} // The published adapter may reject or discard unknown fields.
		output, err := tool.Executor.Execute(ctx, runtime.ToolCall{Input: input})
		if err != nil {
			t.Fatal(err)
		}
		var got shell.Result
		if err := json.Unmarshal(output.Structured, &got); err != nil {
			t.Fatal(err)
		}
		shelltest.Success(t, got, "host", "", 0)
		shelltest.Absent(t, marker)
	}
}
