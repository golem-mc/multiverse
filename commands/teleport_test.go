package commands_test

import (
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// where answers the world name and block position p stands at.
func where(env *mvtest.Env, p *plugintest.Player) (world string, pos plugin.BlockPos) {
	env.T.Helper()
	env.Sync(func() {
		h := p.Handle()
		world = h.World().Key().Path
		pos = h.Location().Pos.Block()
	})
	return world, pos
}

// /mv tp sends players to world spawns, exact spots and other players,
// checking the safety of the spot unless told not to.
func TestTeleportDestinations(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)

	env.Expect(mvtest.Run(alice, "mv tp Arena"), "Teleported you to Arena.")
	if w, pos := where(env, alice); w != "arena" || pos != (plugin.BlockPos{X: 8, Y: 4, Z: 8}) {
		t.Fatalf("Alice at %s %v", w, pos)
	}
	// An exact destination is taken as it is, even inside the floor.
	env.ExpectNot(mvtest.Run(alice, "mv tp e:Arena:2,2,2 --silent"), "Teleported")
	if w, pos := where(env, alice); w != "arena" || pos != (plugin.BlockPos{X: 2, Y: 2, Z: 2}) {
		t.Errorf("exact destinations skip the safety search: Alice at %s %v", w, pos)
	}
	env.Expect(mvtest.Run(alice, "mvtp w:Arena:north"), "Teleported you to Arena.")
	env.Sync(func() {
		if yaw := alice.Handle().Location().Yaw; yaw != 180 && yaw != -180 {
			t.Errorf("yaw %v, want 180", yaw)
		}
	})

	env.Expect(env.Console("mv tp Nobody world"), "Player(s) Nobody not found.")
	env.Expect(env.Console("mv tp world"), "Invalid player: world. Either specify an online player")
	env.Expect(env.Console("mv tp Alice nowhere"), "World 'nowhere' does not exist or is not loaded!")
	env.Expect(env.Console("mv tp Alice x:y"), "Invalid destination id: x.")
	env.Console("mv tp Alice world")
	env.Srv.RunTicks(3)
	if w, _ := where(env, alice); w != "overworld" {
		t.Errorf("console tp left Alice in %s", w)
	}

	out := env.Console("mv check Alice e:Arena:1,5,1:10:20")
	env.Expect(out, "Alice can teleport to e:Arena:1.0,5.0,1.0:10.0:20.0")
	env.Expect(out, "The destination's location is: Arena:1.00,5.00,1.00:20.00:10.00")

	got := env.MV.Commands.Suggest(env.MV.Server.Console(), nil, "tp Alice ")
	if !slicesHave(got, "tp Alice Arena", "tp Alice world_nether") {
		t.Errorf("tp suggestions = %q", got)
	}
}

func slicesHave(got []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || g == w
		}
		if !found {
			return false
		}
	}
	return true
}

// Entering a world checks its access permission, player limit, blacklist
// and item entry fee, and charges the fee.
func TestEntryChecks(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"config.yml": "world:\n  enforce-access: true\n"})
	env.Console("mv create Arena normal -g Floor")
	bob := env.Join("Bob", false)
	// With use-finer-teleport-permissions, the command needs any finer node
	// and the trip its own.
	env.Permit("Bob", true, "multiverse.teleport.self.pl.Bob", "multiverse.teleport.self.w.Arena")

	env.Expect(mvtest.Run(bob, "mv tp Arena"), "You do not have permissions to access the world.")
	env.Permit("Bob", true, "multiverse.access.Arena")

	arena, _ := env.MV.Worlds.Get("Arena")
	env.Sync(func() { _ = arena.Set("player-limit", "0") })
	env.Expect(mvtest.Run(bob, "mv tp Arena"), "The world has reached its player limit.")
	env.Sync(func() {
		_ = arena.Set("player-limit", "-1")
		_ = arena.Set("world-blacklist", "world")
	})
	env.Expect(mvtest.Run(bob, "mv tp Arena"), "The world 'world' is blacklisted.")
	env.Sync(func() {
		_ = arena.Props.Reset(propNode(t, arena, "world-blacklist"))
		_ = arena.Set("entryfee-enabled", "true")
		_ = arena.Set("entryfee-amount", "2")
		_ = arena.Set("entryfee-currency", "diamond")
	})
	env.Expect(mvtest.Run(bob, "mv tp Arena"), "You are required to pay 2.0 DIAMOND.")
	env.Sync(func() { bob.Handle().Inventory().Add(plugin.ItemStack{Item: registry.ItemDiamond, Count: 3}) })
	out := mvtest.Run(bob, "mv tp Arena")
	env.Expect(out, "You have been charged 2.0 DIAMOND.")
	env.Expect(out, "Teleported you to Arena.")
	env.Sync(func() {
		if n := bob.Handle().Inventory().Count(registry.ItemDiamond); n != 1 {
			t.Errorf("Bob has %d diamonds, want 1", n)
		}
	})
}

