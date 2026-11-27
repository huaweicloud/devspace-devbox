/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package i18n

func init() {
	Msg = Messages{
		Common:   commonMessages(),
		Auth:     authMessages(),
		Sandbox:  sandboxMessages(),
		Template: templateMessages(),
		Client:   clientMessages(),
		Parse:    parseMessages(),
		Config:   configMessages(),
		Output:   outputMessages(),
	}
}

// commonMessages returns the localized strings of the common messages.
func commonMessages() CommonMessages {
	return CommonMessages{
		RootShort: Message{
			ZH: "DevBox 沙箱管理器命令行客户端",
			EN: "command-line client for the DevBox sandbox manager",
		},
		RootLong: Message{
			ZH: "devbox 是 DevBox 沙箱管理器的命令行客户端。\n\n" +
				"覆盖完整的沙箱生命周期（创建、列表、信息、终止、暂停、恢复、连接终端、执行命令）与模板。",
			EN: "devbox is a command-line client for the DevBox Sandbox Manager API.\n\n" +
				"It covers the full sandbox lifecycle (create, list, info, kill, pause, " +
				"resume, connect, exec) and templates.",
		},
	}
}

// authMessages returns the localized strings of the auth messages.
func authMessages() AuthMessages {
	var msgs AuthMessages
	initAuthSessionMessages(&msgs)
	initAuthStatusMessages(&msgs)
	initAuthCallbackMessages(&msgs)
	return msgs
}

// initAuthSessionMessages fills the auth message group session.
func initAuthSessionMessages(msgs *AuthMessages) {
	msgs.AuthShort = Message{
		ZH: "登录 DevBox 沙箱管理器",
		EN: "authenticate to the DevBox sandbox manager",
	}
	msgs.LoginShort = Message{
		ZH: "登录 CLI",
		EN: "log in to CLI",
	}
	msgs.LoginLong = Message{
		ZH: "登录 DevBox 沙箱管理器。\n\n" +
			"API Key 的解析顺序：\n" +
			"  1. --api-key 旗标（校验后存储）\n" +
			"  2. 已存储的 API Key（OS keyring 或配置文件，环境变量优先，校验后复用）\n" +
			"  3. 浏览器登录（对配置的登录页）",
		EN: "Log in to the DevBox sandbox manager.\n\n" +
			"The API key is resolved in this order:\n" +
			"  1. the --api-key flag (verified and stored)\n" +
			"  2. a key already stored in the OS keyring or config file " +
			"(environment variable preferred, verified)\n" +
			"  3. an interactive browser login against the configured login page",
	}
	msgs.LoginFailed = Message{
		ZH: "登录失败：",
		EN: "Login failed: ",
	}
	msgs.LoginSuccess = Message{
		ZH: "登录成功",
		EN: "Login successful",
	}
	msgs.LoginReused = Message{
		ZH: "登录成功（已复用本地保存的 API Key）",
		EN: "Login successful (reused the locally stored API key)",
	}
	msgs.LoginVerifyFailed = Message{
		ZH: "登录失败：本地 API Key 校验未通过：",
		EN: "Login failed: local API key verification failed: ",
	}
	msgs.LoginKeyInvalid = Message{
		ZH: "登录失败：获取到的 API Key 无效：",
		EN: "Login failed: the obtained API key is invalid: ",
	}
	msgs.LoginAPIKeyInvalid = Message{
		ZH: "登录失败：API Key 无效或已被禁用",
		EN: "Login failed: API key is invalid or disabled",
	}
	msgs.LogoutShort = Message{
		ZH: "退出登录",
		EN: "log out of CLI",
	}
	msgs.LogoutSuccess = Message{
		ZH: "已退出登录",
		EN: "Logged out",
	}
	msgs.StatusShort = Message{
		ZH: "查看当前登录状态",
		EN: "show the current login state",
	}
}

