# DevBox JavaScript SDK

用于创建和操作云端隔离沙箱的 TypeScript/JavaScript SDK。要求 Node.js 20.18.1 或更高版本。

## 使用模型

日常使用只需要一个入口和五组能力：

```text
Sandbox
├── commands    命令与进程
├── files       文件系统
├── pty         交互式终端
├── git         Git 操作
└── envd        运行时信息
```

所有网络操作均返回 `Promise`。SDK 自带 TypeScript 类型，不需要额外安装类型包。

## 安装

发布到包仓库后安装：

```bash
npm install devbox-js-sdk
```

从仓库源码安装：

```bash
npm install ./js-sdk
```

设置管理面签发的 API Key：

```bash
export DEVBOX_API_KEY=devbox_xxx
```

Windows CMD：

```bat
set "DEVBOX_API_KEY=devbox_xxx"
```

## 快速开始

```js
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default", { timeout: 300 });

try {
  const result = await sandbox.commands.run("printf 'hello from DevBox'");
  console.log(result.stdout);

  await sandbox.files.write("/tmp/message.txt", "hello from the SDK");
  console.log(await sandbox.files.read("/tmp/message.txt"));
} finally {
  await sandbox.kill();
  await sandbox.close();
}
```

`kill()` 删除远端沙箱，`close()` 只释放本地连接。需要保留沙箱时只调用 `close()`。
`kill()` 在沙箱已经删除或自然过期时返回 `false`，可以用于重复清理。

## 核心方法

| 对象 | 方法 | 用途 |
| --- | --- | --- |
| `client.sandboxes` | `create`、`connect`、`get`、`list` | 批量管理沙箱 |
| `Sandbox` | `create`、`connect`、`getInfo`、`isRunning`、`setTimeout`、`refresh`、`pause`、`resume`、`kill`、`close` | 沙箱生命周期 |
| `sandbox.commands` | `run`、`connect`、`list`、`sendStdin`、`closeStdin`、`sendSignal` | 命令与进程 |
| `sandbox.files` | `read`、`write`、`list`、`stat`、`makeDir`、`move`、`remove` | 远端文件 |
| `sandbox.files` | `upload`、`download`、`watch` | 本地传输与目录监听 |
| `sandbox.pty` | `start`、`connect`、`resize` | 交互式终端 |
| `sandbox.git` | `clone`、`status`、`checkout`、`add`、`commit`、`pull`、`push`、`setConfig` | Git 工作流 |
| `sandbox.envd` | `getMetrics`、`getEnvs`、`health` | EnvD 实时指标、环境变量与健康检查 |

沙箱生命周期的 `timeout`、`duration` 以秒为单位；命令、Git 和请求配置中的
`timeoutMs`、`requestTimeoutMs` 以毫秒为单位。
Manager 请求默认等待 150000 毫秒，数据面普通 HTTP 请求默认等待 30000 毫秒；显式设置 `requestTimeoutMs` 时，两者都使用指定值。命令执行/重连默认 `timeoutMs=60000`，
Git 的 clone/pull/push 默认 `timeoutMs=300000`。PTY 和目录监听不设总时限，完成交互后需关闭句柄或迭代器；
关闭 Sandbox 会中止它的数据面流。调用方传入的自定义 dispatcher 由调用方负责关闭。
`DevBox.close()` 只关闭管理面连接；通过它获取的 Sandbox 需要分别 `close()`。
`setTimeout(300)` 重设剩余 300 秒，可以缩短；`refresh(300)` 保证至少剩余 300 秒，只延长、不累加。
`connect(id, { timeout: 300 })` 从 Manager 获取 Gateway 连接凭证，本身不建立数据面连接；对运行中的沙箱只延长，对暂停的沙箱先执行恢复。
`isRunning()` 查询管理面状态，不是数据面探活；过期清理期间，状态可能短暂滞后。

`pause()` 保留内存和磁盘并关闭当前数据面连接；`resume({ timeout: 300 })` 恢复同一沙箱并用响应中的新 `connectToken` 覆盖暂停前的 Token，返回当前对象。
Manager 不自动过期清理暂停记录，目前仅支持原节点恢复；实际恢复仍依赖节点、快照和隧道可用。已有命令/PTY/监听句柄应在暂停前断开，恢复后按 PID 重新连接或重新建立监听，SDK 不自动重放命令。
SDK 的生命周期时长参数接受 1–86400 秒。创建时可传
`lifecycle: { onTimeout: "pause", autoResume: true }`，让超时行为改为内存快照暂停。
`autoResume: true` 允许兼容的 Gateway 在首次数据面请求到达时唤醒沙箱；显式 `connect()`/`resume()` 始终可用。
SDK 会在 `connectToken` 到期前通过 Manager 静默更新连接凭证，不延长沙箱生命周期，用户无需管理 Token。
暂停保存完整内存和磁盘状态。创建时传入 `network: { allowPublicTraffic: false }`，可要求访问应用端口时携带连接凭证；省略时允许匿名访问应用端口。
请求超时不改变沙箱生存时间，也不能覆盖服务器入口代理的超时限制。

