package portals

import (
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// MVPortalEvent fires before a player is sent through a portal by walking
// into it; cancelling stops them. MV: MVPortalEvent
type MVPortalEvent struct {
	plugin.Cancellable
	// Destination is the portal's action read as a destination, nil when it
	// is not one.
	Destination core.Destination
	Teleportee  plugin.Player
	Portal      *Portal
}

// moveKind says which listener saw a move. MV: MoveType
type moveKind uint8

const (
	playerMove moveKind = iota
	vehicleMove
)

// cooldown answers portal-cooldown.
func (p *Portals) cooldown() time.Duration {
	return time.Duration(p.Config.Int("portal-usage.portal-cooldown")) * time.Millisecond
}

// letVehicleHandle reports whether the player rides a vehicle that is not a
// mob, whose own moves decide. MV: PortalPlayerSession.shouldLetVehicleHandle
func (s *Session) letVehicleHandle() bool {
	v, ok := s.player.Vehicle()
	if !ok {
		return false
	}
	_, living := v.(plugin.LivingEntity)
	return !living
}

// setLocation records the player's block and the portal there: stepping
// into a portal from outside keeps movedOut, a step inside one clears it.
// MV: PortalPlayerSession.setStandingInLocation
func (s *Session) setLocation(loc plugin.Location) {
	s.loc = loc
	pt, in := s.p.Manager.At(loc)
	switch {
	case !in:
		s.movedOut, s.standingIn = true, nil
	case s.standingIn == nil:
		s.standingIn = pt
	default:
		s.movedOut = false
	}
}

// moved records a move to loc; one within the last block is stale.
// MV: PortalPlayerSession.setStaleLocation
func (s *Session) moved(loc plugin.Location, kind moveKind) {
	if s.letVehicleHandle() && kind != vehicleMove {
		return
	}
	if sameBlock(s.loc, loc) {
		s.stale = true
		return
	}
	s.setLocation(loc)
	s.stale = false
}

func sameBlock(a, b plugin.Location) bool {
	return a.World != nil && b.World != nil && a.World.Key() == b.World.Key() && a.Pos.Block() == b.Pos.Block()
}

// shouldTeleport reports whether the portal the player stands in sends
// them. MV: PortalPlayerSession.doTeleportPlayer
func (s *Session) shouldTeleport(kind moveKind) bool {
	if kind == playerMove && s.letVehicleHandle() {
		return false
	}
	return s.movedOut && s.standingIn != nil
}

// didTeleport notes a teleport to to: arriving in a portal does not use it.
// MV: PortalPlayerSession.playerDidTeleport
func (s *Session) didTeleport(to plugin.Location) {
	_, in := s.p.Manager.At(to)
	s.movedOut = !in
}

// showDebug shows the portal the player stands in when debug mode is on,
// reporting whether it did. MV: PortalPlayerSession.showDebugInfo
func (s *Session) showDebug() bool {
	if !s.debug || s.standingIn == nil {
		return false
	}
	s.p.showInfo(s.player, s.standingIn, core.M("mv-portals.portalinfo.debugheader", "{name}", s.standingIn.Name()))
	return true
}

// coolingDown tells the player and reports true while the portal cooldown
// runs. MV: PortalPlayerSession.checkAndSendCooldownMessage
func (s *Session) coolingDown() bool {
	left := time.Until(s.lastTeleport.Add(s.p.cooldown()))
	if left <= 0 {
		return false
	}
	secs := "1s"
	if left >= time.Second {
		secs = strconv.FormatInt(left.Milliseconds()/1000, 10) + "s"
	}
	s.is.Error(core.M("mv-portals.portal.cooldown", "{cooldown}", secs))
	return true
}

// listenTravel registers the listeners that send players and entities
// through portals and keep portal blocks in place. MV: MVPPlayerMoveListener,
// MVPPlayerPortalListener, MVPEntityPortalListener, MVPBlockListener,
// MVPPlayerListener
func (p *Portals) listenTravel() {
	s := p.Server
	plugin.Listen(s, p.onMove, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, p.onPortal, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
	plugin.Listen(s, func(e *plugin.EntityTeleportEvent) {
		if pl, ok := e.Entity.(plugin.Player); ok {
			p.Session(pl).didTeleport(e.To)
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	plugin.Listen(s, p.onPhysics, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
	plugin.Listen(s, p.onFlow, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, p.onBucket, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
}

// onMove sends a player who walks into a portal, unless a nether portal
// fill with nether-animation leaves it to the portal trip.
// MV: MVPPlayerMoveListener.playerMove
func (p *Portals) onMove(e *plugin.PlayerMoveEvent) {
	if !p.Config.Bool("portal-usage.use-on-move") {
		return
	}
	pl := e.Player
	ps := p.Session(pl)
	ps.moved(e.To, playerMove)
	if ps.stale {
		return
	}
	pt := ps.standingIn
	if pt == nil || p.Config.Bool("portal-usage.nether-animation") && !pt.Legacy() ||
		!ps.shouldTeleport(playerMove) || ps.showDebug() {
		return
	}
	pl.SetFallDistance(0)
	if !pt.FrameValid(e.To.Pos.Block()) {
		ps.is.Error(core.M("mv-portals.portal.frame.invalid"))
		return
	}
	if ps.coolingDown() {
		return
	}
	p.usePortal(pt, pl)
}

// usePortal checks and charges player for pt, fires MVPortalEvent and runs
// the portal's action, answering whether it ran.
func (p *Portals) usePortal(pt *Portal, pl plugin.Player) bool {
	canUse, pay := p.checkUse(pt, pl)
	if !canUse {
		return false
	}
	ev := &MVPortalEvent{Teleportee: pl, Portal: pt}
	if pt.ActionType() == (&destinationActionType{}).Name() {
		ev.Destination, _ = p.Core.Destinations.Parse(p.Server.Console(), pt.Action())
	}
	p.Server.Events().Fire(ev)
	if ev.Cancelled() {
		return false
	}
	if pay {
		p.Core.PayFee(pl, pt.Price(), feeCurrency(pt))
	}
	p.Core.Debug(1, "MV-Portals is allowing Player '%s' to use the portal '%s'.", pl.Name(), pt.Name())
	if p.runAction(pt, pl) != nil {
		return false
	}
	p.sendPortalMessage(pl, pt, pt.Props.String(propSuccessMsg), "mv-portals.portal.action.success")
	return true
}

// feeCurrency answers how core reads the portal's currency: air is the
// economy. MV: MVEconomist.VAULT_ECONOMY_MATERIAL
func feeCurrency(pt *Portal) string {
	if c := pt.Currency(); c != "air" && c != "" {
		return c
	}
	return core.VaultEconomyCode
}

// checkUse checks portal access and the entry fee: canUse, and whether a
// fee is due. MV: PortalListenerHelper.checkPlayerCanUsePortal
func (p *Portals) checkUse(pt *Portal, pl plugin.Player) (canUse, pay bool) {
	if p.Config.Bool("portal-usage.enforce-portal-access") && !pt.CanEnter(pl) {
		if p.Config.Bool("portal-usage.send-no-permission-messages") {
			p.sendPortalMessage(pl, pt, pt.Props.String(propNoPermission), "mv-portals.portal.permission.denied")
		}
		p.Core.Debug(1, "MV-Portals is DENYING Player '%s' access to use the portal '%s'.", pl.Name(), pt.Name())
		return false, false
	}
	price := pt.Price()
	if price == 0 || pt.Exempt(pl) {
		return true, false
	}
	cur := feeCurrency(pt)
	if price > 0 && !p.Core.HasFunds(pl, price, cur) {
		kind := "items"
		if cur == core.VaultEconomyCode {
			kind = "funds"
		}
		is := p.Core.Issuer(pl)
		is.SendRaw("Sorry, you don't have enough " + kind + ". " + is.Format(core.M("mv-portals.portal.insufficientfunds",
			"{price}", p.Core.FormatPrice(price, cur), "{name}", pt.Name())))
		return false, false
	}
	return true, true
}

// sendPortalMessage sends a portal's configured message: "@disabled" none,
// "@default" defaultKey, "@@<key>" that key, anything else as written.
// MV: MVPortal.getMessageEither, PortalListenerHelper.sendEitherMessage
func (p *Portals) sendPortalMessage(to plugin.CommandSender, pt *Portal, msg, defaultKey string) {
	var m core.Message
	switch {
	case msg == "" || strings.EqualFold(msg, "@disabled"):
		return
	case strings.EqualFold(msg, "@default"):
		m = core.M(defaultKey)
	case strings.HasPrefix(msg, "@@"):
		m = core.M(msg[2:])
	default:
		m = core.Lit(msg)
	}
	m.Repl = append(m.Repl, "{player}", to.Name(), "{portal}", pt.Name())
	p.Core.Issuer(to).Send(m)
}

// runAction runs pt's action for e, starting a player's cooldown.
// MV: MVPortal.runActionFor
func (p *Portals) runAction(pt *Portal, e plugin.Entity) error {
	sender := p.Server.Console()
	pl, isPlayer := e.(plugin.Player)
	if isPlayer {
		sender = pl
	}
	a, err := pt.ActionHandler(sender)
	if err == nil {
		err = a.Run(pt, e)
	}
	if err != nil {
		p.Log.Warn(core.StripColor("Invalid Portal Action: " + p.Core.Locales.Format(p.Server.Console(), core.ErrorMessage(err))))
		return err
	}
	if isPlayer {
		p.Session(pl).lastTeleport = time.Now()
	}
	return nil
}

// onPortal takes over a portal trip through a nether portal fill: the
// portal's action runs instead, unless portals-default-to-nether lets the
// trip go on when the portal cannot be used.
// MV: MVPPlayerPortalListener.playerPortal, MVPEntityPortalListener.entityPortal
func (p *Portals) onPortal(e *plugin.EntityPortalEvent) {
	at := e.Entity.Location()
	pt, ok := p.Manager.At(at)
	if !ok {
		if next, found := portalBlockNextTo(e.From); found {
			at = next
			pt, ok = p.Manager.At(next)
		}
	}
	if !ok {
		return
	}
	toNether := p.Config.Bool("portal-usage.portals-default-to-nether")
	pl, isPlayer := e.Entity.(plugin.Player)
	if !isPlayer {
		if !p.Config.Bool("portal-usage.teleport-entities") {
			return
		}
		if !pt.TeleportNonPlayers() {
			e.SetCancelled(!toNether)
			return
		}
		if p.runAction(pt, e.Entity) == nil {
			e.SetCancelled(true)
		}
		return
	}
	if !toNether {
		e.SetCancelled(true)
	}
	if !pt.FrameValid(at.Pos.Block()) {
		p.Core.Issuer(pl).Error(core.M("mv-portals.portal.frame.invalid"))
		e.SetCancelled(true)
		return
	}
	ps := p.Session(pl)
	ps.moved(at, playerMove)
	if ps.showDebug() {
		e.SetCancelled(true)
		return
	}
	if ps.coolingDown() {
		return
	}
	if p.usePortal(pt, pl) {
		e.SetCancelled(true)
	}
}

// portalBlockNextTo answers loc when it is in a nether portal block, else
// the nearest of the four blocks around it that is one.
// MV: BlockSafety.findPortalBlockNextTo
func portalBlockNextTo(loc plugin.Location) (plugin.Location, bool) {
	if loc.World == nil {
		return plugin.Location{}, false
	}
	b := loc.Pos.Block()
	if loc.World.BlockState(b).Block() == registry.BlockNetherPortal {
		return loc, true
	}
	var found plugin.Location
	ok := false
	for _, f := range []plugin.BlockFace{plugin.North, plugin.South, plugin.East, plugin.West} {
		n := b.Offset(f, 1)
		if loc.World.BlockState(n).Block() != registry.BlockNetherPortal {
			continue
		}
		cand := plugin.Location{World: loc.World, Pos: plugin.Vec3{X: float64(n.X), Y: float64(n.Y), Z: float64(n.Z)}}
		if !ok || dist2(loc.Pos, cand.Pos) < dist2(loc.Pos, found.Pos) {
			found, ok = cand, true
		}
	}
	return found, ok
}

func dist2(a, b plugin.Vec3) float64 {
	dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return dx*dx + dy*dy + dz*dz
}

// blockLocation answers the location of a block.
func blockLocation(b plugin.Block) plugin.Location {
	return plugin.Location{World: b.World, Pos: b.Pos.Center()}
}

// onPhysics keeps the nether portal blocks of a portal from breaking for
// want of a frame. MV: MVPBlockListener.blockPhysics
func (p *Portals) onPhysics(e *plugin.BlockPhysicsEvent) {
	if e.State.Block() != registry.BlockNetherPortal && e.Source.State().Block() != registry.BlockNetherPortal {
		return
	}
	if _, ok := p.Manager.At(blockLocation(e.Block)); ok {
		e.SetCancelled(true)
	}
}

// onFlow keeps fluids from flowing into a portal, and with bucket-filling
// out of one. MV: MVPBlockListener.blockFromTo
func (p *Portals) onFlow(e *plugin.BlockFromToEvent) {
	if _, ok := p.Manager.At(blockLocation(e.To)); ok {
		e.SetCancelled(true)
		return
	}
	if _, ok := p.Manager.At(blockLocation(e.From)); ok && p.Config.Bool("portal-creation.bucket-filling") {
		e.SetCancelled(true)
	}
}

// portalFor answers the portal at loc, if the player may use it.
// MV: PortalManager.getPortal(Player, Location)
func (p *Portals) portalFor(pl plugin.Player, loc plugin.Location) (*Portal, bool) {
	pt, ok := p.Manager.At(loc)
	if !ok || p.Config.Bool("portal-usage.enforce-portal-access") && !pt.CanEnter(pl) {
		return nil, false
	}
	return pt, true
}

// onBucket fills a portal with the poured water or lava, or empties it of
// the scooped fluid. MV: MVPPlayerListener.playerBucketFill, playerBucketEmpty
func (p *Portals) onBucket(e *plugin.PlayerBucketEvent) {
	if !e.Fill && !p.Config.Bool("portal-creation.bucket-filling") {
		return
	}
	pt, ok := p.portalFor(e.Player, blockLocation(e.Block))
	if !ok {
		return
	}
	ps := p.Session(e.Player)
	if ps.debug {
		ps.ShowInfo(pt)
		e.SetCancelled(true)
		return
	}
	fill := registry.BlockAir
	if !e.Fill {
		if !pt.CanFill(e.Player) {
			e.SetCancelled(true)
			return
		}
		fill = registry.BlockWater
		if e.Item.Item == registry.ItemLavaBucket {
			fill = registry.BlockLava
		}
	}
	p.fill(pt, e.Block.Pos, fill, e.Player)
}

// lightPortal fills a portal with nether portal blocks, or empties one, as
// a player strikes flint and steel at it.
// MV: MVPPlayerListener.lightPortalWithFlintAndSteel
func (p *Portals) lightPortal(e *plugin.PlayerInteractEvent) {
	at := e.Block.Pos.Offset(e.Face, 1)
	loc := plugin.Location{World: e.Block.World, Pos: at.Center()}
	if _, ok := p.Manager.At(loc); !ok || !e.Player.HasPermission("multiverse.portal.create") {
		return
	}
	pt, ok := p.portalFor(e.Player, loc)
	if !ok || !pt.FrameValid(at) {
		return
	}
	ps := p.Session(e.Player)
	if ps.debug {
		ps.ShowInfo(pt)
		e.SetCancelled(true)
		return
	}
	fill := registry.BlockNetherPortal
	if e.Block.World.BlockState(at).Block() == registry.BlockNetherPortal {
		fill = registry.BlockAir
	}
	e.SetCancelled(p.fill(pt, at, fill, e.Player))
}
