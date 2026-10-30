# SDK 接口参考

本页列出 Python 与 JavaScript / TypeScript 的方法。直接发送 HTTP 请求或实现其他语言客户端，请阅读[API 参考](../api/index.md)。

## 使用入口

| 入口 | 适用场景 |
| --- | --- |
| Python `Sandbox` | 同步创建或连接单个沙箱 |
| Python `AsyncSandbox` | 异步程序，网络方法使用 `await` |
| Python / JS `DevBox` | 复用客户端，查询模板、管理多个沙箱 |
| JS `Sandbox` | 网络方法返回 Promise，使用 `await` |

`DevBox` 负责管理入口，`Sandbox` 代表具体沙箱。关闭 `DevBox` 不会代替关闭已获取的 Sandbox 对象。

以下按资源列出 SDK 方法。Python 使用关键字参数，JavaScript / TypeScript 使用 options 对象；JS 网络方法需 `await`，Python 异步接口与同步接口结构一致。完整类型以 SDK 的类型提示为准。

## 沙箱

`client.sandboxes` 管理多个沙箱；创建或连接后得到的 `sandbox` 用于操作一个沙箱。

| 方法 | 主要参数 | 返回与用途 |
| --- | --- | --- |
| `client.sandboxes.create()` | `template`、`timeout`、`build_id` / `buildId`、`envs`、`metadata`、`lifecycle` | 创建沙箱，返回 `Sandbox` |
| `client.sandboxes.connect()` | `sandbox_id` / `sandboxId`、`timeout=300` | 获取访问凭证，必要时恢复沙箱，返回 `Sandbox` |
| `client.sandboxes.get()` | `sandbox_id` / `sandboxId` | 返回最新 `SandboxInfo` |
| `client.sandboxes.list()` | `template`、`states`、`metadata`、`limit`、`next_token` / `nextToken` | 返回 `Page<SandboxInfo>`，包含列表和分页信息 |
| `sandbox.get_info()` / `getInfo()` | 无 | 查询最新 `SandboxInfo`；`sandbox.info` 为本地已获取的信息 |
| `sandbox.is_running()` / `isRunning()` | 无 | 返回是否处于运行状态；不存在时返回 false |
| `sandbox.set_timeout()` / `setTimeout()` | `timeout` | 从现在起重设剩余存活秒数，可延长或缩短 |
| `sandbox.refresh()` | `duration=300` | 保证至少剩余指定秒数，不缩短现有期限 |
| `sandbox.pause()` | 无 | 保存状态并暂停，等待完成后返回 |
| `sandbox.resume()` | `timeout=300` | 恢复同一沙箱，返回 `Sandbox` |
| `sandbox.kill()` | 无 | 销毁沙箱并关闭本地连接；返回本次是否执行了销毁请求 |
| `sandbox.close()` | 无 | 释放本地连接，不销毁远端沙箱 |

列表的 `template` 接受模板 ID 或别名；`limit` 支持 1–1000，`0` 或省略表示不限制。

::: code-group
```python [Python]
page = client.sandboxes.list(template="default", limit=1000)
```

```js [JavaScript / TypeScript]
const page = await client.sandboxes.list({ template: "default", limit: 1000 });
```
:::

