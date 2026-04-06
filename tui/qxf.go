package tui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/8-Lambda-8/sACN_ledfx_bridge/bridge"
)

func buildCapabilities(items []string, label string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  <Capability Min=\"0\" Max=\"0\">%s OFF</Capability>\n", label))
	for i, item := range items {
		name := html.EscapeString(bridge.PrettifyName(item))
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">%s</Capability>\n", i+1, i+1, name))
	}
	if len(items) < 255 {
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"255\">No function</Capability>\n", len(items)+1))
	}
	return b.String()
}

// GenerateQXF produces a QLC+ fixture definition XML from the config.
func GenerateQXF(cfg *bridge.Config) string {
	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE FixtureDefinition>
<FixtureDefinition xmlns="http://www.qlcplus.org/FixtureDefinition">
 <Creator>
  <Name>Q Light Controller Plus</Name>
  <Version>5.0.0</Version>
  <Author>sACN LedFx Bridge</Author>
 </Creator>
 <Manufacturer>LedFx</Manufacturer>
 <Model>sACN LedFx Bridge</Model>
 <Type>Other</Type>
`)

	b.WriteString(" <Channel Name=\"Scene Select\">\n")
	b.WriteString("  <Group Byte=\"0\">Effect</Group>\n")
	b.WriteString(buildCapabilities(cfg.Scenes, "Scene"))
	b.WriteString(" </Channel>\n")

	b.WriteString(" <Channel Name=\"Playlist Select\">\n")
	b.WriteString("  <Group Byte=\"0\">Effect</Group>\n")
	b.WriteString(buildCapabilities(cfg.Playlists, "Playlist"))
	b.WriteString(" </Channel>\n")

	b.WriteString(" <Channel Name=\"Effect Type Override\">\n")
	b.WriteString("  <Group Byte=\"0\">Effect</Group>\n")
	b.WriteString("  <Capability Min=\"0\" Max=\"0\">No override — use current scene or playlist effect</Capability>\n")
	for i, fx := range cfg.Effects {
		name := html.EscapeString(bridge.PrettifyName(fx))
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">%s</Capability>\n", i+1, i+1, name))
	}
	if len(cfg.Effects) < 255 {
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"255\">No function</Capability>\n", len(cfg.Effects)+1))
	}
	b.WriteString(" </Channel>\n")

	b.WriteString(` <Channel Name="Transition Speed">
  <Group Byte="0">Speed</Group>
  <Capability Min="0" Max="0">Instant transition — no blending between effects/scenes</Capability>
  <Capability Min="1" Max="127">Fast transition — 0.02s to 2.5s blend</Capability>
  <Capability Min="128" Max="128">Medium transition — 2.5s blend between effects</Capability>
  <Capability Min="129" Max="255">Slow transition — 2.5s to 5.0s smooth blend</Capability>
 </Channel>
`)

	b.WriteString(` <Channel Name="Effect Speed / Sensitivity">
  <Group Byte="0">Speed</Group>
  <Capability Min="0" Max="0">Freeze — effects stop reacting to audio</Capability>
  <Capability Min="1" Max="127">Low sensitivity — subtle audio response</Capability>
  <Capability Min="128" Max="128">Medium sensitivity — balanced audio response (default)</Capability>
  <Capability Min="129" Max="255">High sensitivity — intense audio response</Capability>
 </Channel>
`)

	numColors := len(cfg.Colors)
	b.WriteString(" <Channel Name=\"Color / Gradient\">\n")
	b.WriteString("  <Group Byte=\"0\">Colour</Group>\n")
	b.WriteString("  <Capability Min=\"0\" Max=\"0\">No override — use scene default colors and palette</Capability>\n")
	for i, color := range cfg.Colors {
		name := html.EscapeString(bridge.PrettifyName(color))
		hex := cfg.ColorHex[color]
		if hex != "" {
			b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\" Preset=\"ColorMacro\" Res1=\"%s\">● %s</Capability>\n",
				i+1, i+1, hex, name))
		} else {
			b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">● %s</Capability>\n",
				i+1, i+1, name))
		}
	}
	for i, grad := range cfg.Gradients {
		name := html.EscapeString(bridge.PrettifyName(grad))
		dmxVal := numColors + i + 1
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">◆ %s (gradient)</Capability>\n",
			dmxVal, dmxVal, name))
	}
	total := numColors + len(cfg.Gradients)
	if total < 255 {
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"255\">No function</Capability>\n", total+1))
	}
	b.WriteString(" </Channel>\n")

	b.WriteString(" <Channel Name=\"Red\" Preset=\"IntensityRed\"/>\n")
	b.WriteString(" <Channel Name=\"Green\" Preset=\"IntensityGreen\"/>\n")
	b.WriteString(" <Channel Name=\"Blue\" Preset=\"IntensityBlue\"/>\n")

	b.WriteString(" <Channel Name=\"Brightness\" Preset=\"IntensityMasterDimmer\"/>\n")

	b.WriteString(` <Channel Name="Brightness Duck (Sidechain)">
  <Group Byte="0">Intensity</Group>
  <Capability Min="0" Max="0">No ducking — LEDs at full brightness fader level</Capability>
  <Capability Min="1" Max="127">Partial duck — LEDs dimmed proportionally (use with moving head cues)</Capability>
  <Capability Min="128" Max="254">Heavy duck — LEDs mostly dark while beams are active</Capability>
  <Capability Min="255" Max="255">Full duck — LEDs completely dark (sidechain fully engaged)</Capability>
 </Channel>
`)

	b.WriteString(` <Channel Name="Freeze / Hold">
  <Group Byte="0">Shutter</Group>
  <Capability Min="0" Max="127">Normal — effects react to audio in real time</Capability>
  <Capability Min="128" Max="255">Freeze — hold current LED state, ignore audio input (sensitivity to 0)</Capability>
 </Channel>
`)

	b.WriteString(` <Channel Name="Strobe / Flash">
  <Group Byte="0">Shutter</Group>
  <Capability Min="0" Max="127" Preset="ShutterOpen">Normal — no flash (LED effects visible)</Capability>
  <Capability Min="128" Max="255" Preset="ShutterStrobeSlowFast">Flash — all LEDs forced white (use QLC+ chaser for strobe effect)</Capability>
 </Channel>
`)

	b.WriteString(` <Channel Name="Blackout">
  <Group Byte="0">Shutter</Group>
  <Capability Min="0" Max="127" Preset="ShutterOpen">Normal operation</Capability>
  <Capability Min="128" Max="255" Preset="ShutterClose">Blackout — all LEDs forced off (overrides everything)</Capability>
 </Channel>
`)

	b.WriteString(" <Mode Name=\"14 Channel\">\n")
	b.WriteString("  <Channel Number=\"0\">Scene Select</Channel>\n")
	b.WriteString("  <Channel Number=\"1\">Playlist Select</Channel>\n")
	b.WriteString("  <Channel Number=\"2\">Effect Type Override</Channel>\n")
	b.WriteString("  <Channel Number=\"3\">Transition Speed</Channel>\n")
	b.WriteString("  <Channel Number=\"4\">Effect Speed / Sensitivity</Channel>\n")
	b.WriteString("  <Channel Number=\"5\">Color / Gradient</Channel>\n")
	b.WriteString("  <Channel Number=\"6\">Red</Channel>\n")
	b.WriteString("  <Channel Number=\"7\">Green</Channel>\n")
	b.WriteString("  <Channel Number=\"8\">Blue</Channel>\n")
	b.WriteString("  <Channel Number=\"9\">Brightness</Channel>\n")
	b.WriteString("  <Channel Number=\"10\">Brightness Duck (Sidechain)</Channel>\n")
	b.WriteString("  <Channel Number=\"11\">Freeze / Hold</Channel>\n")
	b.WriteString("  <Channel Number=\"12\">Strobe / Flash</Channel>\n")
	b.WriteString("  <Channel Number=\"13\">Blackout</Channel>\n")
	b.WriteString(" </Mode>\n")

	b.WriteString(` <Physical>
  <Bulb Type="LED" Lumens="0" ColourTemperature="0"/>
  <Dimensions Weight="0" Width="0" Height="0" Depth="0"/>
  <Lens Name="Other" DegreesMin="0" DegreesMax="0"/>
  <Focus Type="Fixed" PanMax="0" TiltMax="0"/>
  <Technical PowerConsumption="0" DmxConnector="Other"/>
 </Physical>
`)

	b.WriteString("</FixtureDefinition>\n")
	return b.String()
}

func qlcFixtureDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "QLC+", "Fixtures")
	case "linux":
		return filepath.Join(home, ".qlcplus", "Fixtures")
	case "windows":
		return filepath.Join(home, "QLC+", "Fixtures")
	default:
		return ""
	}
}

// ExportFixture writes the QXF file and returns (path, message).
func ExportFixture(cfg *bridge.Config) (string, string) {
	qxf := GenerateQXF(cfg)
	filename := "LedFx-sACN-LedFx-Bridge.qxf"

	dir := qlcFixtureDir()
	if dir != "" {
		if _, err := os.Stat(dir); err == nil {
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(qxf), 0644); err == nil {
				return path, fmt.Sprintf("Exported to %s — Restart QLC+ to load fixture", path)
			}
		}
		if err := os.MkdirAll(dir, 0755); err == nil {
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(qxf), 0644); err == nil {
				return path, fmt.Sprintf("Exported to %s — Restart QLC+ to load fixture", path)
			}
		}
	}

	path := filename
	if err := os.WriteFile(path, []byte(qxf), 0644); err != nil {
		return "", fmt.Sprintf("Export failed: %v", err)
	}
	return path, fmt.Sprintf("Exported to ./%s — Copy to QLC+ Fixtures folder, then restart QLC+", path)
}
