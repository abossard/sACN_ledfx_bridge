package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Hundemeier/go-sacn/sacn"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mattn/go-isatty"
)

var p *tea.Program

// Setting indices for TUI navigation
const (
	settUniverse = iota
	settSceneCh
	settPlaylistCh
	settBrightnessCh
	settBlackoutCh
	settColorCh
	settHost
	settScenes
	settPlaylists
	settColors
	settSave
)

type Channels struct {
	Scene      uint64 `json:"scene"`
	Playlist   uint64 `json:"playlist"`
	Brightness uint64 `json:"brightness"`
	Blackout   uint64 `json:"blackout"`
	Color      uint64 `json:"color"`
}

type Config struct {
	Universe   uint64            `json:"sAcnUniverse"`
	Channel    uint64            `json:"channel,omitempty"` // legacy, migrated to Channels.Scene
	Channels   Channels          `json:"channels"`
	Scenes     []string          `json:"scenes"`
	Playlists  []string          `json:"playlists"`
	Colors     []string          `json:"colors"`
	ColorHex   map[string]string `json:"colorHex,omitempty"`
	LedFx_host string            `json:"ledfx_host"`
}

func ledfxRequest(method, path string, body interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(method, configData.LedFx_host+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func activateScene(sceneId string, deactivate bool) {
	action := "activate"
	if deactivate {
		if ActiveScene == "OFF" {
			return
		}
		sceneId = ActiveScene
		ActiveScene = "OFF"
		action = "deactivate"
	} else {
		ActiveScene = sceneId
		if ActivePlaylist != "OFF" {
			stopPlaylist()
		}
	}
	p.Send(updateStatusMsg{})

	err := ledfxRequest(http.MethodPut, "/api/scenes", map[string]interface{}{
		"id": sceneId, "action": action,
	})
	if err != nil {
		log.Printf("activateScene error: %v", err)
	}
}

func activatePlaylist(playlistId string) {
	if ActiveScene != "OFF" {
		// Deactivate scene without stopping playlist (we're about to start one)
		sceneId := ActiveScene
		ActiveScene = "OFF"
		_ = ledfxRequest(http.MethodPut, "/api/scenes", map[string]interface{}{
			"id": sceneId, "action": "deactivate",
		})
	}
	ActivePlaylist = playlistId
	p.Send(updateStatusMsg{})

	err := ledfxRequest(http.MethodPut, "/api/playlists", map[string]interface{}{
		"action": "start", "id": playlistId,
	})
	if err != nil {
		log.Printf("activatePlaylist error: %v", err)
	}
}

func stopPlaylist() {
	ActivePlaylist = "OFF"
	p.Send(updateStatusMsg{})

	err := ledfxRequest(http.MethodPut, "/api/playlists", map[string]interface{}{
		"action": "stop",
	})
	if err != nil {
		log.Printf("stopPlaylist error: %v", err)
	}
}

func setGlobalBrightness(value float64) {
	err := ledfxRequest(http.MethodPut, "/api/config", map[string]interface{}{
		"global_brightness": value,
	})
	if err != nil {
		log.Printf("setGlobalBrightness error: %v", err)
	}
}

func forceColor(colorName string) {
	ActiveColor = prettifyName(colorName)
	p.Send(updateStatusMsg{})

	err := ledfxRequest(http.MethodPut, "/api/virtuals_tools", map[string]interface{}{
		"tool": "force_color", "color": colorName,
	})
	if err != nil {
		log.Printf("forceColor error: %v", err)
	}
}

func clearForceColor() {
	ActiveColor = "OFF"
	p.Send(updateStatusMsg{})

	if ActiveScene != "OFF" {
		_ = ledfxRequest(http.MethodPut, "/api/scenes", map[string]interface{}{
			"id": ActiveScene, "action": "activate",
		})
	} else {
		_ = ledfxRequest(http.MethodPut, "/api/virtuals_tools", map[string]interface{}{
			"tool": "force_color", "color": "black",
		})
	}
}

var ActiveScene = "OFF"
var ActivePlaylist = "OFF"
var ActiveColor = "OFF"
var BlackoutActive = false
var currentBrightness float64 = 1.0

var sceneChVal byte = 0
var lastSceneChVal byte = 0
var playlistChVal byte = 0
var lastPlaylistChVal byte = 0
var brightnessChVal byte = 0
var lastBrightnessChVal byte = 0
var blackoutChVal byte = 0
var lastBlackoutChVal byte = 0
var colorChVal byte = 0
var lastColorChVal byte = 0

var brightnessTimer *time.Timer

var configFromFile bool = false

var configData Config = Config{
	Universe: 1,
	Channels: Channels{
		Scene:      1,
		Playlist:   2,
		Brightness: 3,
		Blackout:   4,
		Color:      5,
	},
	Scenes:     []string{},
	Playlists:  []string{},
	Colors:     []string{},
	ColorHex:   map[string]string{},
	LedFx_host: "http://127.0.0.1:8888",
}
var tempScenes = []string{}
var tempPlaylists = []string{}
var tempColors = []string{}

var configFile string

func main() {

	var (
		daemonMode bool
		showHelp   bool
		opts       []tea.ProgramOption
	)

	flag.BoolVar(&daemonMode, "d", false, "run as a daemon")
	flag.StringVar(&configFile, "c", "./config.json", "config file path")
	flag.BoolVar(&showHelp, "h", false, "show help")
	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if daemonMode || !isatty.IsTerminal(os.Stdout.Fd()) {
		// If we're in daemon mode don't render the TUI
		opts = []tea.ProgramOption{tea.WithoutRenderer()}
	} else {
		// If we're in TUI mode, discard log output
		log.SetOutput(io.Discard)
	}

	file, err := os.ReadFile(configFile)
	if err == nil {
		err = json.Unmarshal(file, &configData)
		if err != nil {
			log.Fatal(err)
		}
		// Migrate legacy "channel" field to "channels.scene"
		if configData.Channel != 0 && configData.Channels.Scene == 0 {
			configData.Channels.Scene = configData.Channel
			configData.Channel = 0
		}
		configFromFile = true
	}

	recv, err := sacn.NewReceiverSocket("", nil)
	if err != nil {
		log.Fatal(err)
	}

	recv.SetOnChangeCallback(func(old sacn.DataPacket, newD sacn.DataPacket) {
		if newD.Universe() != uint16(configData.Universe) {
			return
		}
		p.Send(recievingMsg(newD.Universe()))
		data := newD.Data()

		// Scene channel
		if configData.Channels.Scene > 0 {
			sceneChVal = data[configData.Channels.Scene-1]
			if sceneChVal != lastSceneChVal {
				lastSceneChVal = sceneChVal
				if sceneChVal == 0 {
					activateScene(ActiveScene, true)
				} else if int(sceneChVal) <= len(configData.Scenes) {
					activateScene(configData.Scenes[sceneChVal-1], false)
				}
			}
		}

		// Playlist channel
		if configData.Channels.Playlist > 0 {
			playlistChVal = data[configData.Channels.Playlist-1]
			if playlistChVal != lastPlaylistChVal {
				lastPlaylistChVal = playlistChVal
				if playlistChVal == 0 {
					if ActivePlaylist != "OFF" {
						stopPlaylist()
					}
				} else if int(playlistChVal) <= len(configData.Playlists) {
					activatePlaylist(configData.Playlists[playlistChVal-1])
				}
			}
		}

		// Brightness channel (debounced)
		if configData.Channels.Brightness > 0 {
			brightnessChVal = data[configData.Channels.Brightness-1]
			if brightnessChVal != lastBrightnessChVal {
				lastBrightnessChVal = brightnessChVal
				brightness := float64(brightnessChVal) / 255.0
				currentBrightness = brightness
				p.Send(updateStatusMsg{})

				if brightnessTimer != nil {
					brightnessTimer.Stop()
				}
				brightnessTimer = time.AfterFunc(50*time.Millisecond, func() {
					if !BlackoutActive {
						setGlobalBrightness(brightness)
					}
				})
			}
		}

		// Blackout channel (threshold at 128)
		if configData.Channels.Blackout > 0 {
			blackoutChVal = data[configData.Channels.Blackout-1]
			if blackoutChVal != lastBlackoutChVal {
				lastBlackoutChVal = blackoutChVal
				wasBlackout := BlackoutActive
				BlackoutActive = blackoutChVal >= 128

				if BlackoutActive != wasBlackout {
					p.Send(updateStatusMsg{})
					if BlackoutActive {
						setGlobalBrightness(0)
					} else {
						setGlobalBrightness(currentBrightness)
					}
				}
			}
		}

		// Color override channel
		if configData.Channels.Color > 0 {
			colorChVal = data[configData.Channels.Color-1]
			if colorChVal != lastColorChVal {
				lastColorChVal = colorChVal
				if colorChVal == 0 {
					if ActiveColor != "OFF" {
						clearForceColor()
					}
				} else if int(colorChVal) <= len(configData.Colors) {
					forceColor(configData.Colors[colorChVal-1])
				}
			}
		}
	})
	recv.SetTimeoutCallback(func(univ uint16) {
		if univ == uint16(configData.Universe) {
			p.Send(timeOutMsg(univ))
		}
	})
	recv.Start()

	p = tea.NewProgram(initialModel(), opts...)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func loadLedfxScenes() {
	var resp *http.Response
	resp, err := http.Get(
		configData.LedFx_host + "/api/scenes",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var apiObj struct {
		Scenes map[string]struct {
			Name string `json:"name"`
		} `json:"scenes"`
	}

	err = json.Unmarshal(body, &apiObj)
	if err != nil {
		log.Fatal(err)
	}

	tempScenes = tempScenes[:0]
	for k := range apiObj.Scenes {
		tempScenes = append(tempScenes, k)
	}

	slices.Sort(tempScenes)
}

func loadLedfxPlaylists() {
	var resp *http.Response
	resp, err := http.Get(
		configData.LedFx_host + "/api/playlists",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var apiObj struct {
		Playlists map[string]struct {
			Name string `json:"name"`
		} `json:"playlists"`
	}

	err = json.Unmarshal(body, &apiObj)
	if err != nil {
		log.Fatal(err)
	}

	tempPlaylists = tempPlaylists[:0]
	for k := range apiObj.Playlists {
		tempPlaylists = append(tempPlaylists, k)
	}

	slices.Sort(tempPlaylists)
}

func loadLedfxColors() {
	resp, err := http.Get(
		configData.LedFx_host + "/api/colors",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var apiObj struct {
		Colors struct {
			Builtin map[string]string `json:"builtin"`
			User    map[string]string `json:"user"`
		} `json:"colors"`
	}

	err = json.Unmarshal(body, &apiObj)
	if err != nil {
		log.Fatal(err)
	}

	tempColors = tempColors[:0]
	if configData.ColorHex == nil {
		configData.ColorHex = map[string]string{}
	}

	for name, hex := range apiObj.Colors.Builtin {
		if name != "black" { // skip black, it's used as "OFF"
			tempColors = append(tempColors, name)
			configData.ColorHex[name] = hex
		}
	}
	for name, hex := range apiObj.Colors.User {
		tempColors = append(tempColors, name)
		configData.ColorHex[name] = hex
	}

	slices.Sort(tempColors)
}

func prettifyName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
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

func buildCapabilities(items []string, label string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  <Capability Min=\"0\" Max=\"0\">%s OFF</Capability>\n", label))
	for i, item := range items {
		name := html.EscapeString(prettifyName(item))
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">%s</Capability>\n", i+1, i+1, name))
	}
	if len(items) < 255 {
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"255\">No function</Capability>\n", len(items)+1))
	}
	return b.String()
}