// initAuthStatusMessages fills the auth message group status.
func initAuthStatusMessages(msgs *AuthMessages) {
	msgs.NotLoggedIn = Message{
		ZH: "未登录",
		EN: "Not logged in",
	}
	msgs.CredInvalidHint = Message{
		ZH: "未登录（本地 API Key 已失效，请运行 devbox auth login）",
		EN: "Not logged in (the locally stored API key is invalid, run devbox auth login)",
	}
	msgs.StatusVerifyFailed = Message{
		ZH: "已登录，但校验失败：%v",
		EN: "Logged in, but verification failed: %v",
	}
	msgs.LoggedIn = Message{
		ZH: "已登录",
		EN: "Logged in",
	}
	msgs.UserName = Message{
		ZH: "用户名: %s",
		EN: "Username: %s",
	}
	msgs.APIKeyFlag = Message{
		ZH: "要校验并保存的 API Key（环境变量 HW_DEVBOX_API_KEY）",
		EN: "api key to verify and store (env HW_DEVBOX_API_KEY)",
	}
	msgs.LoginURLNotSet = Message{
		ZH: "未配置登录页地址：请通过构建期注入设置（-ldflags -X .../internal/config.LoginURL=...）",
		EN: "Login page URL is not configured: set it at build time (-ldflags -X .../internal/config.LoginURL=...)",
	}
	msgs.BrowserOpened = Message{
		ZH: "已在浏览器中打开登录页，请完成登录...",
		EN: "Login page opened in the browser, please complete the login...",
	}
	msgs.NoBrowserHint = Message{
		ZH: "无可用浏览器，请手动登录。API Key 管理页面: %s\n例如: ./devbox auth login --api-key=YOUR_API_KEY",
		EN: "No browser available, please login manually. " +
			"API key management page: %s\nFor example: ./devbox auth login --api-key=YOUR_API_KEY",
	}
	msgs.BindCallbackServer = Message{
		ZH: "绑定登录回调服务器失败：%w",
		EN: "bind login callback server: %w",
	}
	msgs.MissingAPIKey = Message{
		ZH: "登录响应未包含 API Key",
		EN: "login response did not contain an API key",
	}
	msgs.LoginTimeout = Message{
		ZH: "登录超时，未收到 API Key",
		EN: "login timed out; no API key was received",
	}
}

// initAuthCallbackMessages fills the auth message group callback.
func initAuthCallbackMessages(msgs *AuthMessages) {
	msgs.CallbackBizError = Message{
		ZH: "登录失败: %s: %s\n请前往管理页面删除 API Key 后重试。API Key 管理页面： %s",
		EN: "Login failed: %s: %s\n" +
			"Please go to the management page to delete API keys and try again. " +
			"API key management page: %s",
	}
	msgs.CallbackForbidden = Message{
		ZH: "来源校验失败",
		EN: "forbidden",
	}
	msgs.CallbackNoMethod = Message{
		ZH: "仅支持 POST 请求",
		EN: "method not allowed",
	}
	msgs.CallbackBadBody = Message{
		ZH: "请求体无效",
		EN: "bad request",
	}
	msgs.CallbackLoginFail = Message{
		ZH: "登录失败",
		EN: "login failed",
	}
	msgs.CallbackMissingKey = Message{
		ZH: "缺少 API Key",
		EN: "missing api key",
	}
}

// sandboxMessages returns the localized strings of the sandbox messages.
func sandboxMessages() SandboxMessages {
	var msgs SandboxMessages
	initSandboxCommandMessages(&msgs)
	initSandboxInfoKillMessages(&msgs)
	initSandboxLifecycleMessages(&msgs)
	initSandboxExecMessages(&msgs)
	initSandboxSessionFlagMessages(&msgs)
	initSandboxListFlagMessages(&msgs)
	return msgs
}

