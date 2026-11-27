# Skill自动管理云沙箱

安装 `huawei-cloud-devbox-sandbox` 后，支持 Agent Skills 的 AI 代理（如 OpenCode）可以用自然语言完成云沙箱的创建、命令执行、文件传输与清理。本文以一次「把脚本放进沙箱运行并取回结果」的完整任务为例，流程已在真机端到端验证。

## 安装 Skill

```bash
npx skills add https://gitcode.com/huaweicloud/huaweicloud-skills.git \
  --skill huawei-cloud-devbox-sandbox
```

安装后 skill 位于 `~/.agents/skills/huawei-cloud-devbox-sandbox`。代理首次使用时会自动安装 DevBox CLI；如果代理的 Shell 提示找不到 `devbox` 命令，先完成下节的 PATH 配置。

## 准备环境（两个必做步骤）

**第一步：注入 API Key。** 按 [获取 API Key](../cli/authentication.md#获取-api-key) 创建凭证，在启动代理的终端注入。不要把 API Key 贴进对话、代码或提交到仓库。

```bash
export HW_DEVBOX_API_KEY="devbox_xxx"    # CLI 通道
export DEVBOX_API_KEY="devbox_xxx"       # JS / Python SDK 通道
```

**第二步：确认 PATH 与登录状态。** CLI 安装在 `~/.huawei/bin`，代理的非交互 Shell 可能不包含该路径：

```bash
export PATH="$HOME/.huawei/bin:$PATH"
```

再验证登录。注意按输出文本判断——`devbox auth status` 未登录时也返回退出码 0，只看退出码会误判：

```bash
devbox auth status    # 输出包含 "Logged in" 才是已登录
```

未登录时执行 `devbox auth login --api-key "$HW_DEVBOX_API_KEY"` 持久化凭证。

## 让代理完成一次任务

告诉代理：

> 帮我创建一个云沙箱，把当前目录的 stats.py 上传进去运行，把输出发给我

代理会自动执行以下流程，未指定的参数使用默认值，不会逐个确认：

1. 运行 skill 内置的 `devbox_check.py` 自检，缺失时安装 CLI 并确认认证；
2. 列模板确认别名 `default` 可用（解析为 `tpl-devbox-base`）；
3. 创建沙箱：`devbox sbx create default -d`，`-d` 只回显沙箱 ID，默认有效期 300 秒；
4. 上传文件并运行（上传方式见下节）；
5. 回传命令输出；
6. 删除沙箱，并用 `devbox sbx ls` 复查确认无残留。

代理回传的输出示例：

```text
n=1000
mean=49.781
median=49.774
stdev=12.229
min=12.145 max=95.025
top10=95.02, 89.96, 87.98, 86.43, 79.13, 78.49, 78.09, 78.03, 77.86, 77.79
```

## 上传文件：CLI 通道没有上传命令

DevBox CLI 目前没有 `cp` / `upload` 类命令，上传文件让代理按下面任一方式处理。

**方式一：base64 经 `sbx ex` 塞入**（CLI 通道）：

```bash
B64=$(base64 -w0 stats.py)    # macOS 的 base64 默认不换行，可省略 -w0
devbox sbx ex "$SBX" -- bash -c "echo '$B64' | base64 -d > /tmp/stats.py"
```

上传后用 `wc -c` 比对本地与沙箱内的字节数，一致后再运行。

**方式二：SDK 的文件 API**（推荐，绝对路径）：

```python
sandbox.files.write("/tmp/stats.py", Path("stats.py").read_text())
```

涉及文件上传 / 下载的任务，直接让代理选择 Python / JS SDK 通道完成。

## 常用的一句话操作

| 你对代理说 | 代理的动作 |
| --- | --- |
| 开个沙箱终端 | 创建沙箱并进入交互式终端 |
| 列出我所有的沙箱 | 过滤并展示运行中 / 暂停的沙箱 |
| 暂停这个沙箱，明天继续用 | 暂停并保留状态，24 小时内可恢复 |
| 把刚才的沙箱删掉 | 删除指定沙箱及其数据 |

需要指定模板或时长时直接说明，例如「用 OpenCode 模板，有效期 1 小时」。可用模板见[选择模板](../guide/templates.md)。

注意破坏性边界：「删掉**所有**沙箱」对应 `devbox sbx kill --all`，会删除全部匹配的沙箱。这类批量删除不在「零确认」范围内：要求代理先 `sbx ls` 列出清单，经你确认后再删。

## 让代理生成集成代码

要把沙箱嵌入自己的程序时，让代理用 SDK 写代码：

> 用 DevBox Python SDK 写个脚本，在沙箱里克隆仓库并跑测试

代理按 skill 内置的 SDK 参考生成可运行代码，并遵循其约束：Python `with` 退出即删除沙箱、文件 API 仅接受绝对路径、`timeout` 单位为秒（JS 请求选项的 `timeoutMs` 为毫秒）。Node.js 项目用 JS SDK，脚本与自动化用 Python SDK。接口细节见 [SDK 参考](../reference/sdk.md)。

## 实践要点

- **说意图，不说命令**。讲清目标（跑脚本、开终端、嵌代码），skill 会自动选择通道并套用默认参数；直接拼 CLI 命令容易绕过这些保护。
- **管好生命周期**。默认 300 秒即用即删，删完 `sbx ls` 复查；更长任务显式说明时长（最长 24 小时）；要保留就明确说「暂停」（保留 24 小时），重要结果先取回本地。
- **别只信退出码**。未登录的 `auth status` 和拼错的子命令都返回 0，脚本化时按输出文本校验。
- **凭证只走环境变量**。不把 API Key 贴进对话，也不存进易失的沙箱文件。
- **失败时把错误贴回对话**。401/403 先检查环境变量、重新签发 Key 再重试；限流错误按提示退避。

## 验证

安装后可运行 skill 内置脚本做端到端验证（创建沙箱 → 执行命令 → 删除沙箱）：

```bash
python3 ~/.agents/skills/huawei-cloud-devbox-sandbox/scripts/devbox_check.py --smoke
```

前置条件：真实可用的 API Key 与网络。未设置 Key 时脚本直接以退出码 2 结束——这不是 skill 故障，先完成「准备环境」一节再重试。自检下载 `install.sh` 失败（个别源需鉴权）时，设置 `DEVBOX_CLI_INSTALL_URL` 指向可达镜像（如 GitHub release）后重试。

## 相关内容

- [AI Agent Skill](../skill/index.md) — skill 的能力清单、关键约束与安装细节
- [安装 DevBox CLI](../cli/install.md)
- [OpenCode模板沙箱创作个人网站](./opencode-web-demo.md) — 在沙箱内运行 OpenCode 生成网页的完整示例
- [问题排查](../reference/troubleshooting.md)
