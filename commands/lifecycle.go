package commands

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, lifecycleCommands) }

// lifecycleCommands make, load and drop worlds. MV: CreateCommand,
// ImportCommand, LoadCommand, UnloadCommand, RemoveCommand, DeleteCommand,
// CloneCommand, RegenCommand
func lifecycleCommands(c *core.Core) []*core.Cmd {
	generator := &core.Flag{Name: "--generator", Aliases: []string{"-g"}, Value: true,
		Complete: func(ctx *core.Ctx, _ string) []string { return ctx.Core.GeneratorNames() }}
	biome := &core.Flag{Name: "--biome", Aliases: []string{"-b"}, Value: true}
	settings := &core.Flag{Name: "--generator-settings", Aliases: []string{"-gs"}, Value: true}
	noAdjust := &core.Flag{Name: "--no-adjust-spawn", Aliases: []string{"-n"}}
	skipFolder := &core.Flag{Name: "--skip-folder-check", Aliases: []string{"-f"}}
	seed := func(optional bool) *core.Flag {
		return &core.Flag{Name: "--seed", Aliases: []string{"-s"}, Value: true, Optional: optional,
			Complete: func(*core.Ctx, string) []string { return []string{strconv.FormatInt(rand.Int64()-rand.Int64(), 10)} }}
	}
	resetConfig := &core.Flag{Name: "--reset-world-config", Aliases: []string{"-wc"}}
	resetRules := &core.Flag{Name: "--reset-gamerules", Aliases: []string{"-gm"}}
	resetBorder := &core.Flag{Name: "--reset-world-border", Aliases: []string{"-wb"}}
	removePlayers := &core.Flag{Name: "--remove-players", Aliases: []string{"-r"}, Value: true, Optional: true,
		Complete: func(ctx *core.Ctx, input string) []string {
			return ctx.Core.Destinations.Suggest(ctx.Core.Server.Console(), input)
		}}
	noUnloadServer := &core.Flag{Name: "--no-unload-bukkit-world", Aliases: []string{"-b"}}
	noSave := &core.Flag{Name: "--no-save", Aliases: []string{"-n"}}
	environments := func(ctx *core.Ctx, pos int) []string {
		if pos == 1 {
			return core.Environments
		}
		return nil
	}
	return []*core.Cmd{{
		Name: "create", Legacy: []string{"mvcreate", "mvc"}, Permission: "multiverse.core.create",
		Syntax: "<name> <environment> [--seed <seed> --generator <generator[:id]> --world-type <worldtype> " +
			"--no-adjust-spawn --no-structures --generate-bonus-chest --force-spawn-position <x,y,z:pitch:yaw> " +
			"--biome <biome> --generator-settings <json-settings> --properties <prop1=value1,prop2=value2,...>]",
		Description: "mv-core.create.description", MinArgs: 2, MaxArgs: 2,
		Flags: []*core.Flag{seed(false), generator, settings,
			{Name: "--world-type", Aliases: []string{"-t"}, Value: true,
				Complete: func(*core.Ctx, string) []string { return core.WorldTypes }},
			noAdjust, {Name: "--no-structures", Aliases: []string{"-a"}}, biome,
			{Name: "--properties", Aliases: []string{"-p"}, Value: true},
			{Name: "--generate-bonus-chest", Aliases: []string{"-c"}},
			{Name: "--force-spawn-position", Aliases: []string{"-f"}, Value: true}},
		Complete: environments,
		Run:      runCreate,
	}, {
		Name: "import", Legacy: []string{"mvimport", "mvim"}, Permission: "multiverse.core.import",
		Syntax: "<name> <environment> [--generator <generator[:id]> --no-adjust-spawn --biome <biome> " +
			"--skip-folder-check --generator-settings <json-settings>]",
		Description: "mv-core.import.description", MinArgs: 2, MaxArgs: 2,
		Flags: []*core.Flag{generator, noAdjust, biome, skipFolder, settings},
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return ctx.Core.PotentialWorlds()
			}
			return environments(ctx, pos)
		},
		Run: func(ctx *core.Ctx) error {
			env, err := environmentArg(ctx.Arg(1))
			if err != nil {
				return err
			}
			ctx.Info(core.M("mv-core.import.importing", "{world}", ctx.Arg(0)))
			w, err := ctx.Core.Worlds.Import(core.ImportOptions{Name: ctx.Arg(0), Environment: env,
				Generator: ctx.FlagOr("--generator", ""), Biome: ctx.FlagOr("--biome", ""),
				GeneratorSettings: ctx.FlagOr("--generator-settings", ""),
				NoAdjust:          ctx.Has("--no-adjust-spawn"), SkipFolderCheck: ctx.Has("--skip-folder-check")})
			if err != nil {
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			ctx.Info(core.M("mv-core.import.success", "{world}", w.Name()))
			return nil
		},
	}, {
		Name: "load", Legacy: []string{"mvload"}, Permission: "multiverse.core.load",
		Syntax: "<world> [--skip-folder-check]", Description: "mv-core.load.description", MaxArgs: 1,
		Flags:    []*core.Flag{skipFolder},
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("unloaded") },
		Run: func(ctx *core.Ctx) error {
			w, _, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			ctx.Info(core.M("mv-core.load.loading", "{world}", w.Name()))
			if _, err := ctx.Core.Worlds.LoadChecked(w, ctx.Has("--skip-folder-check")); err != nil {
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			ctx.Info(core.M("mv-core.load.success", "{world}", w.Name()))
			return nil
		},
	}, {
		Name: "unload", Legacy: []string{"mvunload"}, Permission: "multiverse.core.unload",
		Syntax:      "<world> [--remove-players [destination]] [--no-unload-bukkit-world] [--no-save]",
		Description: "mv-core.unload.description", MaxArgs: 1,
		Flags:    []*core.Flag{removePlayers, noUnloadServer, noSave},
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("loaded") },
		Run: func(ctx *core.Ctx) error {
			w, _, err := ctx.WorldArg(0, true)
			if err != nil {
				return err
			}
			dest, err := removeDestination(ctx)
			if err != nil {
				return err
			}
			ctx.Info(core.M("mv-core.unload.unloading", "{world}", w.AliasOrName()))
			if !removePlayersFrom(ctx, w, dest) {
				return nil
			}
			if err := ctx.Core.Worlds.UnloadWith(w, unloadOptions(ctx)); err != nil {
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			ctx.Info(core.M("mv-core.unload.success", "{world}", w.Name()))
			return nil
		},
	}, {
		Name: "remove", Legacy: []string{"mvremove"}, Permission: "multiverse.core.remove",
		Syntax:      "<world> [--remove-players [destination]] [--no-unload-bukkit-world] [--no-save]",
		Description: "mv-core.remove.description", MaxArgs: 1,
		Flags:    []*core.Flag{removePlayers, noUnloadServer, noSave},
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, _, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			dest, err := removeDestination(ctx)
			if err != nil {
				return err
			}
			if !removePlayersFrom(ctx, w, dest) {
				return nil
			}
			if err := ctx.Core.Worlds.Remove(w, unloadOptions(ctx)); err != nil {
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			ctx.Info(core.M("mv-core.remove.success", "{world}", w.Name()))
			return nil
		},
	}, {
		Name: "delete", Legacy: []string{"mvdelete"}, Permission: "multiverse.core.delete",
		Syntax: "<world> [--remove-players [destination]]", Description: "mv-core.delete.description", MaxArgs: 1,
		Flags:    []*core.Flag{removePlayers},
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, _, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			dest, err := removeDestination(ctx)
			if err != nil {
				return err
			}
			ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-core.delete.prompt", "{world}", w.Name()), func() {
				ctx.Info(core.M("mv-core.delete.deleting", "{world}", w.Name()))
				if w.Loaded() && !removePlayersFrom(ctx, w, dest) {
					return
				}
				// MV: WorldTickDeferrer: never delete a world while it ticks.
				ctx.Core.Server.Scheduler().RunLater(1, func() {
					if err := ctx.Core.Worlds.Delete(w, nil); err != nil {
						ctx.Error(core.ErrorMessage(err))
						return
					}
					ctx.Info(core.M("mv-core.delete.success", "{world}", w.Name()))
				})
			})
			return nil
		},
	}, {
		Name: "clone", Legacy: []string{"mvcl", "mvclone"}, Permission: "multiverse.core.clone",
		Syntax:      "<world> <new-world-name> [--reset-world-config --reset-gamerules --reset-world-border --no-save]",
		Description: "mv-core.clone.description", MinArgs: 1, MaxArgs: 2,
		Flags: []*core.Flag{resetConfig, resetRules, resetBorder, {Name: "--no-save", Aliases: []string{"-s"}}},
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return ctx.Core.WorldNames("loaded")
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			w, consumed, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			newName := ctx.Arg(1)
			if !consumed {
				newName = ctx.Arg(0)
			}
			if newName == "" {
				return core.ErrUsage
			}
			ctx.Info(core.M("mv-core.clone.cloning", "{world}", w.Name(), "{newworld}", newName))
			nw, err := ctx.Core.Worlds.Clone(core.CloneOptions{From: w, NewName: newName,
				ResetConfig: ctx.Has("--reset-world-config"), ResetGameRules: ctx.Has("--reset-gamerules"),
				ResetBorder: ctx.Has("--reset-world-border"), NoSave: ctx.Has("--no-save")})
			if err != nil {
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			ctx.Info(core.M("mv-core.clone.success", "{world}", nw.Name()))
			return nil
		},
	}, {
		Name: "regen", Legacy: []string{"mvregen"}, Permission: "multiverse.core.regen",
		Syntax: "<world> [--seed [seed]] [--reset-world-config] [--reset-gamerules] [--reset-world-border] " +
			"[--keep-files <files>] [--remove-players [destination]]",
		Description: "mv-core.regen.description", MaxArgs: 1,
		Flags: []*core.Flag{seed(true), resetConfig, resetRules, resetBorder,
			{Name: "--keep-files", Aliases: []string{"-f"}, Value: true,
				Complete: func(*core.Ctx, string) []string { return []string{"paper-world.yml"} }},
			removePlayers},
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("loaded") },
		Run:      runRegen,
	}}
}

