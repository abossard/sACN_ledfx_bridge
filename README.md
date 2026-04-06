# sACN ledfx Bridge

> **⚠️ Disclaimer:** This fork is shamelessly vibecoded on top of [8-Lambda-8's original](https://github.com/8-Lambda-8/sACN_ledfx_bridge). It might only work on my machine 😉

Control [LedFx](https://github.com/LedFx/LedFx) scenes, playlists, effects, brightness, and more via sACN/DMX — designed for live shows with [QLC+](https://www.qlcplus.org/).

## What's different in this fork?

The [original by 8-Lambda-8](https://github.com/8-Lambda-8/sACN_ledfx_bridge) has a single DMX channel for scene selection. This fork turns it into a full 14-channel live show controller:

### New DMX channels
- **Playlist control** — start/stop LedFx playlists via DMX (mutually exclusive with scenes)
- **Effect type override** — switch the effect running on all [virtuals](https://docs.ledfx.app/en/latest/howto/virtuals.html) via DMX, bypassing scenes/playlists
- **Effect speed / sensitivity** — fader controlling audio reactivity via [`apply_global`](https://docs.ledfx.app/en/v2.1.2/apis/global.html)
- **Transition speed** — fader controlling effect blend timing via `apply_global transition_time`
- **Color / Gradient palette** — merged channel: preset colors use [`force_color`](https://docs.ledfx.app/en/latest/apis/api.html), gradients use `apply_global gradient_name`
- **RGB color override** — 3-channel RGB mixer sending arbitrary hex to `force_color` (QLC+ auto-shows a color picker!)
- **Brightness control** — global brightness via [`PUT /api/config`](https://docs.ledfx.app/en/latest/apis/api.html)
- **Brightness duck (sidechain)** — multiplier fader for moving-head interaction (beam fires → LEDs dim, beam off → LEDs restore)
- **Freeze / hold** — hold current LED visual state by setting effect sensitivity to 0
- **Strobe / flash** — force all LEDs white above DMX threshold; use QLC+ chaser for strobe effect
- **Blackout** — ultimate override, forces brightness to 0

### New features
- **14-channel QLC+ fixture generator** — export a self-documenting `.qxf` fixture definition from the TUI (press `e`), with `ColorMacro` swatches, `IntensityRed/Green/Blue` presets, and descriptive capabilities
- **Full busking support** — effect type + gradient + speed + RGB gives complete improvised control without pre-built scenes
- **Immediate forwarding** — every new DMX value is forwarded instantly; duplicate values are silently ignored
- **Centralized HTTP helper** — `ledfxRequest()` replaces scattered `http.Client` calls
- **Non-fatal error handling** — `log.Printf` instead of `log.Fatal` on API errors
- **TUI submenus for effects, gradients, and colors** — load available options directly from LedFx API

### LedFx API endpoints used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| [`/api/scenes`](https://docs.ledfx.app/en/latest/apis/scenes.html) | PUT | Activate/deactivate scenes |
| [`/api/playlists`](https://docs.ledfx.app/en/latest/apis/api.html) | PUT | Start/stop playlists |
| [`/api/config`](https://docs.ledfx.app/en/latest/apis/api.html) | PUT | Set global brightness |
| [`/api/effects`](https://docs.ledfx.app/en/v2.1.2/apis/global.html) | PUT | `apply_global` for sensitivity, gradient, transition |
| [`/api/virtuals/{id}/effects`](https://docs.ledfx.app/en/latest/apis/api.html) | PUT | Set effect type per virtual |
| [`/api/virtuals_tools`](https://docs.ledfx.app/en/latest/apis/api.html) | PUT | `force_color` for solid colors and RGB |
| `/api/schema/effects` | GET | Load available effect types |
| `/api/gradients` | GET | Load available gradient presets |
| [`/api/colors`](https://docs.ledfx.app/en/latest/apis/api.html) | GET | Load available color names + hex values |
| `/api/virtuals` | GET | Cache virtual IDs for effect switching |


## Rewritten in GO
for old nodejs version go to [nodejs branch](https://github.com/8-Lambda-8/sACN_ledfx_bridge/tree/nodejs)

## Features

- **Scene Selection** — activate LedFx scenes by DMX channel value
- **Playlist Selection** — start/stop LedFx playlists by DMX channel value
- **Effect Type Override** — set the effect running on all virtuals, bypassing scenes/playlists
- **Transition Speed** — control blend timing between scenes/effects (0–5s)
- **Effect Speed / Sensitivity** — control how strongly effects react to audio
- **Color / Gradient** — preset palette selection (colors force solid, gradients change palette)
- **RGB Color Override** — 3-channel mixer for arbitrary colors (R, G, B)
- **Brightness Control** — set global LedFx brightness (0–255 → 0%–100%)
- **Brightness Duck** — sidechain dimmer for moving-head interaction
- **Freeze / Hold** — pause effects, hold current visual state
- **Strobe / Flash** — force all LEDs white (use QLC+ chaser for strobe)
- **Blackout** — instant blackout toggle, overrides everything

## DMX Channel Layout

Channels are sorted by priority — higher channels override lower ones. Set any channel to `0` to disable.

| Config Key   | Default Ch | DMX Value | Action                                          |
|--------------|------------|-----------|--------------------------------------------------|
| `scene`      | 1          | 0         | Deactivate current scene                         |
|              |            | 1–N       | Activate scene N (stops active playlist)          |
| `playlist`   | 2          | 0         | Stop current playlist                            |
|              |            | 1–N       | Start playlist N (deactivates active scene)       |
| `effect`     | 3          | 0         | No override (use current scene/playlist effect)   |
|              |            | 1–N       | Set effect type N on all virtuals                 |
| `transition` | 4          | 0–255     | Transition speed (0 = instant, 255 = 5s blend)    |
| `speed`      | 5          | 0         | Freeze (sensitivity = 0)                          |
|              |            | 1–255     | Audio sensitivity (1 = subtle, 255 = intense)     |
| `palette`    | 6          | 0         | No override (use scene defaults)                  |
|              |            | 1–N       | Force color N (solid color on all virtuals)        |
|              |            | N+1–N+M   | Apply gradient M (palette change on effects)       |
| `red`        | 7          | 0–255     | Red component for RGB color override              |
| `green`      | 8          | 0–255     | Green component for RGB color override             |
| `blue`       | 9          | 0–255     | Blue component for RGB color override              |
| `brightness` | 10         | 0–255     | Set base brightness (value / 255)                 |
| `duck`       | 11         | 0         | No ducking (full brightness)                      |
|              |            | 1–255     | Duck brightness (255 = fully dark, for beam cues)  |
| `freeze`     | 12         | 0–127     | Normal (effects react to audio)                   |
|              |            | 128–255   | Freeze (hold current visual, ignore audio)         |
| `strobe`     | 13         | 0–127     | Normal (no flash)                                 |
|              |            | 128–255   | Flash (force all LEDs white)                      |
| `blackout`   | 14         | 0–127     | Normal operation                                  |
|              |            | 128–255   | Blackout (force brightness to 0, overrides all)    |

**Priority rules:**
- Blackout (ch 14) overrides everything
- Strobe (ch 13) overrides visual output when active
- Freeze (ch 12) overrides audio reactivity
- Duck (ch 11) multiplies brightness: `effective = brightness × (1 - duck/255)`
- RGB override (ch 7-9) takes precedence over palette preset (ch 6) when any R/G/B > 0
- Scene and playlist are mutually exclusive
- Effect type override bypasses scene/playlist effects
- All faders forward every new value immediately; duplicates are ignored
- To disable any channel entirely, set it to `0` in the config

### Off states (DMX value 0)

| Channel | DMX 0 means |
|---------|-------------|
| Scene, Playlist, Effect, Palette | No override / deactivate |
| R + G + B (all three = 0) | Clear RGB override |
| Speed | Freeze (sensitivity = 0) |
| Transition | Instant (0s blend) |
| Duck | No ducking (full brightness) |
| **Brightness** | **Dark (0%) — standard DMX dimmer, always active** |
| Freeze, Strobe, Blackout | 0–127 = normal operation |

> **Note:** Brightness at 0 means LEDs are dark — this is standard DMX dimmer behavior. To stop controlling brightness entirely, set `channels.brightness: 0` in config.

## Configuration

```json
{
  "sAcnUniverse": 1,
  "channels": {
    "scene": 1,
    "playlist": 2,
    "effect": 3,
    "transition": 4,
    "speed": 5,
    "palette": 6,
    "red": 7,
    "green": 8,
    "blue": 9,
    "brightness": 10,
    "duck": 11,
    "freeze": 12,
    "strobe": 13,
    "blackout": 14
  },
  "scenes": ["beat-reactor", "cyberpunk-pulse", "..."],
  "playlists": ["ambient-mood", "party-mix", "..."],
  "effects": ["energy(Reactive)", "bars(Reactive)", "..."],
  "gradients": ["Rainbow", "Ocean", "..."],
  "colors": ["red", "blue", "green", "..."],
  "ledfx_host": "http://127.0.0.1:8888"
}
```

Scene, playlist, effect, gradient, and color IDs can be loaded directly from the LedFx API via the TUI submenus.

## New TUI

![alt text](screenshot.png)

### Example configuration for QLC+:
- QLC+, LedFx and sACN_ledfx_bridge running on same machine
- QLC:
  - 127.0.0.1 network
  - Multicast: off
  - Port: 5568 (Default)
  - E1.31 Universe 1 (Default)
  - Transmission Mode: Full (Default)
  - Priority: 100 (Default)
- Bridge:
  - Universe: 1 (Default)
  - Channels 1–14 (all defaults): Scene, Playlist, Effect, Transition, Speed, Palette, R, G, B, Brightness, Duck, Freeze, Strobe, Blackout
  - LedFx Host: http://127.0.0.1:8888 (Default)
  - Use TUI submenus to load scenes, playlists, effects, gradients, and colors from LedFx
  - Press `e` to export the QLC+ fixture definition


![image](https://github.com/user-attachments/assets/27a1b6d9-208f-4606-9000-ac30cd6a63e1)
