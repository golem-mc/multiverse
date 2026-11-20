package commands_test

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// Changing a world's gamemode or allow-flight applies to the players in it,
// a player joining gets the world's game mode, and enforce-gamemode off
// leaves players alone.
func TestGameModeAndFlightAreEnforced(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	alice := env.Join("Alice", true).Handle()
	env.Console("mv modify world set gamemode creative")
	env.Sync(func() {
		if alice.GameMode() != plugin.Creative || !alice.AllowFlight() {
			t.Errorf("game mode %d, flight %v; want creative with flight", alice.GameMode(), alice.AllowFlight())
		}
	})
	env.Console("mv modify world set gamemode survival")
	env.Sync(func() { alice.SetAllowFlight(true) })
	env.Console("mv modify world set allow-flight true")
	env.Console("mv modify world set allow-flight false")
	env.Sync(func() {
		if alice.GameMode() != plugin.Survival || alice.AllowFlight() {
			t.Errorf("game mode %d, flight %v; want survival without flight", alice.GameMode(), alice.AllowFlight())
		}
	})

	env.Console("mv modify world set gamemode adventure")
	bob := env.Join("Bob", false)
	bob.Read(2)
	env.Sync(func() {
		if bob.Handle().GameMode() != plugin.Adventure {
			t.Errorf("Bob joined in game mode %d, want adventure", bob.Handle().GameMode())
		}
	})
	env.Console("mv config enforce-gamemode false")
	env.Console("mv modify world set gamemode creative")
	env.Sync(func() {
		if alice.GameMode() != plugin.Adventure {
			t.Errorf("game mode %d with enforce-gamemode off, want adventure kept", alice.GameMode())
		}
	})
}

