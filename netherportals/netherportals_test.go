package netherportals_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	_ "github.com/golem-mc/multiverse"
	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
	"github.com/golem-mc/multiverse/netherportals"
	"github.com/golem-mc/multiverse/portals"
)

// worlds is a worlds.yml of empty worlds: void with its nether and end,
// and a floor to stand on.
const worlds = `void:
  generator: Void
  version: 1.3
  read-only:
    environment: normal
void_nether:
  generator: Void
  version: 1.3
  read-only:
    environment: nether
void_the_end:
  generator: Void
  version: 1.3
  read-only:
    environment: the_end
floor:
  generator: Floor
  version: 1.3
  read-only:
    environment: normal
`

const legacyConfig = `portal-auto-link-when:
  nether:
    prefix: ''
    suffix: _nether
  end:
    prefix: ''
    suffix: _the_end
bounceback: true
teleport_entities: false
send_disabled_portal_message: false
send_no_destination_message: false
end_platform_drop_blocks: false
worlds:
  world:
    portalgoesto:
      NETHER: world_nether
      END: world_the_end
  other_world:
    portalgoesto:
      ENDER: other_world_the_end
  example:
    world:
      portalgoesto:
        NETHER: example.world_nether
version: 0.0
`

// npFile names a file in Multiverse-NetherPortals' data directory for
// mvtest.Start, which writes under Multiverse-Core's.
func npFile(name string) string { return "../multiverse-netherportals/" + name }

func start(t *testing.T, files map[string]string) (*mvtest.Env, *netherportals.NetherPortals) {
	t.Helper()
	if files == nil {
		files = map[string]string{}
	}
	if _, ok := files["worlds.yml"]; !ok {
		files["worlds.yml"] = worlds
	}
	env := mvtest.Start(t, files)
	np, ok := netherportals.Get()
	if !ok {
		t.Fatal("multiverse-netherportals did not enable")
	}
	return env, np
}

