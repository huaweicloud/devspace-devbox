---
title: AI Agent Skill
description: 通过 huawei-cloud-devbox-sandbox skill 让 AI 代理自动管理云沙箱。
---

# AI Agent Skill

<p class="lead"><code>huawei-cloud-devbox-sandbox</code> 是一个 AI 代理技能，封装了 DevBox CLI 与 JS/Python SDK 的沙箱操作，让代理自动完成安装、认证、创建沙箱、执行命令与清理。</p>

## 适用场景

- 让 AI 代理一键创建云沙箱并运行命令，无需手动逐条执行 CLI 命令；
- 在对话中用自然语言创建、查询、暂停、恢复和删除沙箱；
- 需要把沙箱能力嵌入代码时，代理按 JS/Python SDK 参考生成可运行的集成代码。

## 通道选择

Skill 内置三条通道，按场景选择：

| 通道         | 适用场景                     | 特点                                        |
| ------------ | ---------------------------- | ------------------------------------------- |
| DevBox CLI  | 终端操作、快速验证、一次性命令 | 官方产物包安装，内置交互式终端与远程执行      |
| JS SDK       | Node.js / TypeScript 集成    | ESM、自带类型；`timeoutMs` 等参数单位为毫秒 |
| Python SDK   | 脚本、自动化、AI 代理编排     | `with` 退出即删除沙箱；提供完整的异步镜像    |

## 安装 Skill

### 通过 Skill 管理器安装

```bash
npx skills add https://gitcode.com/developer-skill/huaweicloud-skills.git \
  --skill huawei-cloud-devbox-sandbox
```

安装后 skill 位于 `~/.agents/skills/huawei-cloud-devbox-sandbox`，下文用 `<skill_dir>` 指代该目录。

### 从本地路径安装

```bash
npx skills add <path>/huaweicloud-skills/skills/devtools/devbox/huawei-cloud-devbox-sandbox
```

### 从 Skill 库搜索安装

已安装 `huawei-cloud-find-skills` 的代理可以通过关键词搜索并安装，搜索时直接使用 skill 名称。

## Skill 能做什么

Skill 封装了以下能力：

| 能力         | 通道                | 用途                                 |
| ------------ | ------------------- | ------------------------------------ |
| 环境自检     | `devbox_check.py`   | 安装 CLI、验证版本与认证状态         |
| 端到端冒烟   | `--smoke`           | 创建沙箱 → 执行命令 → 删除沙箱       |
| 沙箱生命周期 | CLI / SDK           | 创建、查询、暂停、恢复、删除         |
| 远程命令     | CLI / SDK           | 一次性命令、后台进程、交互式终端     |
| 文件操作     | SDK                 | 读写、上传下载、目录监听             |
| Git 工作流   | SDK                 | 克隆、提交、推送                     |
| 运行时诊断   | SDK                 | `envd` 实时指标、环境变量、健康探活  |

## 关键约束

| 主题         | 约束                                                                |
| ------------ | ------------------------------------------------------------------- |
| 认证         | CLI 读 `HW_DEVBOX_API_KEY`，SDK 读 `DEVBOX_API_KEY`；一把 Key 通用 |
| 生命周期     | `timeout` / `duration` 单位为秒，范围 10–86400；`refresh` 只延长不缩短 |
| 清理         | Python `with` 退出即删沙箱；CLI 用 `sbx kl`；`sbx kill --all` 批量删除 |
| 错误         | SDK 异常均继承 `DevBoxError`；CLI 为非零退出码 + 本地化 stderr 提示 |
| 会话环境变量 | CLI `-e KEY=VAL` 仅作用于当前终端会话，退出后失效                  |
| 命令与路径   | 命令默认经 `/bin/bash -l -c` 执行；文件 API 仅接受绝对路径          |

## 前置条件

Skill 会在首次使用时自动完成以下准备工作：

1. 运行内置自检脚本，必要时通过官方安装脚本安装 DevBox CLI；
2. 验证 CLI 版本与认证状态；
3. 仅在任务涉及代码集成时配置对应 SDK（纯 CLI 任务无需 SDK）。

自动化环境建议使用 API Key 认证，避免交互登录：

```bash
export HW_DEVBOX_API_KEY="devbox_xxx"    # CLI
export DEVBOX_API_KEY="devbox_xxx"       # JS / Python SDK
```

## 使用示例

### 快速创建沙箱并执行命令

告诉代理：

> 帮我创建一个云沙箱，跑个 hello world

代理会自动执行以下流程：

1. 运行环境自检，必要时安装 CLI 并确认认证状态；
2. 查询模板列表，以别名 `default` 创建沙箱（默认有效期 300 秒）；
3. 执行一次性命令并校验输出；
4. 删除沙箱，释放配额。

完成后代理返回沙箱 ID 与命令输出。

### 指定参数

> 用 `tpl-devbox-base` 模板创建一个沙箱，有效期 1 小时，装好依赖后暂停，明天继续用

代理会使用你指定的参数，未指定的使用默认值，不会逐个确认。

### 嵌入代码

> 用 DevBox Python SDK 写个脚本，在沙箱里克隆仓库并跑测试

代理按 Skill 中的 SDK 参考生成可运行代码，遵循生命周期与清理约束，接口细节见 [SDK 参考](../reference/sdk.md)。

### 默认参数

| 参数     | 默认值                     |
| -------- | -------------------------- |
| 模板     | `default`（别名或模板 ID） |
| 有效期   | 300 秒                     |
| 命令超时 | 60 秒                      |
| 元数据   | 无                         |

## 验证 Skill

安装后可用 Skill 内置脚本做端到端验证：

```bash
# 环境自检：安装 CLI、验证版本与认证
python3 <skill_dir>/scripts/devbox_check.py

# 检查 JS / Python SDK 环境
python3 <skill_dir>/scripts/devbox_check.py --js
python3 <skill_dir>/scripts/devbox_check.py --python

# 端到端冒烟：创建沙箱 → 执行命令 → 删除沙箱
python3 <skill_dir>/scripts/devbox_check.py --smoke

# 离线协议级回归（无需 API Key 与网络，内置 Manager/Gateway mock）
python3 <skill_dir>/evals/run_evals.py --sdk-path "<deps-dir>:<devspace-devbox-src>"
```

## 相关内容

- [DevBox CLI](../cli/index.md)
- [安装 DevBox CLI](../cli/install.md)
- [登录与认证](../cli/authentication.md)
- [选择模板](../guide/templates.md)
- [SDK 参考](../reference/sdk.md)
- [问题排查](../reference/troubleshooting.md)
