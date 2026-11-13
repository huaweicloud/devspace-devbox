# DevBox

<p align="center">
  <strong>云端隔离沙箱 · 创建 / 执行命令 / 读写文件 / 终端与 Git —— 为 AI 智能体与自动化提供安全可编程的运行环境</strong>
</p>

<p align="center">
  <a href="https://huaweicloud.github.io/devspace-devbox/">在线文档</a> · <a href="https://huaweicloud.github.io/devspace-devbox/">快速入门</a> · <a href="python-sdk/README.md">Python SDK</a> · <a href="js-sdk/README.md">JavaScript SDK</a> · <a href="docs/api/index.md">HTTP API</a>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://badgen.net/static/License/Apache-2.0/blue" alt="License" /></a>
  <img src="https://img.shields.io/badge/Python-3.10%2B-3776AB.svg" alt="Python" />
  <img src="https://img.shields.io/badge/Node.js-20.18.1%2B-339933.svg" alt="Node.js" />
  <img src="https://img.shields.io/badge/platform-Linux%20sandbox-lightgrey.svg" alt="Platform" />
  <a href="https://huaweicloud.github.io/devspace-devbox/"><img src="https://img.shields.io/badge/docs-online-blue.svg" alt="Docs" /></a>
</p>

DevBox 是华为云的云端隔离沙箱服务。它为每个请求创建独立的 Linux 沙箱，通过 Python、JavaScript / TypeScript SDK 或 HTTP API 在其中执行命令、读写文件、操作交互式终端与 Git，并在需要时暂停、恢复或销毁沙箱。你的程序运行在本地或应用服务器中，SDK 发出的命令与文件操作发生在远端沙箱里。

📖 在线文档：<https://huaweicloud.github.io/devspace-devbox/>

---

## 简介

DevBox 提供隔离的云端 Linux 沙箱，适合 AI 智能体代码执行、自动化测试、远程调试与原型开发。两套 SDK 使用一致的对象模型，覆盖沙箱管理、命令与进程、文件系统、PTY 和 Git 操作；非 Python / JavaScript 语言可通过 HTTP API 直接接入。

```text
Sandbox
├── commands    命令与进程（前台 / 后台 / 实时输出 / 重连）
├── files       文件系统（读写 / 上传下载 / 目录监听）
├── pty         交互式终端
└── git         Git 操作（clone / commit / push …）
```

## 架构

```text
┌─────────────────────┐   API Key (X-API-Key)   ┌──────────────┐   创建 / 连接   ┌─────────────────────┐
│  你的程序 (本地)     │ ──────────────────────► │   Manager    │ ──────────────► │  云端沙箱 (EnvD)     │
│  Python / JS SDK    │                         │  (管理面)     │                 │  commands / files    │
│  或 HTTP 客户端      │ ◄─── 沙箱对象 + domain ─┤              │                 │  pty / git           │
└─────────────────────┘                         └──────────────┘                 └──────────┬──────────┘
       │                                                                                     │
       │          connectToken (Cookie: relay_token) + E2B-Sandbox-Id                        │
       └────────────────────────────────── 数据面请求 ──────────────────────────────────────►┘
```

- **管理面**：`X-API-Key` 鉴权，用于创建、查询、暂停、恢复、删除沙箱与列出模板。
- **数据面**：使用创建 / 连接响应中的 `domain` 与 `connectToken`，向沙箱发起命令、文件、终端与 Git 请求。
- **生命周期**：沙箱按存活时间到期销毁，可暂停保存状态后恢复，也可主动 `kill()` 删除。

## 快速开始

设置服务签发的 API Key（默认服务地址已内置，无需额外配置）：

```bash
export DEVBOX_API_KEY=devbox_xxx
```

Python（需 3.10+）：

```bash
python -m pip install ./python-sdk
```

```python
from devbox import Sandbox

with Sandbox.create("default", timeout=300) as sandbox:
    result = sandbox.commands.run("printf 'hello from DevBox'")
    print(result.stdout)

    sandbox.files.write("/tmp/message.txt", "hello from the SDK")
    print(sandbox.files.read("/tmp/message.txt"))
```

JavaScript / TypeScript（需 Node.js 20.18.1+）：

```bash
cd js-sdk && npm ci && npm run build
npm install /path/to/devspace-devbox/js-sdk
```

```javascript
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default", { timeout: 300 });
try {
  const result = await sandbox.commands.run("printf 'hello from DevBox'");
  console.log(result.stdout);
} finally {
  try { await sandbox.kill(); } finally { await sandbox.close(); }
}
```

