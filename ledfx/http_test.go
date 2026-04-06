package ledfx

import (
	"log"
	"os"
	"testing"
)

// These tests run against a real LedFx instance.
// Set LEDFX_HOST=http://127.0.0.1:8888 to enable.

func liveClient(t *testing.T) *HTTPClient {
	host := os.Getenv("LEDFX_HOST")
	if host == "" {
		t.Skip("LEDFX_HOST not set, skipping live test")
	}
	c := NewHTTPClient(host)
	c.Debug = true
	c.SetLogger(log.New(os.Stdout, "[ledfx] ", log.LstdFlags))
	return c
}

func TestLive_LoadScenes(t *testing.T) {
	c := liveClient(t)
	scenes, err := c.LoadScenes()
	if err != nil {
		t.Fatalf("LoadScenes: %v", err)
	}
	t.Logf("Scenes (%d): %v", len(scenes), scenes)
	if len(scenes) == 0 {
		t.Error("expected at least one scene")
	}
}

func TestLive_LoadPlaylists(t *testing.T) {
	c := liveClient(t)
	playlists, err := c.LoadPlaylists()
	if err != nil {
		t.Fatalf("LoadPlaylists: %v", err)
	}
	t.Logf("Playlists (%d): %v", len(playlists), playlists)
}

func TestLive_LoadEffects(t *testing.T) {
	c := liveClient(t)
	effects, err := c.LoadEffects()
	if err != nil {
		t.Fatalf("LoadEffects: %v", err)
	}
	t.Logf("Effects (%d): %v", len(effects), effects)
	if len(effects) == 0 {
		t.Error("expected at least one effect type")
	}
}

func TestLive_LoadGradients(t *testing.T) {
	c := liveClient(t)
	gradients, err := c.LoadGradients()
	if err != nil {
		t.Fatalf("LoadGradients: %v", err)
	}
	t.Logf("Gradients (%d): %v", len(gradients), gradients)
}

func TestLive_LoadColors(t *testing.T) {
	c := liveClient(t)
	colors, err := c.LoadColors()
	if err != nil {
		t.Fatalf("LoadColors: %v", err)
	}
	t.Logf("Colors (%d): %v", len(colors), colors)
	if len(colors) == 0 {
		t.Error("expected at least one color")
	}
}

func TestLive_LoadVirtuals(t *testing.T) {
	c := liveClient(t)
	virtuals, err := c.LoadVirtuals()
	if err != nil {
		t.Fatalf("LoadVirtuals: %v", err)
	}
	t.Logf("Virtuals (%d): %v", len(virtuals), virtuals)
	if len(virtuals) == 0 {
		t.Error("expected at least one virtual")
	}
}

func TestLive_SetBrightness(t *testing.T) {
	c := liveClient(t)
	if err := c.SetBrightness(0.5); err != nil {
		t.Fatalf("SetBrightness(0.5): %v", err)
	}
	// Restore
	if err := c.SetBrightness(1.0); err != nil {
		t.Fatalf("SetBrightness(1.0) restore: %v", err)
	}
}

func TestLive_ForceColor(t *testing.T) {
	c := liveClient(t)
	if err := c.ForceColor("#ff0000"); err != nil {
		t.Fatalf("ForceColor(red hex): %v", err)
	}
	if err := c.ForceColor("blue"); err != nil {
		t.Fatalf("ForceColor(blue name): %v", err)
	}
	if err := c.ForceColor("black"); err != nil {
		t.Fatalf("ForceColor(black/clear): %v", err)
	}
}

func TestLive_ApplyGlobal_Gradient(t *testing.T) {
	c := liveClient(t)
	if err := c.ApplyGlobal("gradient", "Rainbow"); err != nil {
		t.Fatalf("ApplyGlobal(gradient, Rainbow): %v", err)
	}
}

func TestLive_ApplyGlobal_Brightness(t *testing.T) {
	c := liveClient(t)
	if err := c.ApplyGlobal("brightness", 0.8); err != nil {
		t.Fatalf("ApplyGlobal(brightness, 0.8): %v", err)
	}
	// Restore
	_ = c.ApplyGlobal("brightness", 1.0)
}

func TestLive_ActivateScene(t *testing.T) {
	c := liveClient(t)
	scenes, err := c.LoadScenes()
	if err != nil || len(scenes) == 0 {
		t.Skip("no scenes available")
	}
	scene := scenes[0]
	if err := c.ActivateScene(scene, "activate"); err != nil {
		t.Fatalf("ActivateScene(%s): %v", scene, err)
	}
	if err := c.ActivateScene(scene, "deactivate"); err != nil {
		t.Fatalf("DeactivateScene(%s): %v", scene, err)
	}
}

func TestLive_Playlist(t *testing.T) {
	c := liveClient(t)
	playlists, err := c.LoadPlaylists()
	if err != nil || len(playlists) == 0 {
		t.Skip("no playlists available")
	}
	playlist := playlists[0]
	if err := c.StartPlaylist(playlist); err != nil {
		t.Fatalf("StartPlaylist(%s): %v", playlist, err)
	}
	if err := c.StopPlaylist(); err != nil {
		t.Fatalf("StopPlaylist: %v", err)
	}
}

func TestLive_UpdateEffectConfig(t *testing.T) {
	c := liveClient(t)
	virtuals, err := c.LoadVirtuals()
	if err != nil || len(virtuals) == 0 {
		t.Skip("no virtuals available")
	}
	vid := virtuals[0]
	if err := c.UpdateEffectConfig(vid, map[string]interface{}{"sensitivity": 0.5}); err != nil {
		t.Fatalf("UpdateEffectConfig(%s, sensitivity=0.5): %v", vid, err)
	}
	// Restore
	_ = c.UpdateEffectConfig(vid, map[string]interface{}{"sensitivity": 0.8})
}

func TestLive_SetEffect(t *testing.T) {
	c := liveClient(t)
	virtuals, err := c.LoadVirtuals()
	if err != nil || len(virtuals) == 0 {
		t.Skip("no virtuals available")
	}
	vid := virtuals[0]
	if err := c.SetEffect(vid, "energy"); err != nil {
		t.Fatalf("SetEffect(%s, energy): %v", vid, err)
	}
}
