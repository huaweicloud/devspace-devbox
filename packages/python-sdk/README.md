# DevBox Python SDK

用于创建和操作云端隔离沙箱的 Python SDK。Python 版本要求为 3.10 或更高。
安装包名称是 `devbox-sdk`，代码中的导入名称是 `devbox`。

## 使用模型

日常使用只需要记住一个入口和四组能力：

```text
Sandbox
├── commands    命令与进程
├── files       文件系统
├── pty         交互式终端
└── git         Git 操作
```

推荐从 `Sandbox.create()` 开始。只有需要复用连接管理多个沙箱时，才使用
`DevBox` 客户端；异步程序使用 `AsyncSandbox`。

## 安装

从项目源码安装时，先在项目目录创建独立虚拟环境。

Linux 或 macOS：

```bash
python3 -m venv .venv
.venv/bin/python -m pip install -e .
```

Windows CMD：

```bat
py -3.12 -m venv .venv
.venv\Scripts\python.exe -m pip install -e .
```

发布到包仓库后也可以直接安装：

```bash
python -m pip install devbox-sdk
```

## 快速开始

设置管理面签发的 API Key：

```bash
export DEVBOX_API_KEY=devbox_xxx
```

Windows CMD 使用：

```bat
set "DEVBOX_API_KEY=devbox_xxx"
```

创建沙箱、执行命令并操作文件：

```python
from devbox import Sandbox

with Sandbox.create("default", timeout=300) as sandbox:
    result = sandbox.commands.run("printf 'hello from DevBox'")
    print(result.stdout)

    sandbox.files.write("/tmp/message.txt", "hello from the SDK")
    print(sandbox.files.read("/tmp/message.txt"))
```

离开 `with` 代码块时，SDK 会删除临时沙箱并关闭本地连接。需要保留沙箱时不要使用
`with`，完成操作后调用 `sandbox.close()`；需要立即删除时调用 `sandbox.kill()`。
`kill()` 在沙箱已经删除或自然过期时返回 `False`，可以用于重复清理。

## 核心方法

| 对象 | 方法 | 用途 |
| --- | --- | --- |
| `client.sandboxes` | `create`、`connect`、`get`、`list`、`metrics` | 批量管理沙箱 |
| `Sandbox` | `create`、`connect`、`get_info`、`is_running`、`set_timeout`、`refresh`、`kill`、`close` | 沙箱生命周期 |
| `sandbox.commands` | `run`、`connect`、`list`、`send_stdin`、`close_stdin`、`send_signal` | 命令与进程 |
| `sandbox.files` | `read`、`write`、`list`、`stat`、`make_dir`、`move`、`remove` | 远端文件操作 |
| `sandbox.files` | `upload`、`download`、`watch` | 本地传输和目录监听 |
| `sandbox.pty` | `start`、`connect`、`resize` | 交互式终端 |
| `sandbox.git` | `clone`、`status`、`checkout`、`add`、`commit`、`pull`、`push`、`set_config` | Git 工作流 |

沙箱生命周期、命令和 Git 操作中的 `timeout`、`duration` 均以秒为单位。
`set_timeout(300)` 和 `refresh(300)` 均把沙箱截止时间重设为当前时间后 300 秒，
不是在原截止时间上累加。要修改剩余时间，请显式调用这两个方法，不要依赖 `connect(timeout=...)` 续期。
`is_running()` 查询管理面状态，不是数据面探活；过期清理期间，状态可能短暂滞后。

PyCharm 会根据这些对象和类型标注提供点号补全。例如创建目录使用
`sandbox.files.make_dir()`，而不是 `sandbox.make_dir()`。

## 后台命令

前台命令直接返回 `CommandResult`：

```python
result = sandbox.commands.run("python --version")
print(result.exit_code, result.stdout, result.stderr)
```

命令使用沙箱配置的默认系统用户。当前 EnvD 不支持切换用户，显式传入 `user` 会抛出
`ConfigurationError`，不会静默以默认用户执行。

后台命令返回 `CommandHandle`，可以继续输入、发送信号或等待结果：

```python
process = sandbox.commands.run("cat", background=True, stdin=True)
process.send_stdin("hello\n")
process.close_stdin()
result = process.wait()
```

`process.disconnect()` 只断开本地输出流，不终止远端进程。之后可以通过
`sandbox.commands.connect(process.pid)` 重新连接。重连只接收重新建立订阅后的新输出，
不会回放断开期间产生的输出。

`sandbox.files.watch()` 返回长期事件流。完成监听后必须关闭生成器，释放数据面连接：

```python
from contextlib import closing

with closing(sandbox.files.watch("/tmp/workspace")) as events:
    for event in events:
        print(event)
        if event.name == "done.txt":
            break
```

## 交互式终端

```python
from devbox import PtySize

session = sandbox.pty.start(size=PtySize(rows=30, cols=100))
session.send_stdin("pwd\n")
sandbox.pty.resize(session.pid, PtySize(rows=40, cols=120))
session.send_stdin("exit\n")
result = session.wait(check=False)
print(result.stdout)
```