// initSandboxCommandMessages fills the sandbox message group command.
func initSandboxCommandMessages(msgs *SandboxMessages) {
	msgs.SandboxShort = Message{
		ZH: "管理沙箱",
		EN: "work with sandboxes",
	}
	msgs.CreateShort = Message{
		ZH: "创建沙箱并进入其终端",
		EN: "create a sandbox and connect to its terminal",
	}
	msgs.InvalidMetadata = Message{
		ZH: "--metadata 无效：%w",
		EN: "invalid --metadata: %w",
	}
	msgs.InvalidEnv = Message{
		ZH: "--env 无效：%w",
		EN: "invalid --env: %w",
	}
	msgs.AutoResumeRequires = Message{
		ZH: "--lifecycle.autoresume 需要 --lifecycle.ontimeout pause",
		EN: "--lifecycle.autoresume requires --lifecycle.ontimeout pause",
	}
	msgs.InvalidOnTimeout = Message{
		ZH: "--lifecycle.ontimeout %q 无效（支持: pause, kill）",
		EN: "invalid --lifecycle.ontimeout %q (supported: pause, kill)",
	}
	msgs.MissingDomain = Message{
		ZH: "沙箱响应未包含 domain",
		EN: "sandbox response does not include a domain",
	}
	msgs.MissingConnectToken = Message{
		ZH: "沙箱响应未包含 connect token",
		EN: "sandbox response does not include a connect token",
	}
	msgs.ConnectingTo = Message{
		ZH: "正在连接沙箱 %s（输入 exit 退出）",
		EN: "Connecting to sandbox %s (type 'exit' to leave)",
	}
	msgs.RemoteShellExited = Message{
		ZH: "远程 shell 已退出（exit_code=%d，status=%s）",
		EN: "Remote shell exited (exit_code=%d, status=%s)",
	}
	msgs.ClosingConnection = Message{
		ZH: "正在关闭沙箱 %s 的终端连接",
		EN: "Closing terminal connection to sandbox %s",
	}
	msgs.ListShort = Message{
		ZH: "列出沙箱",
		EN: "list sandboxes",
	}
}

// initSandboxInfoKillMessages fills the sandbox message group infoKill.
func initSandboxInfoKillMessages(msgs *SandboxMessages) {
	msgs.ListEmpty = Message{
		ZH: "未找到沙箱",
		EN: "No sandboxes found",
	}
	msgs.InvalidState = Message{
		ZH: "状态 %q 无效（支持: running, paused）",
		EN: "invalid state %q (supported: running, paused)",
	}
	msgs.InvalidSandboxID = Message{
		ZH: "沙箱 ID %q 无效（须为 36 位小写字母、数字和中划线）",
		EN: "invalid sandbox ID %q (must be 36 characters of lowercase letters, digits and hyphens)",
	}
	msgs.InvalidTemplateID = Message{
		ZH: "模板 ID %q 无效（须为不超过 64 位的小写字母、数字和中划线，且不能为空）",
		EN: "invalid template ID %q (must be at most 64 characters of lowercase letters, " +
			"digits and hyphens, and must not be empty)",
	}
	msgs.InfoShort = Message{
		ZH: "查看沙箱信息",
		EN: "show information for a sandbox",
	}
	msgs.MetricsShort = Message{
		ZH: "查看沙箱历史监控指标（5 秒采样）",
		EN: "show historical metrics for a sandbox (5s sampling)",
	}
	msgs.MetricsEmpty = Message{
		ZH: "该时间范围内没有监控数据",
		EN: "no metrics found for the requested range",
	}
	msgs.MetricsInvalidTime = Message{
		ZH: "无效的时间参数 %q（支持格式：Unix 秒或 2006-01-02T15:04:05）",
		EN: "invalid time %q (accepted formats: Unix seconds or 2006-01-02T15:04:05)",
	}
	msgs.KillShort = Message{
		ZH: "终止沙箱",
		EN: "kill a sandbox",
	}
	msgs.Killed = Message{
		ZH: "沙箱 %s 已被终止",
		EN: "sandbox %s has been killed",
	}
	msgs.KillAllConflict = Message{
		ZH: "sandbox ID 和 --all 不能同时使用",
		EN: "sandbox IDs and --all cannot be used together",
	}
	msgs.KillRequiresArgs = Message{
		ZH: "请指定 sandbox ID 或使用 --all",
		EN: "specify sandbox ID(s) or use --all",
	}
	msgs.KillPartialFailed = Message{
		ZH: "%d/%d 个沙箱终止失败",
		EN: "%d of %d sandboxes failed to be killed",
	}
	msgs.FlagKillAll = Message{
		ZH: "终止所有沙箱（默认 running 状态）",
		EN: "kill all sandboxes (running state by default)",
	}
	msgs.FlagKillState = Message{
		ZH: "与 --all 一起使用时按状态过滤（默认 running）",
		EN: "when used with --all, filter by state (default running)",
	}
}

