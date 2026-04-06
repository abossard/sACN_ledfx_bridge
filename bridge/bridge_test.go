package bridge

import (
	"fmt"
	"math"
	"testing"

	"github.com/8-Lambda-8/sACN_ledfx_bridge/ledfx"
)

// dmxFrame builds a [512]byte with specific channel values set.
func dmxFrame(settings ...chVal) []byte {
	data := make([]byte, 512)
	for _, s := range settings {
		if s.ch > 0 && s.ch <= 512 {
			data[s.ch-1] = s.val
		}
	}
	return data
}

type chVal struct {
	ch  int
	val byte
}

func ch(channel int, value byte) chVal {
	return chVal{ch: channel, val: value}
}

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.Scenes = []string{"beat-reactor", "cyberpunk-pulse", "northern-lights"}
	cfg.Playlists = []string{"party-mix", "chill-vibes"}
	cfg.Effects = []string{"energy(Reactive)", "bars(Reactive)", "spectrum(Reactive)"}
	cfg.Colors = []string{"red", "blue", "green"}
	cfg.Gradients = []string{"Rainbow", "Ocean"}
	cfg.ColorHex = map[string]string{"red": "#ff0000", "blue": "#0000ff", "green": "#00ff00"}
	return cfg
}

func assertMethod(t *testing.T, calls []ledfx.Call, index int, method string) {
	t.Helper()
	if index >= len(calls) {
		t.Fatalf("expected call %d (%s) but only got %d calls", index, method, len(calls))
	}
	if calls[index].Method != method {
		t.Errorf("call %d: method = %q, want %q", index, calls[index].Method, method)
	}
}

func assertArg(t *testing.T, calls []ledfx.Call, index int, argIdx int, want interface{}) {
	t.Helper()
	if index >= len(calls) {
		t.Fatalf("expected call %d but only got %d calls", index, len(calls))
	}
	if argIdx >= len(calls[index].Args) {
		t.Fatalf("call %d: expected arg %d but only got %d args", index, argIdx, len(calls[index].Args))
	}
	got := calls[index].Args[argIdx]
	// Float comparison with tolerance
	if wf, ok := want.(float64); ok {
		if gf, ok := got.(float64); ok {
			if math.Abs(gf-wf) < 0.01 {
				return
			}
		}
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("call %d arg %d = %v, want %v", index, argIdx, got, want)
	}
}

