// Package netherportals is Multiverse-NetherPortals ported to golem: the
// plugin "multiverse-netherportals", which sends each world's nether and end
// portals to the world links.yml or the world names link it to.
package netherportals

import (
	"embed"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"

	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/portals"
)

// Version is the Multiverse-NetherPortals release this port follows.
const Version = "5.1.0"

// Authors are Multiverse-NetherPortals' authors, as its plugin.yml lists them.
var Authors = []string{"Rigby", "fernferret", "benwoo1110"}

//go:embed resources
var resources embed.FS

// NetherPortals is the running Multiverse-NetherPortals.
// MV: MultiverseNetherPortals, MultiverseNetherPortalsApi
type NetherPortals struct {
	Server plugin.Server
	// Config is config.yml.
	Config *core.Config
	Links  *Links
	// CustomPortals lets other plugins take over portals they handle.
	CustomPortals *CustomPortals

	core *core.Core
	// touching records the players standing in a portal, by portal type, so
	// one is told why they cannot use it once. MV: MVEventRecord
	touching map[plugin.UUID]LinkType
	// bounced holds when a player was last bounced back, by name.
	bounced map[string]time.Time
	task    plugin.Task
	// legacyErr is the failed save of links.yml while config.yml migrated.
	legacyErr error
}

type module struct{ np *NetherPortals }

func init() {
	plugin.Register("multiverse-netherportals", &module{},
		plugin.Depends("multiverse-core"), plugin.SoftDepends("multiverse-portals"))
}

var (
	instMu    sync.Mutex
	instance  *NetherPortals
	whenReady []func(*NetherPortals)
)

// Get answers the running Multiverse-NetherPortals; false before it is
// enabled or after it is disabled. MV: MultiverseNetherPortalsApi.get
func Get() (*NetherPortals, bool) {
	instMu.Lock()
	defer instMu.Unlock()
	return instance, instance != nil
}

// WhenLoaded runs fn once Multiverse-NetherPortals is enabled, at once when
// it already is: a plugin enabled before it (Multiverse-Portals) registers
// its custom portal check here. MV: MultiverseNetherPortalsApi.whenLoaded
func WhenLoaded(fn func(*NetherPortals)) {
	instMu.Lock()
	np := instance
	if np == nil {
		whenReady = append(whenReady, fn)
	}
	instMu.Unlock()
	if np != nil {
		fn(np)
	}
}

// Enable reads config.yml and links.yml, then adds /mvnp and the portal
// listeners. MV: MultiverseNetherPortals.onEnable
func (m *module) Enable(s plugin.Server) error {
	c, ok := core.Get()
	if !ok {
		return errors.New("Multiverse-Core is not enabled")
	}
	np := &NetherPortals{Server: s, core: c, Links: newLinks(s.DataDir()), CustomPortals: &CustomPortals{},
		touching: map[plugin.UUID]LinkType{}, bounced: map[string]time.Time{}}
	np.CustomPortals.Register(handledByPortals)
	np.Config = newConfig(s.DataDir(), np.importLegacyLinks)
	if err := np.load(); err != nil {
		s.Logger().Error("Your configs were not loaded.", "error", err)
		s.Logger().Error("Please check your configs and restart the server.")
		return err
	}
	c.Locales.AddBundles(resources, "resources", "multiverse-netherportals")
	if err := c.Commands.RegisterRoot("mvnp", nil, np.commands()); err != nil {
		return err
	}
	np.listen()
	m.np = np
	instMu.Lock()
	instance = np
	ready := whenReady
	whenReady = nil
	instMu.Unlock()
	for _, fn := range ready {
		fn(np)
	}
	s.Logger().Info("Version " + Version + " Enabled - By " + joinAnd(Authors))
	return nil
}

// Disable saves links.yml and config.yml.
func (m *module) Disable() {
	np := m.np
	if np == nil {
		return
	}
	m.np = nil
	np.task.Cancel()
	instMu.Lock()
	if instance == np {
		instance = nil
	}
	instMu.Unlock()
	if err := np.Links.Save(); err != nil {
		np.Server.Logger().Error("Failed to save links.yml", "error", err)
		return
	}
	if err := np.Config.Save(); err != nil {
		np.Server.Logger().Error("Failed to save config.yml", "error", err)
	}
}

// load reads links.yml, then config.yml (which hands a Multiverse 4 file's
// links to links.yml before it is written without them), and writes
// links.yml back. MV: MultiverseNetherPortals.setupConfig
func (np *NetherPortals) load() error {
	if err := np.Links.Load(); err != nil {
		return err
	}
	np.legacyErr = nil
	if err := np.Config.Load(); err != nil {
		return err
	}
	if np.legacyErr != nil {
		return np.legacyErr
	}
	return np.Links.Save()
}

// importLegacyLinks moves config.yml's worlds.<world>.portalgoesto links to
// links.yml and reports whether links.yml was saved; when it was not,
// config.yml keeps them and load fails. MV: LinksManager.migrateLegacyConfigLinks
func (np *NetherPortals) importLegacyLinks(worlds map[string]any) bool {
	n := np.Links.importLegacy(worlds, "")
	if n == 0 {
		return true
	}
	if err := np.Links.Save(); err != nil {
		np.legacyErr = err
		return false
	}
	np.Server.Logger().Info("Migrated portal links from config.yml to links.yml.", "links", n)
	return true
}

// handledByPortals claims the portals Multiverse-Portals has at loc.
// MV: CustomPortalsHandler.registerMultiversePortalsCheck
func handledByPortals(_ plugin.Entity, loc plugin.Location) bool {
	p, ok := portals.Get()
	if !ok {
		return false
	}
	_, ok = p.Manager.At(loc)
	return ok
}

// joinAnd joins names as "a, b and c". MV: StringFormatter.joinAnd
func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
