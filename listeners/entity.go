package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, entities) }

// entities applies hunger, auto-heal, pvp and the spawning section. MV:
// MVEntityListener, WorldConfigNodes pvp
func entities(c *core.Core) {
	ignoreCancelled := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(c.Server, func(e *plugin.FoodLevelChangeEvent) {
		if w, ok := c.Worlds.Of(e.Player.World()); ok && !w.Props.Bool(core.PropHunger) && e.Level < e.Player.Food() {
			e.SetCancelled(true)
		}
	}, ignoreCancelled)
	plugin.Listen(c.Server, func(e *plugin.EntityRegainHealthEvent) {
		// Bukkit's REGEN is peaceful regeneration; golem calls it natural,
		// as it does food healing short of saturation.
		if e.Reason != plugin.RegainNatural || e.Entity.World().Difficulty() != plugin.DifficultyPeaceful {
			return
		}
		if w, ok := c.Worlds.Of(e.Entity.World()); ok && !w.Props.Bool(core.PropAutoHeal) {
			e.SetCancelled(true)
		}
	}, ignoreCancelled)
	// Bukkit's World.setPVP, which golem worlds lack: a player's blow or
	// shot at another player does nothing. vanilla: ServerPlayer.canHarmPlayer
	plugin.Listen(c.Server, func(e *plugin.EntityDamageEvent) {
		victim, ok := e.Entity.(plugin.Player)
		if !ok {
			return
		}
		if _, byPlayer := e.Source.Causing.(plugin.Player); !byPlayer {
			return
		}
		if w, ok := c.Worlds.Of(victim.World()); ok && !w.Props.Bool(core.PropPvp) {
			e.SetCancelled(true)
		}
	}, ignoreCancelled)
	plugin.Listen(c.Server, func(e *plugin.EntitySpawnEvent) {
		if alwaysAllowed(e.Reason) {
			return
		}
		if w, ok := c.Worlds.Of(e.Entity.World()); ok && !w.Spawning().AllowsEntity(e.Entity.Type().Name()) {
			c.Debug(3, "Cancelling Creature Spawn Event for: %s", e.Entity.Type())
			e.SetCancelled(true)
		}
	}, ignoreCancelled)
}

// alwaysAllowed are the spawns the spawning section never stops: plugins,
// commands, breeding and spawn eggs. MV:
// MVEntityListener.isAlwaysAllowedSpawnReason
func alwaysAllowed(r plugin.SpawnReason) bool {
	switch r {
	case plugin.SpawnPlugin, plugin.SpawnCommand, plugin.SpawnBreeding, plugin.SpawnSpawnerEgg:
		return true
	}
	return false
}