func generateQXF() string {
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

	// Scene Select channel
	b.WriteString(" <Channel Name=\"Scene Select\">\n")
	b.WriteString("  <Group Byte=\"0\">Effect</Group>\n")
	b.WriteString(buildCapabilities(configData.Scenes, "Scene"))
	b.WriteString(" </Channel>\n")

	// Playlist Select channel
	b.WriteString(" <Channel Name=\"Playlist Select\">\n")
	b.WriteString("  <Group Byte=\"0\">Effect</Group>\n")
	b.WriteString(buildCapabilities(configData.Playlists, "Playlist"))
	b.WriteString(" </Channel>\n")

	// Brightness channel (uses preset for proper dimmer icon)
	b.WriteString(" <Channel Name=\"Brightness\" Preset=\"IntensityMasterDimmer\"/>\n")

	// Blackout channel
	b.WriteString(` <Channel Name="Blackout">
  <Group Byte="0">Shutter</Group>
  <Capability Min="0" Max="127" Preset="ShutterOpen">Normal</Capability>
  <Capability Min="128" Max="255" Preset="ShutterClose">Blackout</Capability>
 </Channel>
`)

	// Color Override channel with ColorMacro presets for color swatches
	b.WriteString(" <Channel Name=\"Color Override\">\n")
	b.WriteString("  <Group Byte=\"0\">Colour</Group>\n")
	b.WriteString("  <Capability Min=\"0\" Max=\"0\">Color OFF</Capability>\n")
	for i, color := range configData.Colors {
		name := html.EscapeString(prettifyName(color))
		hex := configData.ColorHex[color]
		if hex != "" {
			b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\" Preset=\"ColorMacro\" Res1=\"%s\">%s</Capability>\n",
				i+1, i+1, hex, name))
		} else {
			b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"%d\">%s</Capability>\n",
				i+1, i+1, name))
		}
	}
	if len(configData.Colors) < 255 {
		b.WriteString(fmt.Sprintf("  <Capability Min=\"%d\" Max=\"255\">No function</Capability>\n", len(configData.Colors)+1))
	}
	b.WriteString(" </Channel>\n")

	// Mode
	b.WriteString(" <Mode Name=\"5 Channel\">\n")
	b.WriteString("  <Channel Number=\"0\">Scene Select</Channel>\n")
	b.WriteString("  <Channel Number=\"1\">Playlist Select</Channel>\n")
	b.WriteString("  <Channel Number=\"2\">Brightness</Channel>\n")
	b.WriteString("  <Channel Number=\"3\">Blackout</Channel>\n")
	b.WriteString("  <Channel Number=\"4\">Color Override</Channel>\n")
	b.WriteString(" </Mode>\n")

	// Physical
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

