package portals_test

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
	"github.com/golem-mc/multiverse/portals"
)

// arena makes a Floor world "Arena" with a portal "gate" one block wide in
// x, at x 2, y 5..7, z 6..8, sending to the overworld; Alice (an operator)
// and Bob are in the arena.
func arena(t *testing.T) (env *mvtest.Env, p *portals.Portals, alice, bob *plugintest.Player) {
	t.Helper()
	env, p = start(t, nil, plugintest.Ops("Alice"))
	env.Console("mv create Arena normal -g Floor")
	env.Console("mvp config portal-cooldown 600000")
	alice = env.Join("Alice", true)
	bob = env.Join("Bob", false)
	env.Sync(func() {
		w, _ := env.MV.Worlds.Get("Arena")
		pt, _ := p.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 2, Y: 5, Z: 6}, plugin.Vec3{X: 2, Y: 7, Z: 8}))
		pt.Props.Set(mustNode(t, pt, "action"), "w:world")
	})
	env.Console("mv tp Alice Arena")
	env.Console("mv tp Bob Arena")
	env.Srv.RunTicks(3)
	alice.Messages(1)
	bob.Messages(1)
	return env, p, alice, bob
}

func mustNode(t *testing.T, pt *portals.Portal, name string) *core.Node {
	t.Helper()
	n, ok := pt.Props.Nodes().Find(name)
	if !ok {
		t.Fatalf("no portal property %s", name)
	}
	return n
}

// walkIn fires the move of pl from the arena's spawn into the portal.
func walkIn(env *mvtest.Env, pl *plugintest.Player) {
	env.T.Helper()
	env.Sync(func() {
		h := pl.Handle()
		from := h.Location()
		to := from
		to.Pos = plugin.Vec3{X: 2.5, Y: 5, Z: 7.5}
		env.MV.Server.Events().Fire(&plugin.PlayerMoveEvent{Player: h, From: from, To: to})
	})
}

// worldOf answers the world pl is in, waiting up to ticks ticks for it to
// become want.
func worldOf(env *mvtest.Env, pl *plugintest.Player, want string, ticks int) string {
	env.T.Helper()
	var w string
	for range ticks {
		env.Sync(func() { w = pl.Handle().World().Key().Path })
		if w == want {
			break
		}
		env.Srv.RunTicks(1)
	}
	return w
}

// Walking into a portal sends a player who may use it and can pay, once a
// cooldown allows; the others are told why not.
func TestWalkingIntoAPortal(t *testing.T) {
	env, _, alice, bob := arena(t)
	var fired int
	plugin.Listen(env.MV.Server, func(*portals.MVPortalEvent) { fired++ })

	walkIn(env, bob)
	env.Expect(bob.Messages(2), "You do not have permission to use this portal.")
	if w := worldOf(env, bob, "overworld", 3); w != "arena" {
		t.Fatalf("Bob without access went to %s", w)
	}

	env.Permit("Bob", true, "multiverse.portal.access.gate")
	env.Console("mvp modify gate price 2")
	env.Console("mvp modify gate currency diamond")
	walkOut(env, bob)
	walkIn(env, bob)
	env.Expect(bob.Messages(2), "Sorry, you don't have enough items. You need 2.0 DIAMOND to enter the gate portal.")

	env.Sync(func() { bob.Handle().Inventory().Add(plugin.ItemStack{Item: registry.ItemDiamond, Count: 3}) })
	walkOut(env, bob)
	walkIn(env, bob)
	if w := worldOf(env, bob, "overworld", 20); w != "overworld" {
		t.Fatalf("Bob was not sent: in %s", w)
	}
	env.Sync(func() {
		if n := bob.Handle().Inventory().Count(registry.ItemDiamond); n != 1 {
			t.Errorf("Bob has %d diamonds, want 1", n)
		}
	})
	if fired != 1 {
		t.Errorf("MVPortalEvent fired %d times, want 1", fired)
	}

	env.Console("mv tp Bob Arena")
	env.Srv.RunTicks(3)
	bob.Messages(1)
	walkOut(env, bob)
	walkIn(env, bob)
	env.Expect(bob.Messages(2), "There is a portal cooldown in effect.")

	// Debug mode shows the portal instead of using it.
	env.Expect(run(alice, "mvp debug on"), "Portal debug mode ENABLED")
	walkIn(env, alice)
	env.Expect(alice.Messages(2), "--- You are currently standing in gate ---")
	if w := worldOf(env, alice, "overworld", 3); w != "arena" {
		t.Errorf("Alice in debug mode went to %s", w)
	}
}

