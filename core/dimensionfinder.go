package core

import (
	"regexp"
	"strings"
)

// OverworldOf answers the overworld a nether or end belongs to, by the
// world-name-format names; an overworld answers itself. MV:
// DimensionFinder.getOverworldWorld
func (ws *Worlds) OverworldOf(w *World) (*World, bool) {
	var format string
	switch w.Environment() {
	case "normal":
		return w, true
	case "nether":
		format = ws.core.Config.String("world.world-name-format.nether")
	case "the_end":
		format = ws.core.Config.String("world.world-name-format.end")
	default:
		return nil, false
	}
	const placeholder = "%overworld%"
	if !strings.Contains(format, placeholder) {
		return nil, false
	}
	re, err := regexp.Compile("^" + strings.ReplaceAll(regexp.QuoteMeta(format), regexp.QuoteMeta(placeholder), "(.+)") + "$")
	if err != nil {
		return nil, false
	}
	m := re.FindStringSubmatch(w.Name())
	if m == nil {
		return nil, false
	}
	return ws.Get(m[1])
}
