package portals

import (
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// showInfo shows a portal under header: its box, action, switches and
// price, and what its action does for the sender. MV: DisplayUtils.showStaticInfo
func (p *Portals) showInfo(sender plugin.CommandSender, pt *Portal, header core.Message) {
	is := p.Core.Issuer(sender)
	is.Info(header)
	loc := strings.Split(pt.Props.String(propLocation), ":")
	for len(loc) < 3 {
		loc = append(loc, "")
	}
	var world any = core.M("mv-portals.portalinfo.world.error")
	if w, ok := pt.World(); ok {
		world = w.Name()
	}
	is.Info(core.M("mv-portals.portalinfo.coordinates", "{fromLocation}", loc[1], "{toLocation}", loc[2], "{world}", world))
	is.Info(core.M("mv-portals.portalinfo.actiontype", "{actionType}", pt.ActionType()))
	if pt.Action() == "" {
		is.Info(core.M("mv-portals.portalinfo.action.notset"))
	} else {
		is.Info(core.M("mv-portals.portalinfo.action", "{action}", pt.Action()))
	}
	action, err := pt.ActionHandler(p.Server.Console())
	is.Info(core.M("mv-portals.portalinfo.checkdestinationsafety", "{value}", boolMessage(pt.CheckDestinationSafety())))
	is.Info(core.M("mv-portals.portalinfo.teleportnonplayers", "{value}", boolMessage(pt.TeleportNonPlayers())))
	p.showPrice(is, pt)
	if e, ok := sender.(plugin.Entity); ok && err == nil {
		is.Info(core.M("mv-portals.portalinfo.playeraction", "{action}", action.Description(e)))
	}
	if err != nil {
		is.Error(core.M("mv-portals.portalinfo.actionerror", "{error}", core.ErrorMessage(err)))
	}
}

func boolMessage(b bool) core.Message {
	if b {
		return core.M("mv-portals.generic.true")
	}
	return core.M("mv-portals.generic.false")
}

// showPrice tells the portal's entry fee. MV: DisplayUtils.showPortalPriceInfo
func (p *Portals) showPrice(is *core.Issuer, pt *Portal) {
	switch price := pt.Price(); {
	case price > 0:
		is.Info(core.M("mv-portals.portalinfo.price", "{value}", p.Core.FormatPrice(price, pt.Currency())))
	case price < 0:
		is.Info(core.M("mv-portals.portalinfo.price", "{value}", p.Core.FormatPrice(-price, pt.Currency())))
	default:
		is.Info(core.M("mv-portals.portalinfo.price.free"))
	}
}

// destinationMessage describes a portal's action as a Multiverse
// destination, for /mvp list. MV: DisplayUtils.formatActionAsMVDestinationMessage
func (p *Portals) destinationMessage(pt *Portal) core.Message {
	action := pt.Action()
	typ, dest, _ := strings.Cut(action, ":")
	switch typ {
	case "w":
		if _, ok := p.Core.Worlds.Get(dest); ok {
			return core.M("mv-portals.destination.world", "{destination}", dest)
		}
	case "e":
		parts := strings.Split(action, ":")
		if len(parts) < 3 {
			return core.M("mv-portals.destination.invalidlocation")
		}
		xyz := strings.Split(parts[2], ",")
		var ints []string
		for i := range 3 {
			if i >= len(xyz) {
				return core.M("mv-portals.destination.invalidlocation")
			}
			f, err := strconv.ParseFloat(xyz[i], 64)
			if err != nil {
				return core.M("mv-portals.destination.invalidlocation")
			}
			ints = append(ints, strconv.Itoa(int(f)))
		}
		return core.M("mv-portals.destination.location", "{world}", parts[1], "{location}", strings.Join(ints, ", "))
	case "i":
		return core.M("mv-portals.destination.invalid")
	}
	return core.M("mv-portals.destination.raw", "{destination}", action)
}
