# 选择模板

模板决定沙箱的初始软件环境与资源规格。先查询可用模板，再按模板 ID 或别名创建。

## 查询可用模板

::: code-group
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
