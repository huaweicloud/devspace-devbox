# 用 E2B SDK 和 CLI 接入 DevBox

已有 E2B 项目可以保留原来的 SDK 或 CLI，将凭证和管理面地址切换到 DevBox，即可创建、执行和删除沙箱。以下示例使用 DevBox 的 Base 模板别名 `default`。

## 配置 DevBox 凭证和地址

按 [DevBox API Key 获取方式](../cli/authentication.md#获取-api-key) 创建凭证，通过环境变量注入 `DEVBOX_API_KEY`，然后在运行 E2B 客户端的终端设置：

```bash
export E2B_API_KEY="${DEVBOX_API_KEY:?请先设置 DEVBOX_API_KEY}"
export E2B_API_URL="https://devbox.developer.myhuaweicloud.com"
```

如果使用其他 DevBox 环境，把 `E2B_API_URL` 设置为该环境的管理面完整地址。已有项目如果在代码里显式指定了 API 地址或 Key，也要把对应配置切换到 DevBox。

## 用 E2B JavaScript SDK 执行命令

```bash
npm install e2b@2.64.0
```

保存为 `app.mjs`，运行 `node app.mjs`。

```js
import { Sandbox } from "e2b";

const sandbox = await Sandbox.create("default", {
  timeoutMs: 120_000,
  network: { allowPublicTraffic: false },
});

try {
  const result = await sandbox.commands.run("printf 'hello from DevBox\\n'", {
    user: "root",
  });
  console.log(result.stdout.trim());
} finally {
  await sandbox.kill();
}
```

示例显式传入了 Base 模板上可用的 `root`。在其他模板上执行命令时，使用该模板中实际存在的用户。

## 用 E2B CLI 执行同一个流程

以下命令使用前面配置的环境变量。依次创建沙箱、执行命令并删除沙箱。

```bash
npx --yes --package @e2b/cli@2.24.0 e2b sbx create default --detach --timeout 120
```

将上一步输出的沙箱 ID 填入 `SANDBOX_ID`，再执行：

```bash
npx --yes --package @e2b/cli@2.24.0 e2b sbx exec --user root "$SANDBOX_ID" printf 'hello from DevBox\n'
npx --yes --package @e2b/cli@2.24.0 e2b sbx kill "$SANDBOX_ID"
```

生产验证使用了上面列出的 SDK 和 CLI 版本。更多支持的请求字段、网络选项和已知限制见 [E2B 兼容说明](../api/sandboxes.md#e2b)。
