# 贡献指南

感谢你对 DevBox 的关注！欢迎提交 Issue、Pull Request 或参与讨论。

参与本项目即表示你同意遵守 Apache License 2.0 的条款，并同意你的贡献将在该许可证下授权。

---

## 项目结构

DevBox 是一个 Monorepo，两个 SDK 各自使用独立的首层目录和工具链：

```text
.
├── .github/              # CI 工作流、Dependabot、Issue 模板
├── python-sdk/           # DevBox Python SDK（Python 3.10+）
│   ├── src/              # 包源码（导入名 devbox）
│   ├── tests/            # pytest 测试
│   ├── examples/         # 验收与本地联调脚本
│   └── pyproject.toml    # ruff / mypy / pytest 配置
├── js-sdk/               # DevBox JavaScript / TypeScript SDK（Node.js 20.18.1+）
│   ├── src/              # TypeScript 源码
│   ├── examples/         # 验收与本地联调脚本
│   └── package.json      # biome / tsc / vitest 配置
├── LICENSE               # Apache 2.0
├── CONTRIBUTING.md       # 本文件
└── README.md             # 仓库总览
```

> **原则**：后续新增组件应各自使用独立的首层目录，将构建配置和依赖文件保存在对应目录中，避免不同工具链相互影响。

---

## 本地环境配置

### 工具链版本要求

| 组件 | 最低版本 | 用途 |
| --- | --- | --- |
| **Git** | 2.20 | 版本控制 |
| **Python** | 3.10 | Python SDK 编译、类型检查与测试 |
| **Node.js** | 20.18.1 | JavaScript SDK 构建与测试 |

### 安装 Python（>= 3.10）

```bash
# 方式一：官方二进制包（推荐），前往 https://www.python.org/downloads/ 下载
# 方式二：包管理器（版本可能滞后，安装后用 python --version 确认）
# macOS:  brew install python@3.12
# Ubuntu: sudo apt install python3.12 python3.12-venv
```

验证安装：

```bash
python --version
# 期望输出: Python 3.12.x
```

### 安装 Node.js（>= 20.18.1）

```bash
# 方式一：官方二进制包，前往 https://nodejs.org/ 下载
# 方式二：包管理器
# macOS:  brew install node@22
# Ubuntu: 见 https://github.com/nodesource/distributions
```

验证安装：

```bash
node --version
# 期望输出: v22.x.x
```

### 一次性确认

```bash
git --version     # >= 2.20
python --version  # >= 3.10
node --version    # >= 20.18.1
```

