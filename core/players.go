package core

import (
	"github.com/golem-mc/golem/plugin"
)

// listenPlayers checks world entry on teleports and portals and sends
// joining players to the join destination or first spawn.
// MV: MVPlayerListener
func (c *Core) listenPlayers() {
	firstJoin := map[plugin.UUID]bool{}
	plugin.Listen(c.Server, func(e *plugin.PlayerPreLoginEvent) {
		if _, played := c.Server.OfflinePlayer(e.Profile.ID); !played {
			firstJoin[e.Profile.ID] = true
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor})
	plugin.Listen(c.Server, func(e *plugin.PlayerJoinEvent) {
		first := firstJoin[e.Player.UUID()]
		delete(firstJoin, e.Player.UUID())
		c.onJoin(e.Player, first)
	}, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(c.Server, c.onTeleport, plugin.ListenOptions{Priority: plugin.Highest, IgnoreCancelled: true})
	plugin.Listen(c.Server, c.onPortal, plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true})
}

// onJoin sends a new player to first-spawn-location and a returning one
// to join-destination. MV: MVPlayerListener.onPlayerJoin
func (c *Core) onJoin(p plugin.Player, first bool) {
	var key, dest string
	switch {
	case first:
		if !c.Config.Bool("spawn.first-spawn-override") {
			return
		}
		key, dest = "FirstSpawnLocation", c.Config.String("spawn.first-spawn-location")
	case c.Config.Bool("spawn.enable-join-destination"):
		dest = c.Config.String("spawn.join-destination")
		if dest == "" {
			c.Log.Warn("Joindestination is enabled but no destination has been specified in config!")
			return
		}
		if p.HasPermission(PermJoinLocationBypass) {
			return
		}
		key = "JoinDestination"
	default:
		return
	}
	d, err := c.Destinations.Parse(c.Server.Console(), dest)
	if err != nil {
		c.Log.Warn("Invalid destination in " + key + " in config: " + StripColor(english.Format(nil, ErrorMessage(err))))
		return
	}
	c.TeleportTo(d).Run(p, func(f TeleportFailure) {
		if f != "" {
			c.Log.Warn("Failed to teleport player " + p.Name() + " on join: " + string(f))
		}
	})
}

// portalCauses are the teleports EntityPortalEvent already checked.
var portalCauses = map[plugin.TeleportCause]bool{
	plugin.TeleportNetherPortal: true, plugin.TeleportEndPortal: true, plugin.TeleportEndGateway: true,
}

// onTeleport refuses a player's trip into a world they may not enter: when
// Multiverse moves them, as whoever asked; any other teleport only with
// teleport-intercept. MV: MVPlayerListener.playerTeleport
func (c *Core) onTeleport(e *plugin.EntityTeleportEvent) {
	p, ok := e.Entity.(plugin.Player)
	if !ok || portalCauses[e.Cause] || e.From.World == nil || e.To.World == nil {
		return
	}
	teleporter, queued := c.teleportQueue[p.Name()]
	if !queued {
		if !c.Config.Bool("teleport.teleport-intercept") {
			return
		}
		teleporter = p
	}
	to, ok := c.Worlds.Of(e.To.World)
	if !ok || !to.Loaded() || e.From.World.Key() == e.To.World.Key() {
		return
	}
	from, _ := c.Worlds.Of(e.From.World)
	pay, err := c.EntryFor(teleporter).CanEnter(from, to)
	if err != nil {
		e.SetCancelled(true)
		c.Issuer(teleporter).Error(ErrorMessage(err))
		return
	}
	if payer, ok := teleporter.(plugin.Player); ok && pay {
		c.PayEntryFee(payer, to)
	}
}

// onPortal refuses a player's portal trip into a world they may not enter.
// MV: MVPlayerListener.playerPortal
func (c *Core) onPortal(e *plugin.EntityPortalEvent) {
	p, ok := e.Entity.(plugin.Player)
	if !ok || e.From.World == nil || e.To.World == nil || e.From.World.Key() == e.To.World.Key() {
		return
	}
	to, ok := c.Worlds.Of(e.To.World)
	if !ok || !to.Loaded() {
		return
	}
	from, _ := c.Worlds.Of(e.From.World)
	if _, err := c.EntryFor(p).CanEnter(from, to); err != nil {
		e.SetCancelled(true)
		c.Issuer(p).Error(ErrorMessage(err))
	}
}
