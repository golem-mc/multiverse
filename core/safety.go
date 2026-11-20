package core

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// solid is Bukkit's Material.isSolid for a state.
func solid(s plugin.BlockState) bool { return registry.IsSolid(uint16(s)) }

// SafeAt reports whether a player can stand at pos: inside the border, feet
// and head free (no solid block, no fire), on a solid block that is not
// deep water. Unloaded chunks read as air. MV: BlockSafety.canSpawnAtBlockSafely
func SafeAt(h plugin.World, pos plugin.BlockPos) bool {
	if !h.Border().Contains(pos.Center()) {
		return false
	}
	body := func(p plugin.BlockPos) bool {
		s := h.BlockState(p)
		return solid(s) || s.Block() == registry.BlockFire
	}
	if body(pos) || body(pos.Offset(plugin.Up, 1)) {
		return false
	}
	below := pos.Offset(plugin.Down, 1)
	floor := h.BlockState(below)
	if !solid(floor) {
		return false
	}
	deepWater := floor.Block() == registry.BlockWater && h.BlockState(below.Offset(plugin.Down, 1)).Block() == registry.BlockWater
	return !deepWater
}

// FindSafe answers the nearest safe block to loc, searching rings of
// horizontal radius h at each height, the loc's first, then up and down to
// v blocks; centred on the block, keeping loc's facing. Chunks must be
// loaded. MV: BlockSafety.findSafeSpawnLocation
func FindSafe(loc plugin.Location, hr, vr int) (plugin.Location, bool) {
	w := loc.World
	start := loc.Pos.Block()
	found, ok := searchXZ(w, start, hr)
	minY, maxY := w.MinY(), w.MinY()+w.Height()
	for i := 1; !ok && i <= vr; i++ {
		if start.Y+i < maxY {
			found, ok = searchXZ(w, plugin.BlockPos{X: start.X, Y: start.Y + i, Z: start.Z}, hr)
		}
		if !ok && start.Y-i >= minY {
			found, ok = searchXZ(w, plugin.BlockPos{X: start.X, Y: start.Y - i, Z: start.Z}, hr)
		}
	}
	if !ok {
		return plugin.Location{}, false
	}
	return plugin.Location{World: w, Pos: plugin.Vec3{X: float64(found.X) + .5, Y: float64(found.Y), Z: float64(found.Z) + .5},
		Yaw: loc.Yaw, Pitch: loc.Pitch}, true
}

// searchXZ walks rings around b in Multiverse's order. MV: BlockSafety.searchAroundXZ
func searchXZ(w plugin.World, b plugin.BlockPos, radius int) (plugin.BlockPos, bool) {
	if SafeAt(w, b) {
		return b, true
	}
	for r := 1; r <= radius; r++ {
		radiusX, increment, offset := true, false, 0
		for range r*2 + 1 {
			var p plugin.BlockPos
			var ok bool
			if radiusX {
				p, ok = searchPlusMinus(w, b, r, offset)
			} else {
				p, ok = searchPlusMinus(w, b, offset, r)
			}
			if ok {
				return p, true
			}
			if increment {
				offset++
			}
			radiusX, increment = !radiusX, !increment
		}
	}
	return plugin.BlockPos{}, false
}

func searchPlusMinus(w plugin.World, b plugin.BlockPos, x, z int) (plugin.BlockPos, bool) {
	try := func(dx, dz int) (plugin.BlockPos, bool) {
		p := plugin.BlockPos{X: b.X + dx, Y: b.Y, Z: b.Z + dz}
		return p, SafeAt(w, p)
	}
	if p, ok := try(-x, -z); ok {
		return p, true
	}
	if z != 0 {
		if p, ok := try(-x, z); ok {
			return p, true
		}
	}
	if x != 0 {
		if p, ok := try(x, -z); ok {
			return p, true
		}
		if z != 0 {
			if p, ok := try(x, z); ok {
				return p, true
			}
		}
	}
	return plugin.BlockPos{}, false
}

// WithChunks loads (generating) the chunks within r blocks of pos in h,
// then runs fn on the tick goroutine; Bukkit reads unloaded blocks by
// loading them, golem reads them as air.
func WithChunks(h plugin.World, pos plugin.Vec3, r int, fn func()) {
	b := pos.Block()
	var chunks []plugin.ChunkPos
	for cx := (b.X - r) >> 4; cx <= (b.X+r)>>4; cx++ {
		for cz := (b.Z - r) >> 4; cz <= (b.Z+r)>>4; cz++ {
			chunks = append(chunks, plugin.ChunkPos{X: cx, Z: cz})
		}
	}
	left := len(chunks)
	for _, c := range chunks {
		h.LoadChunk(c, true, func(plugin.Chunk) {
			if left--; left == 0 {
				fn()
			}
		})
	}
}