func exportFixture() (string, string) {
	qxf := generateQXF()
	filename := "LedFx-sACN-LedFx-Bridge.qxf"

	// Try QLC+ user fixture directory first
	dir := qlcFixtureDir()
	if dir != "" {
		if _, err := os.Stat(dir); err == nil {
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(qxf), 0644); err == nil {
				return path, fmt.Sprintf("Exported to %s — Restart QLC+ to load fixture", path)
			}
		}
		// Directory doesn't exist, try to create it
		if err := os.MkdirAll(dir, 0755); err == nil {
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(qxf), 0644); err == nil {
				return path, fmt.Sprintf("Exported to %s — Restart QLC+ to load fixture", path)
			}
		}
	}

	// Fallback to current working directory
	path := filename
	if err := os.WriteFile(path, []byte(qxf), 0644); err != nil {
		return "", fmt.Sprintf("Export failed: %v", err)
	}
	return path, fmt.Sprintf("Exported to ./%s — Copy to QLC+ Fixtures folder, then restart QLC+", path)
}

type Styles struct {
	colorText      lipgloss.Color
	colorSelected  lipgloss.Color
	colorError     lipgloss.Color
	colorOK        lipgloss.Color
	colorHighlight lipgloss.Color
	Border         lipgloss.Style
	Header         lipgloss.Style
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.colorText = lipgloss.Color("7")
	s.colorError = lipgloss.Color("1")
	s.colorSelected = lipgloss.Color("8")
	s.colorOK = lipgloss.Color("2")
	s.colorHighlight = lipgloss.Color("202")
	s.Border = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	s.Header = lipgloss.NewStyle().Foreground(s.colorHighlight)
	return s
}

