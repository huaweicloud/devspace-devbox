package i18n

// Messages 是 devbox CLI 全部用户可见文案的消息树。
type Messages struct {
	Common   CommonMessages
	Auth     AuthMessages
	Sandbox  SandboxMessages
	Template TemplateMessages
	Client   ClientMessages
	EnvD     EnvDMessages
	Parse    ParseMessages
	Config   ConfigMessages
	Output   OutputMessages
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
	BindCallbackServer Message
	MissingAPIKey      Message
	LoginTimeout       Message
	CallbackBizError   Message
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
	InvalidTemplateID   Message
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

type ClientMessages struct {
	Unauthorized           Message
	InvalidServerURL       Message
	InvalidServerURLScheme Message
	InvalidServerURLHost   Message
	CreateClientFailed     Message
	APIErrorFormat         Message
	ErrorResponsePrefix    Message
	BuildVerifyRequest     Message
	VerifyFailed           Message
	VerifyUnexpectedStatus Message
	InvalidSandboxDomain   Message
	InvalidDomainScheme    Message
	NothingToStore         Message
	StoreCredentialFailed  Message
	LoadCredentialFailed   Message
	NoCredential           Message
}

type EnvDMessages struct {
	EmptyDomain          Message
	EmptyConnectToken    Message
	InvalidDomain        Message
	InvalidDomainScheme  Message
	InvalidDomainHost    Message
	EncodeStartBody      Message
	ReadStartEvent       Message
	ExpectedStartEvent   Message
	StreamEndedMidFrame  Message
	InvalidStreamFrame   Message
	ReadFramePayload     Message
	UnexpectedFrameFlags Message
	DecodeFrame          Message
	RPCError             Message
	DecodeTrailer        Message
	StreamError          Message
	BuildRequest         Message
	OpenStream           Message
	StreamHTTPError      Message
	StreamContentType    Message
	EncodeBody           Message
	CallFailed           Message
	UnaryHTTPError       Message
	UnaryError           Message
}

type ParseMessages struct {
	ExpectedKeyValue Message
}

type ConfigMessages struct {
	BindEnvFailed         Message
	ReadConfigFileFailed  Message
	ResolveHomeFailed     Message
	ParseConfigFileFailed Message
	CreateConfigDirFailed Message
	EncodeConfigFailed    Message
	WriteConfigFileFailed Message
}

type OutputMessages struct {
	UnknownFormat Message
	EncodeFailed  Message
}

var Msg Messages