// runCreate is /mv create. MV: CreateCommand.onCreateCommand
func runCreate(ctx *core.Ctx) error {
	name := ctx.Arg(0)
	env, err := environmentArg(ctx.Arg(1))
	if err != nil {
		return err
	}
	worldType := strings.ToLower(ctx.FlagOr("--world-type", "normal"))
	if !slices.Contains(core.WorldTypes, worldType) {
		return core.InvalidArg(core.Lit("Please specify one of (" + strings.Join(core.WorldTypes, ", ") + ")."))
	}
	var spawnAt *core.SpawnLocation
	if v, ok := ctx.Flag("--force-spawn-position"); ok {
		if spawnAt, err = parsePosition(v); err != nil {
			return err
		}
	}
	properties := map[string]string{}
	for kv := range strings.SplitSeq(ctx.FlagOr("--properties", ""), ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			properties[k] = v
		}
	}
	ctx.Info(core.M("mv-core.create.properties", "{worldName}", name))
	ctx.Info(core.M("mv-core.create.properties.environment", "{environment}", strings.ToUpper(env)))
	ctx.Info(core.M("mv-core.create.properties.seed", "{seed}", ctx.FlagOr("--seed", "RANDOM")))
	ctx.Info(core.M("mv-core.create.properties.worldtype", "{worldType}", strings.ToUpper(worldType)))
	ctx.Info(core.M("mv-core.create.properties.adjustspawn", "{adjustSpawn}", strconv.FormatBool(!ctx.Has("--no-adjust-spawn"))))
	for _, f := range []struct{ flag, key, ph string }{
		{"--biome", "biome", "{biome}"},
		{"--generator", "generator", "{generator}"},
		{"--generator-settings", "generatorsettings", "{generatorSettings}"},
	} {
		if v, ok := ctx.Flag(f.flag); ok {
			ctx.Info(core.M("mv-core.create.properties."+f.key, f.ph, v))
		}
	}
	ctx.Info(core.M("mv-core.create.properties.structures", "{structures}", strconv.FormatBool(!ctx.Has("--no-structures"))))
	ctx.Info(core.M("mv-core.create.loading"))
	w, err := ctx.Core.Worlds.Create(core.CreateOptions{Name: name, Environment: env,
		Seed: ctx.FlagOr("--seed", ""), Generator: ctx.FlagOr("--generator", ""),
		GeneratorSettings: ctx.FlagOr("--generator-settings", ""), WorldType: worldType,
		Biome: ctx.FlagOr("--biome", ""), NoAdjust: ctx.Has("--no-adjust-spawn"),
		NoStruct: ctx.Has("--no-structures"), BonusChest: ctx.Has("--generate-bonus-chest"),
		SpawnAt: spawnAt, Properties: properties})
	if err != nil {
		ctx.Error(core.ErrorMessage(err))
		return nil
	}
	ctx.Info(core.M("mv-core.create.success", "{world}", w.Name()))
	return nil
}