全部通过后，进入 [快速开始](#快速开始)。

---

## 快速开始

### 1. Fork 并克隆

```bash
# 在 GitHub 上 Fork 本仓库，然后：
git clone https://github.com/<你的用户名>/devspace-devbox.git
cd devspace-devbox
```

### 2. 添加上游仓库

```bash
git remote add upstream https://github.com/huaweicloud/devspace-devbox.git
```

### 3. 安装依赖并本地构建验证

```bash
# Python SDK
cd python-sdk
python -m venv .venv
. .venv/bin/activate            # Windows: .venv\Scripts\activate
python -m pip install -e ".[dev]"
ruff check . && mypy && pytest

# JavaScript SDK
cd ../js-sdk
npm ci
npm run check
```

---

## 代码规范

### Python 代码（python-sdk/）

| 规则 | 工具 / 说明 |
| --- | --- |
| 格式化 | `ruff format .`，提交前必须执行；CI 用 `ruff format --check .` |
| 静态检查 | `ruff check .`（配置见 `pyproject.toml`，启用 E/F/I/UP/B/SIM/RUF） |
| 类型检查 | `mypy`（strict 模式，覆盖 `src`、`tests`、`examples`） |
| 命名 | 导出标识符用 PascalCase / snake_case 遵循 PEP 8，包名全小写 |
| 错误处理 | 不忽略异常，自定义异常继承 `DevBoxError` |
| 注释 | 公开函数 / 类型须有 docstring |

行宽上限 100，目标版本 `py310`。

### JavaScript / TypeScript 代码（js-sdk/）

| 规则 | 工具 / 说明 |
| --- | --- |
| 格式化 | `npm run format`（Biome） |
| 静态检查 | `npm run lint`（Biome check） |
| 类型检查 | `npm run typecheck`（tsc --noEmit） |
| 测试 | `npm test`（Vitest） |
| 一键全检 | `npm run check` = lint + typecheck + test + build |

SDK 自带 TypeScript 类型，公开 API 变更须同步更新类型声明。

### 通用约定

- **行尾换行**：文件末尾保留一个空行。
- **编码**：UTF-8，不使用 BOM。
- **凭证**：不得在代码、示例或测试中写入真实 API Key 或连接凭证。

---

## 测试

### Python SDK

```bash
cd python-sdk
pytest              # 运行全部测试
pytest -v           # 详细输出
mypy                # 类型检查
ruff check .        # 静态检查
python -m build     # 构建分发包
```

新增公开 API 时须附带对应的测试。

### JavaScript SDK

```bash
cd js-sdk
npm test            # Vitest 单元测试
npm run typecheck   # 类型检查
npm run build       # 构建产物
```

### 端到端验收

两端各提供验收脚本，需配置 `DEVBOX_API_KEY` 并访问真实服务：

```bash
# Python
cd python-sdk && python examples/validate_full.py
python examples/validate_full.py --mode lifecycle --rounds 5

# JavaScript
cd js-sdk && npm run validate
npm run validate -- --mode lifecycle --rounds 5
```

`basic` 验证管理查询、命令、文件、PTY 和 Git；`lifecycle` 验证生命周期；默认 `full` 执行全部。验收脚本会在结束时清理测试沙箱。

---

## 分支策略

| 分支 | 用途 | 能否直接推送 |
| --- | --- | --- |
| `main` | 稳定发布分支 | ❌ 仅通过合并 |
| `dev` | 开发集成分支 | ❌ 仅通过 PR 合并 |
| `feat/*` | 新功能分支 | ✅（个人 Fork） |
| `fix/*` | Bug 修复分支 | ✅（个人 Fork） |

**PR 目标分支**：默认为 `dev`。仅在发布窗口期由维护者合并至 `main`。

---

## 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```text
<type>(<scope>): <subject>
```

### type 取值

| type | 说明 |
| --- | --- |
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档变更 |
| `style` | 代码格式（不影响逻辑） |
| `refactor` | 重构 |
| `test` | 测试相关 |
| `chore` | 构建、工具、依赖变更 |

### scope 取值

| scope | 对应组件 |
| --- | --- |
| `python` | Python SDK |
| `js` | JavaScript SDK |
| `ci` | CI/CD 配置 |

### 示例

```text
feat(python): 支持沙箱暂停后按 PID 重连
fix(js): 修复流式命令断线后 Promise 未结算
chore(ci): 升级 actions/setup-python 至 v5
```

---

## Pull Request 流程

### 1. 创建工作分支

```bash
git checkout dev
git pull upstream dev
git checkout -b feat/your-feature
```

### 2. 编写代码并本地验证

根据修改的组件，运行对应的检查：

```bash
# 修改了 Python SDK
cd python-sdk && ruff check . && ruff format --check . && mypy && pytest && python -m build

# 修改了 JavaScript SDK
cd js-sdk && npm run check
```

### 3. 提交并推送

```bash
git commit -m "feat(python): your feature description"
git push origin feat/your-feature
```

### 4. 在 GitHub 上创建 PR

- **目标分支**：`dev`
- **PR 标题**：遵循提交规范格式
- **PR 描述**需包含：
  - **变更说明**：做了什么
  - **关联 Issue**：如 `Closes #123`
  - **测试方式**：如何验证（含端到端验收时注明所用模式）
  - **截图**：CLI 或输出变更请附截图

### 5. PR 审查清单

- [ ] CI 全部通过
- [ ] 新功能附带测试
- [ ] 无引入新的 lint / 类型错误
- [ ] 公开 API 变更已更新类型声明与示例
- [ ] 未提交真实凭证

### CI 自动检查

PR 创建后将自动触发以下 CI（见 `.github/workflows/ci.yml`）：

| 工作流 | 触发条件 | 检查内容 |
| --- | --- | --- |
| `CI / python` | PR 到 main/dev | ruff + mypy + pytest + build（矩阵 3.10–3.13） |
| `CI / javascript` | PR 到 main/dev | `npm run check`（矩阵 Node 20/22） |

---

## Issue 指南

### Bug 报告

请包含以下信息：

- **环境**：操作系统、Python / Node.js 版本、SDK 版本
- **复现步骤**：逐步可操作的最小示例
- **期望行为**：应该发生什么
- **实际行为**：实际发生了什么，附错误码与请求 ID（`X-Request-ID`）
- **日志 / 截图**：如有请附上，**不要附 API Key、Token 或敏感文件内容**

### 功能请求

- **使用场景**：你在什么情况下需要这个功能
- **期望方案**：你设想的实现方式（可选）
- **替代方案**：你考虑过的其他方式（可选）

### 好的 Issue

- 一个 Issue 只描述一个问题
- 先搜索已有 Issue，避免重复
- 使用清晰的标题概括问题

---

## 常见问题

### ruff / mypy 报错但本地能跑

确认本地 Python 版本与 CI 一致（3.10–3.13）。`pyproject.toml` 中 `target-version = "py310"`、`mypy` 为 strict 模式，版本差异可能导致本地通过但 CI 失败。

### npm run check 失败

依次确认 `lint`（Biome）、`typecheck`（tsc）、`test`（Vitest）、`build` 四步。Biome 格式问题用 `npm run format` 修复；类型错误须手动修正。

### 端到端验收失败

验收脚本需访问真实 Manager 新建的沙箱，不接受独立的固定 Gateway 地址。确认 `DEVBOX_API_KEY` 有效、网络可达。本地无通配符 DNS 时设置 `DEVBOX_GATEWAY_IP`，通过 `examples/run_local.py` / `npm run validate:local` 运行。

---

## 联系方式

- **Issue**：[GitHub Issues](https://github.com/huaweicloud/devspace-devbox/issues)
- **安全漏洞**：见 [SECURITY.md](SECURITY.md)，请勿在公开 Issue 中披露

---

再次感谢你的贡献！每一个 PR 和 Issue 都让 DevBox 更好。
