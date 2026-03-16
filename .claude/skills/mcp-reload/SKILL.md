---
name: mcp-reload
description: Rebuild, install, and guide session resume for the Finch MCP server after code changes
---

# Reload Finch MCP Server

## Background

The Finch MCP server is a user-scoped stdio server (`finch-mcp` binary on PATH, installed to `~/.local/bin/`). Claude Code spawns a fresh process at session start — there is no in-session reload mechanism. After installing an updated binary, the user must exit and resume the session with `claude --continue` to pick up changes while preserving conversation context.

## Workflow

1. **Determine scope of changes** — check which files have been modified:

   - **MCP-only** — only `mcp/` files changed
   - **Full stack** — any of `proto/`, `core/`, or `daemon/` files also changed

2. **Build, install, and restart** — the install targets use atomic replacement (`cp` + `mv`), so they succeed even while binaries are running:

   MCP-only changes:
   ```bash
   just build-mcp && just install-mcp
   ```

   Full stack changes (proto, core, or daemon involved):
   ```bash
   just all && just install && systemctl --user restart finch.service
   ```

3. **Instruct the user** to exit the session and resume:

   > To pick up the updated MCP server, please exit this session and resume with `claude --continue`.

4. **After the user resumes**, verify connectivity by calling the `mcp__finch__ping` tool. If ping fails, check daemon status with `systemctl --user status finch.service`.
