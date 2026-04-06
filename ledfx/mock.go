package ledfx

import "sync"

// Call records a single method invocation on the mock.
type Call struct {
	Method string
	Args   []interface{}
}

// Mock implements Client by recording all calls for test assertions.
type Mock struct {
	mu    sync.Mutex
	Calls []Call
}

func (m *Mock) record(method string, args ...interface{}) {
	m.mu.Lock()
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
	m.mu.Unlock()
}

// Reset clears recorded calls.
func (m *Mock) Reset() {
	m.mu.Lock()
	m.Calls = nil
	m.mu.Unlock()
}

// GetCalls returns a snapshot of recorded calls.
func (m *Mock) GetCalls() []Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Call, len(m.Calls))
	copy(out, m.Calls)
	return out
}

func (m *Mock) SetBrightness(value float64) error {
	m.record("SetBrightness", value)
	return nil
}

func (m *Mock) ForceColor(color string) error {
	m.record("ForceColor", color)
	return nil
}

func (m *Mock) ApplyGlobal(key string, value interface{}) error {
	m.record("ApplyGlobal", key, value)
	return nil
}

func (m *Mock) ActivateScene(id, action string) error {
	m.record("ActivateScene", id, action)
	return nil
}

func (m *Mock) StartPlaylist(id string) error {
	m.record("StartPlaylist", id)
	return nil
}

func (m *Mock) StopPlaylist() error {
	m.record("StopPlaylist")
	return nil
}

func (m *Mock) SetEffect(virtualID, effectType string) error {
	m.record("SetEffect", virtualID, effectType)
	return nil
}

func (m *Mock) UpdateEffectConfig(virtualID string, config map[string]interface{}) error {
	m.record("UpdateEffectConfig", virtualID, config)
	return nil
}

func (m *Mock) LoadScenes() ([]string, error) {
	m.record("LoadScenes")
	return nil, nil
}

func (m *Mock) LoadPlaylists() ([]string, error) {
	m.record("LoadPlaylists")
	return nil, nil
}

func (m *Mock) LoadEffects() ([]string, error) {
	m.record("LoadEffects")
	return nil, nil
}

func (m *Mock) LoadGradients() ([]string, error) {
	m.record("LoadGradients")
	return nil, nil
}

func (m *Mock) LoadColors() (map[string]string, error) {
	m.record("LoadColors")
	return nil, nil
}

func (m *Mock) LoadVirtuals() ([]string, error) {
	m.record("LoadVirtuals")
	return nil, nil
}
