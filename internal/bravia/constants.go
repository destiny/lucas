package bravia

// Remote Control Codes for Sony Bravia TVs
const (
	// Power Controls
	PowerButton    BraviaRemoteCode = "AAAAAQAAAAEAAAAVAw=="
	PowerOn        BraviaRemoteCode = "AAAAAQAAAAEAAAAuAw=="
	PowerOff       BraviaRemoteCode = "AAAAAQAAAAEAAAAvAw=="

	// Volume Controls
	VolumeUp       BraviaRemoteCode = "AAAAAQAAAAEAAAASAw=="
	VolumeDown     BraviaRemoteCode = "AAAAAQAAAAEAAAATAw=="
	Mute           BraviaRemoteCode = "AAAAAQAAAAEAAAAUAw=="

	// Channel Controls
	ChannelUp      BraviaRemoteCode = "AAAAAQAAAAEAAAAQAw=="
	ChannelDown    BraviaRemoteCode = "AAAAAQAAAAEAAAARAw=="

	// Navigation Controls
	Up             BraviaRemoteCode = "AAAAAQAAAAEAAAB0Aw=="
	Down           BraviaRemoteCode = "AAAAAQAAAAEAAAB1Aw=="
	Left           BraviaRemoteCode = "AAAAAQAAAAEAAAA0Aw=="
	Right          BraviaRemoteCode = "AAAAAQAAAAEAAAAzAw=="
	Confirm        BraviaRemoteCode = "AAAAAQAAAAEAAABlAw=="
	
	// Menu Controls  
	Home           BraviaRemoteCode = "AAAAAQAAAAEAAABgAw=="
	Menu           BraviaRemoteCode = "AAAAAQAAAAEAAAAbAw=="
	Options        BraviaRemoteCode = "AAAAAgAAAAEAAAA2Aw=="
	Return         BraviaRemoteCode = "AAAAAgAAAAEAAAAjAw=="
	Back           BraviaRemoteCode = "AAAAAgAAAAEAAAAjAw=="

	// Input Controls
	Input          BraviaRemoteCode = "AAAAAQAAAAEAAAAlAw=="
	HDMI1          BraviaRemoteCode = "AAAAAgAAAAEAAABoAw=="
	HDMI2          BraviaRemoteCode = "AAAAAgAAAAEAAABpAw=="
	HDMI3          BraviaRemoteCode = "AAAAAgAAAAEAAABqAw=="
	HDMI4          BraviaRemoteCode = "AAAAAgAAAAEAAABrAw=="

	// Playback Controls
	Play           BraviaRemoteCode = "AAAAAgAAAAEAAAAaAw=="
	Pause          BraviaRemoteCode = "AAAAAgAAAAEAAAAZAw=="
	Stop           BraviaRemoteCode = "AAAAAgAAAAEAAAAYAw=="
	Rewind         BraviaRemoteCode = "AAAAAgAAAAEAAAAbAw=="
	FastForward    BraviaRemoteCode = "AAAAAgAAAAEAAAAcAw=="

	// Number Keys
	Num0           BraviaRemoteCode = "AAAAAQAAAAEAAAAJAw=="
	Num1           BraviaRemoteCode = "AAAAAQAAAAEAAAAAAw=="
	Num2           BraviaRemoteCode = "AAAAAQAAAAEAAAABAw=="
	Num3           BraviaRemoteCode = "AAAAAQAAAAEAAAACAw=="
	Num4           BraviaRemoteCode = "AAAAAQAAAAEAAAADAw=="
	Num5           BraviaRemoteCode = "AAAAAQAAAAEAAAAEAw=="
	Num6           BraviaRemoteCode = "AAAAAQAAAAEAAAAFAw=="
	Num7           BraviaRemoteCode = "AAAAAQAAAAEAAAAGAw=="
	Num8           BraviaRemoteCode = "AAAAAQAAAAEAAAAHAw=="
	Num9           BraviaRemoteCode = "AAAAAQAAAAEAAAAIAw=="
)

// API Endpoints for Sony Bravia Control
const (
	SystemEndpoint     BraviaEndpoint = "/sony/system"
	AVContentEndpoint  BraviaEndpoint = "/sony/avContent"
	AudioEndpoint      BraviaEndpoint = "/sony/audio"
	AppControlEndpoint BraviaEndpoint = "/sony/appControl"
	VideoScreenEndpoint BraviaEndpoint = "/sony/videoScreen"
	EncryptionEndpoint BraviaEndpoint = "/sony/encryption"
	IRCCEndpoint       BraviaEndpoint = "/sony/IRCC"
)

// API Methods for Sony Bravia Control
const (
	// System Methods
	GetPowerStatus         BraviaMethod = "getPowerStatus"
	SetPowerStatus         BraviaMethod = "setPowerStatus"
	GetSystemInformation   BraviaMethod = "getSystemInformation"
	GetSystemSupportedFunction BraviaMethod = "getSystemSupportedFunction"
	
	// Audio Methods
	GetVolumeInformation   BraviaMethod = "getVolumeInformation"
	SetAudioVolume         BraviaMethod = "setAudioVolume"
	SetAudioMute           BraviaMethod = "setAudioMute"
	GetSpeakerSettings     BraviaMethod = "getSpeakerSettings"
	SetSpeakerSettings     BraviaMethod = "setSpeakerSettings"
	
	// AV Content Methods
	GetPlayingContentInfo  BraviaMethod = "getPlayingContentInfo"
	GetContentList         BraviaMethod = "getContentList"
	GetSchemeList          BraviaMethod = "getSchemeList"
	GetSourceList          BraviaMethod = "getSourceList"
	SetPlayContent         BraviaMethod = "setPlayContent"
	
	// App Control Methods
	GetApplicationList     BraviaMethod = "getApplicationList"
	GetApplicationStatus   BraviaMethod = "getApplicationStatus"
	SetActiveApp           BraviaMethod = "setActiveApp"
	TerminateApps          BraviaMethod = "terminateApps"
	
	// Video Screen Methods
	GetBannerMode          BraviaMethod = "getBannerMode"
	SetBannerMode          BraviaMethod = "setBannerMode"
	GetSceneSetting        BraviaMethod = "getSceneSetting"
	SetSceneSetting        BraviaMethod = "setSceneSetting"
)