package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, portalForms) }

// portalForms stops nether and end portals forming in worlds whose
// portal-form forbids them, and sets the exit portal search radius. MV:
// MVPortalListener
func portalForms(c *core.Core) {
	opts := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(c.Server, func(e *plugin.EntityPortalEvent) {
		if e.To.World != nil && c.Config.Bool("portal.use-custom-portal-search") {
			e.SearchRadius = c.Config.Int("portal.custom-portal-search-radius")
		}
	}, opts)
	plugin.Listen(c.Server, func(e *plugin.PortalCreateEvent) {
		w, ok := c.Worlds.Of(e.World)
		if !ok {
			return
		}
		kind := portalKind(e)
		if kind == "" || portalAllowed(w.Props.String(core.PropPortalForm), kind) {
			return
		}
		c.Debug(1, "Cancelling creation of %s portal because portalForm disallows.", kind)
		e.SetCancelled(true)
		if p, ok := e.Entity.(plugin.Player); ok {
			c.Issuer(p).Error(core.M("mv-core.portalform.disabled.nether"))
		}
	}, opts)
	// An eye of ender put into a frame starts an end portal.
	plugin.Listen(c.Server, func(e *plugin.PlayerInteractEvent) {
		if e.Action != plugin.RightClickBlock || e.Block == nil || e.Block.State().Block() != registry.BlockEndPortalFrame ||
			e.Item.Item != registry.ItemEnderEye {
			return
		}
		w, ok := c.Worlds.Of(e.Player.World())
		if !ok || portalAllowed(w.Props.String(core.PropPortalForm), "end") {
			return
		}
		c.Debug(1, "Cancelling creation of ENDER portal because portalForm disallows.")
		e.SetCancelled(true)
		c.Issuer(e.Player).Error(core.M("mv-core.portalform.disabled.end"))
	}, opts)
}

// portalKind answers "nether" or "end" for the portal being made, "" for a
// fire that lit no portal. MV: MVPortalListener.getPortalType
func portalKind(e *plugin.PortalCreateEvent) string {
	switch e.Reason {
	case plugin.PortalFire:
		for _, b := range e.Blocks {
			if b.State.Block() == registry.BlockNetherPortal {
				return "nether"
			}
		}
	case plugin.PortalNetherPair:
		return "nether"
	case plugin.PortalEndPlatform:
		return "end"
	}
	return ""
}

// portalAllowed reports whether a portal-form allows a portal of kind.
// MV: AllowedPortalType.isPortalAllowed
func portalAllowed(form, kind string) bool {
	return form == "all" || form == kind
}
