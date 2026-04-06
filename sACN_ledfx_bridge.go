package main

import (
"encoding/json"
"flag"
"io"
"log"
"os"

"github.com/Hundemeier/go-sacn/sacn"
tea "github.com/charmbracelet/bubbletea"
"github.com/mattn/go-isatty"

"github.com/8-Lambda-8/sACN_ledfx_bridge/bridge"
"github.com/8-Lambda-8/sACN_ledfx_bridge/ledfx"
"github.com/8-Lambda-8/sACN_ledfx_bridge/tui"
)

func main() {
var (
daemonMode bool
showHelp   bool
configFile string
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
opts = []tea.ProgramOption{tea.WithoutRenderer()}
} else {
log.SetOutput(io.Discard)
}

// Load config
cfg := bridge.DefaultConfig()
configFromFile := false
file, err := os.ReadFile(configFile)
if err == nil {
if err := json.Unmarshal(file, &cfg); err != nil {
log.Fatal(err)
}
configFromFile = true
}

// Create LedFx client
client := ledfx.NewHTTPClient(cfg.LedFx_host)

// Create bridge (needs tea.Program reference for state callbacks)
var p *tea.Program
b := bridge.New(cfg, client, func() {
if p != nil {
p.Send(tui.UpdateStatusMsg{})
}
})
defer b.Close()

// Set up sACN receiver
recv, err := sacn.NewReceiverSocket("", nil)
if err != nil {
log.Fatal(err)
}

recv.SetOnChangeCallback(func(old sacn.DataPacket, newD sacn.DataPacket) {
if newD.Universe() != uint16(b.Config.Universe) {
return
}
if p != nil {
p.Send(tui.ReceivingMsg(newD.Universe()))
}
b.HandleDMX(newD.Data())
})
recv.SetTimeoutCallback(func(univ uint16) {
if univ == uint16(b.Config.Universe) && p != nil {
p.Send(tui.TimeOutMsg(univ))
}
})
recv.Start()

// Create and run TUI
m := tui.NewModel(&b.Config, &b.State, client, configFile, configFromFile)
p = tea.NewProgram(m, opts...)
if _, err := p.Run(); err != nil {
log.Fatalf("Alas, there's been an error: %v", err)
os.Exit(1)
}
}
