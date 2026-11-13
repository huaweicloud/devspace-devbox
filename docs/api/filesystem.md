# 文件系统 API

请求发送到沙箱 `domain`，使用[连接凭证](./index.md#authentication)。文件内容采用普通 HTTP，目录和元数据使用 ConnectRPC。

| 操作 | 方法与路径 | 类型 |
| --- | --- | --- |
| [读取文件](#read) | `GET /files` | HTTP 字节流 |
| [写入文件](#write) | `POST /files` | HTTP 字节流或 multipart |
| [文件信息](#stat) | `POST /filesystem.Filesystem/Stat` | 单次请求 |
| [目录列表](#list) | `POST /filesystem.Filesystem/ListDir` | 单次请求 |
| [创建目录](#mkdir) | `POST /filesystem.Filesystem/MakeDir` | 单次请求 |
| [移动](#move) | `POST /filesystem.Filesystem/Move` | 单次请求 |
| [删除](#remove) | `POST /filesystem.Filesystem/Remove` | 单次请求 |
| [监听目录](#watch) | `POST /filesystem.Filesystem/WatchDir` | 服务端流 |

所有路径均指沙箱内路径，示例使用绝对路径。返回不存在错误时不要仅检查 HTTP 状态，还需区分 Gateway 路由错误与文件本身不存在，见[错误处理](./index.md#errors)。

## 读取文件 {#read}

`GET /files?path=<path>`

必填 Query 参数 `path`，类型为字符串。成功返回 HTTP 200，`Content-Type: application/octet-stream`，响应体为原文件字节：

```bash
curl --fail-with-body --get "$DEVBOX_DOMAIN/files" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  --data-urlencode 'path=/tmp/message.txt' \
  --output message.txt
```

不存在返回 404，路径为目录返回 400。下载与读取使用同一接口，文本解码由客户端处理。

## 写入文件 {#write}

`POST /files?path=<path>`

必填 Query 参数 `path`，Content-Type 为 `application/octet-stream`，Body 为原始字节。自动创建父目录，覆盖同路径文件：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/files?path=/tmp/message.txt" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Content-Type: application/octet-stream' \
  --data-binary @message.txt
```

也支持 `multipart/form-data` 的 `file` 字段。单文件示例保持相同的 path：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/files?path=/tmp/message.txt" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -F 'file=@message.txt'
```

成功返回 HTTP 200 和 JSON 数组，即使只写入一个文件：

```json
[{"name":"message.txt","path":"/tmp/message.txt","size":5,"type":"file"}]
```

此处 `size` 为数字字节数，`type` 为 `file`。更多元数据通过 Stat 获取。缺少 path 返回 400，不支持的 Content-Type 返回 415。

## 查询文件信息 {#stat}

`POST /filesystem.Filesystem/Stat`

必填字符串 `path`。以下是 ConnectRPC 单次请求：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/filesystem.Filesystem/Stat" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"path":"/tmp/message.txt"}'
```

成功返回 HTTP 200 和 `{"entry": ...}`，entry 为[文件信息对象](#entry)。

## 列出目录 {#list}

`POST /filesystem.Filesystem/ListDir`

必填字符串 `path`。`depth` 为整数，当前服务只列一层，使用 1：

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/filesystem.Filesystem/ListDir" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"path":"/tmp","depth":1}'
```

成功返回 HTTP 200 和 `{"entries":[...]}`，每项为[文件信息对象](#entry)。Protobuf JSON 可能省略空的 entries，客户端视为空列表。

## 创建目录 {#mkdir}

`POST /filesystem.Filesystem/MakeDir`

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/filesystem.Filesystem/MakeDir" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"path":"/tmp/example"}'
```

Body 必填字符串 `path`。成功返回 HTTP 200 和对象，包含可选的 `entry`。

## 移动或重命名 {#move}

`POST /filesystem.Filesystem/Move`

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/filesystem.Filesystem/Move" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"source":"/tmp/message.txt","destination":"/tmp/example/message.txt"}'
```

Body 必填字符串 `source` 和 `destination`。成功返回 HTTP 200，包含可选的目标 `entry`。

## 删除文件或目录 {#remove}

`POST /filesystem.Filesystem/Remove`

```bash
curl --fail-with-body "$DEVBOX_DOMAIN/filesystem.Filesystem/Remove" \
  -H "Cookie: relay_token=$DEVBOX_CONNECT_TOKEN" \
  -H "E2B-Sandbox-Id: $SANDBOX_ID" \
  -H 'E2B-Sandbox-Port: 49983' \
  -H 'Connect-Protocol-Version: 1' \
  -H 'Content-Type: application/json' \
  --data '{"path":"/tmp/example"}'
```

Body 必填字符串 `path`。成功返回 HTTP 200 和 `{}`。

## 监听目录 {#watch}

`POST /filesystem.Filesystem/WatchDir`

使用[ConnectRPC 服务端流](./index.md#connect-stream)。请求帧内 JSON：

```json
{"path":"/tmp/example","recursive":false,"includeEntry":true}
```

| Body 字段 | 类型 | 说明 |
| --- | --- | --- |
| `path` | string | 必填，监听的目录 |
| `recursive` | boolean | 是否监听子目录，默认 false |
| `includeEntry` | boolean | 是否附带文件元数据，默认 false；SDK 默认传 true |

成功返回 HTTP 200，先发 start，之后发 filesystem 或 keepalive 消息：

```json
{"start":{}}
```

```json
{"filesystem":{"name":"message.txt","type":"EVENT_TYPE_CREATE","entry":{"name":"message.txt","path":"/tmp/example/message.txt","type":"FILE_TYPE_FILE","size":"5"}}}
```

type 为 `EVENT_TYPE_CREATE`、`EVENT_TYPE_WRITE`、`EVENT_TYPE_REMOVE`、`EVENT_TYPE_RENAME` 或 `EVENT_TYPE_CHMOD`。entry 为可选[文件信息](#entry)，删除事件等情况下可能没有 entry。

客户端关闭流后释放本次监听；需要长时间监听时，持续读取并处理心跳。沙箱暂停或连接断开后需重新监听，不保证补发断开期间的变化。

## 文件信息对象 {#entry}

```json
{
  "name": "message.txt",
  "path": "/tmp/message.txt",
  "type": "FILE_TYPE_FILE",
  "size": "5",
  "mode": 420,
  "permissions": "-rw-r--r--",
  "owner": "root",
  "group": "root",
  "modifiedTime": "2026-10-21T08:00:00Z"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `name` / `path` | string | 名称和沙箱内路径 |
| `type` | string | `FILE_TYPE_FILE`、`FILE_TYPE_DIRECTORY` 或 `FILE_TYPE_SYMLINK` |
| `size` | string | 文件字节数，Protobuf int64 的 JSON 表示 |
| `mode` | integer | 文件权限位的十进制值，例如 420 对应八进制 0644 |
| `permissions` | string | 可读权限表示 |
| `owner` / `group` | string | 所属用户和组 |
| `modifiedTime` | string | 可选 RFC 3339 修改时间 |
| `symlinkTarget` | string | 可选符号链接目标 |

SDK 的 `exists()` 通过 Stat 判断，`read()`/`read_bytes()` 与 upload/download 使用 `/files`，`write_batch()` 依次执行多次写入。它们不对应额外的服务端接口。