func colStyle(col lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(col)
}

type model struct {
	styles       *Styles
	width        int
	height       int
	cursor       int
	subCursor    int
	settingItems []string
	textInput    textinput.Model
	spinner      spinner.Model
	recieving    bool
	changed      bool
	exportMsg    string
}

var urlRegex = regexp.MustCompile(`(?m)^(?P<protocol>https?):\/\/(?P<host>(?:(?:[a-z0-9\-_]+\b)\.)*\w+\b)(?:\:(?P<port>\d{1,5}))?(?P<path>\/[\/\d\w\.-]*)*(?:\?(?P<query>[^#/]+))?(?:#(?P<fragment>.+))?$`)

func textInputValidatorGen(cursorPos int) textinput.ValidateFunc {
	if cursorPos == settUniverse {
		return textinput.ValidateFunc(func(str string) error {
			i, err := strconv.ParseUint(str, 10, 16)
			if err != nil {
				return err
			}
			if 1 > i || i > 65279 {
				return fmt.Errorf("input out of Range")
			}
			return nil
		})
	} else if cursorPos >= settSceneCh && cursorPos <= settColorCh {
		return textinput.ValidateFunc(func(str string) error {
			i, err := strconv.ParseUint(str, 10, 16)
			if err != nil {
				return err
			}
			if i > 512 {
				return fmt.Errorf("input out of Range (0-512, 0=disabled)")
			}
			return nil
		})
	} else if cursorPos == settHost {
		return textinput.ValidateFunc(func(str string) error {
			if urlRegex.FindString(str) != "" {
				return nil
			}
			return errors.New("url invalid")
		})
	}
	return nil
}

