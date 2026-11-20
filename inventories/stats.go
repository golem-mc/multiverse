package inventories

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// bukkitStat is one of Bukkit's Statistic constants, the names the
// game_statistics share is saved under: an untyped one counting a
// minecraft:custom statistic, or a typed one with a counter per item, block
// or entity type, saved as a map by key.
type bukkitStat struct {
	name string
	// stat is an untyped statistic's counter.
	stat plugin.Statistic
	// keys and stats are a typed statistic's counters, by key.
	keys  []string
	stats []plugin.Statistic
}

// bukkitCustomNames are the Bukkit names of the minecraft:custom
// statistics whose name is not their path in upper case.
// vanilla 26.3 keys; Bukkit names from CraftStatistic
var bukkitCustomNames = map[string]string{
	"minecraft:clean_armor":                  "ARMOR_CLEANED",
	"minecraft:clean_banner":                 "BANNER_CLEANED",
	"minecraft:drop":                         "DROP_COUNT",
	"minecraft:eat_cake_slice":               "CAKE_SLICES_EATEN",
	"minecraft:enchant_item":                 "ITEM_ENCHANTED",
	"minecraft:fill_cauldron":                "CAULDRON_FILLED",
	"minecraft:inspect_dispenser":            "DISPENSER_INSPECTED",
	"minecraft:inspect_dropper":              "DROPPER_INSPECTED",
	"minecraft:inspect_hopper":               "HOPPER_INSPECTED",
	"minecraft:interact_with_beacon":         "BEACON_INTERACTION",
	"minecraft:interact_with_brewingstand":   "BREWINGSTAND_INTERACTION",
	"minecraft:interact_with_crafting_table": "CRAFTING_TABLE_INTERACTION",
	"minecraft:interact_with_furnace":        "FURNACE_INTERACTION",
	"minecraft:open_chest":                   "CHEST_OPENED",
	"minecraft:open_enderchest":              "ENDERCHEST_OPENED",
	"minecraft:open_shulker_box":             "SHULKER_BOX_OPENED",
	"minecraft:play_noteblock":               "NOTEBLOCK_PLAYED",
	"minecraft:play_record":                  "RECORD_PLAYED",
	"minecraft:play_time":                    "PLAY_ONE_MINUTE",
	"minecraft:pot_flower":                   "FLOWER_POTTED",
	"minecraft:trigger_trapped_chest":        "TRAPPED_CHEST_TRIGGERED",
	"minecraft:tune_noteblock":               "NOTEBLOCK_TUNED",
	"minecraft:use_cauldron":                 "CAULDRON_USED",
}

// bukkitTypedNames are Bukkit's typed statistics. Bukkit: CraftStatistic
var bukkitTypedNames = []struct{ name, statType string }{
	{"BREAK_ITEM", registry.StatBroken}, {"CRAFT_ITEM", registry.StatCrafted},
	{"DROP", registry.StatDropped}, {"KILL_ENTITY", registry.StatKilled},
	{"ENTITY_KILLED_BY", registry.StatKilledBy}, {"MINE_BLOCK", registry.StatMined},
	{"PICKUP", registry.StatPickedUp}, {"USE_ITEM", registry.StatUsed},
}

// bukkitStats lists every statistic the game_statistics share switches.
var bukkitStats = sync.OnceValue(func() []bukkitStat {
	statOf := func(statType, key string) (plugin.Statistic, bool) {
		t, k, ok := registry.StatID(statType, key)
		return plugin.Statistic{Type: registry.StatType(t), Value: k}, ok
	}
	var out []bukkitStat
	for _, key := range registry.CustomStats {
		name, ok := bukkitCustomNames[key]
		if !ok {
			name = strings.ToUpper(strings.TrimPrefix(key, "minecraft:"))
		}
		st, _ := statOf(registry.StatCustom, key)
		out = append(out, bukkitStat{name: name, stat: st})
	}
	var blocks []string
	for name := range registry.BlockIDs {
		blocks = append(blocks, name)
	}
	slices.Sort(blocks)
	for _, typed := range bukkitTypedNames {
		keys := registry.Items[1:]
		switch typed.statType {
		case registry.StatMined:
			keys = blocks
		case registry.StatKilled, registry.StatKilledBy:
			keys = registry.EntityTypes
		}
		bs := bukkitStat{name: typed.name}
		for _, k := range keys {
			if st, ok := statOf(typed.statType, k); ok {
				bs.keys = append(bs.keys, k)
				bs.stats = append(bs.stats, st)
			}
		}
		out = append(out, bs)
	}
	return out
})

// readStats answers a player's non-zero statistics by Bukkit name: a count,
// or a map of counts by key. MV: Sharables.GAME_STATISTICS
func readStats(p plugin.Player) map[string]any {
	out := map[string]any{}
	for _, bs := range bukkitStats() {
		if bs.keys == nil {
			if v := p.Statistic(bs.stat); v != 0 {
				out[bs.name] = v
			}
			continue
		}
		counts := map[string]any{}
		for i, st := range bs.stats {
			if v := p.Statistic(st); v != 0 {
				counts[bs.keys[i]] = v
			}
		}
		if len(counts) > 0 {
			out[bs.name] = counts
		}
	}
	return out
}

// applyStats sets every statistic to its saved count, zero when not saved.
func (m *Module) applyStats(p plugin.Player, saved map[string]any) {
	set := func(st plugin.Statistic, raw any, what string) {
		v, ok := statCount(raw)
		if !ok {
			m.Log.Warn("Invalid statistic value for " + what)
			return
		}
		if p.Statistic(st) != v {
			p.SetStatistic(st, v)
		}
	}
	for _, bs := range bukkitStats() {
		raw := saved[bs.name]
		if bs.keys == nil {
			set(bs.stat, raw, bs.name)
			continue
		}
		counts, ok := raw.(map[string]any)
		if raw != nil && !ok {
			m.Log.Warn("Invalid statistic value for " + bs.name)
			continue
		}
		for i, st := range bs.stats {
			set(st, counts[bs.keys[i]], bs.name+" "+bs.keys[i])
		}
	}
}

// statCount reads a saved count; nil is zero.
func statCount(raw any) (int, bool) {
	switch v := raw.(type) {
	case nil:
		return 0, true
	case int:
		return v, true
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	case float64:
		return int(v), v == float64(int(v))
	}
	return 0, false
}

// encodeStats writes the statistics with sorted keys.
func encodeStats(_ *Module, v any) any {
	sortedObj := func(m map[string]any) obj {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(obj, len(keys))
		for i, k := range keys {
			out[i] = kv{k, m[k]}
		}
		return out
	}
	stats, _ := v.(map[string]any)
	out := sortedObj(stats)
	for i, e := range out {
		if counts, ok := e.v.(map[string]any); ok {
			out[i].v = sortedObj(counts)
		}
	}
	return out
}

func decodeStats(_ *Module, raw any) (any, bool) {
	m, ok := raw.(map[string]any)
	return m, ok
}