func propNode(t *testing.T, w *core.World, name string) *core.Node {
	n, ok := w.Props.Nodes().Find(name)
	if !ok {
		t.Fatalf("no property %s", name)
	}
	return n
}

// /mv spawn sends a player to their world's spawn; /mv setspawn moves it.
func TestSpawnAndSetSpawn(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)
	mvtest.Run(alice, "mv tp e:Arena:3,4,3")
	env.Expect(mvtest.Run(alice, "mv setspawn"), "Successfully set spawn in Arena to 3.00, 4.00, 3.00")
	env.Expect(mvtest.Run(alice, "mv setspawn Arena:5,1,5"), "The new spawn location is unsafe!")
	env.Expect(mvtest.Run(alice, "mv setspawn Arena:5,4,5,90,0"), "Successfully set spawn in Arena to 5.00, 4.00, 5.00, pitch:0.00, yaw:90.00!")
	if !strings.Contains(env.ReadFile("worlds.yml"), "x: 5.0") {
		t.Error("worlds.yml lacks the new spawn")
	}
	mvtest.Run(alice, "mv tp e:Arena:9,4,9")
	env.Expect(mvtest.Run(alice, "mv spawn"), "Teleported you to 'Arena' spawn!")
	if _, pos := where(env, alice); pos != (plugin.BlockPos{X: 5, Y: 4, Z: 5}) {
		t.Errorf("Alice at %v after /mv spawn", pos)
	}
}

// Anchors are set, listed, used as destinations and deleted, and kept in
// anchors.yml.
func TestAnchors(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)
	mvtest.Run(alice, "mv tp e:Arena:6,4,6")
	env.Expect(mvtest.Run(alice, "mv anchor set home"), "Anchor home set to Arena:6.00,4.00,6.00:0.00:0.00")
	env.Expect(env.Console("mv anchor set bad nowhere"), "Failed to set anchor bad.")
	env.Expect(env.Console("mv anchor list"), "home - Arena:6.00,4.00,6.00:0.00:0.00")
	if !strings.Contains(env.ReadFile("anchors.yml"), "home: Arena:6.00,4.00,6.00:0.00:0.00") {
		t.Errorf("anchors.yml:\n%s", env.ReadFile("anchors.yml"))
	}
	mvtest.Run(alice, "mv tp world")
	env.Expect(mvtest.Run(alice, "mv tp a:home"), "Teleported you to anchor 'home'.")
	if w, pos := where(env, alice); w != "arena" || pos != (plugin.BlockPos{X: 6, Y: 4, Z: 6}) {
		t.Errorf("Alice at %s %v", w, pos)
	}
	env.Expect(env.Console("mv anchor delete home"), "Anchor home deleted.")
	env.Expect(env.Console("mv anchor delete home"), "The anchor 'home' does not exist.")
}

// A returning player goes to join-destination, unless they hold the bypass.
func TestJoinDestination(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"config.yml": "spawn:\n  enable-join-destination: true\n  join-destination: e:Arena:4,4,4\n"})
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", false)
	if w, _ := where(env, alice); w != "overworld" {
		t.Fatalf("a first join went to %s", w)
	}
	alice.Disconnect()
	env.Srv.RunTicks(2)
	alice = env.Join("Alice", false)
	env.Srv.RunTicks(3)
	if w, pos := where(env, alice); w != "arena" || pos != (plugin.BlockPos{X: 4, Y: 4, Z: 4}) {
		t.Errorf("joining Alice at %s %v", w, pos)
	}
}
