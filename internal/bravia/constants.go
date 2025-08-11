package bravia

// Remote Control Codes for Sony Bravia TVs
const (
	// Power Controls
	PowerButton BraviaRemoteCode = "AAAAAQAAAAEAAAAVAw=="
	PowerOn     BraviaRemoteCode = "AAAAAQAAAAEAAAAuAw=="
	PowerOff    BraviaRemoteCode = "AAAAAQAAAAEAAAAvAw=="

	// Volume Controls
	VolumeUp   BraviaRemoteCode = "AAAAAQAAAAEAAAASAw=="
	VolumeDown BraviaRemoteCode = "AAAAAQAAAAEAAAATAw=="
	Mute       BraviaRemoteCode = "AAAAAQAAAAEAAAAUAw=="

	// Channel Controls
	ChannelUp   BraviaRemoteCode = "AAAAAQAAAAEAAAAQAw=="
	ChannelDown BraviaRemoteCode = "AAAAAQAAAAEAAAARAw=="

	// Navigation Controls
	Up      BraviaRemoteCode = "AAAAAQAAAAEAAAB0Aw=="
	Down    BraviaRemoteCode = "AAAAAQAAAAEAAAB1Aw=="
	Left    BraviaRemoteCode = "AAAAAQAAAAEAAAA0Aw=="
	Right   BraviaRemoteCode = "AAAAAQAAAAEAAAAzAw=="
	Confirm BraviaRemoteCode = "AAAAAQAAAAEAAABlAw=="

	// Menu Controls
	Home BraviaRemoteCode = "AAAAAQAAAAEAAABgAw=="
	Menu BraviaRemoteCode = "AAAAAQAAAAEAAAAbAw==" // Require to update that is not work
	Back BraviaRemoteCode = "AAAAAQAAAAEAAABjAw=="

	// Input Controls
	Input BraviaRemoteCode = "AAAAAQAAAAEAAAAlAw=="
	HDMI1 BraviaRemoteCode = "AAAAAgAAAAEAAABoAw=="
	HDMI2 BraviaRemoteCode = "AAAAAgAAAAEAAABpAw=="
	HDMI3 BraviaRemoteCode = "AAAAAgAAAAEAAABqAw=="
	HDMI4 BraviaRemoteCode = "AAAAAgAAAAEAAABrAw=="

	// Number Keys
	Num0 BraviaRemoteCode = "AAAAAQAAAAEAAAAJAw=="
	Num1 BraviaRemoteCode = "AAAAAQAAAAEAAAAAAw=="
	Num2 BraviaRemoteCode = "AAAAAQAAAAEAAAABAw=="
	Num3 BraviaRemoteCode = "AAAAAQAAAAEAAAACAw=="
	Num4 BraviaRemoteCode = "AAAAAQAAAAEAAAADAw=="
	Num5 BraviaRemoteCode = "AAAAAQAAAAEAAAAEAw=="
	Num6 BraviaRemoteCode = "AAAAAQAAAAEAAAAFAw=="
	Num7 BraviaRemoteCode = "AAAAAQAAAAEAAAAGAw=="
	Num8 BraviaRemoteCode = "AAAAAQAAAAEAAAAHAw=="
	Num9 BraviaRemoteCode = "AAAAAQAAAAEAAAAIAw=="
)

// API Endpoints for Sony Bravia Control
const (
	SystemEndpoint      BraviaEndpoint = "/sony/system"
	AVContentEndpoint   BraviaEndpoint = "/sony/avContent"
	AudioEndpoint       BraviaEndpoint = "/sony/audio"
	AppControlEndpoint  BraviaEndpoint = "/sony/appControl"
	VideoScreenEndpoint BraviaEndpoint = "/sony/videoScreen"
	EncryptionEndpoint  BraviaEndpoint = "/sony/encryption"
	IRCCEndpoint        BraviaEndpoint = "/sony/ircc"
)

// API Methods for Sony Bravia Control
const (
	// System Methods
	GetPowerStatus       BraviaMethod = "getPowerStatus"
	GetSystemInformation BraviaMethod = "getSystemInformation"

	// Audio Methods
	GetVolumeInformation BraviaMethod = "getVolumeInformation"
	SetAudioVolume       BraviaMethod = "setAudioVolume"
	SetAudioMute         BraviaMethod = "setAudioMute"

	// AV Content Methods
	GetPlayingContentInfo BraviaMethod = "getPlayingContentInfo"
	GetContentList        BraviaMethod = "getContentList"

	// App Control Methods
	GetApplicationList BraviaMethod = "getApplicationList"
)
