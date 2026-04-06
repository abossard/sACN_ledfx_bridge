package bridge

// Channels maps each DMX function to a fixed channel number (1-based).
// The layout is hardcoded and matches the generated QXF fixture definition.
type Channels struct {
	Scene      uint64
	Playlist   uint64
	Effect     uint64
	Transition uint64
	Speed      uint64
	Palette    uint64
	Red        uint64
	Green      uint64
	Blue       uint64
	Brightness uint64
	Duck       uint64
	Freeze     uint64
	Strobe     uint64
	Blackout   uint64
}

// FixedChannels returns the hardcoded 14-channel layout.
func FixedChannels() Channels {
	return Channels{
		Scene:      1,
		Playlist:   2,
		Effect:     3,
		Transition: 4,
		Speed:      5,
		Palette:    6,
		Red:        7,
		Green:      8,
		Blue:       9,
		Brightness: 10,
		Duck:       11,
		Freeze:     12,
		Strobe:     13,
		Blackout:   14,
	}
}

// Config holds the bridge configuration, serialized to/from JSON.
type Config struct {
	Universe   uint64            `json:"sAcnUniverse"`
	Scenes     []string          `json:"scenes"`
	Playlists  []string          `json:"playlists"`
	Effects    []string          `json:"effects"`
	Gradients  []string          `json:"gradients"`
	Colors     []string          `json:"colors"`
	ColorHex   map[string]string `json:"colorHex,omitempty"`
	LedFx_host string            `json:"ledfx_host"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Universe:   1,
		Scenes:     []string{},
		Playlists:  []string{},
		Effects:    []string{},
		Gradients:  []string{},
		Colors:     []string{},
		ColorHex:   map[string]string{},
		LedFx_host: "http://127.0.0.1:8888",
	}
}
