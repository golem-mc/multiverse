// Package core is Multiverse-Core's foundation, ported to golem: config.yml,
// worlds.yml and the world manager, messages, permissions, the command
// framework the /mv commands are written against, and the API the modules
// (NetherPortals, Portals, Inventories) use. Everything here runs on the
// tick goroutine unless a comment says otherwise.
package core

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// Version is the Multiverse-Core release this port follows.
const Version = "5.8.2"

// Authors are Multiverse-Core's authors, as its plugin.yml lists them.
var Authors = []string{"dumptruckman", "Rigby", "fernferret", "lithium3141", "main--", "benwoo1110", "Zax71"}

// Core is the running Multiverse-Core. MV: MultiverseCore, MultiverseCoreApi
type Core struct {
	Server       plugin.Server
	Log          *plugin.Logger
	Config       *Config
	Worlds       *Worlds
	Locales      *Locales
	Commands     *Commands
	Permissions  *Permissions
	Destinations *Destinations
	Anchors      *Anchors

	// teleportQueue names who asked for each player's Multiverse teleport
	// while it runs, for the teleport intercept. MV: TeleportQueue
	teleportQueue map[string]plugin.CommandSender

	levelName string
	reloads   []func()
	disables  []func()
}

var (
	instMu   sync.Mutex
	instance *Core
)

// Get answers the running Multiverse-Core, for the modules; false before it
// is enabled or after it is disabled. MV: MultiverseCoreApi.get
func Get() (*Core, bool) {
	instMu.Lock()
	defer instMu.Unlock()
	return instance, instance != nil
}

// events are Multiverse's own events, defined so any plugin can listen.
var events = []any{
	(*MVWorldLoadedEvent)(nil), (*MVWorldUnloadedEvent)(nil), (*MVWorldCreatedEvent)(nil),
	(*MVWorldImportedEvent)(nil), (*MVWorldRemovedEvent)(nil), (*MVWorldDeleteEvent)(nil),
	(*MVWorldClonedEvent)(nil), (*MVWorldRegeneratedEvent)(nil), (*MVWorldPropertyChangedEvent)(nil),
	(*MVConfigReloadEvent)(nil), (*MVDebugModeEvent)(nil), (*MVDumpsDebugInfoEvent)(nil),
	(*MVTeleportDestinationEvent)(nil),
}

// Enable loads config.yml and worlds.yml, loads the worlds and registers
// the permissions; the commands and listeners register after it.
// MV: MultiverseCore.onEnable
func Enable(s plugin.Server) (*Core, error) {
	c := &Core{Server: s, Log: s.Logger()}
	if err := os.MkdirAll(s.DataDir(), 0o755); err != nil {
		return nil, err
	}
	for _, e := range events {
		s.Events().Define(reflect.TypeOf(e))
	}
	c.Locales = newLocales()
	c.Config = newConfig(s.DataDir())
	if err := c.Config.Load(); err != nil {
		return nil, err
	}
	c.applyConfig()
	c.Permissions = newPermissions(c)
	c.Destinations = newDestinations(c)
	c.Commands = newCommands(c)
	c.Worlds = newWorlds(c, s.DataDir())
	c.listenWorlds()
	if err := c.Worlds.Init(); err != nil {
		return nil, err
	}
	c.teleportQueue = map[string]plugin.CommandSender{}
	c.Anchors = newAnchors(c)
	if err := c.Anchors.Load(); err != nil {
		c.Log.Error("Failed to load anchors", "error", err)
	}
	c.listenPlayers()
	instMu.Lock()
	instance = c
	instMu.Unlock()
	return c, nil
}

// applyConfig hands config.yml's messaging settings to the locales.
func (c *Core) applyConfig() {
	c.Locales.def = c.Config.String("messaging.default-locale")
	if c.Locales.bundles[c.Locales.def] == nil {
		c.Locales.def = "en"
	}
	c.Locales.perPlayer = c.Config.Bool("messaging.per-player-locale")
}

// Disable saves worlds.yml and forgets the instance.
func (c *Core) Disable() {
	for _, fn := range c.disables {
		fn()
	}
	if err := c.Worlds.Save(); err != nil {
		c.Log.Error("Failed to save worlds.yml", "error", err)
	}
	instMu.Lock()
	if instance == c {
		instance = nil
	}
	instMu.Unlock()
}

// OnReload registers fn to run on /mv reload, after config.yml and
// worlds.yml are read again: a module reloads its own files there.
func (c *Core) OnReload(fn func()) { c.reloads = append(c.reloads, fn) }

// OnDisable registers fn to run when Multiverse-Core disables.
func (c *Core) OnDisable(fn func()) { c.disables = append(c.disables, fn) }

// Reload reads config.yml and worlds.yml again, runs the modules' reloads
// and answers the files reloaded. MV: ReloadCommand
func (c *Core) Reload() ([]string, error) {
	if err := c.Config.Load(); err != nil {
		return nil, err
	}
	c.applyConfig()
	if err := c.Worlds.Init(); err != nil {
		return nil, err
	}
	if err := c.Anchors.Load(); err != nil {
		return nil, err
	}
	for _, fn := range c.reloads {
		fn()
	}
	e := &MVConfigReloadEvent{Configs: []string{"Multiverse-Core - config.yml", "Multiverse-Core - worlds.yml",
		"Multiverse-Core - anchors.yml"}}
	c.fire(e)
	return e.Configs, nil
}

// fire runs e through the plugins' listeners.
func (c *Core) fire(e plugin.Event) { c.Server.Events().Fire(e) }

// Fire runs one of Multiverse's events, or a module's, through the
// listeners.
func (c *Core) Fire(e plugin.Event) { c.fire(e) }

func (c *Core) firePropertyChanged(w *World, name string, old, new any) {
	if c.Worlds == nil || !slices.Contains(c.Worlds.list, w) {
		return
	}
	c.fire(&MVWorldPropertyChangedEvent{World: w, Name: name, Old: old, New: new})
}

// Debug logs a debug message when global-debug is at least level.
// MV: Logging.fine/finer/finest
func (c *Core) Debug(level int, format string, args ...any) {
	if c.Config.Int("misc.global-debug") >= level {
		c.Log.Info(fmt.Sprintf("[MV-Debug] "+format, args...))
	}
}
