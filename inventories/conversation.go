package inventories

import (
	"regexp"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// prompt is one question of /mvinv group's conversation: what it asks, and
// what it does with the answer, answering the next prompt or nil to end.
// MV: InventoriesPrompt
type prompt struct {
	text   func() core.Message
	accept func(input string) *prompt
}

// conversations are the players answering /mvinv group's prompts.
// MV: Bukkit's Conversation, non-modal with local echo and "##" to escape
type conversations struct {
	m       *Module
	current map[plugin.UUID]*conversation
}

type conversation struct {
	is     *core.Issuer
	prompt *prompt
}

const conversationEscape = "##"

func newConversations(m *Module) *conversations {
	cs := &conversations{m: m, current: map[plugin.UUID]*conversation{}}
	// The answer is taken before any chat listener sees it, as Bukkit's
	// conversations take a player's chat.
	plugin.Listen(m.Server, func(e *plugin.PlayerChatEvent) {
		c, ok := cs.current[e.Player.UUID()]
		if !ok {
			return
		}
		e.SetCancelled(true)
		c.is.SendRaw(e.Message)
		if e.Message == conversationEscape {
			delete(cs.current, e.Player.UUID())
			return
		}
		c.prompt = c.prompt.accept(e.Message)
		if c.prompt == nil {
			delete(cs.current, e.Player.UUID())
			return
		}
		cs.show(c)
	}, plugin.ListenOptions{Priority: plugin.Lowest})
	plugin.Listen(m.Server, func(e *plugin.PlayerQuitEvent) { delete(cs.current, e.Player.UUID()) })
	return cs
}

func (cs *conversations) show(c *conversation) { c.is.SendRaw(c.is.Format(c.prompt.text())) }

// begin starts the group conversation with a player. MV: GroupCommand
func (cs *conversations) begin(is *core.Issuer, p plugin.Player) {
	c := &conversation{is: is, prompt: cs.m.groupControlPrompt(is)}
	cs.current[p.UUID()] = c
	cs.show(c)
}

// groupList names the groups, white separated by gold commas.
func (m *Module) groupList() string {
	return "§f" + strings.Join(m.groupNames(), "§6, §f")
}

// groupControlPrompt asks whether to create, edit or delete a group.
// MV: GroupControlPrompt
func (m *Module) groupControlPrompt(is *core.Issuer) *prompt {
	p := &prompt{text: func() core.Message { return core.M("mv-inventories.group.commandprompt") }}
	p.accept = func(input string) *prompt {
		switch strings.ToLower(input) {
		case "delete":
			return m.groupDeletePrompt(is)
		case "create":
			return m.groupCreatePrompt(is)
		case "edit":
			return m.groupEditPrompt(is)
		}
		is.Error(core.M("mv-inventories.group.invalidoption"))
		return p
	}
	return p
}

var groupNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_]*$`)

// groupCreatePrompt asks the new group's name, then its worlds and shares.
// MV: GroupCreatePrompt
func (m *Module) groupCreatePrompt(is *core.Issuer) *prompt {
	p := &prompt{text: func() core.Message { return core.M("mv-inventories.group.createprompt") }}
	p.accept = func(input string) *prompt {
		g, ok := m.Groups.NewGroup(input)
		if !ok {
			is.Error(core.M("mv-inventories.group.exists", "{group}", input))
			return nil
		}
		if !groupNamePattern.MatchString(input) {
			is.Error(core.M("mv-inventories.group.invalidname"))
			return p
		}
		return m.groupWorldsPrompt(is, g, m.groupSharesPrompt(is, g, nil, true), true)
	}
	return p
}

// groupDeletePrompt asks which group to delete. MV: GroupDeletePrompt
func (m *Module) groupDeletePrompt(is *core.Issuer) *prompt {
	return &prompt{
		text: func() core.Message { return core.M("mv-inventories.group.deleteprompt", "{groups}", m.groupList()) },
		accept: func(input string) *prompt {
			if g, ok := m.Groups.Get(input); ok {
				m.Groups.Remove(g)
				is.Info(core.M("mv-inventories.group.removed", "{group}", input))
			} else {
				is.Error(core.M("mv-inventories.error.nogroup", "{group}", input))
			}
			return nil
		},
	}
}

// groupEditPrompt asks which group to edit. MV: GroupEditPrompt
func (m *Module) groupEditPrompt(is *core.Issuer) *prompt {
	return &prompt{
		text: func() core.Message { return core.M("mv-inventories.group.editprompt", "{groups}", m.groupList()) },
		accept: func(input string) *prompt {
			if g, ok := m.Groups.Get(input); ok {
				return m.groupModifyPrompt(is, g)
			}
			is.Error(core.M("mv-inventories.error.nogroup", "{group}", input))
			return nil
		},
	}
}

// groupModifyPrompt asks whether to change a group's worlds or shares.
// MV: GroupModifyPrompt
func (m *Module) groupModifyPrompt(is *core.Issuer, g *WorldGroup) *prompt {
	p := &prompt{text: func() core.Message { return core.M("mv-inventories.group.modifyprompt", "{group}", g.name) }}
	p.accept = func(input string) *prompt {
		switch strings.ToLower(input) {
		case "worlds":
			return m.groupWorldsPrompt(is, g, p, false)
		case "shares":
			return m.groupSharesPrompt(is, g, p, false)
		}
		is.Error(core.M("mv-inventories.group.invalidoption"))
		return p
	}
	return p
}

// sendGroupInfo shows a changed group.
func (m *Module) sendGroupInfo(is *core.Issuer, g *WorldGroup) {
	is.Info(core.M("mv-inventories.info.group", "{group}", g.name))
	is.Info(core.M("mv-inventories.info.group.info", "{worlds}", javaList(g.configWorlds)))
	is.Info(core.M("mv-inventories.info.group.infoshares", "{shares}", g.shares.String()))
}

// groupWorldsPrompt collects worlds to add, or with "-" remove, until "@".
// MV: GroupWorldsPrompt
func (m *Module) groupWorldsPrompt(is *core.Issuer, g *WorldGroup, next *prompt, creating bool) *prompt {
	worlds := slices.Clone(g.configWorlds)
	p := &prompt{text: func() core.Message {
		return core.M("mv-inventories.group.worldsprompt", "{group}", g.name, "{worlds}", "§f"+strings.Join(worlds, "§6, §f"))
	}}
	p.accept = func(input string) *prompt {
		if input == "@" {
			if len(worlds) == 0 {
				is.Info(core.M("mv-inventories.group.worldsempty"))
				return p
			}
			g.RemoveAllWorlds(false)
			g.AddWorlds(worlds, false)
			if !creating {
				m.Groups.Update(g)
				is.Info(core.M("mv-inventories.group.updated"))
				m.sendGroupInfo(is, g)
			}
			return next
		}
		name, negative := strings.CutPrefix(input, "-")
		if !m.validWorldName(name) {
			is.Error(core.M("mv-inventories.error.noworld", "{world}", input))
			return p
		}
		if negative {
			i := slices.Index(worlds, name)
			if i < 0 {
				is.Error(core.M("mv-inventories.removeworld.worldnotingroup", "{world}", input, "{group}", g.name))
				return p
			}
			worlds = slices.Delete(worlds, i, i+1)
			return p
		}
		if !slices.Contains(worlds, name) {
			worlds = append(worlds, name)
		}
		return p
	}
	return p
}

// groupSharesPrompt collects shares to add, or with "-" remove, until "@".
// MV: GroupSharesPrompt
func (m *Module) groupSharesPrompt(is *core.Issuer, g *WorldGroup, next *prompt, creating bool) *prompt {
	shares := g.shares.Clone()
	p := &prompt{text: func() core.Message {
		return core.M("mv-inventories.group.sharesprompt", "{group}", g.name, "{shares}", "§f"+strings.Join(shares.names(), "§6, §f"))
	}}
	p.accept = func(input string) *prompt {
		if input == "@" {
			g.shares = shares.Clone()
			m.Groups.Update(g)
			if creating {
				is.Info(core.M("mv-inventories.group.creationcomplete", "{group}", g.name))
			} else {
				is.Info(core.M("mv-inventories.group.updated"))
			}
			m.sendGroupInfo(is, g)
			m.sendConflicts(is, false)
			return next
		}
		sh, ok := LookupShares(input)
		negative := false
		if !ok && len(input) > 1 && strings.HasPrefix(input, "-") {
			negative = true
			sh, ok = LookupShares(input[1:])
		}
		switch {
		case !ok:
			is.Error(core.M("mv-inventories.error.nosharesspecified"))
		case negative:
			shares.RemoveAll(sh)
		default:
			shares.AddAll(sh)
		}
		return p
	}
	return p
}
