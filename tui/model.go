package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/8-Lambda-8/sACN_ledfx_bridge/bridge"
	"github.com/8-Lambda-8/sACN_ledfx_bridge/ledfx"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Setting indices for TUI navigation
const (
	settUniverse = iota
	settHost
	settScenes
	settPlaylists
	settEffects
	settPalette
	settSave
)

// Messages for bubbletea
type UpdateStatusMsg struct{}
type ReceivingMsg uint
type TimeOutMsg uint
type errMsg error

type styles struct {
	colorText      lipgloss.Color
	colorSelected  lipgloss.Color
	colorError     lipgloss.Color
	colorOK        lipgloss.Color
	colorHighlight lipgloss.Color
	Border         lipgloss.Style
	Header         lipgloss.Style
}

func defaultStyles() *styles {
	s := new(styles)
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

// Model is the bubbletea model for the TUI.
type Model struct {
	Config     *bridge.Config
	State      *bridge.State
	Client     ledfx.Client
	ConfigFile string
	ConfigFromFile bool

	st           *styles
	width        int
	height       int
	cursor       int
	subCursor    int
	settingItems []string
	textInput    textinput.Model
	spinner      spinner.Model
	receiving    bool
	changed      bool
	exportMsg    string

	tempScenes    []string
	tempPlaylists []string
	tempEffects   []string
	tempPalette   []string
}

// NewModel creates a TUI Model.
func NewModel(cfg *bridge.Config, state *bridge.State, client ledfx.Client, configFile string, configFromFile bool) Model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Spinner.FPS = time.Second / 4

	return Model{
		Config:     cfg,
		State:      state,
		Client:     client,
		ConfigFile: configFile,
		ConfigFromFile: configFromFile,
		st:         defaultStyles(),
		settingItems: []string{
			"Universe",
			"LedFx Host", "Scenes", "Playlists", "Effects", "Colors & Gradients", "[Save]",
		},
		textInput: ti,
		spinner:   sp,
		receiving: false,
		changed:   false,
		subCursor: -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
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

func isTextInputSetting(cursor int) bool {
	return cursor >= settUniverse && cursor <= settHost
}

func isSubMenuSetting(cursor int) bool {
	return cursor == settScenes || cursor == settPlaylists || cursor == settEffects || cursor == settPalette
}

func (m *Model) activeSubList() []string {
	switch m.cursor {
	case settScenes:
		return m.tempScenes
	case settPlaylists:
		return m.tempPlaylists
	case settEffects:
		return m.tempEffects
	case settPalette:
		return m.tempPalette
	default:
		return nil
	}
}

func (m *Model) configValueFromIndex(index int) string {
	switch index {
	case settUniverse:
		return fmt.Sprintf("%d", m.Config.Universe)
	case settHost:
		return m.Config.LedFx_host
	case settScenes:
		return fmt.Sprintf("%d Scenes", len(m.Config.Scenes))
	case settPlaylists:
		return fmt.Sprintf("%d Playlists", len(m.Config.Playlists))
	case settEffects:
		return fmt.Sprintf("%d Effects", len(m.Config.Effects))
	case settPalette:
		return fmt.Sprintf("%d Colors + %d Gradients", len(m.Config.Colors), len(m.Config.Gradients))
	default:
		return ""
	}
}

func (m *Model) loadPalette() {
	m.tempPalette = m.tempPalette[:0]
	if colors, err := m.Client.LoadColors(); err == nil {
		if m.Config.ColorHex == nil {
			m.Config.ColorHex = map[string]string{}
		}
		var names []string
		for name, hex := range colors {
			names = append(names, name)
			m.Config.ColorHex[name] = hex
		}
		slices.Sort(names)
		for _, name := range names {
			m.tempPalette = append(m.tempPalette, "color:"+name)
		}
	}
	if gradients, err := m.Client.LoadGradients(); err == nil {
		for _, name := range gradients {
			m.tempPalette = append(m.tempPalette, "gradient:"+name)
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				_, exportMsg := ExportFixture(m.Config)
				m.exportMsg = exportMsg
			}

		case "r":
			if !m.textInput.Focused() && m.subCursor < 0 {
				// Refresh: load everything from LedFx API and save config
				if scenes, err := m.Client.LoadScenes(); err == nil {
					m.Config.Scenes = scenes
				}
				if playlists, err := m.Client.LoadPlaylists(); err == nil {
					m.Config.Playlists = playlists
				}
				if effects, err := m.Client.LoadEffects(); err == nil {
					m.Config.Effects = effects
				}
				if colors, err := m.Client.LoadColors(); err == nil {
					if m.Config.ColorHex == nil {
						m.Config.ColorHex = map[string]string{}
					}
					m.Config.Colors = m.Config.Colors[:0]
					for name, hex := range colors {
						m.Config.Colors = append(m.Config.Colors, name)
						m.Config.ColorHex[name] = hex
					}
					slices.Sort(m.Config.Colors)
				}
				if gradients, err := m.Client.LoadGradients(); err == nil {
					m.Config.Gradients = gradients
				}
				// Save
				out, err := json.MarshalIndent(m.Config, "", "  ")
				if err == nil {
					if err := os.WriteFile(m.ConfigFile, out, fs.ModeDevice); err == nil {
						m.ConfigFromFile = true
						m.changed = false
						m.exportMsg = fmt.Sprintf("Refreshed: %d scenes, %d playlists, %d effects, %d colors, %d gradients — saved",
							len(m.Config.Scenes), len(m.Config.Playlists), len(m.Config.Effects),
							len(m.Config.Colors), len(m.Config.Gradients))
					}
				}
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
			} else if m.subCursor >= 0 && m.subCursor < len(m.activeSubList()) {
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
				m.textInput.SetValue(m.configValueFromIndex(m.cursor))
			} else if isSubMenuSetting(m.cursor) && m.subCursor < 0 {
				m.subCursor = 0
				switch m.cursor {
				case settScenes:
					m.tempScenes = m.Config.Scenes[:]
				case settPlaylists:
					m.tempPlaylists = m.Config.Playlists[:]
				case settEffects:
					m.tempEffects = m.Config.Effects[:]
				case settPalette:
					m.tempPalette = m.tempPalette[:0]
					for _, c := range m.Config.Colors {
						m.tempPalette = append(m.tempPalette, "color:"+c)
					}
					for _, g := range m.Config.Gradients {
						m.tempPalette = append(m.tempPalette, "gradient:"+g)
					}
				}
			} else if isSubMenuSetting(m.cursor) && m.subCursor == 0 {
				switch m.cursor {
				case settScenes:
					if scenes, err := m.Client.LoadScenes(); err == nil {
						m.tempScenes = scenes
					}
				case settPlaylists:
					if playlists, err := m.Client.LoadPlaylists(); err == nil {
						m.tempPlaylists = playlists
					}
				case settEffects:
					if effects, err := m.Client.LoadEffects(); err == nil {
						m.tempEffects = effects
					}
				case settPalette:
					m.loadPalette()
				}
			} else if msg.String() == "enter" && m.textInput.Focused() && m.textInput.Err == nil {
				value := m.textInput.Value()
				switch m.cursor {
				case settUniverse:
					i, err := strconv.ParseUint(value, 10, 16)
					if err == nil {
						if m.Config.Universe != i {
							m.changed = true
						}
						m.Config.Universe = i
					}
				case settHost:
					if m.Config.LedFx_host != value {
						m.changed = true
					}
					m.Config.LedFx_host = value
				}
				m.textInput.Blur()
			} else if m.cursor == settSave && m.changed {
				out, err := json.MarshalIndent(m.Config, "", "  ")
				if err != nil {
					log.Fatal(err)
				}
				err = os.WriteFile(m.ConfigFile, out, fs.ModeDevice)
				if err != nil {
					log.Fatalf("Failed to write file: %v\n", err)
				}
				m.changed = false
				m.ConfigFromFile = true
			}
		case "pgup":
			if isSubMenuSetting(m.cursor) && m.subCursor > 1 {
				list := m.activeSubList()
				list[m.subCursor-2], list[m.subCursor-1] =
					list[m.subCursor-1], list[m.subCursor-2]
				m.subCursor--
			}
		case "pgdown":
			if isSubMenuSetting(m.cursor) && m.subCursor > 0 && m.subCursor < len(m.activeSubList()) {
				list := m.activeSubList()
				list[m.subCursor-1], list[m.subCursor-0] =
					list[m.subCursor-0], list[m.subCursor-1]
				m.subCursor++
			}
		case "ctrl+s":
			if isSubMenuSetting(m.cursor) && m.subCursor >= 0 {
				switch m.cursor {
				case settScenes:
					m.Config.Scenes = m.tempScenes
				case settPlaylists:
					m.Config.Playlists = m.tempPlaylists
				case settEffects:
					m.Config.Effects = m.tempEffects
				case settPalette:
					m.Config.Colors = m.Config.Colors[:0]
					m.Config.Gradients = m.Config.Gradients[:0]
					for _, item := range m.tempPalette {
						if strings.HasPrefix(item, "color:") {
							m.Config.Colors = append(m.Config.Colors, strings.TrimPrefix(item, "color:"))
						} else if strings.HasPrefix(item, "gradient:") {
							m.Config.Gradients = append(m.Config.Gradients, strings.TrimPrefix(item, "gradient:"))
						}
					}
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

	case UpdateStatusMsg:

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.receiving {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	case ReceivingMsg:
		m.receiving = true
		m.spinner.Style = colStyle(m.st.colorOK)
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(m.spinner.Tick())
		return m, cmd

	case TimeOutMsg:
		m.receiving = false
		m.spinner.Style = colStyle(m.st.colorError)

	case errMsg:
		return m, nil
	}

	if m.textInput.Err != nil {
		m.textInput.TextStyle = colStyle(m.st.colorError)
	} else {
		m.textInput.TextStyle = colStyle(m.st.colorSelected)
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	title := `       ___  ______  __  __          ______       ___      _    __        
  ___ / _ |/ ___/ |/ / / /  ___ ___/ / __/_ __  / _ )____(_)__/ /__ ____ 
 (_-</ __ / /__/    / / /__/ -_) _  / _/ \ \ / / _  / __/ / _  / _ '/ -_)
/___/_/ |_\___/_/|_/ /____/\__/\_,_/_/  /_\_\ /____/_/ /_/\_,_/\_, /\__/ 
                                                              /___/      `

	header :=
		m.st.Header.Render(
			lipgloss.Place(
				m.width-2,
				7,
				lipgloss.Center,
				lipgloss.Center,
				title,
			))

	s := m.State
	blackoutStr := "OFF"
	if s.BlackoutActive {
		blackoutStr = "ON"
	}
	strobeStr := ""
	if s.StrobeActive {
		strobeStr = " | STROBE"
	}
	freezeStr := ""
	if s.FreezeActive {
		freezeStr = " | FROZEN"
	}
	duckStr := ""
	if s.CurrentDuck > 0 {
		duckStr = fmt.Sprintf(" | Duck: %d%%", int(s.CurrentDuck*100))
	}
	paletteStr := ""
	if s.ActivePalette != "OFF" {
		paletteStr = fmt.Sprintf(" | Pal: %s", s.ActivePalette)
	}
	effectStr := ""
	if s.ActiveEffect != "OFF" {
		effectStr = fmt.Sprintf(" | Fx: %s", s.ActiveEffect)
	}
	rgbStr := ""
	if s.RGBActive {
		rgbStr = fmt.Sprintf(" | RGB: #%02x%02x%02x", s.RedVal, s.GreenVal, s.BlueVal)
	}
	sceneInfo := fmt.Sprintf(" Scene: %s | Playlist: %s | Bri: %d%% | BO: %s%s%s%s%s%s%s",
		s.ActiveScene, s.ActivePlaylist, int(s.CurrentBrightness*100), blackoutStr,
		duckStr, paletteStr, rgbStr, effectStr, strobeStr, freezeStr)

	receivingSpinner := m.spinner.View()

	settingsHeader := lipgloss.NewStyle().PaddingLeft(2).Bold(true).
		Foreground(m.st.colorHighlight).
		Render("Settings:")
	if !m.ConfigFromFile {
		settingsHeader += " (not saved)"
	} else if m.changed {
		settingsHeader += " (changed)"
	}

	settingsColumn := ""
	valueColumn := ""
	subMenuColumn := ""

	for i, setting := range m.settingItems {
		cursor := " "
		value := "  " + m.configValueFromIndex(i)
		lineStyle := colStyle(m.st.colorText)
		if m.cursor == i {
			lineStyle = colStyle(m.st.colorSelected)
			if !m.textInput.Focused() {
				if m.subCursor < 0 {
					cursor = ">"
				}
			} else {
				value = m.textInput.View()
			}
		}
		settingsColumn += lineStyle.Render(fmt.Sprintf("%s %s", cursor, setting)) + "\n"
		valueColumn += value + "\n"
	}

	if m.subCursor >= 0 && isSubMenuSetting(m.cursor) {
		var subList []string
		var subLabel string
		switch m.cursor {
		case settScenes:
			subList = m.tempScenes
			subLabel = "[get scenes from LedFx Api]"
		case settPlaylists:
			subList = m.tempPlaylists
			subLabel = "[get playlists from LedFx Api]"
		case settEffects:
			subList = m.tempEffects
			subLabel = "[get effects from LedFx Api]"
		case settPalette:
			subList = m.tempPalette
			subLabel = "[get colors & gradients from LedFx Api]"
		}

		// "Load from API" button
		lineStyle := colStyle(m.st.colorText)
		if m.subCursor == 0 {
			subMenuColumn += ">"
			lineStyle = colStyle(m.st.colorSelected)
		} else {
			subMenuColumn += " "
		}
		subMenuColumn = lineStyle.Render(subMenuColumn+" "+subLabel) + "\n"

		// Windowed scroll: show items that fit in terminal
		maxVisible := m.height - 18 // chrome: header + status + settings + footer + padding
		if maxVisible < 5 {
			maxVisible = 5
		}
		total := len(subList)
		windowStart := 0
		windowEnd := total
		if total > maxVisible {
			// Center window around cursor (subCursor-1 is the item index)
			cursorIdx := m.subCursor - 1 // -1 because subCursor 0 is the "load" button
			if cursorIdx < 0 {
				cursorIdx = 0
			}
			half := maxVisible / 2
			windowStart = cursorIdx - half
			if windowStart < 0 {
				windowStart = 0
			}
			windowEnd = windowStart + maxVisible
			if windowEnd > total {
				windowEnd = total
				windowStart = windowEnd - maxVisible
				if windowStart < 0 {
					windowStart = 0
				}
			}
		}

		if windowStart > 0 {
			subMenuColumn += colStyle(m.st.colorText).Render(fmt.Sprintf("  ▲ %d more", windowStart)) + "\n"
		}

		for i := windowStart; i < windowEnd; i++ {
			item := subList[i]
			cursor := " "
			lineStyle := colStyle(m.st.colorText)
			if m.subCursor-1 == i {
				cursor = ">"
				lineStyle = colStyle(m.st.colorSelected)
			}
			displayName := item
			if m.cursor == settPalette {
				if strings.HasPrefix(item, "color:") {
					displayName = "● " + bridge.PrettifyName(strings.TrimPrefix(item, "color:"))
				} else if strings.HasPrefix(item, "gradient:") {
					displayName = "◆ " + bridge.PrettifyName(strings.TrimPrefix(item, "gradient:"))
				}
			}
			subMenuColumn += lineStyle.Render(fmt.Sprintf("%s %d. %s", cursor, i+1, displayName)) + "\n"
		}

		if windowEnd < total {
			subMenuColumn += colStyle(m.st.colorText).Render(fmt.Sprintf("  ▼ %d more", total-windowEnd)) + "\n"
		}
	}

	pad := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2)

	settingsBlock := settingsHeader + "\n\n"
	if m.subCursor < 0 || !isSubMenuSetting(m.cursor) {
		settingsBlock += lipgloss.JoinHorizontal(lipgloss.Top, pad.Render(settingsColumn), valueColumn)
	} else {
		settingsBlock += lipgloss.JoinHorizontal(lipgloss.Top, pad.Render(settingsColumn), subMenuColumn)
	}

	footer := " Press q to quit, e to export QLC+ fixture, r to refresh from LedFx."
	if m.textInput.Focused() {
		footer = " Press esc to abort edit or enter to submit."
	} else if m.subCursor >= 0 {
		footer = " Press esc to abort, PgUp or PgDn to reorder or ctrl+S to Save"
	} else if m.exportMsg != "" {
		connStatus := "Waiting for sACN…"
		if m.receiving {
			connStatus = "Connected ✓"
		}
		footer = fmt.Sprintf(" %s | %s", m.exportMsg, connStatus)
	}

	return lipgloss.PlaceVertical(m.height, 0,
		table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(m.st.Border).
			BorderRow(true).
			Row(header).
			Row(lipgloss.JoinHorizontal(lipgloss.Center, " ", receivingSpinner, "   ", sceneInfo)).
			Row(settingsBlock).
			Row(footer).
			Render())
}
