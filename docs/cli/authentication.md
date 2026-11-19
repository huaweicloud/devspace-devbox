---
title: 登录与认证
description: 使用浏览器登录或 API Key 登录 DevBox。
---

# 登录与认证

<p class="lead">在创建或连接沙箱前，先使用 DevBox CLI 建立登录状态。</p>

## 浏览器登录

个人开发环境优先使用浏览器登录：

```bash
devbox auth login
```

CLI 会打开浏览器引导你完成身份认证。登录页会把 API Key 和用户信息回传到本地回调端口，CLI 校验通过后存储：API Key 优先存入系统密钥环（keyring），用户信息写入配置文件。

## 使用 API Key

固定服务账号或自动化任务可以使用 API Key 直接登录：

```bash
devbox auth login --api-key "$HW_DEVBOX_API_KEY"
```

也可以通过环境变量注入，所有命令会自动读取：

```bash
export HW_DEVBOX_API_KEY="your-api-key"
```

不要把真实 API Key 直接写进命令、脚本或流水线定义。应通过受保护的环境变量或密钥服务注入。

### 获取 API Key

你可以在 API Key 管理页面创建和管理 API Key：

> **API Key 管理页面**：<https://devstation.connect.huaweicloud.com/space/devbox/apikey>

登录时若提示 API Key 数量超上限，请前往管理页面删除不再使用的 Key 后重试。

## 凭据解析优先级

所有命令按以下顺序解析 API Key：

1. 命令行或登录时显式指定的 Key；
2. 环境变量 `HW_DEVBOX_API_KEY`；
3. OS keyring 中存储的凭据；
4. 配置文件 `~/.huawei/devbox/config.json`（兜底）。

## 查看登录状态

```bash
devbox auth status
```

状态命令会调用校验接口确认当前 Key 是否有效，并显示用户名；不应输出完整 API Key。

## 退出登录

```bash
devbox auth logout
```

退出会清除本地保存的 API Key 和用户信息，但不会影响已经创建的沙箱。
