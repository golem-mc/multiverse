package portals_test

import (
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	_ "github.com/golem-mc/multiverse"
	"github.com/golem-mc/multiverse/internal/mvtest"
	"github.com/golem-mc/multiverse/portals"
)

// start starts a server with Multiverse and Portals; files are written
// under plugins/multiverse-portals.
func start(t *testing.T, files map[string]string, opts ...plugintest.Option) (*mvtest.Env, *portals.Portals) {
	t.Helper()
	in := map[string]string{}
	for name, content := range files {
		in["../multiverse-portals/"+name] = content
	}
	env := mvtest.Start(t, in, opts...)
	p, ok := portals.Get()
	if !ok {
		t.Fatal("multiverse-portals did not enable")
	}
	return env, p
}

// run runs a command as p and answers what they are sent in 2 ticks.
func run(p *plugintest.Player, line string) []string {
	p.Run("/" + line)
	return p.Messages(2)
}

func readPortals(env *mvtest.Env, name string) string {
	return env.ReadFile("../multiverse-portals/" + name)
}

// A Multiverse-Portals 4 config.yml and portals.yml are moved to version
// 5's keys; a portal whose world is unknown stays in the file unloaded.
func TestMigratesPortalsFiles(t *testing.T) {
	env, p := start(t, map[string]string{
		"config.yml": "wand: 271\nportalcooldown: 2500\nenforceportalaccess: false\n",
		"portals.yml": `portals:
  gate:
    world: world
    location: '1,2,3:4,6,3'
    destination: w:world
    owner: Alice
    safeteleport: false
    price: 5.0
    currency: diamond
  lost:
    location: nowhere:0.0,0.0,0.0:1.0,1.0,1.0
    owner: Bob
`,
	})
	if got := p.Config.Int("portal-usage.portal-cooldown"); got != 2500 {
		t.Errorf("portal-cooldown = %d, want 2500", got)
	}
	if p.Config.Bool("portal-usage.enforce-portal-access") || p.Config.Bool("portal-usage.send-no-permission-messages") {
		t.Error("enforce-portal-access and send-no-permission-messages should be false after the migration")
	}
	gate, ok := p.Manager.Get("gate")
	if !ok {
		t.Fatal("gate did not load")
	}
	if gate.Action() != "w:world" || gate.ActionType() != "multiverse-destination" || gate.SafeTeleport() ||
		gate.Price() != 5 || gate.Currency() != "diamond" || gate.Owner() != "Alice" {
		t.Errorf("gate = %v", gate.Props.YAML(false))
	}
	if r, _ := gate.Region(); r.String() != "world:1.0,2.0,3.0:4.0,6.0,3.0" || r.Width() != 4 || r.Height() != 5 || r.Depth() != 1 {
		t.Errorf("gate's region = %s", r)
	}
	if _, ok := p.Manager.Get("lost"); ok {
		t.Error("a portal in an unknown world loaded")
	}
	file := readPortals(env, "portals.yml")
	for _, want := range []string{"  gate:\n    action:\n      type: multiverse-destination\n      value: w:world\n",
		"location: world:1,2,3:4,6,3", "    version: 1.3", "  lost:\n", "nowhere:0.0,0.0,0.0:1.0,1.0,1.0"} {
		if !strings.Contains(file, want) {
			t.Errorf("portals.yml lacks %q:\n%s", want, file)
		}
	}
	cfg := readPortals(env, "config.yml")
	for _, want := range []string{"  wand-material: wooden_pickaxe\n", "  portal-cooldown: 2500\n", "version: 5.1\n",
		"# The time (in milliseconds) a player must wait between using a mvportal."} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
}

// select clicks two corners with the wand, as the left and right clicks
// of a player holding it.
func selectWith(env *mvtest.Env, p *plugintest.Player, a, b plugin.BlockPos) []string {
	env.T.Helper()
	p.StartBreak(int32(a.X), int32(a.Y), int32(a.Z))
	out := p.Messages(2)
	p.PlaceBlock(int32(b.X), int32(b.Y), int32(b.Z), 1)
	return append(out, p.Messages(2)...)
}

