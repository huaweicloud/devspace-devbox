# 用 OpenCode 模板生成并分享 Demo

在私有沙箱中运行 OpenCode，用提示词生成一个静态网页，再将生成的 HTML 复制到专门用于匿名预览的 Base 沙箱。

## 确认模板与凭证

按 [快速入门](../index.md) 安装 DevBox Python SDK，并设置 `DEVBOX_API_KEY`。用 `devbox tpl list` 确认目标环境有 OpenCode 模板。只有 Base 模板时，先请模板管理员发布 OpenCode 模板。

示例以 OpenCode Console 为模型服务。通过本地密钥管理工具注入模型服务的 `OPENCODE_API_KEY`，并将以下变量设置为你实际可用的值：

```bash
export OPENCODE_TEMPLATE="<模板列表中的 OpenCode 模板 ID 或别名>"
export OPENCODE_MODEL="opencode/<你的账号可用的模型 ID>"
```

`DEVBOX_API_KEY` 用来创建和管理沙箱，`OPENCODE_API_KEY` 只传给沙箱内运行 OpenCode 的命令。切换模型服务时，按 [OpenCode Provider 文档](https://opencode.ai/docs/providers/) 设置对应的凭证和模型。

## 生成网页

将下面的代码保存为 `generate_demo.py`，运行 `python generate_demo.py`。

```python
import os
import shlex
from pathlib import Path

from devbox import Sandbox

model = os.environ["OPENCODE_MODEL"]
prompt = (
    "在当前目录创建 index.html，制作一个中文待办清单 Demo。"
    "支持添加、勾选和删除待办事项。"
    "把 CSS 和 JavaScript 内嵌在 HTML 中，仅写入 index.html，"
    "使用 localStorage 保留数据，不引用外部资源。"
)

with Sandbox.create(
    os.environ["OPENCODE_TEMPLATE"],
    timeout=900,
    network={"allow_public_traffic": False},
) as builder:
    builder.commands.run("mkdir -p /tmp/generated-demo")
    builder.commands.run(
        shlex.join(["opencode", "run", "--model", model, prompt]),
        cwd="/tmp/generated-demo",
        envs={"OPENCODE_API_KEY": os.environ["OPENCODE_API_KEY"]},
        timeout=300,
    )
    html = builder.files.read("/tmp/generated-demo/index.html")

Path("index.html").write_text(html, encoding="utf-8")
print("已生成 index.html")
```

在本地查看生成的 `index.html`，确认页面功能和内容后再分享。遇到模型鉴权或模型不可用错误时，核对 Key 对应的模型服务和模型 ID。

## 启动匿名预览

复用 [Base 模板分享 Demo](./base-web-demo.md) 的脚本，把其中执行 `git clone` 的调用换成以下两行，在脚本顶部添加 `from pathlib import Path`：

```python
sandbox.commands.run("mkdir -p /tmp/web-demo")
sandbox.files.write(
    "/tmp/web-demo/index.html", Path("index.html").read_text(encoding="utf-8")
)
```

运行修改后的 `base_demo.py`，再打开它打印的 HTTPS 链接。预览沙箱里只有演示所需的网页文件，模型凭证仍只用于生成步骤。演示结束后按 Base 模板指南清理预览沙箱。
