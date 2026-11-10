# Penetration-testing-AI (Lean Edition)

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

## Tech stack

**Backend**

| Component | Technology | Role |
| --- | --- | --- |
| Language | Go 1.25 | Single binary; CGO required for SQLite |
| HTTP | Gin 1.9 | Routing, middleware, static assets |
| Agent runtime | CloudWeGo Eino 0.9.21 + eino-ext | ADK ChatModelAgent, tool-call loop, context compaction, streaming events |
| Model access | eino-ext openai / agenticopenai / agenticclaude | OpenAI-compatible channels and native Claude channels |
| MCP | In-house server + modelcontextprotocol/go-sdk | Built-in tool registry, external MCP (HTTP / stdio / SSE) federation |
| Storage | SQLite (mattn/go-sqlite3, WAL) | Conversations, messages, vulnerabilities, tool executions; schema repaired on startup |
| Config | gopkg.in/yaml.v3 | YAML config with environment variable expansion |
| Logging | zap | Structured logs plus diagnostic files |
| Observability | OpenTelemetry (OTLP / stdout) | Agent run tracing |
| Token counting | tiktoken-go | Context budgeting and token usage stats |
| JSON | bytedance/sonic (falls back to encoding/json) | Hot-path serialization |
| Process & terminal | creack/pty, golang.org/x/sys | POSIX live shell; Windows Job / Linux cgroup isolation |
| Network | net/http + HTTP/2 (TLS) | HTTP/HTTPS main site, SSE streaming |

**Frontend**

| Component | Technology | Role |
| --- | --- | --- |
| Runtime | Vanilla ES Modules + plain CSS | No framework, no bundler; edit and refresh |
| UI language | Simplified Chinese only | Literal copy, no translation layer |
| Markdown | marked + DOMPurify | Assistant message rendering with XSS sanitization |
| Visualization | Cytoscape + ELK | Graph layout and relationship views |
| Spreadsheets | xlsx.full.min.js | Vulnerability and asset export |
| Package manager | None (no npm / pnpm) | Third-party libraries are vendored in the repo |

**Tool runtime**

| Component | Technology | Role |
| --- | --- | --- |
| Python | Python 3.10+ / venv / pip | Python-based recipes such as `api-fuzzer`; deps in `requirements.txt` |
| Tool definitions | YAML recipes + YAML role prompts | `tools/` declares arguments, command and output handling; `roles/` scopes the tool set |
| Skills | agentskills.io layout | `SKILL.md` under `skills/`, loaded on demand by the skill tool |

**Deployment**

| Item | Notes |
| --- | --- |
| Artifacts | One executable + `web/` assets + `roles/` `skills/` `tools/` config |
| Start | `run.sh` (check deps → venv → build → run) or run the binary directly |
| TLS | Plain HTTP by default; enable with `tls_enabled: true` and real certificates via `server.tls_cert_path` / `server.tls_key_path` |
| Isolation | Windows Job Object / Linux cgroup, capping child processes and resources |

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
go build -o penetration-testing-ai ./cmd/server
./penetration-testing-ai
```

Open `http://127.0.0.1:8080/`.

Plain HTTP is the default, which suits local development and coursework. Pass `--https` for an in-memory self-signed certificate (the browser will warn), set `PTAI_HTTPS=1`, or configure real certificates in `config.yaml`.

One-shot script (Linux / macOS): `./run.sh` — creates `venv/`, installs dependencies, builds and starts.

## Ports

| Port | Purpose | Notes |
| --- | --- | --- |
| 8080 | Web console and REST API | Plain HTTP by default; HTTPS once TLS is enabled, see `server.tls_*` |
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
web/                 # frontend (vanilla JS)
roles/               # role-scoped test prompts
skills/              # skill packages
tools/               # tool recipes (YAML)
```

## License

See [LICENSE](LICENSE).
