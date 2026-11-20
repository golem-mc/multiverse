package netherportals

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/core"
)

// bounceCooldown is how long after a bounce a player is not told again.
// MV: MVNPEntityListener.COOLDOWN
const bounceCooldown = 250 * time.Millisecond

// listen registers the portal listeners and the portal touch check.
// MV: MVNPEntityListener, MVNPPlayerListener, MVNPCoreListener
func (np *NetherPortals) listen() {
	plugin.Listen(np.Server, np.onPortal, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
	plugin.Listen(np.Server, np.onReload, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(np.Server, np.onDumps, plugin.ListenOptions{Priority: plugin.Normal})
	// golem has no EntityPortalEnterEvent: the players' feet are checked
	// each tick, as golem's own portal tick reads them.
	np.task = np.Server.Scheduler().RunTimer(1, 1, np.checkTouching)
}

// onReload reads config.yml and links.yml again. MV: MVNPCoreListener.configReloadEvent
func (np *NetherPortals) onReload(e *core.MVConfigReloadEvent) {
	if err := np.load(); err != nil {
		np.Server.Logger().Error("Failed to reload Multiverse-NetherPortals' configs", "error", err)
	}
	e.Configs = append(e.Configs, "Multiverse-NetherPortals - config.yml", "Multiverse-NetherPortals - links.yml")
}

// onDumps adds config.yml and links.yml to /mv dumps. MV: MVNPCoreListener.versionEvent
func (np *NetherPortals) onDumps(e *core.MVDumpsDebugInfoEvent) {
	for _, name := range []string{"config.yml", "links.yml"} {
		data, err := os.ReadFile(filepath.Join(np.Server.DataDir(), name))
		if err != nil {
			e.PutDetailedDebugInfo("multiverse-netherportals/"+name, "ERROR: Could not load: "+name)
			continue
		}
		e.PutDetailedDebugInfo("multiverse-netherportals/"+name, string(data))
	}
}

// linkTypeOf answers the portal type of a trip's cause.
func linkTypeOf(c plugin.TeleportCause) (LinkType, bool) {
	switch c {
	case plugin.TeleportNetherPortal:
		return Nether, true
	case plugin.TeleportEndPortal:
		return End, true
	}
	return 0, false
}

// onPortal sends a portal trip to the linked world.
// MV: MVNPPlayerListener.onPlayerPortal, MVNPEntityListener.onEntityPortal
func (np *NetherPortals) onPortal(e *plugin.EntityPortalEvent) {
	t, ok := linkTypeOf(e.Cause)
	if !ok || np.CustomPortals.Handled(e.Entity, e.From) {
		return
	}
	from, ok := np.core.Worlds.Of(e.From.World)
	if !ok {
		return
	}
	p, isPlayer := e.Entity.(plugin.Player)
	if !isPlayer && !np.Config.Bool(PathTeleportEntities) {
		e.SetCancelled(true)
		return
	}
	linked := np.Destination(from.Name(), t)
	if !isPlayer {
		linked, _ = np.Links.To(from.Name(), t)
	}
	to, ok := np.destinationOf(e.From, t, from.Name(), linked)
	if !ok {
		e.SetCancelled(true)
		return
	}
	toW, _ := np.core.Worlds.Of(to.World)
	arrival := e.To
	e.To = to
	switch {
	case from.Environment() == "the_end" && t == End:
		e.CanCreatePortal = false
		if isPlayer {
			np.playerEndExit(e, p, from, toW, arrival)
			return
		}
		if spawn, ok := toW.Spawn(); ok {
			e.To = spawn
		}
	case isPlayer && from.Environment() == "nether" && t == Nether:
		e.CanCreatePortal = true
	case toW.Environment() == "the_end" && t == End:
		e.To = endArrival(e.Entity, to.World)
		createEndPlatform(to.World, np.Config.Bool(PathEndPlatformDropBlocks))
	}
}

// playerEndExit sends a player leaving an end world to the linked world:
// to their bed or anchor when it is there, else to its spawn. golem treats
// leaving the vanilla end as a respawn, arrival being where it respawns the
// player. MV: MVNPPlayerListener.handleEndPortalExit, onPlayerPortal
func (np *NetherPortals) playerEndExit(e *plugin.EntityPortalEvent, p plugin.Player, from, to *core.World, arrival plugin.Location) {
	respawn := e.From.World.Key() == plugin.MinecraftKey("the_end")
	if respawn && !np.Config.Bool(PathHandleEndExitRespawn) {
		e.To = arrival
		return
	}
	toH, _ := to.Handle()
	point, hasPoint := p.RespawnPoint()
	if hasPoint && point.World != nil {
		if respawn {
			if near(arrival, point) {
				if from.Props.Bool(core.PropBedRespawn) && point.World.Key() == toH.Key() {
					e.To = arrival
					return
				}
				nether, ok := np.core.Worlds.Of(point.World)
				if from.Props.Bool(core.PropAnchorRespawn) && ok && nether.Name() == np.Destination(to.Name(), Nether) {
					e.To = arrival
					return
				}
			}
		} else if to.Props.Bool(core.PropBedRespawn) && point.World.Key() == toH.Key() {
			e.To = point
			return
		}
	}
	if spawn, ok := to.Spawn(); ok {
		e.To = spawn
	}
}

// near reports whether golem's respawn arrival is at the bed or anchor:
// it stands the player beside the block.
func near(a, b plugin.Location) bool {
	return a.World != nil && b.World != nil && a.World.Key() == b.World.Key() && a.Pos.DistanceSq(b.Pos) <= 9
}

// checkTouching notices the players who stepped into a portal and forgets
// those who left one. MV: MVEventRecord, PlayerTouchingPortalTask,
// MVNPEntityListener.onEntityPortalExit
func (np *NetherPortals) checkTouching() {
	online := map[plugin.UUID]bool{}
	for _, p := range np.Server.Players() {
		id := p.UUID()
		online[id] = true
		pos := p.Pos()
		block := plugin.BlockPos{X: int(math.Floor(pos.X)), Y: int(math.Floor(pos.Y)), Z: int(math.Floor(pos.Z))}
		var t LinkType
		switch p.World().BlockState(block).Block() {
		case registry.BlockNetherPortal:
			t = Nether
		case registry.BlockEndPortal:
			t = End
		default:
			delete(np.touching, id)
			continue
		}
		if prev, ok := np.touching[id]; ok && prev == t {
			continue
		}
		np.touching[id] = t
		np.touched(p, t, block)
	}
	for id := range np.touching {
		if !online[id] {
			delete(np.touching, id)
		}
	}
}

// touched handles a player stepping into a portal: it bounces them back,
// with a message, when the portal leads nowhere they may go.
// MV: MVNPEntityListener.onEntityPortalEnter
func (np *NetherPortals) touched(p plugin.Player, t LinkType, block plugin.BlockPos) {
	w := p.World()
	at := plugin.Location{World: w, Pos: plugin.Vec3{X: float64(block.X), Y: float64(block.Y), Z: float64(block.Z)}}
	if np.CustomPortals.Handled(p, at) {
		return
	}
	e := &core.MVPlayerTouchedPortalEvent{Player: p, BlockTouched: at}
	np.core.Fire(e)
	if e.Cancelled() {
		return
	}
	if !e.CanUseThisPortal() {
		np.bounceAndForget(p, block, t)
	}
	if last, ok := np.bounced[p.Name()]; ok {
		if time.Since(last) < bounceCooldown {
			return
		}
		delete(np.bounced, p.Name())
	}
	from, ok := np.core.Worlds.Of(w)
	if !ok {
		return
	}
	linked, _ := np.Links.To(from.Name(), t)
	to, ok := np.destinationOf(at, t, from.Name(), linked)
	if !ok {
		np.bounceAndForget(p, block, t)
		is := np.core.Issuer(p)
		if linked != "" && strings.EqualFold(from.Name(), linked) {
			if np.Config.Bool(PathSendDisabledPortalMessage) {
				is.Error(core.M("mv-netherportals.portal.disabled", "{linkType}", t.Message()))
			}
		} else if np.Config.Bool(PathSendNoDestinationMessage) {
			auto := np.NetherName(from.Name())
			if t == End {
				auto = np.EndName(from.Name())
			}
			is.Error(core.M("mv-netherportals.portal.nodestination"))
			is.Error(core.M("mv-netherportals.portal.autolinkedworld.notfound", "{linkType}", t.Message(), "{world}", auto))
		}
		return
	}
	toW, ok := np.core.Worlds.Of(to.World)
	if !ok || toW == from {
		return
	}
	if _, err := np.core.EntryFor(p).CanEnter(from, toW); err != nil {
		np.bounceAndForget(p, block, t)
		np.core.Debug(1, "Player '%s' was DENIED ACCESS to '%s': %v", p.Name(), toW.Name(), err)
	}
}

// bounceAndForget bounces p back and, when it did, forgets they touched the
// portal so stepping in again is noticed.
func (np *NetherPortals) bounceAndForget(p plugin.Player, block plugin.BlockPos, t LinkType) {
	if np.bounce(p, block, t) {
		delete(np.touching, p.UUID())
	}
}

// bounce pushes p back out of the nether portal at block, away from its
// face, and reports whether it did: only with bounceback on, and not for end
// portals. MV: MVNPEntityListener.shootPlayer
func (np *NetherPortals) bounce(p plugin.Player, block plugin.BlockPos, t LinkType) bool {
	if !np.Config.Bool(PathBounceback) {
		return false
	}
	np.bounced[p.Name()] = time.Now()
	if t != Nether {
		return false
	}
	w := p.World()
	loc := p.Location()
	var vx, vz float64
	alongX := w.BlockState(block.Offset(plugin.West, 1)).Block() == registry.BlockNetherPortal ||
		w.BlockState(block.Offset(plugin.East, 1)).Block() == registry.BlockNetherPortal
	switch {
	case alongX && loc.Pos.Z < float64(block.Z)+.5:
		vz = -1
	case alongX:
		vz = 1
	case loc.Pos.X < float64(block.X)+.5:
		vx = -1
	default:
		vx = 1
	}
	loc.Pos = loc.Pos.Add(plugin.Vec3{X: vx, Y: .2, Z: vz})
	p.Teleport(loc, plugin.TeleportPlugin)
	p.SetVelocity(plugin.Vec3{X: vx, Y: .6, Z: vz})
	return true
}
