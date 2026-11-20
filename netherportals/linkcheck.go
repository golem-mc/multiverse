package netherportals

import (
	"regexp"
	"strings"

	"github.com/golem-mc/golem/plugin"

	"github.com/golem-mc/multiverse/core"
)

// The prefixes and suffixes are regular expressions, as Multiverse matches
// and splits with Java's String.matches and String.split.

// matchesName reports whether world is prefix, something, suffix.
// MV: MVNameChecker.isValidNetherName, isValidEndName
func matchesName(world, prefix, suffix string) bool {
	re, err := regexp.Compile("^(?:" + prefix + ").+(?:" + suffix + ")$")
	return err == nil && re.MatchString(world)
}

// javaSplit splits s around re as Java's String.split does: trailing empty
// strings removed.
func javaSplit(s, re string) []string {
	r, err := regexp.Compile(re)
	if err != nil {
		return []string{s}
	}
	parts := r.Split(s, -1)
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func (np *NetherPortals) isNetherName(world string) bool {
	return matchesName(world, np.Config.String(PathNetherPrefix), np.Config.String(PathNetherSuffix))
}

func (np *NetherPortals) isEndName(world string) bool {
	return matchesName(world, np.Config.String(PathEndPrefix), np.Config.String(PathEndSuffix))
}

// NetherName answers the nether world a normal world auto-links to.
// MV: MVNameChecker.getNetherName
func (np *NetherPortals) NetherName(normal string) string {
	return np.Config.String(PathNetherPrefix) + normal + np.Config.String(PathNetherSuffix)
}

// EndName answers the end world a normal world auto-links to.
// MV: MVNameChecker.getEndName
func (np *NetherPortals) EndName(normal string) string {
	return np.Config.String(PathEndPrefix) + normal + np.Config.String(PathEndSuffix)
}

// normalName chops t's prefix and suffix off a world's name.
// MV: MVNameChecker.getNormalName
func (np *NetherPortals) normalName(world string, t LinkType) string {
	prefix, suffix := np.Config.String(PathNetherPrefix), np.Config.String(PathNetherSuffix)
	if t == End {
		prefix, suffix = np.Config.String(PathEndPrefix), np.Config.String(PathEndSuffix)
	}
	name := world
	if prefix != "" {
		if parts := javaSplit(name, prefix); len(parts) > 1 {
			name = parts[1]
		}
	}
	if suffix != "" {
		name = javaSplit(name, suffix)[0]
	}
	return name
}

// AutoLink answers the world from's portals of type t lead to by name: a
// normal world's nether is <prefix><world><suffix>, and back.
// MV: MVLinkChecker.getAutoLink
func (np *NetherPortals) AutoLink(from string, t LinkType) string {
	switch {
	case np.isEndName(from):
		if t == End {
			return np.normalName(from, t)
		}
		// As Multiverse, which chops the nether affixes off an end world here.
		return np.NetherName(np.normalName(from, t))
	case np.isNetherName(from):
		if t == End {
			return np.EndName(np.normalName(from, t))
		}
		return np.normalName(from, t)
	case t == End:
		return np.EndName(from)
	}
	return np.NetherName(from)
}

// Destination answers the world from's portals of type t lead to: its link,
// else its auto-link.
func (np *NetherPortals) Destination(from string, t LinkType) string {
	if to, ok := np.Links.To(from, t); ok {
		return to
	}
	return np.AutoLink(from, t)
}

// loadedWorld answers the loaded Multiverse world named name.
func (np *NetherPortals) loadedWorld(name string) (*core.World, bool) {
	w, ok := np.core.Worlds.Get(name)
	if !ok || !w.Loaded() {
		return nil, false
	}
	return w, true
}

// scaledLocation answers from moved to the world named to, x and z scaled
// by the two worlds' scales; false when either is not a loaded Multiverse
// world. MV: MVLinkChecker.findNewTeleportLocation
func (np *NetherPortals) scaledLocation(from plugin.Location, to string) (plugin.Location, bool) {
	toW, ok := np.loadedWorld(to)
	if !ok {
		np.core.Debug(1, "Can't find world %s", to)
		return plugin.Location{}, false
	}
	fromW, ok := np.core.Worlds.Of(from.World)
	if !ok {
		np.core.Log.Warn("World " + from.World.Key().String() + " is not a Multiverse world")
		return plugin.Location{}, false
	}
	scale := fromW.Props.Double(core.PropScale) / toW.Props.Double(core.PropScale)
	loc := from
	loc.Pos.X *= scale
	loc.Pos.Z *= scale
	loc.World, _ = toW.Handle()
	return loc, true
}

// destinationOf answers where an entity in a portal of type t at from goes:
// nowhere when the world links to itself (the portals are disabled) or the
// destination is not loaded. MV: MVNPEntityListener.getLocation
func (np *NetherPortals) destinationOf(from plugin.Location, t LinkType, current, linked string) (plugin.Location, bool) {
	if linked != "" && strings.EqualFold(current, linked) {
		return plugin.Location{}, false
	}
	if linked == "" {
		linked = np.AutoLink(current, t)
	}
	return np.scaledLocation(from, linked)
}
