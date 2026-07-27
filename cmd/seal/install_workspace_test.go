package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
)

func TestInstallCommandProvidesOneStepCompatibilityIntegration(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "seal")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "hooks.json")
	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"install", "codex-cli", "--binary", binary, "--config", config})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "StateSeal installation complete") {
		t.Fatalf("one-step install output is incomplete: %s", out.String())
	}
	raw, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), adapterCommandMarker("codex", "hook")) {
		t.Fatalf("one-step install did not register Codex lifecycle hooks: %s", raw)
	}
}

func TestWorkspaceListCommandShowsRepositoriesWithoutRequiringGitParent(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"services/api", "web"} {
		repository := filepath.Join(root, relative)
		if err := os.MkdirAll(repository, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := identity.Git(repository, "init", "-b", "main"); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"workspace", "list", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "services/api") || !strings.Contains(out.String(), "web") {
		t.Fatalf("workspace list omitted repositories: %s", out.String())
	}
}
