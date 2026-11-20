package commands_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/multiverse/commands"
	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// voidWorld is a worlds.yml with one empty world, the cheapest to load.
const voidWorld = "void:\n  generator: Void\n  version: 1.3\n  read-only:\n    environment: normal\n"

// rule answers a game rule's value in w, read on the tick goroutine.
func rule(env *mvtest.Env, w *core.World, key string) int {
	env.T.Helper()
	r, ok := env.MV.Server.Registries().GameRule(plugin.MinecraftKey(key))
	if !ok {
		env.T.Fatalf("no game rule %s", key)
	}
	var v int
	env.Sync(func() {
		h, _ := w.Handle()
		v = h.GameRule(r)
	})
	return v
}

func TestGameruleCommands(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"config.yml": "command:\n  show-legacy-aliases: true\n"})
	overworld := world(env, "world")
	env.Expect(env.Console("mv gamerule set keepInventory true world"), "Successfully set keep_inventory to true in world.")
	if rule(env, overworld, "keep_inventory") != 1 {
		t.Error("keep_inventory was not set")
	}
	env.Expect(env.Console("mv gamerule list world --filter keep_inv"), "keep_inventory: true")
	env.Expect(env.Console("mv rule reset minecraft:keep_inventory world,world_nether"),
		"Successfully reset keep_inventory in 2 worlds.")
	if rule(env, overworld, "keep_inventory") != 0 {
		t.Error("keep_inventory was not reset")
	}

	out := env.Console("mv gamerule set keep_inventory maybe world")
	env.Expect(out, "Error: The game rule value maybe is not valid for game rule keep_inventory.")
	env.Expect(out, "Usage: /mv gamerule set <gamerule> <value> [worlds|*]")
	env.Expect(env.Console("mv gamerule set no_such_rule 1 world"), "The game rule no_such_rule is not valid.")
	env.Expect(env.Console("mv gamerule set keep_inventory true nowhere"), "World nowhere is not a loaded multiverse world.")
	env.Expect(env.Console("mv gamerule set keep_inventory true"), "This command can only be used by a player in a loaded Multiverse World.")

	var intRule plugin.GameRuleInfo
	for _, r := range env.MV.Server.Registries().GameRules() {
		if !r.Boolean && r.Min > -1000 {
			intRule = r
			break
		}
	}
	env.Expect(env.Console("mv gamerule set "+intRule.Key.Path+" -1000000 *"),
		"Failed to set gamerule "+intRule.Key.Path+" to -1000000 in world. It should be a java.lang.Integer.")
	env.Expect(env.Console("mvrule "+intRule.Key.Path+" "+strconv.Itoa(intRule.Default+1)+" *"),
		"Successfully set "+intRule.Key.Path+" to "+strconv.Itoa(intRule.Default+1)+" in 3 worlds.")
	env.Expect(env.Console("mvrules world_nether"), "--- Gamerules for world_nether ---")

	console := env.MV.Server.Console()
	if got := env.MV.Commands.Suggest(console, nil, "gamerule set keep_inv"); strings.Join(got, ",") != "gamerule set keep_inventory" {
		t.Errorf("rule completion = %v", got)
	}
	if got := env.MV.Commands.Suggest(console, nil, "gamerule set keep_inventory "); strings.Join(got, ",") != "gamerule set keep_inventory false,gamerule set keep_inventory true" {
		t.Errorf("value completion = %v", got)
	}
}

func TestMetaCommands(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": voidWorld})
	env.Expect(env.Console("mv meta info void"), "No metadata found for world 'void'.")
	env.Expect(env.Console("mv meta modify void set colour blue"), "Successfully set metadata key 'colour' to 'blue' for world void.")
	env.Expect(env.Console("mv meta modify void set size big"), "Successfully set metadata key 'size'")
	out := env.Console("mv meta info void")
	env.Expect(out, "---- World Metadata: void ----")
	env.Expect(out, "colour: blue")
	env.Expect(out, "size: big")
	if saved := env.ReadFile("worlds.yml"); !strings.Contains(saved, "  meta:\n    colour: blue\n") {
		t.Errorf("worlds.yml lacks the meta:\n%s", saved)
	}
	env.Expect(env.Console("mv meta modify void remove colour"), "Successfully removed metadata key 'colour' for world void.")
	env.ExpectNot(env.Console("mv meta info void"), "colour")
	env.Expect(env.Console("mv meta modify void drop size"), "Invalid action 'drop'! Valid actions are 'set' and 'remove' only.")
	env.Expect(env.Console("mv meta modify set size small"), "World 'set' is not a multiverse world. Remember to specify the world name")

	console := env.MV.Server.Console()
	if got := env.MV.Commands.Suggest(console, nil, "meta modify void remove "); strings.Join(got, ",") != "meta modify void remove size" {
		t.Errorf("meta key completion = %v", got)
	}
}

