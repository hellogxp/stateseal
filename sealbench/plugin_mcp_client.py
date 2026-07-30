#!/usr/bin/env python3
"""Smoke-test the MCP server through the packaged StateSeal Plugin launcher."""

import json
import os
import subprocess
import sys


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: plugin_mcp_client.py PLUGIN_LAUNCHER SEAL_BINARY")
    launcher, seal = sys.argv[1:]
    environment = os.environ.copy()
    environment["STATESEAL_BIN"] = os.path.abspath(seal)
    process = subprocess.Popen(
        [os.path.abspath(launcher)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=environment,
    )
    assert process.stdin is not None
    assert process.stdout is not None

    def request(request_id: int, method: str, params: dict) -> dict:
        process.stdin.write(
            json.dumps(
                {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
                separators=(",", ":"),
            )
            + "\n"
        )
        process.stdin.flush()
        while True:
            line = process.stdout.readline()
            if not line:
                error = process.stderr.read() if process.stderr is not None else ""
                raise RuntimeError(f"Plugin MCP server stopped before response: {error}")
            response = json.loads(line)
            if response.get("id") == request_id:
                return response

    initialized = request(
        1,
        "initialize",
        {
            "protocolVersion": "2025-03-26",
            "capabilities": {},
            "clientInfo": {"name": "stateseal-plugin-smoke", "version": "1"},
        },
    )
    assert "result" in initialized, initialized
    process.stdin.write(
        '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}\n'
    )
    process.stdin.flush()
    listed = request(2, "tools/list", {})
    tools = {tool["name"]: tool for tool in listed["result"]["tools"]}
    names = set(tools)
    required = {
        "inspect_project",
        "enable_project",
        "start_delivery",
        "get_delivery_status",
        "apply_verified",
        "reject_delivery",
    }
    assert required <= names, names
    inspect_schema = tools["inspect_project"]["inputSchema"]
    assert {"repo_path", "goal"} <= set(inspect_schema["properties"]), inspect_schema

    process.stdin.close()
    process.wait(timeout=5)
    if process.returncode != 0:
        error = process.stderr.read() if process.stderr is not None else ""
        raise RuntimeError(f"Plugin MCP server exited {process.returncode}: {error}")
    print("Plugin MCP handshake passed")


if __name__ == "__main__":
    main()
