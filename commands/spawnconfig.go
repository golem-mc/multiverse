package commands

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, spawnConfigCommands) }

// modifyActions are the actions of a property change. MV: PropertyModifyAction
var modifyActions = []string{"set", "add", "remove", "reset"}

// spawnConfigCommands are /mv entity-spawn-config info and modify. golem
// cannot apply a tick rate or spawn limit to a world yet
// (golem-mc/golem#70), so those two are kept but have no effect. MV:
// EntitySpawnConfigCommand
func spawnConfigCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "entity-spawn-config info", Permission: "multiverse.core.entityspawnconfig.info",
		Syntax: "[world] [--page <page>] [--filter <filter>]", MaxArgs: 1, Flags: core.PageFilterFlags(),
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, err := optionalWorld(ctx, 0, false)
			if err != nil {
				return err
			}
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.entityspawnconfig.info.header", "{world}", w.Name())
			d.Header = &h
			var lines []string
			for _, cat := range core.SpawnCategories {
				v := w.Spawning().Category(cat)
				lines = append(lines,
					ctx.Format(core.M("mv-core.entityspawnconfig.info.category", "{category}", strings.ToUpper(cat))),
					ctx.Format(core.M("mv-core.entityspawnconfig.info.spawn", "{spawn}", v.Bool("spawn"))),
					ctx.Format(core.M("mv-core.entityspawnconfig.info.tickrate", "{tickRate}", v.Custom("tick-rate"))),
					ctx.Format(core.M("mv-core.entityspawnconfig.info.exceptions", "{exceptions}", strings.Join(v.List("exceptions"), ", "))))
			}
			d.Send(ctx.Issuer, lines)
			return nil
		},
	}, {
		Name: "entity-spawn-config modify", Permission: "multiverse.core.entityspawnconfig.modify",
		Syntax: "[world] <spawn-category> <set|add|reset|remove> <property> [value]", MinArgs: 3, MaxArgs: 5,
		Complete: func(ctx *core.Ctx, pos int) []string {
			_, consumed, _ := ctx.WorldArg(0, false)
			if consumed {
				pos--
			}
			switch pos {
			case 0:
				if consumed {
					return core.SpawnCategories
				}
				return append(ctx.Core.WorldNames("both"), core.SpawnCategories...)
			case 1:
				return modifyActions
			case 2:
				return core.NewSpawning().Category("monster").Nodes().Names()
			case 3:
				args := ctx.Args
				if consumed {
					args = args[1:]
				}
				if len(args) < 3 {
					return nil
				}
				if n, ok := core.NewSpawning().Category("monster").Nodes().Find(args[2]); ok {
					if n.Path == "exceptions" {
						cat, _ := pickOne(args[0], core.SpawnCategories)
						var out []string
						for _, t := range core.SpawnCategoryTypes(cat) {
							out = append(out, strings.ToUpper(t))
						}
						return out
					}
					return core.SuggestValues(n, "")
				}
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			w, consumed, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			args := ctx.Args
			if consumed {
				args = args[1:]
			}
			if len(args) < 3 {
				return core.ErrUsage
			}
			cat, err := pickOne(args[0], core.SpawnCategories)
			if err != nil {
				return err
			}
			action, err := pickOne(args[1], modifyActions)
			if err != nil {
				return err
			}
			property := args[2]
			value, hasValue := "", len(args) > 3
			shown := "null"
			if hasValue {
				value, shown = args[3], args[3]
			}
			fail := func(err error) error {
				ctx.Error(core.M("mv-core.entityspawnconfig.modify.failure", "{property}", property, "{value}", shown,
					"{category}", strings.ToUpper(cat), "{world}", w.Name(), "{error}", core.ErrorMessage(err)))
				return nil
			}
			if err := modifySpawnCategory(ctx.Core, w.Spawning().Category(cat), action, property, value, hasValue); err != nil {
				return fail(err)
			}
			if err := ctx.Core.Worlds.Save(); err != nil {
				return fail(err)
			}
			ctx.Info(core.M("mv-core.entityspawnconfig.modify.success", "{property}", property, "{value}", shown,
				"{category}", strings.ToUpper(cat), "{world}", w.Name()))
			warnSpawnApply(ctx, property)
			return nil
		},
	}}
}

// modifySpawnCategory changes one setting of a spawn category. MV:
// StringPropertyHandle.modifyPropertyString
func modifySpawnCategory(c *core.Core, v *core.Values, action, property, value string, hasValue bool) error {
	if action != "reset" && !hasValue {
		return core.MsgError(core.Lit("Value is required for PropertyModifyAction: " + strings.ToUpper(action)))
	}
	n, ok := v.Nodes().Find(property)
	if !ok {
		return core.MsgError(core.M("mv-core.config.node.notfound", "{node}", property))
	}
	if n.Path == "exceptions" && action != "reset" {
		t, err := entityTypeName(c, value)
		if err != nil {
			return err
		}
		value = t
	}
	switch action {
	case "set":
		parsed, err := core.ParseString(n, value)
		if err != nil {
			return err
		}
		return v.Set(n, parsed)
	case "add", "remove":
		if n.Kind != core.KindList {
			return core.MsgError(core.Lit("Property " + property + " is not a list."))
		}
		if action == "add" {
			return v.Add(n, value)
		}
		return v.Remove(n, value)
	}
	return v.Reset(n)
}

// entityTypeName reads an entity type as Bukkit's EntityType enum names
// it (ZOMBIE), the form worlds.yml keeps exceptions in.
func entityTypeName(c *core.Core, s string) (string, error) {
	path := strings.ToLower(strings.TrimPrefix(strings.ToLower(s), "minecraft:"))
	if _, ok := c.Server.Registries().EntityType(plugin.MinecraftKey(path)); !ok {
		return "", core.MsgError(core.Lit("No enum constant org.bukkit.entity.EntityType." + strings.ToUpper(s)))
	}
	return strings.ToUpper(path), nil
}

// warnSpawnApply sends the notes Multiverse gives when a tick rate or spawn
// limit change will not apply. MV: SpawnCategoryConfig.Nodes onChange
func warnSpawnApply(ctx *core.Ctx, property string) {
	switch property {
	case "tick-rate":
		if !ctx.Core.Config.Bool("world.apply-entity-spawn-rate") {
			ctx.SendRaw("§cWarning: Changing tick rates has no effect because 'apply-entity-spawn-rate' is disabled in the core config.")
		}
	case "spawn-limit":
		if !ctx.Core.Config.Bool("world.apply-entity-spawn-limit") {
			ctx.SendRaw("§cWarning: Changing spawn limits has no effect because 'apply-entity-spawn-limit' is disabled in the core config.")
		}
	}
}
