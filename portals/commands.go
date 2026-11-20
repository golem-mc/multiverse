package portals

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// permSelect is /mvp select's permission: ACF requires both of
// multiverse.portal.select and multiverse.portal.create.
const permSelect = "@mvportalselect"

// commands are /mvp's subcommands. MV: commands/*Command.java
func (p *Portals) commands() []*core.Cmd {
	return []*core.Cmd{
		p.configCmd(), p.createCmd(), p.debugCmd(), p.infoCmd(), p.listCmd(),
		p.modifyCmd(), p.removeCmd(), p.selectCmd(), p.wandCmd(),
	}
}

// portalArgMode is how a portal argument falls back to the selected
// portal. MV: PortalsCommandContexts' "resolve" flag
type portalArgMode uint8

const (
	// portalByName takes the argument only.
	portalByName portalArgMode = iota
	// portalIssuerAware takes the argument when it names a portal the
	// sender may access, else the player's selected portal.
	portalIssuerAware
)

// portalArg resolves a portal argument at i; consumed reports whether the
// argument was the portal. With optional, nothing found is no portal and
// no error. MV: PortalsCommandContexts.parseMVPortal
func (p *Portals) portalArg(ctx *core.Ctx, i int, mode portalArgMode, optional bool) (pt *Portal, consumed bool, err error) {
	name := ctx.Arg(i)
	if found, ok := p.Manager.GetFor(name, ctx.Sender); ok && name != "" {
		return found, true, nil
	}
	if mode == portalIssuerAware {
		if pl, ok := ctx.Player(); ok {
			if sel := p.Session(pl).Selected(); sel != nil {
				return sel, false, nil
			}
		}
	}
	if optional {
		return nil, false, nil
	}
	return nil, false, core.InvalidArg(core.M("mv-portals.context.portal.notfound", "{name}", name))
}

// completePortals completes the portals the sender may see. MV: @mvportals
func (p *Portals) completePortals(ctx *core.Ctx) []string {
	var out []string
	for _, pt := range p.Manager.For(ctx.Sender) {
		out = append(out, pt.Name())
	}
	return out
}

func (p *Portals) configCmd() *core.Cmd {
	return &core.Cmd{
		Name: "config", Aliases: []string{"conf"}, Legacy: []string{"mvpconfig", "mvpconf"},
		Permission: "multiverse.portal.config", Syntax: "<property> [value]",
		Description: "mv-portals.config.description", MinArgs: 1, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			nodes := p.Config.Nodes()
			if pos == 0 {
				return nodes.Names()
			}
			if n, ok := nodes.Find(ctx.Arg(0)); ok {
				return core.SuggestValues(n, "")
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			name := ctx.Arg(0)
			n, ok := p.Config.Nodes().Find(name)
			if len(ctx.Args) < 2 {
				if !ok {
					ctx.Error(core.M("mv-core.config.show.error", "{name}", name,
						"{error}", core.M("mv-core.config.node.notfound", "{node}", name)))
					return nil
				}
				ctx.Info(core.M("mv-core.config.show.success", "{name}", name, "{value}", core.FormatValue(p.Config.Get(n))))
				return nil
			}
			value := ctx.Arg(1)
			err := core.MsgError(core.M("mv-core.config.node.notfound", "{node}", name))
			if ok {
				err = p.setConfig(n, value)
			}
			if err != nil {
				ctx.Error(core.M("mv-core.config.set.error", "{name}", name, "{value}", value, "{error}", core.ErrorMessage(err)))
				return nil
			}
			ctx.Info(core.M("mv-core.config.set.success", "{name}", name, "{value}", value))
			return nil
		},
	}
}

// setConfig parses and sets a config.yml option and saves the file.
func (p *Portals) setConfig(n *core.Node, value string) error {
	v, err := core.ParseString(n, value)
	if err != nil {
		return err
	}
	if err := p.Config.Set(n, v); err != nil {
		return err
	}
	return p.Config.Save()
}

