# 文件操作

文件路径指向沙箱内的 Linux 文件系统。`upload` 和 `download` 另外接收本地文件路径。

## 目录与文本

::: code-group
```python [Python]
sandbox.files.make_dir("/tmp/work")
sandbox.files.write("/tmp/work/input.txt", "hello")
print(sandbox.files.read("/tmp/work/input.txt"))
print(sandbox.files.stat("/tmp/work/input.txt"))
for entry in sandbox.files.list("/tmp/work"):
    print(entry.name)
sandbox.files.move("/tmp/work/input.txt", "/tmp/work/output.txt")
sandbox.files.remove("/tmp/work/output.txt")
```

```js [JavaScript / TypeScript]
await sandbox.files.makeDir("/tmp/work");
await sandbox.files.write("/tmp/work/input.txt", "hello");
console.log(await sandbox.files.read("/tmp/work/input.txt"));
console.log(await sandbox.files.stat("/tmp/work/input.txt"));
for (const entry of await sandbox.files.list("/tmp/work")) {
  console.log(entry.name);
}
await sandbox.files.move("/tmp/work/input.txt", "/tmp/work/output.txt");
await sandbox.files.remove("/tmp/work/output.txt");
```
:::

## 上传与下载

先在本地准备 `input.bin`：

::: code-group
```python [Python]
sandbox.files.upload("./input.bin", "/tmp/input.bin")
sandbox.files.download("/tmp/input.bin", "./output.bin")
```

```js [JavaScript / TypeScript]
await sandbox.files.upload("./input.bin", "/tmp/input.bin");
await sandbox.files.download("/tmp/input.bin", "./output.bin");
```
:::

大文件传输应为沙箱预留足够的存活时间；请求超时与存活时间分别配置，见[SDK 参考](../reference/sdk.md#时间单位)。

## 监听目录变化

监听不是一次性查询，而是持续的事件流。请预先创建目录；结束后关闭监听以释放连接。

::: code-group
```python [Python]
from contextlib import closing

with closing(sandbox.files.watch("/tmp/work")) as events:
    for event in events:
        print(event)
        if event.name == "done.txt":
            break
```

```js [JavaScript / TypeScript]
for await (const event of sandbox.files.watch("/tmp/work")) {
  console.log(event);
  if (event.name === "done.txt") break;
}
```
:::

在另一任务中修改目录才会产生事件。沙箱暂停、销毁或连接断开后，不应继续依赖原监听流；恢复后重新建立监听。
