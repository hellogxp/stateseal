package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeChangesWithContentAndIgnoresUntrackedIgnoredFiles(t *testing.T) {
	root := initRepo(t)
	write(t, filepath.Join(root, "a.txt"), "one")
	write(t, filepath.Join(root, ".gitignore"), "ignored\n")
	Git(root, "add", ".")
	Git(root, "commit", "-m", "initial")
	first, err := Tree(root, []string{"**"})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "ignored"), "noise")
	second, err := Tree(root, []string{"**"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("ignored file changed tree identity")
	}
	write(t, filepath.Join(root, "a.txt"), "two")
	third, err := Tree(root, []string{"**"})
	if err != nil {
		t.Fatal(err)
	}
	if third == second {
		t.Fatal("content change did not change tree identity")
	}
}

func TestMatchesAnyUsesDoublestar(t *testing.T) {
	if !MatchesAny(".github/workflows/ci.yml", []string{".github/workflows/**"}) {
		t.Fatal("expected protected path match")
	}
	if MatchesAny("src/main.go", []string{".github/workflows/**"}) {
		t.Fatal("unexpected match")
	}
}

func TestUnsafeSymlinksFindsRepositoryEscape(t *testing.T) {
	root := initRepo(t)
	if err := os.Symlink("../outside", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	unsafe, err := UnsafeSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(unsafe) != 1 || unsafe[0] != "escape" {
		t.Fatalf("unexpected unsafe links: %v", unsafe)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	Git(root, "config", "user.name", "Test")
	Git(root, "config", "user.email", "test@example.com")
	return root
}
func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
