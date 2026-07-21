#!/usr/bin/env python3
"""Minimal deterministic MCP client for StateSeal's stdio conformance case."""

import json
import os
import subprocess
import sys


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: mcp_client.py SEAL REPOSITORY")
    seal, repository = sys.argv[1:]
    process = subprocess.Popen(
        [seal, "mcp", "serve", "--agent", "codex"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=os.environ.copy(),
    )
    assert process.stdin is not None
    assert process.stdout is not None

    next_id = 0

    def send(message: dict) -> None:
        process.stdin.write(json.dumps(message, separators=(",", ":")) + "\n")
        process.stdin.flush()

    def request(method: str, params: dict) -> dict:
        nonlocal next_id
        next_id += 1
        request_id = next_id
        send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        while True:
            line = process.stdout.readline()
            if not line:
                error = process.stderr.read() if process.stderr is not None else ""
                raise RuntimeError(f"MCP server stopped before response: {error}")
            message = json.loads(line)
            if message.get("method") == "elicitation/create" and "id" in message:
                send(
                    {
                        "jsonrpc": "2.0",
                        "id": message["id"],
                        "result": {"action": "accept", "content": {}},
                    }
                )
                continue
            if message.get("id") == request_id:
                return message

    initialized = request(
        "initialize",
        {
            "protocolVersion": "2025-03-26",
            "capabilities": {"elicitation": {"form": {}}},
            "clientInfo": {"name": "sealbench", "version": "1"},
        },
    )
    assert "result" in initialized, initialized
    send({"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}})

    def tool(name: str, arguments: dict, expect_error: bool = False) -> dict:
        response = request("tools/call", {"name": name, "arguments": arguments})
        if "error" in response:
            if expect_error:
                return response
            raise AssertionError(response)
        result = response["result"]
        if result.get("isError", False):
            if expect_error:
                return result
            raise AssertionError(result)
        if expect_error:
            raise AssertionError(f"{name} unexpectedly succeeded: {result}")
        return result["structuredContent"]

    inspection = tool("inspect_project", {"repo_path": repository})
    assert inspection["enabled"] is False
    assert inspection["confirmation_required"] is True
    assert inspection["setup_token"]

    enabled = tool(
        "enable_project",
        {"repo_path": repository, "setup_token": inspection["setup_token"]},
    )
    assert enabled["enabled"] is True

    delivery = tool(
        "start_delivery",
        {
            "repo_path": repository,
            "goal": "Implement the verified Desktop candidate.",
        },
    )
    assert delivery["stage"] == "PENDING_APPLY", delivery
    assert delivery["verdict"] == "ADMITTED", delivery
    assert delivery["changed_files"] == 1, delivery
    with open(os.path.join(repository, "app.txt"), encoding="utf-8") as source:
        assert source.read().strip() == "original"

    tool(
        "apply_verified",
        {"session_id": delivery["session_id"], "receipt_id": "rcpt_wrong"},
        expect_error=True,
    )
    with open(os.path.join(repository, "app.txt"), encoding="utf-8") as source:
        assert source.read().strip() == "original"

    applied = tool(
        "apply_verified",
        {"session_id": delivery["session_id"], "receipt_id": delivery["receipt_id"]},
    )
    assert applied["stage"] == "APPLIED", applied
    assert applied["receipt_id"] == delivery["receipt_id"], applied
    with open(os.path.join(repository, "app.txt"), encoding="utf-8") as source:
        assert source.read().strip() == "desktop candidate"

    process.stdin.close()
    process.wait(timeout=5)
    if process.returncode != 0:
        error = process.stderr.read() if process.stderr is not None else ""
        raise RuntimeError(f"MCP server exited {process.returncode}: {error}")


if __name__ == "__main__":
    main()
