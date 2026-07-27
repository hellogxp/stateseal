# StateSeal hooks

Codex Plugin manifests currently load this package's Skill and MCP server.
StateSeal Core installs and validates the active repository-local lifecycle
hooks during first project enablement because hook paths must contain the
absolute StateSeal binary path and must be committed as protected project
infrastructure.

The hooks enforce managed proposal tool and Stop boundaries. If the hook
runtime itself is unavailable, the hook reports a degraded StateSeal state and
does not block ordinary, unmanaged Agent development.
