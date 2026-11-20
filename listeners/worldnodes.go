package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, worldNodes) }

// worldNodes applies the properties that change the loaded world when they
// change and when it loads, and warns the console of alias conflicts.
// MV: WorldConfigNodes' onLoadAndChange hooks
func worldNodes(c *core.Core) {
	c.Worlds.OnChange(core.PropAllowWeather, func(w *core.World) {
		h, _ := w.Handle()
		if !w.Props.Bool(core.PropAllowWeather) && h.Weather() != plugin.WeatherClear {
			h.SetThundering(false, 0)
			h.SetStorm(false, 0)
		}
	})
	c.Worlds.OnChange(core.PropDifficulty, func(w *core.World) {
		h, _ := w.Handle()
		if d := core.DifficultyOf(w.Props.String(core.PropDifficulty)); h.Difficulty() != d {
			h.SetDifficulty(d)
		}
	})
	c.Worlds.OnChange(core.PropSpawnLocation, func(w *core.World) {
		h, _ := w.Handle()
		s := w.SpawnLocation()
		cur := h.SpawnPoint()
		// The vanilla dimensions share one spawn on golem: only the one
		// holding it may move it, or loading the nether would carry the
		// overworld's spawn there.
		if s == nil || cur.World == nil || cur.World.Key() != h.Key() {
			return
		}
		if want := s.Location(h); want.Pos.Block() != cur.Pos.Block() || want.Yaw != cur.Yaw || want.Pitch != cur.Pitch {
			h.SetSpawnPoint(want)
		}
	})
	warnAliases := func() {
		if !c.Config.Bool("world.warn-alias-conflicts") {
			return
		}
		console := c.Issuer(c.Server.Console())
		for _, w := range c.Worlds.All() {
			c.Worlds.SendAliasConflicts(w, console)
		}
	}
	warnAliases()
	c.OnReload(warnAliases)
}
