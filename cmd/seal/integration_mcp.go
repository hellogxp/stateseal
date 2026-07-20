package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	stateSealMCPBlockBegin = "# BEGIN STATESEAL MCP: codex-desktop"
	stateSealMCPBlockEnd   = "# END STATESEAL MCP: codex-desktop"
)

func installMCPIntegration(spec integrationSpec, path, binary string, force bool) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(raw)
	if err := validateStateSealMCPMarkers(content); err != nil {
		return err
	}
	start, end, marked := stateSealMCPBlock(content)
	if !marked && containsStateSealMCPTable(content) {
		if !force {
			return fmt.Errorf("%s already defines mcp_servers.stateseal outside the StateSeal-owned block; review it or use --force", path)
		}
		content = removeStateSealMCPTableFamily(content)
	}
	block := renderMCPIntegrationBlock(spec, binary)
	if marked {
		content = content[:start] + block + content[end:]
	} else {
		content = strings.TrimRight(content, "\n")
		if content != "" {
			content += "\n\n"
		}
		content += block
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func validateMCPIntegration(spec integrationSpec, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	content := string(raw)
	if err := validateStateSealMCPMarkers(content); err != nil {
		return err
	}
	_, _, marked := stateSealMCPBlock(content)
	if !marked {
		return fmt.Errorf("%s has no StateSeal-owned MCP block", path)
	}
	for _, required := range []string{
		"[mcp_servers.stateseal]", `args = ["mcp", "serve", "--agent", "` + spec.Agent + `"]`,
		`default_tools_approval_mode = "auto"`, "[mcp_servers.stateseal.tools.enable_project]",
		"[mcp_servers.stateseal.tools.apply_verified]", `approval_mode = "prompt"`,
	} {
		if !strings.Contains(content, required) {
			return fmt.Errorf("%s StateSeal MCP integration is missing %q", path, required)
		}
	}
	binary, err := mcpIntegrationBinary(content)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(binary) {
		return fmt.Errorf("StateSeal MCP command must be absolute")
	}
	info, err := os.Stat(binary)
	if err != nil {
		return fmt.Errorf("StateSeal MCP binary %s: %w", binary, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("StateSeal MCP binary %s is not executable", binary)
	}
	return nil
}

func removeMCPIntegration(_ integrationSpec, path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content := string(raw)
	if err := validateStateSealMCPMarkers(content); err != nil {
		return false, err
	}
	start, end, marked := stateSealMCPBlock(content)
	if !marked {
		return false, nil
	}
	content = strings.TrimSpace(content[:start] + content[end:])
	if content != "" {
		content += "\n"
	}
	return true, os.WriteFile(path, []byte(content), 0o600)
}

func renderMCPIntegrationBlock(spec integrationSpec, binary string) string {
	return fmt.Sprintf(`%s
[mcp_servers.stateseal]
command = %s
args = ["mcp", "serve", "--agent", %s]
enabled = true
required = false
startup_timeout_sec = 10
tool_timeout_sec = 3600
default_tools_approval_mode = "auto"

[mcp_servers.stateseal.tools.enable_project]
approval_mode = "prompt"

[mcp_servers.stateseal.tools.apply_verified]
approval_mode = "prompt"
%s
`, stateSealMCPBlockBegin, strconv.Quote(binary), strconv.Quote(spec.Agent), stateSealMCPBlockEnd)
}

func stateSealMCPBlock(content string) (start, end int, ok bool) {
	start = strings.Index(content, stateSealMCPBlockBegin)
	if start < 0 {
		return 0, 0, false
	}
	relativeEnd := strings.Index(content[start:], stateSealMCPBlockEnd)
	if relativeEnd < 0 {
		return 0, 0, false
	}
	end = start + relativeEnd + len(stateSealMCPBlockEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return start, end, true
}

func validateStateSealMCPMarkers(content string) error {
	begins := strings.Count(content, stateSealMCPBlockBegin)
	ends := strings.Count(content, stateSealMCPBlockEnd)
	if begins != ends || begins > 1 {
		return fmt.Errorf("StateSeal MCP ownership markers are malformed; review %q and %q", stateSealMCPBlockBegin, stateSealMCPBlockEnd)
	}
	return nil
}

func containsStateSealMCPTable(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[mcp_servers.stateseal]" || strings.HasPrefix(trimmed, "[mcp_servers.stateseal.") {
			return true
		}
	}
	return false
}

func removeStateSealMCPTableFamily(content string) string {
	lines := strings.SplitAfter(content, "\n")
	result := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		header := strings.TrimSpace(strings.SplitN(trimmed, "#", 2)[0])
		if strings.HasPrefix(header, "[") && strings.HasSuffix(header, "]") {
			isTarget := header == "[mcp_servers.stateseal]" || strings.HasPrefix(header, "[mcp_servers.stateseal.")
			skipping = isTarget
			if isTarget {
				continue
			}
		}
		if !skipping {
			result = append(result, line)
		}
	}
	return strings.Join(result, "")
}

func mcpIntegrationBinary(content string) (string, error) {
	start, end, ok := stateSealMCPBlock(content)
	if !ok {
		return "", fmt.Errorf("StateSeal MCP block is missing")
	}
	for _, line := range strings.Split(content[start:end], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "command = ") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "command = "))
		binary, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("parse StateSeal MCP command: %w", err)
		}
		return binary, nil
	}
	return "", fmt.Errorf("StateSeal MCP command is missing")
}

func mcpIntegrationBinaryFromPath(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return mcpIntegrationBinary(string(raw))
}
