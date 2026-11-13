# DevBox 快速入门

DevBox 提供隔离的云端 Linux 沙箱。通过 Python 或 JavaScript/TypeScript SDK，你可以执行命令、读写文件、操作终端和 Git，并在需要时暂停、恢复或删除沙箱。

你的程序运行在本地或应用服务器中；SDK 中的命令和远端文件操作发生在沙箱里。

使用 Go、Java 等其他语言直接接入，可从[API 参考](./api/index.md)开始；Python 与 JavaScript / TypeScript 用户可继续按下面的步骤使用 SDK。

## 安装

需要 Python 3.10+，或 Node.js 20.18.1+。从源码安装：

```bash
git clone https://github.com/huaweicloud/devspace-devbox.git
cd devspace-devbox
```

::: code-group
```bash [Python]
python -m pip install ./python-sdk
```

```bash [JavaScript / TypeScript]
cd js-sdk
npm ci
npm run build
# 在你的应用目录中安装，路径替换为实际位置：
npm install /path/to/devspace-devbox/js-sdk
```
:::

## 配置凭证

准备服务提供的 API Key 和访问地址。不要将 API Key 写入源码或提交到 Git。

::: code-group
```bash [Linux / macOS]
export DEVBOX_API_KEY="<你的 API Key>"
export DEVBOX_API_URL="https://<服务访问域名>"
```

```bat [Windows CMD]
set "DEVBOX_API_KEY=<你的 API Key>"
set "DEVBOX_API_URL=https://<服务访问域名>"
```
:::

已使用默认服务地址时，可以不设置 `DEVBOX_API_URL`。详见[配置参考](./reference/sdk.md#配置)。

## 创建并使用沙箱

保存为 `app.py` 或 `app.mjs`，然后运行 `python app.py` 或 `node app.mjs`。

::: code-group
```python [Python]
from devbox import Sandbox

with Sandbox.create("default", timeout=300) as sandbox:
    print("Sandbox:", sandbox.sandbox_id)
    result = sandbox.commands.run("printf 'hello from DevBox\n'")
    print(result.stdout)

    sandbox.files.write("/tmp/message.txt", "hello from the SDK")
    print(sandbox.files.read("/tmp/message.txt"))
```

```js [JavaScript / TypeScript]
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default", { timeout: 300 });
try {
  console.log("Sandbox:", sandbox.sandboxId);
  const result = await sandbox.commands.run("printf 'hello from DevBox\n'");
  console.log(result.stdout);

  await sandbox.files.write("/tmp/message.txt", "hello from the SDK");
  console.log(await sandbox.files.read("/tmp/message.txt"));
} finally {
  try {
    await sandbox.kill();
  } finally {
    await sandbox.close();
  }
}
```
:::

`default` 是模板别名，实际可用模板以[模板列表](./guide/templates.md)为准。

## 保留还是删除

| 操作 | 效果 |
| --- | --- |
| `kill()` | 删除远端沙箱，不再保留其中的数据 |
| `close()` | 释放本地 SDK 连接，不删除远端沙箱 |
| `pause()` | 暂停沙箱，保存状态供后续恢复 |

Python 的 `with Sandbox.create(...)` 会在离开代码块时执行删除。需要保留沙箱时，手动创建并在使用结束后调用 `close()`。记录沙箱 ID 后，可使用 `Sandbox.connect(id)` 再次访问。

沙箱默认存活 300 秒，到期销毁。更长任务或暂停恢复，请阅读[生命周期](./guide/lifecycle.md)。
