# Public Surface Conventions

finch is a public repository. Everything checked in is read by contributors and tools that do not share the maintainer's personal environment — other people's editors, review bots, and agents that load only what the repo itself ships. Tracked files must therefore stand on their own.

## No Private Tooling References

Do NOT name personal or environment-specific tooling in tracked files. Such a reference resolves only inside one maintainer's setup and dangles for everyone else:

- Personal agent skills invoked as slash-commands (e.g. a `/personal-skill` that lives in someone's own config rather than in this repo).
- Personal MCP servers (e.g. an external project-management backend wired up in a personal environment).
- The maintainer's private repositories, or literal personal home paths (`/home/<user>/...`).

State the intent in plain English instead. If a piece of guidance currently lives only in personal tooling, copy the load-bearing content into the repo so the repo owns it.

## What's Fine

- **finch's own paths.** The application's runtime and config paths — `$XDG_RUNTIME_DIR/finch/...`, `~/.local/share/finch/...`, `FINCH_DB_PATH` — describe finch's behavior, not a person. Use them freely.
- **finch's own in-repo skills.** Skills defined under `.claude/skills/` (e.g. the `mcp-reload` skill) ship with the repo and resolve for anyone who has it. Referencing them is fine.
- **Named convention slots.** A slot written as `(extension point: \`name\`)` names an abstract contract, not a private tool, so it is fine to declare. The guidance supplied under each slot must read as a standalone project convention — a contributor with no personal tooling should understand it on its own.

## Why

finch's conventions must be legible to any contributor or agent — including review bots — without the maintainer's personal setup. A reference that only resolves inside one person's environment is noise to everyone else, and silently rots when that personal tooling changes.