`default` 是模板别名，实际可用模板以[模板列表](https://huaweicloud.github.io/devspace-devbox/guide/templates)为准。Go、Java 等其他语言从 [API 参考](https://huaweicloud.github.io/devspace-devbox/api/) 开始接入。

## 核心能力

| 能力 | Python | JS / TS | HTTP API | 详情 |
| --- | --- | --- | --- | --- |
| 沙箱生命周期（创建 / 连接 / 暂停 / 恢复 / 删除） | ✅ | ✅ | ✅ | [生命周期](https://huaweicloud.github.io/devspace-devbox/guide/lifecycle) |
| 模板查询与指定 Build | ✅ | ✅ | ✅ | [模板](https://huaweicloud.github.io/devspace-devbox/guide/templates) |
| 命令执行（前台 / 后台 / 实时输出 / stdin） | ✅ | ✅ | ✅ | [命令与进程](https://huaweicloud.github.io/devspace-devbox/guide/commands) |
| 进程重连、信号与列表 | ✅ | ✅ | ✅ | [命令与进程](https://huaweicloud.github.io/devspace-devbox/guide/commands) |
| 文件读写、上传下载、目录监听 | ✅ | ✅ | ✅ | [文件操作](https://huaweicloud.github.io/devspace-devbox/guide/files) |
| 交互式终端（PTY） | ✅ | ✅ | ✅ | [终端与 Git](https://huaweicloud.github.io/devspace-devbox/guide/terminal-and-git) |
| Git 工作流（clone / commit / push …） | ✅ | ✅ | — | [终端与 Git](https://huaweicloud.github.io/devspace-devbox/guide/terminal-and-git) |
| 存活时间管理（set_timeout / refresh） | ✅ | ✅ | ✅ | [生命周期](https://huaweicloud.github.io/devspace-devbox/guide/lifecycle) |
| 异步接口（AsyncSandbox / Promise） | ✅ | ✅ | — | [SDK 参考](https://huaweicloud.github.io/devspace-devbox/reference/sdk) |

## 仓库结构

```text
.
├── .github/              # CI、文档发布与镜像同步工作流
├── python-sdk/           # DevBox Python SDK（Python 3.10+）
├── js-sdk/               # DevBox JavaScript / TypeScript SDK（Node.js 20.18.1+）
├── docs/                 # VitePress 文档站点（独立工具链，Node.js 24）
├── LICENSE               # Apache License 2.0
├── CONTRIBUTING.md       # 贡献指南
└── README.md             # 仓库总览（本文件）
```

各组件使用独立首层目录，构建配置与依赖文件保存在对应目录中，避免工具链相互影响。

| 组件 | 说明 | 详情 |
| --- | --- | --- |
| Python SDK | `devbox` 包，同步与异步入口 | [python-sdk/README.md](python-sdk/README.md) |
| JavaScript SDK | `devbox-js-sdk` 包，自带 TypeScript 类型 | [js-sdk/README.md](js-sdk/README.md) |
| 文档 | VitePress 站点，发布到 GitHub Pages | [docs/](docs/) |

## 文档

文档站点源码位于 `docs/`，由 VitePress 生成并发布到 GitHub Pages：<https://huaweicloud.github.io/devspace-devbox/>。本地预览与构建：

```bash
cd docs
npm ci
npm run dev      # 本地预览
npm run build    # 生成静态站点到 docs/.vitepress/dist/
```

主要页面：

| 主题 | 页面 |
| --- | --- |
| 快速入门 | [index](https://huaweicloud.github.io/devspace-devbox/) |
| 使用沙箱 | [命令与进程](https://huaweicloud.github.io/devspace-devbox/guide/commands) · [文件](https://huaweicloud.github.io/devspace-devbox/guide/files) · [终端与 Git](https://huaweicloud.github.io/devspace-devbox/guide/terminal-and-git) · [生命周期](https://huaweicloud.github.io/devspace-devbox/guide/lifecycle) |
| API 参考 | [通用约定](https://huaweicloud.github.io/devspace-devbox/api/) · [沙箱](https://huaweicloud.github.io/devspace-devbox/api/sandboxes) · [进程](https://huaweicloud.github.io/devspace-devbox/api/processes) · [文件系统](https://huaweicloud.github.io/devspace-devbox/api/filesystem) |
| SDK 参考 | [方法与配置](https://huaweicloud.github.io/devspace-devbox/reference/sdk) · [问题排查](https://huaweicloud.github.io/devspace-devbox/reference/troubleshooting) |

仓库启用 GitHub Pages（Source 选择 GitHub Actions）后，文档工作流从 `dev` 分支发布至上列地址。

## 贡献

欢迎贡献。开发环境、代码规范、测试、分支策略、提交规范与 PR 流程见 [贡献指南](CONTRIBUTING.md)。

## 免责声明

DevBox 云开发环境按"现状"（AS IS）与"当前"（AS AVAILABLE）基础提供，不承诺服务不间断、无错误或适配特定用途；在沙箱中运行的代码与内容由用户自行承担责任，请妥善保管 API Key 与连接凭证、保留证书校验，并对重要代码和数据自行备份。完整条款见[免责声明](https://huaweicloud.github.io/devspace-devbox/reference/disclaimer)（源码：[docs/reference/disclaimer.md](docs/reference/disclaimer.md)）。

## 许可证

[Apache License 2.0](LICENSE)。仓库治理文件使用 Apache-2.0 许可证，各 SDK 的许可证以对应包目录中的 `LICENSE` 为准。
