# 使用default模板创建沙箱，clone并运行代码，浏览器访问

在 DevBox 沙箱中克隆 GitHub 的 [Spoon-Knife 示例仓库](https://github.com/octocat/Spoon-Knife)，用 Python 提供静态页面，再把 HTTPS 地址分享给其他人。Base 模板的别名为 `default`，预装了 Git 和 Python。

## 创建沙箱并启动页面

按 [快速入门](../index.md) 安装 DevBox Python SDK，并设置 `DEVBOX_API_KEY`。将下面的代码保存为 `base_demo.py`，运行 `python base_demo.py`。

```python
from devbox import Sandbox

sandbox = Sandbox.create(
    "default",
    timeout=900,
    network={"allow_public_traffic": True},
)
try:
    sandbox.commands.run(
        "git clone --depth 1 https://github.com/octocat/Spoon-Knife.git /tmp/web-demo-src"
        " && mkdir -p /tmp/web-demo"
        " && git -C /tmp/web-demo-src archive HEAD | tar -x -C /tmp/web-demo",
        timeout=120,
    )
    sandbox.commands.run(
        "python3 -m http.server 8080 --bind 0.0.0.0 --directory /tmp/web-demo",
        background=True,
    )
    sandbox.commands.run(
        "curl -fsS --retry 5 --retry-connrefused http://127.0.0.1:8080/ -o /dev/null",
    )
    preview_url = sandbox.envd_url.replace("-49983.", "-8080.", 1)
    print("浏览器地址", preview_url)
    print("沙箱 ID", sandbox.sandbox_id)
except Exception:
    sandbox.kill()
    raise
finally:
    sandbox.close()
```

## 用浏览器访问并分享

打开脚本打印的 HTTPS 地址。也可以用无痕窗口验证匿名访问，再把相同链接发给别人。应用监听沙箱内的 `8080`，网关用主机名中的 `-8080` 转发，浏览器使用默认的 HTTPS 端口。

`allow_public_traffic=True` 在创建时允许匿名访问该沙箱的应用端口。此示例只运行公开 Demo。分享时只发送页面链接。

脚本结束后，沙箱继续运行，链接在沙箱销毁后失效。演示结束可提前释放：

```bash
devbox sbx kill "$SANDBOX_ID"
```

将 `SANDBOX_ID` 设为脚本打印的 ID，使用已登录同一个 DevBox 账号的 CLI 执行。端口地址规则见 [访问应用端口](../api/sandboxes.md#application-port)。
