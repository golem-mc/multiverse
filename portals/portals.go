// Package portals is Multiverse-Portals ported to golem: the plugin
// registered as "multiverse-portals", which needs multiverse-core. It keeps
// portals.yml and config.yml in plugins/multiverse-portals, adds /mvp and
// the p: and rp: destinations.
package portals

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// Version is the Multiverse-Portals release this port follows.
const Version = "5.3.0"

//go:embed resources
var resources embed.FS

func init() {
	plugin.Register("multiverse-portals", &Plugin{}, plugin.Depends("multiverse-core"))
}

// Plugin is the golem plugin.
type Plugin struct{ p *Portals }

// Portals is the running Multiverse-Portals. MV: MultiversePortals,
// MultiversePortalsApi
type Portals struct {
	Core    *core.Core
	Server  plugin.Server
	Log     *plugin.Logger
	Config  *core.Config
	Manager *Manager
	Actions *Actions

	nodes    *core.Nodes
	sessions map[plugin.UUID]*Session
	// wandEnabled is /mvp wand's switch; the wand selects nothing while off.
	wandEnabled bool
}

var (
	instMu   sync.Mutex
	instance *Portals
)

// Get answers the running Multiverse-Portals; false before it is enabled or
// after it is disabled. MV: MultiversePortalsApi.get
func Get() (*Portals, bool) {
	instMu.Lock()
	defer instMu.Unlock()
	return instance, instance != nil
}

// Enable reads config.yml and portals.yml and registers /mvp, the
// destinations and the listeners. MV: MultiversePortals.onEnable
func (pl *Plugin) Enable(s plugin.Server) error {
	c, ok := core.Get()
	if !ok {
		return errors.New("multiverse-core is not enabled")
	}
	p := &Portals{Core: c, Server: s, Log: s.Logger(), sessions: map[plugin.UUID]*Session{}, wandEnabled: true}
	c.Locales.AddBundles(resources, "resources", "multiverse-portals")
	p.Config = core.NewConfigFile(filepath.Join(s.DataDir(), "config.yml"), configNodes(), migrateConfig, configVersion)
	if err := p.Config.Load(); err != nil {
		return err
	}
	p.nodes = p.portalNodes()
	p.Actions = newActions(p)
	p.Manager = newManager(p)
	c.Destinations.Register(&portalDestinationType{p})
	c.Destinations.Register(&randomPortalDestinationType{p})
	if err := p.Manager.Load(); err != nil {
		return err
	}
	c.Permissions.Register("multiverse.portal.select", core.DefaultOp)
	c.Commands.SetDynamicPermission(permSelect, func(sender plugin.CommandSender) bool {
		return sender.HasPermission("multiverse.portal.select") && sender.HasPermission("multiverse.portal.create")
	})
	if err := c.Commands.RegisterRoot("mvp", nil, p.commands()); err != nil {
		return err
	}
	p.listen()
	pl.p = p
	instMu.Lock()
	instance = p
	instMu.Unlock()
	p.Log.Info("Version " + Version + " Enabled - By Rigby, fernferret and benwoo1110")
	return nil
}

// Disable saves portals.yml.
func (pl *Plugin) Disable() {
	if pl.p == nil {
		return
	}
	pl.p.Manager.save()
	instMu.Lock()
	if instance == pl.p {
		instance = nil
	}
	instMu.Unlock()
}

// Reload reads portals.yml and config.yml again. MV: MultiversePortals.reloadConfigs
func (p *Portals) Reload() error {
	if err := p.Manager.Load(); err != nil {
		return err
	}
	return p.Config.Load()
}

// listen registers the listeners. MV: MVPCoreListener, MVPPlayerListener
func (p *Portals) listen() {
	s := p.Server
	plugin.Listen(s, func(e *core.MVConfigReloadEvent) {
		if err := p.Reload(); err != nil {
			p.Log.Error("Failed to reload Multiverse-Portals", "error", err)
		}
		e.Configs = append(e.Configs, "Multiverse-Portals - portals.yml", "Multiverse-Portals - config.yml")
	})
	plugin.Listen(s, func(e *core.MVDumpsDebugInfoEvent) {
		for _, name := range []string{"config.yml", "portals.yml"} {
			if data, err := os.ReadFile(filepath.Join(s.DataDir(), name)); err == nil {
				e.PutDetailedDebugInfo("multiverse-portals/"+name, string(data))
			}
		}
	})
	plugin.Listen(s, func(e *plugin.PlayerQuitEvent) { delete(p.sessions, e.Player.UUID()) })
	plugin.Listen(s, p.onInteract, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
}

// onInteract sets the wand's corners: a left click the first, a right
// click the second. MV: MVPPlayerListener.playerInteract
func (p *Portals) onInteract(e *plugin.PlayerInteractEvent) {
	if e.Block == nil || e.Hand != plugin.MainHand || !e.Player.HasPermission("multiverse.portal.create") ||
		e.Player.Inventory().Get(plugin.SlotMainHand).Item != p.wandItem() {
		return
	}
	w, _ := p.Core.Worlds.Of(e.Player.World())
	switch e.Action {
	case plugin.LeftClickBlock:
		e.SetCancelled(p.Session(e.Player).SetLeft(e.Block.Pos, w))
	case plugin.RightClickBlock:
		e.SetCancelled(p.Session(e.Player).SetRight(e.Block.Pos, w))
	}
}
