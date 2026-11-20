package commands

import (
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, infoCommands) }

// infoCommands is /mv info. MV: InfoCommand
func infoCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "info", Legacy: []string{"mvinfo", "mvi"},
		Permission: "multiverse.core.info", Syntax: "[world] [--page <page>] [--filter <filter>]",
		Description: "mv-core.info.description", MaxArgs: 1, Flags: core.PageFilterFlags(),
		Complete: func(ctx *core.Ctx, pos int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, _, err := ctx.WorldArg(0, true)
			if err != nil {
				return err
			}
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.info.header", "{world}", w.Name())
			nc := core.M("mv-core.info.nocontent")
			d.Header, d.NoContent = &h, &nc
			keys, vals := worldInfo(ctx.Core, w)
			d.Send(ctx.Issuer, core.MapLines(ctx.Issuer, keys, vals, "§b", "§f"))
			return nil
		},
	}}
}

// worldInfo answers /mv info's rows in Multiverse's order.
func worldInfo(c *core.Core, w *core.World) ([]string, map[string]*string) {
	var keys []string
	vals := map[string]*string{}
	put := func(k string, v *string) { keys = append(keys, k); vals[k] = v }
	s := func(v any) *string { str := core.FormatValue(v); return &str }
	p := w.Props
	put("World Key", s(w.Key().String()))
	put("World Name", s(w.Name()))
	put("World Alias", s(w.Alias()))
	put("World UID", s(worldUID(w)))
	put("Game Mode", s(upper(p.String(core.PropGameMode))))
	put("Difficulty", s(upper(p.String(core.PropDifficulty))))
	put("Spawn Location", s(spawnCoords(w)))
	put("Respawn World", s(p.String(core.PropRespawnWorld)))
	put("Bed Respawn", s(p.Bool(core.PropBedRespawn)))
	put("Anchor Respawn", s(p.Bool(core.PropAnchorRespawn)))
	put("Seed", s(core.SeedOf(w)))
	put("Environment", s(upper(w.Environment())))
	put("World Type", s("NORMAL"))
	put("Biome", s(p.String(core.PropBiome)))
	put("Generator", s(w.Generator()))
	put("Generator Settings", s(p.String(core.PropGeneratorSettings)))
	put("Generate Structures", nil)
	put("Auto Load", s(w.AutoLoad()))
	put("Keep Spawn In Memory", s(p.Bool(core.PropKeepSpawnInMemory)))
	price := p.Double(core.PropEntryFeeAmount)
	switch {
	case price == 0:
		put("Entry Fee", s("FREE!"))
	case price > 0:
		put("Entry Fee", s(c.FormatPrice(price, p.String(core.PropEntryFeeCurrency))))
	default:
		put("Entry Reward", s(c.FormatPrice(-price, p.String(core.PropEntryFeeCurrency))))
	}
	put("World Scale", s(p.Double(core.PropScale)))
	put("Weather Enabled", s(p.Bool(core.PropAllowWeather)))
	put("Allow Flight", s(p.Bool(core.PropAllowFlight)))
	put("Grant Advancements", s(p.Bool(core.PropAdvancementGrant)))
	put("Auto Heal", s(p.Bool(core.PropAutoHeal)))
	put("Hunger Depletes", s(p.Bool(core.PropHunger)))
	put("PVP Enabled", s(p.Bool(core.PropPvp)))
	put("Portal Form", s(upper(p.String(core.PropPortalForm))))
	put("Player Limit", s(p.Int(core.PropPlayerLimit)))
	put("World Blacklist", s(strings.Join(p.List(core.PropWorldBlacklist), ", ")))
	return keys, vals
}

// worldUID stands in for Bukkit's world UUID, which golem worlds do not
// have: a name-based UUID of the key, stable across restarts.
func worldUID(w *core.World) string {
	return core.NameUUID("world:" + w.Key().String()).String()
}

// spawnCoords writes the spawn as LocationManipulation.strCoords does.
func spawnCoords(w *core.World) string {
	s := w.SpawnLocation()
	if s == nil {
		return ""
	}
	return core.StrCoords(s.X, s.Y, s.Z, s.Pitch, s.Yaw)
}

func upper(s string) string { return strings.ToUpper(s) }
