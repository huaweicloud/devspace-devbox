---
title: 安装 DevBox CLI
description: 使用官方安装脚本安装并验证 DevBox CLI。
---

# 安装 DevBox CLI

<p class="lead">DevBox CLI 提供沙箱全生命周期、交互式终端与远程命令执行命令。</p>

## 系统要求

- Bash 和 `curl`，或者 PowerShell 5.1 及以上版本；
- x86-64 或 ARM64 架构；
- 可以访问 GitHub、GitCode 或 OBS 安装源；
- 对 `~/.huawei` 目录具有写权限。

## 安装

### Bash

任选一个渠道运行安装脚本：

::: code-group

```bash [GitHub]
curl -fsSL https://github.com/huaweicloud/devspace-devbox/releases/latest/download/install.sh | bash
```

```bash [GitCode]
curl -fsSL https://gitcode.com/CloudDeveloperDepartment/devbox/releases/download/latest/install.sh | bash
```

```bash [OBS]
curl -fsSL https://tools-artifact.developer.huaweicloud.com/sharedata/devbox/install.sh | bash
```

:::

### PowerShell

任选一个渠道运行安装脚本：

::: code-group

```powershell [GitHub]
irm https://github.com/huaweicloud/devspace-devbox/releases/latest/download/install.ps1 | iex
```

```powershell [GitCode]
irm https://gitcode.com/CloudDeveloperDepartment/devbox/releases/download/latest/install.ps1 | iex
```

```powershell [OBS]
irm https://tools-artifact.developer.huaweicloud.com/sharedata/devbox/install.ps1 | iex
```

:::

标准安装会安装当前发布版本并使用默认目录，不需要指定版本或下载源。

| 内容           | 默认位置                |
| -------------- | ----------------------- |
| CLI 可执行文件 | `~/.huawei/bin`         |
| CLI 配置与状态 | `~/.huawei/devbox`      |

## 配置 PATH

如果终端无法直接找到 `devbox`，在当前会话执行：

```bash
export PATH="$HOME/.huawei/bin:$PATH"
```

如果你使用 PowerShell，则执行：

```powershell
$env:Path = "$HOME/.huawei/bin;$env:Path"
```

若要对后续终端持续生效，把相应命令加入 Shell 启动文件。Bash 或 Zsh 可写入 `~/.bashrc` 或 `~/.zshrc`。

## 验证安装

```console
devbox --version
devbox --help
```

第一条命令显示当前 CLI 版本；第二条命令显示当前版本支持的命令。