func (p *Portals) createCmd() *core.Cmd {
	return &core.Cmd{
		Name: "create", Legacy: []string{"mvpcreate", "mvpc"},
		Permission: "multiverse.portal.create", Syntax: "<portal-name> [destination]",
		Description: "mv-portals.create.description", MinArgs: 1, MaxArgs: 2, PlayerOnly: true,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 1 {
				return p.Core.Destinations.Suggest(ctx.Sender, ctx.Arg(1))
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			pl, _ := ctx.Player()
			world, _, err := ctx.WorldArg(len(ctx.Args), true)
			if err != nil {
				return err
			}
			name := ctx.Arg(0)
			var dest core.Destination
			if ctx.Arg(1) != "" {
				if dest, err = p.Core.Destinations.Parse(ctx.Sender, ctx.Arg(1)); err != nil {
					return core.InvalidArg(core.ErrorMessage(err))
				}
			}
			s := p.Session(pl)
			r, ok := s.SelectedRegion()
			if !ok {
				return nil
			}
			if !NamePattern.MatchString(name) {
				ctx.Error(core.M("mv-portals.create.invalidname"))
				return nil
			}
			// MV: CreateCommand puts the portal in the player's world.
			r.World = world
			pt, created := p.Manager.Create(name, pl.Name(), r)
			if created {
				ctx.Info(core.M("mv-portals.create.success", "{name}", name))
			} else {
				ctx.Error(core.M("mv-portals.create.alreadyexists", "{name}", name))
				pt, _ = p.Manager.Get(name)
			}
			s.Select(pt)
			if pt == nil {
				return nil
			}
			if dest != nil {
				pt.set(propAction, core.DestinationString(dest))
				p.Manager.save()
			} else {
				ctx.Error(core.M("mv-portals.create.actionnotset"))
			}
			return nil
		},
	}
}

func (p *Portals) debugCmd() *core.Cmd {
	return &core.Cmd{
		Name: "debug", Legacy: []string{"mvpdebug", "mvpd"},
		Permission: "multiverse.portal.debug", Syntax: "[on|off]",
		Description: "mv-portals.debug.description", MaxArgs: 1, PlayerOnly: true,
		Complete: func(*core.Ctx, int) []string { return []string{"on", "off"} },
		Run: func(ctx *core.Ctx) error {
			pl, _ := ctx.Player()
			s := p.Session(pl)
			if t := ctx.Arg(0); t != "" {
				s.SetDebug(strings.EqualFold(t, "on"))
			} else {
				s.SetDebug(!s.Debug())
			}
			return nil
		},
	}
}

func (p *Portals) infoCmd() *core.Cmd {
	return &core.Cmd{
		Name: "info", Legacy: []string{"mvpinfo", "mvpi"},
		Permission: "multiverse.portal.info", Syntax: "[portal]",
		Description: "mv-portals.info.description", MaxArgs: 1,
		Complete: func(ctx *core.Ctx, _ int) []string { return p.completePortals(ctx) },
		Run: func(ctx *core.Ctx) error {
			pt, _, err := p.portalArg(ctx, 0, portalIssuerAware, false)
			if err != nil {
				return err
			}
			if pl, ok := ctx.Player(); ok {
				p.Session(pl).ShowInfo(pt)
				return nil
			}
			p.showInfo(ctx.Sender, pt, core.M("mv-portals.portalinfo.commandheader", "{name}", pt.Name()))
			return nil
		},
	}
}

// listPerPage is /mvp list's page size. MV: ListCommand.ITEMS_PER_PAGE
const listPerPage = 9

