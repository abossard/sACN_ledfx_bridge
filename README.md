# sACN ledfx Bridge

Control LedFx scenes, playlists, brightness, and blackout via sACN/DMX — designed for use with QLC+.

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
![image](https://github.com/user-attachments/assets/ce494616-2060-41cf-95fc-bc634cd4999f)

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

### Backward Compatibility

Old config files using the legacy `"channel"` field are automatically migrated to the new `"channels"` format on load.


![image](https://github.com/user-attachments/assets/27a1b6d9-208f-4606-9000-ac30cd6a63e1)