// A player selects a box with the wand and makes, lists, inspects,
// modifies, selects and removes a portal there.
func TestPortalCommands(t *testing.T) {
	env, p := start(t, nil, plugintest.Ops("Alice"))
	alice := env.Join("Alice", true)
	var at plugin.BlockPos
	env.Sync(func() {
		h := alice.Handle()
		at = h.Location().Pos.Block()
		h.Inventory().Set(plugin.SlotMainHand, plugin.ItemStack{Item: registry.ItemWoodenPickaxe, Count: 1})
	})
	floor := at.Offset(plugin.Down, 1)
	out := selectWith(env, alice, floor, floor.Offset(plugin.East, 2))
	env.Expect(out, "First position set to: (")
	env.Expect(out, "Second position set to: (")
	env.Expect(out, "(3 blocks)")

	env.Expect(run(alice, "mvp create bad.name"), "Invalid portal name")
	out = run(alice, "mvp create gate w:world")
	env.Expect(out, "New portal (gate) created and selected!")
	env.ExpectNot(out, "Portal action not set")
	env.Expect(run(alice, "mvp create gate"), "It already existed and has been selected.")
	env.Expect(run(alice, "mvp create other"), "Portal action not set.")

	out = env.Console("mvp list")
	env.Expect(out, "2 Portals - Page 1/1")
	env.Expect(out, "gate -> (World) world")
	env.Expect(out, "other -> ")
	env.Expect(env.Console("mvp list oth"), "1 Portals [oth] - Page 1/1")

	out = run(alice, "mvp info gate")
	env.Expect(out, "--- Portal Info gate ---")
	env.Expect(out, "Configured Action Type: multiverse-destination")
	env.Expect(out, "Teleports to w:world")
	env.Expect(out, "Price: FREE!")
	out = env.Console("mvp info other")
	env.Expect(out, "--- Portal Info: other ---")
	env.Expect(out, "Configured Action: NOT SET!")
	env.Expect(out, "Please specific a multiverse destination")

	// other is selected; modify works on it without naming it.
	env.Expect(run(alice, "mvp modify dest e:@here"), "Property dest of Portal other was set to e:world:")
	env.Expect(env.Console("mvp modify gate price 3"), "Property price of Portal gate was set to 3.0")
	out = env.Console("mvp modify gate action-type nothing")
	env.Expect(out, "was set to nothing")
	env.Expect(out, "does NOT modify")
	out = env.Console("mvp modify gate dest w:world")
	env.Expect(out, "Property dest of Portal gate was NOT set to w:world")
	env.Expect(out, "Unknown action type 'nothing'. Supported types are: command, multiverse-destination, server")
	env.Expect(env.Console("mvp modify gate action-type command"), "was set to command")
	env.Expect(env.Console("mvp modify gate action say hi %player%"), "was set to say hi %player%")
	env.Expect(run(alice, "mvp info gate"), "Runs command myself /say hi Alice")
	env.Expect(env.Console("mvp modify gate nope 1"), "Node not found in config: nope")

	env.Expect(run(alice, "mvp select"), "You have selected: other")
	env.Expect(run(alice, "mvp select gate"), "Portal: gate has been selected.")
	env.Expect(env.Console("mvp remove gate"), "Portal gate was removed successfully!")
	env.Expect(env.Console("mvp remove gate"), "does NOT exist!")
	file := readPortals(env, "portals.yml")
	if strings.Contains(file, "gate") || !strings.Contains(file, "owner: Alice") {
		t.Errorf("portals.yml after the removal:\n%s", file)
	}

	env.Expect(env.Console("mvp config portal-cooldown 500"), "Successfully set config 'portal-cooldown' to '500'.")
	env.Expect(env.Console("mvp config portal-cooldown"), "Config 'portal-cooldown' is currently set to '500'.")
	if !strings.Contains(readPortals(env, "config.yml"), "portal-cooldown: 500") {
		t.Error("config.yml was not saved")
	}

	if all := p.Manager.All(); len(all) != 1 || all[0].Name() != "other" {
		t.Errorf("portals = %v", p.Manager.All())
	}
}

// The p: destination sends a player to the middle of a portal, standing on
// its floor; rp: to one of the portals named.
func TestPortalDestination(t *testing.T) {
	env, p := start(t, nil, plugintest.Ops("Alice"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)
	env.Sync(func() {
		w, _ := env.MV.Worlds.Get("Arena")
		p.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 2, Y: 4, Z: 6}, plugin.Vec3{X: 4, Y: 6, Z: 6}))
	})
	env.Expect(mvtest.Run(alice, "mv tp p:gate"), "Teleported you to p:gate")
	env.Sync(func() {
		loc := alice.Handle().Location()
		if loc.World.Key().Path != "arena" || loc.Pos != (plugin.Vec3{X: 3.5, Y: 4, Z: 6.5}) {
			t.Errorf("Alice at %v %v, want arena (3.5, 4, 6.5)", loc.World.Key(), loc.Pos)
		}
	})
	env.Expect(mvtest.Run(alice, "mv tp p:nothing"), "Portal 'nothing' does not exist!")
	env.Expect(mvtest.Run(alice, "mv tp world"), "Teleported you to world")
	env.Expect(mvtest.Run(alice, "mv tp rp:gate"), "Teleported you to rp:gate")
	env.Sync(func() {
		if w := alice.Handle().World().Key().Path; w != "arena" {
			t.Errorf("rp:gate left Alice in %s", w)
		}
	})
	got := env.MV.Commands.Suggest(alice.Handle(), nil, "tp p:")
	if !slicesHave(got, "tp p:gate") {
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

// The Multiverse-Portals 4 alias /mvpl lists portals like /mvp list.
func TestLegacyListAlias(t *testing.T) {
	env, p := start(t, nil)
	env.Sync(func() {
		w, _ := env.MV.Worlds.Get("world")
		p.Manager.Create("gate", "Alice", portals.NewRegion(w, plugin.Vec3{X: 2, Y: 5, Z: 6}, plugin.Vec3{X: 2, Y: 7, Z: 8}))
	})
	out := env.Console("mvpl")
	env.Expect(out, "1 Portals - Page 1/1")
	env.Expect(out, "gate -> ")
}
