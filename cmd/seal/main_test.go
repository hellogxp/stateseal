package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetectChecksUsesPortableNPMTest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"ava"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checks, detected := detectChecks(root)
	if detected != "npm test" || len(checks) != 1 || !reflect.DeepEqual(checks[0].Command, []string{"npm", "test"}) {
		t.Fatalf("unexpected Node detection: %s %+v", detected, checks)
	}
}

func TestAugmentLocalToolPath(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	restore := augmentLocalToolPath(root)
	if !strings.HasPrefix(os.Getenv("PATH"), bin+string(os.PathListSeparator)) {
		t.Fatalf("tool path was not prepended: %s", os.Getenv("PATH"))
	}
	restore()
	if os.Getenv("PATH") != original {
		t.Fatal("PATH was not restored")
	}
}
