package core

import (
	"maps"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// mv4Worlds is a worlds.yml as Multiverse-Core 4 writes it.
const mv4Worlds = `worlds:
  survival:
    ==: MVWorld
    hidden: 'false'
    alias: Survival
    color: GOLD
    style: BOLD
    pvp: 'false'
    scale: '1.0'
    respawnWorld: ''
    allowWeather: 'true'
    difficulty: HARD
    spawning:
      ==: MVSpawnSettings
      animals:
        ==: MVSpawnSubSettings
        spawn: 'true'
        spawnrate: '-1'
        exceptions: []
      monsters:
        ==: MVSpawnSubSettings
        spawn: 'false'
        spawnrate: '400'
        exceptions:
        - CREEPER
    entryfee:
      ==: MVEntryFee
      amount: '5.0'
      currency: DIAMOND
    hunger: 'true'
    autoHeal: 'true'
    adjustSpawn: 'true'
    portalForm: ALL
    gameMode: ADVENTURE
    keepSpawnInMemory: 'true'
    spawnLocation:
      ==: MVSpawnLocation
      x: 10.5
      y: 64.0
      z: -3.5
      pitch: 0.0
      yaw: 90.0
    autoLoad: 'false'
    bedRespawn: 'true'
    worldBlacklist: []
    environment: NETHER
    seed: '-123'
    generator: 'null'
    playerLimit: '20'
    allowFlight: 'true'
  my:
    world:
      ==: MVWorld
      alias: ''
      environment: NORMAL
      entryfee:
        ==: MVEntryFee
        amount: '0.0'
`

func TestMigrateMV4WorldsFile(t *testing.T) {
	out, ok, err := migrateMV4Worlds([]byte(mv4Worlds))
	if err != nil || !ok {
		t.Fatalf("migrate: ok %v, err %v", ok, err)
	}
	if keys := orderedKeys(out); strings.Join(keys, ",") != "survival,my[dot]world" {
		t.Fatalf("top-level keys = %v:\n%s", keys, out)
	}
	m := map[string]any{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	sec := m["survival"].(map[string]any)
	if from, migrated := migrateWorldSection(sec, "survival"); from != 0 || !migrated {
		t.Errorf("migrated from %v: %v", from, migrated)
	}
	want := map[string]any{
		"version":                     1.3,
		"alias":                       "&l&6Survival",
		"pvp":                         false,
		"allow-weather":               true,
		"adjust-spawn":                true,
		"auto-load":                   false,
		"gamemode":                    "ADVENTURE",
		"portal-form":                 "ALL",
		"keep-spawn-in-memory":        true,
		"player-limit":                int64(20),
		"scale":                       1.0,
		"generator":                   "",
		"biome":                       "",
		"entry-fee.amount":            5.0,
		"entry-fee.currency":          "DIAMOND",
		"entry-fee.enabled":           true,
		"spawning.animal.spawn":       true,
		"spawning.animal.tick-rate":   int64(-1),
		"spawning.monster.spawn":      false,
		"spawning.monster.tick-rate":  int64(400),
		"spawn-location.x":            10.5,
		"read-only.environment":       "NETHER",
		"read-only.seed":              int64(-123),
		"read-only.legacy-world-name": "survival",
		"respawn-world":               "",
		"difficulty":                  "HARD",
		"hunger":                      true,
		"auto-heal":                   true,
		"bed-respawn":                 true,
		"allow-flight":                true,
		"hidden":                      false,
		"spawn-location.==":           "MVSpawnLocation",
		"w@":                          "world",
	}
	for path, v := range want {
		if got, _ := lookupPath(sec, path); got != v {
			t.Errorf("%s = %#v, want %#v", path, got, v)
		}
	}
	for _, gone := range []string{"color", "style", "entryfee", "gameMode", "seed", "environment",
		"spawning.monsters", "spawning.animals", "autoLoad", "spawnLocation", "worldBlacklist"} {
		if _, ok := lookupPath(sec, gone); ok {
			t.Errorf("%s is still there", gone)
		}
	}

	// The migrated section loads into the world's properties (but the
	// currency, which needs a server to parse).
	v := NewValues(NewNodes((&World{core: &Core{}}).worldNodes()...))
	loaded := maps.Clone(sec)
	delete(loaded, "entry-fee")
	v.Load(loaded)
	if v.String(PropGameMode) != "adventure" || v.String(PropEnvironment) != "nether" || v.Bool(PropAutoLoad) ||
		v.Int(PropPlayerLimit) != 20 || v.Long(PropSeed) != -123 {
		t.Errorf("loaded: gamemode %q env %q auto-load %v limit %d seed %d", v.String(PropGameMode),
			v.String(PropEnvironment), v.Bool(PropAutoLoad), v.Int(PropPlayerLimit), v.Long(PropSeed))
	}
	sp := v.Custom(PropSpawning).(*Spawning)
	if sp.Allows("monster", "zombie") || !sp.Allows("monster", "creeper") || sp.Category("monster").Int("tick-rate") != 400 {
		t.Errorf("spawning: %v %v %v", sp.Allows("monster", "zombie"), sp.Allows("monster", "creeper"),
			sp.Category("monster").Int("tick-rate"))
	}
	if loc := v.Custom(PropSpawnLocation).(*SpawnLocation); loc == nil || loc.X != 10.5 || loc.Yaw != 90 {
		t.Errorf("spawn location = %+v", loc)
	}

	nested := m["my[dot]world"].(map[string]any)
	migrateWorldSection(nested, "my.world")
	if got, _ := lookupPath(nested, "entry-fee.enabled"); got != false {
		t.Errorf("a fee with no currency stays off: %v", got)
	}
	if got, _ := lookupPath(nested, "entry-fee.currency"); got != VaultEconomyCode {
		t.Errorf("currency = %v", got)
	}
	if got := nested["alias"]; got != "" {
		t.Errorf("empty alias = %q", got)
	}
}

func TestMigrateMV4WorldsLeavesV5Alone(t *testing.T) {
	in := []byte("world:\n  version: 1.3\n")
	if out, ok, err := migrateMV4Worlds(in); ok || err != nil || string(out) != string(in) {
		t.Errorf("a version 5 file was rewritten: %v %v %s", ok, err, out)
	}
}

func TestMigrateWorldSectionVersions(t *testing.T) {
	// 1.0 to 1.3: the biome and the spawn categories are renamed, and the
	// environment and seed move under read-only.
	m := map[string]any{"version": 1.0, "biome": "plains", "environment": "THE_END", "seed": 9,
		"spawning": map[string]any{"monsters": map[string]any{"spawn": false}}}
	if from, ok := migrateWorldSection(m, "end"); from != 1.0 || !ok {
		t.Errorf("from %v ok %v", from, ok)
	}
	for path, want := range map[string]any{"biome": "@single:plains", "spawning.monster.spawn": false,
		"read-only.environment": "THE_END", "read-only.seed": 9, "read-only.legacy-world-name": "end", "version": 1.3} {
		if got, _ := lookupPath(m, path); got != want {
			t.Errorf("%s = %#v, want %#v", path, got, want)
		}
	}
	vanilla := map[string]any{"version": 1.0, "biome": "@vanilla"}
	migrateWorldSection(vanilla, "x")
	if vanilla["biome"] != "" {
		t.Errorf("@vanilla biome = %q", vanilla["biome"])
	}

	// A current section is left alone, an empty one gets the version.
	cur := map[string]any{"version": 1.3, "alias": "x"}
	if _, ok := migrateWorldSection(cur, "x"); ok || len(cur) != 2 {
		t.Errorf("current section changed: %v", cur)
	}
	empty := map[string]any{}
	if _, ok := migrateWorldSection(empty, "x"); ok || empty["version"] != WorldConfigVersion {
		t.Errorf("empty section = %v", empty)
	}
}
