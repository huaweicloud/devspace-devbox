# DevBox Python SDK

用于创建和操作云端隔离沙箱的 Python SDK。Python 版本要求为 3.10 或更高。
安装包名称是 `devbox-sdk`，代码中的导入名称是 `devbox`。

## 使用模型

日常使用只需要记住一个入口和五组能力：

```text
Sandbox
├── commands    命令与进程
├── files       文件系统
├── pty         交互式终端
├── git         Git 操作
└── envd        运行时信息
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
| `client.sandboxes` | `create`、`connect`、`get`、`list` | 批量管理沙箱 |
| `Sandbox` | `create`、`connect`、`get_info`、`is_running`、`set_timeout`、`refresh`、`pause`、`resume`、`kill`、`close` | 沙箱生命周期 |
| `sandbox.commands` | `run`、`connect`、`list`、`send_stdin`、`close_stdin`、`send_signal` | 命令与进程 |
| `sandbox.files` | `read`、`write`、`list`、`stat`、`make_dir`、`move`、`remove` | 远端文件操作 |
| `sandbox.files` | `upload`、`download`、`watch` | 本地传输和目录监听 |
| `sandbox.pty` | `start`、`connect`、`resize` | 交互式终端 |
| `sandbox.git` | `clone`、`status`、`checkout`、`add`、`commit`、`pull`、`push`、`set_config` | Git 工作流 |
| `sandbox.envd` | `get_metrics`、`get_envs`、`health` | EnvD 实时指标、环境变量与健康检查 |

沙箱生命周期、命令和 Git 操作中的 `timeout`、`duration` 均以秒为单位。
Manager 请求默认等待 150 秒，数据面普通 HTTP 请求默认等待 30 秒；显式设置 `request_timeout` 时，两者都使用指定值。命令执行/重连默认 `timeout=60`，
Git 的 clone/pull/push 默认 `timeout=300`。PTY 和目录监听不设流读取超时，但建立连接、发送请求和
等待连接池仍受 `request_timeout` 约束。HTTPX 的超时按网络阶段计算，
不是整个操作的总耗时上限；命令超时还会通过 Connect 协议传递给服务端。
`DevBox.close()` 只关闭管理面连接；通过它获取的 Sandbox 需要分别 `close()`。
`set_timeout(300)` 重设剩余 300 秒，可以缩短；`refresh(300)` 保证至少剩余 300 秒，只延长、不累加。
`connect(timeout=300)` 从 Manager 获取 Gateway 连接凭证，本身不建立数据面连接；对运行中的沙箱只延长，对暂停的沙箱先执行恢复。
`is_running()` 查询管理面状态，不是数据面探活；过期清理期间，状态可能短暂滞后。

`pause()` 保留内存和磁盘并关闭当前数据面连接；`resume(timeout=300)` 恢复同一沙箱并用响应中的新 `connectToken` 覆盖暂停前的 Token，返回当前对象。
Manager 不自动过期清理暂停记录，目前仅支持原节点恢复；实际恢复仍依赖节点、快照和隧道可用。暂停后要保留沙箱，应调用 `close()`，不要退出会自动 `kill()` 的 Sandbox 上下文。
已有命令/PTY/监听句柄应在暂停前断开，恢复后按 PID 重新连接或重新建立监听，SDK 不自动重放命令。
SDK 的生命周期时长参数接受 1–86400 秒。创建时可传
`lifecycle=SandboxLifecycle(on_timeout="pause", auto_resume=True)`，让超时行为改为内存快照暂停。
`auto_resume=True` 允许兼容的 Gateway 在首次数据面请求到达时唤醒沙箱；显式 `connect()`/`resume()` 始终可用。
SDK 会在 `connectToken` 到期前通过 Manager 静默更新连接凭证，不延长沙箱生命周期，用户无需管理 Token。
暂停保存完整内存和磁盘状态。创建时传入 `network={"allow_public_traffic": False}`，可要求访问应用端口时携带连接凭证；省略时允许匿名访问应用端口。
请求超时不改变沙箱生存时间，也不能覆盖服务器入口代理的超时限制。

部署新版 Manager 后，使用现有连接环境变量运行生命周期专项验证：

```bash
python examples/validate_full.py --mode lifecycle
```

PyCharm 会根据这些对象和类型标注提供点号补全。例如创建目录使用
`sandbox.files.make_dir()`，而不是 `sandbox.make_dir()`。

## 后台命令

前台命令直接返回 `CommandResult`：

```python
result = sandbox.commands.run("python --version")
print(result.exit_code, result.stdout, result.stderr)
```

默认使用沙箱配置的系统用户。使用支持用户切换的 EnvD 模板时，可以指定沙箱内已有的用户：

```python
result = sandbox.commands.run("id -un", user="nobody", cwd="/tmp")
terminal = sandbox.pty.start(user="nobody", cwd="/tmp")
```

`user` 不创建用户；工作目录需要该用户有访问权限。旧 EnvD 可能忽略身份选择，不要在未经验证的模板上依赖该参数隔离权限。

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

## EnvD 运行时信息

```python
health = sandbox.envd.health()
metrics = sandbox.envd.get_metrics()
envs = sandbox.envd.get_envs()

print(health.envd_version, metrics.cpu_used_pct, envs["DEVBOX_ID"])
```

`get_metrics()` 返回一次实时采样：CPU 核数与使用率、CPU hotplug 状态、
内存总量/使用量/缓存、磁盘总量/使用量。
`get_envs()` 返回 EnvD 当前合并后的环境变量。`health()` 直接探测数据面，成功时返回
`X-Envd-Version`；这些接口与 Manager 的 `get_info()`/`is_running()` 不同，不会查询管理面状态。

## 管理多个沙箱

`DevBox` 复用一个管理面连接，适合服务端程序或批量操作：

```python
from devbox import DevBox

