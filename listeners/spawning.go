package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, spawning) }

// spawning applies the spawning section's tick rates and spawn limits to
// the world, and removes the entities it forbids from a chunk as it loads.
// MV: EntitySpawnConfig.applyConfigToWorld, MVEntityListener.chunkLoad
func spawning(c *core.Core) {
	c.Worlds.OnChange(core.PropSpawning, func(w *core.World) {
		h, _ := w.Handle()
		for _, cat := range core.SpawnCategories {
			if cat != "misc" {
				applySpawnCategory(c, h, cat, w.Spawning().Category(cat))
			}
		}
	})
	plugin.Listen(c.Server, func(e *plugin.ChunkLoadEvent) {
		w, ok := c.Worlds.Of(e.Chunk.World())
		if !ok {
			return
		}
		s := w.Spawning()
		removed := 0
		for _, ent := range e.Chunk.Entities() {
			if _, player := ent.(plugin.Player); player || alwaysAllowed(ent.SpawnReason()) || s.AllowsEntity(ent.Type().Name()) {
				continue
			}
			ent.Remove()
			removed++
		}
		if removed > 0 {
			c.Debug(3, "Removed %d entities from chunk %d, %d in world %s due to spawn category settings.",
				removed, e.Chunk.Pos().X, e.Chunk.Pos().Z, w.Name())
		}
	})
}

// applySpawnCategory sets one category's tick rate and spawn limit: a
// category that spawns nothing is never ticked, and '@unset' leaves the
// world's value. MV: SpawnCategoryConfig.applyConfigToWorld
func applySpawnCategory(c *core.Core, h plugin.World, cat string, v *core.Values) {
	mc, ok := core.MobCategoryOf(cat)
	if !ok {
		return
	}
	if c.Config.Bool("world.apply-entity-spawn-rate") {
		switch rate := v.Custom("tick-rate").(int); {
		case !v.Bool("spawn") && len(v.List("exceptions")) == 0:
			h.SetTicksPerSpawn(mc, 0)
		case rate != core.SpawnUnset:
			h.SetTicksPerSpawn(mc, rate)
		}
	}
	if limit := v.Custom("spawn-limit").(int); c.Config.Bool("world.apply-entity-spawn-limit") && limit != core.SpawnUnset {
		h.SetSpawnLimit(mc, limit)
	}
}
