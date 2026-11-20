package commands

import (
	"slices"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, modifyCommands) }

// removedProperties are Multiverse 4 properties /mv modify points elsewhere.
var removedProperties = map[string]string{
	"animals":  "/mv entity-spawn-config modify [world] animal set spawn <true|false>",
	"monsters": "/mv entity-spawn-config modify [world] monster set spawn <true|false>",
}

// modifyCommands is /mv modify. MV: ModifyCommand
func modifyCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "modify", Legacy: []string{"mvmodify", "mvm"},
		Permission: "multiverse.core.modify", Syntax: "[world] <set|add|remove|reset> <property> [value]",
		Description: "mv-core.modify.description", MinArgs: 2, MaxArgs: 4,
		Complete: completeModify,
		Run: func(ctx *core.Ctx) error {
			w, rest, err := modifyWorld(ctx)
			if err != nil {
				return err
			}
			if len(rest) < 2 {
				return core.ErrUsage
			}
			action := strings.ToLower(rest[0])
			if !slices.Contains(core.ModifyActions, action) {
				return core.InvalidArg(core.Lit("Please specify one of (" + strings.Join(core.ModifyActions, ", ") + ")."))
			}
			name := rest[1]
			if repl, ok := removedProperties[name]; ok {
				ctx.Send(core.M("mv-core.modify.propertyremoved", "{property}", name, "{replacement}", repl))
				return nil
			}
			value, hasValue := "", len(rest) > 2
			if hasValue {
				value = rest[2]
			}
			if core.ModifyNeedsValue(action) != hasValue {
				key := "mv-core.modify.specifyvalue"
				if hasValue {
					key = "mv-core.modify.cannothavevalue"
				}
				ctx.Send(core.M(key, "{action}", action, "{property}", name))
				return nil
			}
			if err := w.Modify(action, name, value); err != nil {
				if hasValue {
					ctx.Send(core.M("mv-core.modify.failure", "{action}", action, "{property}", name, "{value}", value,
						"{world}", w.Name(), "{error}", core.ErrorMessage(err)))
				} else {
					ctx.Send(core.M("mv-core.modify.failure.novalue", "{action}", action, "{property}", name,
						"{world}", w.Name(), "{error}", core.ErrorMessage(err)))
				}
				return nil
			}
			ctx.Send(core.M("mv-core.modify.success", "{action}", action, "{property}", name,
				"{value}", core.FormatValue(w.Props.GetPath(name)), "{world}", w.Name()))
			afterModify(ctx, w, name)
			return nil
		},
	}}
}

// modifyWorld resolves the world, which may be left out for the player's
// own unless all four arguments are given, and answers the arguments after
// it. MV: @Flags("resolve=issuerAware,maxArgForAware=3")
func modifyWorld(ctx *core.Ctx) (*core.World, []string, error) {
	if len(ctx.Args) > 3 {
		w, err := ctx.RequireWorld(0, false)
		return w, ctx.Args[1:], err
	}
	w, consumed, err := ctx.WorldArg(0, false)
	if consumed {
		return w, ctx.Args[1:], err
	}
	return w, ctx.Args, err
}

// afterModify sends what a property's change hook tells its sender in
// Multiverse: the alias conflicts, and that keep-spawn-in-memory does
// nothing. MV: WorldConfigNodes alias and keepSpawnInMemory
func afterModify(ctx *core.Ctx, w *core.World, name string) {
	n, ok := w.Props.Nodes().Find(name)
	if !ok {
		return
	}
	switch n.Path {
	case core.PropAlias:
		if ctx.Core.Config.Bool("world.warn-alias-conflicts") {
			ctx.Core.Worlds.SendAliasConflicts(w, ctx.Issuer)
		}
	case core.PropKeepSpawnInMemory:
		if w.Loaded() && w.Props.Bool(core.PropKeepSpawnInMemory) {
			ctx.SendRaw("§cKeep spawn in memory feature has been removed by Minecraft in 1.21.9+ and will no longer have any effect when set to true.")
		}
	}
}

// completeModify completes the world, then the action, the property and
// its value, with the world left out when the first argument is not one.
// MV: @mvworlds|@propsmodifyaction|@mvworldpropsname|@mvworldpropsvalue
func completeModify(ctx *core.Ctx, pos int) []string {
	if pos == 0 {
		return ctx.Core.WorldNames("both")
	}
	w, rest, err := modifyWorld(ctx)
	if err != nil {
		return nil
	}
	switch pos - (len(ctx.Args) - len(rest)) {
	case 0:
		return core.ModifyActions
	case 1:
		return w.ModifiableNames(strings.ToLower(rest[0]))
	case 2:
		return w.SuggestModify(strings.ToLower(rest[0]), rest[1], "")
	}
	return nil
}
