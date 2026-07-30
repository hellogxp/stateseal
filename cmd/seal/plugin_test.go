package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledCodexPluginIsObservationOnly(t *testing.T) {
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(working, "..", ".."))
	plugin := filepath.Join(root, "plugins", "stateseal")

	var manifest struct {
		Name       string `json:"name"`
		MCPServers string `json:"mcpServers"`
		Skills     string `json:"skills"`
	}
	readPluginJSON(t, filepath.Join(plugin, ".codex-plugin", "plugin.json"), &manifest)
	if manifest.Name != "stateseal" || manifest.MCPServers != "" || manifest.Skills != "./skills/" {
		t.Fatalf("unexpected StateSeal Plugin manifest: %+v", manifest)
	}

	for _, removed := range []string{".mcp.json", filepath.Join("scripts", "stateseal-mcp")} {
		if _, err := os.Stat(filepath.Join(plugin, removed)); !os.IsNotExist(err) {
			t.Fatalf("observation-only Plugin still carries %s", removed)
		}
	}

	skill, err := os.ReadFile(filepath.Join(plugin, "skills", "stateseal", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Never route a coding request through StateSeal",
		"Never start, stop, retry, continue, deny, block, approve, apply, restore",
		"Observed", "Derived", "Inferred",
	} {
		if !strings.Contains(string(skill), expected) {
			t.Fatalf("StateSeal Plugin Skill is missing %q", expected)
		}
	}

	var marketplace struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	readPluginJSON(t, filepath.Join(root, ".agents", "plugins", "marketplace.json"), &marketplace)
	if marketplace.Name != "stateseal" || len(marketplace.Plugins) != 1 || marketplace.Plugins[0].Name != "stateseal" {
		t.Fatalf("unexpected StateSeal marketplace: %+v", marketplace)
	}
}

func readPluginJSON(t *testing.T, path string, target any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
