# DevBox 快速入门

DevBox 提供隔离的云端 Linux 沙箱。通过 Python 或 JavaScript/TypeScript SDK，你可以执行命令、读写文件、操作终端和 Git，并在需要时暂停、恢复或删除沙箱。

你的程序运行在本地或应用服务器中；SDK 中的命令和远端文件操作发生在沙箱里。

使用 Go、Java 等其他语言直接接入，可从[API 参考](./api/index.md)开始；Python 与 JavaScript / TypeScript 用户可继续按下面的步骤使用 SDK。

## 安装

Python 需要 3.10+，安装包发布在 [PyPI](https://pypi.org/project/devbox-sdk/)，在代码中用 `import devbox` 导入。JavaScript / TypeScript 需要 Node.js 20.18.1+，在应用目录从 [npm](https://www.npmjs.com/package/devbox-js-sdk) 安装。

::: code-group
```bash [Python]
python -m pip install devbox-sdk
```

```bash [JavaScript / TypeScript]
npm install devbox-js-sdk
```
:::

## 配置凭证

准备 DevBox API Key。SDK 默认连接生产管理面 `https://devbox.developer.myhuaweicloud.com`。不要将 API Key 写入源码或提交到 Git。

::: code-group
```bash [Linux / macOS]
export DEVBOX_API_KEY="<你的 API Key>"
```

```bat [Windows CMD]
set "DEVBOX_API_KEY=<你的 API Key>"
```
:::

使用其他环境时，通过 `DEVBOX_API_URL` 设置该环境的管理面地址。详见[配置参考](./reference/sdk.md#配置)。

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

## 最佳实践

- [兼容E2B生态](./best-practices/e2b.md)
- [沙箱运行代码并浏览器预览](./best-practices/base-web-demo.md)
- [OpenCode模板沙箱创作个人网站](./best-practices/opencode-web-demo.md)
- [Skill自动管理云沙箱](./best-practices/agent-skill.md)