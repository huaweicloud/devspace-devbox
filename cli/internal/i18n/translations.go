package i18n

func init() {
	Msg = Messages{
		Common: CommonMessages{
			RootShort: Message{
				ZH: "DevBox 沙箱管理器命令行客户端",
				EN: "CLI for the DevBox sandbox manager",
			},
			RootLong: Message{
				ZH: "devbox 是 DevBox 沙箱管理器的命令行客户端。\n\n覆盖完整的沙箱生命周期（创建、列表、信息、终止、暂停、恢复、连接终端、执行命令）与模板。\n\n配置的解析顺序：\n  1. flags                --api-key (auth login)\n  2. environment          HW_DEVBOX_API_KEY\n  3. config file          ~/.devbox/config.json\n  4. build-time default   API server URL injected via Makefile, currently ",
				EN: "devbox is a command-line client for the DevBox Sandbox Manager API.\n\nIt covers the full sandbox lifecycle (create, list, info, kill, pause,\nresume, connect, exec) and templates.\n\nConfiguration is resolved in this order:\n  1. flags                --api-key (auth login)\n  2. environment          HW_DEVBOX_API_KEY\n  3. config file          ~/.devbox/config.json\n  4. build-time default   API server URL injected via Makefile, currently ",
			},
			FlagVerbose: Message{
				ZH: "打印调试日志（含接口请求/返回）",
				EN: "print debug logs (API requests/responses)",
			},
			VersionShort: Message{
				ZH: "显示版本号",
				EN: "show the version",
			},
		},
		Auth: AuthMessages{
			AuthShort: Message{
				ZH: "登录 DevBox 沙箱管理器",
				EN: "Authenticate to the DevBox sandbox manager",
			},
			LoginShort: Message{
				ZH: "登录 CLI",
				EN: "log in to CLI",
			},
			LoginLong: Message{
				ZH: "登录 DevBox 沙箱管理器。\n\nAPI Key 的解析顺序：\n  1. --api-key 旗标（校验后存储）\n  2. 已存储的 API Key（OS keyring 或配置文件，环境变量优先，校验后复用）\n  3. 浏览器登录（对配置的登录页）",
				EN: "Log in to the DevBox sandbox manager.\n\nThe API key is resolved in this order:\n  1. the --api-key flag (verified and stored)\n  2. a key already stored in the OS keyring or config file (environment variable preferred, verified)\n  3. an interactive browser login against the configured login page",
			},
			LoginFailed: Message{
				ZH: "登录失败：",
				EN: "Login failed: ",
			},
			LoginSuccess: Message{
				ZH: "登录成功",
				EN: "Login successful",
			},
			LoginReused: Message{
				ZH: "登录成功（已复用本地保存的 API Key）",
				EN: "Login successful (reused the locally stored API key)",
			},
			LoginVerifyFailed: Message{
				ZH: "登录失败：本地 API Key 校验未通过：",
				EN: "Login failed: local API key verification failed: ",
			},
			LoginKeyInvalid: Message{
				ZH: "登录失败：获取到的 API Key 无效：",
				EN: "Login failed: the obtained API key is invalid: ",
			},
			LoginAPIKeyInvalid: Message{
				ZH: "登录失败：API Key 无效或已被禁用",
				EN: "Login failed: API key is invalid or disabled",
			},
			LogoutShort: Message{
				ZH: "退出登录",
				EN: "log out of CLI",
			},
			LogoutSuccess: Message{
				ZH: "已退出登录",
				EN: "Logged out",
			},
			StatusShort: Message{
				ZH: "查看当前登录状态",
				EN: "Show the current login state",
			},
			NotLoggedIn: Message{
				ZH: "未登录",
				EN: "Not logged in",
			},
			CredInvalidHint: Message{
				ZH: "未登录（本地 API Key 已失效，请运行 devbox auth login）",
				EN: "Not logged in (the locally stored API key is invalid, run devbox auth login)",
			},
			StatusVerifyFailed: Message{
				ZH: "已登录，但校验失败：%v",
				EN: "Logged in, but verification failed: %v",
			},
			LoggedIn: Message{
				ZH: "已登录",
				EN: "Logged in",
			},
			UserName: Message{
				ZH: "用户名: %s",
				EN: "Username: %s",
			},
			APIKeyFlag: Message{
				ZH: "要校验并保存的 API Key（环境变量 HW_DEVBOX_API_KEY）",
				EN: "API key to verify and store (env HW_DEVBOX_API_KEY)",
			},
			LoginURLNotSet: Message{
				ZH: "未配置登录页地址：请通过构建期注入设置（-ldflags -X .../internal/config.LoginURL=...）",
				EN: "Login page URL is not configured: set it at build time (-ldflags -X .../internal/config.LoginURL=...)",
			},
			BrowserOpened: Message{
				ZH: "已在浏览器中打开登录页，请完成登录...",
				EN: "Login page opened in the browser, please complete the login...",
			},
			NoBrowserHint: Message{
				ZH: "无可用浏览器，请手动登录。API Key 管理页面: %s\n例如: ./devbox auth login --api-key=YOUR_API_KEY",
				EN: "No browser available, please login manually. API key management page: %s\nFor example: ./devbox auth login --api-key=YOUR_API_KEY",
			},
			BindCallbackServer: Message{
				ZH: "绑定登录回调服务器失败：%w",
				EN: "bind login callback server: %w",
			},
			MissingAPIKey: Message{
				ZH: "登录响应未包含 API Key",
				EN: "login response did not contain an API key",
			},
			LoginTimeout: Message{
				ZH: "登录超时，未收到 API Key",
				EN: "login timed out; no API key was received",
			},
			CallbackBizError: Message{
				ZH: "登录失败: %s: %s\n请前往管理页面删除 API Key 后重试。API Key 管理页面： %s",
				EN: "Login failed: %s: %s\nPlease go to the management page to delete API keys and try again. API key management page: %s",
			},
		},
		Sandbox: SandboxMessages{
			SandboxShort: Message{
				ZH: "管理沙箱",
				EN: "work with sandboxes",
			},
			CreateShort: Message{
				ZH: "创建沙箱并进入其终端",
				EN: "Create a sandbox and connect to its terminal",
			},
			InvalidMetadata: Message{
				ZH: "--metadata 无效：%w",
				EN: "invalid --metadata: %w",
			},
			InvalidEnv: Message{
				ZH: "--env 无效：%w",
				EN: "invalid --env: %w",
			},
			AutoResumeRequires: Message{
				ZH: "--lifecycle.autoresume 需要 --lifecycle.ontimeout pause",
				EN: "--lifecycle.autoresume requires --lifecycle.ontimeout pause",
			},
			InvalidOnTimeout: Message{
				ZH: "--lifecycle.ontimeout %q 无效（支持: pause, kill）",
				EN: "invalid --lifecycle.ontimeout %q (supported: pause, kill)",
			},
			MissingDomain: Message{
				ZH: "沙箱响应未包含 domain",
				EN: "sandbox response does not include a domain",
			},
			MissingConnectToken: Message{
				ZH: "沙箱响应未包含 connect token",
				EN: "sandbox response does not include a connect token",
			},
			ConnectingTo: Message{
				ZH: "正在连接沙箱 %s（输入 exit 退出）",
				EN: "Connecting to sandbox %s (type 'exit' to leave)",
			},
			RemoteShellExited: Message{
				ZH: "远程 shell 已退出（exit_code=%d，status=%s）",
				EN: "Remote shell exited (exit_code=%d, status=%s)",
			},
			ClosingConnection: Message{
				ZH: "正在关闭沙箱 %s 的终端连接",
				EN: "Closing terminal connection to sandbox %s",
			},
			ListShort: Message{
				ZH: "列出沙箱",
				EN: "List sandboxes",
			},
			ListEmpty: Message{
				ZH: "未找到沙箱",
				EN: "No sandboxes found",
			},
			InvalidState: Message{
				ZH: "状态 %q 无效（支持: running, paused）",
				EN: "invalid state %q (supported: running, paused)",
			},
			InvalidSandboxID: Message{
				ZH: "沙箱 ID %q 无效（须为 36 位小写字母、数字和中划线）",
				EN: "invalid sandbox ID %q (must be 36 characters of lowercase letters, digits and hyphens)",
			},
			InvalidTemplateID: Message{
				ZH: "模板 ID %q 无效（须为不超过 64 位的小写字母、数字和中划线，且不能为空）",
				EN: "invalid template ID %q (must be at most 64 characters of lowercase letters, digits and hyphens, and must not be empty)",
			},
			InfoShort: Message{
				ZH: "查看沙箱信息",
				EN: "show information for a sandbox",
			},
			KillShort: Message{
				ZH: "终止沙箱",
				EN: "Kill a sandbox",
			},
			Killed: Message{
				ZH: "沙箱 %s 已被终止",
				EN: "sandbox %s has been killed",
			},
			KillAllConflict: Message{
				ZH: "sandbox ID 和 --all 不能同时使用",
				EN: "sandbox IDs and --all cannot be used together",
			},
			KillRequiresArgs: Message{
				ZH: "请指定 sandbox ID 或使用 --all",
				EN: "specify sandbox ID(s) or use --all",
			},
			FlagKillAll: Message{
				ZH: "终止所有沙箱（默认 running 状态）",
				EN: "kill all sandboxes (running state by default)",
			},
			FlagKillState: Message{
				ZH: "与 --all 一起使用时按状态过滤（默认 running）",
				EN: "when used with --all, filter by state (default running)",
			},
			FlagKillMetadata: Message{
				ZH: "与 --all 一起使用时按元数据过滤",
				EN: "when used with --all, filter by metadata",
			},
			PauseShort: Message{
				ZH: "暂停沙箱",
				EN: "Pause a sandbox",
			},
			Paused: Message{
				ZH: "沙箱 %s 已被暂停",
				EN: "sandbox %s has been paused",
			},
			ResumeShort: Message{
				ZH: "恢复已暂停的沙箱",
				EN: "Resume a paused sandbox",
			},
			Resumed: Message{
				ZH: "沙箱 %s 已被恢复",
				EN: "sandbox %s has been resumed",
			},
			ConnectShort: Message{
				ZH: "进入沙箱交互式终端（暂停的沙箱会自动恢复）",
				EN: "Open an interactive terminal to a sandbox, resuming it if paused",
			},
			TimeoutShort: Message{
				ZH: "设置沙箱存活时间",
				EN: "Set the time-to-live of a sandbox",
			},
			TimeoutSet: Message{
				ZH: "沙箱 %s 的存活时间已设置为 %d 秒",
				EN: "sandbox %s timeout set to %ds",
			},
			RefreshShort: Message{
				ZH: "延长沙箱存活时间",
				EN: "Extend the lifetime of a sandbox",
			},
			Refreshed: Message{
				ZH: "沙箱 %s 的存活时间已延长 %d 秒",
				EN: "sandbox %s lifetime extended by %ds",
			},
			ExecShort: Message{
				ZH: "在运行中的沙箱里执行命令",
				EN: "Execute a command in a running sandbox",
			},
			ExecLong: Message{
				ZH: "在运行中的沙箱里执行命令。\n\nsandbox ID 之后的全部内容透传给远程命令，devbox 选项必须放在 sandbox ID 之前：\n\n  devbox sbx exec -c /tmp <sandboxID> codex exec \"prompt\" --help",
				EN: "Execute a command in a running sandbox.\n\nEverything after the sandbox ID is passed through to the remote command,\nso devbox options must come before the sandbox ID:\n\n  devbox sbx exec -c /tmp <sandboxID> codex exec \"prompt\" --help",
			},
			NoProcessID: Message{
				ZH: "沙箱未返回进程 ID",
				EN: "sandbox did not return a process id",
			},
			StdinAfterExit: Message{
				ZH: "devbox: 远程命令已退出，stdin 未能送达。",
				EN: "devbox: Remote command exited before stdin could be delivered.",
			},
			MissingCommand: Message{
				ZH: "缺少要执行的命令",
				EN: "missing command to execute",
			},
			FlagTimeout: Message{
				ZH: "沙箱存活时间，单位秒（10-86400）",
				EN: "Time to live for the sandbox in seconds (10-86400)",
			},
			FlagNewTimeout: Message{
				ZH: "新的存活时间，单位秒（10-86400）",
				EN: "New time to live in seconds (10-86400)",
			},
			FlagTimeoutKeep: Message{
				ZH: "存活时间，单位秒（10-86400，省略则保持当前 TTL）",
				EN: "Time to live in seconds (10-86400, omit to keep the current TTL)",
			},
			InvalidTimeout: Message{
				ZH: "timeout/duration 必须在 10-86400 秒之间，当前 %d",
				EN: "timeout/duration must be between 10 and 86400 seconds, got %d",
			},
			FlagMetadata: Message{
				ZH: "元数据 key=value（可重复）",
				EN: "Metadata key=value (repeatable)",
			},
			FlagMetadataFilter: Message{
				ZH: "元数据过滤，如 \"user=abc&app=prod\"",
				EN: `Metadata filter, e.g. "user=abc&app=prod"`,
			},
			FlagTemplateFilter: Message{
				ZH: "按模板 ID 或别名过滤，如 base",
				EN: "filter by template ID or alias, e.g. base",
			},
			FlagEnv: Message{
				ZH: "环境变量 key=value（可重复）",
				EN: "Environment variable key=value (repeatable)",
			},
			FlagEnvTerm: Message{
				ZH: "终端会话的环境变量 key=value（可重复）",
				EN: "Environment variable key=value for the terminal session (repeatable)",
			},
			FlagBuildID: Message{
				ZH: "模板构建标识",
				EN: "Template build identifier",
			},
			FlagOnTimeout: Message{
				ZH: "沙箱超时后的动作: pause 或 kill",
				EN: "Action when sandbox timeout is reached: pause or kill",
			},
			FlagAutoResume: Message{
				ZH: "启用沙箱自动恢复，需要 --lifecycle.ontimeout pause",
				EN: "Enable sandbox auto-resume, requires --lifecycle.ontimeout pause",
			},
			FlagDetach: Message{
				ZH: "只创建沙箱，不进入终端",
				EN: "Create the sandbox without connecting to its terminal",
			},
			FlagCwd: Message{
				ZH: "终端会话在沙箱内的工作目录",
				EN: "Working directory for the terminal session inside the sandbox",
			},
			FlagState: Message{
				ZH: "按状态过滤: running 或 paused（可重复）",
				EN: "Filter by state: running or paused (repeatable)",
			},
			FlagLimit: Message{
				ZH: "返回的沙箱数量上限（0 表示不限制）",
				EN: "Limit the number of sandboxes returned (0 for no limit)",
			},
			FlagFormat: Message{
				ZH: "输出格式: pretty 或 json",
				EN: "Output format: pretty or json",
			},
			FlagDuration: Message{
				ZH: "存活时间延长秒数（10-86400）",
				EN: "Lifetime extension in seconds (10-86400)",
			},
			FlagBackground: Message{
				ZH: "后台运行并立即返回",
				EN: "Run in background and return immediately",
			},
		},
		Template: TemplateMessages{
			TemplateShort: Message{
				ZH: "管理沙箱模板",
				EN: "Manage sandbox templates",
			},
			ListShort: Message{
				ZH: "列出模板",
				EN: "List templates",
			},
			ListEmpty: Message{
				ZH: "未找到模板",
				EN: "No templates found",
			},
		},
		Client: ClientMessages{
			Unauthorized: Message{
				ZH: "API Key 无效或已被禁用",
				EN: "api key is invalid or disabled",
			},
			InvalidServerURL: Message{
				ZH: "无效的服务器地址 %q：%w",
				EN: "invalid server URL %q: %w",
			},
			InvalidServerURLScheme: Message{
				ZH: "无效的服务器地址 %q：需要 http(s)://host 形式",
				EN: "invalid server URL %q: expected http(s)://host[:port]",
			},
			InvalidServerURLHost: Message{
				ZH: "无效的服务器地址 %q：缺少主机名",
				EN: "invalid server URL %q: missing host",
			},
			CreateClientFailed: Message{
				ZH: "创建 API 客户端失败：%w",
				EN: "create API client: %w",
			},
			APIErrorFormat: Message{
				ZH: "DevBox API %d %s：%s",
				EN: "DevBox API %d %s: %s",
			},
			ErrorResponsePrefix: Message{
				ZH: "错误响应：",
				EN: "error response: ",
			},
			BuildVerifyRequest: Message{
				ZH: "构造校验请求失败：%w",
				EN: "build verify request: %w",
			},
			VerifyFailed: Message{
				ZH: "API Key 校验失败：%w",
				EN: "verify api key: %w",
			},
			VerifyUnexpectedStatus: Message{
				ZH: "API Key 校验失败：意外的状态码 %d %s",
				EN: "verify api key: unexpected status %d %s",
			},
			InvalidSandboxDomain: Message{
				ZH: "无效的沙箱地址 %q：%w",
				EN: "invalid sandbox domain %q: %w",
			},
			InvalidDomainScheme: Message{
				ZH: "无效的沙箱地址 %q：需要 http(s)://host 形式",
				EN: "invalid sandbox domain %q: expected http(s)://host[:port]",
			},
			NothingToStore: Message{
				ZH: "没有可存储的内容：API Key 为空",
				EN: "nothing to store: API key is empty",
			},
			StoreCredentialFailed: Message{
				ZH: "存储凭据失败：keyring 和配置文件均失败：%w",
				EN: "store credential: keyring and config file both failed: %w",
			},
			LoadCredentialFailed: Message{
				ZH: "读取凭据失败：%w",
				EN: "load credential: %w",
			},
			NoCredential: Message{
				ZH: "未找到凭据",
				EN: "no credential found",
			},
		},
		Parse: ParseMessages{
			ExpectedKeyValue: Message{
				ZH: "预期 key=value 格式，实际 %q",
				EN: "expected key=value, got %q",
			},
		},
		Config: ConfigMessages{
			BindEnvFailed: Message{
				ZH: "绑定 %s 环境变量失败：%w",
				EN: "bind %s: %w",
			},
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
		},
		Output: OutputMessages{
			UnknownFormat: Message{
				ZH: "未知的输出格式 %q（支持: pretty, json）",
				EN: "unknown output format %q (supported: pretty, json)",
			},
			EncodeFailed: Message{
				ZH: "编码 JSON 失败：%w",
				EN: "encode JSON: %w",
			},
		},
	}
}
