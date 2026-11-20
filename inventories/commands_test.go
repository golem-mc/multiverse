package inventories_test

import (
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/internal/mvtest"
)

// confirm runs the /mv confirm a prompt asks for, as run; what the
// confirmed command says goes to the sender of the prompt.
func confirm(env *mvtest.Env, prompt []string, run func(string) []string) []string {
	env.T.Helper()
	for _, l := range prompt {
		if _, cmd, ok := strings.Cut(l, "/mv confirm"); ok {
			return run("mv confirm " + strings.Fields(cmd)[0])
		}
	}
	env.T.Fatalf("no confirm prompt in %q", prompt)
	return nil
}

// The group commands change groups.yml and report as Multiverse-Inventories
// does; delete-group waits for /mv confirm.
func TestGroupCommands(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": "first-run: false\n"})
	createWorlds(env, "g1", "g2")
	env.Expect(env.Console("mvinv create-group games g1 inventory,-armor"), "You created a new group 'games'!")
	env.Expect(env.Console("mvinv create-group games"), "That group already exists! (games)")
	env.Expect(env.Console("mvinv add-worlds games nope"), "There is no world with the name: nope")
	env.Expect(env.Console("mvinv add-worlds games g1"), "World: g1 already part of Group: games")
	env.Expect(env.Console("mvinv add-worlds games G2"), "World: g2 added to Group: games")
	env.Expect(env.Console("mvinv add-shares games hunger"),
		"Group: games is now sharing: [inventory_contents, ender_chest, off_hand, food_level, saturation, exhaustion] and NOT sharing: [armor_contents, max_hit_points")
	env.Expect(env.Console("mvinv remove-shares games off_hand,ender_chest"), "is now sharing: [inventory_contents, food_level")
	env.Expect(env.Console("mvinv add-disabled-shares games saturation"),
		"Group games now has the following disabled shares: [saturation]")
	out := env.Console("mvinv add-shares games bogus")
	env.Expect(out, "Error: You did not specify any valid shares!")
	env.Expect(out, "Usage: /mvinv add-shares <group> <share[,extra]>")
	env.Expect(env.Console("mvinv add-shares nogroup hunger"), "There is no group with the name: nogroup")

	groups := readFile(t, env, "groups.yml")
	want := "  games:\n    worlds:\n      - g1\n      - g2\n    shares:\n      - inventory_contents\n      - food_level\n" +
		"      - saturation\n      - exhaustion\n    disabled-shares:\n      - saturation\n"
	if !strings.Contains(groups, want) {
		t.Errorf("groups.yml:\n%s", groups)
	}

	info := env.Console("mvinv info g1")
	env.Expect(info, "In Groups: games")
	env.Expect(info, "There is no group with the name: g1")
	info = env.Console("mvinv info games")
	env.Expect(info, "There is no world profile for the world: games")
	env.Expect(info, "Config Worlds: g1, g2")
	env.Expect(info, "Applicable Shares: inventory_contents, food_level, exhaustion")
	env.Expect(env.Console("mvinv list"), "Groups: games")

	env.Expect(env.Console("mvinv remove-worlds games g9"), "World: g9 is not part of Group: games")
	env.Expect(env.Console("mvinv remove-worlds games g2"), "World: g2 removed from Group: games")

	out = env.Console("mvinv delete-group games")
	env.Expect(out, "Are you sure you want to delete group games?")
	confirm(env, out, env.Console)
	if strings.Contains(readFile(t, env, "groups.yml"), "games") {
		t.Error("groups.yml still has the deleted group")
	}

	env.Expect(env.Console("mvinv toggle inventory_contents"), "inventory_contents is not an optional share!")
	env.Expect(env.Console("mvinv toggle last_location"), "last_location will now be considered")
	env.Expect(env.Console("mvinv config enable-bypass"), "enable-bypass is currently set to false")
	env.Expect(env.Console("mvinv config enable-bypass true"), "Successfully set enable-bypass to true")
	env.Expect(env.Console("mvinv config enable-bypass maybe"), "Unable to set enable-bypass to maybe.")
	cfg := readFile(t, env, "config.yml")
	if !strings.Contains(cfg, "enable-bypass-permissions: true") || !strings.Contains(cfg, "  active-optional-shares:\n    - last_location") {
		t.Errorf("config.yml:\n%s", cfg)
	}
	env.Expect(env.Console("mvinv check-group-conflicts"), "No group conflicts found!")
	env.Expect(env.Console("mvinv reload"), "===[ Reload Complete! ]===")

	help := env.Console("mvinv")
	env.Expect(help, "=== Showing help for /mvinv (1/")
	env.Expect(help, "/mvinv add-disabled-shares <group> <share[,extra]> - Add one or more disabled shares to a group.")
	env.Expect(env.Console("mvinvl"), "Groups: N/A")
}

// /mvinv give puts items in the stored inventory of a world the player is
// not in, and in the live one of the world they are in; bulkedit delete
// clears a share from stored profiles.
func TestGiveAndBulkEdit(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": "first-run: false\n"})
	createWorlds(env, "away")
	p := env.Join("Gifted", true)
	env.Sync(func() { p.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemStone, 1)) })

	env.Expect(env.Console("mvinv give Gifted world dirt 3"), "Gave player Gifted 3 ItemStack{DIRT x 3} in world world.")
	env.Expect(env.Console("mvinv give Gifted away oak_log 70"), "Gave player Gifted 70 Oak Log in world away.")
	env.Expect(env.Console("mvinv give Gifted away notanitem"), "Invalid Material: notanitem")
	env.Expect(env.Console("mvinv give Gifted away dirt 0"), "You have to give at least 1 item.")
	env.Expect(env.Console("mvinv give Nobody away dirt"), "No player matching Nobody could be found.")
	env.Sync(func() {
		if s := p.Handle().Inventory().Item(1); s.Item != registry.ItemDirt || s.Count != 3 {
			t.Errorf("live slot 1 = %+v, want 3 dirt", s)
		}
	})
	if f := readFile(t, env, "worlds/away/Gifted.json"); !strings.Contains(f, `"0":{"==":"org.bukkit.inventory.ItemStack","v":5023,"type":"OAK_LOG","amount":64}`) ||
		!strings.Contains(f, `"type":"OAK_LOG","amount":6}`) {
		t.Errorf("worlds/away/Gifted.json = %s", f)
	}
	tp(env, p, "away")
	env.Sync(func() {
		if s := p.Handle().Inventory().Item(0); s.Item != registry.ItemOakLog || s.Count != 64 {
			t.Errorf("away slot 0 = %+v, want 64 oak logs", s)
		}
	})

	run := func(line string) []string { return mvtest.Run(p, line) }
	out := run("mvinv bulkedit playerprofile delete inventory_contents Gifted world=world")
	env.Expect(out, "  Players: Gifted")
	env.Expect(out, "  Worlds: world")
	env.Expect(out, "  Profile Types: survival, creative, adventure, spectator")
	out = confirm(env, out, run)
	env.Expect(out, "Successfully processed 4 profiles!")
	if f := readFile(t, env, "worlds/world/Gifted.json"); strings.Contains(f, "inventoryContents") {
		t.Errorf("world's inventory was not deleted: %s", f)
	}
	env.Expect(env.Console("mvinv bulkedit playerprofile clone-world-group Gifted world=nope world=away"), "The world name nope does not have any data.")

	stats := env.Console("mvinv cache stats")
	env.Expect(stats, "Cache: playerFileCache")
	env.Expect(stats, "  hit rate: ")
	if !mvtest.Has(stats, "Cache: globalProfileCache") {
		t.Errorf("cache stats = %q", stats)
	}
}
