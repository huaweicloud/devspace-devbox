package i18n

// Messages is the tree of all user-facing strings of the devbox CLI.
type Messages struct {
	Common   CommonMessages
	Auth     AuthMessages
	Sandbox  SandboxMessages
	Template TemplateMessages
	Client   ClientMessages
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
	CallbackForbidden  Message
	CallbackNoMethod   Message
	CallbackBadBody    Message
	CallbackLoginFail  Message
	CallbackMissingKey Message
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
	KillAllConflict     Message
	KillRequiresArgs    Message
	KillPartialFailed   Message
	FlagKillAll         Message
	FlagKillState       Message
	FlagKillMetadata    Message
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
	StdinForwardFailed  Message
	CloseStdinFailed    Message
	MissingCommand      Message
	FlagTimeout         Message
	FlagNewTimeout      Message
	FlagTimeoutKeep     Message
	InvalidTimeout      Message
	FlagMetadata        Message
	FlagMetadataFilter  Message
	FlagTemplateFilter  Message
	FlagEnv             Message
	FlagEnvTerm         Message
	FlagBuildID         Message
	FlagOnTimeout       Message
	FlagAutoResume      Message
	FlagNetworkPublic   Message
	FlagDetach          Message
	FlagCwd             Message
	FlagStartedAfter    Message
	FlagOrder           Message
	InvalidOrder        Message
	InvalidStartedAfter Message
	FlagState           Message
	FlagLimit           Message
	InvalidLimit        Message
	FlagFormat          Message
	FlagDuration        Message
	FlagBackground      Message
	FlagUser            Message
}

type TemplateMessages struct {
	TemplateShort Message
	ListShort     Message
	ListEmpty     Message
}

type ClientMessages struct {
	Unauthorized           Message
	UpgradeRequired        Message
	UpdateLabel            Message
	InvalidServerURL       Message
	InvalidServerURLScheme Message
	InvalidServerURLHost   Message
	CreateClientFailed     Message
	APIErrorFormat         Message
	ErrorResponsePrefix    Message
	BuildVerifyRequest     Message
	VerifyFailed           Message
	VerifyUnexpectedStatus Message
	EmptyResponseBody      Message
	InvalidSandboxDomain   Message
	InvalidDomainScheme    Message
	NothingToStore         Message
	StoreCredentialFailed  Message
	LoadCredentialFailed   Message
	NoCredential           Message
}

type ParseMessages struct {
	ExpectedKeyValue Message
}

type ConfigMessages struct {
	ReadConfigFileFailed  Message
	ResolveHomeFailed     Message
	ParseConfigFileFailed Message
	CreateConfigDirFailed Message
	EncodeConfigFailed    Message
	WriteConfigFileFailed Message
}

type OutputMessages struct {
	UnknownFormat     Message
	EncodeFailed      Message
	RowLengthMismatch Message
}

var Msg Messages