// runRegen is /mv regen. MV: RegenCommand.onRegenCommand
func runRegen(ctx *core.Ctx) error {
	w, _, err := ctx.WorldArg(0, true)
	if err != nil {
		return err
	}
	dest, err := removeDestination(ctx)
	if err != nil {
		return err
	}
	seed, randomSeed := ctx.Flag("--seed")
	var keep []string
	if v := ctx.FlagOr("--keep-files", ""); v != "" {
		keep = strings.Split(v, ",")
	}
	ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-core.regen.prompt", "{world}", w.Name()), func() {
		ctx.Info(core.M("mv-core.regen.regenerating", "{world}", w.Name()))
		players := w.Players()
		if !removePlayersFrom(ctx, w, dest) {
			return
		}
		ctx.Core.Server.Scheduler().RunLater(1, func() {
			nw, err := ctx.Core.Worlds.Regen(core.RegenOptions{World: w, Seed: seed, RandomSeed: randomSeed,
				ResetConfig: ctx.Has("--reset-world-config"), ResetGameRules: ctx.Has("--reset-gamerules"),
				ResetBorder: ctx.Has("--reset-world-border"), KeepFiles: keep})
			if err != nil {
				ctx.Error(core.ErrorMessage(err))
				return
			}
			ctx.Info(core.M("mv-core.regen.success", "{world}", nw.Name()))
			if dest != nil {
				// MV: PlayerWorldTeleporter.teleportPlayersToWorld
				to := ctx.Core.WorldDestinationOf(nw)
				for _, p := range players {
					if loc, ok := to.Location(p); ok {
						p.Teleport(loc, plugin.TeleportPlugin)
					}
				}
			}
		})
	})
	return nil
}