// A player respawns at the spawn of the world they died in, of its overworld
// when it is a nether, or of its respawn-world; a bed respawn is kept, and
// MVRespawnEvent can change or cancel the move.
func TestRespawnFollowsTheWorldsRespawnSettings(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": `
lobby:
  generator: Void
  read-only:
    environment: normal
lobby_nether:
  generator: Void
  read-only:
    environment: nether
`}, plugintestOps("Alice"))
	alice := env.Join("Alice", true).Handle()
	lobby, _ := env.MV.Worlds.Get("lobby")
	nether, _ := env.MV.Worlds.Get("lobby_nether")
	overworld, _ := env.MV.Worlds.Get("world")
	respawnAt := func(w *core.World, bed bool) plugin.Location {
		var loc plugin.Location
		env.Sync(func() {
			h, _ := w.Handle()
			alice.Teleport(plugin.Location{World: h, Pos: plugin.Vec3{X: 0.5, Y: 70, Z: 0.5}}, plugin.TeleportPlugin)
			e := &plugin.PlayerRespawnEvent{Player: alice, Bed: bed,
				Location: plugin.Location{World: h, Pos: plugin.Vec3{X: 100, Y: 80, Z: 100}}}
			env.MV.Server.Events().Fire(e)
			loc = e.Location
		})
		return loc
	}
	lobbySpawn, _ := lobby.Spawn()
	if loc := respawnAt(nether, false); loc.World.Key() != lobby.Key() || loc.Pos != lobbySpawn.Pos {
		t.Errorf("respawn from the nether at %v %v, want lobby's spawn %v", loc.World.Key(), loc.Pos, lobbySpawn.Pos)
	}
	if loc := respawnAt(lobby, false); loc.World.Key() != lobby.Key() || loc.Pos != lobbySpawn.Pos {
		t.Errorf("respawn in lobby at %v %v, want its spawn", loc.World.Key(), loc.Pos)
	}
	if loc := respawnAt(lobby, true); loc.Pos.X != 100 {
		t.Errorf("bed respawn moved to %v", loc.Pos)
	}
	env.Console("mv modify lobby set respawn-world world")
	spawn, _ := overworld.Spawn()
	if loc := respawnAt(lobby, false); loc.World.Key() != overworld.Key() || loc.Pos != spawn.Pos {
		t.Errorf("respawn with respawn-world at %v %v, want world's spawn %v", loc.World.Key(), loc.Pos, spawn.Pos)
	}
	plugin.Listen(env.Srv.Host(), func(e *core.MVRespawnEvent) { e.SetCancelled(true) })
	env.Console("mv modify lobby set difficulty hard")
	if h, _ := lobby.Handle(); h.Difficulty() != plugin.DifficultyHard || env.MV.Server.Overworld().Difficulty() == plugin.DifficultyHard {
		t.Errorf("difficulty hard in lobby: lobby %d, overworld %d", h.Difficulty(), env.MV.Server.Overworld().Difficulty())
	}
	if loc := respawnAt(lobby, false); loc.World.Key() != lobby.Key() || loc.Pos.X != 100 {
		t.Errorf("cancelled MVRespawnEvent still moved the respawn to %v %v", loc.World.Key(), loc.Pos)
	}
}

// hunger, auto-heal, pvp and the spawning section cancel the events they
// cover in their world.
func TestEntityPropertiesCancelEvents(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": `
world:
  hunger: false
  auto-heal: false
  pvp: false
  spawning:
    monster:
      spawn: false
      exceptions:
      - CREEPER
`}, plugintestOps("Alice"))
	alice := env.Join("Alice", true).Handle()
	bob := env.Join("Bob", false).Handle()
	fire := func(e interface {
		plugin.Event
		Cancelled() bool
	}) (cancelled bool) {
		env.Sync(func() { env.MV.Server.Events().Fire(e); cancelled = e.Cancelled() })
		return cancelled
	}
	if !fire(&plugin.FoodLevelChangeEvent{Player: alice, Level: alice.Food() - 1}) {
		t.Error("hunger: false did not stop food going down")
	}
	if fire(&plugin.FoodLevelChangeEvent{Player: alice, Level: alice.Food() + 1}) {
		t.Error("hunger: false stopped food going up")
	}
	if !fire(&plugin.EntityDamageEvent{Entity: bob, Source: plugin.DamageSource{Causing: alice}, Amount: 1}) {
		t.Error("pvp: false did not stop a player hurting a player")
	}
	if fire(&plugin.EntityDamageEvent{Entity: bob, Amount: 1}) {
		t.Error("pvp: false stopped damage with no player behind it")
	}
	var zombie, creeper, cow plugin.Entity
	env.Sync(func() {
		h := env.MV.Server.Overworld()
		at := plugin.Location{World: h, Pos: plugin.Vec3{X: 3, Y: 64, Z: 3}}
		zombie, _ = h.Spawn(registry.EntityTypeZombie, at, nil)
		creeper, _ = h.Spawn(registry.EntityTypeCreeper, at, nil)
		cow, _ = h.Spawn(registry.EntityTypeCow, at, nil)
	})
	if zombie == nil || creeper == nil || cow == nil {
		t.Fatal("a plugin's spawn was refused")
	}
	if !fire(&plugin.EntitySpawnEvent{Entity: zombie, Reason: plugin.SpawnNatural}) {
		t.Error("monster spawn: false let a zombie spawn")
	}
	if fire(&plugin.EntitySpawnEvent{Entity: creeper, Reason: plugin.SpawnNatural}) {
		t.Error("the CREEPER exception was refused")
	}
	if fire(&plugin.EntitySpawnEvent{Entity: cow, Reason: plugin.SpawnNatural}) {
		t.Error("animals were refused")
	}
	if fire(&plugin.EntitySpawnEvent{Entity: zombie, Reason: plugin.SpawnCommand}) ||
		fire(&plugin.EntitySpawnEvent{Entity: zombie, Reason: plugin.SpawnSpawnerEgg}) {
		t.Error("a summoned or spawn-egg zombie was refused")
	}

	// Peaceful last: it removes the monsters above.
	if fire(&plugin.EntityRegainHealthEvent{Entity: alice, Amount: 1, Reason: plugin.RegainNatural}) {
		t.Error("auto-heal: false stopped natural healing outside peaceful")
	}
	env.Console("difficulty peaceful")
	if !fire(&plugin.EntityRegainHealthEvent{Entity: alice, Amount: 1, Reason: plugin.RegainNatural}) {
		t.Error("auto-heal: false did not stop peaceful regeneration")
	}
	if fire(&plugin.EntityRegainHealthEvent{Entity: alice, Amount: 1, Reason: plugin.RegainEating}) {
		t.Error("auto-heal: false stopped healing by eating")
	}
}

// allow-weather stops rain and clears it, allow-advancement-grant stops
// criteria, portal-form stops portals, and the chat prefix shows the world.
func TestWeatherAdvancementsPortalsAndChatPrefix(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)
	h := env.MV.Server.Overworld()
	env.Console("weather rain")
	env.Console("mv modify world set allow-weather false")
	if h.Weather() != plugin.WeatherClear {
		t.Errorf("weather %d after allow-weather false, want clear", h.Weather())
	}
	env.Console("weather thunder")
	if h.Weather() != plugin.WeatherClear {
		t.Errorf("weather %d after /weather thunder, want clear", h.Weather())
	}

	env.Console("mv modify world set allow-advancement-grant false")
	adv, _ := env.MV.Server.Advancement(plugin.MinecraftKey("story/mine_stone"))
	var granted bool
	env.Sync(func() { granted = alice.Handle().Advancements(adv).Award("get_stone") })
	if granted {
		t.Error("a criterion was granted with allow-advancement-grant false")
	}
	env.Console("mv modify world set allow-advancement-grant true")
	env.Sync(func() { granted = alice.Handle().Advancements(adv).Award("get_stone") })
	if !granted {
		t.Error("a criterion was refused with allow-advancement-grant true")
	}

	env.Console("mv modify world set portal-form end")
	alice.Messages(1)
	e := &plugin.PortalCreateEvent{World: h, Reason: plugin.PortalNetherPair, Entity: alice.Handle()}
	env.Sync(func() { env.MV.Server.Events().Fire(e) })
	if !e.Cancelled() {
		t.Error("portal-form end let a nether portal form")
	}
	env.Expect(alice.Messages(1), "Creating nether portals is disabled in this world.")
	env.Console("mv modify world set portal-form nether")
	frame := plugin.BlockPos{X: 4, Y: 64, Z: 4}
	use := &plugin.PlayerInteractEvent{Player: alice.Handle(), Action: plugin.RightClickBlock,
		Item: plugin.ItemStack{Item: registry.ItemEnderEye, Count: 1}, Block: &plugin.Block{World: h, Pos: frame}}
	env.Sync(func() {
		h.SetBlockState(frame, plugin.DefaultState(registry.BlockEndPortalFrame), plugin.UpdateAll)
		env.MV.Server.Events().Fire(use)
	})
	if !use.Cancelled() {
		t.Error("portal-form nether let an eye go into a frame")
	}
	env.Expect(alice.Messages(1), "Creating end portals is disabled in this world.")

	env.Console("mv config use-custom-portal-search true")
	env.Console("mv config custom-portal-search-radius 32")
	trip := &plugin.EntityPortalEvent{Entity: alice.Handle(), To: plugin.Location{World: h}, SearchRadius: 128}
	env.Sync(func() { env.MV.Server.Events().Fire(trip) })
	if trip.SearchRadius != 32 {
		t.Errorf("portal search radius %d, want 32", trip.SearchRadius)
	}

	env.Console("mv config enable-chat-prefix true")
	env.Console("mv modify world set alias &aOverworld")
	bob.Messages(1)
	env.Sync(func() { alice.Handle().Chat("hello") })
	env.Expect(bob.Messages(2), "[Overworld]<Alice> hello")
}

// The spawning section's tick rates and limits reach the world, on load and
// from /mv entity-spawn-config, and a chunk loading loses the entities the
// section forbids, but not a plugin's.
func TestSpawningAppliesRatesLimitsAndCleansChunks(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": `
world:
  spawning:
    animal:
      tick-rate: 100
      spawn-limit: 5
    water_animal:
      spawn: false
    monster:
      spawn: false
      exceptions:
      - CREEPER
`}, plugintestOps("Alice"))
	env.Join("Alice", true)
	h := env.MV.Server.Overworld()
	if h.TicksPerSpawn(registry.MobCategoryCreature) != 100 || h.SpawnLimit(registry.MobCategoryCreature) != 5 ||
		h.TicksPerSpawn(registry.MobCategoryWaterCreature) != 0 || h.TicksPerSpawn(registry.MobCategoryMonster) != -1 {
		t.Errorf("animal %d/%d, water animal %d, monster %d", h.TicksPerSpawn(registry.MobCategoryCreature),
			h.SpawnLimit(registry.MobCategoryCreature), h.TicksPerSpawn(registry.MobCategoryWaterCreature),
			h.TicksPerSpawn(registry.MobCategoryMonster))
	}
	env.Console("mv entity-spawn-config modify world ambient set tick-rate 50")
	if h.TicksPerSpawn(registry.MobCategoryAmbient) != 50 {
		t.Errorf("ambient tick rate %d after entity-spawn-config, want 50", h.TicksPerSpawn(registry.MobCategoryAmbient))
	}

	// A shot arrow is a default spawn of the misc category.
	var arrow, zombie plugin.Entity
	env.Sync(func() {
		zombie, _ = h.Spawn(registry.EntityTypeZombie, plugin.Location{World: h, Pos: plugin.Vec3{X: 3, Y: 64, Z: 3}}, nil)
		arrow, _ = zombie.(plugin.LivingEntity).LaunchProjectile(registry.EntityTypeArrow, plugin.Vec3{})
		w, _ := env.MV.Worlds.Of(h)
		v := w.Spawning().Category("misc")
		n, _ := v.Nodes().Find("spawn")
		_ = v.Set(n, false)
		chunk, _ := h.Chunk(plugin.ChunkPos{})
		env.MV.Server.Events().Fire(&plugin.ChunkLoadEvent{Chunk: chunk})
		if arrow.IsValid() || !zombie.IsValid() {
			t.Errorf("after the chunk load: arrow kept %v, plugin's zombie kept %v", arrow.IsValid(), zombie.IsValid())
		}
	})
}