func readNP(t *testing.T, env *mvtest.Env, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(env.Dir, "plugins", "multiverse-netherportals", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A Multiverse-NetherPortals 4 config.yml is moved to 5.1's keys and its
// links to links.yml.
func TestLegacyConfigMigrates(t *testing.T) {
	env, np := start(t, map[string]string{npFile("config.yml"): legacyConfig})
	for path, want := range map[string]bool{
		netherportals.PathTeleportEntities: false, netherportals.PathSendDisabledPortalMessage: false,
		netherportals.PathSendNoDestinationMessage: false, netherportals.PathEndPlatformDropBlocks: false,
		netherportals.PathHandleEndExitRespawn: false, netherportals.PathBounceback: true,
	} {
		if got := np.Config.Bool(path); got != want {
			t.Errorf("%s = %v, want %v", path, got, want)
		}
	}
	cfg := readNP(t, env, "config.yml")
	for _, gone := range []string{"teleport_entities", "worlds:", "portalgoesto"} {
		if strings.Contains(cfg, gone) {
			t.Errorf("config.yml still has %s:\n%s", gone, cfg)
		}
	}
	for _, want := range []string{"teleport-entities: false", "version: 5.1", "# When enabled, players are pushed out of a portal"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
	for from, want := range map[string]string{"world": "world_nether", "example.world": "example.world_nether"} {
		if got, _ := np.Links.To(from, netherportals.Nether); got != want {
			t.Errorf("%s nether link = %q, want %q", from, got, want)
		}
	}
	if got, _ := np.Links.To("other_world", netherportals.End); got != "other_world_the_end" {
		t.Errorf("other_world end link = %q", got)
	}
	links := readNP(t, env, "links.yml")
	if !strings.Contains(links, "example[dot]world:\n  nether: example.world_nether\n") ||
		!strings.Contains(links, "world:\n  nether: world_nether\n  end: world_the_end\n") {
		t.Errorf("links.yml:\n%s", links)
	}

	env.Expect(env.Console("mv reload"), "Multiverse-NetherPortals - links.yml")
}

// A fresh config.yml has the current keys and defaults, and an empty
// links.yml.
func TestFreshConfig(t *testing.T) {
	env, np := start(t, nil)
	if !np.Config.Bool(netherportals.PathHandleEndExitRespawn) || np.Config.String(netherportals.PathNetherSuffix) != "_nether" {
		t.Error("fresh config.yml lacks the defaults")
	}
	if links := readNP(t, env, "links.yml"); links != "{}\n" {
		t.Errorf("links.yml = %q", links)
	}
	if got := np.AutoLink("void", netherportals.Nether); got != "void_nether" {
		t.Errorf("void's nether auto-link = %q", got)
	}
	if got := np.AutoLink("void_the_end", netherportals.End); got != "void" {
		t.Errorf("void_the_end's end auto-link = %q", got)
	}
	// As Multiverse, which chops the end affixes off a nether world here.
	if got := np.AutoLink("void_nether", netherportals.End); got != "void_nether_the_end" {
		t.Errorf("void_nether's end auto-link = %q", got)
	}
}

// /mvnp link, unlink and list, with --bidirectional and the legacy roots.
func TestLinkCommands(t *testing.T) {
	env, np := start(t, map[string]string{"config.yml": "command:\n  show-legacy-aliases: true\n"})
	env.Expect(env.Console("mvnp link nether void floor --bidirectional"),
		"The nether portals in 'void' are now linked to 'floor'.")
	if to, _ := np.Links.To("floor", netherportals.Nether); to != "void" {
		t.Errorf("--bidirectional did not link floor back: %q", to)
	}
	if links := readNP(t, env, "links.yml"); !strings.Contains(links, "floor:\n  nether: void\n") {
		t.Errorf("links.yml:\n%s", links)
	}
	out := env.Console("mvnp list nether")
	env.Expect(out, "====[ nether Portal Links ]====")
	env.Expect(out, "[nether] floor <---> void")
	env.Expect(out, "[nether] void_nether --> void (auto)")
	env.Expect(env.Console("mvnp list"), "[end] void <---> void_the_end (auto)")

	env.Expect(env.Console("mvnpl end floor floor"), "Successfully disabled end portals in world 'floor'.")
	env.Expect(env.Console("mvnp list end"), "[end] floor -- DISABLED")
	env.Expect(env.Console("mvnpu end floor"), "Successfully enabled end portals in world 'floor'.")
	env.Expect(env.Console("mvnp unlink end floor"), "The world 'floor' does not have a end portal link.")

	env.Console("mvnp link nether void void_nether")
	out = env.Console("mvnp unlink nether floor -b")
	env.Expect(out, "The nether portals in 'floor' are now unlinked from 'void'.")
	env.Expect(out, "The world 'void' nether portal does not link back to 'floor', it links to 'void_nether' instead.")

	out = env.Console("mvnp link sideways void floor")
	env.Expect(out, "Please specify one of (nether, end).")
	env.Expect(out, "Usage: /mvnp link <nether|end> [fromWorld] <toWorld>")
	env.Expect(env.Console("mvnp link nether void"), "Usage: /mvnp link")
	env.Expect(env.Console("mvnp"), "/mvnp unlink <nether|end> [fromWorld]")
}

// fire runs a portal event through the listeners and answers it.
func fire(env *mvtest.Env, e *plugin.EntityPortalEvent) *plugin.EntityPortalEvent {
	env.T.Helper()
	env.Sync(func() { env.MV.Server.Events().Fire(e) })
	return e
}

func handle(t *testing.T, env *mvtest.Env, name string) plugin.World {
	t.Helper()
	w, ok := env.MV.Worlds.Get(name)
	if !ok {
		t.Fatalf("no world %s", name)
	}
	h, ok := w.Handle()
	if !ok {
		t.Fatalf("%s is not loaded", name)
	}
	return h
}

// A portal trip goes to the linked or auto-linked world, scaled.
func TestPortalTrips(t *testing.T) {
	env, _ := start(t, map[string]string{"config.yml": "portal:\n  use-custom-portal-search: true\n  custom-portal-search-radius: 32\n"})
	alice := env.Join("Alice", true)
	p := alice.Handle()
	void, nether, end := handle(t, env, "void"), handle(t, env, "void_nether"), handle(t, env, "void_the_end")

	at := func(w plugin.World, x, y, z float64) plugin.Location {
		return plugin.Location{World: w, Pos: plugin.Vec3{X: x, Y: y, Z: z}}
	}
	e := fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(void, 80, 70, -16), To: at(void, 10, 70, -2),
		Cause: plugin.TeleportNetherPortal})
	if e.SearchRadius != 32 {
		t.Errorf("use-custom-portal-search: search radius %d", e.SearchRadius)
	}
	if e.Cancelled() || e.To.World.Key() != nether.Key() || e.To.Pos != (plugin.Vec3{X: 10, Y: 70, Z: -2}) {
		t.Errorf("void's nether portal leads to %v %v (cancelled %v)", e.To.World.Key(), e.To.Pos, e.Cancelled())
	}
	e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(nether, 10, 70, -2), To: at(void, 0, 0, 0),
		Cause: plugin.TeleportNetherPortal})
	if e.To.World.Key() != void.Key() || e.To.Pos.X != 80 || !e.CanCreatePortal {
		t.Errorf("void_nether's portal leads to %v %v", e.To.World.Key(), e.To.Pos)
	}

	e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(void, 5, 70, 5), To: at(void, 0, 0, 0),
		Cause: plugin.TeleportEndPortal})
	if e.To.World.Key() != end.Key() || e.To.Pos != (plugin.Vec3{X: 100, Y: 49, Z: 0}) || e.To.Yaw != 90 {
		t.Errorf("void's end portal leads to %v %v", e.To.World.Key(), e.To.Pos)
	}
	// The platform is built once its chunks load; read it back the same way.
	var platform registry.Block
	env.Sync(func() {
		end.LoadChunk(plugin.ChunkPos{X: 6, Z: -1}, true, func(plugin.Chunk) {
			platform = end.BlockState(plugin.BlockPos{X: 98, Y: 48, Z: -2}).Block()
		})
	})
	env.Srv.RunTicks(2)
	env.Sync(func() {
		if platform != registry.BlockObsidian {
			t.Errorf("no end platform: %v", platform)
		}
	})
	e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(end, 0, 64, 0), To: at(end, 100, 49, 0),
		Cause: plugin.TeleportEndPortal})
	if e.To.World.Key() != void.Key() || e.To.Pos.Block() != (plugin.BlockPos{X: 0, Y: 64, Z: 0}) || e.CanCreatePortal {
		t.Errorf("leaving void_the_end leads to %v %v", e.To.World.Key(), e.To.Pos)
	}

	// A world linked to itself has its portals disabled; one with no
	// destination world goes nowhere.
	env.Console("mvnp link nether void void")
	if e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(void, 0, 70, 0), To: at(nether, 0, 70, 0),
		Cause: plugin.TeleportNetherPortal}); !e.Cancelled() {
		t.Error("a disabled portal was used")
	}
	if e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(handle(t, env, "floor"), 0, 70, 0), To: at(nether, 0, 70, 0),
		Cause: plugin.TeleportNetherPortal}); !e.Cancelled() {
		t.Error("a portal with no destination was used")
	}

	// A Multiverse-Portals portal is left to Multiverse-Portals.
	env.Sync(func() {
		mvp, _ := portals.Get()
		w, _ := env.MV.Worlds.Of(void)
		mvp.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 20, Y: 69, Z: 20}, plugin.Vec3{X: 22, Y: 72, Z: 22}))
	})
	if e = fire(env, &plugin.EntityPortalEvent{Entity: p, From: at(void, 21, 70, 21), To: at(nether, 3, 70, 3),
		Cause: plugin.TeleportNetherPortal}); e.Cancelled() || e.To.World.Key() != nether.Key() || e.To.Pos.X != 3 {
		t.Errorf("a Multiverse-Portals portal was retargeted to %v %v", e.To.World.Key(), e.To.Pos)
	}
}