部署新版 Manager 后，使用现有连接环境变量运行生命周期专项验证：

```bash
npm run build
node examples/validate-full.mjs --mode lifecycle
```

编辑器会根据类型声明为这些对象提供点号补全。例如创建目录使用
`sandbox.files.makeDir()`，而不是 `sandbox.makeDir()`。

## 后台命令

前台命令返回 `CommandResult`。后台命令返回 `CommandHandle`：

```js
const process = await sandbox.commands.run("cat", {
  background: true,
  stdin: true,
});

await process.sendStdin("hello\n");
await process.closeStdin();
const result = await process.wait();
```

`disconnect()` 只断开本地输出流，不终止远端进程。之后可通过
`sandbox.commands.connect(process.pid)` 重新连接；重连只接收新输出，不回放断开期间的内容。

非零退出码默认抛出 `CommandExitError`。使用 `{ check: false }` 可以直接读取退出结果。
默认使用沙箱配置的系统用户。使用支持用户切换的 EnvD 模板时，可以指定沙箱内已有的用户：

```js
const result = await sandbox.commands.run("id -un", { user: "nobody", cwd: "/tmp" });
const terminal = await sandbox.pty.start("/bin/bash", { user: "nobody", cwd: "/tmp" });
```

`user` 不创建用户；工作目录需要该用户有访问权限。旧 EnvD 可能忽略身份选择，不要在未经验证的模板上依赖该参数隔离权限。

## 文件监听

`watch()` 返回异步迭代器。退出循环时，SDK 会关闭数据面流：

```js
for await (const event of sandbox.files.watch("/tmp/workspace")) {
  console.log(event);
  if (event.name === "done.txt") break;
}
```

## 交互式终端

```js
const session = await sandbox.pty.start("/bin/bash", {
  size: { rows: 30, cols: 100 },
});

await session.sendStdin("pwd\n");
await sandbox.pty.resize(session.pid, { rows: 40, cols: 120 });
await session.sendStdin("exit\n");
const result = await session.wait({ check: false });
console.log(result.stdout);
```

PTY 继承沙箱的语言环境，不强制设置镜像可能未安装的 locale；需要覆盖时通过 `envs` 传入。
`exit` 结束远端终端进程，`disconnect()` 只断开输出流。`wait()` 已返回退出结果后，句柄不再接受输入。

## EnvD 运行时信息

```js
const health = await sandbox.envd.health();
const metrics = await sandbox.envd.getMetrics();
const envs = await sandbox.envd.getEnvs();

console.log(health.envdVersion, metrics.cpuUsedPct, envs.DEVBOX_ID);
```

`getMetrics()` 返回一次实时采样：CPU 核数与使用率、CPU hotplug 状态、
内存总量/使用量/缓存、磁盘总量/使用量。
`getEnvs()` 返回 EnvD 当前合并后的环境变量。`health()` 直接探测数据面，成功时返回
`X-Envd-Version`；这些接口与 Manager 的 `getInfo()`/`isRunning()` 不同，不会查询管理面状态。

## 管理多个沙箱

`DevBox` 复用管理面连接，适合服务端程序和批量操作：

```js
import { DevBox } from "devbox-js-sdk";

const client = new DevBox();
try {
  const [template] = await client.templates.list();
  const page = await client.sandboxes.list();
  const sandbox = await client.sandboxes.create(template.templateId);
  console.log(sandbox.sandboxId, page.total);
  await sandbox.kill();
} finally {
  await client.close();
}
```

## 配置

| 配置 | 环境变量 | 默认值 |
| --- | --- | --- |
| API Key | `DEVBOX_API_KEY` 或 `E2B_API_KEY` | 必填 |
| 管理面地址 | `DEVBOX_API_URL` 或 `E2B_API_URL` | `https://devbox.developer.myhuaweicloud.com` |
| 数据面地址覆盖 | `DEVBOX_GATEWAY_URL` | Manager 返回的地址 |
| 请求超时 | `requestTimeoutMs` | Manager 150000 毫秒，数据面 30000 毫秒；显式指定时统一使用该值 |

