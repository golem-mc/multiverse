package inventories

import (
	"math"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// Player defaults a sharable falls back to. MV: PlayerStats
const (
	inventorySize   = 36
	armorSize       = 4
	enderChestSize  = 27
	defaultHealth   = 20.0
	defaultMaxAir   = 300
	defaultFood     = 20
	defaultSaturate = 5
)

// The sharables, in Multiverse-Inventories' order. MV: Sharables
var (
	EnderChest = register(&Sharable{names: []string{"ender_chest", "ender"}, fileTag: "enderChestContents",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, p.EnderChest().Contents())
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			inv := p.EnderChest()
			v, ok := d.Get(s).([]plugin.ItemStack)
			contents := make([]plugin.ItemStack, inv.Size())
			copy(contents, v)
			inv.SetContents(contents)
			return ok
		},
		encode: encodeSlots, decode: decodeSlots("misc.max-items-size.ender-chest")})

	Inventory = register(&Sharable{names: []string{"inventory_contents"}, fileTag: "inventoryContents",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, slotsOf(p.Inventory(), 0, inventorySize))
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).([]plugin.ItemStack)
			setSlots(p.Inventory(), 0, inventorySize, v)
			return ok
		},
		encode: encodeSlots, decode: decodeSlots("misc.max-items-size.inventory")})

	Armor = register(&Sharable{names: []string{"armor_contents", "armor"}, fileTag: "armorContents",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, slotsOf(p.Inventory(), inventorySize, armorSize))
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).([]plugin.ItemStack)
			setSlots(p.Inventory(), inventorySize, armorSize, v)
			return ok
		},
		encode: encodeSlots, decode: decodeSlots("misc.max-items-size.armor")})

	OffHand = register(&Sharable{names: []string{"off_hand", "shield"}, fileTag: "offHandItem",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, p.Inventory().Item(offHandSlot))
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).(plugin.ItemStack)
			p.Inventory().SetItem(offHandSlot, v)
			return ok
		},
		encode: func(m *Module, v any) any { return m.encodeItem(v.(plugin.ItemStack)) },
		decode: func(m *Module, raw any) (any, bool) {
			s, ok := m.decodeItem(raw)
			return s, ok
		}})

	MaxHealth = register(&Sharable{names: []string{"max_hit_points", "maxhealth", "maxhp", "maxhitpoints"},
		stat: true, fileTag: "mhp",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, baseMaxHealth(p))
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).(float64)
			if !ok {
				v = defaultHealth
			}
			setBaseMaxHealth(p, v)
			return ok
		},
		encode: encodeStat, decode: decodeDouble})

	Health = register(&Sharable{names: []string{"hit_points", "health", "hp", "hitpoints"},
		stat: true, fileTag: "hp",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			health := float64(p.Health())
			// A dead player gets their health back on respawn.
			if health <= 0 {
				health = float64(p.MaxHealth())
			}
			d.Set(s, health)
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).(float64)
			switch {
			case !ok:
				p.SetHealth(defaultHealth)
				return false
			case v < 0:
				p.SetHealth(0)
				return false
			}
			if v > float64(p.MaxHealth()) {
				maxHealth, ok := d.Get(MaxHealth).(float64)
				if !ok {
					maxHealth = defaultHealth
				}
				setBaseMaxHealth(p, maxHealth)
			}
			p.SetHealth(float32(min(v, float64(p.MaxHealth()))))
			return true
		},
		encode: encodeStat, decode: decodeDouble})

	RemainingAir = register(intStat([]string{"remaining_air"}, "ra", defaultMaxAir,
		func(p plugin.Player) int { return p.Air() }, func(p plugin.Player, v int) { p.SetAir(v) }))

	// MaximumAir is Bukkit's own maximum, which vanilla fixes at 300: it is
	// saved as 300 and changes nothing.
	MaximumAir = register(intStat([]string{"maximum_air"}, "ma", defaultMaxAir,
		func(plugin.Player) int { return defaultMaxAir }, func(plugin.Player, int) {}))

	FallDistance = register(floatStat([]string{"fall_distance", "falling"}, "fd", 0,
		func(p plugin.Player) float32 { return float32(p.FallDistance()) },
		func(p plugin.Player, v float32) { p.SetFallDistance(float64(v)) }))

	FireTicks = register(intStat([]string{"fire_ticks", "fire", "burning"}, "ft", 0,
		func(p plugin.Player) int { return p.FireTicks() }, func(p plugin.Player, v int) { p.SetFireTicks(v) }))

	Experience = register(floatStat([]string{"xp"}, "xp", 0,
		func(p plugin.Player) float32 { return p.Progress() }, func(p plugin.Player, v float32) { p.SetProgress(v) }))

	Level = register(intStat([]string{"lvl"}, "el", 0,
		func(p plugin.Player) int { return p.Level() }, func(p plugin.Player, v int) { p.SetLevel(v) }))

	// TotalExperience sets the points earned alone, as Bukkit's
	// setTotalExperience does: golem's restarts the bar, so the level and
	// progress are put back.
	TotalExperience = register(intStat([]string{"total_xp"}, "txp", 0,
		func(p plugin.Player) int { return p.TotalExperience() },
		func(p plugin.Player, v int) {
			level, progress := p.Level(), p.Progress()
			p.SetTotalExperience(v)
			p.SetLevel(level)
			p.SetProgress(progress)
		}))

	FoodLevel = register(intStat([]string{"food_level", "food"}, "fl", defaultFood,
		func(p plugin.Player) int { return p.Food() }, func(p plugin.Player, v int) { p.SetFood(v) }))

	Exhaustion = register(floatStat([]string{"exhaustion", "exhaust", "exh"}, "ex", 0,
		func(p plugin.Player) float32 { return p.Exhaustion() }, func(p plugin.Player, v float32) { p.SetExhaustion(v) }))

	Saturation = register(floatStat([]string{"saturation", "sat"}, "sa", defaultSaturate,
		func(p plugin.Player) float32 { return p.Saturation() }, func(p plugin.Player, v float32) { p.SetSaturation(v) }))

	BedSpawn = register(&Sharable{names: []string{"bed_spawn", "bedspawn", "bed", "beds", "bedspawns"},
		fileTag: "bedSpawnLocation",
		read: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, m.respawnLocationOf(p))
		},
		apply: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			return m.applyBedSpawn(p, d.Get(s))
		},
		encode: encodeLocation, decode: decodeLocation})

	// LastLocation is written as a player leaves a world (on teleport,
	// quit and game mode change), when it is too late to read here.
	LastLocation = register(&Sharable{names: []string{"last_location", "loc", "location", "pos", "position"},
		optional: true, fileTag: "lastLocation",
		read: func(*Module, *Sharable, plugin.Player, *ProfileData) {},
		apply: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			loc, ok := d.Get(s).(*Loc)
			if !ok {
				return false
			}
			to, ok := m.locationOf(loc)
			if !ok || to == p.Location() {
				return false
			}
			p.Teleport(to, plugin.TeleportPlugin)
			return true
		},
		encode: encodeLocation, decode: decodeLocation})

	// Economy is the balance in golem-mc/vault's economy service; without
	// one it warns as Multiverse-Inventories does without Vault.
	Economy = register(&Sharable{names: []string{"economy", "money", "econ", "cash", "balance"},
		optional: true, fileTag: "balance",
		read: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			if eco, ok := m.economy(); ok {
				d.Set(s, eco.Balance(p.UUID()))
			}
		},
		apply: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			eco, ok := m.economy()
			if !ok {
				return false
			}
			money, ok := d.Get(s).(float64)
			// MV: MVEconomist.setBalance, withdrawing the balance and depositing the new one
			switch cur := eco.Balance(p.UUID()); {
			case money > cur:
				eco.Deposit(p.UUID(), money-cur)
			case money < cur:
				eco.Withdraw(p.UUID(), cur-money)
			}
			return ok
		},
		encode: encodeStat, decode: decodeDouble})

	Potions = register(&Sharable{names: []string{"potion_effects", "potion", "potions"}, fileTag: "potions",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) {
			d.Set(s, p.Effects())
		},
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			p.ClearEffects()
			effects, ok := d.Get(s).([]plugin.EffectInstance)
			for _, e := range effects {
				p.AddEffect(e)
			}
			return ok
		},
		encode: encodeEffects, decode: decodeEffects})

	// Advancements and Recipes are kept in the profile files but not read
	// from or applied to players: see the README.
	Advancements = register(&Sharable{names: []string{"advancements", "achievements"}, optional: true,
		fileTag: "advancements", read: keepProfile, apply: hasValue,
		encode: identity, decode: decodeRaw})

	GameStatistics = register(&Sharable{names: []string{"game_statistics"}, fileTag: "game_statistics",
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) { d.Set(s, readStats(p)) },
		apply: func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			saved, ok := d.Get(s).(map[string]any)
			m.applyStats(p, saved)
			return ok
		},
		encode: encodeStats, decode: decodeStats})

	Recipes = register(&Sharable{names: []string{"recipes"}, optional: true, fileTag: "recipes",
		read: keepProfile, apply: hasValue,
		encode: identity, decode: decodeRaw})
)

