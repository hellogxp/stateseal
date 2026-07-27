# StateSeal Plugin

The Plugin bundles StateSeal's Codex Skill and MCP registration. Users install
StateSeal Core and this Plugin; they do not install a separate MCP server.

The included launcher resolves the Core binary from `STATESEAL_BIN` or `seal`
on `PATH`. StateSeal Core creates and validates repository-local lifecycle
hooks when a project is first enabled.

From a source checkout:

```bash
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

The marketplace command only registers this local source tree as a Codex Plugin
catalog. It does not publish the Plugin, upload source code, create an online
account, or install MCP separately.

Start a new Codex task after installation. Ordinary code-changing requests are
routed automatically and visibly; `@stateseal` and `/seal` commands are
optional controls. Read-only work is not routed. The first project contract and
final Apply require native confirmation.

If StateSeal's own confirmation, MCP, store, or isolated worker is unavailable,
the Agent continues its native development flow with a visible `UNVERIFIED`
result and no receipt. A healthy enforce-policy rejection remains authoritative,
and final Apply never fails open.