// walkOut fires a move back to the arena's spawn.
func walkOut(env *mvtest.Env, pl *plugintest.Player) {
	env.T.Helper()
	env.Sync(func() {
		h := pl.Handle()
		from := h.Location()
		to := from
		to.Pos = plugin.Vec3{X: 8.5, Y: 4, Z: 8.5}
		env.MV.Server.Events().Fire(&plugin.PlayerMoveEvent{Player: h, From: from, To: to})
	})
}

// Flint and steel fills a portal framed in one of frame-materials, which
// vanilla would not light, with nether portal blocks; a nether portal trip
// through it runs the portal's action instead, and buckets empty and fill it.
func TestLightingAPortal(t *testing.T) {
	env, p, alice, _ := arena(t)
	env.Console("mvp config frame-materials gold_block")
	gold := plugin.DefaultState(registry.BlockGoldBlock)
	env.Sync(func() {
		h, _ := env.MV.Worlds.Get("Arena")
		w, _ := h.Handle()
		for z := 6; z <= 8; z++ {
			w.SetBlockState(plugin.BlockPos{X: 2, Y: 4, Z: z}, gold, plugin.UpdateAll)
			w.SetBlockState(plugin.BlockPos{X: 2, Y: 8, Z: z}, gold, plugin.UpdateAll)
		}
		for y := 5; y <= 7; y++ {
			w.SetBlockState(plugin.BlockPos{X: 2, Y: y, Z: 5}, gold, plugin.UpdateAll)
			w.SetBlockState(plugin.BlockPos{X: 2, Y: y, Z: 9}, gold, plugin.UpdateAll)
		}
		pt, _ := p.Manager.Get("gate")
		if !pt.FrameValid(plugin.BlockPos{X: 2, Y: 6, Z: 7}) {
			t.Error("a gold frame is not valid")
		}
		h2 := alice.Handle()
		h2.Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 4.5, Y: 4, Z: 7.5}}, plugin.TeleportPlugin)
		h2.Inventory().Set(plugin.SlotMainHand, plugin.ItemStack{Item: registry.ItemFlintAndSteel, Count: 1})
	})
	env.Srv.RunTicks(2)
	alice.PlaceBlock(2, 4, 7, 1)
	env.Srv.RunTicks(2)
	env.Sync(func() {
		h, _ := env.MV.Worlds.Get("Arena")
		w, _ := h.Handle()
		for y := 5; y <= 7; y++ {
			for z := 6; z <= 8; z++ {
				s := w.BlockState(plugin.BlockPos{X: 2, Y: y, Z: z})
				if axis, _ := s.Prop(registry.PropAxis); s.Block() != registry.BlockNetherPortal || axis != "z" {
					t.Fatalf("(2, %d, %d) is %s axis %s, want a nether portal along z", y, z, s.Block().Name(), axis)
				}
			}
		}
		pt, _ := p.Manager.Get("gate")
		if pt.Legacy() {
			t.Error("a nether portal fill reads as legacy")
		}
		// The portal trip runs the action instead of going to the nether.
		h2 := alice.Handle()
		at := plugin.Location{World: w, Pos: plugin.Vec3{X: 2.5, Y: 5, Z: 7.5}}
		h2.Teleport(at, plugin.TeleportPlugin)
		e := &plugin.EntityPortalEvent{Entity: h2, From: at, To: at, Cause: plugin.TeleportNetherPortal}
		env.MV.Server.Events().Fire(e)
		if !e.Cancelled() {
			t.Error("the nether portal trip was not cancelled")
		}
	})
	if w := worldOf(env, alice, "overworld", 20); w != "overworld" {
		t.Errorf("Alice is in %s, want the portal's destination", w)
	}

	// Scooping a portal's fill empties it; pouring water fills it.
	env.Sync(func() {
		h, _ := env.MV.Worlds.Get("Arena")
		w, _ := h.Handle()
		at := plugin.Block{World: w, Pos: plugin.BlockPos{X: 2, Y: 6, Z: 7}}
		for _, fill := range []bool{true, false} {
			env.MV.Server.Events().Fire(&plugin.PlayerBucketEvent{Player: alice.Handle(), Block: at, Face: plugin.East,
				Fill: fill, Item: plugin.ItemStack{Item: registry.ItemWaterBucket, Count: 1}})
			want := registry.BlockAir
			if !fill {
				want = registry.BlockWater
			}
			for y := 5; y <= 7; y++ {
				if b := w.BlockState(plugin.BlockPos{X: 2, Y: y, Z: 8}).Block(); b != want {
					t.Errorf("fill %v: (2, %d, 8) is %s, want %s", fill, y, b.Name(), want.Name())
				}
			}
		}
	})

	env.Console("mvp config frame-materials obsidian")
	env.Sync(func() {
		pt, _ := p.Manager.Get("gate")
		if pt.FrameValid(plugin.BlockPos{X: 2, Y: 6, Z: 7}) {
			t.Error("a gold frame is valid with frame-materials [obsidian]")
		}
	})
}

