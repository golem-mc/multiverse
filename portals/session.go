package portals

import (
	"fmt"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// Session is what Portals remembers of a player while they are online:
// their wand selection, selected portal, debug mode and where they stand.
// MV: PortalPlayerSession
type Session struct {
	p      *Portals
	player plugin.Player
	is     *core.Issuer

	selected *Portal
	debug    bool

	// left and right are the wand's corners and the worlds they are in.
	left, right           *plugin.BlockPos
	leftWorld, rightWorld *core.World

	// loc is the player's last block, standingIn the portal there.
	loc        plugin.Location
	standingIn *Portal
	// stale says the last move stayed in loc's block.
	stale bool
	// movedOut says the player has left the portal they last used, so
	// standing in one sends them.
	movedOut     bool
	lastTeleport time.Time
}

// Session answers p's session, making one on first use.
// MV: MultiversePortals.getPortalSession
func (p *Portals) Session(pl plugin.Player) *Session {
	s, ok := p.sessions[pl.UUID()]
	if !ok {
		s = &Session{p: p, player: pl, is: p.Core.Issuer(pl), movedOut: true,
			lastTeleport: time.Now().Add(-p.cooldown())}
		p.sessions[pl.UUID()] = s
		s.setLocation(pl.Location())
	}
	s.player = pl
	return s
}

// Select makes pt the selected portal.
func (s *Session) Select(pt *Portal) { s.selected = pt }

// Selected answers the selected portal, nil for none.
func (s *Session) Selected() *Portal { return s.selected }

// Debug reports whether debug mode is on.
func (s *Session) Debug() bool { return s.debug }

// SetDebug turns debug mode on or off and tells the player.
// MV: PortalPlayerSession.setDebugMode
func (s *Session) SetDebug(on bool) {
	s.debug = on
	if on {
		s.is.Info(core.M("mv-portals.debug.enabled"))
	} else {
		s.is.Info(core.M("mv-portals.debug.disabled"))
	}
}

// SetLeft and SetRight set a wand corner and tell the player; false when
// the wand is disabled. MV: PortalPlayerSession.setLeftClickSelection
func (s *Session) SetLeft(pos plugin.BlockPos, w *core.World) bool {
	return s.setCorner(&s.left, &s.leftWorld, s.right, s.rightWorld, pos, w, "mv-portals.selection.first")
}

func (s *Session) SetRight(pos plugin.BlockPos, w *core.World) bool {
	return s.setCorner(&s.right, &s.rightWorld, s.left, s.leftWorld, pos, w, "mv-portals.selection.second")
}

func (s *Session) setCorner(corner **plugin.BlockPos, world **core.World, other *plugin.BlockPos, otherWorld *core.World,
	pos plugin.BlockPos, w *core.World, key string) bool {
	if !s.p.wandEnabled {
		return false
	}
	*corner, *world = &pos, w
	position := fmt.Sprintf("(%d, %d, %d)", pos.X, pos.Y, pos.Z)
	if other != nil && otherWorld == w {
		r := NewRegion(w, blockVec(pos), blockVec(*other))
		s.is.Info(core.M(key+".area", "{position}", position, "{count}", r.Area()))
	} else {
		s.is.Info(core.M(key, "{position}", position))
	}
	return true
}

func blockVec(b plugin.BlockPos) plugin.Vec3 {
	return plugin.Vec3{X: float64(b.X), Y: float64(b.Y), Z: float64(b.Z)}
}

// SelectedRegion answers the box between the wand's corners, telling the
// player what is missing when there is none. MV: PortalPlayerSession.getSelectedRegion
func (s *Session) SelectedRegion() (Region, bool) {
	switch {
	case s.left == nil:
		s.is.Error(core.M("mv-portals.selection.left.required"))
	case s.right == nil:
		s.is.Error(core.M("mv-portals.selection.right.required"))
	case s.leftWorld != s.rightWorld:
		s.is.Error(core.M("mv-portals.selection.sameworld.required",
			"{leftWorld}", aliasOf(s.leftWorld), "{rightWorld}", aliasOf(s.rightWorld)))
	default:
		return NewRegion(s.leftWorld, blockVec(*s.left), blockVec(*s.right)), true
	}
	return Region{}, false
}

// hasSelection reports whether both wand corners are set in one world.
func (s *Session) hasSelection() bool {
	return s.left != nil && s.right != nil && s.leftWorld == s.rightWorld
}

func aliasOf(w *core.World) string {
	if w == nil {
		return "null"
	}
	return w.AliasOrName()
}

// ShowInfo shows pt to the player when they may enter it, and logs that
// they looked. MV: PortalPlayerSession.showDebugInfo(MVPortal)
func (s *Session) ShowInfo(pt *Portal) {
	if pt.CanEnter(s.player) {
		s.p.showInfo(s.player, pt, core.M("mv-portals.portalinfo.header", "{name}", pt.Name()))
	}
	s.p.Log.Info("Player " + s.player.Name() + " walked through" + pt.Name() + " with debug on.")
}
