# 选择模板

模板决定沙箱的初始软件环境与资源规格。先查询可用模板，再按模板 ID 或别名创建。

## 生产模板

| 别名 | 模板 ID | vCPU | 内存 | 磁盘 | 用途 |
| --- | --- | ---: | ---: | ---: | --- |
| `default` | `tpl-devbox-base` | 1 | 2 GiB | 10 GiB | 运行代码、调试和预览应用 |
| `opencode` | `tpl-devbox-opencode` | 1 | 2 GiB | 10 GiB | 在沙箱中用 OpenCode 生成和修改代码 |

### 两个模板的预装组件

| 类别 | 组件与版本 |
| --- | --- |
| 操作系统 | openEuler 24.03 LTS（ARM64 / aarch64），Linux 内核 6.6.158 |
| Python | Python 3.11.6，pip 23.3.1 |
| JavaScript | Node.js 20.18.0，npm 10.8.2 |
| 命令行与版本控制 | Bash 5.2.15，Git 2.43.0 |
| 下载与数据处理 | curl 8.4.0，Wget 1.21.4，jq 1.6，GNU tar 1.35 |
| 沙箱运行时 | EnvD 0.1.11 |

### OpenCode 模板额外预装的组件

| 组件 | 版本 | 命令 |
| --- | --- | --- |
| OpenCode | 1.18.32 | `opencode` |
| ripgrep | 15.1.0 | `rg` |

组件版本以所选 Build 为准。使用 OpenCode 时，按[创建并预览个人网站](../best-practices/opencode-web-demo.md)配置模型 API Key。

## 查询可用模板

新模板上线后会出现在模板列表里。下面的命令和 SDK 示例会查询当前连接环境的可用模板、别名、资源规格和 Build ID。连接生产环境时可用它们确认最新列表。通过 HTTP 接入时使用 [GET /templates](../api/templates.md#list)。

::: code-group
```bash [CLI]
devbox tpl list
```

```python [Python]
from devbox import DevBox

with DevBox() as client:
    for template in client.templates.list():
        print(
            template.template_id,
            template.name,
            template.aliases,
            template.build_id,
            template.cpu_count,
            template.memory_mb,
            template.disk_size_mb,
        )
```

```js [JavaScript / TypeScript]
import { DevBox } from "devbox-js-sdk";

const client = new DevBox();
try {
  for (const template of await client.templates.list()) {
    console.log(
      template.templateId, template.name, template.aliases, template.buildId,
      template.cpuCount, template.memoryMb, template.diskSizeMb,
    );
  }
} finally {
  await client.close();
}
```
:::

内存和磁盘以 MB 展示。列表中的 Build 是该模板当前供创建使用的构建版本，不是所有历史构建的列表。

## 使用指定 Build

一般使用模板 ID 或别名即可。需要固定环境、避免后续模板更新影响新沙箱时，可同时指定列表返回的 Build ID。

::: code-group
```python [Python]
from devbox import DevBox

with DevBox() as client:
    templates = client.templates.list()
    if not templates:
        raise RuntimeError("没有可用模板")
    template = templates[0]
    sandbox = client.sandboxes.create(
        template.template_id,
        build_id=template.build_id,
        timeout=300,
        envs={"APP_MODE": "demo"},
        metadata={"project": "example"},
    )
    try:
        print(sandbox.commands.run("printenv APP_MODE").stdout)
    finally:
        try:
            sandbox.kill()
        finally:
            sandbox.close()
```

```js [JavaScript / TypeScript]
import { DevBox } from "devbox-js-sdk";

const client = new DevBox();
try {
  const [template] = await client.templates.list();
  if (!template) throw new Error("没有可用模板");
  const sandbox = await client.sandboxes.create(template.templateId, {
    buildId: template.buildId,
    timeout: 300,
    envs: { APP_MODE: "demo" },
    metadata: { project: "example" },
  });
  try {
    console.log((await sandbox.commands.run("printenv APP_MODE")).stdout);
  } finally {
    try { await sandbox.kill(); } finally { await sandbox.close(); }
  }
} finally {
  await client.close();
}
```
:::

`envs` 为沙箱内环境变量，`metadata` 为便于识别沙箱的业务标签。模板中的软件、Git 和网络能力以所选环境实际提供的内容为准。