func TestEntitySpawnConfigCommands(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": voidWorld})
	out := env.Console("mv entity-spawn-config info void")
	env.Expect(out, "=== [ Entity Spawn Config 'void' ] ===")
	env.Expect(out, "MONSTER:")
	env.Expect(out, "  spawn: true")
	env.Expect(out, "  tick-rate: -2")
	env.Expect(out, "WATER_UNDERGROUND_CREATURE:")

	env.Expect(env.Console("mv entity-spawn-config modify void monster set spawn false"),
		"Successfully set spawn to false for MONSTER in void")
	env.Expect(env.Console("mv entity-spawn-config modify void Monster add exceptions minecraft:zombie"),
		"Successfully set exceptions to minecraft:zombie for MONSTER in void")
	env.Expect(env.Console("mv entity-spawn-config modify void water_ambient set tick-rate 400"),
		"Successfully set tick-rate to 400 for WATER_AMBIENT in void")
	s := world(env, "void").Spawning()
	if s.Allows("monster", "creeper") || !s.AllowsEntity("minecraft:zombie") || s.Category("water_ambient").Int("tick-rate") != 400 {
		t.Errorf("spawning not changed: %v %v", s.Allows("monster", "creeper"), s.AllowsEntity("zombie"))
	}
	saved := env.ReadFile("worlds.yml")
	if !strings.Contains(saved, "    monster:\n      exceptions:\n        - ZOMBIE\n      spawn: false\n") {
		t.Errorf("worlds.yml lacks the monster settings:\n%s", saved)
	}

	out = env.Console("mv entity-spawn-config modify void monster add exceptions notamob")
	env.Expect(out, "Unable to set exceptions to notamob for MONSTER in void")
	env.Expect(out, "No enum constant org.bukkit.entity.EntityType.NOTAMOB")
	env.Expect(env.Console("mv entity-spawn-config modify void monster set nope 1"), "Node not found in config: nope")
	env.Expect(env.Console("mv entity-spawn-config modify void monster set spawn"), "Value is required for PropertyModifyAction: SET")
	env.Expect(env.Console("mv entity-spawn-config modify void monster reset spawn"), "Successfully set spawn to null for MONSTER in void")
	env.Expect(env.Console("mv entity-spawn-config modify void ghosts set spawn true"), "Please specify one of (monster, animal,")
	env.Expect(env.Console("mv entity-spawn-config info void --filter monster"), "MONSTER:")
}

// loadSpawnChunk keeps w's chunk 0,0 loaded, for entities to be spawned in.
func loadSpawnChunk(env *mvtest.Env, w *core.World) {
	env.T.Helper()
	h, _ := w.Handle()
	env.Sync(func() { h.AddTicket(plugin.ChunkPos{}, 1) })
	for range 100 {
		var ok bool
		env.Sync(func() { _, ok = h.Chunk(plugin.ChunkPos{}) })
		if ok {
			return
		}
	}
	env.T.Fatal("chunk 0,0 did not load")
}

// spawn spawns an entity of type t on the Floor world's floor, on the tick
// goroutine.
func spawn(env *mvtest.Env, w *core.World, t registry.EntityType) {
	env.T.Helper()
	var err error
	env.Sync(func() {
		h, _ := w.Handle()
		_, err = h.Spawn(t, plugin.Location{World: h, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.5}}, nil)
	})
	if err != nil {
		env.T.Fatalf("spawn %v: %v", t, err)
	}
}

