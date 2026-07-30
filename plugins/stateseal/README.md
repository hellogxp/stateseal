# StateSeal Plugin

This optional Plugin teaches Codex how to explain StateSeal's local, read-only
runtime panorama when the user explicitly asks for it.

It does not register a delivery-control MCP server, auto-route coding tasks,
install lifecycle hooks, launch workers, block execution, request approval, or
apply changes. Ordinary coding work never triggers StateSeal.

The primary product is the local UI:

```bash
seal ui
```

The Skill uses three evidence labels—Observed, Derived, and Inferred—and never
presents an inference as Agent authority.
