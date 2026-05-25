//go:build integration

package beads

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	beadssdk "github.com/mattsp1290/beads-go/beads"
)

func TestIntegrationCloseWithRealBD(t *testing.T) {
	repo, bdPath := setupIntegrationRepo(t)
	id := strings.TrimSpace(runIntegrationCommand(t, repo, bdPath,
		"create", "tracker beads adapter issue",
		"--priority", "1",
		"--silent",
	))

	client, err := beadssdk.NewClient(
		beadssdk.WithBinary(bdPath),
		beadssdk.WithDataDir(filepath.Join(repo, ".beads")),
	)
	if err != nil {
		t.Fatalf("beads.NewClient: %v", err)
	}
	adapter, err := New(client)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := adapter.Close(context.Background(), id, "integration complete"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	closed, err := client.Show(context.Background(), id)
	if err != nil {
		t.Fatalf("Show after Close: %v", err)
	}
	if closed.Status != "closed" {
		t.Fatalf("status after Close = %q, want closed", closed.Status)
	}
}

func setupIntegrationRepo(t *testing.T) (repo string, bdPath string) {
	t.Helper()
	bdPath = requireIntegrationBinary(t, "bd")
	gitPath := requireIntegrationBinary(t, "git")

	repo = t.TempDir()
	runIntegrationCommand(t, repo, gitPath, "init")
	runIntegrationCommand(t, repo, bdPath,
		"init",
		"--non-interactive",
		"--skip-agents",
		"--skip-hooks",
		"--prefix", "eitest",
		"--database", "eitest",
	)
	return repo, bdPath
}

func requireIntegrationBinary(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not found on PATH", name)
	}
	return path
}

func runIntegrationCommand(t *testing.T, dir, binary string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", binary, strings.Join(args, " "), err, out)
	}
	return string(out)
}
