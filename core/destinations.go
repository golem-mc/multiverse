package core

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Destination is a parsed destination: where an entity is sent.
// MV: DestinationInstance
type Destination interface {
	// Type answers the destination type that parsed it.
	Type() DestinationType
	// Location answers where teleportee goes; false when there is nowhere
	// (an unloaded world, an offline player).
	Location(teleportee plugin.Entity) (plugin.Location, bool)
	// Velocity answers the velocity teleportee leaves with, false for none.
	Velocity(teleportee plugin.Entity) (plugin.Vec3, bool)
	// CheckSafety reports whether the teleport looks for a safe spot.
	CheckSafety() bool
	// PermissionSuffix answers what follows the type in the finer teleport
	// permission (multiverse.teleport.self.w.<suffix>), false for none.
	PermissionSuffix() (string, bool)
	// Display answers how messages name the destination.
	Display() Message
	// Serialise answers the destination string after "<id>:".
	Serialise() string
}

// DestinationString answers the full destination string: "<id>:<rest>".
// MV: DestinationInstance.toString
func DestinationString(d Destination) string { return d.Type().ID() + ":" + d.Serialise() }

// DestinationType parses one kind of destination. MV: Destination
type DestinationType interface {
	// ID is the prefix before the colon: "w", "e", "pl", "a", "b", "ca".
	ID() string
	// Parse reads the part after "<id>:".
	Parse(sender plugin.CommandSender, params string) (Destination, error)
	// Suggest answers the destination strings sender may complete, with
	// their "<id>:" prefix ("" prefix for a bare world name).
	Suggest(sender plugin.CommandSender, params string) []string
}

// Destinations is the destination registry. MV: DestinationsProvider
type Destinations struct {
	core  *Core
	types []DestinationType
}

func newDestinations(c *Core) *Destinations {
	d := &Destinations{core: c}
	d.Register(&WorldDestinationType{core: c})
	return d
}

// Register adds a destination type, as a module adds its own (Portals' p:).
func (d *Destinations) Register(t DestinationType) {
	d.types = slices.DeleteFunc(d.types, func(x DestinationType) bool { return x.ID() == t.ID() })
	d.types = append(d.types, t)
	if c := d.core; c.Permissions != nil {
		c.Permissions.Register(Concat(PermTeleport, "self", t.ID()), DefaultOp)
		c.Permissions.Register(Concat(PermTeleport, "other", t.ID()), DefaultOp)
	}
}

// Types answers the registered destination types.
func (d *Destinations) Types() []DestinationType { return slices.Clone(d.types) }

// Type answers the destination type with id.
func (d *Destinations) Type(id string) (DestinationType, bool) {
	for _, t := range d.types {
		if t.ID() == id {
			return t, true
		}
	}
	return nil, false
}

// Parse reads a destination string; without a colon it is a world.
// MV: DestinationsProvider.parseDestination
func (d *Destinations) Parse(sender plugin.CommandSender, s string) (Destination, error) {
	id, params, ok := strings.Cut(s, ":")
	if !ok {
		id, params = "w", s
	}
	t, found := d.Type(id)
	if !found {
		ids := make([]string, len(d.types))
		for i, t := range d.types {
			ids[i] = t.ID()
		}
		return nil, MsgError(M("mv-core.destination.parse.failurereason.invaliddestinationid",
			"{id}", id, "{ids}", strings.Join(ids, ", ")))
	}
	return t.Parse(sender, params)
}

// Suggest answers every destination string sender may complete.
// MV: DestinationsProvider.suggestDestinationStrings
func (d *Destinations) Suggest(sender plugin.CommandSender, params string) []string {
	var out []string
	for _, t := range d.types {
		out = append(out, t.Suggest(sender, params)...)
	}
	return out
}

// WorldDestinationType is "w:<world>[:<direction>]", or a bare world name:
// the world's spawn. MV: WorldDestination
type WorldDestinationType struct{ core *Core }

func (*WorldDestinationType) ID() string { return "w" }

func (t *WorldDestinationType) Parse(_ plugin.CommandSender, params string) (Destination, error) {
	parts := strings.SplitN(params, ":", 3)
	w, ok := t.core.Worlds.GetByNameOrAlias(parts[0])
	if !ok {
		return nil, MsgError(M("mv-core.destination.shared.failurereason.worldnotfound", "{world}", parts[0]))
	}
	d := &WorldDestination{typ: t, World: w, Yaw: -1}
	if len(parts) == 2 {
		d.Direction = parts[1]
		d.Yaw = YawOf(parts[1])
	}
	return d, nil
}

func (t *WorldDestinationType) Suggest(sender plugin.CommandSender, _ string) []string {
	var out []string
	for _, w := range t.core.Worlds.Loaded() {
		if t.core.CanAccess(sender, w) {
			out = append(out, w.TabCompleteName())
		}
	}
	return out
}

// WorldDestination is a world's spawn, optionally facing Direction.
type WorldDestination struct {
	typ       *WorldDestinationType
	World     *World
	Direction string
	// Yaw replaces the spawn's yaw when not negative.
	Yaw float32
}

// WorldDestinationOf answers the destination of w's spawn.
func (c *Core) WorldDestinationOf(w *World) *WorldDestination {
	t, _ := c.Destinations.Type("w")
	return &WorldDestination{typ: t.(*WorldDestinationType), World: w, Yaw: -1}
}

func (d *WorldDestination) Type() DestinationType { return d.typ }

func (d *WorldDestination) Location(plugin.Entity) (plugin.Location, bool) {
	loc, ok := d.World.Spawn()
	if ok && d.Yaw >= 0 {
		loc.Yaw = d.Yaw
	}
	return loc, ok
}

func (*WorldDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*WorldDestination) CheckSafety() bool                          { return true }
func (d *WorldDestination) PermissionSuffix() (string, bool)         { return d.World.Name(), true }
func (d *WorldDestination) Display() Message                         { return Lit(d.World.AliasOrName()) }

func (d *WorldDestination) Serialise() string {
	if d.Direction != "" {
		return d.World.Name() + ":" + d.Direction
	}
	return d.World.Name()
}

// YawOf reads a direction: a compass word (n, ne, e, ..., or north, east,
// ...); 0 for anything else. MV: LocationManipulation.getYaw
func YawOf(direction string) float32 {
	switch strings.ToLower(direction) {
	case "n", "north":
		return 180
	case "ne", "northeast":
		return 225
	case "e", "east":
		return 270
	case "se", "southeast":
		return 315
	case "s", "south":
		return 0
	case "sw", "southwest":
		return 45
	case "w", "west":
		return 90
	case "nw", "northwest":
		return 135
	}
	return 0
}
