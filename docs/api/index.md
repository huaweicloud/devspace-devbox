# API 通用约定

DevBox API 分为沙箱管理与沙箱内操作。各资源页面列出真实 HTTP 路径、请求字段和返回值，可用于实现任意语言的客户端。

| 资源 | 请求地址 | 鉴权 |
| --- | --- | --- |
| [沙箱](./sandboxes.md)、[模板](./templates.md) | 服务提供的 API 地址 | `X-API-Key` |
| [进程与终端](./processes.md)、[文件系统](./filesystem.md) | 创建或连接响应中的 `domain` | `Cookie: relay_token=<connectToken>` |

## 地址与鉴权 {#authentication}

管理请求使用 HTTPS。以下示例中的变量由使用者配置：

```bash
export DEVBOX_API_URL="https://<服务访问域名>"
export DEVBOX_API_KEY="<devbox scope 的 API Key>"
```

API Key 对应的 Namespace 决定沙箱归属，调用时不需要传入 Namespace。

创建沙箱或调用 [connect](./sandboxes.md#connect) 后，从响应中取得 `sandboxID`、`domain`、`connectToken`：

```bash
export SANDBOX_ID="<sandboxID>"
export DEVBOX_DOMAIN="<domain 返回的完整 HTTPS 地址>"
export DEVBOX_CONNECT_TOKEN="<connectToken>"
```

沙箱内请求使用以下请求头；后续示例均沿用这些变量：

```http
Cookie: relay_token=<connectToken>
E2B-Sandbox-Id: <sandboxID>
E2B-Sandbox-Port: 49983
```

使用响应中的完整地址 `envdUrl`，无需拼接域名。`tokenExpiration` 是凭证的 Unix 到期秒数；临近到期时调用 connect 获取新凭证，resume 后也使用返回的最新地址与凭证。访问沙箱内的应用见[应用端口](./sandboxes.md#application-port)。

启用 `lifecycle.autoResume` 时，对暂停沙箱的访问可触发恢复，首个请求需要等待。客户端应给恢复请求留出足够的等待时间。请求断开不意味着沙箱、进程或文件写入操作已撤销。

## 数据格式 {#format}

- 管理接口使用 JSON，字段名严格按接口定义，例如 `templateID`、`envVars`；SDK 的 `template`、`envs` 是语言封装后的名称。
- 存活时间、refresh 的时长使用秒；`startedAt`、`endAt` 使用 RFC 3339 时间，示例以 UTC 的 `Z` 表示。
- 文件上传、下载直接传输字节。ConnectRPC 的 `bytes` 字段在 JSON 中使用 Base64，64 位整数字段通常使用十进制字符串。
- JSON 示例为结构示意，ID、时间、资源规格以实际响应为准。Protobuf JSON 可能省略值为零、false 或空集合的字段，客户端按字段默认值处理。

## ConnectRPC 单次请求 {#connect-unary}

进程控制和文件元数据接口采用 ConnectRPC JSON。请求为普通 JSON，使用 POST，并附加：

```http
Content-Type: application/json
Connect-Protocol-Version: 1
```

例如查询沙箱内的进程：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/process.Process/List" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{}'
```

成功返回 HTTP 200 与 JSON。错误响应格式见[错误处理](#errors)。

## ConnectRPC 流式请求 {#connect-stream}

`process.Process/Start`、`process.Process/Connect` 与 `filesystem.Filesystem/WatchDir` 返回服务端流。它们使用 POST，不能将整个响应直接当作一个 JSON 对象解析。

```http
Content-Type: application/connect+json
Connect-Protocol-Version: 1
Connect-Timeout-Ms: 90000
```

`Connect-Timeout-Ms` 可省略，单位毫秒，限制本次 RPC 的等待时间，不改变沙箱存活时间。目录长期监听通常省略该头；网络读取需要容纳空闲及心跳时间。

请求体和响应体均使用以下消息帧。资源页面中的 JSON 表示**帧内消息**：

| 部分 | 长度 | 内容 |
| --- | --- | --- |
| 标志 | 1 字节 | `0x00` 为未压缩消息；响应 `0x02` 为结束帧 |
| 消息长度 | 4 字节 | 大端无符号整数，表示后面 JSON 的 UTF-8 字节数 |
| 消息体 | 上述长度 | UTF-8 JSON |

服务端流式方法的请求体是一帧普通消息。响应是一串消息帧，最后是一帧结束信息，例如 `{}` 或 `{"error":{"code":"internal","message":"..."}}`。

HTTP 读取块与消息帧没有一一对应关系：客户端先读满 5 字节帧头，再按长度读取消息体。HTTP 200 后仍可能在结束帧中出现错误；缺少结束帧的意外断开不能视为成功。本文示例不协商压缩。

<details>
<summary>Go 流式请求示例：启动命令并读取消息帧</summary>

仅使用 Go 标准库。设置上面的三个沙箱变量，保存为 `main.go` 后运行 `go run main.go`。输出为原始事件 JSON，其中 stdout/stderr 等字节字段需要 Base64 解码，见[进程事件](./processes.md#events)。

```go
package main

import (
    "bytes"
    "context"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"
)

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    payload := []byte(`{"process":{"cmd":"/bin/bash","args":["-lc","printf hello"]},"stdin":false}`)
    frame := make([]byte, 5+len(payload))
    binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
    copy(frame[5:], payload)

    ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
    defer cancel()
    endpoint := strings.TrimRight(os.Getenv("DEVBOX_DOMAIN"), "/") + "/process.Process/Start"
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(frame))
    if err != nil {
        return err
    }
    req.Header.Set("Cookie", "relay_token="+os.Getenv("DEVBOX_CONNECT_TOKEN"))
    req.Header.Set("E2B-Sandbox-Id", os.Getenv("SANDBOX_ID"))
    req.Header.Set("E2B-Sandbox-Port", "49983")
    req.Header.Set("Content-Type", "application/connect+json")
    req.Header.Set("Connect-Protocol-Version", "1")
    req.Header.Set("Connect-Timeout-Ms", "90000")
    client := &http.Client{
        CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
    }
    defer client.CloseIdleConnections()
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("request returned HTTP %d", resp.StatusCode)
    }
    if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/connect+json") {
        return fmt.Errorf("unexpected stream content type")
    }
    for {
        var header [5]byte
        if _, err := io.ReadFull(resp.Body, header[:]); err != nil {
            return fmt.Errorf("stream ended without a complete trailer: %w", err)
        }
        size := binary.BigEndian.Uint32(header[1:])
        if size > 16<<20 {
            return fmt.Errorf("message exceeds example's 16 MiB limit")
        }
        message := make([]byte, size)
        if _, err := io.ReadFull(resp.Body, message); err != nil {
            return err
        }
        switch header[0] {
        case 0:
            if !json.Valid(message) {
                return fmt.Errorf("invalid event JSON")
            }
            fmt.Println(string(message))
        case 2:
            var trailer struct {
                Error *struct{ Code, Message string }
            }
            if err := json.Unmarshal(message, &trailer); err != nil {
                return err
            }
            if trailer.Error != nil {
                return fmt.Errorf("%s: %s", trailer.Error.Code, trailer.Error.Message)
            }
            return nil
        default:
            return fmt.Errorf("unsupported frame flags: %d", header[0])
        }
    }
}
```

</details>

## 错误处理 {#errors}

管理接口错误返回 HTTP 状态码和如下 JSON，`error` 是业务错误码：

```json
{"error":"not_found","message":"Sandbox not found"}
```

| HTTP 状态 | 含义 |
| --- | --- |
| 400 | 参数、分页或请求格式错误 |
| 401 / 403 | 凭证无效或无权访问 |
| 404 | 资源不存在、已删除或已到期 |
| 409 | 当前沙箱状态不允许此操作 |
| 426 | 客户端版本低于服务端支持门槛，`error=upgrade_required`；升级客户端后重试 |
| 429 | 请求限流、运行配额或隧道配额耗尽 |
| 500 / 502 / 503 | 服务处理失败或依赖不可用 |

保存响应中的 `X-Request-ID` 便于排查。限流响应如含 `Retry-After`，按其等待秒数处理；资源配额耗尽需先释放不用的沙箱或隧道。

DevBox 客户端通过 `User-Agent` 携带版本标记，例如 `devbox-cli/1.2.0`、`devbox-python-sdk/0.1.0` 或 `devbox-js-sdk/0.1.0`。服务端启用对应最低版本要求后，低于该版本的请求返回 426；重复发送同一请求不能解决，需要升级客户端。自行实现的 API 调用不要求伪装成 DevBox CLI 或 SDK。

ConnectRPC 错误使用 `{"code":"not_found","message":"..."}`；流式错误位于结束帧的 `error` 字段。文件 HTTP 接口的错误可能为 `{"error":"file not found"}`。Gateway 也可能在请求到达沙箱之前返回 401、403、404 或 503，客户端先检查 HTTP 状态，再按响应类型解析。

连接超时或流断开后，先查询沙箱状态或按 PID 重连。创建、命令启动与写入可能已经生效，不能无条件重放请求。
