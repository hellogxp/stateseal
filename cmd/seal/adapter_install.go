package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type nestedHookSpec struct {
	event   string
	matcher string
	timeout int
}

func installAgentAdapter(root, agent, binary string, force bool) (string, error) {
	if !filepath.IsAbs(binary) {
		return "", fmt.Errorf("--binary must be an absolute path")
	}
	var path string
	var err error
	switch agent {
	case "codex":
		path = filepath.Join(root, ".codex", "hooks.json")
		err = installNestedHooks(path, binary, agent, force, []nestedHookSpec{{"PostToolUse", "^Bash$", 1800}, {"Stop", "", 1800}})
	case "claude":
		path = filepath.Join(root, ".claude", "settings.json")
		err = installNestedHooks(path, binary, agent, force, []nestedHookSpec{{"PostToolUse", "^Bash$", 1800}, {"Stop", "", 1800}})
	case "qoder":
		path = filepath.Join(root, ".qoder", "settings.json")
		err = installNestedHooks(path, binary, agent, force, []nestedHookSpec{{"PostToolUse", "^(Bash|run_in_terminal)$", 30}, {"Stop", "", 30}})
	case "gemini":
		path = filepath.Join(root, ".gemini", "settings.json")
		err = installNestedHooks(path, binary, agent, force, []nestedHookSpec{{"AfterTool", "^run_shell_command$", 1800000}, {"AfterAgent", "", 1800000}})
	case "cursor":
		path = filepath.Join(root, ".cursor", "hooks.json")
		err = installDirectHooks(path, binary, agent, force, []string{"afterShellExecution", "stop"})
	case "copilot":
		path = filepath.Join(root, ".github", "hooks", "stateseal.json")
		err = installCopilotHooks(path, binary, force)
	case "opencode":
		path = filepath.Join(root, ".opencode", "plugins", "stateseal.js")
		err = installOpenCodePlugin(path, binary, force)
	default:
		return "", fmt.Errorf("unsupported adapter %q", agent)
	}
	return path, err
}

func installCodexHooks(path, binary string, force bool) error {
	return installNestedHooks(path, binary, "codex", force, []nestedHookSpec{{"PostToolUse", "^Bash$", 1800}, {"Stop", "", 1800}})
}

func installNestedHooks(path, binary, agent string, force bool, specs []nestedHookSpec) error {
	root, err := readJSONObject(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	marker := adapterMarker(agent)
	for _, spec := range specs {
		groups, _ := hooks[spec.event].([]any)
		if force || containsMarker(groups, marker) {
			groups = removeMarkedNestedGroups(groups, marker)
		}
		handler := map[string]any{"type": "command", "command": hookCommandLine(binary, agent, spec.event)}
		if agent == "gemini" {
			handler["name"] = "stateseal-" + strings.ToLower(spec.event)
			handler["description"] = "Submit a state-bound candidate to StateSeal"
			handler["timeout"] = spec.timeout
		} else {
			handler["timeout"] = spec.timeout
			if agent == "codex" {
				handler["statusMessage"] = "Sealing candidate state"
			}
		}
		group := map[string]any{"hooks": []any{handler}}
		if spec.matcher != "" {
			group["matcher"] = spec.matcher
		}
		hooks[spec.event] = append(groups, group)
	}
	return writeJSONObject(path, root)
}

func installDirectHooks(path, binary, agent string, force bool, events []string) error {
	root, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	marker := adapterMarker(agent)
	for _, event := range events {
		handlers, _ := hooks[event].([]any)
		if force || containsMarker(handlers, marker) {
			handlers = removeMarkedValues(handlers, marker)
		}
		hooks[event] = append(handlers, map[string]any{"command": hookCommandLine(binary, agent, event)})
	}
	return writeJSONObject(path, root)
}

func installCopilotHooks(path, binary string, force bool) error {
	if raw, err := os.ReadFile(path); err == nil && !force && !strings.Contains(string(raw), adapterMarker("copilot")) {
		return fmt.Errorf("%s already exists and is not managed by StateSeal; use --force to replace it", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	root := map[string]any{"version": 1, "hooks": map[string]any{
		"postToolUse": []any{map[string]any{"type": "command", "command": hookCommandLine(binary, "copilot", "postToolUse"), "matcher": "bash|powershell", "timeoutSec": 1800}},
		"agentStop":   []any{map[string]any{"type": "command", "command": hookCommandLine(binary, "copilot", "agentStop"), "timeoutSec": 1800}},
	}}
	return writeJSONObject(path, root)
}

func installOpenCodePlugin(path, binary string, force bool) error {
	if raw, err := os.ReadFile(path); err == nil && !force && !strings.Contains(string(raw), `"opencode", "hook"`) {
		return fmt.Errorf("%s already exists and is not managed by StateSeal; use --force to replace it", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	quoted, _ := json.Marshal(binary)
	source := `const seal = ` + string(quoted) + `

async function submit(event, payload = {}) {
  if (!process.env.STATESEAL_SUBMIT_DIR) return
  const proc = Bun.spawn([seal, "adapter", "opencode", "hook", event], {
    env: process.env,
    stdin: "pipe",
    stdout: "pipe",
    stderr: "pipe",
  })
  proc.stdin.write(JSON.stringify({ hook_event_name: event, ...payload }))
  proc.stdin.end()
  await proc.exited
}

export const StateSealPlugin = async () => ({
  "tool.execute.after": async (input, output) => {
    if (input.tool !== "bash") return
    await submit("PostToolUse", { tool_input: input?.args ?? output?.args ?? {} })
  },
  event: async ({ event }) => {
    if (event.type === "session.idle") await submit("Stop")
  },
})
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(source), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hookCommandLine(binary, agent, event string) string {
	return shellQuote(binary) + " " + adapterMarker(agent) + " " + shellQuote(event)
}

func adapterMarker(agent string) string { return "adapter " + agent + " hook" }

func readJSONObject(path string) (map[string]any, error) {
	root := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("parse existing %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return root, nil
}

func writeJSONObject(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func containsMarker(value any, marker string) bool {
	raw, _ := json.Marshal(value)
	return strings.Contains(string(raw), marker)
}

func removeMarkedValues(values []any, marker string) []any {
	filtered := values[:0]
	for _, value := range values {
		if !containsMarker(value, marker) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func removeMarkedNestedGroups(groups []any, marker string) []any {
	filtered := groups[:0]
	for _, value := range groups {
		group, ok := value.(map[string]any)
		if !ok {
			filtered = append(filtered, value)
			continue
		}
		handlers, ok := group["hooks"].([]any)
		if !ok {
			filtered = append(filtered, value)
			continue
		}
		handlers = removeMarkedValues(handlers, marker)
		if len(handlers) > 0 {
			group["hooks"] = handlers
			filtered = append(filtered, group)
		}
	}
	return filtered
}
