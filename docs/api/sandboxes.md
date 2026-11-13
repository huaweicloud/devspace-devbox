# 沙箱 API

请求发送到 `DEVBOX_API_URL`，使用 `X-API-Key`，只操作当前凭证所属 Namespace 的沙箱。公共鉴权、错误与时间格式见[通用约定](./index.md)。

| 操作 | 方法与路径 | 成功响应 |
| --- | --- | --- |
| [创建](#create) | `POST /sandboxes` | 201，沙箱对象 |
| [列表](#list) | `GET /sandboxes` | 200，沙箱数组 |
| [详情](#get) | `GET /sandboxes/{sandboxID}` | 200，沙箱对象 |
| [连接](#connect) | `POST /sandboxes/{sandboxID}/connect` | 200 或 201，沙箱对象及凭证 |
| [暂停](#pause) | `POST /sandboxes/{sandboxID}/pause` | 204 |
| [恢复](#resume) | `POST /sandboxes/{sandboxID}/resume` | 201，沙箱对象及凭证 |
| [重设存活时间](#timeout) | `POST /sandboxes/{sandboxID}/timeout` | 204 |
| [延长存活时间](#refresh) | `POST /sandboxes/{sandboxID}/refreshes` | 204 |
| [删除](#delete) | `DELETE /sandboxes/{sandboxID}` | 204 |

## 创建沙箱 {#create}

`POST /sandboxes`

请求体为 JSON：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `templateID` | string | 是 | [模板 ID 或别名](./templates.md)，如 `default` |
| `buildID` | string | 否 | 固定 Build，省略时使用所选模板的最新可用 Build |
| `timeout` | integer | 否 | 存活秒数，1–86400；省略或传 `0` 使用服务默认值（通常 300 秒） |
| `envVars` | object | 否 | 沙箱环境变量，值为字符串 |
| `metadata` | object | 否 | 业务标签，值为字符串 |
| `autoPause` | boolean | 否 | 到期后暂停而非销毁，默认 false；与 E2B 请求字段一致 |
| `autoResume` | object / boolean | 否 | 推荐 `{"enabled":true}`，也接受 boolean；启用时要求 `autoPause=true` |
| `autoPauseMemory` | boolean | 否 | 只支持 true 或省略，保存完整内存快照 |
| `lifecycle.onTimeout` | string | 否 | `kill` 或 `pause`，默认 `kill` |
| `lifecycle.autoResume` | boolean | 否 | 允许访问时自动恢复，默认 false；启用时 `onTimeout` 必须为 `pause` |

推荐使用 `autoPause` 与 `autoResume`。原 `lifecycle` 请求继续兼容；同时传两套字段时必须一致，否则返回 400。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"templateID":"default","timeout":300,"envVars":{"APP_MODE":"demo"},"metadata":{"project":"example"},"lifecycle":{"onTimeout":"pause","autoResume":true}}'
```

成功返回 HTTP 201 和[沙箱对象](#sandbox)。保存 `sandboxID`；沙箱内请求使用响应中的 `domain` 和 `connectToken`。

模板或指定 Build 不可用时返回 404；资源或依赖暂不可用时可返回 503；请求或资源配额耗尽时返回 429。出现 `tunnel_quota_exceeded` 时，需释放账户下闲置沙箱或隧道。

## 列出沙箱 {#list}

`GET /sandboxes`

| Query 参数 | 类型 | 说明 |
| --- | --- | --- |
| `template` | string | 按模板 ID 或别名精确筛选，如 `tpl-devbox-base-test` 或 `default`；省略时不限制模板 |
| `state` | string | `running`、`paused`，多个状态用逗号分隔；省略时包含两者 |
| `metadata` | string | URL 编码的键值筛选条件，如 `project=example&stage=test`；所有条件须同时满足 |
| `startedAfter` | string | 按本次运行的 `startedAt` 筛选，包含边界；使用带时区的 RFC3339 时间，如 `2026-11-03T00:00:00Z` |
| `order` | string | 按 `startedAt` 排序：`asc` 或 `desc`，默认 `desc`；时间相同时按 sandboxID 升序 |
| `limit` | integer | 每页 1–1000 条；`0` 或省略时返回全部匹配项，建议显式指定 |
| `nextToken` | string | 上一页 `X-Next-Token` 响应头，按原值传回 |

```bash
curl --fail-with-body -i --get "$DEVBOX_API_URL/sandboxes" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  --data-urlencode 'template=default' \
  --data-urlencode 'state=running,paused' \
  --data-urlencode 'metadata=project=example' \
  --data-urlencode 'limit=20'
```

返回 HTTP 200，JSON 为[沙箱对象](#sandbox)数组，默认按本次运行的 `startedAt` 倒序排列；空列表为 `[]`。恢复会更新本次运行的 `startedAt`，时间筛选不是按首次创建时间。

| 响应头 | 说明 |
| --- | --- |
| `X-Next-Token` | 下一页游标；为空表示没有下一页 |
| `X-Total-Running` | 当前 Namespace 的运行数，独立于本次筛选条件和分页，不是返回条数 |

获取下一页时保持筛选条件和排序不变，将游标放入 `nextToken`。列表不是固定快照；沙箱同时创建或删除时，分页结果可能变化。非法时间或排序返回 400。

模板、状态和 metadata 条件同时生效，筛选完成后再分页。没有匹配项时返回 `[]`。

## 查询沙箱 {#get}

`GET /sandboxes/{sandboxID}`

`sandboxID` 为路径参数，必填。后续示例假设已将它保存到 `SANDBOX_ID`。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID" \
  -H "X-API-Key: $DEVBOX_API_KEY"
```

返回 HTTP 200 和[沙箱对象](#sandbox)。已删除、已销毁或无权访问的 ID 返回 404。查询不会延长存活时间，也不会恢复暂停沙箱。

## 连接沙箱 {#connect}

`POST /sandboxes/{sandboxID}/connect`

请求体可省略，或传入 `{"timeout":300}`。`timeout` 为可选整数，单位秒，范围 1–86400。

可选 `resume` 为 boolean，默认 true。传 `{"resume":false}` 只为运行中的沙箱准备凭证；沙箱已暂停时返回 409 `sandbox_paused`，不会恢复。SDK 自动换凭证在未允许自动恢复时使用此选项。主动调用 `connect()` 或 `resume()` 仍可恢复 `autoResume=false` 的沙箱。

`timeout=0` 等同于省略。连接并恢复暂停沙箱时可传 `autoPause`；设为 false 会同时关闭自动唤醒。仅连接运行中的沙箱时不会修改暂停策略。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID/connect" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"timeout":300}'
```

| 当前状态 | 行为 | 成功响应 |
| --- | --- | --- |
| running | 返回访问凭证；指定 timeout 时保证至少剩余相应时间，不缩短期限；省略时不主动延长 | 200 |
| paused | 恢复同一沙箱并返回访问凭证；按 timeout 设置新的运行期限，省略时使用服务默认时长 | 201 |

运行中返回 HTTP 200，执行恢复后返回 HTTP 201，响应均为[沙箱对象](#sandbox)。该接口准备访问入口，实际数据连接由调用者向 `domain` 发起。状态暂不允许操作时返回 409。

## 暂停沙箱 {#pause}

`POST /sandboxes/{sandboxID}/pause`

无请求体。请求等待快照保存与暂停完成后返回 HTTP 204，无响应体；已暂停时也返回 204。

也可传 `{"memory":true}`。暂停、恢复和连接均接受 `memory:true` 或省略；`memory:false` 返回 400 `memory_snapshot_required`，不会静默改成完整内存恢复。

```bash
curl --fail-with-body -X POST "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID/pause" \
  -H "X-API-Key: $DEVBOX_API_KEY"
```

保存内存与文件状态；HTTP、PTY 和监听连接需要客户端重新建立。Manager 不自动过期清理暂停记录，暂停期间不消耗本次运行的 timeout；恢复依赖原节点、快照和隧道可用。完成前查询仍可能显示 running。其他状态不允许暂停时返回 409。

## 恢复沙箱 {#resume}

`POST /sandboxes/{sandboxID}/resume`

请求体可省略，或传入 `{"timeout":300}`。timeout 的单位、范围与创建相同；省略时使用服务默认运行时长。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID/resume" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"timeout":300}'
```

成功返回 HTTP 201 和[沙箱对象](#sandbox)，使用其中最新的访问凭证。恢复后的运行期限重新计算，不承接暂停前剩余秒数；快照中的文件及未结束进程继续保留。当前恢复依赖原节点可用。

运行中的沙箱调用 resume 返回 409；希望兼容运行和暂停两种状态时使用 [connect](#connect)。

## 重设存活时间 {#timeout}

`POST /sandboxes/{sandboxID}/timeout`

请求体必须包含整数 `timeout`，单位秒，范围 1–86400。从现在起重设剩余存活时间，可以延长或缩短，仅用于运行中的沙箱。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID/timeout" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"timeout":7200}'
```

成功返回 HTTP 204，无响应体；状态不允许修改时返回 409。需要确认新期限时调用[查询沙箱](#get)。

## 延长存活时间 {#refresh}

`POST /sandboxes/{sandboxID}/refreshes`

请求体必须包含整数 `duration`，单位秒，范围 1–86400。保证至少剩余指定时间，不累加，也不缩短现有期限，仅用于运行中的沙箱。

```bash
curl --fail-with-body "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID/refreshes" \
  -H "X-API-Key: $DEVBOX_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"duration":300}'
```

成功返回 HTTP 204，无响应体。例如已经剩余 600 秒，refresh 300 不改变期限；set timeout 300 则会缩短为剩余 300 秒。

## 删除沙箱 {#delete}

`DELETE /sandboxes/{sandboxID}`

```bash
curl --fail-with-body -X DELETE "$DEVBOX_API_URL/sandboxes/$SANDBOX_ID" \
  -H "X-API-Key: $DEVBOX_API_KEY"
```

成功返回 HTTP 204，无响应体，释放运行实例、暂停记录与隧道；之后不能再恢复。已经终止但仍保有记录时也可返回 204，资源不存在时返回 404。客户端清理流程可将这两者都视为已释放。

## 沙箱对象 {#sandbox}

```json
{
  "sandboxID": "<sandbox-id>",
  "templateID": "<template-id>",
  "state": "running",
  "startedAt": "2026-10-21T08:00:00Z",
  "endAt": "2026-10-21T08:05:00Z",
  "cpuCount": 2,
  "memoryMB": 1024,
  "diskSizeMB": 2040,
  "metadata": {"project": "example"},
  "lifecycle": {"onTimeout": "pause", "autoResume": true},
  "domain": "https://<沙箱访问域名>",
  "tunnelId": "<tunnel-id>",
  "connectToken": "<connect-token>",
  "tokenExpiration": 1792656000
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `sandboxID` / `templateID` | string | 沙箱和来源模板 ID |
| `state` | string | `running` 或 `paused`；销毁后通过 404 表示不可访问 |
| `startedAt` | string | 本次运行开始时间，恢复后更新 |
| `endAt` | string | 运行状态为存活截止时间；暂停状态保留上次运行截止时间，仅用于兼容客户端，不表示暂停记录的删除期限 |
| `cpuCount` | integer | CPU 核数 |
| `memoryMB` / `diskSizeMB` | integer | 内存与磁盘规格，单位 MiB |
| `metadata` | object | 业务标签，可省略 |
| `lifecycle` | object | `onTimeout` 与 `autoResume` 配置 |
| `domain` | string | 完整沙箱 HTTPS 地址 |
| `tunnelId` | string | 隧道标识；正常访问使用 domain 即可 |
| `connectToken` | string | 沙箱访问凭证，用于 relay_token Cookie |
| `envdAccessToken` | string | 与 connectToken 相同的凭证；DevBox SDK 同时使用 X-Access-Token 头和兼容 Cookie |
| `envdUrl` | string | 数据面完整 HTTPS 地址；与原接口的 domain 相同 |
| `envdVersion` | string | 当前 EnvD 协议版本 |
| `clientID` | string | E2B 兼容字段，当前为空字符串，不作为身份或权限依据 |
| `tokenExpiration` | integer | 访问凭证的 Unix 到期秒数 |

暂停对象不包含 `domain`、`tunnelId`、`connectToken`、`tokenExpiration`。读取状态不会触发恢复；需要访问入口时调用 connect。

## 访问应用端口 {#application-port}

创建沙箱时设置 `network.allowPublicTraffic=false`，应用端口访问需要连接凭证。向应用端口的 HTTPS 地址发送请求时，传入 `X-Access-Token: <envdAccessToken>`；未带凭证返回 401。DevBox SDK 的写法见[创建参数](../reference/sdk.md#创建参数)。

## 使用 E2B 客户端 {#e2b}

兼容 `POST /v2/sandboxes`、`GET /v2/sandboxes`、`POST /v2/sandboxes/{sandboxID}/connect`，复用上述业务逻辑、鉴权和配额。E2B 接口返回的 `sandboxID` 使用固定 tunnelId 作为公开别名；单沙箱操作同时接受该别名与原 UUID，仍按当前凭证所属 Namespace 隔离。

E2B 客户端使用响应中的基础域名 `domain` 拼接端口地址。直接访问 EnvD 时使用完整的 `envdUrl`。列表可按 metadata 筛选，包括中文和特殊字符。

Python 客户端可通过 `get_host(8080)` 获取[应用端口地址](#application-port)。执行命令时省略 `user` 会使用沙箱默认用户。

仅支持完整内存快照。其他非 null 网络字段（例如 `dnsServers`、`httpsPorts`）或 `allow_internet_access:false` 返回 400 `network_policy_unsupported`。创建时非 null 的 `cpuCount`、`volumeMounts`、`iam`、`mcp` 或非空 `Idempotency-Key` 返回 400 `unsupported_create_option`；CPU 规格由模板决定，不支持创建幂等。

恢复请求也接受 `autoPause`；省略时保留原生命周期策略。DevBox 的 `timeout=0` 和 `limit=0` 是扩展约定；不支持 E2B 的冷恢复、fork、历史指标、沙箱日志或动态网络更新接口。
