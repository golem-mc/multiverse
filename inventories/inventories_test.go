package inventories_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	_ "github.com/golem-mc/multiverse"
	"github.com/golem-mc/multiverse/internal/mvtest"
	"github.com/golem-mc/multiverse/inventories"
)

// inv is where a test writes Multiverse-Inventories' files, relative to
// Multiverse-Core's data directory.
const inv = "../multiverse-inventories/"

func start(t *testing.T, files map[string]string) (*mvtest.Env, *inventories.Module) {
	t.Helper()
	env := mvtest.Start(t, files)
	m, ok := inventories.Get()
	if !ok {
		t.Fatal("multiverse-inventories did not enable")
	}
	return env, m
}

func readFile(t *testing.T, env *mvtest.Env, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(env.Dir, "plugins", "multiverse-inventories", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// createWorlds makes cheap empty worlds.
func createWorlds(env *mvtest.Env, names ...string) {
	env.T.Helper()
	for _, n := range names {
		env.Expect(env.Console("mv create "+n+" normal -g Void"), "World '"+n+"' created!")
	}
}

// tp moves the player to a world's spawn and lets the world change settle.
func tp(env *mvtest.Env, p *plugintest.Player, world string) {
	env.T.Helper()
	w, ok := env.MV.Worlds.Get(world)
	if !ok {
		env.T.Fatalf("no world %s", world)
	}
	env.Sync(func() {
		spawn, _ := w.Spawn()
		p.Handle().Teleport(spawn, plugin.TeleportCommand)
	})
	env.Srv.RunTicks(2)
}

// A fresh server writes config.yml with Multiverse-Inventories' options and
// a default group of the vanilla worlds sharing everything.
func TestFreshServerWritesConfigAndDefaultGroup(t *testing.T) {
	env, _ := start(t, nil)
	env.Srv.RunTicks(2)
	cfg := readFile(t, env, "config.yml")
	for _, want := range []string{"share-handling:\n  # If this is set to true, it will enable bypass permissions",
		"  enable-bypass-permissions: false", "  active-optional-shares: []", "    inventory: 36",
		"first-run: false", "version: 5.0", "use-byte-serialization-for-inventory-data: false"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
	groups := readFile(t, env, "groups.yml")
	want := "groups:\n  default:\n    worlds:\n      - world\n      - world_nether\n      - world_the_end\n    shares:\n      - all\n"
	if !strings.Contains(groups, "# Multiverse-Inventories Groups") || !strings.Contains(groups, want) {
		t.Errorf("groups.yml:\n%s", groups)
	}
}

// Worlds of one group share inventories and stats; a world in no group has
// its own, and each is saved in Multiverse-Inventories' JSON files.
func TestWorldChangeSwitchesData(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "groups.yml": `groups:
  creative:
    worlds:
    - c1
    - c2
    shares:
    - inventory
    - hunger
`, inv + "config.yml": "first-run: false\n"})
	createWorlds(env, "c1", "c2", "lone")
	p := env.Join("Alex", true)
	tp(env, p, "c1")
	sword := plugin.NewStack(registry.ItemDiamondSword, 1)
	sharp := map[plugin.Enchantment]int{}
	env.Sync(func() {
		h := p.Handle()
		key, _ := env.MV.Server.Registries().Enchantment(plugin.MinecraftKey("sharpness"))
		sharp[key] = 3
		sword.Patch.Enchantments = sharp
		h.Inventory().SetItem(0, sword)
		h.Inventory().SetItem(1, plugin.NewStack(registry.ItemDirt, 40))
		h.EnderChest().SetItem(3, plugin.NewStack(registry.ItemApple, 2))
		h.SetFood(7)
		h.SetLevel(5)
	})
	tp(env, p, "lone")
	env.Sync(func() {
		h := p.Handle()
		if !h.Inventory().Item(0).Empty() || !h.EnderChest().Item(3).Empty() {
			t.Error("the lone world kept c1's inventory")
		}
		if h.Food() != 20 || h.Level() != 0 {
			t.Errorf("lone world stats: food %d level %d", h.Food(), h.Level())
		}
		h.Inventory().SetItem(0, plugin.NewStack(registry.ItemStone, 3))
	})
	tp(env, p, "c2")
	env.Sync(func() {
		h := p.Handle()
		got := h.Inventory().Item(0)
		if got.Item != registry.ItemDiamondSword || got.Patch.Enchantments == nil {
			t.Errorf("c2 slot 0 = %+v, want the enchanted sword", got)
		}
		if h.Inventory().Item(1).Count != 40 || h.EnderChest().Item(3).Count != 2 || h.Food() != 7 {
			t.Errorf("c2 lost the group's data: dirt %d apples %d food %d",
				h.Inventory().Item(1).Count, h.EnderChest().Item(3).Count, h.Food())
		}
		// Level is not shared by the group: c2's own, never set.
		if h.Level() != 0 {
			t.Errorf("level %d carried into c2", h.Level())
		}
	})
	tp(env, p, "lone")
	env.Sync(func() {
		if s := p.Handle().Inventory().Item(0); s.Item != registry.ItemStone || s.Count != 3 {
			t.Errorf("lone slot 0 = %+v", s)
		}
	})

	var group map[string]map[string]any
	if err := json.Unmarshal([]byte(readFile(t, env, "groups/creative/Alex.json")), &group); err != nil {
		t.Fatal(err)
	}
	contents, _ := group["SURVIVAL"]["inventoryContents"].(map[string]any)
	if s, ok := contents["0"].(string); !ok || s == "" {
		t.Errorf("the enchanted sword is not in Paper's byte form: %v", contents["0"])
	}
	dirt, _ := contents["1"].(map[string]any)
	if dirt["=="] != "org.bukkit.inventory.ItemStack" || dirt["type"] != "DIRT" || dirt["amount"] != 40.0 {
		t.Errorf("dirt = %v", dirt)
	}
	if stats, _ := group["SURVIVAL"]["stats"].(map[string]any); stats["fl"] != "7" {
		t.Errorf("group stats = %v", group["SURVIVAL"]["stats"])
	}
	lone := readFile(t, env, "worlds/lone/Alex.json")
	if !strings.Contains(lone, `"type":"STONE","amount":3`) || !strings.Contains(lone, `"hp":"20.0"`) {
		t.Errorf("worlds/lone/Alex.json = %s", lone)
	}
}

// With enable-gamemode-share-handling each game mode has its own inventory.
func TestGameModeChangeSwitchesData(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": `share-handling:
  enable-gamemode-share-handling: true
first-run: false
`})
	p := env.Join("Gamer", true)
	env.Sync(func() { p.Handle().Inventory().SetItem(4, plugin.NewStack(registry.ItemBread, 5)) })
	env.Sync(func() { p.Handle().SetGameMode(plugin.Creative) })
	env.Sync(func() {
		if !p.Handle().Inventory().Item(4).Empty() {
			t.Error("creative kept the survival inventory")
		}
		p.Handle().Inventory().SetItem(4, plugin.NewStack(registry.ItemStone, 1))
	})
	env.Sync(func() { p.Handle().SetGameMode(plugin.Survival) })
	env.Sync(func() {
		if s := p.Handle().Inventory().Item(4); s.Item != registry.ItemBread || s.Count != 5 {
			t.Errorf("survival slot 4 = %+v", s)
		}
	})
	data := readFile(t, env, "worlds/world/Gamer.json")
	if !strings.Contains(data, `"SURVIVAL":{`) || !strings.Contains(data, `"CREATIVE":{`) {
		t.Errorf("worlds/world/Gamer.json = %s", data)
	}
}

// A Multiverse-Inventories 4 config.yml is migrated, its groups moving to
// groups.yml; a player's file from a Bukkit server is read, and the
// global profile follows the player's world.
func TestOldConfigAndBukkitDataAreRead(t *testing.T) {
	env, m := start(t, map[string]string{inv + "config.yml": `settings:
  first_run: false
  use_bypass: true
  save_load_on_log_in_out: true
shares:
  use_optionals:
  - last_location
groups:
  old:
    worlds:
    - world
    - r=bukkit_.*
    shares:
    - all
    - -economy
    spawn:
      world: world
      priority: high
`, inv + "groups/old/Steve.json": `{"SURVIVAL":{"inventoryContents":{"2":{"==":"org.bukkit.inventory.ItemStack","v":4189,"type":"COMPARATOR","amount":7}},` +
		`"lastLocation":{"==":"org.bukkit.Location","world":"world","x":-39.5,"y":72.0,"z":0.5,"pitch":0.0,"yaw":0.0},` +
		`"potions":[{"==":"PotionEffect","effect":"minecraft:speed","duration":600,"amplifier":1,"ambient":false,"has-particles":true,"has-icon":true}],` +
		`"stats":{"fl":"11","hp":"13.5","el":"3"}}}`,
		// Saved on Bukkit with apply-playerdata-on-join, under
		// Multiverse-Inventories 2's name for the file.
		inv + "players/Steve.json": `{"playerData":{"lastWorld":"world","lastKnownName":"Steve","loadOnLogin":true}}`})
	cfg := readFile(t, env, "config.yml")
	for _, want := range []string{"enable-bypass-permissions: true", "apply-playerdata-on-join: true",
		"active-optional-shares:\n    - last_location"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "use_bypass") || strings.Contains(cfg, "\ngroups:") {
		t.Errorf("config.yml kept old keys:\n%s", cfg)
	}
	g, ok := m.Groups.Get("old")
	if !ok || !g.ContainsWorld("world") || g.SpawnWorld() != "world" || g.IsSharing(inventories.Economy) ||
		!g.IsSharing(inventories.LastLocation) || g.IsSharing(inventories.Recipes) {
		t.Fatalf("group old = %v", g)
	}
	groups := readFile(t, env, "groups.yml")
	if !strings.Contains(groups, "      - r=bukkit_.*") || !strings.Contains(groups, "      priority: HIGH") {
		t.Errorf("groups.yml:\n%s", groups)
	}

	p := env.Join("Steve", false)
	id := p.Handle().UUID().String()
	env.Srv.RunTicks(2)
	env.Sync(func() {
		h := p.Handle()
		if s := h.Inventory().Item(2); s.Item != registry.ItemComparator || s.Count != 7 {
			t.Errorf("slot 2 = %+v", s)
		}
		if h.Food() != 11 || h.Health() != 13.5 || h.Level() != 3 {
			t.Errorf("food %d health %v level %d", h.Food(), h.Health(), h.Level())
		}
		if len(h.Effects()) != 1 || h.Effects()[0].Amplifier != 1 {
			t.Errorf("effects %+v", h.Effects())
		}
	})
	p.Disconnect()
	env.Srv.RunTicks(2)
	global := readFile(t, env, "players/"+id+".json")
	if global != `{"playerData":{"lastWorld":"world","lastKnownName":"Steve","loadOnLogin":true}}` {
		t.Errorf("global profile = %s", global)
	}
	if names := readFile(t, env, "playernames.json"); !strings.Contains(names, `"Steve"`) {
		t.Errorf("playernames.json = %s", names)
	}
	steve := readFile(t, env, "groups/old/Steve.json")
	if !strings.Contains(steve, `"lastLocation":{"==":"UnloadedWorldLocation","world":"world"`) {
		t.Errorf("groups/old/Steve.json = %s", steve)
	}
}

// With last_location active a player entering a world returns to where they
// left it.
func TestLastLocationIsRestored(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": `share-handling:
  active-optional-shares: [last_location]
first-run: false
`})
	createWorlds(env, "here", "there")
	p := env.Join("Walker", true)
	tp(env, p, "here")
	env.Sync(func() {
		h := p.Handle()
		h.Teleport(plugin.Location{World: h.World(), Pos: plugin.Vec3{X: 5.5, Y: 70, Z: -3.5}, Yaw: 90}, plugin.TeleportCommand)
	})
	env.Srv.RunTicks(2)
	tp(env, p, "there")
	tp(env, p, "here")
	env.Sync(func() {
		if pos := p.Handle().Location().Pos; pos.X != 5.5 || pos.Y != 70 || pos.Z != -3.5 {
			t.Errorf("back in here at %v, want where they left", pos)
		}
	})
	data := readFile(t, env, "worlds/here/Walker.json")
	if !strings.Contains(data, `"lastLocation":{"==":"UnloadedWorldLocation","world":"here","x":5.5,"y":70.0,"z":-3.5,"yaw":90.0,"pitch":0.0}`) {
		t.Errorf("worlds/here/Walker.json = %s", data)
	}
}
