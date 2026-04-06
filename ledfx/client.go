package ledfx

// Client defines the interface for communicating with the LedFx API.
// Implementations include HTTPClient (real) and Mock (for tests).
type Client interface {
	// Control
	SetBrightness(value float64) error
	ForceColor(color string) error
	ApplyGlobal(key string, value interface{}) error
	ActivateScene(id, action string) error
	StartPlaylist(id string) error
	StopPlaylist() error
	SetEffect(virtualID, effectType string) error
	UpdateEffectConfig(virtualID string, config map[string]interface{}) error

	// Loaders
	LoadScenes() ([]string, error)
	LoadPlaylists() ([]string, error)
	LoadEffects() ([]string, error)
	LoadGradients() ([]string, error)
	LoadColors() (map[string]string, error)
	LoadVirtuals() ([]string, error)
}
