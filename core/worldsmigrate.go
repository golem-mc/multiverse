package core

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// migrateMV4Worlds rewrites a Multiverse 4 worlds.yml, whose worlds are
// "==: MVWorld" sections under worlds: (a name with dots nested a level per
// dot), into version 5's form: one top-level section per world, keyed by
// its name with "." written "[dot]". ok is false when data is not
// Multiverse 4's. MV: WorldsConfigManager.migrateRemoveOldConfigSerializable
func migrateMV4Worlds(data []byte) (out []byte, ok bool, err error) {
	if !bytes.Contains(data, []byte("==: MVWorld")) {
		return data, false, nil
	}
	s := strings.ReplaceAll(string(data), "==: MVWorld", "w@: world")
	for _, marker := range []string{"==: MVSpawnSettings", "==: MVSpawnSubSettings", "==: MVEntryFee"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		return nil, false, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return data, false, nil
	}
	root := doc.Content[0]
	kept := &yaml.Node{Kind: yaml.MappingNode}
	var worlds []*yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "worlds" {
			collectMV4Worlds(root.Content[i+1], nil, &worlds)
			continue
		}
		kept.Content = append(kept.Content, root.Content[i], root.Content[i+1])
	}
	kept.Content = append(kept.Content, worlds...)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{kept}}); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

// collectMV4Worlds appends a key and section for each world section (one
// holding "w@") under n, path being the keys above it.
func collectMV4Worlds(n *yaml.Node, path []string, out *[]*yaml.Node) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "w@" {
			key := strings.ReplaceAll(strings.Join(path, "."), ".", "[dot]")
			*out = append(*out, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, n)
			return
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		collectMV4Worlds(n.Content[i+1], append(path[:len(path):len(path)], n.Content[i].Value), out)
	}
}

// worldMigrations bring a world's section of worlds.yml up a version each,
// in order; name is the world's usable name. MV: WorldConfig.migrator
var worldMigrations = []struct {
	version float64
	migrate func(m map[string]any, name string)
}{
	{1.0, func(m map[string]any, _ string) {
		for _, p := range [][2]string{{"adjustSpawn", "adjust-spawn"}, {"allowFlight", "allow-flight"},
			{"allowWeather", "allow-weather"}, {"autoHeal", "auto-heal"}, {"autoLoad", "auto-load"},
			{"bedRespawn", "bed-respawn"}} {
			movePath(m, p[0], p[1])
			migrateBool(m, p[1])
		}
		movePath(m, "entryfee.amount", "entry-fee.amount")
		migrateNumber(m, "entry-fee.amount", 64, true)
		movePath(m, "entryfee.currency", "entry-fee.currency")
		deletePath(m, "entryfee")
		movePath(m, "gameMode", "gamemode")
		if g, ok := lookupPath(m, "generator"); ok && fmt.Sprint(g) == "null" {
			setPath(m, "generator", "")
		}
		migrateBool(m, "hidden")
		migrateBool(m, "hunger")
		movePath(m, "keepSpawnInMemory", "keep-spawn-in-memory")
		migrateBool(m, "keep-spawn-in-memory")
		movePath(m, "playerLimit", "player-limit")
		migrateNumber(m, "player-limit", 32, false)
		movePath(m, "portalForm", "portal-form")
		migrateBool(m, "pvp")
		movePath(m, "respawnWorld", "respawn-world")
		migrateNumber(m, "scale", 64, true)
		migrateNumber(m, "seed", 64, false)
		movePath(m, "spawnLocation", "spawn-location")
		for _, c := range []string{"animals", "monsters"} {
			migrateBool(m, "spawning."+c+".spawn")
			movePath(m, "spawning."+c+".spawnrate", "spawning."+c+".tick-rate")
			migrateNumber(m, "spawning."+c+".tick-rate", 32, false)
		}
		movePath(m, "worldBlacklist", "world-blacklist")
		migrateEntryFee(m)
		migrateLegacyAlias(m)
	}},
	{1.1, func(m map[string]any, _ string) {
		// MV: WorldConfig.BiomeMigrator
		biome := stringAt(m, "biome")
		if biome == "@vanilla" {
			biome = ""
		} else if biome != "" {
			biome = "@single:" + biome
		}
		setPath(m, "biome", biome)
	}},
	{1.2, func(m map[string]any, _ string) {
		movePath(m, "spawning.animals", "spawning.animal")
		movePath(m, "spawning.monsters", "spawning.monster")
	}},
	{1.3, func(m map[string]any, name string) {
		setPath(m, "read-only.legacy-world-name", name)
		movePath(m, "environment", "read-only.environment")
		movePath(m, "seed", "read-only.seed")
	}},
}

