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

## 核心方法

| 对象 | 方法 | 用途 |
| --- | --- | --- |
| `Sandbox` | `create`、`connect`、`getInfo`、`setTimeout`、`refresh`、`kill` | 沙箱生命周期 |
| `sandbox.commands` | `run`、`connect`、`list`、`sendStdin`、`sendSignal` | 命令与进程 |
| `sandbox.files` | `read`、`write`、`list`、`stat`、`makeDir`、`move`、`remove` | 远端文件 |
| `sandbox.files` | `upload`、`download`、`watch` | 本地传输与目录监听 |
| `sandbox.pty` | `start`、`connect`、`resize` | 交互式终端 |
| `sandbox.git` | `clone`、`status`、`checkout`、`add`、`commit`、`pull`、`push` | Git 工作流 |

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

`client.templates`、`client.snapshots` 和 `client.nodes` 是管理面扩展资源。部署环境未开放对应接口时会返回 `NotFoundError`，不影响基础沙箱能力。

## 配置

| 配置 | 环境变量 | 默认值 |
| --- | --- | --- |
| API Key | `DEVBOX_API_KEY` 或 `E2B_API_KEY` | 必填 |
| 管理面地址 | `DEVBOX_API_URL` 或 `E2B_API_URL` | `https://devbox.developer.myhuaweicloud.com` |
| 数据面地址覆盖 | `DEVBOX_GATEWAY_URL` | Manager 返回的地址 |
| 请求超时 | `requestTimeoutMs` | 30000 毫秒 |

构造参数优先于环境变量。API Key 只发送给管理面，Manager 返回的短期令牌只发送给对应数据面。SDK 不持久化凭证，也不会把凭证跟随重定向发送到其他地址。

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

## 开发

```bash
npm ci
npm run check
```

`check` 依次执行代码检查、类型检查、单元测试和发布构建。
