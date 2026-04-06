package bridge

import (
	"fmt"
	"log"
	"strings"

	"github.com/8-Lambda-8/sACN_ledfx_bridge/coalesce"
	"github.com/8-Lambda-8/sACN_ledfx_bridge/ledfx"
)

// StateCallback is called whenever bridge state changes (for TUI updates).
type StateCallback func()

// Bridge maps DMX channel values to LedFx API calls.
// All API calls are dispatched via coalesce senders (non-blocking).
type Bridge struct {
	Config  Config
	State   State
	Client  ledfx.Client
	OnState StateCallback

	brightness    *coalesce.Sender[float64]
	effects       *coalesce.Sender[effectsMsg]
	virtualConfig *coalesce.Sender[virtualConfigMsg]
	color         *coalesce.Sender[colorMsg]
	scene         *coalesce.Sender[sceneMsg]
	playlist      *coalesce.Sender[playlistMsg]
	effectType    *coalesce.Sender[effectTypeMsg]
}

type effectsMsg struct {
	key   string
	value interface{}
}

type virtualConfigMsg struct {
	config   map[string]interface{}
	virtuals []string
}

type colorMsg struct {
	color string // hex or name; empty = clear
	clear bool
}

type sceneMsg struct {
	id     string
	action string // "activate" or "deactivate"
}

type playlistMsg struct {
	id   string // empty = stop
	stop bool
}

type effectTypeMsg struct {
	effectName string
	virtuals   []string
}

func PrettifyName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// New creates a Bridge with coalesce senders wired to the client.
func New(cfg Config, client ledfx.Client, onState StateCallback) *Bridge {
	if onState == nil {
		onState = func() {}
	}
	b := &Bridge{
		Config:  cfg,
		State:   NewState(),
		Client:  client,
		OnState: onState,
	}

	b.brightness = coalesce.New(func(val float64) {
		if err := client.SetBrightness(val); err != nil {
			log.Printf("SetBrightness error: %v", err)
		}
	})

	b.effects = coalesce.New(func(msg effectsMsg) {
		if err := client.ApplyGlobal(msg.key, msg.value); err != nil {
			log.Printf("ApplyGlobal(%s) error: %v", msg.key, err)
		}
	})

	b.virtualConfig = coalesce.New(func(msg virtualConfigMsg) {
		for _, vid := range msg.virtuals {
			if err := client.UpdateEffectConfig(vid, msg.config); err != nil {
				log.Printf("UpdateEffectConfig(%s) error: %v", vid, err)
			}
		}
	})

	b.color = coalesce.New(func(msg colorMsg) {
		if msg.clear {
			if b.State.ActiveScene != "OFF" {
				_ = client.ActivateScene(b.State.ActiveScene, "activate")
			} else {
				_ = client.ForceColor("black")
			}
		} else {
			if err := client.ForceColor(msg.color); err != nil {
				log.Printf("ForceColor error: %v", err)
			}
		}
	})

	b.scene = coalesce.New(func(msg sceneMsg) {
		if err := client.ActivateScene(msg.id, msg.action); err != nil {
			log.Printf("ActivateScene error: %v", err)
		}
	})

	b.playlist = coalesce.New(func(msg playlistMsg) {
		if msg.stop {
			if err := client.StopPlaylist(); err != nil {
				log.Printf("StopPlaylist error: %v", err)
			}
		} else {
			if err := client.StartPlaylist(msg.id); err != nil {
				log.Printf("StartPlaylist error: %v", err)
			}
		}
	})

	b.effectType = coalesce.New(func(msg effectTypeMsg) {
		for _, vid := range msg.virtuals {
			if err := client.SetEffect(vid, msg.effectName); err != nil {
				log.Printf("SetEffect(%s, %s) error: %v", vid, msg.effectName, err)
			}
		}
	})

	return b
}

// Close shuts down all coalesce workers, flushing pending calls.
func (b *Bridge) Close() {
	b.brightness.Close()
	b.effects.Close()
	b.virtualConfig.Close()
	b.color.Close()
	b.scene.Close()
	b.playlist.Close()
	b.effectType.Close()
}

func (b *Bridge) ensureVirtuals() []string {
	if len(b.State.CachedVirtuals) == 0 {
		if virts, err := b.Client.LoadVirtuals(); err == nil {
			b.State.CachedVirtuals = virts
		}
	}
	return b.State.CachedVirtuals
}

