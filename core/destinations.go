package core

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
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

// Suggestion is a destination tab completion with the finer permission
// suffix it needs. MV: DestinationSuggestionPacket
type Suggestion struct {
	Type DestinationType
	// Text is the part after "<id>:".
	Text       string
	PermSuffix string
}

// String answers what is typed: a world destination without its "w:".
// MV: DestinationSuggestionPacket.parsableString
func (s Suggestion) String() string {
	if s.Type.ID() == "w" {
		return s.Text
	}
	return s.Type.ID() + ":" + s.Text
}

// DestinationType parses one kind of destination. MV: Destination
type DestinationType interface {
	// ID is the prefix before the colon: "w", "e", "pl", "a", "b", "ca".
	ID() string
	// Parse reads the part after "<id>:".
	Parse(sender plugin.CommandSender, params string) (Destination, error)
	// Suggest answers the completions sender may be offered.
	Suggest(sender plugin.CommandSender, params string) []Suggestion
}

// Destinations is the destination registry. MV: DestinationsProvider
type Destinations struct {
	core  *Core
	types []DestinationType
}

func newDestinations(c *Core) *Destinations {
	d := &Destinations{core: c}
	for _, t := range []DestinationType{&anchorType{c}, &bedType{c}, &cannonType{c}, &exactType{c},
		&playerType{c}, &WorldDestinationType{c}} {
		d.Register(t)
	}
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

// SuggestAll answers every completion sender may be offered.
// MV: DestinationsProvider.suggestDestinations
func (d *Destinations) SuggestAll(sender plugin.CommandSender, params string) []Suggestion {
	var out []Suggestion
	for _, t := range d.types {
		out = append(out, t.Suggest(sender, params)...)
	}
	return out
}

// Suggest answers the destination strings sender may complete.
// MV: DestinationsProvider.suggestDestinationStrings
func (d *Destinations) Suggest(sender plugin.CommandSender, params string) []string {
	var out []string
	for _, s := range d.SuggestAll(sender, params) {
		out = append(out, s.String())
	}
	return out
}

func (c *Core) accessibleLoaded(sender plugin.CommandSender) []*World {
	var out []*World
	for _, w := range c.Worlds.Loaded() {
		if c.CanAccess(sender, w) {
			out = append(out, w)
		}
	}
	return out
}

// worldOf answers the Multiverse world named s, by alias too when
// resolve-alias-name is on. MV: WorldDestination.getMultiverseWorld
func (c *Core) worldOf(s string) (*World, bool) {
	if c.Config.Bool("command.resolve-alias-name") {
		return c.Worlds.GetByNameOrAlias(s)
	}
	return c.Worlds.Get(s)
}

// WorldDestinationType is "w:<world>[:<direction>]", or a bare world name:
// the world's spawn. MV: WorldDestination
type WorldDestinationType struct{ core *Core }

func (*WorldDestinationType) ID() string { return "w" }

func (t *WorldDestinationType) Parse(_ plugin.CommandSender, params string) (Destination, error) {
	parts := strings.SplitN(params, ":", 3)
	w, ok := t.core.worldOf(parts[0])
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

func (t *WorldDestinationType) Suggest(sender plugin.CommandSender, _ string) []Suggestion {
	var out []Suggestion
	for _, w := range t.core.accessibleLoaded(sender) {
		out = append(out, Suggestion{Type: t, Text: w.TabCompleteName(), PermSuffix: w.Name()})
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
	case "sw", "southwest":
		return 45
	case "w", "west":
		return 90
	case "nw", "northwest":
		return 135
	}
	return 0
}

// senderLocation answers where sender stands, if it is a player.
func senderLocation(sender plugin.CommandSender) (plugin.Location, bool) {
	if p, ok := sender.(plugin.Player); ok {
		return p.Location(), true
	}
	return plugin.Location{}, false
}

// exactType is "e:<world>:<x,y,z>[:pitch:yaw]", or "e:@here".
// MV: ExactDestination
type exactType struct{ core *Core }

func (*exactType) ID() string { return "e" }

func (t *exactType) Parse(sender plugin.CommandSender, params string) (Destination, error) {
	worldName, pos, ok := strings.Cut(params, ":")
	if !ok {
		if params == "@here" {
			if loc, ok := senderLocation(sender); ok {
				if w, ok := t.core.Worlds.Of(loc.World); ok {
					return &exactDestination{typ: t, world: w, pos: PositionOf(loc)}, nil
				}
			}
			return nil, MsgError(M("mv-core.destination.shared.failurereason.invalidcoordinatesformat"))
		}
		return nil, MsgError(M("mv-core.destination.exact.failurereason.invalidformat"))
	}
	w, found := t.core.worldOf(worldName)
	if !found || !w.Loaded() {
		return nil, MsgError(M("mv-core.destination.shared.failurereason.worldnotfound", "{world}", worldName))
	}
	p, err := ParseEntityPosition(pos)
	if err != nil {
		return nil, MsgError(M("mv-core.destination.shared.failurereason.invalidnumberformat", "{error}", ErrorMessage(err)))
	}
	return &exactDestination{typ: t, world: w, pos: p}, nil
}

func (t *exactType) Suggest(sender plugin.CommandSender, _ string) []Suggestion {
	var out []Suggestion
	for _, w := range t.core.accessibleLoaded(sender) {
		out = append(out, Suggestion{Type: t, Text: w.TabCompleteName() + ":", PermSuffix: w.Name()})
	}
	if loc, ok := senderLocation(sender); ok {
		if w, ok := t.core.Worlds.Of(loc.World); ok {
			f := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
			xyz := w.Name() + ":" + f(loc.Pos.X) + "," + f(loc.Pos.Y) + "," + f(loc.Pos.Z)
			out = append(out, Suggestion{Type: t, Text: "@here", PermSuffix: w.Name()},
				Suggestion{Type: t, Text: xyz, PermSuffix: w.Name()},
				Suggestion{Type: t, Text: xyz + ":" + f(float64(loc.Pitch)) + ":" + f(float64(loc.Yaw)), PermSuffix: w.Name()})
		}
	}
	return out
}

type exactDestination struct {
	typ   *exactType
	world *World
	pos   EntityPosition
}

func (d *exactDestination) Type() DestinationType { return d.typ }

func (d *exactDestination) Location(e plugin.Entity) (plugin.Location, bool) {
	h, ok := d.world.Handle()
	if !ok {
		return plugin.Location{}, false
	}
	return d.pos.Location(h, e.Location()), true
}

func (*exactDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*exactDestination) CheckSafety() bool                          { return false }
func (d *exactDestination) PermissionSuffix() (string, bool)         { return d.world.Name(), true }
func (d *exactDestination) Display() Message {
	return Lit(d.world.AliasOrName() + " at " + d.pos.String())
}
func (d *exactDestination) Serialise() string { return d.world.Name() + ":" + d.pos.String() }

// playerType is "pl:<player>": where the player stands. MV: PlayerDestination
type playerType struct{ core *Core }

func (*playerType) ID() string { return "pl" }

func (t *playerType) Parse(sender plugin.CommandSender, params string) (Destination, error) {
	p, ok := t.core.FindPlayer(sender, params)
	if !ok {
		return nil, MsgError(M("mv-core.destination.player.failurereason.playernotfound", "{player}", params))
	}
	return &playerDestination{typ: t, id: p.UUID(), name: p.Name()}, nil
}

func (t *playerType) Suggest(sender plugin.CommandSender, _ string) []Suggestion {
	var out []Suggestion
	for _, name := range t.core.PlayerNames(sender) {
		out = append(out, Suggestion{Type: t, Text: name, PermSuffix: name})
	}
	return out
}

type playerDestination struct {
	typ  *playerType
	id   plugin.UUID
	name string
}

func (d *playerDestination) Type() DestinationType { return d.typ }

func (d *playerDestination) Location(plugin.Entity) (plugin.Location, bool) {
	p, ok := d.typ.core.Server.Player(d.id)
	if !ok {
		return plugin.Location{}, false
	}
	return p.Location(), true
}

func (*playerDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*playerDestination) CheckSafety() bool                          { return true }
func (d *playerDestination) PermissionSuffix() (string, bool)         { return d.name, true }
func (d *playerDestination) Display() Message                         { return Lit(d.name) }
func (d *playerDestination) Serialise() string                        { return d.name }

// anchorType is "a:<anchor>". MV: AnchorDestination
type anchorType struct{ core *Core }

func (*anchorType) ID() string { return "a" }

func (t *anchorType) Parse(_ plugin.CommandSender, params string) (Destination, error) {
	a, ok := t.core.Anchors.Get(params)
	if !ok {
		return nil, MsgError(M("mv-core.destination.anchor.failurereason.anchornotfound", "{anchor}", params))
	}
	return &anchorDestination{typ: t, name: params, loc: a.Location}, nil
}

func (t *anchorType) Suggest(sender plugin.CommandSender, _ string) []Suggestion {
	var out []Suggestion
	for _, a := range t.core.Anchors.For(sender) {
		out = append(out, Suggestion{Type: t, Text: a.Name, PermSuffix: a.Name})
	}
	return out
}

type anchorDestination struct {
	typ  *anchorType
	name string
	loc  WorldLocation
}

func (d *anchorDestination) Type() DestinationType { return d.typ }

func (d *anchorDestination) Location(plugin.Entity) (plugin.Location, bool) {
	w, ok := d.typ.core.Worlds.Get(d.loc.World)
	if !ok {
		return plugin.Location{}, false
	}
	h, ok := w.Handle()
	if !ok {
		return plugin.Location{}, false
	}
	return d.loc.In(h), true
}

func (*anchorDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*anchorDestination) CheckSafety() bool                          { return true }
func (d *anchorDestination) PermissionSuffix() (string, bool)         { return d.name, true }
func (d *anchorDestination) Display() Message {
	return M("mv-core.destination.anchor.display", "{anchor}", d.name)
}
func (d *anchorDestination) Serialise() string { return d.name }

// ownBed is the bed destination of the teleportee's own bed.
const ownBed = "playerbed"

// bedType is "b:<player>" or "b:playerbed". MV: BedDestination
type bedType struct{ core *Core }

func (*bedType) ID() string { return "b" }

func (t *bedType) Parse(sender plugin.CommandSender, params string) (Destination, error) {
	if params == ownBed {
		return &bedDestination{typ: t}, nil
	}
	p, ok := t.core.FindPlayer(sender, params)
	if !ok {
		return nil, MsgError(M("mv-core.destination.bed.failurereason.playernotfound", "{player}", params))
	}
	return &bedDestination{typ: t, player: p}, nil
}

func (t *bedType) Suggest(sender plugin.CommandSender, _ string) []Suggestion {
	var out []Suggestion
	for _, name := range t.core.PlayerNames(sender) {
		out = append(out, Suggestion{Type: t, Text: name, PermSuffix: name})
	}
	if _, ok := sender.(plugin.Player); ok {
		out = append(out, Suggestion{Type: t, Text: ownBed, PermSuffix: ownBed})
	}
	return out
}

type bedDestination struct {
	typ    *bedType
	player plugin.Player
}

func (d *bedDestination) Type() DestinationType { return d.typ }

func (d *bedDestination) Location(e plugin.Entity) (plugin.Location, bool) {
	p := d.player
	if p == nil {
		var ok bool
		if p, ok = e.(plugin.Player); !ok {
			return plugin.Location{}, false
		}
	}
	return p.RespawnPoint()
}

func (*bedDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) { return plugin.Vec3{}, false }
func (*bedDestination) CheckSafety() bool                          { return false }
func (d *bedDestination) PermissionSuffix() (string, bool)         { return d.Serialise(), true }

func (d *bedDestination) Display() Message {
	if d.player == nil {
		return M("mv-core.destination.bed.display.own")
	}
	return M("mv-core.destination.bed.display.other", "{player}", d.player.Name())
}

func (d *bedDestination) Serialise() string {
	if d.player == nil {
		return ownBed
	}
	return d.player.Name()
}

// cannonType is "ca:<world>:<x,y,z>:<pitch>:<yaw>:<speed>": a launch.
// MV: CannonDestination
type cannonType struct{ core *Core }

func (*cannonType) ID() string { return "ca" }

func (t *cannonType) Parse(_ plugin.CommandSender, params string) (Destination, error) {
	parts := strings.Split(params, ":")
	if len(parts) != 5 {
		return nil, MsgError(M("mv-core.destination.cannon.failurereason.invalidformat"))
	}
	xyz := strings.Split(parts[1], ",")
	if len(xyz) != 3 {
		return nil, MsgError(M("mv-core.destination.shared.failurereason.invalidcoordinatesformat"))
	}
	w, ok := t.core.Worlds.Get(parts[0])
	if !ok || !w.Loaded() {
		return nil, MsgError(M("mv-core.destination.shared.failurereason.worldnotfound", "{world}", parts[0]))
	}
	d := &cannonDestination{typ: t, world: w}
	nums := []string{xyz[0], xyz[1], xyz[2], parts[3], parts[2], parts[4]}
	dst := []*float64{&d.loc.X, &d.loc.Y, &d.loc.Z, nil, nil, &d.speed}
	var angles [2]float64
	dst[3], dst[4] = &angles[0], &angles[1]
	for i, s := range nums {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, MsgError(M("mv-core.destination.shared.failurereason.invalidnumberformat",
				"{error}", fmt.Sprintf("java.lang.NumberFormatException: For input string: \"%s\"", s)))
		}
		*dst[i] = f
	}
	d.loc.World, d.loc.Yaw, d.loc.Pitch = w.Name(), float32(angles[0]), float32(angles[1])
	return d, nil
}

func (*cannonType) Suggest(plugin.CommandSender, string) []Suggestion { return nil }

type cannonDestination struct {
	typ   *cannonType
	world *World
	loc   WorldLocation
	speed float64
}

func (d *cannonDestination) Type() DestinationType { return d.typ }

func (d *cannonDestination) Location(plugin.Entity) (plugin.Location, bool) {
	h, ok := d.world.Handle()
	if !ok {
		return plugin.Location{}, false
	}
	return d.loc.In(h), true
}

// Velocity launches along the facing. MV: CannonDestinationInstance.getVelocity
func (d *cannonDestination) Velocity(plugin.Entity) (plugin.Vec3, bool) {
	pitch := float64(d.loc.Pitch) * math.Pi / 180
	yaw := float64(d.loc.Yaw) * math.Pi / 180
	x := -math.Sin(yaw) * d.speed
	y := -math.Sin(pitch) * d.speed
	z := math.Cos(yaw) * d.speed
	return plugin.Vec3{X: math.Cos(pitch) * x, Y: y, Z: math.Cos(pitch) * z}, true
}

func (*cannonDestination) CheckSafety() bool                  { return false }
func (d *cannonDestination) PermissionSuffix() (string, bool) { return d.world.Name(), true }
func (d *cannonDestination) Display() Message                 { return Lit(DestinationString(d)) }

func (d *cannonDestination) Serialise() string {
	return d.world.Name() + ":" + JavaDouble(d.loc.X) + "," + JavaDouble(d.loc.Y) + "," + JavaDouble(d.loc.Z) + ":" +
		JavaFloat(d.loc.Pitch) + ":" + JavaFloat(d.loc.Yaw) + ":" + JavaDouble(d.speed)
}

// selectors are the vanilla selectors Multiverse accepts for players.
var selectors = []string{"@a", "@e", "@r", "@p", "@s"}

// FindPlayer answers the online player named, with that UUID, or picked
// by a bare selector (@p, @r, @s; @a and @e when one player matches).
// MV: PlayerFinder.get
func (c *Core) FindPlayer(sender plugin.CommandSender, s string) (plugin.Player, bool) {
	if p, ok := c.playerExact(s); ok {
		return p, true
	}
	if id, err := uuid.Parse(s); err == nil {
		return c.Server.Player(id)
	}
	ps, err := c.FindPlayers(sender, s)
	if err != nil || len(ps) == 0 {
		return nil, false
	}
	return ps[0], true
}

func (c *Core) playerExact(name string) (plugin.Player, bool) {
	p, ok := c.Server.PlayerNamed(name)
	if !ok || p.Name() != name && !strings.EqualFold(p.Name(), name) {
		return nil, false
	}
	return p, true
}

// FindPlayers answers the players a comma-separated list of names, UUIDs
// and selectors names. Selectors take no arguments here. MV:
// PlayerFinder.tryGetMulti
func (c *Core) FindPlayers(sender plugin.CommandSender, s string) ([]plugin.Player, error) {
	if s == "" {
		return nil, nil
	}
	if isSelector(s) {
		return c.selectPlayers(sender, s)
	}
	var out []plugin.Player
	for part := range strings.SplitSeq(s, ",") {
		if p, ok := c.playerExact(part); ok {
			out = append(out, p)
			continue
		}
		if id, err := uuid.Parse(part); err == nil {
			if p, ok := c.Server.Player(id); ok {
				out = append(out, p)
			}
			continue
		}
		ps, err := c.selectPlayers(sender, part)
		if err != nil {
			return nil, err
		}
		out = append(out, ps...)
	}
	return out, nil
}

func isSelector(s string) bool {
	for _, sel := range selectors {
		if strings.HasPrefix(s, sel) {
			return true
		}
	}
	return false
}

// selectPlayers resolves a bare selector to players: golem offers plugins no
// selector parser, so arguments in brackets are refused.
func (c *Core) selectPlayers(sender plugin.CommandSender, sel string) ([]plugin.Player, error) {
	if !isSelector(sel) {
		return nil, nil
	}
	if len(sel) > 2 {
		return nil, MsgError(M("mv-core.commands.error.player.selector", "{player}", sel))
	}
	all := c.Server.Players()
	self, isPlayer := sender.(plugin.Player)
	switch sel {
	case "@a", "@e":
		return all, nil
	case "@s":
		if isPlayer {
			return []plugin.Player{self}, nil
		}
	case "@r":
		if len(all) > 0 {
			return []plugin.Player{all[rand.IntN(len(all))]}, nil
		}
	case "@p":
		if isPlayer {
			return []plugin.Player{self}, nil
		}
		if len(all) > 0 {
			return all[:1], nil
		}
	}
	return nil, nil
}