// A player stepping into a portal that leads nowhere is told and bounced
// back; a listener of MVPlayerTouchedPortalEvent can deny any portal.
func TestTouchingAPortal(t *testing.T) {
	env, _ := start(t, nil)
	alice := env.Join("Alice", true)
	floor := handle(t, env, "floor")
	portal, _ := plugin.DefaultState(registry.BlockNetherPortal).With(registry.PropAxis, "x")
	env.Sync(func() {
		alice.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 6.5}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(3)
	env.Sync(func() {
		floor.SetBlockState(plugin.BlockPos{X: 8, Y: 4, Z: 8}, portal, 0)
		floor.SetBlockState(plugin.BlockPos{X: 9, Y: 4, Z: 8}, portal, 0)
	})
	alice.Messages(2)
	env.Sync(func() {
		alice.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.3}}, plugin.TeleportPlugin)
	})
	out := alice.Messages(5)
	env.Expect(out, "This portal has no destination.")
	env.Expect(out, "No nether world is linked to this world, and the automatically selected world 'floor_nether' does not exist or is not loaded.")
	env.Sync(func() {
		if z := alice.Handle().Pos().Z; z > 8 {
			t.Errorf("Alice was not bounced back north: z %v", z)
		}
	})

	// A portal leading somewhere is used unless a listener denies it.
	env.Console("mvnp link nether floor void")
	denied := 0
	plugin.Listen(env.Srv.Host(), func(e *core.MVPlayerTouchedPortalEvent) {
		denied++
		e.SetCanUseThisPortal(false)
	})
	env.Srv.RunTicks(10)
	env.Sync(func() {
		alice.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.7}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(3)
	env.Sync(func() {
		if denied != 1 {
			t.Errorf("MVPlayerTouchedPortalEvent fired %d times", denied)
		}
		if z := alice.Handle().Pos().Z; z < 9 {
			t.Errorf("Alice was not bounced back south: z %v", z)
		}
	})
}

