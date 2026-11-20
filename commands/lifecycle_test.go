package commands_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// confirm answers a console /mv confirm prompt with its code.
func confirm(env *mvtest.Env, prompt []string) []string {
	env.T.Helper()
	re := regexp.MustCompile(`/mv confirm (\d{3})`)
	for _, l := range prompt {
		if m := re.FindStringSubmatch(l); m != nil {
			return env.Console("mv confirm " + m[1])
		}
	}
	env.T.Fatalf("no confirm prompt in %q", prompt)
	return nil
}

func world(env *mvtest.Env, name string) *core.World {
	env.T.Helper()
	w, ok := env.MV.Worlds.Get(name)
	if !ok {
		env.T.Fatalf("world %s is not known", name)
	}
	return w
}

// saveWorld writes a world's files, as an autosave would.
func saveWorld(env *mvtest.Env, w *core.World) {
	env.T.Helper()
	env.Sync(func() {
		if h, ok := w.Handle(); ok {
			h.Save(true)
		}
	})
}

// /mv create makes a world with its flags and properties; unload, load and
// remove follow it through worlds.yml; and what golem cannot make is
// refused.
func TestCreateLoadUnloadRemove(t *testing.T) {
	env := mvtest.Start(t, nil)
	out := env.Console("mv create Void1 normal -g Void -s 42 --properties alias=Empty,pvp=false")
	env.Expect(out, "Creating world Void1 with the following properties:")
	env.Expect(out, "- Environment: NORMAL")
	env.Expect(out, "- Seed: 42")
	env.Expect(out, "- Generator: Void")
	env.Expect(out, "World 'Void1' created!")
	w := world(env, "Void1")
	if h, ok := w.Handle(); !ok || h.Seed() != 42 || h.Key() != plugin.MinecraftKey("void1") {
		t.Fatalf("Void1 not loaded with seed 42 as minecraft:void1")
	}
	if w.Alias() != "Empty" || w.Props.Bool(core.PropPvp) {
		t.Errorf("properties not applied: alias %q pvp %v", w.Alias(), w.Props.Bool(core.PropPvp))
	}
	saved := env.ReadFile("worlds.yml")
	if !strings.Contains(saved, "\nVoid1:\n") || !strings.Contains(saved, "generator: Void") {
		t.Errorf("worlds.yml lacks Void1:\n%s", saved)
	}

	env.Expect(env.Console("mv create void1 normal"), "World 'void1' already exists!")
	env.Expect(env.Console("mv create Other bogus"), "Please specify one of (normal, nether, the_end, custom).")
	env.Expect(env.Console("mv create Chest normal -c"), "a bonus chest is not supported by golem yet")

	env.Expect(env.Console("mv unload Void1"), "World 'Void1' unloaded!")
	if w.Loaded() {
		t.Fatal("Void1 still loaded")
	}
	if _, ok := env.MV.Server.World(plugin.MinecraftKey("void1")); ok {
		t.Fatal("the server still has void1")
	}
	env.Expect(env.Console("mv unload Void1"), "World 'Void1' is not a loaded multiverse world.")
	env.Expect(env.Console("mv list"), "Empty - UNLOADED")
	env.Expect(env.Console("mv load Void1"), "World 'Void1' loaded!")
	if h, ok := w.Handle(); !ok || h.Seed() != 42 {
		t.Fatal("Void1 did not load with its seed")
	}
	env.Expect(env.Console("mv load Void1"), "World 'Void1' is already loaded!")
	env.Expect(env.Console("mv unload world"), "You can't unload the default world! World world is the default world")

	env.Expect(env.Console("mv remove Void1"), "World 'Void1' removed!")
	if _, ok := env.MV.Worlds.Get("Void1"); ok {
		t.Fatal("Void1 still known")
	}
	if strings.Contains(env.ReadFile("worlds.yml"), "Void1") {
		t.Error("worlds.yml still names Void1")
	}
}

// A world with players in it unloads only with --remove-players, which
// sends them to the default world's spawn.
func TestUnloadRemovesPlayers(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	env.Console("mv create Arena normal -g Floor")
	alice := env.Join("Alice", true)
	w := world(env, "Arena")
	env.Sync(func() {
		h, _ := w.Handle()
		alice.Handle().Teleport(plugin.Location{World: h, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.5}}, plugin.TeleportPlugin)
	})
	env.Srv.RunTicks(2)
	env.Expect(env.Console("mv unload Arena"), "There are still 1 player(s) in the world!")
	env.Expect(env.Console("mv unload Arena -r nowhere"), "World 'nowhere' does not exist or is not loaded!")
	env.Expect(env.Console("mv unload Arena --remove-players"), "World 'Arena' unloaded!")
	env.Sync(func() {
		if k := alice.Handle().World().Key(); k != plugin.MinecraftKey("overworld") {
			t.Errorf("Alice is in %s", k)
		}
	})
}

