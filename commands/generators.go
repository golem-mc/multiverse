package commands

import (
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, generatorsCommands) }

// generatorsCommands are /mv generators list and info, over the generators
// registered with core.RegisterGenerator. MV: GeneratorsCommand
func generatorsCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "generators list", Legacy: []string{"mvgenerators", "mvgens"}, Permission: "multiverse.core.generator",
		Syntax: "[--page <page>] [--filter <filter>]", Description: "mv-core.generators.description",
		MaxArgs: 0, Flags: core.PageFilterFlags(),
		Run: func(ctx *core.Ctx) error {
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			generators := core.GeneratorNames()
			if len(generators) == 0 {
				ctx.Send(core.M("mv-core.generators.empty"))
				return nil
			}
			h := core.M("mv-core.generators.header")
			d.Header = &h
			d.Send(ctx.Issuer, generators)
			return nil
		},
	}, {
		Name: "generators info", Permission: "multiverse.core.generator", Syntax: "<generator>",
		MinArgs: 1, MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return core.GeneratorNames() },
		Run: func(ctx *core.Ctx) error {
			var name string
			for _, g := range core.GeneratorNames() {
				if strings.EqualFold(g, ctx.Arg(0)) {
					name = g
				}
			}
			if name == "" {
				return core.InvalidArg(core.Lit("Invalid generator plugin: " + ctx.Arg(0)))
			}
			// A registered generator has no example usages or info link, as
			// Multiverse's SimpleGeneratorPlugin has none.
			ctx.Send(core.M("mv-core.generators.info.plugin", "{plugin}", name))
			ctx.Send(core.M("mv-core.generators.info.exampleusages"))
			ctx.Send(core.M("mv-core.generators.info.usages", "{usages}", ""))
			ctx.Send(core.M("mv-core.generators.info.infolink", "{link}", "null"))
			return nil
		},
	}}
}
