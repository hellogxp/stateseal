package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	workspacepkg "github.com/hellogxp/stateseal/internal/workspace"
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

func TestRunResolvesSoleRepositoryFromNonGitWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	repository := filepath.Join(workspaceRoot, "product")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(repository, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspaceRoot)

	cmd := newRoot()
	root, decision, err := resolveRunRepository(
		cmd,
		"",
		"开发 product 的兼容性修复",
		i18n.SimplifiedChinese,
		true,
	)
	if err != nil {
		t.Fatalf("run repository resolution failed outside Git: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	if root != resolved || decision.Selected == nil || decision.Method != "sole_repository" {
		t.Fatalf("unexpected run routing decision: root=%q decision=%+v", root, decision)
	}
}

func TestAmbiguousWorkspaceUsesNaturalProjectQuestion(t *testing.T) {
	decision := workspacepkg.Decision{Inspection: workspacepkg.Inspection{
		Root: "/workspace",
		Repositories: []workspacepkg.Repository{
			{Name: "api", RelativePath: "services/api"},
			{Name: "web", RelativePath: "apps/web"},
		},
	}}
	var output bytes.Buffer
	selected, err := chooseWorkspaceProject(strings.NewReader("2\n"), &output, decision, i18n.SimplifiedChinese)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "apps/web" {
		t.Fatalf("selected=%q", selected)
	}
	if !strings.Contains(output.String(), "本次任务要修改哪个项目") ||
		strings.Contains(output.String(), "Git 仓库") {
		t.Fatalf("clarification exposed implementation jargon: %s", output.String())
	}
}
