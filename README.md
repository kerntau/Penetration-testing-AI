# CyberStrikeAI (Lean Edition)

[中文](README_CN.md) | [English](README.md)

An AI-driven 渗透测试 workbench: turn natural-language intent into auditable 渗透测试 actions. Built in Go with an Eino single agent, MCP tool integration, role-scoped prompts and Skills, with every tool execution observable.

> [!IMPORTANT]
> Use only against systems you own or are explicitly authorized in writing to test. This edition runs **without authentication**: deploy it on an internal or otherwise controlled network, never on the public internet.

## Features

### Agent
- 🤖 **Single agent execution** — Eino ADK ChatModelAgent with multi-turn tool calls, streaming output and task cancellation
- 🧩 **MCP integration** — built-in MCP server (`/mcp`) plus external MCP servers over HTTP / stdio / SSE with dynamic tool discovery
- 🎭 **Role-scoped testing** — prompts and tool policies per scenario under `roles/`, switchable per conversation
- 📦 **Skills** — agentskills.io style skill packages under `skills/`, loaded on demand by the skill tool
- 🧰 **Tool recipes** — 85 YAML recipes under `tools/`, enable or disable per role

### Security operations
- 🛡️ **Vulnerability management** — severity, status workflow, filtering, statistics, detail export and retest branches
- 📊 **Execution monitoring** — arguments, output, duration and status for every tool call, with cancel and cleanup
- 🛑 **Tool call guard** — regex interception and reminder templates before MCP execution
- 🗂️ **Conversation management** — grouping, pinning, renaming, per-message process details

### Platform
- 🔒 **Result governance** — the database stores the same capped tool result the agent actually saw; restore paths re-apply the caps
- 📡 **Process isolation** — task containment and cleanup (Windows Job / cgroup)
- ⚙️ **No login** — open access, no account system

## Requirements

- Go 1.25+ (see `go.mod`)
- Python 3.10+ (only for Python-based tool recipes; the project `venv/` is used automatically)

## Quick start

```bash
# 1. Prepare the config (the program also generates one on first run)
cp config.example.yaml config.yaml

# 2. Configure an AI channel (required): edit the `ai` section of config.yaml
#    ai:
#      default_channel: main
#      channels:
#        main:
#          name: Main
#          provider: openai_compatible
#          api_key: "sk-..."
#          base_url: "https://api.deepseek.com/v1"
#          model: "deepseek-chat"

# 3. Build and run
go build -o cyberstrike-ai ./cmd/server
./cyberstrike-ai --https
```

Open `https://127.0.0.1:8080/` and accept the local self-signed certificate once.

HTTPS is the default; pass `--http` for plain HTTP, or set `CYBERSTRIKE_HTTPS=1`.

One-shot script (Linux / macOS): `./run.sh` — creates `venv/`, installs dependencies, builds and starts.

## Ports

| Port | Purpose | Notes |
| --- | --- | --- |
| 8080 | Web console and REST API | HTTPS by default (self-signed); configure real certificates via `server.tls_*` |
| 8081 | MCP server | For external MCP clients; toggled by `mcp.enabled` |

## Layout

```
cmd/server/          # entrypoint
internal/app/        # wiring and routes
internal/handler/    # HTTP handlers
internal/agent/      # single agent and tool invocation
internal/multiagent/ # Eino runtime (compaction, middleware, streaming events)
internal/mcp/        # MCP server and external MCP clients
internal/security/   # command execution, process isolation, tool guard
internal/database/   # SQLite storage
web/                 # frontend (vanilla JS + i18n)
roles/               # role-scoped test prompts
skills/              # skill packages
tools/               # tool recipes (YAML)
```

## License

See [LICENSE](LICENSE).
