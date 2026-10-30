# devbox CLI

`devbox` 是 DevBox 沙箱管理服务的端侧命令行客户端，命令风格参考 E2B CLI，覆盖沙箱全生命周期与模板的管理，支持交互式终端与远程命令执行。

## 功能特性

- 基于 `cobra` 的命令框架，子命令支持简写（`sbx`/`tpl`/`cn`/`ls` 等）
- 中英文双语提示，`DEVBOX_LANG=zh` 切换中文，默认英文
- 配置按优先级解析：命令行参数 > 环境变量 > 配置文件 > 构建期默认值
- API Key 优先存 OS keyring，不可用时降级本地配置文件
- 支持 `-f/--format pretty|json` 两种输出格式
- 入参校验：sandboxID 格式（36 位小写/数字/中划线）、timeout/duration 范围（10-86400 秒）
- Go >= 1.26

## 环境要求

- Go 1.26 或以上

## 构建

```bash
cd cli
make build VERSION=v1.0.0   # 输出到 bin/devbox，版本号构建期注入
# 或手动注入版本号
go build -trimpath -ldflags "-X github.com/huaweicloud/devspace-devbox/cli/cmd.Version=v1.0.0" -o devbox.exe ./cmd/devbox
```

## 使用

### 认证

```bash
devbox auth status                                  # 查看登录状态（含用户名）
devbox auth login --api-key <你的API密钥>           # 校验并保存 API Key
devbox auth login                                   # 复用本地 Key（环境变量 > keyring/配置文件），否则浏览器登录
devbox auth logout                                  # 清除 API Key 与用户信息
```

登录凭据解析优先级：命令行 flag > `HW_DEVBOX_API_KEY` 环境变量 > keyring/配置文件。API Key 的校验地址为 `{AuthCheckDomain}/v1/api-keys/check`（构建期注入）。

### 沙箱管理

```bash
devbox sbx create <templateID>                      # 创建并进入交互式终端
devbox sbx create <templateID> -d                   # 只创建不进终端，打印沙箱 ID
devbox sbx create <templateID> -c /root -e FOO=bar -t 600 --lifecycle.ontimeout pause --lifecycle.autoresume
devbox sbx ls                                       # 列出沙箱
devbox sbx ls -l 50 -s running,paused -m "user=abc"
devbox sbx ls -f json                               # 输出原始 API 数据
devbox sbx in <sandboxID>                           # 查看沙箱详情
devbox sbx kl <sandboxID>                           # 终止沙箱
devbox sbx ps <sandboxID>                           # 暂停沙箱
devbox sbx rs <sandboxID> -t 600                    # 恢复沙箱
devbox sbx cn <sandboxID> -c /root -e FOO=bar       # 进入交互式终端
devbox sbx to <sandboxID> -t 600                    # 设置沙箱 TTL
devbox sbx rf <sandboxID> -d 300                    # 延长沙箱存活时间
devbox sbx ex <sandboxID> <command...>              # 执行一次性命令
devbox sbx ex <sandboxID> <command> -b              # 后台执行，打印 PID
```

说明：
- `create`/`connect` 默认进入交互式终端，本地终端成为沙箱 shell，输入 `exit` 退出
- `create`/`connect` 的 `-e` 都是终端会话的环境变量（对齐 e2b：不持久化到沙箱，`exit` 后消失）
- `-t/-d` 非必传：不传用默认值 300，传了做范围校验（10-86400 秒，显式 0 报错）
- sandboxID 必须为 36 位小写字母、数字和中划线

### 模板

```bash
devbox tpl list                                     # 列出模板
devbox tpl list -f json
```

## 项目结构

```text
cli/
├── cmd/
│   ├── devbox/main.go    # 入口 main
│   └── *.go              # cobra 命令树（auth/sandbox/template）
├── internal/
│   ├── api/              # 类型化客户端（client.gen.go 手维护）+ 指针工具
│   ├── api/proto/        # envd process/terminal API 的 proto 与生成代码
│   ├── auth/             # 登录：keyring+配置存储、API Key 校验、浏览器回调
│   ├── client/           # 端侧封装：鉴权、错误解码、envd 数据面客户端
│   ├── config/           # 配置解析与持久化
│   ├── i18n/             # 中英文消息（DEVBOX_LANG）
│   ├── output/           # pretty/json 渲染与展示工具
│   ├── parse/            # key=value 参数解析
│   └── term/             # 交互式终端：raw mode、双向 pump、resize
├── Makefile
└── openapi.yaml          # 已移除；客户端 client.gen.go 手维护
```

## 开发验证

```bash
make fmt
make vet
make lint    # 需要 golangci-lint
make test
```

## 许可证

Apache-2.0