// environmentArg reads a World.Environment argument, any case.
func environmentArg(s string) (string, error) {
	env := strings.ToLower(s)
	if !slices.Contains(core.Environments, env) {
		return "", core.InvalidArg(core.Lit("Please specify one of (" + strings.Join(core.Environments, ", ") + ")."))
	}
	return env, nil
}

// parsePosition reads "x,y,z[:pitch:yaw]". MV: EntityPosition.fromString
func parsePosition(s string) (*core.SpawnLocation, error) {
	xyz, dir, _ := strings.Cut(s, ":")
	parts := strings.Split(xyz, ",")
	bad := core.InvalidArg(core.Lit("Invalid position: " + s))
	if len(parts) != 3 {
		return nil, bad
	}
	var v [3]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, bad
		}
		v[i] = f
	}
	pos := &core.SpawnLocation{X: v[0], Y: v[1], Z: v[2]}
	if dir != "" {
		ps, ys, ok := strings.Cut(dir, ":")
		pitch, err1 := strconv.ParseFloat(ps, 32)
		yaw, err2 := strconv.ParseFloat(ys, 32)
		if !ok || err1 != nil || err2 != nil {
			return nil, bad
		}
		pos.Pitch, pos.Yaw = float32(pitch), float32(yaw)
	}
	return pos, nil
}

// removeDestination reads --remove-players: nil when absent, the default
// world's spawn when given without a destination. MV:
// RemovePlayerDestinationFlags
func removeDestination(ctx *core.Ctx) (core.Destination, error) {
	v, ok := ctx.Flag("--remove-players")
	if !ok {
		return nil, nil
	}
	if v == "" {
		w, ok := ctx.Core.Worlds.Default()
		if !ok {
			return nil, core.InvalidArg(core.M("mv-core.commands.error.removeplayers.nodefault"))
		}
		return ctx.Core.WorldDestinationOf(w), nil
	}
	d, err := ctx.Core.Destinations.Parse(ctx.Core.Server.Console(), v)
	if err != nil {
		return nil, core.InvalidArg(core.ErrorMessage(err))
	}
	return d, nil
}

// removePlayersFrom sends w's players to dest (when set), reporting a
// failure; false when one could not be moved.
func removePlayersFrom(ctx *core.Ctx, w *core.World, dest core.Destination) bool {
	if dest == nil {
		return true
	}
	if err := ctx.Core.Worlds.RemovePlayers(w, dest); err != nil {
		ctx.Error(core.ErrorMessage(err))
		return false
	}
	return true
}

func unloadOptions(ctx *core.Ctx) core.UnloadOptions {
	return core.UnloadOptions{KeepServerWorld: ctx.Has("--no-unload-bukkit-world"), NoSave: ctx.Has("--no-save")}
}
