# devbox CLI

`devbox` 是 DevBox 沙箱管理服务的端侧命令行客户端，命令风格参考 E2B CLI。它覆盖沙箱全生命周期、模板与构建、快照以及计算节点的管理，API 定义见 `openapi.yaml`。

## 功能特性

- 基于 `cobra` 的命令框架，支持自动补全（`devbox completion`）
- API 客户端由 `openapi.yaml` 通过 `oapi-codegen` 自动生成，与接口契约保持同步
- 配置按优先级解析：命令行参数 > 环境变量 > 配置文件 > 默认值
- 支持 `text`（表格/键值）与 `json` 两种输出格式
- Go >= 1.26

## 环境要求

- Go 1.26 或以上

## 构建

```bash
cd packages/cli
make build            # 输出到 bin/devbox
# 或直接
go build ./cmd/devbox
```

## 使用

### 配置

配置来源优先级：

1. `--server` / `--api-key` 命令行参数
2. `DEVBOX_SERVER_URL` / `DEVBOX_API_KEY` / `DEVBOX_LOGIN_URL` 环境变量
3. `~/.devbox/config.json` 配置文件
4. 默认服务器 `http://localhost:8080`

```bash
# 写入配置文件
devbox config set server http://localhost:8080
devbox config set api-key <你的API密钥>
devbox config set login-url <登录页地址>   # 浏览器登录用
devbox config get
devbox config unset api_key
```

### 登录

`auth` 命令参考 E2B / devbridge。API Key 优先存在 OS keyring，不可用时降级写入本地配置文件。

```bash
devbox auth status                                  # 查看登录状态
devbox auth login --api-key <你的API密钥>           # 校验并保存 API Key
devbox auth login                                   # 复用本地 Key 校验，否则浏览器登录
devbox auth login --login-url https://host/login    # 手动指定登录页
devbox auth logout
```

登录流程：本地已有 API Key → 调用校验接口验证（成功即登录成功，失败提示失效）；无本地 Key → 检测浏览器，有则自动打开登录页，无则给出登录页链接，登录页把 API Key 回传到本地回调端口，校验通过后存入 keyring（降级配置文件）。

### 沙箱管理

```bash
devbox sandbox list
devbox sandbox list --state running --limit 20 --output json
devbox sandbox create <templateID> --timeout 300 --env PYTHONUNBUFFERED=1
devbox sandbox get <sandboxID>
devbox sandbox kill <sandboxID>
devbox sandbox logs <sandboxID> --limit 500 --level INFO
devbox sandbox metrics <sandboxID>
devbox sandbox pause <sandboxID>
devbox sandbox resume <sandboxID>
devbox sandbox connect <sandboxID> --timeout 120
devbox sandbox set-timeout <sandboxID> 600
devbox sandbox refresh <sandboxID> 300
devbox sandbox fork <sandboxID> --count 3
devbox sandbox network <sandboxID> --deny-out 10.0.0.0/8
devbox sandbox snapshot <sandboxID> --name nightly
```

### 模板与构建

```bash
devbox template list
devbox template create --alias base --name python --vcpu 2 --ram-mb 2048 --total-disk-mb 10240
devbox template get <templateID>
devbox template update <templateID> --name "Python 3.12"
devbox template delete <templateID>
devbox template build start <templateID> <buildID>
devbox template build status <templateID> <buildID>
devbox template build logs <templateID> <buildID>
devbox template set-tags --target <templateID>@<buildID> --tag stable
```

### 快照与节点

```bash
devbox snapshot list --sandbox-id <sandboxID>
devbox snapshot create <sandboxID> --name snapshot-name
devbox node list
devbox node get <nodeID>
devbox node update <nodeID> --status draining
```

任何命令都可追加 `--output json` 输出原始 API 数据，便于脚本集成。

## 项目结构

```text
packages/cli/
├── openapi.yaml          # API 契约（来源）
├── cmd/
│   ├── devbox/main.go    # 入口 main
│   └── *.go              # cobra 命令树（auth/config/health/sandbox/snapshot/node/...）
├── internal/
│   ├── api/              # oapi-codegen 生成的类型化客户端（勿手改）
│   ├── auth/             # 登录：keyring+配置存储、API Key 校验、浏览器回调
│   ├── client/           # 端侧封装：鉴权、错误解码、分页
│   ├── config/           # viper 配置解析与持久化
│   └── output/           # text/json 渲染
├── Makefile
└── docs/ (待补充)
```

## 代码生成

API 客户端由 `openapi.yaml` 生成，接口变更后重新生成即可：

```bash
make generate     # 等价于 go generate ./...
```

生成代码位于 `internal/api/client.gen.go`，请勿手工修改。

## 开发验证

```bash
make fmt
make vet
make lint    # 需要 golangci-lint
make test
```

## 许可证

Apache-2.0