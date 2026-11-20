package inventories

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// lastLocationType is "ll:<world>": where the player last was in a world,
// else its spawn. MV: LastLocationDestination
type lastLocationType struct{ m *Module }

func (*lastLocationType) ID() string { return "ll" }

func (t *lastLocationType) Parse(_ plugin.CommandSender, params string) (core.Destination, error) {
	w, ok := t.m.Core.Worlds.GetLoaded(params)
	if !ok {
		return nil, core.MsgError(core.M("mv-core.destination.shared.failurereason.worldnotfound", "{world}", params))
	}
	return &lastLocation{typ: t, world: w, name: params}, nil
}

func (t *lastLocationType) Suggest(plugin.CommandSender, string) []core.Suggestion {
	var out []core.Suggestion
	for _, w := range t.m.Core.Worlds.Loaded() {
		out = append(out, core.Suggestion{Type: t, Text: w.Name(), PermSuffix: w.Name()})
	}
	return out
}

type lastLocation struct {
	typ   *lastLocationType
	world *core.World
	// name is the world as given, which the permission suffix and the
	// destination string keep.
	name string
}

func (d *lastLocation) Type() core.DestinationType { return d.typ }

// Location answers the player's last location in the world, read from the
// group sharing it with no other world the player is in, else from the
// world's profile; the world's spawn when there is none, when the player is
// already in the world, or when a group disables last_location.
// MV: LastLocationDestinationInstance.getLocation
func (d *lastLocation) Location(e plugin.Entity) (plugin.Location, bool) {
	m := d.typ.m
	spawn := func() (plugin.Location, bool) {
		if _, ok := d.world.Handle(); !ok {
			return plugin.Location{}, false
		}
		return d.world.Spawn()
	}
	p, ok := e.(plugin.Player)
	if !ok {
		return spawn()
	}
	world, playerWorld := d.world.Name(), m.worldName(p.World())
	if playerWorld == world {
		return spawn()
	}
	stored := func(t ContainerType, name string) (plugin.Location, bool) {
		if loc, ok := m.store.Profile(key(t, name, m.profileType(p), p)).Get(LastLocation).(*Loc); ok {
			if to, ok := m.locationOf(loc); ok {
				return to, true
			}
		}
		return spawn()
	}
	for _, g := range m.Groups.ForWorld(world) {
		if !g.ContainsWorld(playerWorld) && g.applicableShares.Has(LastLocation) {
			return stored(ContainerGroup, g.name)
		}
		if g.disabled.Has(LastLocation) {
			return spawn()
		}
	}
	return stored(ContainerWorld, world)
}

func (*lastLocation) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*lastLocation) CheckSafety() bool                          { return false }
func (d *lastLocation) PermissionSuffix() (string, bool)         { return d.name, true }
func (d *lastLocation) Display() core.Message                    { return core.Lit(core.DestinationString(d)) }
func (d *lastLocation) Serialise() string                        { return d.name }
