---
title: DevBox CLI
description: DevBox 命令行客户端的安装、认证与沙箱管理。
---

# DevBox CLI

<p class="lead">DevBox CLI 是沙箱管理服务的端侧命令行客户端，覆盖沙箱全生命周期、交互式终端与远程命令执行。</p>

## 命令组

| 命令组 | 简写 | 用途 |
| --- | --- | --- |
| `devbox auth` | — | 登录、查看状态、退出 |
| `devbox sandbox` | `sbx` | 沙箱全生命周期管理 |
| `devbox template` | `tpl` | 查看模板 |

每个子命令都有简写，例如 `devbox sandbox ls`、`devbox sandbox cn <id>`、`devbox sandbox kl <id>`。

## 全局选项

| 选项 | 说明 |
| --- | --- |
| `-v, --verbose` | 打印调试日志（含接口请求/返回，API Key 自动脱敏） |
| `--version` | 显示版本号 |

## 界面语言

CLI 提示信息支持中英文，通过环境变量 `DEVBOX_LANG` 切换：

```bash
export DEVBOX_LANG=zh    # 中文提示
```

取值以 `zh` 开头为中文，其他值或未设置时为英文（默认）。

## 配置文件

配置文件位于 `~/.huawei/devbox/config.json`，存储用户信息等本地状态。API Key 优先存放在 OS keyring，不落在配置文件中。

## 目录

- [安装 DevBox CLI](./install)
- [登录与认证](./authentication)
- [模板](./templates)
- [沙箱管理](./sandboxes)
