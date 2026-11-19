---
title: 沙箱管理
description: 创建、连接、查询和管理 DevBox 沙箱的生命周期。
---

# 沙箱管理

<p class="lead">命令组 <code>devbox sandbox</code> 覆盖沙箱的创建、连接、查询、生命周期管理与远程命令执行。</p>

## 创建沙箱

从模板创建一个新的沙箱，成功后默认直接进入交互式终端：

```bash
devbox sandbox create <templateID>
```

| 参数 | 说明 |
| --- | --- |
| `-t, --timeout` | 沙箱存活时间，单位秒（10-86400，默认 300） |
| `-c, --cwd` | 终端会话在沙箱内的工作目录 |
| `-e, --env` | 终端会话的环境变量，`key=value`，可重复 |
| `-m, --metadata` | 沙箱业务标签，`key=value`，可重复 |
| `-b, --build-id` | 固定模板构建版本 |
| `-d, --detach` | 只创建沙箱，不进入终端 |
| `--lifecycle.ontimeout` | 到期动作：`pause` 或 `kill` |
| `--lifecycle.autoresume` | 到期暂停后允许网关自动恢复，需配合 `--lifecycle.ontimeout pause` |
| `--network.allow-public-traffic=false` | 应用端口访问需要连接凭证；省略此选项时允许匿名访问应用端口 |

### 只创建不进终端

自动化场景加上 `-d/--detach`，创建成功后只打印沙箱 ID：

```bash
devbox sandbox create <templateID> -d -t 600
```

## 连接沙箱

连接运行中的沙箱，暂停的沙箱会自动恢复。本地终端成为沙箱 shell，输入 `exit` 退出：

```bash
devbox sandbox connect <sandboxID>
devbox sandbox connect <sandboxID> -c /tmp -e MYTEST=123
```

| 参数 | 说明 |
| --- | --- |
| `-t, --timeout` | 存活时间，单位秒（10–86400）；省略时，运行中的沙箱不延长，暂停的沙箱恢复后使用服务默认时长 |
| `-c, --cwd` | 终端会话在沙箱内的工作目录 |
| `-e, --env` | 终端会话的环境变量，`key=value`，可重复 |

终端内 Ctrl+C 透传给远端进程；拖动窗口改变大小时，尺寸会自动同步到远端。

## 列出沙箱

列出沙箱，支持按模板、状态、元数据与启动时间过滤，并可按启动时间排序：

```bash
devbox sandbox list
devbox sandbox list -t <templateID> -s running -m "user=abc"
devbox sandbox list --started-after 2025-01-01T00:00:00Z -o asc
devbox sandbox list -f json
```

| 参数 | 说明 |
| --- | --- |
| `-t, --template` | 按模板 ID 或别名过滤 |
| `-s, --state` | 按状态过滤：`running` / `paused`，可重复 |
| `-m, --metadata` | 按元数据过滤 |
| `--started-after` | 只返回在该时间点（含）之后启动的沙箱，ISO 8601 格式，如 `2025-01-01T00:00:00Z` |
| `-o, --order` | 按启动时间排序：`asc` / `desc`（默认 `desc`） |
| `-l, --limit` | 返回数量上限，0 表示不限制（默认 100，上限 1000） |
| `-f, --format` | 输出格式：`pretty` 或 `json` |

## 查看沙箱详情

查看单个沙箱的详情（状态、存活时间、资源规格、生命周期、连接信息）：

```bash
devbox sandbox info <sandboxID>
devbox sandbox info <sandboxID> -f json
```

## 暂停沙箱

暂停会保存沙箱的内存与磁盘状态：

```bash
devbox sandbox pause <sandboxID>
```

## 恢复沙箱

恢复已暂停的沙箱，数据不丢失：

```bash
devbox sandbox resume <sandboxID>
devbox sandbox resume <sandboxID> -t 600
```

| 参数 | 说明 |
| --- | --- |
| `-t, --timeout` | 恢复后的存活时间，单位秒（10–86400）；省略时使用服务默认时长 |

## 重设存活时间

重新设置沙箱的存活时间：

```bash
devbox sandbox timeout <sandboxID> -t 600
```

| 参数 | 说明 |
| --- | --- |
| `-t, --timeout` | 新的存活时间，单位秒（10-86400，默认 300） |

## 延长存活时间

保证沙箱至少再存活指定秒数：

```bash
devbox sandbox refresh <sandboxID> -d 300
```

| 参数 | 说明 |
| --- | --- |
| `-d, --duration` | 存活时间延长秒数（10-86400，默认 300） |

## 删除沙箱

删除单个或多个沙箱：

```bash
devbox sandbox kill <sandboxID>
devbox sandbox kill <id1> <id2>
```

也可以按条件批量删除所有匹配的沙箱：

```bash
devbox sandbox kill --all                                # 删除所有 running 沙箱
devbox sandbox kill --all -s paused -m "user=abc"        # 按状态与元数据过滤
```

| 参数 | 说明 |
| --- | --- |
| `-a, --all` | 删除所有沙箱（默认 running 状态），与 sandbox ID 互斥 |
| `-s, --state` | 与 `--all` 一起使用时按状态过滤，可重复 |
| `-m, --metadata` | 与 `--all` 一起使用时按元数据过滤 |

## 执行远程命令

在运行中的沙箱里执行一次性命令，远程输出实时透传：

```bash
devbox sandbox exec <sandboxID> -- uname -a
devbox sandbox exec <sandboxID> -c /tmp -- ls -la
devbox sandbox exec <sandboxID> -e GO_ENV=dev -- go version
```

| 参数 | 说明 |
| --- | --- |
| `-b, --background` | 后台执行：打印 PID 到 stderr 并立即返回 |
| `-c, --cwd` | 远程命令的工作目录 |
| `-e, --env` | 远程命令的环境变量，`key=value`，可重复 |

sandbox ID 之后的全部内容透传给远程命令，devbox 选项必须放在 sandbox ID 之前。前台执行时远程命令的退出码作为本地退出码；管道内容会作为远程命令的 stdin。