func (p *Portals) listCmd() *core.Cmd {
	return &core.Cmd{
		Name: "list", Legacy: []string{"mvplist", "mvpl"},
		Permission: "multiverse.portal.list", Syntax: "[filter/world] [page]",
		Description: "mv-portals.list.description", MaxArgs: 2,
		Run: func(ctx *core.Ctx) error {
			page := 1
			if s := ctx.Arg(1); s != "" {
				n, err := strconv.Atoi(s)
				if err != nil {
					return core.InvalidArg(core.Lit("Please specify a number."))
				}
				page = n
			}
			filter := ctx.Arg(0)
			world, isWorld := p.Core.Worlds.GetLoaded(filter)
			if isWorld {
				filter = ""
			}
			lines := p.listLines(ctx, world, filter)
			total := int(math.Ceil(float64(len(lines)) / listPerPage))
			if (page < 1 || page > total) && filter == "" {
				page = total
			}
			var worldMsg, filterMsg any = "", ""
			if isWorld {
				worldMsg = core.M("mv-portals.list.header.world", "{world}", world.AliasOrName())
			}
			if filter != "" {
				filterMsg = core.M("mv-portals.list.header.filter", "{filter}", filter)
			}
			ctx.Info(core.M("mv-portals.list.header", "{count}", len(lines), "{world}", worldMsg,
				"{filter}", filterMsg, "{page}", page, "{totalPages}", total))
			for i, l := range lines {
				if i >= (page-1)*listPerPage && i < page*listPerPage {
					ctx.SendRaw("§9" + l)
				}
			}
			return nil
		},
	}
}

// listLines answers /mvp list's lines for the portals the sender may see,
// in world when it is not nil, matching filter by name or destination.
// MV: ListCommand.getPortals
func (p *Portals) listLines(ctx *core.Ctx, world *core.World, filter string) []string {
	var out []string
	for _, pt := range p.Manager.For(ctx.Sender) {
		if w, ok := pt.World(); world != nil && (!ok || w.Name() != world.Name()) {
			continue
		}
		dest := ctx.Format(p.destinationMessage(pt))
		f := strings.ToLower(filter)
		if strings.Contains(strings.ToLower(pt.Name()), f) || strings.Contains(strings.ToLower(dest), f) {
			out = append(out, ctx.Format(core.M("mv-portals.list.entry", "{name}", pt.Name(), "{destination}", dest)))
		}
	}
	slices.Sort(out)
	return out
}

func (p *Portals) modifyCmd() *core.Cmd {
	return &core.Cmd{
		Name: "modify", Legacy: []string{"mvpmodify", "mvpm"},
		Permission: "multiverse.portal.modify", Syntax: "[portal] <property> <value>",
		Description: "mv-portals.modify.description", MinArgs: 2, MaxArgs: -1,
		Complete: p.completeModify,
		Run: func(ctx *core.Ctx) error {
			pt, consumed, err := p.portalArg(ctx, 0, portalIssuerAware, false)
			if err != nil {
				return err
			}
			rest := ctx.Args
			if consumed {
				rest = rest[1:]
			}
			if len(rest) < 2 {
				return core.ErrUsage
			}
			property, value := rest[0], strings.Join(rest[1:], " ")
			if (strings.EqualFold(property, "dest") || strings.EqualFold(property, "destination")) &&
				strings.EqualFold(value, "here") {
				if _, isWorld := p.Core.Worlds.Get("here"); !isWorld {
					p.Log.Warn("Using 'here' as a destination is deprecated and will be removed in a future version. Use 'e:@here' instead.")
					ctx.Error(core.M("mv-portals.modify.heredeprecated"))
					value = "e:@here"
				}
			}
			if err := pt.SetProperty(ctx.Sender, property, value); err != nil {
				ctx.Error(core.M("mv-portals.modify.failure", "{name}", property, "{portal}", pt.Name(), "{value}", value))
				ctx.Error(core.ErrorMessage(err))
				return nil
			}
			if !p.Manager.save() {
				ctx.Error(core.M("mv-portals.modify.savefailed"))
				return nil
			}
			n, _ := p.nodes.Find(property)
			ctx.Info(core.M("mv-portals.modify.success", "{name}", property, "{portal}", pt.Name(),
				"{value}", core.FormatValue(pt.Props.Get(n))))
			if strings.EqualFold(property, "action-type") {
				ctx.Error(core.M("mv-portals.modify.actiontypewarning"))
			}
			return nil
		},
	}
}