PTY 继承沙箱的语言环境，不强制设置镜像可能未安装的 locale；需要覆盖时通过 `envs` 传入。
`exit` 结束远端终端进程，`disconnect()` 只断开输出流。`wait()` 已返回退出结果后，句柄不再接受输入。

## 管理多个沙箱

`DevBox` 复用一个管理面连接，适合服务端程序或批量操作：

```python
from devbox import DevBox

with DevBox() as client:
    page = client.sandboxes.list()
    sandbox = client.sandboxes.create("default")
    print(sandbox.sandbox_id, len(page.items))
    sandbox.kill()
```

## 异步调用

异步 API 与同步 API 的对象结构保持一致：

```python
import asyncio

from devbox import AsyncSandbox


async def main() -> None:
    async with await AsyncSandbox.create("default") as sandbox:
        result = await sandbox.commands.run("uname -a")
        print(result.stdout)


asyncio.run(main())
```

## 配置

| 配置 | 环境变量 | 默认值 |
| --- | --- | --- |
| API Key | `DEVBOX_API_KEY` 或 `E2B_API_KEY` | 必填 |
| 管理面地址 | `DEVBOX_API_URL` 或 `E2B_API_URL` | `https://devbox.developer.myhuaweicloud.com` |
| 数据面地址覆盖 | `DEVBOX_GATEWAY_URL` | Manager 返回的地址 |
| 请求超时 | 构造参数 `request_timeout` | 30 秒 |

构造参数优先于环境变量。API Key 只发送给管理面；Manager 返回的 EnvD 访问令牌只发送给
数据面。SDK 不持久化 API Key、EnvD 访问令牌或 Connect Token。

Manager 未直接返回数据面地址的部署可以配置 URL 模板，例如
`https://{tunnel_id}-{port}.cn-north-4-bridge.myhuaweicloud.com`。

Manager 的 `connectToken` 是 Relay 连接凭证，`tokenExpiration` 是其 Unix 秒过期时间；
`tunnelExpiration` 是隧道自身的过期时间，两者独立。SDK 在内部保留这些字段，不放入沙箱公开信息或日志。
`connect()` 获取 Manager 当前保存的连接信息，不保证签发新 Token。SDK 当前没有后台续期或
Token 过期自动重连机制；长期运行前需要服务端先明确凭证更新和数据面鉴权契约。

数据面使用 EnvD 的 `/process.Process/*`、`/filesystem.Filesystem/*` 和 `/files`，
不使用仅供 Orchestrator 调用的 `/envd/*` 控制接口。
当前已对齐的 Manager 源码仍返回占位 `envdAccessToken`；SDK 沿用 `X-Access-Token`，
尚未将 `connectToken` 用于 HTTP 鉴权，需确认 Relay Gateway 的 Header 契约后完成。

SDK 仅重试连接建立失败，不自动重试限流、服务端错误或可能已经到达服务端的写操作。

## 错误处理

所有 SDK 异常都继承自 `DevBoxError`：

```python
from devbox import DevBoxError, Sandbox

try:
    sandbox = Sandbox.create("default")
except DevBoxError as error:
    print(error.code, error.status_code, error.request_id, error.message)
```

常用异常包括鉴权错误、参数错误、资源不存在、冲突、限流、超时、服务不可用和协议错误。
非零命令退出码默认抛出 `CommandExitError`；传入 `check=False` 可以直接检查返回结果。

## 示例与验证

| 文件 | 用途 |
| --- | --- |
| `examples/basic.py` | 推荐的同步入门路径 |
| `examples/async_basic.py` | 异步入门路径 |
| `examples/demo.py` | 无交互的核心能力演示 |
| `examples/validate_full.py` | 创建临时沙箱并执行完整验收 |

核心能力演示：

```bash
python examples/demo.py
```

全量验收覆盖生命周期、命令、文件传输、PTY 和本地 Git 工作流，并在结束时删除测试沙箱：

```bash
python examples/validate_full.py
```

`DEVBOX_GATEWAY_URL` 使用固定地址时，Full Test 会先检查该地址是否可用，
并标明 Manager 与固定数据面的独立验证模式；这不证明新建沙箱与该数据面属于同一实例。

本地环境尚未配置通配符 DNS 时，可设置 `DEVBOX_GATEWAY_IP` 为 Gateway IP，
`DEVBOX_GATEWAY_URL` 为 `https://{tunnel_id}-{port}.<gateway-domain>`，通过以下入口运行：

```bash
python examples/run_local.py validate_full
```

该入口也支持 `basic`、`async_basic`、`demo`。DNS 覆盖仅对当前示例进程和配置的
Gateway 域名生效，保留 HTTPS Host、SNI 和证书校验，不修改系统 hosts。
Gateway 必须能查询到 Manager 新建的 tunnel；IP 映射不会创建或恢复 tunnel。

## 开发验证

```bash
python -m pip install -e ".[dev]"
ruff check .
ruff format --check .
mypy
pytest
python -m build
```

公开 API 由项目直接维护，HTTP、TLS 和 ConnectRPC 协议细节保留在内部模块中，避免协议
实现影响用户侧调用方式。
