package core

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// spawnCategoryTypes lists, per Bukkit spawn category, the entity types
// (registry key paths) vanilla 26.3 registers in it; every other type is
// misc. golem's plugin API has no EntityType category yet
// (golem-mc/golem#70), so this follows vanilla's EntityTypes by hand.
// MV: SpawnCategoryMapper; vanilla: EntityType.Builder.of(..., MobCategory)
var spawnCategoryTypes = map[string][]string{
	"monster": {"blaze", "bogged", "breeze", "camel_husk", "cave_spider", "creaking", "creeper", "drowned",
		"elder_guardian", "ender_dragon", "enderman", "endermite", "evoker", "ghast", "giant", "guardian", "hoglin",
		"husk", "illusioner", "magma_cube", "parched", "phantom", "piglin", "piglin_brute", "pillager", "ravager",
		"shulker", "silverfish", "skeleton", "slime", "spider", "stray", "sulfur_cube", "vex", "vindicator", "warden",
		"witch", "wither", "wither_skeleton", "zoglin", "zombie", "zombie_horse", "zombie_nautilus",
		"zombie_villager", "zombified_piglin"},
	"animal": {"allay", "armadillo", "bee", "camel", "cat", "chicken", "cow", "donkey", "fox", "frog", "goat",
		"happy_ghast", "horse", "llama", "mooshroom", "mule", "ocelot", "panda", "parrot", "pig", "polar_bear",
		"rabbit", "sheep", "skeleton_horse", "sniffer", "strider", "tadpole", "trader_llama", "turtle",
		"wandering_trader", "wolf"},
	"water_animal":               {"dolphin", "nautilus", "squid"},
	"water_ambient":              {"cod", "pufferfish", "salmon", "tropical_fish"},
	"water_underground_creature": {"glow_squid"},
	"ambient":                    {"bat"},
	"axolotl":                    {"axolotl"},
}

var spawnCategoryOf = func() map[string]string {
	m := map[string]string{}
	for category, types := range spawnCategoryTypes {
		for _, t := range types {
			m[t] = category
		}
	}
	return m
}()

// SpawnCategoryOf answers the spawn category of an entity type, by its key
// ("minecraft:zombie" or "zombie"): one of SpawnCategories.
func SpawnCategoryOf(entityType string) string {
	if c, ok := spawnCategoryOf[strings.TrimPrefix(strings.ToLower(entityType), "minecraft:")]; ok {
		return c
	}
	return "misc"
}

// SpawnCategoryTypes answers the entity types (key paths) in a spawn
// category; nil for misc, which holds every type no other category does.
func SpawnCategoryTypes(category string) []string { return spawnCategoryTypes[category] }

// AllowsEntity reports whether an entity of entityType (its key) may be in
// the world by its category's settings. MV: EntitySpawnConfig.shouldAllowSpawn
func (s *Spawning) AllowsEntity(entityType string) bool {
	t := strings.TrimPrefix(strings.ToLower(entityType), "minecraft:")
	return s.Allows(SpawnCategoryOf(t), t)
}

// PurgeEntities removes the entities of the loaded world, players aside,
// that purge selects, and answers how many. MV: EntityPurger
func (w *World) PurgeEntities(purge func(plugin.Entity) bool) int {
	h, ok := w.Handle()
	if !ok {
		return 0
	}
	var doomed []plugin.Entity
	for e := range h.Entities() {
		if _, player := e.(plugin.Player); !player && purge(e) {
			doomed = append(doomed, e)
		}
	}
	for _, e := range doomed {
		e.Remove()
	}
	return len(doomed)
}

// PurgeDisallowedEntities removes the entities the world's spawning
// settings do not allow. MV: EntityPurger.purgeEntities
func (w *World) PurgeDisallowedEntities() int {
	s := w.Spawning()
	return w.PurgeEntities(func(e plugin.Entity) bool { return !s.AllowsEntity(e.Type().Name()) })
}
