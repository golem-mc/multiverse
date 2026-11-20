package inventories

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// listen registers the listeners that switch players' data.
// MV: ShareHandleListener, RespawnListener, MVEventsListener
func (m *Module) listen() {
	s := m.Server
	monitor := plugin.ListenOptions{Priority: plugin.Monitor}
	plugin.Listen(s, m.onJoin)
	plugin.Listen(s, m.onQuit)
	plugin.Listen(s, m.onGameModeChange, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	plugin.Listen(s, m.onChangedWorld, plugin.ListenOptions{Priority: plugin.Low})
	plugin.Listen(s, m.onTeleport, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	plugin.Listen(s, m.onDeath, monitor)
	plugin.Listen(s, m.onRespawnVerify, monitor)
	plugin.Listen(s, m.onEntityPortal, plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true})
	for i, pr := range []plugin.Priority{plugin.Lowest, plugin.Low, plugin.Normal, plugin.High, plugin.Highest, plugin.Monitor} {
		plugin.Listen(s, func(e *plugin.PlayerRespawnEvent) { m.groupRespawn(e, priorityNames[i]) },
			plugin.ListenOptions{Priority: pr})
	}
	plugin.Listen(s, func(e *plugin.WorldLoadEvent) { m.Groups.worldLoaded(m.worldName(e.World)) })
	plugin.Listen(s, func(e *plugin.WorldUnloadEvent) { m.worldUnload(m.worldName(e.World)) })
	plugin.Listen(s, func(e *core.MVWorldLoadedEvent) { m.Groups.worldLoaded(e.World.Name()) })
	plugin.Listen(s, func(e *core.MVWorldUnloadedEvent) { m.Groups.worldUnloaded(e.World.Name()) })
	plugin.Listen(s, func(e *core.MVConfigReloadEvent) {
		e.Configs = append(e.Configs, "Multiverse-Inventories - config.yml")
	})
}

// onJoin follows a name change, loads the player's data with
// apply-playerdata-on-join and switches it when they join in another world
// than they left. MV: ShareHandleListener.playerJoin
func (m *Module) onJoin(e *plugin.PlayerJoinEvent) {
	p := e.Player
	m.verifyName(p)
	g := m.store.Global(p.UUID(), p.Name())
	if g.LoadOnLogin {
		m.readOnly(p)
	}
	g.LoadOnLogin = false
	m.verifyWorld(p, g)
	m.store.SaveGlobal(g)
	m.dupe.forget(p)
}

// verifyName moves a renamed player's files to their new name.
// MV: ShareHandleListener.verifyCorrectPlayerName
func (m *Module) verifyName(p plugin.Player) {
	g, ok := m.store.ExistingGlobal(p.UUID(), p.Name())
	if !ok || g.LastKnownName == p.Name() {
		return
	}
	m.Log.Info("Player " + p.UUID().String() + " changed name from '" + g.LastKnownName + "' to '" + p.Name() +
		"'. Attempting to migrate playerdata...")
	m.store.migrateName(p.UUID(), g.LastKnownName, p.Name())
	g = m.store.Global(p.UUID(), p.Name())
	g.LastKnownName = p.Name()
	m.store.SaveGlobal(g)
	m.Log.Info("Migration complete!")
}

// verifyWorld switches a player's data when they are not in the world the
// global profile last saw them in. MV: ShareHandleListener.verifyCorrectWorld
func (m *Module) verifyWorld(p plugin.Player, g *GlobalProfile) {
	world := m.worldName(p.World())
	switch {
	case g.LastWorld == "":
		g.LastWorld = world
	case g.LastWorld != world:
		m.Debug(1, "Player did not spawn in the world they were last reported to be in!")
		m.worldChange(p, g.LastWorld, world)
		g.LastWorld = world
	}
}

// onQuit saves the player's data and forgets it. MV: ShareHandleListener.playerQuit
func (m *Module) onQuit(e *plugin.PlayerQuitEvent) {
	p := e.Player
	g := m.store.Global(p.UUID(), p.Name())
	g.LastWorld = m.worldName(p.World())
	// The last location is written as a player may join in another world.
	m.writeLastLocation(p, p.Location())
	m.writeOnly(p)
	if m.Config.Bool("performance.apply-playerdata-on-join") {
		g.LoadOnLogin = true
	}
	m.store.SaveGlobal(g)
	m.store.forgetPlayer(p.UUID())
	m.dupe.forget(p)
}

// onGameModeChange switches the player's data to the new game mode's
// profiles with enable-gamemode-share-handling.
// MV: ShareHandleListener.playerGameModeChange
func (m *Module) onGameModeChange(e *plugin.PlayerGameModeChangeEvent) {
	if !m.Config.Bool("share-handling.enable-gamemode-share-handling") {
		return
	}
	p := e.Player
	m.writeLastLocation(p, p.Location())
	m.gameModeChange(p, p.GameMode(), e.New)
}

// onChangedWorld switches the player's data to the new world's.
// MV: ShareHandleListener.playerChangedWorld
func (m *Module) onChangedWorld(e *plugin.PlayerChangedWorldEvent) {
	p := e.Player
	from, to := m.worldName(e.From), m.worldName(p.World())
	if from == to {
		m.Debug(1, "PlayerChangedWorldEvent fired when player travelling in same world.")
		return
	}
	m.worldChange(p, from, to)
	g := m.store.Global(p.UUID(), p.Name())
	g.LastWorld = to
	m.store.SaveGlobal(g)
	m.dupe.changedWorld(p)
}

// onTeleport records where a player leaves a world from.
// MV: ShareHandleListener.playerTeleport
func (m *Module) onTeleport(e *plugin.EntityTeleportEvent) {
	p, ok := e.Entity.(plugin.Player)
	if !ok || e.From.World == e.To.World || !m.activeOptionalShares().Has(LastLocation) {
		return
	}
	m.writeLastLocation(p, e.From)
	// Closing the inventory may prevent an item duping exploit.
	p.CloseInventory()
}

// onDeath writes the experience the player keeps into the profiles of the
// world they died in. MV: ShareHandleListener.playerDeath
func (m *Module) onDeath(e *plugin.PlayerDeathEvent) {
	p := e.Player
	m.Debug(2, "=== Handling PlayerDeathEvent for: %s ===", p.Name())
	world := m.worldName(p.World())
	typ := m.profileType(p)
	level, progress, total := 0, float32(0), 0
	if e.KeepLevel {
		level, progress, total = p.Level(), p.Progress(), p.TotalExperience()
	}
	reset := func(k ProfileKey) {
		prof := m.store.Profile(k)
		prof.Set(Level, level)
		prof.Set(Experience, progress)
		prof.Set(TotalExperience, total)
		if m.Config.Bool("sharables.reset-last-location-on-death") {
			prof.Set(LastLocation, nil)
		}
		m.store.Save(prof)
	}
	reset(key(ContainerWorld, world, typ, p))
	for _, g := range m.Groups.ForWorld(world) {
		reset(key(ContainerGroup, g.name, typ, p))
	}
	m.Debug(2, "=== Finished handling PlayerDeathEvent for: %s! ===", p.Name())
}

// onRespawnVerify switches the player's data two ticks after a respawn
// that took them to another world. MV: ShareHandleListener.playerRespawn
func (m *Module) onRespawnVerify(e *plugin.PlayerRespawnEvent) {
	p := e.Player
	m.Server.Scheduler().RunLater(2, func() {
		if !p.IsValid() {
			return
		}
		g := m.store.Global(p.UUID(), p.Name())
		m.verifyWorld(p, g)
		m.store.SaveGlobal(g)
	})
}

// groupRespawn sends a player who did not respawn at a bed or anchor to
// their group's spawn world, at the group's spawn priority; at Monitor it
// points their compass there. MV: RespawnListener
func (m *Module) groupRespawn(e *plugin.PlayerRespawnEvent, priority string) {
	if e.Bed || e.Anchor {
		return
	}
	if priority == "LOWEST" {
		m.groupSpawn = nil
	}
	for _, g := range m.Groups.ForWorld(m.worldName(e.Player.World())) {
		if g.spawnPriority != priority || g.spawnWorld == "" {
			continue
		}
		w, ok := m.Core.Worlds.GetLoaded(g.spawnWorld)
		if !ok {
			continue
		}
		if spawn, ok := w.Spawn(); ok {
			m.groupSpawn = &spawn
			e.Location = spawn
			break
		}
	}
	if priority == "MONITOR" && m.groupSpawn != nil && e.Location == *m.groupSpawn {
		e.Player.SetCompassTarget(m.groupSpawn)
	}
}

// onEntityPortal keeps items and entities holding an inventory from
// travelling between worlds that do not share inventories.
// MV: ShareHandleListener.entityPortal
func (m *Module) onEntityPortal(e *plugin.EntityPortalEvent) {
	_, isItem := e.Entity.(plugin.ItemEntity)
	_, holder := e.Entity.(interface{ Inventory() plugin.Inventory })
	if !isItem && !holder || e.From.World == nil || e.To.World == nil || e.From.World == e.To.World {
		return
	}
	sharing := func(world string) []*WorldGroup {
		return slices.DeleteFunc(m.Groups.ForWorld(world), func(g *WorldGroup) bool { return !g.IsSharing(Inventory) })
	}
	to := sharing(m.worldName(e.To.World))
	for _, g := range sharing(m.worldName(e.From.World)) {
		if slices.Contains(to, g) {
			return
		}
	}
	e.SetCancelled(true)
}

// worldUnload forgets the profiles of a world that unloads and of its
// groups. MV: ShareHandleListener.worldUnload
func (m *Module) worldUnload(name string) {
	m.store.forgetContainer(ContainerWorld, name)
	for _, g := range m.Groups.ForWorld(name) {
		m.store.forgetContainer(ContainerGroup, g.name)
	}
	m.Groups.worldUnloaded(name)
}

// dupingPatch refuses creative inventory changes for a few ticks after a
// world change, which could otherwise copy items between groups.
// MV: InventoriesDupingPatch
type dupingPatch struct {
	timeouts map[plugin.UUID]int
}

// slotTimeout is how many ticks after a world change creative slot changes
// are refused.
const slotTimeout = 5

func newDupingPatch(m *Module) *dupingPatch {
	d := &dupingPatch{timeouts: map[plugin.UUID]int{}}
	plugin.Listen(m.Server, func(e *plugin.InventoryClickEvent) {
		if len(d.timeouts) == 0 || e.Click != plugin.ClickCreative {
			return
		}
		if _, ok := d.timeouts[e.Player.UUID()]; ok {
			e.SetCancelled(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Lowest})
	m.Server.Scheduler().RunTimer(1, 1, func() {
		for id, left := range d.timeouts {
			if left <= 1 {
				delete(d.timeouts, id)
			} else {
				d.timeouts[id] = left - 1
			}
		}
	})
	return d
}

func (d *dupingPatch) changedWorld(p plugin.Player) { d.timeouts[p.UUID()] = slotTimeout }
func (d *dupingPatch) forget(p plugin.Player)       { delete(d.timeouts, p.UUID()) }