// migrateWorldSection brings a world's section up to WorldConfigVersion,
// answering the version it had and whether it changed. An empty section is
// a new world's and only gets the version. MV: ConfigMigrator.migrate
func migrateWorldSection(m map[string]any, name string) (from float64, migrated bool) {
	if len(m) == 0 {
		m["version"] = WorldConfigVersion
		return WorldConfigVersion, false
	}
	from = numberAt(m, "version")
	for _, step := range worldMigrations {
		if numberAt(m, "version") < step.version {
			step.migrate(m, name)
			m["version"] = step.version
			migrated = true
		}
	}
	return from, migrated
}

func numberAt(m map[string]any, path string) float64 {
	v, _ := lookupPath(m, path)
	f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
	return f
}

func stringAt(m map[string]any, path string) string {
	v, ok := lookupPath(m, path)
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// movePath moves the value at from to to, when there is one.
// MV: MoveMigratorAction
func movePath(m map[string]any, from, to string) {
	if v, ok := deletePath(m, from); ok && v != nil {
		setPath(m, to, v)
	}
}

// migrateBool turns the string at path into a boolean as ACF's isTruthy
// does. Unlike Multiverse, which writes false for a missing key, it leaves
// a missing key to its default: a hand-written section without a version
// would otherwise lose auto-load, pvp and the rest. MV: BooleanMigratorAction
func migrateBool(m map[string]any, path string) {
	v, ok := lookupPath(m, path)
	if !ok || v == nil {
		return
	}
	switch fmt.Sprint(v) {
	case "t", "true", "on", "y", "yes", "1":
		setPath(m, path, true)
	default:
		setPath(m, path, false)
	}
}

// migrateNumber parses the string at path as a number, removing it when it
// is not one. MV: DoubleMigratorAction, IntegerMigratorAction, LongMigratorAction
func migrateNumber(m map[string]any, path string, bits int, float bool) {
	v, ok := lookupPath(m, path)
	if !ok {
		return
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if float {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			setPath(m, path, f)
			return
		}
	} else if i, err := parseJavaInt(s, bits); err == nil {
		setPath(m, path, i)
		return
	}
	deletePath(m, path)
}

// migrateEntryFee enables the fee of a world that had a currency, and
// points one without at the economy. MV: WorldConfig.EntryFeeMigrator
func migrateEntryFee(m map[string]any) {
	if stringAt(m, "entry-fee.currency") == "" {
		setPath(m, "entry-fee.enabled", false)
		setPath(m, "entry-fee.currency", VaultEconomyCode)
	} else {
		setPath(m, "entry-fee.enabled", true)
	}
}

// legacyColourCodes and legacyStyleCodes are Multiverse 4's alias colour and
// style names. MV: LegacyAliasMigrator.EnglishChatColor, EnglishChatStyle
var (
	legacyColourCodes = map[string]string{"AQUA": "b", "BLACK": "0", "BLUE": "9", "DARKAQUA": "3",
		"DARKBLUE": "1", "DARKGRAY": "8", "DARKGREEN": "2", "DARKPURPLE": "5", "DARKRED": "4", "GOLD": "6",
		"GRAY": "7", "GREEN": "a", "LIGHTPURPLE": "d", "RED": "c", "YELLOW": "e", "WHITE": ""}
	legacyStyleCodes = map[string]string{"NORMAL": "", "MAGIC": "k", "BOLD": "l", "STRIKETHROUGH": "m",
		"UNDERLINE": "n", "ITALIC": "o"}
)

// migrateLegacyAlias folds the old color and style keys into the alias as
// '&' codes. MV: WorldConfig.LegacyAliasMigrator
func migrateLegacyAlias(m map[string]any) {
	alias := stringAt(m, "alias")
	colour, style := strings.ToUpper(stringAt(m, "color")), strings.ToUpper(stringAt(m, "style"))
	delete(m, "color")
	delete(m, "style")
	if alias == "" {
		return
	}
	if c := legacyColourCodes[colour]; c != "" {
		alias = "&" + c + alias
	}
	if s := legacyStyleCodes[style]; s != "" {
		alias = "&" + s + alias
	}
	m["alias"] = alias
}
