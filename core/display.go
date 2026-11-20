package core

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Filter is a content filter: a regex with "r=", else a plain substring,
// matched against the colourless line, ignoring case. MV: RegexContentFilter
type Filter struct {
	raw string
	re  *regexp.Regexp
}

var regexSpecial = regexp.MustCompile(`[.+*?\[^\]$(){}=!<>|:\-\\]`)

// ParseFilter reads a --filter value; "" matches everything.
func ParseFilter(s string) Filter {
	if s == "" {
		return Filter{}
	}
	expr := regexSpecial.ReplaceAllString(strings.ToLower(s), `\$0`)
	if r, ok := strings.CutPrefix(s, "r="); ok {
		expr = r
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return Filter{raw: expr}
	}
	return Filter{raw: expr, re: re}
}

// Active reports whether the filter filters anything.
func (f Filter) Active() bool { return f.re != nil }

// Match reports whether line passes.
func (f Filter) Match(line string) bool {
	if f.re == nil {
		return true
	}
	return f.re.MatchString(strings.ToLower(StripColor(line)))
}

func (f Filter) String() string {
	if f.re == nil {
		return "N/A"
	}
	return f.raw
}

// Display sends a list of lines with a header, filtered and paged. MV:
// ContentDisplay with PagedSendHandler
type Display struct {
	Header    *Message
	NoContent *Message
	Filter    Filter
	Page      int
	// Paginate pages the lines (8 per page, padded); the console gets every
	// line at once, as Multiverse does.
	Paginate bool
	PerPage  int
}

// NewDisplay answers a paged display of page 1 with Multiverse's defaults.
func NewDisplay() *Display {
	nc := M("mv-core.contentdisplay.nocontent")
	return &Display{NoContent: &nc, Page: 1, Paginate: true, PerPage: 8}
}

// Send shows lines to is.
func (d *Display) Send(is *Issuer, lines []string) {
	if d.Header != nil {
		is.Send(*d.Header)
	}
	var shown []string
	for _, l := range lines {
		if d.Filter.Match(l) {
			shown = append(shown, l)
		}
	}
	if len(shown) == 0 && d.NoContent != nil {
		is.Send(*d.NoContent)
		return
	}
	if !d.Paginate || !is.IsPlayer() {
		if d.Filter.Active() {
			is.Send(M("mv-core.contentdisplay.filter", "{filter}", d.Filter.String()))
		}
		for _, l := range shown {
			is.SendRaw(l)
		}
		return
	}
	total := (len(shown) + d.PerPage - 1) / d.PerPage
	if d.Page < 1 || d.Page > total {
		is.Send(M("mv-core.contentdisplay.invalidpage", "{total}", total))
		return
	}
	page := M("mv-core.contentdisplay.page", "{current}", d.Page, "{total}", total)
	if d.Filter.Active() {
		is.Send(M("mv-core.contentdisplay.pagefilter", "{page}", page,
			"{filter}", M("mv-core.contentdisplay.filter", "{filter}", d.Filter.String())))
	} else {
		is.Send(page)
	}
	start := (d.Page - 1) * d.PerPage
	end := min(start+d.PerPage, len(shown))
	for _, l := range shown[start:end] {
		is.SendRaw(l)
	}
	for range start + d.PerPage - end {
		is.SendRaw("")
	}
}

// PageFilterFlags are the --page and --filter flags of the listing
// commands. MV: PageFilterFlags
func PageFilterFlags() []*Flag {
	return []*Flag{
		{Name: "--page", Aliases: []string{"-p"}, Value: true,
			Complete: func(*Ctx, string) []string { return []string{"1", "2", "3"} }},
		{Name: "--filter", Aliases: []string{"-f"}, Value: true},
	}
}

// PagedDisplay answers a display set from ctx's --page and --filter.
func (ctx *Ctx) PagedDisplay() (*Display, error) {
	d := NewDisplay()
	if v, ok := ctx.flags["--page"]; ok {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, InvalidArg(Lit("Please specify a number."))
		}
		d.Page = p
	}
	d.Filter = ParseFilter(ctx.flags["--filter"])
	return d, nil
}

// MapLines formats key-value pairs as Multiverse's MapContentProvider does:
// key colour, key, ": ", value colour, value; an empty value shows as
// "empty" and a nil one as "null".
func MapLines(is *Issuer, keys []string, values map[string]*string, keyColour, valueColour string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := values[k]
		var s string
		switch {
		case v == nil:
			s = is.Format(M("mv-core.contentdisplay.null"))
		case *v == "":
			s = is.Format(M("mv-core.contentdisplay.empty"))
		default:
			s = *v
		}
		out = append(out, keyColour+k+": "+valueColour+s)
	}
	return out
}

