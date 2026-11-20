package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, enforcement) }

// enforcement gives players their world's game mode and flight when they
// join or change worlds, and when the world's property changes. MV:
// MVPlayerListener.onPlayerJoin and playerChangedWorld, WorldConfigNodes
// gamemode and allow-flight
func enforcement(c *core.Core) {
	c.Worlds.OnChange(core.PropGameMode, func(w *core.World) {
		for _, p := range w.Players() {
			enforceGameMode(c, p)
		}
	})
	c.Worlds.OnChange(core.PropAllowFlight, func(w *core.World) {
		for _, p := range w.Players() {
			enforceFlight(c, p)
		}
	})
	plugin.Listen(c.Server, func(e *plugin.PlayerJoinEvent) {
		enforceLater(c, e.Player, e.Player.World())
	}, plugin.ListenOptions{Priority: c.EventPriority("player-spawn-location")})
	plugin.Listen(c.Server, func(e *plugin.PlayerChangedWorldEvent) {
		enforceLater(c, e.Player, e.Player.World())
	}, plugin.ListenOptions{Priority: c.EventPriority("player-changed-world")})
}

// enforceLater enforces game mode and flight after
// gamemode-and-flight-enforce-delay ticks, so Multiverse has the last word
// over other plugins, unless the player has left h by then. MV:
// MVPlayerListener.handleGameModeAndFlight
func enforceLater(c *core.Core, p plugin.Player, h plugin.World) {
	run := func() {
		if !p.IsValid() || p.World().Key() != h.Key() {
			c.Debug(2, "Player %s is no longer online or not in the expected world '%s'", p.Name(), h.Key())
			return
		}
		enforceFlight(c, p)
		enforceGameMode(c, p)
	}
	if delay := c.Config.Int("world.gamemode-and-flight-enforce-delay"); delay > 0 {
		c.Server.Scheduler().RunLater(delay, run)
		return
	}
	run()
}

// enforceGameMode gives p their world's game mode, unless they have
// mv.bypass.gamemode.<world>. MV: EnforcementHandler.handleGameModeEnforcement
func enforceGameMode(c *core.Core, p plugin.Player) {
	if !c.Config.Bool("world.enforce-gamemode") {
		return
	}
	w, ok := c.Worlds.Of(p.World())
	if !ok {
		return
	}
	if p.HasPermission(core.Concat(core.PermGameModeBypass, w.Name())) {
		c.Debug(2, "Player is immune to gamemode enforcement: %s", p.Name())
		return
	}
	if mode := w.GameMode(); p.GameMode() != mode {
		p.SetGameMode(mode)
	}
}

// enforceFlight lets p fly or not as their world's allow-flight says:
// spectators always fly, creative players may when it allows, and the
// others lose flight when it does not. MV:
// EnforcementHandler.handleFlightEnforcement
func enforceFlight(c *core.Core, p plugin.Player) {
	if !c.Config.Bool("world.enforce-flight") {
		return
	}
	w, ok := c.Worlds.Of(p.World())
	if !ok {
		return
	}
	switch {
	case p.GameMode() == plugin.Spectator:
		p.SetAllowFlight(true)
		p.SetFlying(true)
	case w.Props.Bool(core.PropAllowFlight):
		if p.GameMode() == plugin.Creative {
			p.SetAllowFlight(true)
		}
	case p.AllowFlight() && p.GameMode() != plugin.Creative:
		if p.Flying() {
			p.SetFlying(false)
		}
		p.SetAllowFlight(false)
	}
}
