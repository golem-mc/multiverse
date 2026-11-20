package listeners

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, chat) }

// chat puts the sender's world alias before chat lines when
// enable-chat-prefix is on, through chat-prefix-format: %world% is the
// alias or name, %chat% the line as it would be rendered. MV:
// MVChatListener.asyncChat
func chat(c *core.Core) {
	plugin.Listen(c.Server, func(e *plugin.PlayerChatEvent) {
		if !c.Config.Bool("messaging.enable-chat-prefix") {
			return
		}
		inner := e.Format
		e.Format = func(sender plugin.Player, message plugin.Text, viewer plugin.Player) plugin.Text {
			line := plugin.Translate("chat.type.text", sender.DisplayName(), message)
			if inner != nil {
				line = inner(sender, message, viewer)
			}
			return prefixed(c.Config.String("messaging.chat-prefix-format"), chatWorldName(c, sender), line)
		}
	}, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
}

// chatWorldName answers the alias or name of p's world; "" for a hidden
// world or one Multiverse does not know. MV: MVChatListener.getWorldName
func chatWorldName(c *core.Core, p plugin.Player) string {
	w, ok := c.Worlds.Of(p.World())
	if !ok || w.Hidden() {
		return ""
	}
	return w.AliasOrName()
}

// prefixed renders format with '&' codes, %world% replaced by world (with
// its own codes) and %chat% by line, which takes the colour in force where
// it stands.
func prefixed(format, world string, line plugin.Text) plugin.Text {
	before, after, found := strings.Cut(strings.ReplaceAll(format, "%world%", world), "%chat%")
	head := core.Legacy(before)
	out := plugin.Plain("").Append(head)
	if found {
		style := head.Style
		if n := len(head.Children); n > 0 {
			style = head.Children[n-1].Style
		}
		out = out.Append(plugin.Text{Style: style, Children: []plugin.Text{line}}, core.Legacy(after))
	}
	return out
}