// Help shows the subcommands sender may run, those matching filter, paged
// as ACF's CommandHelp: 4 per page for a player, 7 for the console.
// MV: HelpCommand (ACF's help)
func (cs *Commands) Help(is *Issuer, label string, cmds []*Cmd, filter string, page int) {
	type entry struct{ usage, desc string }
	var entries []entry
	for _, cmd := range cmds {
		if !cs.allowed(is.Sender, cmd) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(cmd.Name), strings.ToLower(filter)) &&
			!strings.Contains(strings.ToLower(is.Format(M(cmd.Description))), strings.ToLower(filter)) {
			continue
		}
		usage := "/" + label
		if cs.isRoot(label) {
			usage += " " + cmd.Name
		}
		if cmd.Syntax != "" {
			usage += " §a" + cmd.Syntax
		}
		entries = append(entries, entry{usage, is.Format(M(cmd.Description))})
	}
	per := 7
	if is.IsPlayer() {
		per = 4
	}
	total := max((len(entries)+per-1)/per, 1)
	page = min(max(page, 1), total)
	if filter != "" {
		is.SendRaw(fmt.Sprintf("§3=== §bShowing help for §3/%s§b, searching for §3%s §b(§3%d§b/§3%d§b) §3===", label, filter, page, total))
	} else {
		is.SendRaw(fmt.Sprintf("§3=== §bShowing help for §3/%s §b(§3%d§b/§3%d§b) §3===", label, page, total))
	}
	for _, e := range entries[min((page-1)*per, len(entries)):min(page*per, len(entries))] {
		is.SendRaw("§b" + e.usage + " §e- " + e.desc)
	}
	if page < total {
		is.SendRaw(fmt.Sprintf("§3=== §bType §3/%s help %s%d §bfor more §3===", label, filterPrefix(filter), page+1))
	}
}

func filterPrefix(filter string) string {
	if filter == "" {
		return ""
	}
	return filter + " "
}

// confirmQueue holds each sender's command waiting for /mv confirm.
// MV: CommandQueueManager
type confirmQueue struct {
	core    *Core
	pending map[string]*queued
}

type queued struct {
	otp    string
	action func()
	expire plugin.Task
}

func newConfirmQueue(c *Core) *confirmQueue {
	return &confirmQueue{core: c, pending: map[string]*queued{}}
}

// senderKey names a sender in the queue: @console for the console.
func (q *confirmQueue) senderKey(is *Issuer) string {
	if is.IsConsole() {
		return "@console"
	}
	if !is.IsPlayer() {
		return "@commandblock"
	}
	return is.Sender.Name()
}

// Confirm runs action once the sender confirms with /mv confirm, or at once
// when confirm-mode lets this sender skip it; prompt says what is at stake.
// MV: CommandQueueManager.addToQueue
func (cs *Commands) Confirm(is *Issuer, prompt Message, action func()) {
	q := cs.queue
	key := q.senderKey(is)
	if q.canRunImmediately(key) {
		action()
		return
	}
	q.remove(key)
	p := &queued{otp: strconv.Itoa(100 + rand.IntN(900)), action: action}
	q.pending[key] = p
	timeout := cs.core.Config.Int("command.confirm-timeout")
	p.expire = cs.core.Server.Scheduler().RunLater(timeout*20, func() {
		if q.pending[key] == p {
			delete(q.pending, key)
			is.Send(M("mv-core.queuecommand.expired"))
		}
	})
	is.Info(prompt)
	command := "/mv confirm"
	if cs.core.Config.Bool("command.use-confirm-otp") {
		command += " " + p.otp
	}
	is.Send(M("mv-core.queuecommand.prompt", "{command}", command, "{timeout}", timeout))
}

func (q *confirmQueue) canRunImmediately(key string) bool {
	switch q.core.Config.String("command.confirm-mode") {
	case "player_only":
		return key == "@console" || key == "@commandblock"
	case "disable_command_blocks":
		return key == "@commandblock"
	case "disable_console":
		return key == "@console"
	case "disable":
		return true
	}
	return false
}

func (q *confirmQueue) remove(key string) {
	if p, ok := q.pending[key]; ok {
		p.expire.Cancel()
		delete(q.pending, key)
	}
}

// RunConfirmed runs the sender's queued command. MV:
// CommandQueueManager.runQueuedCommand
func (cs *Commands) RunConfirmed(is *Issuer, otp string) error {
	q := cs.queue
	key := q.senderKey(is)
	p, ok := q.pending[key]
	if !ok {
		return MsgError(M("mv-core.queuecommand.nocommandinqueue"))
	}
	if cs.core.Config.Bool("command.use-confirm-otp") && p.otp != otp {
		return MsgError(M("mv-core.queuecommand.invalidotp", "{otp}", otp))
	}
	q.remove(key)
	p.action()
	return nil
}
