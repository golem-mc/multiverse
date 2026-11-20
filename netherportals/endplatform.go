package netherportals

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// endArrival answers where an entity arrives in an end world: the vanilla
// platform at 100, 49, 0, facing west; a non-player stands on the block's
// centre. MV: EndPlatformCreator.getVanillaLocation
func endArrival(e plugin.Entity, w plugin.World) plugin.Location {
	if _, ok := e.(plugin.Player); ok {
		return plugin.Location{World: w, Pos: plugin.Vec3{X: 100, Y: 49, Z: 0}, Yaw: 90}
	}
	return plugin.Location{World: w, Pos: plugin.Vec3{X: 100.5, Y: 50, Z: 0.5}, Yaw: 90}
}

// createEndPlatform builds the 5x5 obsidian platform under 100, 49, 0 and
// clears three blocks above it, dropping what it clears when drop is set.
// Each chunk is built once it is loaded. MV: EndPlatformCreator.createEndPlatform
func createEndPlatform(w plugin.World, drop bool) {
	const cx, y, cz = 100, 49, 0
	build := func(c plugin.Chunk) {
		if c == nil {
			return
		}
		obsidian := plugin.DefaultState(registry.BlockObsidian)
		air := plugin.DefaultState(registry.BlockAir)
		for x := cx - 2; x <= cx+2; x++ {
			for z := cz - 2; z <= cz+2; z++ {
				floor := plugin.BlockPos{X: x, Y: y - 1, Z: z}
				if floor.Chunk() != c.Pos() {
					continue
				}
				if w.BlockState(floor).Block() != registry.BlockObsidian {
					w.SetBlockState(floor, obsidian, plugin.UpdateAll)
				}
				for dy := 1; dy <= 3; dy++ {
					pos := floor.Offset(plugin.Up, dy)
					b := w.BlockState(pos).Block()
					if b == registry.BlockAir {
						continue
					}
					w.SetBlockState(pos, air, plugin.UpdateAll)
					// golem has no loot-table break for plugins: the block
					// drops its own item.
					if item, ok := b.Item(); ok && drop {
						w.DropItem(pos.Center(), plugin.ItemStack{Item: item, Count: 1}, true)
					}
				}
			}
		}
	}
	seen := map[plugin.ChunkPos]bool{}
	for _, z := range []int{cz - 2, cz + 2} {
		pos := plugin.BlockPos{X: cx, Y: y, Z: z}.Chunk()
		if seen[pos] {
			continue
		}
		seen[pos] = true
		if c, ok := w.Chunk(pos); ok {
			build(c)
		} else {
			w.LoadChunk(pos, true, build)
		}
	}
}
