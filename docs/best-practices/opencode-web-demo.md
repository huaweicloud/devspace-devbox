# 用 OpenCode 模板创建并预览个人网站

生产环境的 `opencode` 模板预装了 OpenCode 和 ripgrep。用 OpenCode 生成 `index.html`，检查网页内容后，再用 Base 沙箱提供公开预览。

## 配置 API Key

按[快速入门](../index.md)安装 DevBox Python SDK，设置 `DEVBOX_API_KEY`。

示例使用 OpenCode Console。通过密钥管理工具将模型服务的 Key 注入 `OPENCODE_API_KEY`，再选择该 Key 可用的模型。

```bash
export OPENCODE_MODEL="opencode/<模型 ID>"
```

其他模型服务的模型格式和凭证配置见 [OpenCode Provider 文档](https://opencode.ai/docs/providers/)。

## 生成网页

将以下代码保存为 `generate_demo.py`，运行 `python generate_demo.py`。脚本在私有沙箱中运行 OpenCode，将生成的 `index.html` 下载到当前目录，随后删除沙箱。

```python
import os
import shlex
from pathlib import Path

from devbox import Sandbox

model = os.environ["OPENCODE_MODEL"]
model_key = os.environ["OPENCODE_API_KEY"]
prompt = (
    "在当前目录创建 index.html，制作一个中文个人网站。"
    "使用虚构姓名和内容，包含个人简介、技能和两个示例项目。"
    "仅写入 index.html，内嵌 CSS，适配手机和桌面，不引用外部资源。"
)

with Sandbox.create(
    "opencode",
    timeout=900,
    network={"allow_public_traffic": False},
) as sandbox:
    sandbox.commands.run("mkdir -p /tmp/site")
    sandbox.commands.run(
        shlex.join(["opencode", "run", "--model", model, prompt]),
        cwd="/tmp/site",
        envs={"OPENCODE_API_KEY": model_key},
        timeout=300,
    )
    html = sandbox.files.read("/tmp/site/index.html")

Path("index.html").write_text(html, encoding="utf-8")
```

在浏览器中打开本地的 `index.html` 检查页面。

## 分享预览

将以下代码保存为 `preview_demo.py`，在 `index.html` 所在目录运行 `python preview_demo.py`。脚本只将网页文件上传到预览沙箱。

```python
from pathlib import Path

from devbox import Sandbox

html = Path("index.html").read_text(encoding="utf-8")
preview = Sandbox.create(
    "default",
    timeout=900,
    network={"allow_public_traffic": True},
)
try:
    preview.commands.run("mkdir -p /tmp/site")
    preview.files.write("/tmp/site/index.html", html)
    preview.commands.run(
        "python3 -m http.server 8080 --bind 0.0.0.0 --directory /tmp/site",
        background=True,
    )
    preview.commands.run(
        "curl -fsS --retry 5 --retry-connrefused http://127.0.0.1:8080/ -o /dev/null",
    )
    print("浏览器地址", preview.envd_url.replace("-49983.", "-8080.", 1))
    print("沙箱 ID", preview.sandbox_id)
except Exception:
    preview.kill()
    raise
finally:
    preview.close()
```

打开脚本打印的 HTTPS 地址，也可以把同一个地址发给别人。链接随沙箱到期失效。提前结束时，用 `devbox sbx kill "$SANDBOX_ID"` 删除沙箱，其中 `SANDBOX_ID` 是脚本打印的 ID。
