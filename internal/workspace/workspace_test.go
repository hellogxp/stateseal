package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
)

func TestInspectNonGitWorkspaceFindsSiblingRepositories(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"services/api", "frontend"} {
		repository := filepath.Join(root, relative)
		if err := os.MkdirAll(repository, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := identity.Git(repository, "init", "-b", "main"); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(filepath.Join(root, "node_modules", "ignored"), "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}

	inspection, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Repositories) != 2 {
		t.Fatalf("repositories=%+v, want 2", inspection.Repositories)
	}
	if inspection.Repositories[0].RelativePath != "frontend" || inspection.Repositories[1].RelativePath != "services/api" {
		t.Fatalf("unexpected repositories: %+v", inspection.Repositories)
	}
}

func TestResolveRequiresSelectionAndAcceptsRelativeRepository(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"api", "web"} {
		repository := filepath.Join(root, relative)
		if err := os.MkdirAll(repository, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := identity.Git(repository, "init", "-b", "main"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Resolve(root, ""); err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("ambiguous workspace error=%v", err)
	}
	selected, err := Resolve(root, "web")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(filepath.Join(root, "web"))
	if err != nil {
		t.Fatal(err)
	}
	if selected != expected {
		t.Fatalf("selected=%s", selected)
	}
}

func TestInspectInsideRepositoryUsesContainingRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "pkg", "cache")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(child)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Root != expected || len(inspection.Repositories) != 1 || inspection.Repositories[0].Root != expected {
		t.Fatalf("inspection=%+v", inspection)
	}
}
