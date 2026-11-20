package netherportals

import (
	"github.com/golem-mc/multiverse/core"
)

// bidirectional is the --bidirectional flag of link and unlink.
// MV: BidirectionalFlag
func bidirectional() []*core.Flag {
	return []*core.Flag{{Name: "--bidirectional", Aliases: []string{"-b"}}}
}

func linkTypeNames() []string { return []string{"nether", "end"} }

// commands are /mvnp's subcommands. MV: LinkCommand, UnlinkCommand,
// ListCommand, MVNPCommandCompletions
func (np *NetherPortals) commands() []*core.Cmd {
	return []*core.Cmd{{
		Name: "link", Legacy: []string{"mvnplink", "mvnpl"},
		Permission: "multiverse.netherportals.link", Syntax: "<nether|end> [fromWorld] <toWorld>",
		Description: "mv-netherportals.link.description", MinArgs: 2, MaxArgs: 3, Flags: bidirectional(),
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return linkTypeNames()
			}
			return ctx.Core.WorldNames("both")
		},
		Run: np.runLink,
	}, {
		Name: "unlink", Legacy: []string{"mvnpunlink", "mvnpu"},
		Permission: "multiverse.netherportals.unlink", Syntax: "<nether|end> [fromWorld]",
		Description: "mv-netherportals.unlink.description", MinArgs: 2, MaxArgs: 2, Flags: bidirectional(),
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return linkTypeNames()
			}
			t, err := ParseLinkType(ctx.Arg(0))
			if err != nil {
				return nil
			}
			var out []string
			for _, l := range np.Links.All() {
				if _, ok := l.To(t); ok {
					out = append(out, l.From)
				}
			}
			return out
		},
		Run: np.runUnlink,
	}, {
		Name: "list", Legacy: []string{"mvnplist", "mvnpli"},
		Permission: "multiverse.netherportals.show", Syntax: "[nether|end]",
		Description: "mv-netherportals.list.description", MaxArgs: 1,
		Complete: func(_ *core.Ctx, pos int) []string {
			if pos == 0 {
				return linkTypeNames()
			}
			return nil
		},
		Run: np.runList,
	}}
}

// runLink links a world's portals to another world, or to itself, which
// disables them. MV: LinkCommand
func (np *NetherPortals) runLink(ctx *core.Ctx) error {
	t, err := ParseLinkType(ctx.Arg(0))
	if err != nil {
		return err
	}
	from, consumed, err := ctx.WorldArg(1, false)
	if err != nil {
		return err
	}
	i := 1
	if consumed {
		i = 2
	}
	if ctx.Arg(i) == "" {
		return core.ErrUsage
	}
	to, err := ctx.RequireWorld(i, false)
	if err != nil {
		return err
	}
	both := ctx.Has("--bidirectional")
	np.Links.Add(from.Name(), to.Name(), t)
	if both {
		np.Links.Add(to.Name(), from.Name(), t)
	}
	if err := np.Links.Save(); err != nil {
		ctx.Error(core.M("mv-netherportals.link.failed", "{error}", err))
		return core.ErrHandled
	}
	if from.Name() == to.Name() {
		ctx.Send(core.M("mv-netherportals.link.disabled", "{linkType}", t.Message(), "{world}", to.AliasOrName()))
		return nil
	}
	ctx.Send(core.M("mv-netherportals.link.success", "{linkType}", t.Message(),
		"{fromWorld}", from.AliasOrName(), "{toWorld}", to.AliasOrName()))
	if both {
		ctx.Send(core.M("mv-netherportals.link.success", "{linkType}", t.Message(),
			"{fromWorld}", to.AliasOrName(), "{toWorld}", from.AliasOrName()))
	}
	return nil
}

// runUnlink removes a world's link; the world may be gone already.
// MV: UnlinkCommand
func (np *NetherPortals) runUnlink(ctx *core.Ctx) error {
	t, err := ParseLinkType(ctx.Arg(0))
	if err != nil {
		return err
	}
	from := ctx.Arg(1)
	w, ok := ctx.Core.Worlds.Get(from)
	if ctx.Core.Config.Bool("command.resolve-alias-name") {
		w, ok = ctx.Core.Worlds.GetByNameOrAlias(from)
	}
	if ok {
		from = w.Name()
	}
	to, _ := np.Links.To(from, t)
	np.unlink(ctx, t, from, to)
	if !ctx.Has("--bidirectional") || to == "" {
		return nil
	}
	back, ok := np.Links.To(to, t)
	switch {
	case !ok:
		ctx.Send(core.M("mv-netherportals.unlink.notlinked", "{world}", to, "{linkType}", t.Message()))
	case back != from:
		ctx.Send(core.M("mv-netherportals.unlink.bidirectional.mismatch", "{linkType}", t.Message(),
			"{fromWorld}", from, "{toWorld}", to, "{linkedWorld}", back))
	default:
		np.unlink(ctx, t, to, from)
	}
	return nil
}

