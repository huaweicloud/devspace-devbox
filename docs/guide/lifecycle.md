# 沙箱生命周期

## 存活时间

创建时 `timeout` 为沙箱存活时间，单位秒，默认 300，范围 1–86400。默认到期销毁；命令还在运行不会自动延长这个期限。

| 调用 | 含义 |
| --- | --- |
| `set_timeout(300)` / `setTimeout(300)` | 从现在起重设剩余 300 秒，可缩短 |
| `refresh(300)` | 保证至少还剩 300 秒，只延长、不累加 |
| `connect(id, timeout=300)` | 连接已有沙箱；运行中只延长，暂停中先恢复 |
| `resume(timeout=300)` | 恢复暂停的沙箱，重新设置运行时长 |

例如原本剩余 600 秒，`refresh(300)` 不改变期限，`set_timeout(300)` 则缩短为剩余 300 秒。存活时间与单次网络请求超时是不同概念。

## 手动暂停与恢复

暂停保存内存和磁盘状态，恢复后仍是同一个沙箱，原有文件及未结束的进程可继续使用。

::: code-group
```python [Python]
from devbox import Sandbox

sandbox = Sandbox.create("default", timeout=300)
try:
    sandbox.files.write("/tmp/message.txt", "saved before pause")
    sandbox.pause()
    sandbox.resume(timeout=300)
    print(sandbox.files.read("/tmp/message.txt"))
finally:
    try:
        sandbox.kill()
    finally:
        sandbox.close()
```

```js [JavaScript / TypeScript]
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default", { timeout: 300 });
try {
  await sandbox.files.write("/tmp/message.txt", "saved before pause");
  await sandbox.pause();
  await sandbox.resume({ timeout: 300 });
  console.log(await sandbox.files.read("/tmp/message.txt"));
} finally {
  try { await sandbox.kill(); } finally { await sandbox.close(); }
}
```
:::

恢复时明确指定新的存活时间，不是接着扣除暂停前剩余的秒数。Manager 不自动过期清理暂停记录，恢复依赖原节点、快照和隧道可用。暂停状态的 endAt 保留上次运行截止时间以兼容 E2B 客户端，不表示暂停记录的删除期限。

需要稍后在另一个程序中恢复时，保存 `sandbox_id` / `sandboxId`，暂停后只调用 `close()`。随后通过 `Sandbox.connect(id)` 恢复并获取可用对象。

::: warning 本地连接不随快照保存
暂停前关闭命令、PTY 和监听流。恢复后按 PID 重连仍在运行的进程，重新建立监听。SDK 不回放断开期间的输出，也不自动重放可能已执行过的命令。
:::

## 到期暂停与自动恢复

创建时配置到期暂停，并允许访问时自动恢复：

::: code-group
```python [Python]
from devbox import Sandbox, SandboxLifecycle

sandbox = Sandbox.create(
    "default",
    timeout=300,
    lifecycle=SandboxLifecycle(on_timeout="pause", auto_resume=True),
)
```

```js [JavaScript / TypeScript]
import { Sandbox } from "devbox-js-sdk";

const sandbox = await Sandbox.create("default", {
  timeout: 300,
  lifecycle: { onTimeout: "pause", autoResume: true },
});
```
:::

自动暂停由服务端定期检查触发，不会在截止时间瞬间完成。实际暂停完成前仍显示 `running`，完成后显示 `paused`。启用自动恢复且服务支持该能力时，再次执行命令或访问文件会触发恢复，首次请求比正常请求慢。

也可以显式调用 `resume()`，或在另一个程序中通过 `Sandbox.connect(id)` 恢复。手动生命周期调用不应被当作无条件可重试操作；失败后先查询状态，避免重复执行用户任务。

## 查询与释放

`get_info()` / `getInfo()` 获取实时状态；`is_running()` / `isRunning()` 查询是否运行，不代表应用已就绪。

默认销毁模式到期后，查询可能返回资源不存在（404）。不要依赖一定存在一个可查询的 `stopped` 状态。远端资源由服务清理，本地程序仍需调用 `close()`。

`kill()` 主动删除沙箱，也可用于重复清理；服务确认删除时返回 `True` / `true`，资源不存在时返回 `False` / `false`。暂停不等于删除，不再需要的暂停沙箱也应主动删除。
