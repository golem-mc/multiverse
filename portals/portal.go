package portals

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// NamePattern is what a portal's name may be made of. MV: MVPortal.PORTAL_NAME_PATTERN
var NamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Permission node prefixes of each portal. MV: MVPortal.setUpPermissions
const (
	PermAccess = "multiverse.portal.access"
	PermFill   = "multiverse.portal.fill"
	PermExempt = "multiverse.portal.exempt"
)

// Region is a box of blocks in a Multiverse world, its corners as written
// in portals.yml. MV: MultiverseRegion, PortalLocation
type Region struct {
	World    *core.World
	Min, Max plugin.Vec3
}

// NewRegion answers the box with corners a and b in w.
func NewRegion(w *core.World, a, b plugin.Vec3) Region {
	return Region{World: w,
		Min: plugin.Vec3{X: min(a.X, b.X), Y: min(a.Y, b.Y), Z: min(a.Z, b.Z)},
		Max: plugin.Vec3{X: max(a.X, b.X), Y: max(a.Y, b.Y), Z: max(a.Z, b.Z)}}
}

// MinBlock and MaxBlock answer the corner blocks.
func (r Region) MinBlock() plugin.BlockPos { return r.Min.Block() }
func (r Region) MaxBlock() plugin.BlockPos { return r.Max.Block() }

// Width, Height and Depth answer the size in blocks along x, y and z.
func (r Region) Width() int  { return absInt(r.MaxBlock().X + 1 - r.MinBlock().X) }
func (r Region) Height() int { return absInt(r.MaxBlock().Y + 1 - r.MinBlock().Y) }
func (r Region) Depth() int  { return absInt(r.MaxBlock().Z + 1 - r.MinBlock().Z) }

// Area answers the number of blocks in the box.
func (r Region) Area() int { return r.Width() * r.Height() * r.Depth() }

// Contains reports whether pos in world w is inside the box.
// MV: MultiverseRegion.containsVector
func (r Region) Contains(w *core.World, pos plugin.BlockPos) bool {
	if w != nil && r.World != nil && w.Name() != r.World.Name() {
		return false
	}
	lo, hi := r.MinBlock(), r.MaxBlock()
	return pos.X >= lo.X && pos.X <= hi.X && pos.Y >= lo.Y && pos.Y <= hi.Y && pos.Z >= lo.Z && pos.Z <= hi.Z
}

// String writes the box as portals.yml keeps it: "world:x,y,z:x,y,z" with
// Java doubles. MV: MultiverseRegion.toString
func (r Region) String() string {
	name := ""
	if r.World != nil {
		name = r.World.Name()
	}
	v := func(p plugin.Vec3) string {
		return core.JavaDouble(p.X) + "," + core.JavaDouble(p.Y) + "," + core.JavaDouble(p.Z)
	}
	return name + ":" + v(r.Min) + ":" + v(r.Max)
}

func absInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// parseRegion reads "WORLD:X,Y,Z:X,Y,Z" with a world Multiverse knows,
// loaded or not; false when it is not one. MV: PortalLocation.parseLocation
func (p *Portals) parseRegion(s string) (Region, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		p.Log.Warn("Failed Parsing Location (Format Error, was expecting: `WORLD:X,Y,Z:X,Y,Z`, but got: `" + s + "`)")
		return Region{}, false
	}
	w, ok := p.Core.Worlds.Get(parts[0])
	if !ok {
		p.Log.Warn("Failed Parsing World (World Error, World did not exist or was not imported into Multiverse-Core!)")
		return Region{}, false
	}
	a, okA := parseVector(parts[1])
	b, okB := parseVector(parts[2])
	if !okA || !okB {
		p.Log.Warn("Failed Parsing Location (Vector Error, was expecting: `WORLD:X,Y,Z:X,Y,Z`, but got: `" + s + "`)")
		return Region{}, false
	}
	return NewRegion(w, a, b), true
}

func parseVector(s string) (plugin.Vec3, bool) {
	parts := strings.Split(s, ",")
	if len(parts) < 3 {
		return plugin.Vec3{}, false
	}
	var out [3]float64
	for i := range out {
		f, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			return plugin.Vec3{}, false
		}
		out[i] = f
	}
	return plugin.Vec3{X: out[0], Y: out[1], Z: out[2]}, true
}

// Portal property paths in portals.yml.
const (
	propActionType   = "action.type"
	propAction       = "action.value"
	propCheckSafety  = "check-destination-safety"
	propCurrency     = "entry-fee.currency"
	propPrice        = "entry-fee.price"
	propLocation     = "location"
	propSuccessMsg   = "message.action-success"
	propNoPermission = "message.no-permission"
	propOwner        = "owner"
	propSafeTeleport = "safe-teleport"
	propNonPlayers   = "teleport-non-players"
	propVersion      = "version"
)

// portalVersion is the version of a portal's section this port writes.
const portalVersion = 1.3

