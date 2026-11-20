// Package inventories is Multiverse-Inventories, ported to golem: the
// plugin registered as "multiverse-inventories". It keeps each world's or
// group's players' inventories and stats apart, switching them as players
// change worlds and game modes.
package inventories

import (
	"embed"
	"fmt"
	"maps"
	"math"
	"reflect"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// Version is the Multiverse-Inventories release this port follows.
const Version = "5.3.0"

//go:embed resources
var resources embed.FS

func init() {
	plugin.Register("multiverse-inventories", &Module{}, plugin.Depends("multiverse-core"))
}

// Module is the running Multiverse-Inventories. MV: MultiverseInventories
type Module struct {
	Server plugin.Server
	Log    *plugin.Logger
	Core   *core.Core
	Config *core.Config
	Groups *Groups

	store *store
	// handling holds the profiles a share handling is applying, by player.
	handling map[plugin.UUID]*AffectedProfiles
	// legacyGroups are the groups an old config.yml held, for groups.yml.
	legacyGroups map[string]any
	// economyWarned keeps the missing economy warning to once.
	economyWarned bool
	dupe          *dupingPatch
	firstRun      *core.Node
	// groupSpawn is the group spawn a respawn was sent to.
	groupSpawn *plugin.Location
}

var (
	instMu   sync.Mutex
	instance *Module
)

// Get answers the running Multiverse-Inventories; false before it is
// enabled or after it is disabled. MV: MultiverseInventoriesApi.get
func Get() (*Module, bool) {
	instMu.Lock()
	defer instMu.Unlock()
	return instance, instance != nil
}

// Enable reads config.yml and groups.yml and starts switching players'
// data. MV: MultiverseInventories.onEnable
func (m *Module) Enable(s plugin.Server) error {
	c, ok := core.Get()
	if !ok {
		return fmt.Errorf("multiverse-core is not enabled")
	}
	*m = Module{Server: s, Log: s.Logger(), Core: c, handling: map[plugin.UUID]*AffectedProfiles{}}
	for _, e := range shareEvents {
		s.Events().Define(reflect.TypeOf(e))
	}
	c.Locales.AddBundles(resources, "resources", "multiverse-inventories")
	m.Config = m.newConfig()
	m.Groups = newGroups(m)
	m.store = newStore(m, s.DataDir())
	m.registerPermissions()
	if err := m.reload(); err != nil {
		return err
	}
	m.store.names.load()
	m.listen()
	m.dupe = newDupingPatch(m)
	c.OnReload(func() {
		if err := m.reload(); err != nil {
			m.Log.Error("Failed to reload Multiverse-Inventories", "error", err)
		}
	})
	instMu.Lock()
	instance = m
	instMu.Unlock()
	return nil
}

// Disable saves the online players' data. MV: MultiverseInventories.onDisable
func (m *Module) Disable() {
	if m.Server == nil {
		return
	}
	for _, p := range m.Server.Players() {
		m.writeLastLocation(p, p.Location())
		m.writeOnly(p)
		if m.Config.Bool("performance.apply-playerdata-on-join") {
			g := m.store.Global(p.UUID(), p.Name())
			g.LoadOnLogin = true
			m.store.SaveGlobal(g)
		}
	}
	instMu.Lock()
	if instance == m {
		instance = nil
	}
	instMu.Unlock()
}

// reload reads config.yml and groups.yml and forgets the profiles read; a
// tick later it makes the default group on the first run and reports
// conflicting groups. MV: MultiverseInventories.reloadConfig
func (m *Module) reload() error {
	if err := m.Config.Load(); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}
	if err := m.Groups.Load(); err != nil {
		m.Log.Error("Failed to load world groups!", "error", err)
	}
	m.store.clear()
	m.Server.Scheduler().RunLater(1, func() {
		if m.Config.Bool("first-run") {
			m.Log.Info("First run!")
			if len(m.Groups.All()) == 0 {
				m.Groups.createDefault()
			}
			err := m.Config.Set(m.firstRun, false)
			if err == nil {
				err = m.Config.Save()
			}
			if err != nil {
				m.Log.Error("Failed to save config file!", "error", err)
			}
		}
		m.sendConflicts(m.Core.Issuer(m.Server.Console()), false)
	})
	return nil
}

