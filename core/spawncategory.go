package core

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// bukkitCategories names each of vanilla's mob categories as Bukkit's
// SpawnCategory does, the names worlds.yml uses. MV: CraftSpawnCategory.toBukkit
var bukkitCategories = [registry.MobCategoryCount]string{
	registry.MobCategoryMonster:                  "monster",
	registry.MobCategoryCreature:                 "animal",
	registry.MobCategoryAmbient:                  "ambient",
	registry.MobCategoryAxolotls:                 "axolotl",
	registry.MobCategoryUndergroundWaterCreature: "water_underground_creature",
	registry.MobCategoryWaterCreature:            "water_animal",
	registry.MobCategoryWaterAmbient:             "water_ambient",
	registry.MobCategoryMisc:                     "misc",
}

// MobCategoryOf answers vanilla's mob category of a spawn category.
func MobCategoryOf(category string) (registry.MobCategory, bool) {
	i := slices.Index(bukkitCategories[:], category)
	return registry.MobCategory(i), i >= 0
}

// SpawnCategoryOf answers the spawn category of an entity type, by its key
// ("minecraft:zombie" or "zombie"): one of SpawnCategories. MV:
// SpawnCategoryMapper
func SpawnCategoryOf(entityType string) string {
	t, ok := registry.EntityTypeByName("minecraft:" + strings.TrimPrefix(strings.ToLower(entityType), "minecraft:"))
	if !ok {
		return "misc"
	}
	return bukkitCategories[t.Category()]
}

// SpawnCategoryTypes answers the entity types (key paths) in a spawn
// category; nil for misc, which holds every type no other category does.
func SpawnCategoryTypes(category string) []string {
	if category == "misc" {
		return nil
	}
	var out []string
	for _, name := range registry.EntityTypes {
		if SpawnCategoryOf(name) == category {
			out = append(out, strings.TrimPrefix(name, "minecraft:"))
		}
	}
	return out
}

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
