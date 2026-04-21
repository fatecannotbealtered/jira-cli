# jira-cli

[![CI](https://github.com/fatecannotbealtered/jira-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/fatecannotbealtered/jira-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fatecannotbealtered/jira-cli)](https://goreportcard.com/report/github.com/fatecannotbealtered/jira-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![npm version](https://img.shields.io/npm/v/@fatecannotbealtered-/jira-cli.svg)](https://www.npmjs.com/package/@fatecannotbealtered-/jira-cli)

[English](README.md) | 中文

全功能 Jira Data Center 命令行工具，为人类和 AI Agent 而生。在终端中管理 Issue、Sprint、Board、Epic、项目、用户和过滤器。

Go 构建，标准库与少量固定依赖，单文件二进制。

[安装](#安装) · [认证](#认证) · [命令](#命令) · [JSON 输出](#json-输出) · [安全](#安全) · [贡献](#贡献) · [使用说明](#使用说明)

## 使用说明

本项目以**个人学习、研究、日常自用**为主分享，不视为带 SLA 的正式产品；作者**不提供商业支持**，也不对是否适合生产环境作承诺。若你在公司或商业场景下使用，请**自行评估**并遵守贵司规定。软件按「原样」提供，不作任何明示或默示担保。

## 特性

| 能力 | 说明 |
|------|------|
| 🎯 **完整覆盖** | Issue、Sprint、Board、Epic、项目、用户、过滤器 |
| 🤖 **AI 友好** | `--json` 扁平输出（省 token）、`--quiet`、`--dry-run`、`--force`、`--fields`、`--raw` |
| ⚡ **单文件二进制** | 无需单独安装运行时，下载即用 |
| 🔄 **智能重试** | 自动处理 429 限流和 5xx 错误（指数退避） |
| 🌈 **美观输出** | 彩色表格，支持中日韩字符宽度 |
| 🔍 **强大搜索** | 完整 JQL 支持，自动翻页 |
| 🔧 **自定义字段** | 支持创建和编辑时设置自定义字段 |
| 🔐 **PAT 认证** | Bearer Token 认证（Personal Access Token） |
| 🌐 **环境变量** | `JIRA_HOST` 和 `JIRA_TOKEN` 覆盖配置文件，适合 CI/Agent |
| 📋 **审计日志** | 所有写操作自动记录 JSONL 审计日志，按月轮转，自动清理 |

## 安装

### 快速开始

推荐流程：先通过 npm 安装 CLI，再用 `npx skills add` 安装 AI Agent Skill。

```bash
# 安装 CLI
npm install -g @fatecannotbealtered-/jira-cli

# 安装 CLI Skill
npx skills add fatecannotbealtered/jira-cli -y -g

# 登录并验证
jira-cli login
jira-cli doctor
```

CLI 包提供 `jira-cli` 命令；Skill 会让兼容的 AI 编程助手知道如何安全地调用它。如果你是正在协助用户安装的 AI Agent，请执行同一组步骤，并在遇到交互式浏览器或终端提示时让用户配合完成。

### 其他安装方式

```bash
# Go install
go install github.com/fatecannotbealtered/jira-cli/cmd/jira-cli@latest
```

或从 [GitHub Releases](https://github.com/fatecannotbealtered/jira-cli/releases) 下载二进制文件并添加到 PATH。

## 认证

jira-cli 支持 **Jira Data Center**（私有化部署），使用 **Personal Access Token (PAT)** 认证。

### 交互式登录

```bash
jira-cli login
# Jira host: https://jira.company.com
# Personal Access Token (PAT): ****
# ✔ Logged in as John Doe (johndoe)

jira-cli doctor    # 验证连通性
jira-cli logout    # 删除凭据
```

### 非交互式登录（CI / AI Agent）

```bash
jira-cli login --host https://jira.company.com --token <PAT>
```

### 环境变量

环境变量优先于配置文件，推荐在 CI 和 AI Agent 场景使用：

```bash
export JIRA_HOST=https://jira.company.com
export JIRA_TOKEN=<your-personal-access-token>
jira-cli doctor --json
```

### 生成 PAT

1. 登录你的 Jira Data Center 实例
2. 进入 **个人资料** → **Personal Access Tokens**
3. 创建具有适当权限的新 Token

## 命令

### Issue 管理

```bash
# 查看
jira-cli issue get PROJ-123
jira-cli issue list --project PROJ
jira-cli issue list --project PROJ --status "In Progress" --assignee me

# 创建和编辑
jira-cli issue create --project PROJ --summary "修复登录 Bug" --type Bug
jira-cli issue create --project PROJ --summary "新功能" --field "Story Points=5"
jira-cli issue edit PROJ-123 --priority High --assignee me
jira-cli issue edit PROJ-123 --field "Story Points=8" --field "Team=Backend"
jira-cli issue delete PROJ-123 --force          # --force 跳过确认

# 克隆
jira-cli issue clone PROJ-123
jira-cli issue clone PROJ-123 --summary "新标题" --with-links

# 状态流转
jira-cli issue transitions PROJ-123          # 列出可用流转
jira-cli issue transition PROJ-123 "Done"    # 需要提供状态名称

# 批量流转
jira-cli issue bulk-transition "Done" --issues PROJ-1,PROJ-2,PROJ-3
jira-cli issue bulk-transition "In Progress" --jql "sprint = 10 AND status = 'To Do'"

# 协作
jira-cli issue assign PROJ-123 me               # 分配给当前用户
jira-cli issue assign PROJ-123 johndoe          # 按用户名分配（DC 用 name，非 accountId）
jira-cli issue watch PROJ-123
jira-cli issue vote PROJ-123

# 评论
jira-cli issue comment add PROJ-123 --body "已在 PR #42 中修复"
jira-cli issue comment list PROJ-123

# 工时
jira-cli issue worklog add PROJ-123 --time 2h --comment "调试"
jira-cli issue worklog list PROJ-123

# 链接和附件
jira-cli issue link PROJ-123 --to PROJ-456 --type "blocks"
jira-cli issue attach PROJ-123 --file ./screenshot.png
jira-cli issue remote-link PROJ-123 --url https://pr.url --title "PR #42"
```

### 搜索（JQL）

```bash
jira-cli search "assignee = currentUser() AND status != Done"
jira-cli search "project = PROJ AND sprint in openSprints()" --all
jira-cli search "type = Bug AND priority = High" --count
jira-cli search "project = PROJ" --limit 100 --order-by updated
```

### Sprint 管理

```bash
jira-cli sprint list --board 42
jira-cli sprint active --board 42
jira-cli sprint create --board 42 --name "Sprint 5" --start-date 2024-02-01 --end-date 2024-02-14
jira-cli sprint move --sprint 10 --issues PROJ-123,PROJ-124
jira-cli sprint close --sprint 10 --force
```

### Board 和 Backlog

```bash
jira-cli board list
jira-cli board backlog --board 42
jira-cli board epics --board 42
```

### 项目、用户和过滤器

```bash
jira-cli project list
jira-cli project versions PROJ --unreleased
jira-cli project fields --custom              # 列出自定义字段
jira-cli user search --query "john"
jira-cli user me
jira-cli filter list
jira-cli filter run <filterId>
```

## JSON 输出

所有命令支持 `--json` 获取机器可读输出。默认使用**扁平格式**（token 效率高，适合 AI Agent）：

```bash
# 扁平 JSON（默认）—— 最小字段，低 token 开销
jira-cli issue get PROJ-123 --json
jira-cli search "project = PROJ" --json | jq '.issues[].key'

# 只输出需要的字段
jira-cli issue get PROJ-123 --json --fields key,summary,status,assignee

# 原始 Jira API 响应（完整嵌套结构）
jira-cli issue get PROJ-123 --json --raw

# 干净输出（抑制所有非 JSON 噪声）
jira-cli issue get PROJ-123 --json --quiet

# 预览写操作（不实际执行）
jira-cli issue delete PROJ-123 --dry-run --json
```

错误响应包含机器可读的错误码和可操作提示：

```json
{
  "error": "Jira API error 404: Issue does not exist",
  "statusCode": 404,
  "errorCode": "NOT_FOUND",
  "hint": "Verify the issue key exists and you have permission to view it"
}
```

设置 `NO_COLOR=1` 禁用彩色输出（适用于 CI/CD 环境）。

运行 `jira-cli reference` 获取所有命令和标志的结构化列表。

## 环境变量

| 变量 | 说明 |
|------|------|
| `JIRA_HOST` | Jira Data Center 主机 URL（覆盖配置文件） |
| `JIRA_TOKEN` | Personal Access Token（覆盖配置文件） |
| `NO_COLOR` | 设置任意值禁用彩色输出（[no-color.org](https://no-color.org)） |
| `JIRA_CLI_USER_AGENT` | 自定义 HTTP User-Agent |
| `JIRA_NO_AUDIT` | 设为 `1` 禁用审计日志 |
| `JIRA_AUDIT_RETENTION_MONTHS` | 自动删除超过 N 个月的审计文件（默认 `3`，`0` = 永久保留） |

## 全局标志

| 标志 | 说明 |
|------|------|
| `--json` | 以 JSON 格式输出（默认扁平格式；用 `--raw` 获取完整 Jira 响应） |
| `--force` | 跳过交互式确认提示 |
| `--quiet` | 抑制非 JSON 标准输出（适用于脚本和 AI Agent） |
| `--dry-run` | 显示将要执行的操作但不实际执行（仅写命令） |

### 命令级标志

| 标志 | 适用命令 | 说明 |
|------|----------|------|
| `--raw` | `issue get`、`issue list`、`search`、`sprint list`、`sprint issues`、`sprint active` | 返回原始 Jira API 响应而非扁平格式 |
| `--fields` | `issue get`、`issue list`、`sprint list`、`sprint issues` | 只输出指定字段（如 `--fields key,summary,status`） |

## 配置文件

凭据存储在 `~/.jira-cli/config.json`（权限：0600）：

```json
{
  "host": "https://jira.company.com",
  "token": "your-personal-access-token"
}
```

## 故障排除

| 问题 | 解决方案 |
|------|---------|
| 找不到配置 | 运行 `jira-cli login` 或设置 `JIRA_HOST` 和 `JIRA_TOKEN` 环境变量 |
| 认证失败 | 在 Jira DC 个人资料设置中重新生成 PAT |
| 权限不足 | 检查 PAT 权限范围和项目权限 |
| 资源未找到 | 确认 Issue Key 或项目 Key 是否存在 |
| 限流（429） | CLI 会自动重试；如持续出现请降低请求频率 |
| Host 必须以 https:// 开头 | 确保主机 URL 包含 `https://` 协议 |

## 安全

- 凭据本地存储在 `~/.jira-cli/config.json`，文件权限 `0600`（仅用户可读）
- 配置目录权限 `0700`
- `jira-cli login` 时 PAT 输入隐藏（使用终端安全输入）
- 所有通信使用 HTTPS（host 必须以 `https://` 开头）
- 凭据不会被记录或传输给第三方
- 环境变量 `JIRA_HOST` 和 `JIRA_TOKEN` 优先于配置文件

> **AI Agent 注意事项：** 此工具可被 AI Agent 调用以自动化 Jira 操作。使用 `--force` 跳过交互式确认，使用 `--json` 获取结构化输出。设置 `JIRA_HOST` 和 `JIRA_TOKEN` 环境变量进行非交互式认证。

安全漏洞请见 [SECURITY.md](SECURITY.md)（勿在公开 issue 中披露未公开漏洞）。

## 审计日志

所有写操作命令（create、edit、delete、transition、assign、comment 等）自动记录到 `~/.jira-cli/audit/`，JSONL 格式，按月分文件。

```bash
# 查看今日审计日志
cat ~/.jira-cli/audit/audit-2026-05.jsonl

# 每条记录格式：
# {"ts":"2026-05-03T14:22:01+08:00","cmd":"issue edit","args":["issue","edit","PROJ-123","--summary","new"],"exit":0,"ms":2031}
```

### 配置

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `JIRA_NO_AUDIT` | （未设置） | 设为 `1` 完全禁用审计日志 |
| `JIRA_AUDIT_RETENTION_MONTHS` | `3` | 自动删除超过 N 个月的审计文件。设为 `0` 永久保留。 |

清理在每次写命令时惰性执行——无需后台进程或定时任务。

## E2E 集成测试

完整的 E2E 测试脚本覆盖 **全部 jira-cli 命令**（55+ 操作），针对真实 Jira Data Center 实例运行。

### 快速开始

```bash
# 只读模式（安全——不修改任何数据）
export JIRA_HOST=https://jira.company.com
export JIRA_TOKEN=<your-pat>
export JIRA_E2E_MUTATE=0
pwsh ./scripts/e2e-full.ps1
```

### 完整测试（会创建和删除测试 Issue、过滤器）

```bash
pwsh ./scripts/e2e-full.ps1
```

### 包含 Sprint 写操作测试

```bash
export JIRA_E2E_SPRINT=1
pwsh ./scripts/e2e-full.ps1
```

脚本输出：
- 终端汇总（PASS / FAIL / SKIP 计数）
- `scripts/e2e-report.csv` — 机器可读结果，可用于 CI 仪表板

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JIRA_HOST` | 必填 | Jira DC 主机 URL |
| `JIRA_TOKEN` | 必填 | Personal Access Token |
| `JIRA_CLI_BIN` | `jira-cli` | 可执行文件路径 |
| `JIRA_E2E_PROJECT` | 自动检测 | 指定项目 key |
| `JIRA_E2E_MUTATE` | `1` | 设为 `0` 仅测试读命令 |
| `JIRA_E2E_SPRINT` | `0` | 设为 `1` 启用 Sprint 写操作测试 |
| `JIRA_E2E_CLEANUP` | `1` | 设为 `0` 保留测试资源 |

## 项目结构

```
jira-cli/
├── cmd/
│   ├── jira-cli/
│   │   └── main.go          # 入口（语义化退出码）
│   ├── root.go              # 根命令、全局标志、错误处理
│   ├── login.go             # 认证（仅 PAT，支持非交互式）
│   ├── doctor.go            # 诊断
│   ├── issue.go             # Issue CRUD
│   ├── issue_*.go           # Issue 子命令
│   ├── flatten.go           # 扁平 JSON 输出助手（Issue、Sprint）
│   ├── reference.go         # 自描述命令参考
│   ├── sprint.go            # Sprint 管理
│   ├── board.go             # Board 操作
│   ├── project.go           # 项目管理
│   ├── search.go            # JQL 搜索
│   ├── user.go              # 用户操作
│   ├── filter.go            # 保存的过滤器
│   └── epic.go              # Epic 操作
├── internal/
│   ├── api/                 # Jira REST API v2 客户端 + 类型定义
│   ├── audit/               # 写操作审计日志（JSONL）
│   ├── config/              # 配置文件 + 环境变量管理
│   └── output/              # 输出格式化（表格、颜色、扁平类型）
├── scripts/
│   ├── install.js           # npm postinstall（下载二进制）
│   ├── run.js               # npm bin 包装器
│   └── e2e-full.ps1         # 完整 E2E 集成测试（所有命令）
├── skills/                  # AI Agent 技能
├── package.json             # npm 分发
├── .goreleaser.yml          # 发布自动化
├── Makefile                 # 本地开发
└── .github/workflows/       # CI/CD
```

## 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md)。版本记录见 [CHANGELOG.md](CHANGELOG.md)。

## 许可证

MIT © fatecannotbealtered