// recalculateEnabledShares refreshes the groups' shares after the active
// optional shares change. MV: Sharables.recalculateEnabledShares
func (m *Module) recalculateEnabledShares() {
	if m.Groups != nil {
		m.Groups.RecalculateShares()
	}
}

// sendConflicts tells is about conflicting groups; with checking, also
// that it checks and when there are none. MV: GroupingConflictResult
func (m *Module) sendConflicts(is *core.Issuer, checking bool) {
	if checking {
		is.Info(core.M("mv-inventories.conflict.checking"))
	}
	conflicts := m.Groups.Conflicts()
	for _, c := range conflicts {
		is.Info(core.M("mv-inventories.conflict.results", "{group1}", c.First.name, "{group2}", c.Second.name,
			"{shares}", c.Shares.String(), "{worlds}", strings.Join(c.Worlds(), ", ")))
	}
	switch {
	case len(conflicts) > 0:
		is.Info(core.M("mv-inventories.conflict.found"))
	case checking:
		is.Info(core.M("mv-inventories.conflict.notfound"))
	}
}

// Debug logs a debug message when Multiverse-Core's global-debug is at
// least level. MV: Logging.fine/finer/finest
func (m *Module) Debug(level int, format string, args ...any) {
	if m.Core.Config.Int("misc.global-debug") >= level {
		m.Log.Info(fmt.Sprintf("[MVInv-Debug] "+format, args...))
	}
}

// worldName answers a loaded world's name as Multiverse names it.
func (m *Module) worldName(w plugin.World) string {
	if w == nil {
		return ""
	}
	if mw, ok := m.Core.Worlds.Of(w); ok {
		return mw.Name()
	}
	return m.Core.RefOfKey(w.Key()).Name
}

// worldNamed answers the loaded world with a name, ignoring case, as
// Bukkit.getWorld does.
func (m *Module) worldNamed(name string) (plugin.World, bool) {
	if mw, ok := m.Core.Worlds.Get(name); ok {
		if h, ok := mw.Handle(); ok {
			return h, true
		}
	}
	for _, w := range m.Server.Worlds() {
		if strings.EqualFold(m.worldName(w), name) {
			return w, true
		}
	}
	return nil, false
}

// validWorldName reports whether a group's world entry names a world, or
// is a pattern. MV: GroupWorldNameValidator
func (m *Module) validWorldName(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "*") || strings.HasPrefix(name, "r=") {
		return true
	}
	if _, ok := m.Core.Worlds.Get(name); ok {
		return true
	}
	_, ok := m.worldNamed(name)
	return ok
}

// locOf answers a saved location of loc.
func (m *Module) locOf(loc plugin.Location, respawn string) *Loc {
	return &Loc{World: m.worldName(loc.World), X: loc.Pos.X, Y: loc.Pos.Y, Z: loc.Pos.Z,
		Yaw: loc.Yaw, Pitch: loc.Pitch, Respawn: respawn}
}

// locationOf answers where a saved location is, false when its world is
// not loaded.
func (m *Module) locationOf(l *Loc) (plugin.Location, bool) {
	w, ok := m.worldNamed(l.World)
	if !ok {
		return plugin.Location{}, false
	}
	return plugin.Location{World: w, Pos: plugin.Vec3{X: l.X, Y: l.Y, Z: l.Z}, Yaw: l.Yaw, Pitch: l.Pitch}, true
}

func blockAtPos(l plugin.Location) registry.Block {
	pos := plugin.BlockPos{X: int(math.Floor(l.Pos.X)), Y: int(math.Floor(l.Pos.Y)), Z: int(math.Floor(l.Pos.Z))}
	return l.World.BlockState(pos).Block()
}

func isBed(b registry.Block) bool { return b.InTag(registry.BlockTagOf("minecraft:beds")) }