// portalNodes are a portal's properties, as /mvp modify names them.
// MV: MVPortalNodes
func (p *Portals) portalNodes() *core.Nodes {
	return core.NewNodes(
		&core.Node{Path: propActionType, Name: "action-type", Kind: core.KindString, Default: "multiverse-destination",
			Suggest: func(string) []string { return p.Actions.Names() }},
		&core.Node{Path: propAction, Name: "action", Aliases: []string{"destination", "dest"}, Kind: core.KindString, Default: ""},
		&core.Node{Path: propCheckSafety, Kind: core.KindBool, Default: true},
		&core.Node{Path: propCurrency, Name: "currency", Aliases: []string{"curr"}, Kind: core.KindString, Default: "air",
			Parse: parseItemName},
		&core.Node{Path: propPrice, Name: "price", Kind: core.KindDouble, Default: 0.0},
		&core.Node{Path: propLocation, Aliases: []string{"loc"}, Kind: core.KindString, Default: ""},
		&core.Node{Path: propSuccessMsg, Name: "action-success-message", Kind: core.KindString, Default: "@disabled"},
		&core.Node{Path: propNoPermission, Name: "no-permission-message", Kind: core.KindString, Default: "@default"},
		&core.Node{Path: propOwner, Kind: core.KindString, Default: ""},
		&core.Node{Path: propSafeTeleport, Aliases: []string{"safe"}, Kind: core.KindBool, Default: true},
		&core.Node{Path: propNonPlayers, Aliases: []string{"telenonplayers"}, Kind: core.KindBool, Default: false},
		&core.Node{Path: propVersion, Kind: core.KindDouble, Default: 0.0, Hidden: true},
	)
}

// migratePortal moves an older portal section's keys to version 1.3's.
// MV: MVPortal.setUpConfigHandle's ConfigMigrator
func migratePortal(m map[string]any) {
	if len(m) == 0 {
		return
	}
	v := versionOf(m)
	if v < 1.0 {
		movePath(m, "safeteleport", "safe-teleport")
		movePath(m, "teleportnonplayers", "teleport-non-players")
	}
	if v < 1.1 {
		world, okW := stringAt(m, "world")
		loc, okL := stringAt(m, "location")
		if okW && okL {
			m["location"] = world + ":" + loc
		}
	}
	if v < 1.2 {
		movePath(m, "destination", "action")
	}
	if v < 1.3 {
		movePath(m, "action", "action.value")
		movePath(m, "action-type", "action.type")
		movePath(m, "currency", "entry-fee.currency")
		movePath(m, "price", "entry-fee.price")
	}
}

// Portal is a Multiverse portal: a box that runs an action for whoever
// enters it. MV: MVPortal
type Portal struct {
	p    *Portals
	name string
	// Props are the portal's properties in portals.yml.
	Props  *core.Values
	region Region
	valid  bool
}

func (p *Portals) newPortal(name string, section map[string]any) *Portal {
	pt := &Portal{p: p, name: name, Props: core.NewValues(p.nodes)}
	if section == nil {
		section = map[string]any{}
	}
	migratePortal(section)
	pt.Props.Load(section)
	_ = pt.Props.Set(p.versionNode(), portalVersion)
	p.registerPermissions(name)
	return pt
}

// versionNode answers the hidden version node, which Find does not.
func (p *Portals) versionNode() *core.Node {
	for _, n := range p.nodes.All() {
		if n.Path == propVersion {
			return n
		}
	}
	return nil
}

// registerPermissions registers a portal's access, fill and exempt nodes.
func (p *Portals) registerPermissions(name string) {
	p.Core.Permissions.Register(core.Concat(PermAccess, name), core.DefaultOp)
	p.Core.Permissions.Register(core.Concat(PermFill, name), core.DefaultOp)
	p.Core.Permissions.Register(core.Concat(PermExempt, name), core.DefaultFalse)
}

// Name answers the portal's name.
func (pt *Portal) Name() string { return pt.name }

// Owner answers who made the portal.
func (pt *Portal) Owner() string { return pt.Props.String(propOwner) }

// Region answers the portal's box; false when its location is invalid.
func (pt *Portal) Region() (Region, bool) { return pt.region, pt.valid }

// World answers the Multiverse world the portal is in.
func (pt *Portal) World() (*core.World, bool) {
	if !pt.valid || pt.region.World == nil {
		return nil, false
	}
	return pt.region.World, true
}

// ActionType and Action answer the configured action.
func (pt *Portal) ActionType() string { return pt.Props.String(propActionType) }
func (pt *Portal) Action() string     { return pt.Props.String(propAction) }

// CheckDestinationSafety, SafeTeleport and TeleportNonPlayers answer the
// portal's switches.
func (pt *Portal) CheckDestinationSafety() bool { return pt.Props.Bool(propCheckSafety) }
func (pt *Portal) SafeTeleport() bool           { return pt.Props.Bool(propSafeTeleport) }
func (pt *Portal) TeleportNonPlayers() bool     { return pt.Props.Bool(propNonPlayers) }

