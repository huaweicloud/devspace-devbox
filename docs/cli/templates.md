---
title: 模板
description: 查看可用的沙箱模板与模板字段说明。
---

# 模板

<p class="lead">创建沙箱前，先用 <code>devbox template</code>（简写 <code>tpl</code>）查看当前可用的沙箱模板。</p>

## 查看可用模板

```bash
devbox tpl list
devbox tpl ls -f json
```

| 参数 | 说明 |
| --- | --- |
| `-f, --format` | 输出格式：`pretty` 或 `json` |

返回字段：

| 字段 | 说明 |
| --- | --- |
| Template ID | 模板 ID，创建沙箱时使用 |
| Name | 模板名称 |
| Aliases | 模板别名，可代替模板 ID |
| Build ID | 当前构建版本 |
| VCPUS / RAM(MB) / Disk(MB) | 模板默认资源规格 |

## 使用模板创建沙箱

`create` 的模板参数接受模板 ID 或别名：

::: code-group

```bash [模板 ID]
devbox sandbox create <templateID>
```

```bash [别名]
devbox sandbox create <templateAlias>
```

:::
