# 进程与终端 API

请求发送到沙箱 `domain`，使用[连接凭证](./index.md#authentication)。接口前缀为 `/process.Process`，HTTP 方法均为 POST。

| 操作 | 路径 | 类型 |
| --- | --- | --- |
| [启动](#start) | `/process.Process/Start` | 服务端流 |
| [连接已有进程](#connect) | `/process.Process/Connect` | 服务端流 |
| [列表](#list) | `/process.Process/List` | 单次请求 |
| [输入](#input) | `/process.Process/SendInput` | 单次请求 |
| [关闭标准输入](#stdin) | `/process.Process/CloseStdin` | 单次请求 |
| [发送信号](#signal) | `/process.Process/SendSignal` | 单次请求 |
| [调整终端尺寸](#resize) | `/process.Process/Update` | 单次请求 |

单次请求使用 `application/json`；流式请求使用 `application/connect+json` 消息帧，见[协议与 Go 示例](./index.md#connect-stream)。下面流式接口的 JSON 为帧内消息。

## 启动进程 {#start}

`POST /process.Process/Start`

默认使用沙箱配置的用户。可选请求头 `Authorization: Basic <base64(username:)>` 用于指定已有沙箱用户（密码为空）；仍需单独提供连接凭证。Python 和 JS SDK 的命令执行和 PTY 支持 `user` 参数，需要配套 EnvD 版本。

| Body 字段 | 类型 | 说明 |
| --- | --- | --- |
| `process.cmd` | string | 必填，可执行程序；执行 Shell 命令时指定 `/bin/bash` |
| `process.args` | string[] | 程序参数；Shell 命令可使用 `["-lc", "命令"]` |
| `process.envs` | object | 环境变量，值为字符串 |
| `process.cwd` | string | 可选工作目录 |
| `stdin` | boolean | 标准输入配置，普通前台调用通常传 false；需要交互时传 true |
| `pty.size.rows` / `cols` | integer | 创建 PTY 时指定终端行列数 |

普通命令的帧内请求：

```json
{"process":{"cmd":"/bin/bash","args":["-lc","printf hello"],"cwd":"/tmp"},"stdin":false}
```

成功返回 HTTP 200 和[进程事件流](#events)。首个 start 事件提供 PID，之后可继续读取输出，也可用 PID 调用其他进程接口。程序启动失败时按实际 RPC 错误处理；进程本身退出码非零不等于 HTTP 请求失败。

### 创建 PTY

PTY 使用同一个 Start 接口，增加 `pty` 字段：

```json
{
  "process": {"cmd":"/bin/bash","args":["-i","-l"],"envs":{"TERM":"xterm-256color"}},
  "stdin": true,
  "pty": {"size":{"rows":24,"cols":80}}
}
```

PTY 输出在 `data.pty` 中，可能包含 ANSI 控制序列。调用 SendInput 的 `input.pty` 输入文本，调用 Update 调整窗口。`exit` 是发送给 Shell 的命令；断开 HTTP 流只关闭本次订阅，不等于退出 Shell。

## 连接已有进程 {#connect}

`POST /process.Process/Connect`

必填 `process.pid`，类型为正整数。使用 Start 返回的 PID：

```json
{"process":{"pid":123}}
```

成功返回 HTTP 200 和[进程事件流](#events)，首个 start 事件确认 PID。订阅后续输出，不保证回放断开期间的内容；进程不存在时返回 `not_found`。

这是按 PID 连接进程，与 [Manager connect](./sandboxes.md#connect) 获取沙箱凭证是两个接口。暂停恢复后也需重新建立输出订阅。

## 列出进程 {#list}

`POST /process.Process/List`

Body 为 `{}`。完整 cURL 见[单次请求示例](./index.md#connect-unary)。返回 HTTP 200：

```json
{"processes":[{"pid":123,"config":{"cmd":"/bin/bash","args":["-lc","sleep 60"]}}]}
```

`processes` 是 EnvD 管理的进程记录，不等同于系统 `ps` 的所有进程。每项包含 `pid` 和 `config`；此响应没有独立的 running 字段，不应仅凭列表项判断进程一定仍在运行。

## 写入输入 {#input}

`POST /process.Process/SendInput`

必填 `process.pid` 和 `input`。`input.stdin` / `input.pty` 为 Base64 字节串，二者只选择一个。下面的 `aGVsbG8K` 是 `hello\n`：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/process.Process/SendInput" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"process":{"pid":123},"input":{"stdin":"aGVsbG8K"}}'
```

成功返回 HTTP 200 和 `{}`。向 PTY 输入时将 `stdin` 改为 `pty`。

## 关闭标准输入 {#stdin}

`POST /process.Process/CloseStdin`

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/process.Process/CloseStdin" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"process":{"pid":123}}'
```

成功返回 HTTP 200 和 `{}`。让普通进程收到 EOF，例如 `cat` 可在消费完输入后退出；这不等同于发送终止信号。

## 发送信号 {#signal}

`POST /process.Process/SendSignal`

必填 `process.pid` 与 `signal`。支持 `SIGNAL_SIGTERM`、`SIGNAL_SIGKILL`：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/process.Process/SendSignal" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"process":{"pid":123},"signal":"SIGNAL_SIGTERM"}'
```

成功返回 HTTP 200 和 `{}`，表示信号已发送。进程何时退出、退出状态仍从事件流确认。SDK 的进程 `kill()` 使用 `SIGNAL_SIGKILL`。

## 调整终端尺寸 {#resize}

`POST /process.Process/Update`

必填 `process.pid`，修改终端时传入 `pty.size.rows` 与 `cols`，均为正整数：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/process.Process/Update" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"process":{"pid":123},"pty":{"size":{"rows":30,"cols":100}}}'
```

成功返回 HTTP 200 和 `{}`。

## 进程事件 {#events}

Start 与 Connect 使用相同的响应结构，每帧只有一种 event：

```json
{"event":{"start":{"pid":123}}}
```

```json
{"event":{"data":{"stdout":"aGVsbG8="}}}
```

```json
{"event":{"end":{"exitCode":0,"exited":true,"status":"exited"}}}
```

| 事件 | 字段 | 含义 |
| --- | --- | --- |
| `start` | `pid` | 进程 ID |
| `data` | `stdout`、`stderr` 或 `pty` | Base64 字节数据；解码后按输出顺序处理 |
| `end` | `exitCode`、`exited`、`status`，可选 `error` | 进程结束；正常退出时 status 为 exited，被信号终止时可为 signaled |
| `keepalive` | 空对象 | 流心跳 |

收到 `end` 后仍需检查 Connect 结束帧是否含协议错误。SDK 的 `wait()` 在本地收集这些事件；`disconnect()` 关闭当前流；Git 方法通过启动 Git 命令实现。它们均无额外的独立 HTTP 路径。
