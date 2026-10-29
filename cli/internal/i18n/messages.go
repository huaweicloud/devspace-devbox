package i18n

// Messages 是 devbox CLI 全部用户可见文案的消息树。
type Messages struct {
	Common   CommonMessages
	Auth     AuthMessages
	Sandbox  SandboxMessages
	Template TemplateMessages
}

type CommonMessages struct {
	RootShort   Message
	RootLong    Message
	FlagVerbose Message
}

type AuthMessages struct {
	AuthShort          Message
	LoginShort         Message
	LoginLong          Message
	LoginFailed        Message
	LoginSuccess       Message
	LoginReused        Message
	LoginVerifyFailed  Message
	LoginKeyInvalid    Message
	LoginAPIKeyInvalid Message
	LogoutShort        Message
	LogoutSuccess      Message
	StatusShort        Message
	NotLoggedIn        Message
	CredInvalidHint    Message
	StatusVerifyFailed Message
	LoggedIn           Message
	UserName           Message
	APIKeyFlag         Message
	LoginURLNotSet     Message
	BrowserOpened      Message
	NoBrowserHint      Message
}

type SandboxMessages struct {
	SandboxShort        Message
	CreateShort         Message
	InvalidMetadata     Message
	InvalidEnv          Message
	AutoResumeRequires  Message
	InvalidOnTimeout    Message
	MissingDomain       Message
	MissingConnectToken Message
	ConnectingTo        Message
	RemoteShellExited   Message
	ClosingConnection   Message
	ListShort           Message
	ListEmpty           Message
	InvalidState        Message
	InvalidSandboxID    Message
	InfoShort           Message
	KillShort           Message
	Killed              Message
	PauseShort          Message
	Paused              Message
	ResumeShort         Message
	Resumed             Message
	ConnectShort        Message
	TimeoutShort        Message
	TimeoutSet          Message
	RefreshShort        Message
	Refreshed           Message
	ExecShort           Message
	ExecLong            Message
	NoProcessID         Message
	StdinAfterExit      Message
	MissingCommand      Message
	FlagTimeout         Message
	FlagNewTimeout      Message
	FlagTimeoutKeep     Message
	InvalidTimeout      Message
	FlagMetadata        Message
	FlagMetadataFilter  Message
	FlagEnv             Message
	FlagEnvTerm         Message
	FlagBuildID         Message
	FlagOnTimeout       Message
	FlagAutoResume      Message
	FlagDetach          Message
	FlagCwd             Message
	FlagState           Message
	FlagLimit           Message
	FlagFormat          Message
	FlagDuration        Message
	FlagBackground      Message
}

type TemplateMessages struct {
	TemplateShort Message
	ListShort     Message
	ListEmpty     Message
}

var Msg Messages
