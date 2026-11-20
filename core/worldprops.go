package core

import (
	"fmt"
	"math"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"go.yaml.in/yaml/v4"
)

// WorldConfigVersion is the version a world's section in worlds.yml is
// written with.
const WorldConfigVersion = 1.3

// Enumerations of the world properties, as Bukkit's enums lowercase.
var (
	Environments = []string{"normal", "nether", "the_end", "custom"}
	Difficulties = []string{"peaceful", "easy", "normal", "hard"}
	GameModes    = []string{"survival", "creative", "adventure", "spectator"}
	PortalForms  = []string{"none", "all", "nether", "end"}
)

// VaultEconomyCode is the entry-fee currency that pays with the economy
// service rather than an item. MV: MVEconomist.VAULT_ECONOMY_CODE
const VaultEconomyCode = "@vault-economy"

// Paths of the world properties in worlds.yml.
const (
	PropAdjustSpawn       = "adjust-spawn"
	PropAlias             = "alias"
	PropAdvancementGrant  = "allow-advancement-grant"
	PropAllowFlight       = "allow-flight"
	PropAllowWeather      = "allow-weather"
	PropAnchorRespawn     = "anchor-respawn"
	PropAutoHeal          = "auto-heal"
	PropAutoLoad          = "auto-load"
	PropBedRespawn        = "bed-respawn"
	PropBiome             = "biome"
	PropDifficulty        = "difficulty"
	PropEntryFeeEnabled   = "entry-fee.enabled"
	PropEntryFeeAmount    = "entry-fee.amount"
	PropEntryFeeCurrency  = "entry-fee.currency"
	PropGameMode          = "gamemode"
	PropGenerator         = "generator"
	PropHidden            = "hidden"
	PropHunger            = "hunger"
	PropKeepSpawnInMemory = "keep-spawn-in-memory"
	PropMeta              = "meta"
	PropPlayerLimit       = "player-limit"
	PropPortalForm        = "portal-form"
	PropPvp               = "pvp"
	PropRespawnWorld      = "respawn-world"
	PropScale             = "scale"
	PropSpawnLocation     = "spawn-location"
	PropSpawning          = "spawning"
	PropWorldBlacklist    = "world-blacklist"
	PropEnvironment       = "read-only.environment"
	PropGeneratorSettings = "read-only.generator-settings"
	PropLegacyWorldName   = "read-only.legacy-world-name"
	PropSeed              = "read-only.seed"
	PropVersion           = "version"
)

// worldNodes is a world's section of worlds.yml in Multiverse-Core 5's
// order; w is the world the change hooks act on. MV: WorldConfigNodes
func (w *World) worldNodes() []*Node {
	changed := func(name string, apply func()) func(old, new any) {
		return func(old, new any) {
			if old != nil && fmt.Sprint(old) == fmt.Sprint(new) {
				return
			}
			w.core.firePropertyChanged(w, name, old, new)
			apply()
		}
	}
	hook := func(n *Node) *Node {
		path := n.Path
		n.OnChange = changed(n.NameOrPath(), func() {
			if apply := w.propertyHooks[path]; apply != nil {
				apply()
			}
		})
		return n
	}
	return []*Node{
		hook(&Node{Path: PropAdjustSpawn, Kind: KindBool, Default: false}),
		hook(&Node{Path: PropAlias, Kind: KindString, Default: ""}),
		hook(&Node{Path: PropAdvancementGrant, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropAllowFlight, Kind: KindBool, Default: false}),
		hook(&Node{Path: PropAllowWeather, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropAnchorRespawn, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropAutoHeal, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropAutoLoad, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropBedRespawn, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropBiome, Kind: KindString, Default: "@error", Hidden: true}),
		hook(&Node{Path: PropDifficulty, Kind: KindEnum, Enum: Difficulties, Default: "normal"}),
		hook(&Node{Path: PropEntryFeeEnabled, Name: "entryfee-enabled", Kind: KindBool, Default: false}),
		hook(&Node{Path: PropEntryFeeAmount, Name: "entryfee-amount", Kind: KindDouble, Default: 0.0}),
		hook(&Node{Path: PropEntryFeeCurrency, Name: "entryfee-currency", Kind: KindString, Default: VaultEconomyCode,
			Parse: w.core.parseCurrency, Suggest: w.core.suggestCurrency}),
		hook(&Node{Path: PropGameMode, Kind: KindEnum, Enum: GameModes, Default: "survival"}),
		hook(&Node{Path: PropGenerator, Kind: KindString, Default: "@error", Hidden: true}),
		hook(&Node{Path: PropHidden, Kind: KindBool, Default: false}),
		hook(&Node{Path: PropHunger, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropKeepSpawnInMemory, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropMeta, Kind: KindCustom, DefaultFn: func(*Values) any { return Meta{} }, Hidden: true,
			Decode: decodeMeta, Encode: encodeMeta}),
		hook(&Node{Path: PropPlayerLimit, Kind: KindInt, Default: -1}),
		hook(&Node{Path: PropPortalForm, Kind: KindEnum, Enum: PortalForms, Default: "all"}),
		hook(&Node{Path: PropPvp, Kind: KindBool, Default: true}),
		hook(&Node{Path: PropRespawnWorld, Kind: KindString, Default: "", Suggest: w.core.suggestWorldNames}),
		hook(&Node{Path: PropScale, Kind: KindDouble, DefaultFn: func(v *Values) any { return defaultScale(v.String(PropEnvironment)) }}),
		hook(&Node{Path: PropSpawnLocation, Kind: KindCustom, Default: (*SpawnLocation)(nil), Hidden: true,
			Decode: decodeSpawnLocation, Encode: encodeSpawnLocation}),
		hook(&Node{Path: PropSpawning, Kind: KindCustom, Hidden: true,
			DefaultFn: func(*Values) any { return NewSpawning() },
			Decode:    func(raw any) any { return decodeSpawning(raw) },
			Encode:    func(v any) any { return v.(*Spawning).yaml() }}),
		hook(&Node{Path: PropWorldBlacklist, Kind: KindList, Default: []string{}}),
		hook(&Node{Path: PropEnvironment, Kind: KindEnum, Enum: Environments, Default: "normal", Hidden: true}),
		hook(&Node{Path: PropGeneratorSettings, Kind: KindString, Default: "", Hidden: true}),
		hook(&Node{Path: PropLegacyWorldName, Kind: KindString, Default: "", Hidden: true}),
		hook(&Node{Path: PropSeed, Kind: KindLong, Default: int64(math.MinInt64), Hidden: true}),
		hook(&Node{Path: PropVersion, Kind: KindDouble, Default: WorldConfigVersion, Hidden: true}),
	}
}

