# CyberStrikeAI（精简版）

[中文](README_CN.md) | [English](README.md)

AI 驱动的渗透测试工作台：把自然语言意图转成可审计的渗透测试动作。基于 Go 构建，内置 Eino 单智能体、MCP 工具集成、角色化提示词与 Skills，工具执行全程可观测。

> [!IMPORTANT]
> 仅可用于自有系统或已获得明确书面授权的目标。本版本采用**免登录**设计，请部署在内网或受控环境，不要直接暴露到公网。

## 功能特性

### 智能体
- 🤖 **单智能体执行**：Eino ADK ChatModelAgent 驱动，支持多轮工具调用、流式输出与任务取消
- 🧩 **MCP 集成**：内置 MCP 服务端（`/mcp`），同时支持外部 MCP（HTTP / stdio / SSE）接入与动态工具发现
- 🎭 **角色化测试**：`roles/` 下按场景定义提示词与工具策略，对话时可切换
- 📦 **Skills**：`skills/` 下遵循 agentskills.io 的技能包，由 skill 工具按需加载
- 🧰 **工具配方**：`tools/` 下 85 个 YAML 工具配方，可按角色启用/禁用

### 安全运营
- 🛡️ **漏洞管理**：严重程度分级、状态流转、筛选统计、明细导出与复测分支
- 📊 **执行监控**：工具调用的入参、输出、耗时与状态全程留痕，支持取消与清理
- 🛑 **调用拦截**：MCP 执行前的正则拦截与提醒模板，防止误触高危命令
- 🗂️ **对话管理**：分组、置顶、重命名，消息过程详情可回看

### 平台
- 🔒 **结果治理**：数据库保存与 Agent 实际看到的同一份兜底后工具结果，恢复路径再次防御超大输出
- 📡 **进程隔离**：任务进程隔离与清理（Windows Job / cgroup）
- ⚙️ **零登录**：访问即用，无需账号体系

## 环境要求

- Go 1.25+（以 `go.mod` 为准）
- Python 3.10+（仅部分 Python 工具配方需要；会自动使用项目下的 `venv/`）

## 快速开始

```bash
# 1. 准备配置（首次运行程序也会自动生成）
cp config.example.yaml config.yaml

# 2. 配置 AI 通道（必填）：编辑 config.yaml 的 ai 段
#    ai:
#      default_channel: main
#      channels:
#        main:
#          name: Main
#          provider: openai_compatible
#          api_key: "sk-..."
#          base_url: "https://api.deepseek.com/v1"
#          model: "deepseek-chat"

# 3. 构建并启动
go build -o cyberstrike-ai ./cmd/server
./cyberstrike-ai --https
```

浏览器打开 `https://127.0.0.1:8080/`，首次访问需接受本地自签证书。

默认 HTTPS；只要明文 HTTP 用 `--http`。手动启动也可用环境变量 `CYBERSTRIKE_HTTPS=1` 开启 HTTPS。

一键脚本（Linux / macOS）：`./run.sh`，脚本会自动创建 `venv/`、安装依赖、编译并启动。

## 服务端口

| 端口 | 用途 | 说明 |
| --- | --- | --- |
| 8080 | Web 控制台与 REST API | 默认 HTTPS（自签证书），可在 `server.tls_*` 配置正式证书 |
| 8081 | MCP 服务端 | 供外部 MCP 客户端接入，`mcp.enabled` 控制开关 |

## 目录结构

```
cmd/server/          # 程序入口
internal/app/        # 装配与路由
internal/handler/    # HTTP 处理器
internal/agent/      # 单智能体与工具调用
internal/multiagent/ # Eino 运行时（上下文压缩、中间件、流式事件）
internal/mcp/        # MCP 服务端与外部 MCP 客户端
internal/security/   # 命令执行、进程隔离、调用拦截
internal/database/   # SQLite 存储
web/                 # 前端（原生 JS + i18n）
roles/               # 角色化测试配置
skills/              # Skills 技能包
tools/               # 工具配方（YAML）
```

## 配置要点

- `ai.channels`：至少配置一个 AI 通道，`ai.default_channel` 指定新对话默认通道
- `agent.max_iterations`：单轮对话最大工具调用轮次，默认 30
- `tool_guard`：调用拦截规则，命中后按配置拦截或提醒
- `mcp` / `external_mcp`：MCP 服务端与外部 MCP 接入
- `security.tool_description_mode`：工具描述模式（精简 / 完整）

完整配置项见 `config.example.yaml` 内的注释。

## 开发

```bash
go build ./...          # 编译全部包
go vet ./...            # 静态检查
go test ./internal/...  # 单元测试
```

前端为原生 ES Module + i18next，无构建步骤，直接修改 `web/static/` 后刷新即可。

## 许可

见 [LICENSE](LICENSE)。
