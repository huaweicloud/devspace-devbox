# DevBox JavaScript SDK

用于创建和操作云端隔离沙箱的 TypeScript/JavaScript SDK。要求 Node.js 20.18.1 或更高版本。

## 使用模型

日常使用只需要一个入口和四组能力：

```text
Sandbox
├── commands    命令与进程
├── files       文件系统
├── pty         交互式终端
└── git         Git 操作
```

所有网络操作均返回 `Promise`。SDK 自带 TypeScript 类型，不需要额外安装类型包。

## 安装

发布到包仓库后安装：

```bash
npm install devbox-js-sdk
```

从仓库源码安装：

```bash
npm install ./packages/js-sdk
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
| `client.sandboxes` | `create`、`connect`、`get`、`list`、`metrics` | 批量管理沙箱 |
| `Sandbox` | `create`、`connect`、`getInfo`、`isRunning`、`setTimeout`、`refresh`、`kill`、`close` | 沙箱生命周期 |
| `sandbox.commands` | `run`、`connect`、`list`、`sendStdin`、`closeStdin`、`sendSignal` | 命令与进程 |
| `sandbox.files` | `read`、`write`、`list`、`stat`、`makeDir`、`move`、`remove` | 远端文件 |
| `sandbox.files` | `upload`、`download`、`watch` | 本地传输与目录监听 |
| `sandbox.pty` | `start`、`connect`、`resize` | 交互式终端 |
| `sandbox.git` | `clone`、`status`、`checkout`、`add`、`commit`、`pull`、`push`、`setConfig` | Git 工作流 |

沙箱生命周期的 `timeout`、`duration` 以秒为单位；命令、Git 和请求配置中的
`timeoutMs`、`requestTimeoutMs` 以毫秒为单位。
`setTimeout(300)` 和 `refresh(300)` 均把沙箱截止时间重设为当前时间后 300 秒，
不是在原截止时间上累加。要修改剩余时间，请显式调用这两个方法，不要依赖 `connect` 的 `timeout` 选项续期。
`isRunning()` 查询管理面状态，不是数据面探活；过期清理期间，状态可能短暂滞后。

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
命令使用沙箱配置的默认系统用户。当前 EnvD 不支持切换用户，显式传入 `user` 会抛出
`ConfigurationError`，不会静默以默认用户执行。

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

## 管理多个沙箱

`DevBox` 复用管理面连接，适合服务端程序和批量操作：

```js
import { DevBox } from "devbox-js-sdk";

const client = new DevBox();
try {
  const page = await client.sandboxes.list();
  console.log(page.items, page.total);
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
| 请求超时 | `requestTimeoutMs` | 30000 毫秒 |

构造参数优先于环境变量。API Key 只发送给管理面，Manager 返回的 `connectToken` 只发送给数据面，使用 `Cookie: relay_token=<connectToken>`。SDK 不持久化凭证，也不会把凭证跟随重定向发送到其他地址。

Manager 未直接返回数据面地址的部署可以设置 URL 模板，例如 `DEVBOX_GATEWAY_URL=https://{tunnel_id}-{port}.cn-north-4-bridge.myhuaweicloud.com`。

Manager 的 `connectToken` 是 Relay 连接凭证，`tokenExpiration` 是其 Unix 秒过期时间；
`tunnelExpiration` 是隧道自身的过期时间，两者独立。SDK 在内部保留这些字段，不放入沙箱公开信息或日志。
`connect()` 获取 Manager 当前保存的连接信息，不保证签发新 Token。SDK 当前没有后台续期或
Token 过期自动重连机制；获取新的连接凭证需显式调用 `connect()`，由 Manager 保证返回有效凭证。

数据面使用 EnvD 的 `/process.Process/*`、`/filesystem.Filesystem/*` 和 `/files`，
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

```bash
npm run example
npm run validate
```

`validate` 会创建一个临时沙箱，验证管理面生命周期以及命令、文件、PTY、Git 等数据面能力，最后删除沙箱。模板可通过 `DEVBOX_TEST_TEMPLATE` 覆盖。

本地联调需要指定 Gateway IP 时，设置 `DEVBOX_GATEWAY_IP`，并将 `DEVBOX_GATEWAY_URL`
配置为 `https://{tunnel_id}-{port}.<gateway-domain>`，运行 `npm run validate:local`。
此入口仅在当前进程内覆盖匹配域名的 DNS，保留 HTTPS 域名和证书校验，不修改系统 hosts 或 SDK。

## 开发

```bash
npm ci
npm run check
```

`check` 依次执行代码检查、类型检查、单元测试和发布构建。