func TestPurgeEntities(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": "floor:\n  generator: Floor\n  version: 1.3\n"})
	w := world(env, "floor")
	loadSpawnChunk(env, w)
	for _, typ := range []registry.EntityType{registry.EntityTypeZombie, registry.EntityTypeZombie,
		registry.EntityTypeCow, registry.EntityTypeArmorStand} {
		spawn(env, w, typ)
	}
	env.Expect(env.Console("mv purge-entities floor"), "Successfully purged 0 entities in world floor.")
	w.Spawning().Category("monster").Set(mustNode(t, w, "spawn"), false)
	env.Expect(env.Console("mv purge-entities floor"), "Successfully purged 2 entities in world floor.")
	env.Expect(env.Console("mv purge-all-entities floor animal,bogus"),
		"Successfully purged 1 entities in world floor for spawn categories ANIMAL.")
	env.Expect(env.Console("mv purge-all-entities floor ghosts"), "Please specify one of (monster, animal,")
	env.Expect(env.Console("mv purge-all-entities floor"), "Successfully purged 1 entities in world floor.")
	env.Expect(env.Console("mv purge-all-entities monster"), "World 'monster' is not a loaded multiverse world.")
}

func mustNode(t *testing.T, w *core.World, name string) *core.Node {
	n, ok := w.Spawning().Category("monster").Nodes().Find(name)
	if !ok {
		t.Fatalf("no spawning node %s", name)
	}
	return n
}

func TestWorldBorderCommands(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Expect(env.Console("mv worldborder get world_nether"), "The world border is currently 5.9999968E7 block(s) wide for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder set 100 0 world_nether"), "Set the world border to 100.0 block(s) wide for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder set 100 0 world_nether"), "Nothing changed. The world border is already that size for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder add -50 5s world_nether"), "Shrinking the world border to 50.0 block(s) wide over 5.00 second(s) for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder center 10 -20.5 world_nether"), "Set the center of the world border to 10.0, -20.5 for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder damage amount 0.5 world_nether"), "Set the world border damage to 0.5 per block each second for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder damage buffer 2 world_nether"), "Set the world border damage buffer to 2.0 block(s) for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder warning distance 9 world_nether"), "Set the world border warning distance to 9 block(s) for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder warning time 30 world_nether"), "Set the world border warning time to 1.50 second(s) for world 'world_nether'.")
	env.Expect(env.Console("mv worldborder warning time 30 world_nether"), "Nothing changed. The world border warning is already that amount of time")
	var size, buffer, damage float64
	var cx, cz float64
	var warn time.Duration
	nether := world(env, "world_nether")
	env.Sync(func() {
		h, _ := nether.Handle()
		b := h.Border()
		size, buffer, damage, warn = b.Size(), b.DamageBuffer(), b.DamagePerBlock(), b.WarningTime()
		cx, cz = b.Center()
	})
	if size > 100 || buffer != 2 || damage != 0.5 || warn != 1500*time.Millisecond || cx != 10 || cz != -20.5 {
		t.Errorf("border: size %v buffer %v damage %v warning %v center %v,%v", size, buffer, damage, warn, cx, cz)
	}
	env.Expect(env.Console("mv worldborder set 100 world_nether"), "Invalid time duration format: \"world_nether\"")
	env.Expect(env.Console("mv worldborder set 0 0 world_nether"), "Error: java.lang.IllegalArgumentException: newSize must be between 1.0 and 5.9999968E7")
	env.Expect(env.Console("mv worldborder set lots"), "Please specify a number.")
	env.Expect(env.Console("mv worldborder get"), "This command can only be used by a player in a loaded Multiverse World.")
}

