# Getting started

## Install

Requirements: macOS or Linux. Source builds require Go 1.24 or newer.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal version
```

## Open the runtime panorama

```bash
seal ui
```

StateSeal opens a local browser view and passively discovers recent supported
Agent transcripts. No project initialization, Agent integration, Hook, MCP
server, policy, or approval is required.

Use a fixed port or avoid opening the browser:

```bash
seal ui --address 127.0.0.1:9137
seal ui --no-open
```

## Read the graph

The graph presents a reconstructed execution chain:

```text
Agent session → Skill attribution → tool calls → observed outcome
```

- **Observed** means the signal is present in a transcript.
- **Derived** means StateSeal correlated observed identifiers or paths.
- **Inferred** means a diagnostic hypothesis and may be wrong.

“Completed” means the Agent reported session completion. It does not mean the
change is semantically correct, production-ready, or approved.

## Privacy defaults

StateSeal runs on loopback, does not upload session data, summarizes tool input,
does not expose raw tool output by default, and redacts common token patterns
from displayed request summaries. Treat local transcripts as sensitive data.

## Remove obsolete integrations

Current StateSeal does not install integrations. If an older release installed
Agent lifecycle hooks or a StateSeal MCP entry, remove those entries from the
Agent configuration. Until removed, the upgraded binary keeps legacy hook
commands inert and always returns a neutral result.