// respawnLocationOf answers a player's respawn point, marked a bed or an
// anchor by the block there; nil when they have none.
// MV: Sharables.BED_SPAWN.updateProfile, SpawnChangeListener
func (m *Module) respawnLocationOf(p plugin.Player) any {
	loc, ok := p.RespawnPoint()
	if !ok || loc.World == nil {
		return nil
	}
	kind := "UNKNOWN"
	switch b := blockAtPos(loc); {
	case isBed(b):
		kind = "BED"
	case b == registry.BlockRespawnAnchor:
		kind = "ANCHOR"
	}
	return m.locOf(loc, kind)
}

// applyBedSpawn sets a player's respawn point from the profile: the world
// spawn of their world when it has none, its world is not loaded, or with
// validate-bed-anchor-respawn-location the bed or anchor is gone.
// MV: Sharables.BED_SPAWN.updatePlayer
func (m *Module) applyBedSpawn(p plugin.Player, v any) bool {
	toSpawn := func() bool {
		spawn := p.World().SpawnPoint()
		p.SetRespawnPoint(&spawn, true)
		return false
	}
	l, ok := v.(*Loc)
	if !ok {
		m.Debug(2, "No respawn location saved")
		return toSpawn()
	}
	loc, ok := m.locationOf(l)
	if !ok {
		m.Log.Warn("Respawn location has invalid world!")
		return toSpawn()
	}
	if m.Config.Bool("sharables.validate-bed-anchor-respawn-location") {
		b := blockAtPos(loc)
		if l.Respawn == "BED" && !isBed(b) || l.Respawn == "ANCHOR" && b != registry.BlockRespawnAnchor {
			m.Debug(2, "Respawn location validation failed for respawn type: %s", l.Respawn)
			return toSpawn()
		}
	}
	p.SetRespawnPoint(&loc, true)
	return true
}

// warnNoEconomy says once that the economy sharable cannot work.
func (m *Module) warnNoEconomy() {
	if m.economyWarned {
		return
	}
	m.economyWarned = true
	m.Log.Warn("You do not have an an economy plugin with Vault. Economy sharable will not work!")
	m.Log.Warn("Check that your economy and vault plugin are both working correctly.")
	m.Log.Warn("If you enabled the sharable by mistake, run '/mvinv toggle economy' to resolve this.")
}

// The bypass permission prefixes. MV: Perm
const (
	bypassGroup    = "mvinv.bypass.group."
	bypassWorld    = "mvinv.bypass.world."
	bypassGameMode = "mvinv.bypass.gamemode."
	bypassAll      = "mvinv.bypass.*"
)

// registerPermissions registers the bypass wildcards. MV: Perm.register
func (m *Module) registerPermissions() {
	reg := m.Server.Permissions()
	for _, node := range []string{bypassGroup + "*", bypassWorld + "*", bypassGameMode + "*"} {
		if _, ok := reg.Get(node); !ok {
			reg.Register(plugin.Permission{Name: node, Default: core.DefaultFalse, Children: map[string]bool{}})
		}
	}
	if _, ok := reg.Get(bypassAll); !ok {
		reg.Register(plugin.Permission{Name: bypassAll, Default: core.DefaultFalse,
			Description: "Allows bypassing all of your groups/worlds and constantly use the same inventory",
			Children:    map[string]bool{bypassGroup + "*": true, bypassWorld + "*": true}})
	}
}

// hasBypass reports whether a player bypasses share handling for a group,
// world or game mode, registering its node on first use. Always false
// unless enable-bypass-permissions. MV: Perm.hasBypass
func (m *Module) hasBypass(p plugin.Player, prefix, name string) bool {
	if !m.Config.Bool("share-handling.enable-bypass-permissions") {
		return false
	}
	node := prefix + name
	reg := m.Server.Permissions()
	if _, ok := reg.Get(node); !ok {
		reg.Register(plugin.Permission{Name: node, Default: core.DefaultFalse})
		addChild := func(wildcard string) {
			perm, _ := reg.Get(wildcard)
			children := map[string]bool{node: true}
			maps.Copy(children, perm.Children)
			perm.Children = children
			reg.Register(perm)
		}
		addChild(prefix + "*")
		if prefix != bypassGameMode {
			addChild(bypassAll)
		}
	}
	has := p.HasPermission(node)
	if has {
		m.Debug(1, "Player: %s in World: %s has permission: %s", p.Name(), m.worldName(p.World()), node)
	}
	return has
}
