# sACN ledfx Bridge

> **⚠️ Disclaimer:** This fork is shamelessly vibecoded on top of [8-Lambda-8's original](https://github.com/8-Lambda-8/sACN_ledfx_bridge). It might only work on my machine 😉

Control LedFx scenes, playlists, brightness, and blackout via sACN/DMX — designed for use with QLC+.

## What's different in this fork?

Compared to the [original by 8-Lambda-8](https://github.com/8-Lambda-8/sACN_ledfx_bridge), this fork adds:

- **Multi-channel DMX control** — the original used a single DMX channel for scene selection; this fork supports independent channels for scene, playlist, brightness, blackout, and color
- **Playlist support** — start/stop LedFx playlists via a dedicated DMX channel
- **Brightness control** — set global LedFx brightness (0–255 → 0%–100%) via DMX
- **Blackout toggle** — instant blackout via DMX threshold (128–255), restores previous brightness on release
- **Color override channel** — force a color on all virtuals via DMX, with a configurable color palette
- **QLC+ fixture generator** — export a QLC+ fixture definition XML directly from the TUI (press `e`), auto-installs to the correct OS-specific directory
- **Debounced brightness updates** — 50ms debounce to avoid flooding the LedFx API
- **Improved error handling** — non-fatal logging instead of crashing on API errors


## Rewritten in GO
for old nodejs version go to [nodejs branch](https://github.com/8-Lambda-8/sACN_ledfx_bridge/tree/nodejs)

## Features

- **Scene Selection** — activate LedFx scenes by DMX channel value
- **Playlist Selection** — start/stop LedFx playlists by DMX channel value
- **Brightness Control** — set global LedFx brightness (0–255 → 0%–100%)
- **Blackout** — instant blackout toggle via DMX threshold

## DMX Channel Layout

Each function is mapped to its own configurable DMX channel (set channel to `0` to disable):

| Config Key   | Default Ch | DMX Value | Action                                    |
|--------------|------------|-----------|-------------------------------------------|
| `scene`      | 1          | 0         | Deactivate current scene                  |
|              |            | 1–N       | Activate scene N (stops active playlist)   |
| `playlist`   | 2          | 0         | Stop current playlist                     |
|              |            | 1–N       | Start playlist N (deactivates active scene)|
| `brightness` | 3          | 0–255     | Set global brightness (value / 255)        |
| `blackout`   | 4          | 0–127     | Normal operation                          |
|              |            | 128–255   | Blackout (force brightness to 0)           |

**Priority rules:**
- Blackout overrides brightness — when released, the brightness channel value is restored
- Scene and playlist are mutually exclusive
- Brightness updates are debounced (50ms) to avoid flooding the LedFx API

## Configuration

```json
{
  "sAcnUniverse": 1,
  "channels": {
    "scene": 1,
    "playlist": 2,
    "brightness": 3,
    "blackout": 4
  },
  "scenes": ["beat-reactor", "cyberpunk-pulse", "..."],
  "playlists": ["ambient-mood", "party-mix", "..."],
  "ledfx_host": "http://127.0.0.1:8888"
}
```

Scene and playlist IDs can be loaded directly from the LedFx API via the TUI.

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
  - Scene Channel: 1 (Default)
  - Playlist Channel: 2 (Default)
  - Brightness Channel: 3 (Default)
  - Blackout Channel: 4 (Default)
  - LedFx Host: http://127.0.0.1:8888 (Default)
  - Scenes: go into scene submenu and select "get scenes from LedFx" to load all scenes
  - Playlists: go into playlist submenu and select "get playlists from LedFx" to load all playlists


![image](https://github.com/user-attachments/assets/27a1b6d9-208f-4606-9000-ac30cd6a63e1)
