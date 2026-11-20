package commands

import (
	"slices"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

// optionalWorld resolves a "[world]" argument at i that nothing follows
// (Multiverse's maxArgForAware=0): given, it must name a world; left out,
// it is the player's.
func optionalWorld(ctx *core.Ctx, i int, loaded bool) (*core.World, error) {
	if ctx.Arg(i) != "" {
		return ctx.RequireWorld(i, loaded)
	}
	w, _, err := ctx.WorldArg(i, loaded)
	return w, err
}

// worldsArg resolves a "[worlds|*]" argument at i: loaded worlds separated
// by commas, "*" for every loaded world, and the player's world when left
// out. MV: MVCommandContexts' LoadedMultiverseWorld[] resolver
func worldsArg(ctx *core.Ctx, i int) ([]*core.World, error) {
	arg := ctx.Arg(i)
	if arg == "" {
		w, _, err := ctx.WorldArg(i, true)
		if err != nil {
			return nil, err
		}
		return []*core.World{w}, nil
	}
	var out []*core.World
	for name := range strings.SplitSeq(arg, ",") {
		if name == "*" {
			return ctx.Core.Worlds.Loaded(), nil
		}
		w, ok := ctx.Core.Worlds.GetLoaded(name)
		if !ok {
			return nil, core.InvalidArg(core.M("mv-core.commands.error.loadedmultiverseworld.input", "{world}", name))
		}
		if !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out, nil
}

// pickOne matches s against options as ACF's enum resolver does (case and
// underscores ignored), failing with ACF's "Please specify one of".
func pickOne(s string, options []string) (string, error) {
	simple := func(x string) string { return strings.ToLower(strings.ReplaceAll(x, "_", "")) }
	for _, o := range options {
		if simple(o) == simple(s) {
			return o, nil
		}
	}
	return "", core.InvalidArg(core.Lit("Please specify one of (" + strings.Join(options, ", ") + ")."))
}
