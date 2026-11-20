package commands

import (
	"math"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, coordinatesCommands) }

// coordinatesCommands is /mv coordinates. MV: CoordinatesCommand
func coordinatesCommands(c *core.Core) []*core.Cmd {
	var aliases []string
	if c.Config.Bool("command.show-legacy-aliases") {
		aliases = []string{"coords", "coord", "co"}
	}
	return []*core.Cmd{{
		Name: "coordinates", Aliases: aliases, Legacy: []string{"mvcoord", "mvco"},
		Permission: "multiverse.core.coord", Description: "mv-core.coordinates.description", PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			p, _ := ctx.Player()
			w, ok := ctx.Core.Worlds.Of(p.World())
			if !ok {
				return core.InvalidArg(core.M("mv-core.commands.error.multiverseworld.playersonly"))
			}
			loc := p.Location()
			ctx.Info(core.M("mv-core.coordinates.info.title"))
			ctx.Info(core.M("mv-core.coordinates.info.world", "{world}", w.Name()))
			ctx.Info(core.M("mv-core.coordinates.info.alias", "{alias}", w.AliasOrName()))
			ctx.Info(core.M("mv-core.coordinates.info.worldscale", "{scale}", core.JavaDouble(w.Props.Double(core.PropScale))))
			ctx.Info(core.M("mv-core.coordinates.info.coordinates", "{coordinates}",
				core.StrCoords(loc.Pos.X, loc.Pos.Y, loc.Pos.Z, loc.Pitch, loc.Yaw)))
			ctx.Info(core.M("mv-core.coordinates.info.direction", "{direction}", direction(loc.Yaw)))
			return nil
		},
	}}
}

// direction answers the compass point, n to nw, a yaw faces. MV:
// LocationManipulation.getDirection
func direction(yaw float32) string {
	r := math.Mod(float64(yaw), 360) + 180
	for i, limit := range []float64{22.5, 67.5, 112.5, 157.5, 202.5, 247.5, 292.5, 337.5} {
		if r < limit {
			return []string{"n", "ne", "e", "se", "s", "sw", "w", "nw"}[i]
		}
	}
	return "n"
}
