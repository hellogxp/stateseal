package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledCodexPluginCarriesSkillAndMCPRegistration(t *testing.T) {
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
	if manifest.Name != "stateseal" || manifest.MCPServers != "./.mcp.json" || manifest.Skills != "./skills/" {
		t.Fatalf("unexpected StateSeal Plugin manifest: %+v", manifest)
	}

	var mcpConfig struct {
		Servers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	readPluginJSON(t, filepath.Join(plugin, ".mcp.json"), &mcpConfig)
	server, ok := mcpConfig.Servers["stateseal"]
	if !ok || server.Command != "./scripts/stateseal-mcp" {
		t.Fatalf("StateSeal Plugin does not register its MCP launcher: %+v", mcpConfig)
	}

	launcher := filepath.Join(plugin, "scripts", "stateseal-mcp")
	info, err := os.Stat(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("StateSeal MCP launcher is not executable: %s", launcher)
	}
	raw, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "--confirmation host-tool") {
		t.Fatalf("Plugin MCP launcher does not use the single host approval boundary: %s", raw)
	}

	skill, err := os.ReadFile(filepath.Join(plugin, "skills", "stateseal", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"StateSeal 已介入", "UNVERIFIED", "/seal status", "must not be confused"} {
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