// A nether portal takes a player to the linked world, where golem builds
// the exit portal around the scaled position.
func TestNetherPortalTravel(t *testing.T) {
	// Creative crosses on the first tick in the portal, survival on the 81st.
	env := mvtest.Start(t, map[string]string{"worlds.yml": worlds, "config.yml": "world:\n  enforce-gamemode: false\n"},
		plugintest.WithGameMode("creative"))
	alice := env.Join("Alice", true)
	floor, nether := handle(t, env, "floor"), handle(t, env, "void_nether")
	env.Console("mvnp link nether floor void_nether")
	portal, _ := plugin.DefaultState(registry.BlockNetherPortal).With(registry.PropAxis, "x")
	env.Sync(func() {
		alice.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 40.5, Y: 4, Z: 6.5}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(3)
	env.Sync(func() {
		floor.SetBlockState(plugin.BlockPos{X: 40, Y: 4, Z: 8}, portal, 0)
		floor.SetBlockState(plugin.BlockPos{X: 41, Y: 4, Z: 8}, portal, 0)
		alice.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 40.5, Y: 4, Z: 8.5}}, plugin.TeleportPlugin)
	})
	var w plugin.World
	var pos plugin.Vec3
	for range 40 {
		env.Srv.RunTicks(1)
		env.Sync(func() { w, pos = alice.Handle().World(), alice.Handle().Pos() })
		if w.Key() == nether.Key() {
			break
		}
	}
	if w.Key() != nether.Key() {
		t.Fatalf("Alice is in %v, not void_nether", w.Key())
	}
	// 40 / 8 = 5: the exit is built within the creation radius of x 5.
	if pos.X < 5-17 || pos.X > 5+17 {
		t.Errorf("Alice arrived at %v", pos)
	}
	env.Sync(func() {
		if b := nether.BlockState(pos.Block()).Block(); b != registry.BlockNetherPortal {
			t.Errorf("Alice stands in %v, not an exit portal", b)
		}
	})
}

// A Multiverse-Portals portal built in a nether portal block asks
// Multiverse-Portals first: a player without its access permission is
// bounced back.
func TestPortalsDenyTouchingItsPortal(t *testing.T) {
	env, _ := start(t, nil)
	bob := env.Join("Bob", false)
	floor := handle(t, env, "floor")
	portal, _ := plugin.DefaultState(registry.BlockNetherPortal).With(registry.PropAxis, "x")
	env.Sync(func() {
		bob.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 6.5}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(3)
	env.Sync(func() {
		floor.SetBlockState(plugin.BlockPos{X: 8, Y: 4, Z: 8}, portal, 0)
		floor.SetBlockState(plugin.BlockPos{X: 9, Y: 4, Z: 8}, portal, 0)
		mvp, _ := portals.Get()
		w, _ := env.MV.Worlds.Of(floor)
		mvp.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 8, Y: 4, Z: 8}, plugin.Vec3{X: 9, Y: 4, Z: 8}))
	})
	bob.Messages(2)
	env.Sync(func() {
		bob.Handle().Teleport(plugin.Location{World: floor, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.3}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(3)
	env.Sync(func() {
		if z := bob.Handle().Pos().Z; z > 8 {
			t.Errorf("Bob was not bounced back north: z %v", z)
		}
	})
}
