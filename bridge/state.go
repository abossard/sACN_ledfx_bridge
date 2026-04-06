package bridge

// State holds all runtime state for the bridge.
type State struct {
	ActiveScene    string
	ActivePlaylist string
	ActivePalette  string
	ActiveColor    string
	ActiveGradient string
	ActiveEffect   string

	BlackoutActive bool
	StrobeActive   bool
	FreezeActive   bool
	RGBActive      bool

	CurrentBrightness float64
	CurrentDuck       float64
	CurrentSensitivity float64

	// Last DMX values for change detection
	LastScene      byte
	LastPlaylist   byte
	LastEffect     byte
	LastTransition byte
	LastSpeed      byte
	LastPalette    byte
	LastRed        byte
	LastGreen      byte
	LastBlue       byte
	LastBrightness byte
	LastDuck       byte
	LastFreeze     byte
	LastStrobe     byte
	LastBlackout   byte

	// Current RGB values for display
	RedVal   byte
	GreenVal byte
	BlueVal  byte

	// Cached virtual IDs for effect switching
	CachedVirtuals []string

	// FirstFrame is true until the first DMX frame is fully processed.
	// On the first frame, all channels are processed regardless of last values.
	FirstFrame bool
}

func NewState() State {
	return State{
		ActiveScene:        "OFF",
		ActivePlaylist:     "OFF",
		ActivePalette:      "OFF",
		ActiveColor:        "OFF",
		ActiveGradient:     "OFF",
		ActiveEffect:       "OFF",
		CurrentBrightness:  1.0,
		CurrentSensitivity: 1.0,
		FirstFrame:         true,
	}
}
