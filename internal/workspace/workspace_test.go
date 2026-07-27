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

func TestRouteAutomaticallySelectsRepositoryFromGoalAndReadOnlyEvidence(t *testing.T) {
	root := t.TempDir()
	cache := initializeRoutingRepository(t, root, "services/cache", map[string]string{
		"lru.go": "package cache\n\ntype LRUG[K comparable, V any] struct{}\n",
	})
	initializeRoutingRepository(t, root, "services/billing", map[string]string{
		"invoice.go": "package billing\n\ntype Invoice struct{}\n",
	})

	decision, err := Route(root, "", "为 LRU 和泛型 LRUG 增加一个不改变访问顺序的 Oldest 方法")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Selected == nil || decision.Selected.Root != cache {
		t.Fatalf("routing decision=%+v, want cache repository", decision)
	}
	if decision.Method != "goal_and_workspace_evidence" || len(decision.Evidence) == 0 {
		t.Fatalf("routing did not disclose automatic evidence: %+v", decision)
	}
}

func TestRouteUsesNamedProjectWithoutGitPath(t *testing.T) {
	root := t.TempDir()
	initializeRoutingRepository(t, root, "api", nil)
	web := initializeRoutingRepository(t, root, "web-client", nil)

	decision, err := Route(root, "", "Update web-client input validation")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Selected == nil || decision.Selected.Root != web || decision.Confidence != "high" {
		t.Fatalf("named project was not routed automatically: %+v", decision)
	}
}

func TestRoutePreservesGenuineProductAmbiguity(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"api", "web"} {
		initializeRoutingRepository(t, root, relative, map[string]string{
			"cache.go": "package project\n\nfunc Cache() {}\n",
		})
	}

	decision, err := Route(root, "", "Fix cache behavior")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Selected != nil || len(decision.Candidates) != 2 ||
		decision.Candidates[0].Score != decision.Candidates[1].Score {
		t.Fatalf("equal evidence must remain ambiguous: %+v", decision)
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

func initializeRoutingRepository(t *testing.T, workspaceRoot, relative string, files map[string]string) string {
	t.Helper()
	root := filepath.Join(workspaceRoot, relative)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := identity.Git(root, "add", "."); err != nil {
		t.Fatal(err)
	}
	resolved, err := identity.GitRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
