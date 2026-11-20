package portals

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// interior are the blocks a portal's inside may be made of.
// MV: MVPortal.INTERIOR_MATERIALS
var interior = []registry.Block{registry.BlockNetherPortal, registry.BlockVine, registry.BlockSnow,
	registry.BlockAir, registry.BlockWater, registry.BlockLava}

func isInterior(b registry.Block) bool { return slices.Contains(interior, b) }

// handle answers the loaded world the portal is in.
func (pt *Portal) handle() (plugin.World, bool) {
	w, ok := pt.World()
	if !ok {
		return nil, false
	}
	return w.Handle()
}

// Legacy reports whether the portal is not filled with nether portal
// blocks, read at the middle of its box; a portal whose world is not loaded
// counts as legacy. MV: MVPortal.isLegacyPortal
func (pt *Portal) Legacy() bool {
	h, ok := pt.handle()
	if !ok {
		return true
	}
	mid := plugin.Vec3{X: (pt.region.Min.X + pt.region.Max.X) / 2, Y: (pt.region.Min.Y + pt.region.Max.Y) / 2,
		Z: (pt.region.Min.Z + pt.region.Max.Z) / 2}
	return h.BlockState(mid.Block()).Block() != registry.BlockNetherPortal
}

// FrameValid reports whether the frame around pos, inside the portal, is
// made of one of frame-materials; any frame is valid when the list is
// empty. MV: MVPortal.isFrameValid
func (pt *Portal) FrameValid(pos plugin.BlockPos) bool {
	materials := pt.p.Config.List("portal-creation.frame-materials")
	if len(materials) == 0 {
		return true
	}
	r := pt.region
	// A frame is looked for in each flat dimension of the box, within the
	// box grown by a block in the other two.
	if r.Width() == 1 && pt.frameValidIn(pos, grow(r, 0, 1, 1), materials) {
		return true
	}
	if r.Height() == 1 && pt.frameValidIn(pos, grow(r, 1, 0, 1), materials) {
		return true
	}
	return r.Depth() == 1 && pt.frameValidIn(pos, grow(r, 1, 1, 0), materials)
}

func grow(r Region, x, y, z float64) Region {
	r.Min = plugin.Vec3{X: r.Min.X - x, Y: r.Min.Y - y, Z: r.Min.Z - z}
	r.Max = plugin.Vec3{X: r.Max.X + x, Y: r.Max.Y + y, Z: r.Max.Z + z}
	return r
}

// frameValidIn floods the inside from pos across the plane of search (flat
// in one dimension): the blocks that bound it must all be one material, in
// materials, and the flood must not reach search's edge.
func (pt *Portal) frameValidIn(pos plugin.BlockPos, search Region, materials []string) bool {
	h, ok := pt.handle()
	if !ok {
		return false
	}
	use := func(n int) int {
		if n == 1 {
			return 0
		}
		return 1
	}
	ux, uy, uz := use(search.Width()), use(search.Height()), use(search.Depth())
	if ux+uy+uz != 2 {
		return false
	}
	var common registry.Block
	found := false
	visited := map[plugin.BlockPos]bool{pos: true}
	frontier := []plugin.BlockPos{pos}
	for len(frontier) > 0 {
		at := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		b := h.BlockState(at).Block()
		if !isInterior(b) {
			if !found {
				common, found = b, true
			} else if b != common {
				return false
			}
			continue
		}
		for d1 := -1; d1 <= 1; d1++ {
			for d2 := -1; d2 <= 1; d2++ {
				if (d1 == 0) == (d2 == 0) {
					continue
				}
				dy := uy * d2
				if ux == 0 {
					dy = uy * d1
				}
				next := plugin.BlockPos{X: at.X + ux*d1, Y: at.Y + dy, Z: at.Z + uz*d2}
				if !search.Contains(nil, next) {
					return false
				}
				if !visited[next] {
					visited[next] = true
					frontier = append(frontier, next)
				}
			}
		}
	}
	return found && slices.Contains(materials, materialPath(common.Name()))
}

// fill floods the portal's plane from pos with block, through its interior
// blocks; it needs a portal one block wide or deep, and tells pl otherwise.
// MV: PortalFiller.fillRegion
func (p *Portals) fill(pt *Portal, pos plugin.BlockPos, block registry.Block, pl plugin.Player) bool {
	r := pt.region
	if r.Width() != 1 && r.Depth() != 1 {
		p.Core.Issuer(pl).Error(core.M("mv-portals.portal.fill.invalidsize", "{width}", r.Width(), "{depth}", r.Depth()))
		return false
	}
	h, ok := pt.handle()
	if !ok {
		return false
	}
	ux, uz := 1, 1
	if r.Width() == 1 {
		ux = 0
	}
	if r.Depth() == 1 {
		uz = 0
	}
	state := plugin.DefaultState(block)
	flags := plugin.UpdateAll
	if block == registry.BlockNetherPortal {
		// Without physics: the blocks have no frame the vanilla rules accept,
		// and the physics listener keeps them.
		flags = plugin.UpdateClients
		if ux == 0 {
			state, _ = state.With(registry.PropAxis, "z")
		}
	}
	p.doFill(h, r, pos, ux, uz, state, flags)
	return true
}

// doFill sets pos and spreads sideways and up and down, stopping a branch
// at the box's edge. MV: PortalFiller.doFill
func (p *Portals) doFill(h plugin.World, r Region, pos plugin.BlockPos, ux, uz int, state plugin.BlockState, flags plugin.UpdateFlags) {
	if fillable(h, pos, state) {
		h.SetBlockState(pos, state, flags)
	}
	for _, d := range [][3]int{{ux, 0, uz}, {0, 1, 0}, {-ux, 0, -uz}, {0, -1, 0}} {
		next := plugin.BlockPos{X: pos.X + d[0], Y: pos.Y + d[1], Z: pos.Z + d[2]}
		if !fillable(h, next, state) {
			continue
		}
		if !r.Contains(nil, next) {
			return
		}
		p.doFill(h, r, next, ux, uz, state, flags)
	}
}

// fillable reports whether pos is inside a portal and not already state's
// block. MV: PortalFiller.isValidPortalRegion
func fillable(h plugin.World, pos plugin.BlockPos, state plugin.BlockState) bool {
	b := h.BlockState(pos).Block()
	return b != state.Block() && isInterior(b)
}