// /mv delete asks for confirmation, then removes the world and its folder.
func TestDeleteAfterConfirm(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Console("mv create Doomed normal -g Void")
	w := world(env, "Doomed")
	saveWorld(env, w)
	folder := env.MV.WorldFolder(w.Key())
	if _, err := os.Stat(folder); err != nil {
		t.Fatalf("no world folder: %v", err)
	}
	out := env.Console("mv delete Doomed")
	env.Expect(out, "Are you sure you want to delete world 'Doomed'?")
	confirm(env, out)
	env.Srv.RunTicks(3)
	if _, ok := env.MV.Worlds.Get("Doomed"); ok {
		t.Fatal("Doomed still known")
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("folder survived: %v", err)
	}
	env.Expect(env.Console("mv delete Doomed"), "World 'Doomed' is not a multiverse world.")
}

// /mv clone copies a world's folder and properties; /mv import takes back a
// removed world's folder.
func TestCloneAndImport(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Console("mv create Base normal -g Floor -s 9 --properties alias=Home")
	base := world(env, "Base")
	saveWorld(env, base)
	env.Expect(env.Console("mv clone Base Copy"), "World cloned to 'Copy'!")
	cp := world(env, "Copy")
	if h, ok := cp.Handle(); !ok || h.Seed() != 9 {
		t.Fatal("Copy not loaded with Base's seed")
	}
	if cp.Alias() != "Home" || cp.Generator() != "Floor" {
		t.Errorf("Copy alias %q generator %q", cp.Alias(), cp.Generator())
	}
	env.Expect(env.Console("mv clone Base Copy"), "World 'Copy' already exists!")

	env.Console("mv remove Copy")
	got := env.MV.Commands.Suggest(env.MV.Server.Console(), nil, "import ")
	if strings.Join(got, ",") != "import copy" {
		t.Errorf("import suggestions = %q", got)
	}
	env.Expect(env.Console("mv import Missing normal"), "folder contents does not seem to be a valid world!")
	out := env.Console("mv import copy normal -g Floor")
	env.Expect(out, "Starting import of world 'copy'...")
	env.Expect(out, "World 'copy' imported!")
	if h, ok := world(env, "copy").Handle(); !ok || h.Seed() != 9 {
		t.Fatal("copy not imported with its seed")
	}
}

// /mv regen recreates a world with a new seed, keeping its properties.
func TestRegenWithSeed(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Console("mv create Mine normal -g Void -s 1 --properties alias=Pit")
	saveWorld(env, world(env, "Mine"))
	out := env.Console("mv regen Mine --seed 5")
	env.Expect(out, "Are you sure you want to regenerate world 'Mine'?")
	confirm(env, out)
	env.Srv.RunTicks(3)
	w := world(env, "Mine")
	if h, ok := w.Handle(); !ok || h.Seed() != 5 {
		t.Fatal("Mine not regenerated with seed 5")
	}
	if w.Alias() != "Pit" {
		t.Errorf("alias %q", w.Alias())
	}
}

// /mv create makes vanilla flat worlds, with their generator settings.
func TestCreateFlatWorld(t *testing.T) {
	env := mvtest.Start(t, nil)
	out := env.Console(`mv create Flat normal -t flat -a -gs {"layers":[{"block":"minecraft:gold_block","height":2}],"biome":"minecraft:desert"}`)
	env.Expect(out, "- World Type: FLAT")
	env.Expect(out, "- Structures: false")
	env.Expect(out, "World 'Flat' created!")
	w := world(env, "Flat")
	env.Sync(func() {
		h, _ := w.Handle()
		h.LoadChunk(plugin.ChunkPos{}, true, func(plugin.Chunk) {
			if b := h.BlockState(plugin.BlockPos{X: 0, Y: h.MinY() + 1, Z: 0}).Block(); b != registry.BlockGoldBlock {
				t.Errorf("block at the floor is %v", b)
			}
		})
	})
	env.Srv.RunTicks(3)
	if w.Props.String(core.PropGeneratorSettings) == "" {
		t.Error("worlds.yml lost the generator settings")
	}
}