func TestHandleDMX(t *testing.T) {
	tests := []struct {
		name       string
		frames     [][]byte
		assertFunc func(t *testing.T, mock *ledfx.Mock, b *Bridge)
	}{
		{
			name:   "scene activation",
			frames: [][]byte{dmxFrame(ch(1, 1))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "ActivateScene")
				assertArg(t, calls, 0, 0, "beat-reactor")
				assertArg(t, calls, 0, 1, "activate")
				if b.State.ActiveScene != "beat-reactor" {
					t.Errorf("ActiveScene = %q, want beat-reactor", b.State.ActiveScene)
				}
			},
		},
		{
			name:   "scene deactivation",
			frames: [][]byte{dmxFrame(ch(1, 1)), dmxFrame(ch(1, 0))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				if b.State.ActiveScene != "OFF" {
					t.Errorf("ActiveScene = %q, want OFF", b.State.ActiveScene)
				}
			},
		},
		{
			name:   "playlist start",
			frames: [][]byte{dmxFrame(ch(2, 1))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "StartPlaylist")
				assertArg(t, calls, 0, 0, "party-mix")
			},
		},
		{
			name:   "brightness sets global",
			frames: [][]byte{dmxFrame(ch(10, 128))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "SetBrightness")
				assertArg(t, calls, 0, 0, 128.0/255.0)
			},
		},
		{
			name:   "brightness 0 means dark",
			frames: [][]byte{dmxFrame(ch(10, 128)), dmxFrame(ch(10, 0))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				// Last call should be SetBrightness(0)
				last := calls[len(calls)-1]
				if last.Method != "SetBrightness" {
					t.Fatalf("last call = %q, want SetBrightness", last.Method)
				}
				assertArg(t, calls, len(calls)-1, 0, 0.0)
			},
		},
		{
			name:   "blackout overrides brightness",
			frames: [][]byte{dmxFrame(ch(10, 255), ch(14, 200))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				// Last brightness call should be 0 (blackout)
				calls := mock.GetCalls()
				last := calls[len(calls)-1]
				if last.Method != "SetBrightness" {
					t.Fatalf("last call = %q, want SetBrightness", last.Method)
				}
				assertArg(t, calls, len(calls)-1, 0, 0.0)
			},
		},
		{
			name:   "duck multiplies brightness",
			frames: [][]byte{dmxFrame(ch(10, 255), ch(11, 128))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				// effective = 1.0 * (1 - 128/255) ≈ 0.498
				calls := mock.GetCalls()
				last := calls[len(calls)-1]
				assertMethod(t, calls, len(calls)-1, "SetBrightness")
				got := last.Args[0].(float64)
				want := 1.0 * (1.0 - 128.0/255.0)
				if math.Abs(got-want) > 0.01 {
					t.Errorf("brightness = %f, want ~%f", got, want)
				}
			},
		},
		{
			name:   "RGB sends hex",
			frames: [][]byte{dmxFrame(ch(7, 255), ch(8, 0), ch(9, 128))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "ForceColor")
				assertArg(t, calls, 0, 0, "#ff0080")
				if !b.State.RGBActive {
					t.Error("RGBActive should be true")
				}
			},
		},
		{
			name:   "RGB all zero clears",
			frames: [][]byte{dmxFrame(ch(7, 255), ch(8, 128), ch(9, 64)), dmxFrame(ch(7, 0), ch(8, 0), ch(9, 0))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				if b.State.RGBActive {
					t.Error("RGBActive should be false after all zeros")
				}
			},
		},
		{
			name:   "palette color forces solid",
			frames: [][]byte{dmxFrame(ch(6, 1))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "ForceColor")
				assertArg(t, calls, 0, 0, "red")
			},
		},
		{
			name:   "palette gradient applies global",
			frames: [][]byte{dmxFrame(ch(6, 4))}, // 3 colors + 1st gradient = Rainbow
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				found := false
				for _, c := range calls {
					if c.Method == "ApplyGlobal" && len(c.Args) >= 2 {
						if c.Args[0] == "gradient" && c.Args[1] == "Rainbow" {
							found = true
						}
					}
				}
				if !found {
					t.Errorf("expected ApplyGlobal(gradient, Rainbow), got %v", calls)
				}
			},
		},
		{
			name:   "freeze sets sensitivity 0",
			frames: [][]byte{dmxFrame(ch(12, 200))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				if !b.State.FreezeActive {
					t.Error("FreezeActive should be true")
				}
				calls := mock.GetCalls()
				found := false
				for _, c := range calls {
					if c.Method == "UpdateEffectConfig" && len(c.Args) >= 2 {
						cfg, ok := c.Args[1].(map[string]interface{})
						if ok && cfg["sensitivity"] == 0.0 {
							found = true
						}
					}
				}
				if !found {
					t.Errorf("expected UpdateEffectConfig with sensitivity=0, got %v", calls)
				}
			},
		},
		{
			name:   "strobe forces white",
			frames: [][]byte{dmxFrame(ch(13, 200))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				assertMethod(t, calls, 0, "ForceColor")
				assertArg(t, calls, 0, 0, "white")
			},
		},
		{
			name:   "duplicate value ignored",
			frames: [][]byte{dmxFrame(ch(10, 128)), dmxFrame(ch(10, 128))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				// Should only have 1 SetBrightness call (dedup)
				count := 0
				for _, c := range calls {
					if c.Method == "SetBrightness" {
						count++
					}
				}
				if count != 1 {
					t.Errorf("SetBrightness called %d times, want 1 (dedup)", count)
				}
			},
		},
		{
			name:   "speed controls sensitivity",
			frames: [][]byte{dmxFrame(ch(5, 128))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				found := false
				for _, c := range calls {
					if c.Method == "UpdateEffectConfig" && len(c.Args) >= 2 {
						cfg, ok := c.Args[1].(map[string]interface{})
						if ok {
							if s, ok := cfg["sensitivity"].(float64); ok && s > 0.49 && s < 0.51 {
								found = true
							}
						}
					}
				}
				if !found {
					t.Errorf("expected UpdateEffectConfig with sensitivity≈0.50, got %v", calls)
				}
			},
		},
		{
			name:   "transition speed",
			frames: [][]byte{dmxFrame(ch(4, 255))},
			assertFunc: func(t *testing.T, mock *ledfx.Mock, b *Bridge) {
				calls := mock.GetCalls()
				found := false
				for _, c := range calls {
					if c.Method == "UpdateEffectConfig" && len(c.Args) >= 2 {
						cfg, ok := c.Args[1].(map[string]interface{})
						if ok {
							if tt, ok := cfg["transition_time"].(float64); ok && tt > 4.9 {
								found = true
							}
						}
					}
				}
				if !found {
					t.Errorf("expected UpdateEffectConfig with transition_time≈5.0, got %v", calls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			mock := &ledfx.Mock{}
			b := New(cfg, mock, nil)
			b.State.CachedVirtuals = []string{"peter", "dummy"}
			b.State.FirstFrame = false // tests start with known clean state

			for _, frame := range tt.frames {
				b.HandleDMX(frame)
			}
			b.Close() // flush pending coalesce calls

			tt.assertFunc(t, mock, b)
		})
	}
}

func TestFirstFrame(t *testing.T) {
	cfg := testConfig()
	mock := &ledfx.Mock{}
	b := New(cfg, mock, nil)
	b.State.CachedVirtuals = []string{"peter"}
	// FirstFrame is true by default — should process brightness=0 on first frame
	b.HandleDMX(make([]byte, 512))
	b.Close()

	calls := mock.GetCalls()
	found := false
	for _, c := range calls {
		if c.Method == "SetBrightness" {
			found = true
		}
	}
	if !found {
		t.Error("expected SetBrightness on first frame with all-zero data")
	}
}

func TestPrettifyName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"beat-reactor", "Beat Reactor"},
		{"cyberpunk-pulse", "Cyberpunk Pulse"},
		{"red", "Red"},
		{"energy(Reactive)", "Energy(Reactive)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := PrettifyName(tt.input); got != tt.want {
				t.Errorf("PrettifyName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