// initSandboxLifecycleMessages fills the sandbox message group lifecycle.
func initSandboxLifecycleMessages(msgs *SandboxMessages) {
	msgs.FlagKillMetadata = Message{
		ZH: "与 --all 一起使用时按元数据过滤",
		EN: "when used with --all, filter by metadata",
	}
	msgs.PauseShort = Message{
		ZH: "暂停沙箱",
		EN: "pause a sandbox",
	}
	msgs.Paused = Message{
		ZH: "沙箱 %s 已被暂停",
		EN: "sandbox %s has been paused",
	}
	msgs.ResumeShort = Message{
		ZH: "恢复已暂停的沙箱",
		EN: "resume a paused sandbox",
	}
	msgs.Resumed = Message{
		ZH: "沙箱 %s 已被恢复",
		EN: "sandbox %s has been resumed",
	}
	msgs.ConnectShort = Message{
		ZH: "进入沙箱交互式终端（暂停的沙箱会自动恢复）",
		EN: "open an interactive terminal to a sandbox, resuming it if paused",
	}
	msgs.TimeoutShort = Message{
		ZH: "设置沙箱存活时间",
		EN: "set the time-to-live of a sandbox",
	}
	msgs.TimeoutSet = Message{
		ZH: "沙箱 %s 的存活时间已设置为 %d 秒",
		EN: "sandbox %s timeout set to %ds",
	}
	msgs.RefreshShort = Message{
		ZH: "延长沙箱存活时间",
		EN: "extend the lifetime of a sandbox",
	}
	msgs.Refreshed = Message{
		ZH: "沙箱 %s 的存活时间已延长 %d 秒",
		EN: "sandbox %s lifetime extended by %ds",
	}
	msgs.ExecShort = Message{
		ZH: "在运行中的沙箱里执行命令",
		EN: "execute a command in a running sandbox",
	}
	msgs.ExecLong = Message{
		ZH: "在运行中的沙箱里执行命令。\n\n" +
			"sandbox ID 之后的全部内容透传给远程命令，devbox 选项必须放在 sandbox ID 之前：\n\n" +
			`  devbox sbx exec -c /tmp <sandboxID> codex exec "prompt" --help`,
		EN: "Execute a command in a running sandbox.\n\n" +
			"Everything after the sandbox ID is passed through to the remote command,\n" +
			"so devbox options must come before the sandbox ID:\n\n" +
			`  devbox sbx exec -c /tmp <sandboxID> codex exec "prompt" --help`,
	}
}

// initSandboxExecMessages fills the sandbox message group exec.
func initSandboxExecMessages(msgs *SandboxMessages) {
	msgs.NoProcessID = Message{
		ZH: "沙箱未返回进程 ID",
		EN: "sandbox did not return a process id",
	}
	msgs.StdinAfterExit = Message{
		ZH: "devbox: 远端命令已退出，stdin 未送达。",
		EN: "devbox: Remote command exited before stdin could be delivered.",
	}
	msgs.StdinForwardFailed = Message{
		ZH: "devbox: 向远端转发标准输入失败：%v",
		EN: "devbox: forwarding stdin to the sandbox failed: %v",
	}
	msgs.CloseStdinFailed = Message{
		ZH: "devbox: 关闭远端标准输入失败：%v",
		EN: "devbox: closing remote stdin failed: %v",
	}
	msgs.MissingCommand = Message{
		ZH: "缺少要执行的命令",
		EN: "missing command to execute",
	}
	msgs.FlagTimeout = Message{
		ZH: "沙箱存活时间，单位秒（10-86400）",
		EN: "time to live for the sandbox in seconds (10-86400)",
	}
	msgs.FlagNewTimeout = Message{
		ZH: "新的存活时间，单位秒（10-86400）",
		EN: "new time to live in seconds (10-86400)",
	}
	msgs.FlagTimeoutKeep = Message{
		ZH: "存活时间，单位秒（10-86400）",
		EN: "time to live in seconds (10-86400)",
	}
	msgs.InvalidTimeout = Message{
		ZH: "timeout/duration 必须在 10-86400 秒之间，当前 %d",
		EN: "timeout/duration must be between 10 and 86400 seconds, got %d",
	}
	msgs.FlagMetadata = Message{
		ZH: "元数据 key=value（可重复）",
		EN: "metadata key=value (repeatable)",
	}
	msgs.FlagMetadataFilter = Message{
		ZH: "元数据过滤，如 user=abc&app=prod",
		EN: `metadata filter, eg. user=abc&app=prod`,
	}
	msgs.FlagTemplateFilter = Message{
		ZH: "按模板 ID 或别名过滤，如 base",
		EN: "filter by template ID or alias, eg. base",
	}
}