// defaultScale is the coordinate scale a world of env has. MV:
// WorldConfigNodes.scale
func defaultScale(env string) float64 {
	switch env {
	case "nether":
		return 8.0
	case "the_end":
		return 16.0
	}
	return 1.0
}

// parseCurrency reads an entry-fee currency: @vault-economy or an item.
func (c *Core) parseCurrency(input string) (any, error) {
	if strings.EqualFold(input, VaultEconomyCode) {
		return VaultEconomyCode, nil
	}
	key, err := plugin.ParseKey(strings.ToLower(input))
	if err == nil {
		if _, ok := c.Server.Registries().Item(key); ok {
			return strings.ToUpper(key.Path), nil
		}
	}
	return nil, MsgError(M("mv-core.config.string.parser.invalidmaterial", "{input}", input))
}

func (c *Core) suggestCurrency(string) []string {
	return []string{VaultEconomyCode, "diamond", "emerald", "gold_ingot", "iron_ingot"}
}

func (c *Core) suggestWorldNames(string) []string {
	var out []string
	for _, w := range c.Worlds.All() {
		out = append(out, w.Name())
	}
	return out
}

// Meta is a world's free-form metadata, kept in worlds.yml's meta section.
type Meta map[string]string

func decodeMeta(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return Meta{}
	}
	out := Meta{}
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}

func encodeMeta(v any) any {
	m, _ := v.(Meta)
	if m == nil {
		return map[string]string{}
	}
	return map[string]string(m)
}

// SpawnLocation is a world's spawn as worlds.yml keeps it. MV: SpawnLocation
type SpawnLocation struct {
	X, Y, Z    float64
	Pitch, Yaw float32
}

// Location answers the spawn as a location in w.
func (s *SpawnLocation) Location(w plugin.World) plugin.Location {
	return plugin.Location{World: w, Pos: plugin.Vec3{X: s.X, Y: s.Y, Z: s.Z}, Yaw: s.Yaw, Pitch: s.Pitch}
}

// SpawnLocationOf answers the spawn kept for loc.
func SpawnLocationOf(loc plugin.Location) *SpawnLocation {
	return &SpawnLocation{X: loc.Pos.X, Y: loc.Pos.Y, Z: loc.Pos.Z, Pitch: loc.Pitch, Yaw: loc.Yaw}
}

func decodeSpawnLocation(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return (*SpawnLocation)(nil)
	}
	f := func(k string) float64 {
		switch v := m[k].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
		return 0
	}
	if _, ok := m["x"]; !ok {
		return (*SpawnLocation)(nil)
	}
	return &SpawnLocation{X: f("x"), Y: f("y"), Z: f("z"), Pitch: float32(f("pitch")), Yaw: float32(f("yaw"))}
}

// encodeSpawnLocation writes the spawn as Bukkit serializes MV's
// SpawnLocation, with its "==" type marker so Multiverse on Bukkit reads it.
func encodeSpawnLocation(v any) any {
	s, _ := v.(*SpawnLocation)
	if s == nil {
		return yamlMap("==", "MVNullLocation (It's a bug if you see this in your config file)")
	}
	return yamlMap("==", "MVSpawnLocation", "x", s.X, "y", s.Y, "z", s.Z,
		"pitch", float64(s.Pitch), "yaw", float64(s.Yaw))
}

// yamlMap builds an ordered YAML mapping from key, value pairs.
func yamlMap(kv ...any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(kv); i += 2 {
		val, err := encodeYAML(kv[i+1])
		if err != nil {
			continue
		}
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(kv[i])}, val)
	}
	return n
}
