# 交互式终端与 Git

## 交互式终端

PTY 为交互式程序提供终端环境，适合 Shell、带颜色输出以及需要双向输入的程序。普通脚本优先使用 `commands.run()`。

::: code-group
```python [Python]
from devbox import PtySize

session = sandbox.pty.start(size=PtySize(rows=30, cols=100))
try:
    session.send_stdin("pwd\nstty size\nexit 0\n")
    result = session.wait()
    print(result.stdout)
finally:
    session.disconnect()
```

```js [JavaScript / TypeScript]
const session = await sandbox.pty.start("/bin/bash", {
  size: { rows: 30, cols: 100 },
});
try {
  await session.sendStdin("pwd\nstty size\nexit 0\n");
  console.log((await session.wait()).stdout);
} finally {
  await session.disconnect();
}
```
:::

`rows` 与 `cols` 是终端行列数，会影响程序换行与布局。窗口大小改变时，用 `sandbox.pty.resize(pid, size)` 更新。

可用 `user` 和 `cwd` 指定会话用户和目录。例如 Python `sandbox.pty.start(user="nobody", cwd="/tmp")`，JS `await sandbox.pty.start("/bin/bash", { user: "nobody", cwd: "/tmp" })`。省略 `user` 时使用沙箱默认用户。

终端输出包含 ANSI 颜色、光标或标题控制序列，不是纯文本日志。构建交互界面时应交给终端渲染组件；只需纯文本时优先使用命令接口。

`exit` 结束远端 Shell；`disconnect()` 只断开输出连接。进程退出后不能继续发送输入，但仍可读取已收集的退出结果。

## Git 仓库

Git 操作在沙箱内执行，需要模板已安装 Git，且沙箱能访问目标仓库。仓库权限与 DevBox API Key 是两套不同凭证。

::: code-group
```python [Python]
result = sandbox.git.clone(
    "https://github.com/huaweicloud/devspace-devbox.git",
    "/tmp/devspace-devbox",
    depth=1,
)
print(result.stdout, result.stderr)
print(sandbox.git.status("/tmp/devspace-devbox").stdout)
```

```js [JavaScript / TypeScript]
const result = await sandbox.git.clone(
  "https://github.com/huaweicloud/devspace-devbox.git",
  "/tmp/devspace-devbox",
  { depth: 1 },
);
console.log(result.stdout, result.stderr);
console.log((await sandbox.git.status("/tmp/devspace-devbox")).stdout);
```
:::

同时提供 `checkout`、`add`、`commit`、`pull`、`push` 及 Git 配置方法。提交前需设置仓库中的 `user.name` 和 `user.email`。私有仓库可通过方法的 `credentials` 参数传入凭证，不要把凭证拼入仓库 URL 或打印出来。
