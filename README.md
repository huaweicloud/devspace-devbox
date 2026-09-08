# DevBox SDK

[![License](https://img.shields.io/badge/License-Apache-2.0-blue.svg)](LICENSE)

DevBox 为云端隔离沙箱提供 Python 和 JavaScript/TypeScript SDK。两套 SDK 使用一致的对象模型，覆盖沙箱管理、命令与进程、文件系统、PTY 和 Git 操作。

## 项目结构

```text
packages/
├── python-sdk/   Python SDK
└── js-sdk/       JavaScript / TypeScript SDK
```

每个 SDK 独立维护依赖、测试、构建配置和使用文档：

- [Python SDK](packages/python-sdk/README.md)
- [JavaScript SDK](packages/js-sdk/README.md)

## 安装

从仓库检出代码后安装：

```bash
python -m pip install ./packages/python-sdk
npm install ./packages/js-sdk
```

发布到语言包仓库后安装：

```bash
python -m pip install devbox-sdk
npm install devbox-js-sdk
```

## 快速开始

Python：

```python
from devbox import Sandbox

with Sandbox.create("default") as sandbox:
    result = sandbox.commands.run("printf 'hello from DevBox'")
    print(result.stdout)
```

JavaScript / TypeScript：

```typescript
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default");
try {
  const result = await sandbox.commands.run("printf 'hello from DevBox'");
  console.log(result.stdout);
} finally {
  await sandbox.kill();
  await sandbox.close();
}
```

## 开发验证

```bash
cd packages/python-sdk && python -m pip install -e ".[dev]" && ruff check . && mypy && pytest
cd packages/js-sdk && npm ci && npm run check
```

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

仓库治理文件使用 Apache-2.0 许可证。各 SDK 的许可证以对应包目录中的 `LICENSE` 为准。
