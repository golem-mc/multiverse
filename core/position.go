package core

import (
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// PosNumber is one coordinate or angle: absolute, or relative ("~2") to a
// base. MV: PositionNumber
type PosNumber struct {
	Value    float64
	Relative bool
}

// Of answers the number against base.
func (n PosNumber) Of(base float64) float64 {
	if n.Relative {
		return base + n.Value
	}
	return n.Value
}

func (n PosNumber) String() string {
	if n.Relative {
		return "~" + JavaDouble(n.Value)
	}
	return JavaDouble(n.Value)
}

func parsePosNumber(s string) (PosNumber, error) {
	rel := strings.HasPrefix(s, "~")
	if rel {
		s = s[1:]
		if s == "" {
			return PosNumber{Relative: true}, nil
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return PosNumber{}, MsgError(M("mv-core.exception.positionparse.invalidnumber", "{number}", s))
	}
	return PosNumber{Value: f, Relative: rel}, nil
}

// EntityPosition is "x,y,z[:pitch:yaw]", each part absolute or relative.
// MV: EntityPosition
type EntityPosition struct {
	X, Y, Z, Pitch, Yaw PosNumber
}

// ParseEntityPosition reads "x,y,z[:pitch:yaw]"; a missing direction is
// pitch 0, yaw 0. MV: EntityPosition.fromString
func ParseEntityPosition(s string) (EntityPosition, error) {
	var p EntityPosition
	xyz, dir, hasDir := strings.Cut(s, ":")
	parts := strings.Split(xyz, ",")
	if len(parts) != 3 {
		return p, MsgError(M("mv-core.exception.positionparse.invalidcoordinates", "{format}", xyz))
	}
	var err error
	for i, dst := range []*PosNumber{&p.X, &p.Y, &p.Z} {
		if *dst, err = parsePosNumber(parts[i]); err != nil {
			return p, err
		}
	}
	if hasDir {
		pitch, yaw, ok := strings.Cut(dir, ":")
		if !ok {
			return p, MsgError(M("mv-core.exception.positionparse.invaliddirection", "{format}", dir))
		}
		if p.Pitch, err = parsePosNumber(pitch); err != nil {
			return p, err
		}
		if p.Yaw, err = parsePosNumber(yaw); err != nil {
			return p, err
		}
	}
	return p, nil
}

// PositionOf answers the absolute position of loc.
func PositionOf(loc plugin.Location) EntityPosition {
	return EntityPosition{X: PosNumber{Value: loc.Pos.X}, Y: PosNumber{Value: loc.Pos.Y}, Z: PosNumber{Value: loc.Pos.Z},
		Pitch: PosNumber{Value: float64(loc.Pitch)}, Yaw: PosNumber{Value: float64(loc.Yaw)}}
}

// Location answers the position in w against base (for relative parts).
func (p EntityPosition) Location(w plugin.World, base plugin.Location) plugin.Location {
	return plugin.Location{World: w,
		Pos:   plugin.Vec3{X: p.X.Of(base.Pos.X), Y: p.Y.Of(base.Pos.Y), Z: p.Z.Of(base.Pos.Z)},
		Pitch: float32(p.Pitch.Of(float64(base.Pitch))), Yaw: float32(p.Yaw.Of(float64(base.Yaw)))}
}

func (p EntityPosition) String() string {
	return p.X.String() + "," + p.Y.String() + "," + p.Z.String() + ":" + p.Pitch.String() + ":" + p.Yaw.String()
}

// LocationString writes a location as anchors.yml keeps it:
// "world:x,y,z:yaw:pitch" with two decimals. MV:
// LocationManipulation.locationToString
func LocationString(world string, loc plugin.Location) string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	return world + ":" + f(loc.Pos.X) + "," + f(loc.Pos.Y) + "," + f(loc.Pos.Z) + ":" +
		f(float64(loc.Yaw)) + ":" + f(float64(loc.Pitch))
}

// WorldLocation is a location in a world named but perhaps not loaded.
// MV: UnloadedWorldLocation
type WorldLocation struct {
	World      string
	X, Y, Z    float64
	Yaw, Pitch float32
}

// ParseLocationString reads "world:x,y,z[:yaw[:pitch]]"; false when it is
// not one. MV: LocationManipulation.stringToLocation
func ParseLocationString(s string) (WorldLocation, bool) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 4 || parts[0] == "" {
		return WorldLocation{}, false
	}
	xyz := strings.Split(parts[1], ",")
	if len(xyz) != 3 {
		return WorldLocation{}, false
	}
	l := WorldLocation{World: parts[0]}
	var err error
	for i, dst := range []*float64{&l.X, &l.Y, &l.Z} {
		if *dst, err = strconv.ParseFloat(xyz[i], 64); err != nil {
			return WorldLocation{}, false
		}
	}
	for i, dst := range []*float32{&l.Yaw, &l.Pitch} {
		if len(parts) > 2+i {
			f, err := strconv.ParseFloat(parts[2+i], 64)
			if err != nil {
				return WorldLocation{}, false
			}
			*dst = float32(f)
		}
	}
	return l, true
}

// In answers the location in the loaded world h.
func (l WorldLocation) In(h plugin.World) plugin.Location {
	return plugin.Location{World: h, Pos: plugin.Vec3{X: l.X, Y: l.Y, Z: l.Z}, Yaw: l.Yaw, Pitch: l.Pitch}
}

// String writes the location as LocationString does.
func (l WorldLocation) String() string {
	return LocationString(l.World, plugin.Location{Pos: plugin.Vec3{X: l.X, Y: l.Y, Z: l.Z}, Yaw: l.Yaw, Pitch: l.Pitch})
}