// completeModify completes /mvp modify: portals, then property names,
// then values for the property. MV: @mvportals @portalpropertynames
// @portalpropertyvalues
func (p *Portals) completeModify(ctx *core.Ctx, pos int) []string {
	switch pos {
	case 0:
		return p.completePortals(ctx)
	case 1:
		return p.nodes.Names()
	case 2:
		pt, _, _ := p.portalArg(ctx, 0, portalIssuerAware, true)
		n, ok := p.nodes.Find(ctx.Arg(1))
		if pt == nil || !ok {
			return nil
		}
		switch n.Path {
		case propAction:
			if t, err := p.Actions.Type(pt.ActionType()); err == nil {
				return t.Suggest(ctx.Sender, "")
			}
			return nil
		case propLocation:
			if pl, ok := ctx.Player(); ok && p.Session(pl).hasSelection() {
				return []string{"@selected-region"}
			}
			return nil
		}
		return core.SuggestValues(n, "")
	}
	return nil
}

func (p *Portals) removeCmd() *core.Cmd {
	return &core.Cmd{
		Name: "remove", Legacy: []string{"mvpremove", "mvpr"},
		Permission: "multiverse.portal.remove", Syntax: "<portal-name>",
		Description: "mv-portals.remove.description", MinArgs: 1, MaxArgs: 1,
		Complete: func(ctx *core.Ctx, _ int) []string { return p.completePortals(ctx) },
		Run: func(ctx *core.Ctx) error {
			name := ctx.Arg(0)
			pt, ok := p.Manager.Remove(name, true)
			if !ok {
				ctx.Error(core.M("mv-portals.remove.notfound", "{name}", name))
				return nil
			}
			ctx.Info(core.M("mv-portals.remove.success", "{name}", pt.Name()))
			return nil
		},
	}
}

func (p *Portals) selectCmd() *core.Cmd {
	return &core.Cmd{
		Name: "select", Legacy: []string{"mvpselect", "mvps"},
		Permission: permSelect, Syntax: "<portal>",
		Description: "mv-portals.select.description", MaxArgs: 1, PlayerOnly: true,
		Complete: func(ctx *core.Ctx, _ int) []string { return p.completePortals(ctx) },
		Run: func(ctx *core.Ctx) error {
			pl, _ := ctx.Player()
			s := p.Session(pl)
			// MV: the argument is optional, so an unknown name shows the
			// selection as no name does.
			pt, _, _ := p.portalArg(ctx, 0, portalByName, true)
			if pt == nil {
				if s.Selected() == nil {
					ctx.Error(core.M("mv-portals.select.none",
						"{wandMaterial}", materialName(p.Config.String("portal-creation.wand-material"))))
					return nil
				}
				ctx.Info(core.M("mv-portals.select.current", "{name}", s.Selected().Name()))
				return nil
			}
			s.Select(pt)
			ctx.Info(core.M("mv-portals.select.success", "{name}", pt.Name()))
			return nil
		},
	}
}

func (p *Portals) wandCmd() *core.Cmd {
	return &core.Cmd{
		Name: "wand", Legacy: []string{"mvpwand", "mvpw"},
		Permission: "multiverse.portal.givewand", Syntax: "[enable|disable|toggle]",
		Description: "mv-portals.wand.description", MaxArgs: 1, PlayerOnly: true,
		Complete: func(*core.Ctx, int) []string { return []string{"enable", "disable", "toggle"} },
		Run: func(ctx *core.Ctx) error {
			pl, _ := ctx.Player()
			switch ctx.Arg(0) {
			case "":
			case "enable":
				p.wandEnabled = true
				return nil
			case "disable":
				p.wandEnabled = false
				return nil
			case "toggle":
				p.wandEnabled = !p.wandEnabled
				return nil
			default:
				ctx.Error(core.M("mv-portals.wand.invalidaction"))
				return nil
			}
			wand := plugin.ItemStack{Item: p.wandItem(), Count: 1}
			name := materialName(p.Config.String("portal-creation.wand-material"))
			inv := pl.Inventory()
			switch {
			case inv.Get(plugin.SlotMainHand).Empty():
				inv.Set(plugin.SlotMainHand, wand)
				ctx.Info(core.M("mv-portals.wand.given", "{wandMaterial}", name))
			case len(inv.Add(wand)) == 0:
				ctx.Info(core.M("mv-portals.wand.inventory", "{wandMaterial}", name))
			default:
				ctx.Info(core.M("mv-portals.wand.dropped", "{wandMaterial}", name))
				pl.World().DropItem(pl.Location().Pos, wand, true)
			}
			return nil
		},
	}
}