// initSandboxSessionFlagMessages fills the sandbox message group sessionFlag.
func initSandboxSessionFlagMessages(msgs *SandboxMessages) {
	msgs.FlagEnv = Message{
		ZH: "环境变量 key=value（可重复）",
		EN: "environment variable key=value (repeatable)",
	}
	msgs.FlagEnvTerm = Message{
		ZH: "终端会话的环境变量 key=value（可重复）",
		EN: "environment variable key=value for the terminal session (repeatable)",
	}
	msgs.FlagEnvSession = Message{
		ZH: "为终端会话设置环境变量 key=value（可重复）",
		EN: "set environment variable for the terminal session (repeatable)",
	}
	msgs.FlagBuildID = Message{
		ZH: "模板构建标识",
		EN: "template build identifier",
	}
	msgs.FlagOnTimeout = Message{
		ZH: "沙箱超时后的动作: pause 或 kill",
		EN: "action when sandbox timeout is reached: pause or kill",
	}
	msgs.FlagAutoResume = Message{
		ZH: "启用沙箱自动恢复，需要 --lifecycle.ontimeout pause",
		EN: "enable sandbox auto-resume, requires --lifecycle.ontimeout pause",
	}
	msgs.FlagNetworkPublic = Message{
		ZH: "是否允许无需鉴权地直接访问该沙箱实例开放的公网 URL",
		EN: "allow direct, unauthenticated access to the public URLs exposed by this sandbox instance",
	}
	msgs.FlagDetach = Message{
		ZH: "只创建沙箱，不进入终端",
		EN: "create the sandbox without connecting to its terminal",
	}
	msgs.FlagCwd = Message{
		ZH: "终端会话在沙箱内的工作目录",
		EN: "working directory for the terminal session inside the sandbox",
	}
	msgs.FlagUser = Message{
		ZH: "沙箱内以指定用户运行命令或打开终端",
		EN: "run the command or open the terminal as the specified user inside the sandbox",
	}
	msgs.SessionEndedSandboxGone = Message{
		ZH: "沙箱 %s 已销毁（TTL 到期或被终止），连接已断开",
		EN: "sandbox %s was destroyed (TTL reached or the sandbox was killed); the connection has been closed",
	}
	msgs.SessionEndedSandboxPaused = Message{
		ZH: "沙箱 %s 已暂停（超时自动暂停），连接已断开；可 resume 恢复后重连",
		EN: "sandbox %s was paused (auto-paused on timeout); the connection has been closed - resume it and reconnect",
	}
}