// The groups of sharables. MV: Sharables
var (
	AllInventory  = group(SharesOf(Inventory, Armor, EnderChest, OffHand), "inventory", "inv", "inventories")
	AllExperience = group(SharesOf(Experience, TotalExperience, Level), "experience", "exp", "level")
	Air           = group(SharesOf(RemainingAir, MaximumAir), "air", "breath")
	Hunger        = group(SharesOf(FoodLevel, Saturation, Exhaustion), "hunger")
	AllHealth     = group(SharesOf(Health, MaxHealth, RemainingAir, MaximumAir, FallDistance, FireTicks), "health")
	Stats         = group(SharesOf(Health, MaxHealth, FoodLevel, Saturation, Exhaustion, Experience, TotalExperience,
		Level, RemainingAir, MaximumAir, FallDistance, FireTicks, Potions, GameStatistics, Advancements), "stats")
	AllDefault = group(SharesOf(Health, MaxHealth, Economy, FoodLevel, Saturation, Exhaustion, Experience,
		TotalExperience, Level, Inventory, Armor, BedSpawn, MaximumAir, RemainingAir, FallDistance, FireTicks,
		Potions, LastLocation, EnderChest, OffHand, GameStatistics, Advancements, Recipes), "all", "*", "everything")
)

// offHandSlot is the off hand in golem's player inventory numbering.
const offHandSlot = 40

