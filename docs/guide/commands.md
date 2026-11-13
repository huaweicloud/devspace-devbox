# 命令与进程

以下示例中的 `sandbox` 来自[创建沙箱](../index.md#创建并使用沙箱)。

## 等待执行结果

`run()` 默认等待命令结束，返回标准输出、标准错误和退出码。

::: code-group
```python [Python]
result = sandbox.commands.run("uname -a")
print(result.stdout)
print(result.stderr)
print(result.exit_code)
```

```js [JavaScript / TypeScript]
const result = await sandbox.commands.run("uname -a");
console.log(result.stdout, result.stderr, result.exitCode);
```
:::

命令默认最多等待 60 秒。Python 使用 `timeout=120`，JS 使用 `{ timeoutMs: 120000 }` 调整等待时间；这不会延长沙箱存活时间。

## 实时输出

::: code-group
```python [Python]
result = sandbox.commands.run(
    "printf 'step 1\n'; sleep 1; printf 'step 2\n'",
    on_stdout=lambda chunk: print(chunk, end=""),
    on_stderr=lambda chunk: print(chunk, end=""),
)
```

```js [JavaScript / TypeScript]
const result = await sandbox.commands.run(
  "printf 'step 1\n'; sleep 1; printf 'step 2\n'",
  {
    onStdout: (chunk) => { process.stdout.write(chunk); },
    onStderr: (chunk) => { process.stderr.write(chunk); },
  },
);
```
:::

## 后台运行与输入

后台模式返回进程句柄。需要进程读到输入结束时调用 `close_stdin()` / `closeStdin()`，它不是退出终端的命令。

::: code-group
```python [Python]
process = sandbox.commands.run("cat", background=True, stdin=True)
try:
    print("PID:", process.pid)
    process.send_stdin("hello\n")
    process.close_stdin()
    result = process.wait()
    print(result.stdout)
finally:
    process.disconnect()
```

```js [JavaScript / TypeScript]
const command = await sandbox.commands.run("cat", {
  background: true,
  stdin: true,
});
try {
  console.log("PID:", command.pid);
  await command.sendStdin("hello\n");
  await command.closeStdin();
  console.log((await command.wait()).stdout);
} finally {
  await command.disconnect();
}
```
:::

## 断开与重连

`disconnect()` 只断开输出流，不结束远端进程。保存 PID 后，可以通过 `sandbox.commands.connect(pid)` 重新订阅输出，再调用 `wait()` 获取退出结果。

重连不会回放断开期间的输出。重要结果应写入沙箱文件，再通过文件接口读取。列出现有进程使用 `sandbox.commands.list()`；结束进程使用句柄的 `send_signal("SIGTERM")`，JS 对应 `sendSignal("SIGTERM")`。

## 非零退出码

非零退出码默认抛出 `CommandExitError`。需要自行判断时，Python 传入 `check=False`，JS 传入 `{ check: false }`。

## 指定执行用户

省略 `user` 时使用沙箱默认用户。指定的用户须已存在，并有权访问 `cwd`。

::: code-group
```python [Python]
result = sandbox.commands.run("id", user="nobody", cwd="/tmp")
```

```js [JavaScript / TypeScript]
const result = await sandbox.commands.run("id", { user: "nobody", cwd: "/tmp" });
```
:::

不要将未受信任的输入直接拼接到 Shell 命令中。