// initSandboxListFlagMessages fills the sandbox message group listFlag.
func initSandboxListFlagMessages(msgs *SandboxMessages) {
	msgs.TunnelExpired = Message{
		ZH: "沙箱 %s 的原隧道已过期，重复连接不会恢复原地址；请联系支持，或销毁后重建沙箱",
		EN: "the tunnel for sandbox %s has expired - reconnecting will not restore " +
			"the original address; contact support or recreate the sandbox",
	}
	msgs.TunnelValidityInsufficient = Message{
		ZH: "隧道剩余有效期不足以覆盖请求的运行时长（含 5 分钟余量）；可缩短 --timeout 后重试",
		EN: "the tunnel's remaining validity cannot cover the requested runtime " +
			"plus a 5-minute margin; try a shorter --timeout and retry",
	}
	msgs.FlagState = Message{
		ZH: "按状态过滤: running 或 paused（可重复）",
		EN: "filter by state: running or paused (repeatable)",
	}
	msgs.FlagStartedAfter = Message{
		ZH: "仅列出此时间之后启动的沙箱，如 2025-01-01T00:00:00Z",
		EN: "filter by start time, only sandboxes started at or after this time are returned, eg. 2025-01-01T00:00:00Z",
	}
	msgs.FlagOrder = Message{
		ZH: "按启动时间排序: asc 或 desc（默认 desc）",
		EN: "sort order by start time, asc or desc (default: desc)",
	}
	msgs.InvalidOrder = Message{
		ZH: "排序方式 %q 无效（支持: asc, desc）",
		EN: "invalid order %q (supported: asc, desc)",
	}
	msgs.InvalidStartedAfter = Message{
		ZH: "日期 %q 无效（预期 ISO 8601 时间戳，如 2025-01-01T00:00:00Z）",
		EN: "invalid date %q (expected an ISO 8601 timestamp, eg. 2025-01-01T00:00:00Z)",
	}
	msgs.InvalidLimit = Message{
		ZH: "limit 必须在 0-1000 之间，当前 %d",
		EN: "limit must be between 0 and 1000, got %d",
	}
	msgs.FlagLimit = Message{
		ZH: "返回的沙箱数量上限（0 表示不限制）",
		EN: "limit the number of sandboxes returned (0 for no limit)",
	}
	msgs.FlagFormat = Message{
		ZH: "输出格式: pretty 或 json",
		EN: "output format: pretty or json",
	}
	msgs.FlagMetricsStart = Message{
		ZH: "起始时间（Unix 秒或 RFC3339，默认最早样本）",
		EN: "range start (Unix seconds or RFC3339, defaults to the earliest sample)",
	}
	msgs.FlagMetricsEnd = Message{
		ZH: "结束时间（Unix 秒或 RFC3339，默认当前时间）",
		EN: "range end (Unix seconds or RFC3339, defaults to now)",
	}
	msgs.FlagDuration = Message{
		ZH: "存活时间延长秒数（10-86400）",
		EN: "lifetime extension in seconds (10-86400)",
	}
	msgs.FlagBackground = Message{
		ZH: "后台运行并立即返回",
		EN: "run in background and return immediately",
	}
}

// templateMessages returns the localized strings of the template messages.
func templateMessages() TemplateMessages {
	return TemplateMessages{
		TemplateShort: Message{
			ZH: "管理沙箱模板",
			EN: "manage sandbox templates",
		},
		ListShort: Message{
			ZH: "列出模板",
			EN: "list templates",
		},
		ListEmpty: Message{
			ZH: "未找到模板",
			EN: "No templates found",
		},
	}
}

// clientMessages returns the localized strings of the client messages.
func clientMessages() ClientMessages {
	var msgs ClientMessages
	initClientHTTPMessages(&msgs)
	initClientVerifyMessages(&msgs)
	return msgs
}

// initClientHTTPMessages fills the client message group hTTP.
func initClientHTTPMessages(msgs *ClientMessages) {
	msgs.Unauthorized = Message{
		ZH: "API Key 无效或已被禁用",
		EN: "api key is invalid or disabled",
	}
	msgs.UpgradeRequired = Message{
		ZH: "当前 CLI 版本过低，请升级后重试",
		EN: "your CLI version is too old, please upgrade and retry",
	}
	msgs.UpdateLabel = Message{
		ZH: "更新方式：",
		EN: "Update:",
	}
	msgs.InvalidServerURL = Message{
		ZH: "无效的服务器地址 %q：%w",
		EN: "invalid server URL %q: %w",
	}
	msgs.InvalidServerURLScheme = Message{
		ZH: "无效的服务器地址 %q：需要 http(s)://host 形式",
		EN: "invalid server URL %q: expected http(s)://host[:port]",
	}
	msgs.InvalidServerURLHost = Message{
		ZH: "无效的服务器地址 %q：缺少主机名",
		EN: "invalid server URL %q: missing host",
	}
	msgs.CreateClientFailed = Message{
		ZH: "创建 API 客户端失败：%w",
		EN: "create API client: %w",
	}
	msgs.APIErrorFormat = Message{
		ZH: "DevBox API %d %s：%s",
		EN: "DevBox API %d %s: %s",
	}
	msgs.ErrorResponsePrefix = Message{
		ZH: "错误响应：",
		EN: "error response: ",
	}
	msgs.BuildVerifyRequest = Message{
		ZH: "构造校验请求失败：%w",
		EN: "build verify request: %w",
	}
}