`Sandbox.create()` 和 `Sandbox.connect()` 是无需先创建 `DevBox` 的快捷入口，支持相同的业务参数及连接配置。这里的 `connect()` 用于获取沙箱访问入口，与[命令的 connect](#命令与进程) 按 PID 订阅输出不同。

### 创建参数

| Python / JS 参数 | 用途 |
| --- | --- |
| `template` | 模板 ID 或别名，默认 `default` |
| `timeout` | 存活时间，默认 300 秒 |
| `build_id` / `buildId` | 固定模板构建版本 |
| `envs` | 沙箱环境变量，字符串键值 |
| `metadata` | 沙箱业务标签，字符串键值 |
| `lifecycle` | 到期销毁或暂停，是否允许自动恢复 |

沙箱存活时间支持 10～86400 秒。`lifecycle.on_timeout` / `onTimeout` 为 `"kill"` 或 `"pause"`；`auto_resume` / `autoResume` 仅在到期暂停时启用。运行中的 `connect(timeout=...)` 只延长、不缩短期限；恢复时从恢复成功开始计算新期限。

示例与行为说明见[快速入门](../index.md#创建并使用沙箱)和[生命周期](../guide/lifecycle.md)。

## 模板

入口：`client.templates`。

| 方法 | 参数 | 返回 |
| --- | --- | --- |
| `list()` | 无 | `TemplateInfo` 列表，包含可选模板、Build 和资源规格 |

将模板 ID 或别名传给 `sandboxes.create(template)`；需要固定版本时同时指定 `build_id` / `buildId`。见[选择模板](../guide/templates.md)。

## 命令与进程

入口：`sandbox.commands`。

| Python / JS 方法 | 主要参数 | 返回与用途 |
| --- | --- | --- |
| `run()` | `command`、`background`、`cwd`、`envs`、`stdin`、`timeout` / `timeoutMs`、输出回调、`check` | 前台返回 `CommandResult`；后台返回 `CommandHandle` |
| `connect()` | `pid`、`timeout` / `timeoutMs` | 返回已有进程的 `CommandHandle`，订阅后续输出 |
| `list()` | 无 | `ProcessInfo` 列表 |
| `send_stdin()` / `sendStdin()` | `pid`、`data` | 向进程写入标准输入 |
| `close_stdin()` / `closeStdin()` | `pid` | 关闭标准输入，让进程收到输入结束 |
| `send_signal()` / `sendSignal()` | `pid`、`signal` | 发送 `SIGTERM` 或 `SIGKILL` |

命令与 PTY 共用进程句柄：

| 句柄方法 | 参数 | 返回与用途 |
| --- | --- | --- |
| `wait()` | 输出回调、`check` | 等待退出并返回 `CommandResult`，包含 stdout、stderr、退出码 |
| `send_stdin()` / `sendStdin()` | `data` | 向该进程输入 |
| `close_stdin()` / `closeStdin()` | 无 | 关闭该进程的输入流 |
| `send_signal()` / `sendSignal()` | `signal` | 发送终止或强制结束信号 |
| `kill()` | 无 | 向该进程发送 `SIGKILL` |
| `disconnect()` | 无 | 断开本地输出流，不终止远端进程 |

Python 回调为 `on_stdout`、`on_stderr`，JS 为 `onStdout`、`onStderr`。后台模式的回调传给 `handle.wait()`。重连不回放断开期间的输出。见[命令与进程示例](../guide/commands.md)。

命令和终端均支持 `cwd`，目录须存在且执行用户有访问权限；当前不支持 `user` 切换，显式传入会报错。

## 文件

入口：`sandbox.files`。`path` 指沙箱内的路径，只有上传和下载包含本地路径。

| Python / JS 方法 | 主要参数 | 返回与用途 |
| --- | --- | --- |
| `read()` | `path`、`encoding` | 文件文本，默认 UTF-8 |
| `read_bytes()` / `readBytes()` | `path` | Python `bytes` / JS `Uint8Array` |
| `write()` | `path`、`data`、`encoding` | 写入文本或二进制，返回 `FileInfo` |
| `write_batch()` / `writeBatch()` | 路径与内容的映射、`encoding` | 按顺序写入多个文件，返回 `FileInfo` 列表；不保证整批原子性 |
| `list()` | `path`、`depth=1` | 文件与目录的 `FileInfo` 列表；当前服务只列一层 |
| `stat()` | `path` | 一个路径的 `FileInfo` |
| `exists()` | `path` | 路径是否存在 |
| `make_dir()` / `makeDir()` | `path` | 创建目录 |
| `move()` | `source`、`destination` | 移动或重命名 |
| `remove()` | `path` | 删除文件或目录 |
| `upload()` | `local_path` / `localPath`、`remote_path` / `remotePath` | 上传本地文件，返回 `FileInfo` |
| `download()` | `remote_path` / `remotePath`、`local_path` / `localPath` | 下载到本地，返回 Python `Path` / JS 路径字符串 |
| `watch()` | `path`、`recursive=false`、`include_entry` / `includeEntry` | 持续产生 `FileWatchEvent` 的事件流 |

`FileInfo` 包含名称、路径、类型、大小、权限等元数据。监听结束时关闭 Python 生成器或退出 JS 的异步循环。见[文件操作示例](../guide/files.md)。

## 终端

入口：`sandbox.pty`。

| 方法 | 主要参数 | 返回与用途 |
| --- | --- | --- |
| `start()` | `command="/bin/bash"`、`size`、`envs`、`cwd` | 创建终端并返回 `CommandHandle` |
| `connect()` | `pid`、`timeout` / `timeoutMs` | 重新订阅已有终端，返回 `CommandHandle` |
| `resize()` | `pid`、`size` | 修改终端的 `rows`、`cols` |

通过句柄输入、读取输出、发送信号和断开连接。终端输出可能包含 ANSI 控制序列。见[交互式终端](../guide/terminal-and-git.md)。

## Git

入口：`sandbox.git`。仓库和工作目录位于沙箱内，方法返回 `CommandResult`。

| Python / JS 方法 | 主要参数 | 用途 |
| --- | --- | --- |
| `clone()` | `url`、`path`、`branch`、`depth`、`credentials`、`timeout` / `timeoutMs` | 克隆仓库 |
| `status()` | `repository` | 获取工作区状态 |
| `checkout()` | `repository`、`reference`、`create=false` | 切换分支或创建分支 |
| `add()` | `repository`、`paths="."` | 暂存文件 |
| `commit()` | `repository`、`message` | 提交暂存内容 |
| `pull()` / `push()` | `repository`、`remote="origin"`、`branch`、`credentials`、`timeout` / `timeoutMs` | 拉取或推送 |
| `set_config()` / `setConfig()` | `repository`、`key`、`value` | 设置仓库级 Git 配置 |

沙箱模板需要安装 Git，并能访问仓库地址。见[Git 示例](../guide/terminal-and-git.md)。

## 配置

| 环境变量 | 用途 |
| --- | --- |
| `DEVBOX_API_KEY` | API Key，必填 |
| `DEVBOX_API_URL` | 服务地址，默认 `https://devbox.developer.myhuaweicloud.com` |

Python 也可用 `api_key`、`api_url` 构造参数；JS 对应 `apiKey`、`apiUrl`。显式参数优先于环境变量。通常只需配置 API Key 与服务地址，沙箱连接凭证由 SDK 管理。

创建或连接沙箱时，Manager 返回完整数据面地址 `domain`。SDK 直接使用该地址，不要求用户配置集群名、区域或网关域名。`DEVBOX_GATEWAY_URL`（或 `gateway_url` / `gatewayUrl` 参数）仅用于显式覆盖调试，会替代服务端返回的地址，正常使用不要设置。

API Key 用于沙箱管理和模板查询请求，服务据此确定用户归属。执行命令、访问文件等沙箱操作由 SDK 使用连接凭证认证，无需用户再次传入 API Key。

本地示例的 `run_local.py` / `run-local.mjs` 可通过 `DEVBOX_GATEWAY_IP` 临时覆盖 EnvD 地址的 DNS 解析；不需要同时设置 `DEVBOX_GATEWAY_URL`，原请求域名和 TLS 校验仍然保留。该变量仅由本地示例入口读取。

使用 HTTPS 并保留证书校验。不要把 API Key 或沙箱连接凭证写入日志、浏览器地址栏或公开仓库。

## 时间单位

| 配置 | Python | JavaScript / TypeScript |
| --- | --- | --- |
| 沙箱存活时间 | 秒 | 秒 |
| `refresh` 保证的剩余时间 | 秒 | 秒 |
| 命令等待 | `timeout`，秒，默认 60 | `timeoutMs`，毫秒，默认 60000 |
| Git clone / pull / push 等待 | 秒，默认 300 | 毫秒，默认 300000 |
| 网络请求等待 | `request_timeout`，秒 | `requestTimeoutMs`，毫秒 |

默认管理请求等待 150 秒，普通沙箱请求等待 30 秒。PTY 和目录监听允许持续读取，但连接建立仍受网络超时限制。调整请求超时不会延长沙箱存活时间，也不能覆盖服务入口的超时限制。

## 返回与异常

命令返回 stdout、stderr 和退出码；普通文本不由 SDK 自动打印。实时输出使用回调。

异常统一继承 `DevBoxError`，包含错误码、HTTP 状态码、请求 ID 与可读信息：

::: code-group
```python [Python]
from devbox import DevBoxError, Sandbox

try:
    sandbox = Sandbox.connect("<sandbox-id>")
except DevBoxError as error:
    print(error.code, error.status_code, error.request_id, error.message)
else:
    sandbox.close()
```

```js [JavaScript / TypeScript]
import { DevBoxError, Sandbox } from "devbox-js-sdk";

try {
  const sandbox = await Sandbox.connect("<sandbox-id>");
  await sandbox.close();
} catch (error) {
  if (!(error instanceof DevBoxError)) throw error;
  console.log(error.code, error.statusCode, error.requestId, error.message);
}
```
:::
