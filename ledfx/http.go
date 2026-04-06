package ledfx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"time"
)

// HTTPClient implements Client using the LedFx REST API.
type HTTPClient struct {
	Host       string
	Debug      bool
	httpClient *http.Client
	logger     *log.Logger
}

func NewHTTPClient(host string) *HTTPClient {
	return &HTTPClient{
		Host:       host,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     log.Default(),
	}
}

// SetLogger sets a custom logger for debug output.
func (c *HTTPClient) SetLogger(l *log.Logger) {
	c.logger = l
}

func (c *HTTPClient) debugf(format string, args ...interface{}) {
	if c.Debug && c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func (c *HTTPClient) request(method, path string, body interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	c.debugf("→ %s %s %s", method, path, string(payload))
	req, err := http.NewRequest(method, c.Host+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debugf("✗ %s %s error: %v", method, path, err)
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		c.debugf("✗ %s %s %d: %s", method, path, resp.StatusCode, string(respBody))
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	c.debugf("✓ %s %s %d: %s", method, path, resp.StatusCode, string(respBody))
	return nil
}

func (c *HTTPClient) get(path string) ([]byte, error) {
	c.debugf("→ GET %s", path)
	resp, err := http.Get(c.Host + path)
	if err != nil {
		c.debugf("✗ GET %s error: %v", path, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		c.debugf("✗ GET %s %d: %s", path, resp.StatusCode, string(body))
		return nil, fmt.Errorf("GET %s: %d %s", path, resp.StatusCode, string(body))
	}
	c.debugf("✓ GET %s %d (%d bytes)", path, resp.StatusCode, len(body))
	return body, nil
}

func (c *HTTPClient) SetBrightness(value float64) error {
	return c.request(http.MethodPut, "/api/config", map[string]interface{}{
		"global_brightness": value,
	})
}

func (c *HTTPClient) ForceColor(color string) error {
	return c.request(http.MethodPut, "/api/virtuals_tools", map[string]interface{}{
		"tool": "force_color", "color": color,
	})
}

func (c *HTTPClient) ApplyGlobal(key string, value interface{}) error {
	return c.request(http.MethodPut, "/api/effects", map[string]interface{}{
		"action": "apply_global",
		key:      value,
	})
}

func (c *HTTPClient) ActivateScene(id, action string) error {
	return c.request(http.MethodPut, "/api/scenes", map[string]interface{}{
		"id": id, "action": action,
	})
}

func (c *HTTPClient) StartPlaylist(id string) error {
	return c.request(http.MethodPut, "/api/playlists", map[string]interface{}{
		"action": "start", "id": id,
	})
}

func (c *HTTPClient) StopPlaylist() error {
	return c.request(http.MethodPut, "/api/playlists", map[string]interface{}{
		"action": "stop",
	})
}

func (c *HTTPClient) SetEffect(virtualID, effectType string) error {
	return c.request(http.MethodPut, "/api/virtuals/"+virtualID+"/effects", map[string]interface{}{
		"type": effectType,
	})
}

func (c *HTTPClient) UpdateEffectConfig(virtualID string, config map[string]interface{}) error {
	return c.request(http.MethodPut, "/api/virtuals/"+virtualID+"/effects", map[string]interface{}{
		"config": config,
	})
}

func (c *HTTPClient) LoadScenes() ([]string, error) {
	body, err := c.get("/api/scenes")
	if err != nil {
		return nil, err
	}
	var obj struct {
		Scenes map[string]struct{ Name string `json:"name"` } `json:"scenes"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	var out []string
	for k := range obj.Scenes {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

func (c *HTTPClient) LoadPlaylists() ([]string, error) {
	body, err := c.get("/api/playlists")
	if err != nil {
		return nil, err
	}
	var obj struct {
		Playlists map[string]struct{ Name string `json:"name"` } `json:"playlists"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	var out []string
	for k := range obj.Playlists {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

func (c *HTTPClient) LoadEffects() ([]string, error) {
	body, err := c.get("/api/schema")
	if err != nil {
		return nil, err
	}
	var obj struct {
		Effects map[string]interface{} `json:"effects"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parse effects: %w", err)
	}
	var out []string
	for k := range obj.Effects {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

func (c *HTTPClient) LoadGradients() ([]string, error) {
	body, err := c.get("/api/config")
	if err != nil {
		return nil, err
	}
	var obj struct {
		UserGradients map[string]interface{} `json:"user_gradients"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parse gradients: %w", err)
	}
	var out []string
	for k := range obj.UserGradients {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

func (c *HTTPClient) LoadColors() (map[string]string, error) {
	body, err := c.get("/api/colors")
	if err != nil {
		return nil, err
	}
	var obj struct {
		Colors struct {
			Builtin map[string]string `json:"builtin"`
			User    map[string]string `json:"user"`
		} `json:"colors"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for name, hex := range obj.Colors.Builtin {
		if name != "black" {
			out[name] = hex
		}
	}
	for name, hex := range obj.Colors.User {
		out[name] = hex
	}
	return out, nil
}

func (c *HTTPClient) LoadVirtuals() ([]string, error) {
	body, err := c.get("/api/virtuals")
	if err != nil {
		return nil, err
	}
	var obj struct {
		Virtuals map[string]interface{} `json:"virtuals"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	var out []string
	for k := range obj.Virtuals {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}