func (np *NetherPortals) unlink(ctx *core.Ctx, t LinkType, from, to string) {
	if !np.Links.Remove(from, t) {
		ctx.Send(core.M("mv-netherportals.unlink.notlinked", "{world}", from, "{linkType}", t.Message()))
		return
	}
	if err := np.Links.Save(); err != nil {
		ctx.Error(core.M("mv-netherportals.unlink.failed", "{error}", err))
		return
	}
	if from == to {
		ctx.Send(core.M("mv-netherportals.unlink.enabled", "{linkType}", t.Message(), "{world}", from))
		return
	}
	ctx.Send(core.M("mv-netherportals.unlink.success", "{linkType}", t.Message(), "{fromWorld}", from, "{toWorld}", to))
}

// runList shows the links, and the auto-links between worlds that exist.
// MV: ListCommand
func (np *NetherPortals) runList(ctx *core.Ctx) error {
	types := LinkTypes
	d := core.NewDisplay()
	d.Paginate = false
	header, empty := core.M("mv-netherportals.list.header.all"), core.M("mv-netherportals.list.nocontent.all")
	if ctx.Arg(0) != "" {
		t, err := ParseLinkType(ctx.Arg(0))
		if err != nil {
			return err
		}
		types = []LinkType{t}
		header = core.M("mv-netherportals.list.header", "{linkType}", t.Message())
		empty = core.M("mv-netherportals.list.nocontent", "{linkType}", t.Message())
	}
	d.Header, d.NoContent = &header, &empty
	var lines []string
	for _, t := range types {
		for _, row := range np.linkRows(t) {
			lines = append(lines, ctx.Format(row.message()))
		}
	}
	d.Send(ctx.Issuer, lines)
	return nil
}

// linkRow is one line of /mvnp list. MV: ListCommand.LinkRow
type linkRow struct {
	t            LinkType
	from, to     string
	twoWay, auto bool
}

// linkRows answers t's links in world name order, a link and its way back
// as one two-way row, then the auto-links of the worlds without one.
func (np *NetherPortals) linkRows(t LinkType) []*linkRow {
	links := np.Links.Map(t)
	var rows []*linkRow
	byFrom := map[string]*linkRow{}
	for _, l := range np.Links.All() {
		to, ok := l.To(t)
		if !ok {
			continue
		}
		if r := byFrom[to]; r != nil && r.to == l.From {
			r.twoWay = true
			continue
		}
		r := &linkRow{t: t, from: l.From, to: to}
		byFrom[l.From] = r
		rows = append(rows, r)
	}
	for _, w := range np.core.Worlds.All() {
		if _, ok := links[w.Name()]; ok {
			continue
		}
		to := np.AutoLink(w.Name(), t)
		if _, ok := np.core.Worlds.Get(to); !ok {
			continue
		}
		if r := byFrom[to]; r != nil && r.auto {
			r.twoWay = true
			continue
		}
		r := &linkRow{t: t, from: w.Name(), to: to, auto: true}
		byFrom[w.Name()] = r
		rows = append(rows, r)
	}
	return rows
}

func (r *linkRow) message() core.Message {
	tag := core.Lit("&4[&cnether&4]")
	if r.t == End {
		tag = core.Lit("&3[&bend&3]")
	}
	auto := ""
	if r.auto {
		auto = " &6&o(auto)"
	}
	switch {
	case r.from == r.to:
		return core.Lit("{linkType} &f{fromWorld} &7&l-- &cDISABLED", "{fromWorld}", r.from, "{linkType}", tag)
	case r.twoWay:
		return core.Lit("{linkType} &f{fromWorld} &7&l<--->&r &f{toWorld}{auto}",
			"{fromWorld}", r.from, "{linkType}", tag, "{toWorld}", r.to, "{auto}", auto)
	}
	return core.Lit("{linkType} &f{fromWorld} &7&l-->&r &f{toWorld}{auto}",
		"{fromWorld}", r.from, "{linkType}", tag, "{toWorld}", r.to, "{auto}", auto)
}