with DevBox() as client:
    template = client.templates.list()[0]
    page = client.sandboxes.list()
    sandbox = client.sandboxes.create(template.template_id)
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
| 请求超时 | 构造参数 `request_timeout` | Manager 150 秒，数据面 30 秒；显式指定时统一使用该值 |

构造参数优先于环境变量。API Key 只发送给管理面；Manager 返回的 `connectToken` 只发送给
数据面，使用 `Cookie: relay_token=<connectToken>`。SDK 不持久化凭证，也不会跟随重定向发送凭证。

Manager 未直接返回数据面地址的部署可以配置 URL 模板，例如
`https://{tunnel_id}-{port}.cn-north-4-bridge.myhuaweicloud.com`。

Manager 的 `connectToken` 是 Relay 连接凭证，`tokenExpiration` 是其 Unix 秒过期时间；
`tunnelExpiration` 是隧道自身的过期时间，两者独立。SDK 在内部保留这些字段，不放入沙箱公开信息或日志。
`connect()` 获取 Manager 当前保存的连接信息，不保证每次签发新 Token。SDK 在发起数据面请求前，
会检查凭证有效期并在需要时向 Manager 更新；不运行后台定时续期，也不延长沙箱生命周期。

数据面使用 EnvD 的 `/process.Process/*`、`/filesystem.Filesystem/*`、`/files`、
`/metrics`、`/envs` 和 `/health`，
不使用仅供 Orchestrator 调用的 `/envd/*` 控制接口。
所有数据面请求（包括流式命令和文件传输）均使用 Relay Token Cookie 鉴权，
不使用 `envdAccessToken` 或 `X-Access-Token`。Manager 未返回有效 `connectToken` 时，SDK 拒绝发起数据面请求。

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
| `examples/validate_full.py` | 讲解、验收和串行批量测试 |
| `examples/run_local.py` | 本地 DNS 覆盖运行工具 |

完整验收按以下顺序执行，结束时删除测试沙箱：

1. 查询模板列表，显示配置并选择具体 build；`DEVBOX_TEST_TEMPLATE` 支持名称、ID 或别名。
2. 创建、查询、列表、连接、环境变量，以及命令、流式回调、后台进程、stdin、进程重连与终止。
3. EnvD 数据面健康检查、实时资源指标和运行时环境变量。
4. 文件读写、元数据、二进制传输、目录监听、PTY 和本地 Git 工作流。
5. 验证 `set_timeout` 可延长和缩短，`refresh` 只延长、不缩短。
6. 手动 pause/resume、通过 Manager connect 恢复，以及超时暂停后直接访问数据面触发 Gateway 自动唤醒。
7. 每次恢复核对同一沙箱中的 1 MiB 文件、原后台进程和环境变量；另建短生命周期沙箱验证超时销毁。

```bash
python examples/validate_full.py
python examples/validate_full.py --mode basic
python examples/validate_full.py --mode lifecycle --rounds 5
```

`basic` 验证管理查询、EnvD 运行时信息、命令、文件、PTY 和 Git；`lifecycle` 验证生命周期；
`full` 执行全部。同步与异步最小用法见本文前面的示例。

控制台中的 `DO` 表示 SDK 调用，`STREAM`/`EVENT` 展示实时数据，`CHECK` 展示业务核对，
`WAIT`/`STATE` 展示异步状态流转，最后以 `PASS`/`FAIL` 和汇总收尾。展示命令实际 stdout、
stderr、退出码和耗时，不打印 API Key 或 Token；PTY 控制字符只在展示时过滤，SDK 原始返回值不变。
失败不会被重试覆盖，相关生命周期步骤会停止，清理仍执行；任一验证失败时退出码为 1。

完整验收必须访问 Manager 新建的沙箱，不接受独立的固定 Gateway 地址。
不指定 `DEVBOX_GATEWAY_URL` 时使用 Manager 返回的地址；覆盖地址时必须包含 `{tunnel_id}`。
自动唤醒需要环境中已部署 Gateway 到 Manager 的内部恢复链路，不会跳过未接通的功能。

本地环境尚未配置通配符 DNS 时，可设置 `DEVBOX_GATEWAY_IP` 为 Gateway IP，
仍使用 Manager 返回的域名，通过以下入口运行：

```bash
python examples/run_local.py --mode lifecycle
```

该入口接受相同的 `--mode`、`--rounds` 参数。DNS 覆盖仅对当前示例进程中
首段以 `-49983` 结尾的域名生效，保留 HTTPS Host、SNI 和证书校验，不修改系统 hosts。
Gateway 必须能查询到 Manager 新建的 tunnel；IP 映射不会创建或恢复 tunnel。
本机设置了 HTTP 代理时，将测试域名加入 `NO_PROXY`，否则请求会交给代理解析，绕过此 DNS 覆盖。

仅在数据面证书尚未配置好的临时调试环境，可设置
`DEVBOX_GATEWAY_SKIP_TLS_VERIFY=true`，相当于对 Gateway 请求使用 `curl -k`。
此开关同时适用于 Python 同步和异步接口，不影响 Manager 的证书校验；默认关闭，
开启时会提示警告。它不能修复 DNS、连接超时或接口错误，且无法保证服务端身份，
存在 Token 和沙箱数据被截获的风险，不应在生产使用。调试完成后删除该变量。

`--rounds` 串行重复所选流程，输出每项成功/失败数和 min/avg/max 耗时。
创建或清理失败后停止新增资源；其他失败保留在汇总中，不以重试覆盖。
生命周期测试分别验证新连接和复用 SDK 句柄自动唤醒，恢复后读取原文件而非重新写入。

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