func TestWhoAndCoordinates(t *testing.T) {
	env := mvtest.Start(t, nil, plugintest.Ops("Alice"))
	alice := env.Join("Alice", true)
	out := mvtest.Run(alice, "mv who")
	env.Expect(out, "====[ Multiverse World Players List ]====")
	env.Expect(out, "world: Alice")
	env.Expect(env.Console("mv who world_nether"), "world_nether: empty")
	out = env.Console("mv whoall")
	env.Expect(out, "world: Alice")
	env.ExpectNot(out, "world_nether")

	out = mvtest.Run(alice, "mv coordinates")
	env.Expect(out, "--- Location Information ---")
	env.Expect(out, "World: world")
	env.Expect(out, "World Scale: 1.0")
	env.Expect(out, "Coordinates: X: 0.5 Y: 64 Z: 0.5 P: 0 Y: 0")
	env.Expect(out, "Direction: s")
	env.Expect(env.Console("mv coordinates"), "This command can only be used by players")
}

func TestGeneratorsCommands(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"config.yml": "command:\n  show-legacy-aliases: true\n"})
	out := env.Console("mv generators list")
	env.Expect(out, "====[ Multiverse Generator List ]====")
	env.Expect(out, "Floor")
	env.Expect(out, "Void")
	env.Expect(env.Console("mvgens --filter flo"), "Floor")
	env.Expect(env.Console("mv generators info void"), "Generator Plugin: Void")
	env.Expect(env.Console("mv generators info nothing"), "Invalid generator plugin: nothing")
	env.Expect(env.Console("mvco"), "This command can only be used by players")
}

// /mv dumps uploads the files and log to the paste service and answers the
// viewer's link; the paste services are a local server here.
func TestDumpsUploads(t *testing.T) {
	type upload struct {
		contentType string
		body        []byte
	}
	got := make(chan upload, 2)
	paste := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- upload{r.Header.Get("Content-Type"), body}
		if r.URL.Path == "/mclogs" {
			_, _ = w.Write([]byte(`{"success":true,"id":"m1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"key":"abc"}`))
	}))
	defer paste.Close()
	saved := commands.PasteEndpoints
	commands.PasteEndpoints = map[string]string{"pastesdev": paste.URL + "/post", "mclogs": paste.URL + "/mclogs"}
	t.Cleanup(func() { commands.PasteEndpoints = saved })

	env := mvtest.Start(t, nil, plugintest.Ops("Alice"))
	if err := os.MkdirAll(filepath.Join(env.Dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.Dir, "logs", "latest.log"), []byte("[INFO] hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alice := env.Join("Alice", true)
	out := mvtest.Run(alice, "mv dumps")
	env.Expect(out, "Gathering logs and debug info...")
	waitFor(env, alice, out, "Uploaded with pastes.dev backend: https://mvdumps.c0ding.party/abc")
	u := <-got
	var payload struct {
		Files []struct{ Name, Data, Type string }
	}
	if err := json.Unmarshal(u.body, &payload); err != nil || !strings.HasPrefix(u.contentType, "application/json") {
		t.Fatalf("payload %s (%v)", u.contentType, err)
	}
	names := map[string]string{}
	for _, f := range payload.Files {
		names[f.Name] = f.Type + ":" + f.Data
	}
	for name, want := range map[string]string{"version.md": "markdown:# Multiverse-Core Version info",
		"multiverse-core/config.yml": "yaml:", "multiverse-core/worlds.yml": "yaml:", "latest.log": "accesslog:[INFO] hello"} {
		if !strings.HasPrefix(names[name], want) {
			t.Errorf("%s = %.60q, want %q...", name, names[name], want)
		}
	}

	waitFor(env, alice, mvtest.Run(alice, "mv dumps -u mclogs"), "Uploaded with mclo.gs backend: https://mvdumps.c0ding.party/m1?service=mclogs")
	u = <-got
	form, err := url.ParseQuery(string(u.body))
	if err != nil || !strings.Contains(form.Get("content"), `"name":"latest.log"`) {
		t.Errorf("mclogs form: %v %.80q", err, u.body)
	}
	env.Expect(env.Console("mv dumps --upload pastebin"), "Please specify one of (pastesdev, mclogs).")
}

// waitFor reads p's messages until one contains sub, for up to 200 ticks;
// seen are the messages already read.
func waitFor(env *mvtest.Env, p *plugintest.Player, seen []string, sub string) {
	env.T.Helper()
	for range 20 {
		if mvtest.Has(seen, sub) {
			return
		}
		seen = p.Messages(10)
	}
	env.T.Fatalf("no message contains %q", sub)
}