// initClientVerifyMessages fills the client message group verify.
func initClientVerifyMessages(msgs *ClientMessages) {
	msgs.VerifyFailed = Message{
		ZH: "API Key 校验失败：%w",
		EN: "verify api key: %w",
	}
	msgs.VerifyUnexpectedStatus = Message{
		ZH: "API Key 校验失败：意外的状态码 %d %s",
		EN: "verify api key: unexpected status %d %s",
	}
	msgs.InvalidSandboxDomain = Message{
		ZH: "无效的沙箱地址 %q：%w",
		EN: "invalid sandbox domain %q: %w",
	}
	msgs.InvalidDomainScheme = Message{
		ZH: "无效的沙箱地址 %q：需要 http(s)://host 形式",
		EN: "invalid sandbox domain %q: expected http(s)://host[:port]",
	}
	msgs.NothingToStore = Message{
		ZH: "没有可存储的内容：API Key 为空",
		EN: "nothing to store: API key is empty",
	}
	msgs.StoreCredentialFailed = Message{
		ZH: "存储凭据失败：keyring 和配置文件均失败：%w",
		EN: "store credential: keyring and config file both failed: %w",
	}
	msgs.LoadCredentialFailed = Message{
		ZH: "读取凭据失败：%w",
		EN: "load credential: %w",
	}
	msgs.NoCredential = Message{
		ZH: "未找到凭据",
		EN: "no credential found",
	}
	msgs.EmptyResponseBody = Message{
		ZH: "服务端返回了意外的空响应体",
		EN: "unexpected empty response body from server",
	}
}

// parseMessages returns the localized strings of the parse messages.
func parseMessages() ParseMessages {
	return ParseMessages{
		ExpectedKeyValue: Message{
			ZH: "预期 key=value 格式，实际 %q",
			EN: "expected key=value, got %q",
		},
	}
}

// configMessages returns the localized strings of the config messages.
func configMessages() ConfigMessages {
	return ConfigMessages{
		ReadConfigFileFailed: Message{
			ZH: "读取配置文件失败：%w",
			EN: "read config file: %w",
		},
		ResolveHomeFailed: Message{
			ZH: "解析用户主目录失败：%w",
			EN: "resolve home directory: %w",
		},
		ParseConfigFileFailed: Message{
			ZH: "解析配置文件失败：%w",
			EN: "parse config file: %w",
		},
		CreateConfigDirFailed: Message{
			ZH: "创建配置目录失败：%w",
			EN: "create config dir: %w",
		},
		EncodeConfigFailed: Message{
			ZH: "编码配置失败：%w",
			EN: "encode config: %w",
		},
		WriteConfigFileFailed: Message{
			ZH: "写入配置文件失败：%w",
			EN: "write config file: %w",
		},
	}
}

// outputMessages returns the localized strings of the output messages.
func outputMessages() OutputMessages {
	return OutputMessages{
		UnknownFormat: Message{
			ZH: "未知的输出格式 %q（支持: pretty, json）",
			EN: "unknown output format %q (supported: pretty, json)",
		},
		EncodeFailed: Message{
			ZH: "编码 JSON 失败：%w",
			EN: "encode JSON: %w",
		},
		RowLengthMismatch: Message{
			ZH: "表格行有 %d 列，与表头 %d 列不一致",
			EN: "table row has %d columns, expected %d",
		},
	}
}
