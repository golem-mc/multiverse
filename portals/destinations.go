package portals

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// portalDestinationType is "p:<portal>[:<direction>]": the middle of a
// portal. MV: PortalDestination
type portalDestinationType struct{ p *Portals }

func (*portalDestinationType) ID() string { return "p" }

func (t *portalDestinationType) Parse(_ plugin.CommandSender, params string) (core.Destination, error) {
	items := strings.Split(params, ":")
	if len(items) > 3 {
		return nil, core.MsgError(core.M("mv-portals.destination.portal.invalidformat"))
	}
	pt, ok := t.p.Manager.Get(items[0])
	if !ok {
		return nil, core.MsgError(core.M("mv-portals.destination.portal.notfound", "{name}", items[0]))
	}
	d := &portalDestination{typ: t, portal: pt, yaw: -1}
	if len(items) == 2 {
		d.direction = items[1]
		d.yaw = core.YawOf(items[1])
	}
	return d, nil
}

func (t *portalDestinationType) Suggest(plugin.CommandSender, string) []core.Suggestion {
	var out []core.Suggestion
	for _, pt := range t.p.Manager.All() {
		out = append(out, core.Suggestion{Type: t, Text: pt.Name(), PermSuffix: pt.Name()})
	}
	return out
}

type portalDestination struct {
	typ       *portalDestinationType
	portal    *Portal
	direction string
	yaw       float32
}

func (d *portalDestination) Type() core.DestinationType { return d.typ }

// Location answers the portal's spawn point facing the direction given; it
// faces yaw -1 without one, as Multiverse sets it.
// MV: PortalDestinationInstance.getLocation
func (d *portalDestination) Location(plugin.Entity) (plugin.Location, bool) {
	loc, ok := d.portal.SafeSpawn()
	loc.Yaw = d.yaw
	return loc, ok
}

func (*portalDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (d *portalDestination) CheckSafety() bool                        { return d.portal.SafeTeleport() }
func (d *portalDestination) PermissionSuffix() (string, bool)         { return d.portal.Name(), true }
func (d *portalDestination) Display() core.Message                    { return core.Lit(core.DestinationString(d)) }

func (d *portalDestination) Serialise() string {
	if d.direction != "" {
		return d.portal.Name() + ":" + d.direction
	}
	return d.portal.Name()
}

// randomPortalDestinationType is "rp:<portal>,<portal>,...": one of the
// portals, at random; every portal with none named.
// MV: RandomPortalDestination
type randomPortalDestinationType struct{ p *Portals }

func (*randomPortalDestinationType) ID() string { return "rp" }

func (t *randomPortalDestinationType) Parse(_ plugin.CommandSender, params string) (core.Destination, error) {
	var names []string
	if params != "" {
		names = strings.Split(params, ",")
	}
	return &randomPortalDestination{typ: t, names: names}, nil
}

// Suggest completes the next name of the comma-separated list.
// MV: StringFormatter.addOnToCommaSeparated
func (t *randomPortalDestinationType) Suggest(_ plugin.CommandSender, input string) []core.Suggestion {
	input = strings.TrimPrefix(input, "rp:")
	previous := input[:strings.LastIndexByte(input, ',')+1]
	typed := strings.Split(input, ",")
	var out []core.Suggestion
	for _, pt := range t.p.Manager.All() {
		if input == "" || !slices.Contains(typed, pt.Name()) {
			out = append(out, core.Suggestion{Type: t, Text: previous + pt.Name()})
		}
	}
	return out
}

type randomPortalDestination struct {
	typ   *randomPortalDestinationType
	names []string
}

func (d *randomPortalDestination) Type() core.DestinationType { return d.typ }

// Location answers the spawn point of a portal picked at random.
// MV: RandomPortalDestinationInstance.getLocation
func (d *randomPortalDestination) Location(plugin.Entity) (plugin.Location, bool) {
	names := d.names
	if len(names) == 0 {
		for _, pt := range d.typ.p.Manager.All() {
			names = append(names, pt.Name())
		}
	}
	if len(names) == 0 {
		return plugin.Location{}, false
	}
	pt, ok := d.typ.p.Manager.Get(names[rand.IntN(len(names))])
	if !ok {
		return plugin.Location{}, false
	}
	return pt.SafeSpawn()
}

func (*randomPortalDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) {
	return plugin.Vec3{}, false
}
func (*randomPortalDestination) CheckSafety() bool                { return true }
func (*randomPortalDestination) PermissionSuffix() (string, bool) { return "", false }
func (d *randomPortalDestination) Display() core.Message          { return core.Lit(core.DestinationString(d)) }
func (d *randomPortalDestination) Serialise() string              { return strings.Join(d.names, ",") }