func (b *Bridge) effectiveBrightness() float64 {
	if b.State.BlackoutActive {
		return 0
	}
	eff := b.State.CurrentBrightness * (1.0 - b.State.CurrentDuck)
	if eff < 0 {
		return 0
	}
	return eff
}

func (b *Bridge) clearForceColor() {
	b.State.ActiveColor = "OFF"
	b.color.Send(colorMsg{clear: true})
}

// HandleDMX processes a DMX data frame and dispatches API calls.
func (b *Bridge) HandleDMX(dataSlice []byte) {
	var data [512]byte
	copy(data[:], dataSlice)
	ch := FixedChannels()
	first := b.State.FirstFrame

	// changed returns true if the value differs from last, or if this is the first frame.
	changed := func(v byte, last *byte) bool {
		if first || v != *last {
			*last = v
			return true
		}
		return false
	}

	// Ch 1: Scene
	if ch.Scene > 0 {
		v := data[ch.Scene-1]
		if changed(v, &b.State.LastScene) {
			if v == 0 {
				if b.State.ActiveScene != "OFF" {
					sceneID := b.State.ActiveScene
					b.State.ActiveScene = "OFF"
					b.OnState()
					b.scene.Send(sceneMsg{id: sceneID, action: "deactivate"})
				}
			} else if int(v) <= len(b.Config.Scenes) {
				sceneID := b.Config.Scenes[v-1]
				b.State.ActiveScene = sceneID
				if b.State.ActivePlaylist != "OFF" {
					b.State.ActivePlaylist = "OFF"
					b.playlist.Send(playlistMsg{stop: true})
				}
				b.OnState()
				b.scene.Send(sceneMsg{id: sceneID, action: "activate"})
			}
		}
	}

	// Ch 2: Playlist
	if ch.Playlist > 0 {
		v := data[ch.Playlist-1]
		if changed(v, &b.State.LastPlaylist) {
			if v == 0 {
				if b.State.ActivePlaylist != "OFF" {
					b.State.ActivePlaylist = "OFF"
					b.OnState()
					b.playlist.Send(playlistMsg{stop: true})
				}
			} else if int(v) <= len(b.Config.Playlists) {
				playlistID := b.Config.Playlists[v-1]
				if b.State.ActiveScene != "OFF" {
					sceneID := b.State.ActiveScene
					b.State.ActiveScene = "OFF"
					b.scene.Send(sceneMsg{id: sceneID, action: "deactivate"})
				}
				b.State.ActivePlaylist = playlistID
				b.OnState()
				b.playlist.Send(playlistMsg{id: playlistID})
			}
		}
	}

	// Ch 3: Effect type override
	if ch.Effect > 0 {
		v := data[ch.Effect-1]
		if changed(v, &b.State.LastEffect) {
			if v == 0 {
				if b.State.ActiveEffect != "OFF" {
					b.State.ActiveEffect = "OFF"
					b.OnState()
					if b.State.ActiveScene != "OFF" {
						b.scene.Send(sceneMsg{id: b.State.ActiveScene, action: "activate"})
					}
				}
			} else if int(v) <= len(b.Config.Effects) {
				effectName := b.Config.Effects[v-1]
				b.State.ActiveEffect = PrettifyName(effectName)
				b.OnState()
				virts := b.ensureVirtuals()
				b.effectType.Send(effectTypeMsg{effectName: effectName, virtuals: virts})
			}
		}
	}

	// Ch 4: Transition speed (per-virtual config update)
	if ch.Transition > 0 {
		v := data[ch.Transition-1]
		if changed(v, &b.State.LastTransition) {
			virts := b.ensureVirtuals()
			b.virtualConfig.Send(virtualConfigMsg{
				config:   map[string]interface{}{"transition_time": float64(v) / 255.0 * 5.0},
				virtuals: virts,
			})
		}
	}

	// Ch 5: Effect speed / sensitivity (per-virtual config update)
	if ch.Speed > 0 {
		v := data[ch.Speed-1]
		if changed(v, &b.State.LastSpeed) {
			b.State.CurrentSensitivity = float64(v) / 255.0
			if !b.State.FreezeActive {
				virts := b.ensureVirtuals()
				b.virtualConfig.Send(virtualConfigMsg{
					config:   map[string]interface{}{"sensitivity": b.State.CurrentSensitivity},
					virtuals: virts,
				})
			}
		}
	}

	// Ch 6: Color / Gradient palette
	if ch.Palette > 0 {
		v := data[ch.Palette-1]
		if changed(v, &b.State.LastPalette) {
			numColors := len(b.Config.Colors)
			if v == 0 {
				if b.State.ActivePalette != "OFF" {
					b.State.ActivePalette = "OFF"
					b.State.ActiveColor = "OFF"
					b.State.ActiveGradient = "OFF"
					b.OnState()
					b.clearForceColor()
				}
			} else if int(v) <= numColors {
				colorName := b.Config.Colors[v-1]
				b.State.ActivePalette = "Color: " + PrettifyName(colorName)
				b.State.ActiveColor = PrettifyName(colorName)
				b.State.ActiveGradient = "OFF"
				b.OnState()
				b.color.Send(colorMsg{color: colorName})
			} else if int(v) <= numColors+len(b.Config.Gradients) {
				gradIdx := int(v) - numColors - 1
				gradName := b.Config.Gradients[gradIdx]
				b.State.ActivePalette = "Grad: " + PrettifyName(gradName)
				b.State.ActiveGradient = PrettifyName(gradName)
				b.State.ActiveColor = "OFF"
				b.OnState()
				b.clearForceColor()
				b.effects.Send(effectsMsg{key: "gradient", value: gradName})
			}
		}
	}

	// Ch 7-9: RGB
	if ch.Red > 0 && ch.Green > 0 && ch.Blue > 0 {
		r, g, bv := data[ch.Red-1], data[ch.Green-1], data[ch.Blue-1]
		if first || r != b.State.LastRed || g != b.State.LastGreen || bv != b.State.LastBlue {
			b.State.LastRed = r
			b.State.LastGreen = g
			b.State.LastBlue = bv
			b.State.RedVal = r
			b.State.GreenVal = g
			b.State.BlueVal = bv

			if r == 0 && g == 0 && bv == 0 {
				if b.State.RGBActive {
					b.State.RGBActive = false
					b.State.ActiveColor = "OFF"
					b.OnState()
					b.clearForceColor()
				}
			} else {
				b.State.RGBActive = true
				hex := fmt.Sprintf("#%02x%02x%02x", r, g, bv)
				b.State.ActiveColor = "RGB " + hex
				b.OnState()
				b.color.Send(colorMsg{color: hex})
			}
		}
	}

	// Ch 10: Brightness
	if ch.Brightness > 0 {
		v := data[ch.Brightness-1]
		if changed(v, &b.State.LastBrightness) {
			b.State.CurrentBrightness = float64(v) / 255.0
			b.OnState()
			b.brightness.Send(b.effectiveBrightness())
		}
	}

	// Ch 11: Duck
	if ch.Duck > 0 {
		v := data[ch.Duck-1]
		if changed(v, &b.State.LastDuck) {
			b.State.CurrentDuck = float64(v) / 255.0
			b.OnState()
			b.brightness.Send(b.effectiveBrightness())
		}
	}

	// Ch 12: Freeze
	if ch.Freeze > 0 {
		v := data[ch.Freeze-1]
		if changed(v, &b.State.LastFreeze) {
			wasFrozen := b.State.FreezeActive
			b.State.FreezeActive = v >= 128
			if b.State.FreezeActive != wasFrozen {
				b.OnState()
				virts := b.ensureVirtuals()
				if b.State.FreezeActive {
					b.virtualConfig.Send(virtualConfigMsg{
						config:   map[string]interface{}{"sensitivity": 0.0},
						virtuals: virts,
					})
				} else {
					b.virtualConfig.Send(virtualConfigMsg{
						config:   map[string]interface{}{"sensitivity": b.State.CurrentSensitivity},
						virtuals: virts,
					})
				}
			}
		}
	}

	// Ch 13: Strobe
	if ch.Strobe > 0 {
		v := data[ch.Strobe-1]
		if changed(v, &b.State.LastStrobe) {
			wasStrobe := b.State.StrobeActive
			b.State.StrobeActive = v >= 128
			if b.State.StrobeActive != wasStrobe {
				b.OnState()
				if b.State.StrobeActive {
					b.color.Send(colorMsg{color: "white"})
				} else {
					b.clearForceColor()
				}
			}
		}
	}

	// Ch 14: Blackout (highest priority)
	if ch.Blackout > 0 {
		v := data[ch.Blackout-1]
		if changed(v, &b.State.LastBlackout) {
			wasBlackout := b.State.BlackoutActive
			b.State.BlackoutActive = v >= 128
			if b.State.BlackoutActive != wasBlackout {
				b.OnState()
				b.brightness.Send(b.effectiveBrightness())
			}
		}
	}

	b.State.FirstFrame = false
}