// A nether portal trip through a Portals portal whose action runs instead
// leaves no exit portal in the nether: golem asks plugins before it searches
// for or builds one.
func TestCancelledNetherTripBuildsNoExit(t *testing.T) {
	env, p := start(t, nil, plugintest.Ops("Alice"), plugintest.WithGameMode("creative"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)
	env.Sync(func() {
		w, _ := env.MV.Worlds.Get("Arena")
		pt, _ := p.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 2, Y: 5, Z: 6}, plugin.Vec3{X: 2, Y: 7, Z: 8}))
		pt.Props.Set(mustNode(t, pt, "action"), "w:world")
	})
	env.Console("mvp config portal-cooldown 600000")
	env.Console("mv tp Alice Arena")
	env.Srv.RunTicks(3)
	alice.Messages(1)

	nether, ok := env.Srv.Host().World(plugin.Key{Namespace: "minecraft", Path: "the_nether"})
	if !ok {
		t.Fatal("no nether")
	}
	portal, _ := plugin.DefaultState(registry.BlockNetherPortal).With(registry.PropAxis, "z")
	env.Sync(func() {
		h, _ := env.MV.Worlds.Get("Arena")
		w, _ := h.Handle()
		for y := 5; y <= 7; y++ {
			for z := 6; z <= 8; z++ {
				w.SetBlockState(plugin.BlockPos{X: 2, Y: y, Z: z}, portal, 0)
			}
		}
		alice.Handle().Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 2.5, Y: 5, Z: 7.5}}, plugin.TeleportPlugin)
	})
	if w := worldOf(env, alice, "overworld", 40); w != "overworld" {
		t.Fatalf("Alice is in %s, want the portal's destination", w)
	}
	env.Sync(func() {
		// 2 / 8 and 7 / 8 scale to the nether's origin; the creation radius is 16.
		for x := -20; x <= 20; x++ {
			for z := -20; z <= 20; z++ {
				for y := nether.MinY(); y < nether.MinY()+nether.Height(); y++ {
					if nether.BlockState(plugin.BlockPos{X: x, Y: y, Z: z}).Block() == registry.BlockNetherPortal {
						t.Fatalf("a nether portal was built at %d %d %d", x, y, z)
					}
				}
			}
		}
	})
}