构造参数优先于环境变量。API Key 只发送给管理面，Manager 返回的 `connectToken` 只发送给数据面，使用 `Cookie: relay_token=<connectToken>`。SDK 不持久化凭证，也不会把凭证跟随重定向发送到其他地址。

Manager 未直接返回数据面地址的部署可以设置 URL 模板，例如 `DEVBOX_GATEWAY_URL=https://{tunnel_id}-{port}.cn-north-4-bridge.myhuaweicloud.com`。

Manager 的 `connectToken` 是 Relay 连接凭证，`tokenExpiration` 是其 Unix 秒过期时间；
`tunnelExpiration` 是隧道自身的过期时间，两者独立。SDK 在内部保留这些字段，不放入沙箱公开信息或日志。
`connect()` 获取 Manager 当前保存的连接信息，不保证每次签发新 Token。SDK 在发起数据面请求前，
会检查凭证有效期并在需要时向 Manager 更新；不运行后台定时续期，也不延长沙箱生命周期。

数据面使用 EnvD 的 `/process.Process/*`、`/filesystem.Filesystem/*`、`/files`、
`/metrics`、`/envs` 和 `/health`，
不使用仅供 Orchestrator 调用的 `/envd/*` 控制接口。
所有数据面请求（包括流式命令和文件传输）均使用 Relay Token Cookie 鉴权，
不使用 `envdAccessToken` 或 `X-Access-Token`。Manager 未返回有效 `connectToken` 时，SDK 拒绝发起数据面请求。

SDK 只对连接建立失败进行两次短间隔重试，不重试服务端错误、限流或可能已到达服务端的写操作。

## 错误处理

所有 SDK 异常均继承自 `DevBoxError`：

```js
import { DevBoxError, RateLimitError, Sandbox } from "devbox-js-sdk";

try {
  await Sandbox.create();
} catch (error) {
  if (error instanceof RateLimitError) console.log(error.retryAfter);
  if (error instanceof DevBoxError) {
    console.log(error.code, error.statusCode, error.requestId, error.message);
  }
}
```

## 示例与验证

| 入口 | 用途 |
| --- | --- |
| `examples/validate-full.mjs` | 讲解、验收和串行批量测试 |
| `examples/run-local.mjs` | 本地 DNS 覆盖运行工具 |

```bash
npm run example
npm run validate
npm run validate -- --mode lifecycle --rounds 5
```

`basic` 验证管理查询、EnvD 运行时信息、命令、文件、PTY 和 Git；`lifecycle` 验证生命周期；
默认 `full` 执行全部。最小用法见本文前面的示例。
`--rounds` 串行重复所选流程，输出每项失败数和 min/avg/max 耗时。
创建或清理失败后停止新增资源，不以重试覆盖失败。

完整验收顺序与 Python 一致：

1. 列出模板并按名称、ID 或别名选择具体 build，创建、查询、列表和连接沙箱。
2. 验证环境变量、EnvD 健康检查/实时指标/运行时环境变量、命令 stdout/stderr、流式回调、后台进程、stdin、重连和终止。
3. 验证文件读写、元数据、二进制传输、目录监听、PTY 和本地 Git 工作流。
4. 验证 `setTimeout` 延长与缩短、`refresh` 只延长，以及 pause/resume、Manager connect 恢复和 Gateway 自动唤醒。
5. 每次恢复后核对 1 MiB 文件、原进程和环境变量，另建短生命周期沙箱验证超时销毁，最后清理。

`DO` 展示调用，`STREAM`/`EVENT` 展示实时数据，`CHECK` 展示业务核对，`WAIT`/`STATE` 展示状态流转，
最后输出 `PASS`/`FAIL` 和汇总。打印实际命令输出、退出码和耗时，不打印 API Key 或 Token；
PTY 控制字符只在展示时过滤。失败保留为失败，相关生命周期步骤停止，清理仍执行，退出码为 1。

模板通过 `DEVBOX_TEST_TEMPLATE` 配置，支持名称、ID 或别名。完整验收不接受独立的固定 Gateway 地址；
默认使用 Manager 返回的地址，覆盖时必须包含 `{tunnel_id}`。自动唤醒依赖环境中的 Gateway 到 Manager 内部链路。

本地联调需要指定 Gateway IP 时，设置 `DEVBOX_GATEWAY_IP`，运行 `npm run validate:local`，
仍使用 Manager 返回的域名。
此入口仅在当前进程内覆盖匹配域名的 DNS，保留 HTTPS 域名和证书校验，不修改系统 hosts 或 SDK。
只验证生命周期时运行 `npm run validate:local -- --mode lifecycle`。

## 开发

```bash
npm ci
npm run check
```

`check` 依次执行代码检查、类型检查、单元测试和发布构建。
