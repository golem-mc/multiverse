package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, respawn) }

// respawn sends a respawning player to their world's respawn-world, the
// overworld of a nether or end, or the spawn of the world they died in, as
// the world's properties and config.yml's spawn section say, unless they
// respawn at a bed or anchor the world honours. MV:
// MVPlayerListener.playerRespawn
func respawn(c *core.Core) {
	plugin.Listen(c.Server, func(e *plugin.PlayerRespawnEvent) {
		p := e.Player
		w, ok := c.Worlds.Of(p.World())
		if !ok {
			c.Debug(2, "Player '%s' died in a world that is not managed by Multiverse.", p.Name())
			return
		}
		if w.Props.Bool(core.PropBedRespawn) && e.Bed || w.Props.Bool(core.PropAnchorRespawn) && e.Anchor {
			return
		}
		to, ok := respawnWorld(c, w)
		if !ok {
			return
		}
		h, _ := to.Handle()
		if !c.Config.Bool("spawn.enforce-respawn-at-world-spawn") && e.Location.World != nil && e.Location.World.Key() == h.Key() {
			return
		}
		loc, _ := to.Spawn()
		re := &core.MVRespawnEvent{Player: p, RespawnLocation: loc}
		c.Fire(re)
		if re.Cancelled() {
			return
		}
		e.Location = re.RespawnLocation
	}, plugin.ListenOptions{Priority: c.EventPriority("player-respawn")})
}

// respawnWorld answers the loaded world a player who died in w respawns
// in. MV: MVPlayerListener.getRespawnWorld
func respawnWorld(c *core.Core, w *core.World) (*core.World, bool) {
	if name := w.Props.String(core.PropRespawnWorld); name != "" {
		to, ok := c.Worlds.Get(name)
		if !ok || !to.Loaded() {
			c.Log.Warn("World '" + w.Name() + "' has respawn-world property of '" + name + "' that does not exist!")
			return nil, false
		}
		return to, true
	}
	if w.Environment() != "normal" && c.Config.Bool("spawn.default-respawn-in-overworld") {
		to, ok := c.Worlds.OverworldOf(w)
		if !ok || !to.Loaded() {
			c.Log.Warn("World '" + w.Name() + "' has no overworld to teleport to!")
			return nil, false
		}
		return to, true
	}
	if c.Config.Bool("spawn.default-respawn-within-same-world") {
		return w, true
	}
	return nil, false
}