func slotsOf(inv plugin.Inventory, from, n int) []plugin.ItemStack {
	out := make([]plugin.ItemStack, n)
	for i := range n {
		out[i] = inv.Item(from + i)
	}
	return out
}

func setSlots(inv plugin.Inventory, from, n int, v []plugin.ItemStack) {
	for i := range n {
		var s plugin.ItemStack
		if i < len(v) {
			s = v[i]
		}
		inv.SetItem(from+i, s)
	}
}

func maxHealthAttribute(p plugin.Player) (plugin.AttributeInstance, bool) {
	return p.Attribute(registry.AttributeMaxHealth)
}

func baseMaxHealth(p plugin.Player) float64 {
	if a, ok := maxHealthAttribute(p); ok {
		return a.Base()
	}
	return defaultHealth
}

func setBaseMaxHealth(p plugin.Player, v float64) {
	if a, ok := maxHealthAttribute(p); ok {
		a.SetBase(v)
	}
}

// intStat and floatStat build a sharable of one number kept in "stats".
func intStat(names []string, tag string, def int, get func(plugin.Player) int, set func(plugin.Player, int)) *Sharable {
	return &Sharable{names: names, stat: true, fileTag: tag, encode: encodeStat, decode: decodeInt,
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) { d.Set(s, get(p)) },
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).(int)
			if !ok {
				v = def
			}
			set(p, v)
			return ok
		}}
}

func floatStat(names []string, tag string, def float32, get func(plugin.Player) float32, set func(plugin.Player, float32)) *Sharable {
	return &Sharable{names: names, stat: true, fileTag: tag, encode: encodeStat, decode: decodeFloat,
		read: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) { d.Set(s, get(p)) },
		apply: func(_ *Module, s *Sharable, p plugin.Player, d *ProfileData) bool {
			v, ok := d.Get(s).(float32)
			if !ok || math.IsNaN(float64(v)) {
				v, ok = def, false
			}
			set(p, v)
			return ok
		}}
}

// keepProfile leaves what the profile holds as it is.
func keepProfile(*Module, *Sharable, plugin.Player, *ProfileData) {}

// hasValue reports whether the profile holds a value, changing nothing.
func hasValue(_ *Module, s *Sharable, _ plugin.Player, d *ProfileData) bool { return d.Get(s) != nil }

func identity(_ *Module, v any) any { return v }

func decodeRaw(_ *Module, raw any) (any, bool) { return raw, raw != nil }