// Price and Currency answer the entry fee: a price in items of Currency
// (lowercase, "air" for an economy).
func (pt *Portal) Price() float64   { return pt.Props.Double(propPrice) }
func (pt *Portal) Currency() string { return pt.Props.String(propCurrency) }

// CanEnter reports whether p holds the portal's access permission.
// MV: MVPortal.playerCanEnterPortal
func (pt *Portal) CanEnter(p plugin.CommandSender) bool {
	return p.HasPermission(core.Concat(PermAccess, pt.name))
}

// CanFill reports whether p may fill the portal. MV: MVPortal.playerCanFillPortal
func (pt *Portal) CanFill(p plugin.CommandSender) bool {
	return p.HasPermission(core.Concat(PermFill, pt.name))
}

// Exempt reports whether p uses the portal for free.
func (pt *Portal) Exempt(p plugin.CommandSender) bool {
	return p.HasPermission(core.Concat(PermExempt, pt.name))
}

// ActionHandler parses the portal's action as sender would.
// MV: MVPortal.getActionHandler
func (pt *Portal) ActionHandler(sender plugin.CommandSender) (Action, error) {
	return pt.p.Actions.Parse(sender, pt.ActionType(), pt.Action())
}

// set stores a property by path.
func (pt *Portal) set(path string, v any) {
	n, _ := pt.p.nodes.Find(path)
	_ = pt.setNode(n, v)
}

// setNode stores a property's value, keeping the box and the chunk index
// in step with location.
func (pt *Portal) setNode(n *core.Node, v any) error {
	if err := pt.Props.Set(n, v); err != nil {
		return err
	}
	if n.Path == propLocation {
		pt.p.Manager.unindex(pt)
		pt.region, pt.valid = pt.p.parseRegion(pt.Props.String(propLocation))
		pt.p.Manager.index(pt)
	}
	return nil
}

// SetProperty parses value as the property called name and sets it, as
// /mvp modify does: the action through the portal's action type as sender,
// the location from "@selected-region" or "WORLD:X,Y,Z:X,Y,Z".
// MV: StringPropertyHandle.setPropertyString with MVPortalNodes' parsers
func (pt *Portal) SetProperty(sender plugin.CommandSender, name, value string) error {
	n, ok := pt.p.nodes.Find(name)
	if !ok {
		return core.MsgError(core.M("mv-core.config.node.notfound", "{node}", name))
	}
	var v any
	var err error
	switch n.Path {
	case propAction:
		var t ActionType
		if t, err = pt.p.Actions.Type(pt.ActionType()); err == nil {
			var a Action
			if a, err = t.Parse(sender, value); err == nil {
				v = a.Serialise()
			}
		}
	case propLocation:
		v, err = pt.p.parseLocationProperty(sender, value)
	default:
		v, err = core.ParseString(n, value)
	}
	if err != nil {
		return err
	}
	return pt.setNode(n, v)
}

// parseLocationProperty reads a location typed for a portal.
func (p *Portals) parseLocationProperty(sender plugin.CommandSender, value string) (string, error) {
	if value == "@selected-region" {
		pl, ok := sender.(plugin.Player)
		if !ok {
			return "", core.MsgError(core.M("mv-portals.portalconfig.location.playersonly"))
		}
		r, ok := p.Session(pl).SelectedRegion()
		if !ok {
			return "", core.MsgError(core.M("mv-portals.portalconfig.location.selectionrequired"))
		}
		return r.String(), nil
	}
	r, ok := p.parseRegion(value)
	if !ok {
		return "", core.MsgError(core.M("mv-portals.portalconfig.location.invalid"))
	}
	return r.String(), nil
}

// SafeSpawn answers where a player arriving at the portal is put: the
// middle of its box, at the lowest height inside it where a player can
// stand, else its bottom. MV: MVPortal.getSafePlayerSpawnLocation
func (pt *Portal) SafeSpawn() (plugin.Location, bool) {
	w, ok := pt.World()
	if !ok {
		return plugin.Location{}, false
	}
	h, ok := w.Handle()
	if !ok {
		return plugin.Location{}, false
	}
	lo, hi := pt.region.MinBlock(), pt.region.MaxBlock()
	x := float64(absInt(hi.X-lo.X)+1)/2 + float64(lo.X)
	z := float64(absInt(hi.Z-lo.Z)+1)/2 + float64(lo.Z)
	y := lo.Y
	for i := lo.Y; i < hi.Y; i++ {
		if core.SafeAt(h, plugin.BlockPos{X: int(math.Floor(x)), Y: i, Z: int(math.Floor(z))}) {
			y = i
			break
		}
	}
	return plugin.Location{World: h, Pos: plugin.Vec3{X: x, Y: float64(y), Z: z}}, true
}