func initialModel() model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Spinner.FPS = time.Second / 4

	return model{
		styles: DefaultStyles(),
		settingItems: []string{
			"Universe", "Scene Channel", "Playlist Channel",
			"Brightness Channel", "Blackout Channel", "Color Channel",
			"LedFx Host", "Scenes", "Playlists", "Colors", "[Save]",
		},
		textInput: ti,
		spinner:   sp,
		recieving: false,
		changed:   false,
		subCursor: -1,
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

type updateStatusMsg struct{}
type recievingMsg uint
type timeOutMsg uint
type errMsg error

func isTextInputSetting(cursor int) bool {
	return cursor >= settUniverse && cursor <= settHost
}

func isSubMenuSetting(cursor int) bool {
	return cursor == settScenes || cursor == settPlaylists || cursor == settColors
}

func activeSubList(cursor int) []string {
	switch cursor {
	case settScenes:
		return tempScenes
	case settPlaylists:
		return tempPlaylists
	case settColors:
		return tempColors
	default:
		return nil
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.textInput.Focused() && m.subCursor < 0 {
				return m, tea.Quit
			}

		case "e":
			if !m.textInput.Focused() && m.subCursor < 0 {
				_, msg := exportFixture()
				m.exportMsg = msg
			}

		case "up", "k", "w":
			if m.cursor > 0 && !m.textInput.Focused() && m.subCursor < 0 {
				m.cursor--
			} else if m.subCursor > 0 {
				m.subCursor--
			} else if isTextInputSetting(m.cursor) && m.cursor != settHost && m.textInput.Focused() {
				i, err := strconv.ParseUint(m.textInput.Value(), 10, 16)
				if err == nil {
					m.textInput.SetValue(fmt.Sprint(i + 1))
				}
			}

		case "down", "j", "s":
			if m.cursor < len(m.settingItems)-1 && !m.textInput.Focused() && m.subCursor < 0 {
				m.cursor++
			} else if m.subCursor >= 0 && m.subCursor < len(activeSubList(m.cursor)) {
				m.subCursor++
			} else if isTextInputSetting(m.cursor) && m.cursor != settHost && m.textInput.Focused() {
				i, err := strconv.ParseUint(m.textInput.Value(), 10, 16)
				if err == nil && i > 0 {
					m.textInput.SetValue(fmt.Sprint(i - 1))
				}
			}

		case "enter", " ":
			if isTextInputSetting(m.cursor) && !m.textInput.Focused() {
				m.textInput.Validate = textInputValidatorGen(m.cursor)
				m.textInput.Focus()
				m.textInput.SetValue(configValueFromIndex(m.cursor))
			} else if isSubMenuSetting(m.cursor) && m.subCursor < 0 {
				m.subCursor = 0
				if m.cursor == settScenes {
					tempScenes = configData.Scenes[:]
				} else if m.cursor == settPlaylists {
					tempPlaylists = configData.Playlists[:]
				} else {
					tempColors = configData.Colors[:]
				}
			} else if isSubMenuSetting(m.cursor) && m.subCursor == 0 {
				if m.cursor == settScenes {
					loadLedfxScenes()
				} else if m.cursor == settPlaylists {
					loadLedfxPlaylists()
				} else {
					loadLedfxColors()
				}
			} else if msg.String() == "enter" && m.textInput.Focused() && m.textInput.Err == nil {
				value := m.textInput.Value()
				switch m.cursor {
				case settUniverse:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Universe != i {
							m.changed = true
						}
						configData.Universe = i
					}
				case settSceneCh:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Channels.Scene != i {
							m.changed = true
						}
						configData.Channels.Scene = i
					}
				case settPlaylistCh:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Channels.Playlist != i {
							m.changed = true
						}
						configData.Channels.Playlist = i
					}
				case settBrightnessCh:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Channels.Brightness != i {
							m.changed = true
						}
						configData.Channels.Brightness = i
					}
				case settBlackoutCh:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Channels.Blackout != i {
							m.changed = true
						}
						configData.Channels.Blackout = i
					}
				case settColorCh:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if configData.Channels.Color != i {
							m.changed = true
						}
						configData.Channels.Color = i
					}
				case settHost:
					if configData.LedFx_host != value {
						m.changed = true
					}
					configData.LedFx_host = value
				}

				m.textInput.Blur()
			} else if m.cursor == settSave && m.changed {
				out, err := json.MarshalIndent(configData, "", "  ")
				if err != nil {
					log.Fatal(err)
				}

				err = os.WriteFile(configFile, out, fs.ModeDevice)
				if err != nil {
					log.Fatalf("Failed to write file: %v\n", err)
				}

				m.changed = false
				configFromFile = true
			}
		case "pgup":
			if isSubMenuSetting(m.cursor) && m.subCursor > 1 {
				list := activeSubList(m.cursor)
				list[m.subCursor-2], list[m.subCursor-1] =
					list[m.subCursor-1], list[m.subCursor-2]
				m.subCursor--
			}
		case "pgdown":
			if isSubMenuSetting(m.cursor) && m.subCursor > 0 && m.subCursor < len(activeSubList(m.cursor)) {
				list := activeSubList(m.cursor)
				list[m.subCursor-1], list[m.subCursor-0] =
					list[m.subCursor-0], list[m.subCursor-1]
				m.subCursor++
			}
		case "ctrl+s":
			if isSubMenuSetting(m.cursor) && m.subCursor >= 0 {
				if m.cursor == settScenes {
					configData.Scenes = tempScenes
				} else if m.cursor == settPlaylists {
					configData.Playlists = tempPlaylists
				} else {
					configData.Colors = tempColors
				}
				m.subCursor = -1
				m.changed = true
			}
		case "esc":
			m.textInput.Blur()
			m.subCursor = -1

		case "tab":
			m.textInput.Reset()
		}

	case updateStatusMsg:

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.recieving {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	case recievingMsg:
		m.recieving = true
		m.spinner.Style = colStyle(m.styles.colorOK)

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(m.spinner.Tick())

		return m, cmd

	case timeOutMsg:
		m.recieving = false
		m.spinner.Style = colStyle(m.styles.colorError)

	case errMsg:
		// ToDo: Handle this
		return m, nil
	}

	if m.textInput.Err != nil {
		m.textInput.TextStyle = colStyle(m.styles.colorError)
	} else {
		m.textInput.TextStyle = colStyle(m.styles.colorSelected)
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func configValueFromIndex(index int) string {
	switch index {
	case settUniverse:
		return fmt.Sprintf("%d", configData.Universe)
	case settSceneCh:
		return fmt.Sprintf("%d", configData.Channels.Scene)
	case settPlaylistCh:
		return fmt.Sprintf("%d", configData.Channels.Playlist)
	case settBrightnessCh:
		return fmt.Sprintf("%d", configData.Channels.Brightness)
	case settBlackoutCh:
		return fmt.Sprintf("%d", configData.Channels.Blackout)
	case settColorCh:
		return fmt.Sprintf("%d", configData.Channels.Color)
	case settHost:
		return configData.LedFx_host
	case settScenes:
		return fmt.Sprintf("%d Scenes", len(configData.Scenes))
	case settPlaylists:
		return fmt.Sprintf("%d Playlists", len(configData.Playlists))
	case settColors:
		return fmt.Sprintf("%d Colors", len(configData.Colors))
	default:
		return ""
	}
}

func (m model) View() string {
	// The header
	title := `       ___  ______  __  __          ______       ___      _    __        
  ___ / _ |/ ___/ |/ / / /  ___ ___/ / __/_ __  / _ )____(_)__/ /__ ____ 
 (_-</ __ / /__/    / / /__/ -_) _  / _/ \ \ / / _  / __/ / _  / _ '/ -_)
/___/_/ |_\___/_/|_/ /____/\__/\_,_/_/  /_\_\ /____/_/ /_/\_,_/\_, /\__/ 
                                                              /___/      `

	header :=
		m.styles.Header.Render(
			lipgloss.Place(
				m.width-2,
				7,
				lipgloss.Center,
				lipgloss.Center,
				title,
			))

	// Status line
	blackoutStr := "OFF"
	if BlackoutActive {
		blackoutStr = "ON"
	}
	sceneInfo := fmt.Sprintf(" Scene: %s | Playlist: %s | Bri: %d%% | Blackout: %s | Color: %s",
		ActiveScene, ActivePlaylist, int(currentBrightness*100), blackoutStr, ActiveColor)

	recievingSpinner := m.spinner.View()

	settingsHeader := lipgloss.NewStyle().PaddingLeft(2).Bold(true).
		Foreground(m.styles.colorHighlight).
		Render("Settings:")
	if !configFromFile {
		settingsHeader += " (not saved)"
	} else if m.changed {
		settingsHeader += " (changed)"
	}

	settingsColumn := ""
	valueColumn := ""
	subMenuColumn := ""

	for i, setting := range m.settingItems {
		cursor := " " // no cursor
		value := "  " + configValueFromIndex(i)
		lineStyle := colStyle(m.styles.colorText)
		if m.cursor == i {
			lineStyle = colStyle(m.styles.colorSelected)
			if !m.textInput.Focused() {
				if m.subCursor < 0 {
					cursor = ">" // cursor!
				}
			} else {
				value = m.textInput.View()
			}
		}

		settingsColumn += lineStyle.Render(fmt.Sprintf("%s %s", cursor, setting)) + "\n"
		valueColumn += value + "\n"
	}

	// Build submenu column for scenes or playlists
	if m.subCursor >= 0 && isSubMenuSetting(m.cursor) {
		var subList []string
		var subLabel string
		if m.cursor == settScenes {
			subList = tempScenes
			subLabel = "[get scenes from LedFx Api]"
		} else if m.cursor == settPlaylists {
			subList = tempPlaylists
			subLabel = "[get playlists from LedFx Api]"
		} else {
			subList = tempColors
			subLabel = "[get colors from LedFx Api]"
		}

		lineStyle := colStyle(m.styles.colorText)
		if m.subCursor == 0 {
			subMenuColumn += ">"
			lineStyle = colStyle(m.styles.colorSelected)
		} else {
			subMenuColumn += " "
		}
		subMenuColumn = lineStyle.Render(subMenuColumn+" "+subLabel) + "\n"

		for i, item := range subList {
			cursor := " "
			lineStyle := colStyle(m.styles.colorText)
			if m.subCursor-1 == i {
				cursor = ">"
				lineStyle = colStyle(m.styles.colorSelected)
			}
			subMenuColumn += lineStyle.Render(fmt.Sprintf("%s %d. %s", cursor, i+1, item)) + "\n"
		}
	}

	pad := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2)

	settingsBlock := settingsHeader + "\n\n"
	if m.subCursor < 0 || !isSubMenuSetting(m.cursor) {
		settingsBlock += lipgloss.JoinHorizontal(lipgloss.Top, pad.Render(settingsColumn), valueColumn)
	} else {
		settingsBlock += lipgloss.JoinHorizontal(lipgloss.Top, pad.Render(settingsColumn), subMenuColumn)
	}

	// The footer
	footer := " Press q to quit, e to export QLC+ fixture."
	if m.textInput.Focused() {
		footer = " Press esc to abort edit or enter to submit."
	} else if m.subCursor >= 0 {
		footer = " Press esc to abort, PgUp or PgDn to reorder or ctrl+S to Save"
	} else if m.exportMsg != "" {
		connStatus := "Waiting for sACN…"
		if m.recieving {
			connStatus = "Connected ✓"
		}
		footer = fmt.Sprintf(" %s | %s", m.exportMsg, connStatus)
	}

	// Send the UI for rendering

	return lipgloss.PlaceVertical(m.height, 0,
		table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(m.styles.Border).
			BorderRow(true).
			Row(header).
			Row(lipgloss.JoinHorizontal(lipgloss.Center, " ", recievingSpinner, "   ", sceneInfo)).
			Row(settingsBlock).
			Row(footer).
			Render())
}
