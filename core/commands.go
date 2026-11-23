package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Cmd is one /mv subcommand: a port of an ACF @Subcommand method. MV:
// CoreCommand subclasses
type Cmd struct {
	// Name is the subcommand path, one or two words: "create", "anchor set".
	Name string
	// Aliases are other paths for it: "tp" for "teleport".
	Aliases []string
	// Legacy are its Multiverse 4 root commands (mvcreate, mvc), registered
	// when show-legacy-aliases is on. With two words the second is a
	// subcommand of the legacy root: "mvrule set".
	Legacy []string
	// Roots are root commands that run it whatever show-legacy-aliases
	// says, as an ACF @CommandAlias on the method does (/mvtp).
	Roots      []string
	Permission string
	// Syntax is the usage after the subcommand, e.g. "<name> <environment>".
	Syntax string
	// Description is the help line's message key.
	Description string
	// MinArgs and MaxArgs bound the positional arguments, those before the
	// flags; MaxArgs -1 takes any number.
	MinArgs, MaxArgs int
	Flags            []*Flag
	// PlayerOnly refuses the console.
	PlayerOnly bool
	Run        func(ctx *Ctx) error
	// Complete suggests the positional argument at index pos.
	Complete func(ctx *Ctx, pos int) []string
}

// Flag is a command flag: --name or one of its aliases, with or without a
// value. MV: CommandFlag, CommandValueFlag
type Flag struct {
	Name    string
	Aliases []string
	// Value means the flag takes the next word as its value; Optional, that
	// the value may be left out.
	Value, Optional bool
	Complete        func(ctx *Ctx, input string) []string
}

func (f *Flag) matches(word string) bool {
	return f.Name == word || slices.Contains(f.Aliases, word)
}

// Ctx is a subcommand being run or completed.
type Ctx struct {
	*Issuer
	Core *Core
	Cmd  *Cmd
	// Label is the root command typed, e.g. "mv" or "mvtp".
	Label string
	// Args are the positional arguments; Words is every word after the
	// subcommand.
	Args  []string
	Words []string
	flags map[string]string
	set   map[string]bool
}

// Has reports whether a flag was given.
func (ctx *Ctx) Has(flag string) bool { return ctx.set[flag] }

// Flag answers a flag's value and whether the flag was given.
func (ctx *Ctx) Flag(flag string) (string, bool) {
	v, ok := ctx.flags[flag]
	return v, ok || ctx.set[flag]
}

// FlagOr answers a flag's value, or def when it was not given or had none.
func (ctx *Ctx) FlagOr(flag, def string) string {
	if v, ok := ctx.flags[flag]; ok && v != "" {
		return v
	}
	return def
}

// Arg answers positional argument i, "" when missing.
func (ctx *Ctx) Arg(i int) string {
	if i < len(ctx.Args) {
		return ctx.Args[i]
	}
	return ""
}

// ErrUsage makes the command fail with its usage line.
var ErrUsage = errors.New("usage")

// UsageError is a failure shown as ACF shows an InvalidCommandArgument: the
// message, then the usage.
type UsageError struct{ Msg Message }

func (e *UsageError) Error() string { return english.Format(nil, e.Msg) }

// InvalidArg fails the command with a message and the usage.
func InvalidArg(m Message) error { return &UsageError{Msg: m} }

// Commands is /mv and its subcommands. MV: MVCommandManager
type Commands struct {
	core *Core
	list []*Cmd
	// legacy maps a legacy root command to the subcommands it runs.
	legacy map[string][]*Cmd
	// dynamic decides the "@name" permissions, ACF's permission
	// resolvers: @mvteleport is any teleport permission.
	dynamic map[string]func(plugin.CommandSender) bool
	queue   *confirmQueue
	// roots are the module roots (/mvinv), whose usage names the
	// subcommand as /mv's does.
	roots map[string]bool
}

// isRoot reports whether label is /mv or a module's root, not a legacy alias.
func (cs *Commands) isRoot(label string) bool { return label == "mv" || cs.roots[label] }

// SetDynamicPermission decides cmd.Permission "@name" with check.
func (cs *Commands) SetDynamicPermission(name string, check func(plugin.CommandSender) bool) {
	cs.dynamic[name] = check
}

func newCommands(c *Core) *Commands {
	return &Commands{core: c, legacy: map[string][]*Cmd{}, dynamic: map[string]func(plugin.CommandSender) bool{},
		queue: newConfirmQueue(c), roots: map[string]bool{}}
}

// Add adds a subcommand; Register must not have run yet unless the module
// adds its own root (as Multiverse modules add /mvp and /mvnp).
func (cs *Commands) Add(cmds ...*Cmd) {
	for _, cmd := range cmds {
		cs.list = append(cs.list, cmd)
		if cmd.Permission != "" && !strings.HasPrefix(cmd.Permission, "@") {
			cs.core.Permissions.Register(cmd.Permission, DefaultOp)
		}
	}
}

// All answers the subcommands in the order they were added.
func (cs *Commands) All() []*Cmd { return slices.Clone(cs.list) }

// Register registers /mv, each command's Roots, and the legacy roots when
// show-legacy-aliases is on, with the server.
func (cs *Commands) Register() error {
	if err := cs.registerRoot("mv", nil, "Multiverse-Core commands"); err != nil {
		return err
	}
	cs.registerAliases(cs.list)
	return nil
}

// registerAliases registers cmds' Roots, and their legacy roots when
// show-legacy-aliases is on.
func (cs *Commands) registerAliases(cmds []*Cmd) {
	legacy := cs.core.Config.Bool("command.show-legacy-aliases")
	var added []string
	for _, cmd := range cmds {
		roots := cmd.Roots
		if legacy {
			roots = append(slices.Clone(roots), cmd.Legacy...)
		}
		for _, l := range roots {
			root, _, _ := strings.Cut(l, " ")
			if cs.legacy[root] == nil {
				added = append(added, root)
			}
			if !slices.Contains(cs.legacy[root], cmd) {
				cs.legacy[root] = append(cs.legacy[root], cmd)
			}
		}
	}
	slices.Sort(added)
	for _, root := range added {
		if err := cs.registerRoot(root, cs.legacy[root], ""); err != nil {
			cs.core.Log.Warn("Could not register legacy alias", "alias", root, "error", err)
		}
	}
}

// registerRoot registers one root command: with only, a legacy alias that
// runs those subcommands.
func (cs *Commands) registerRoot(name string, only []*Cmd, desc string) error {
	run := func(ctx *plugin.CommandContext) (int, error) {
		cs.Dispatch(ctx.Sender, name, only, plugin.Arg[string](ctx, "args"))
		return 1, nil
	}
	return cs.core.Server.Commands().Register(plugin.Command{
		Name: name, Description: desc, Usage: "/" + name,
		Run: run, Children: []plugin.Node{{
			Arg: plugin.GreedyArg("args"), Run: run,
			Suggest: func(ctx *plugin.CommandContext, partial string) []string {
				return cs.Suggest(ctx.Sender, only, partial)
			},
		}},
	})
}

// RegisterRoot registers a module's own root command (/mvp, /mvnp, /mvinv)
// whose subcommands are cmds, run like /mv's, and their Roots and Legacy
// roots as /mv's are.
func (cs *Commands) RegisterRoot(name string, aliases []string, cmds []*Cmd) error {
	cs.roots[name] = true
	defer cs.registerAliases(cmds)
	for _, cmd := range cmds {
		if cmd.Permission != "" && !strings.HasPrefix(cmd.Permission, "@") {
			cs.core.Permissions.Register(cmd.Permission, DefaultOp)
		}
	}
	run := func(ctx *plugin.CommandContext) (int, error) {
		cs.dispatchIn(ctx.Sender, name, cmds, plugin.Arg[string](ctx, "args"))
		return 1, nil
	}
	return cs.core.Server.Commands().Register(plugin.Command{
		Name: name, Aliases: aliases, Usage: "/" + name,
		Run: run, Children: []plugin.Node{{
			Arg: plugin.GreedyArg("args"), Run: run,
			Suggest: func(ctx *plugin.CommandContext, partial string) []string {
				return cs.suggestIn(ctx.Sender, cmds, partial, false)
			},
		}},
	})
}

// find answers the subcommand line starts with and the words after its
// path, preferring the longest path.
func find(cmds []*Cmd, words []string) (*Cmd, []string) {
	var best *Cmd
	bestLen := 0
	for _, cmd := range cmds {
		for _, path := range append([]string{cmd.Name}, cmd.Aliases...) {
			parts := strings.Fields(path)
			if len(parts) <= len(words) && len(parts) > bestLen && equalFoldAll(parts, words[:len(parts)]) {
				best, bestLen = cmd, len(parts)
			}
		}
	}
	if best == nil {
		return nil, words
	}
	return best, words[bestLen:]
}

func equalFoldAll(a, b []string) bool {
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Dispatch runs a /mv line (the words after the root). only, for a legacy
// root, holds the subcommands it runs. MV: ACF's command dispatch
func (cs *Commands) Dispatch(sender plugin.CommandSender, label string, only []*Cmd, line string) {
	if only != nil {
		cs.dispatchLegacy(sender, label, only, line)
		return
	}
	cs.dispatchIn(sender, label, cs.list, line)
}

func (cs *Commands) dispatchIn(sender plugin.CommandSender, label string, cmds []*Cmd, line string) {
	words := strings.Fields(line)
	is := cs.core.Issuer(sender)
	if len(words) == 0 {
		if label == "mv" {
			is.Info(M("mv-core.root.title", "{name}", "Multiverse-Core", "{version}", Version))
			is.Info(M("mv-core.root.help"))
			return
		}
		cs.Help(is, label, cmds, "", 1)
		return
	}
	cmd, rest := find(cmds, words)
	if cmd == nil {
		// ACF runs the @HelpCommand for an unknown subcommand.
		cs.Help(is, label, cmds, "", 1)
		return
	}
	cs.run(is, label, cmd, rest)
}

// dispatchLegacy runs a legacy root: one subcommand takes the words as they
// are, several pick theirs by the next word ("mvrule set ...").
func (cs *Commands) dispatchLegacy(sender plugin.CommandSender, label string, only []*Cmd, line string) {
	words := strings.Fields(line)
	is := cs.core.Issuer(sender)
	for _, cmd := range only {
		for _, l := range cmd.rootPaths() {
			root, sub, _ := strings.Cut(l, " ")
			if root != label {
				continue
			}
			if sub == "" {
				cs.run(is, label, cmd, words)
				return
			}
			if len(words) > 0 && strings.EqualFold(words[0], sub) {
				cs.run(is, label+" "+sub, cmd, words[1:])
				return
			}
		}
	}
	cs.Help(is, label, only, "", 1)
}

// run checks the permission, splits the words into arguments and flags, and
// runs cmd, reporting its error.
func (cs *Commands) run(is *Issuer, label string, cmd *Cmd, words []string) {
	if !cs.allowed(is.Sender, cmd) {
		is.Error(Lit("I'm sorry, but you do not have permission to perform this command."))
		return
	}
	if cmd.PlayerOnly && !is.IsPlayer() {
		is.Error(M("mv-core.commands.error.playersonly"))
		return
	}
	ctx := &Ctx{Issuer: is, Core: cs.core, Cmd: cmd, Label: label, Words: words}
	if err := ctx.parse(words); err != nil {
		cs.report(ctx, err)
		return
	}
	if len(ctx.Args) < cmd.MinArgs {
		cs.report(ctx, ErrUsage)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			cs.core.Log.Error(fmt.Sprintf("Failed to execute command /%s %s %s: %v", label, cmd.Name, strings.Join(words, " "), r))
			is.Error(Lit("I'm sorry, but there was an error performing this command."))
		}
	}()
	if err := cmd.Run(ctx); err != nil {
		cs.report(ctx, err)
	}
}

// Usage answers cmd's usage line: /mv create <name> <environment> ...
func (ctx *Ctx) Usage() string {
	return strings.TrimSpace(ctx.usagePath() + " " + ctx.Cmd.Syntax)
}

func (ctx *Ctx) usagePath() string {
	label := ctx.Label
	if ctx.Core.Commands.isRoot(label) {
		label += " " + ctx.Cmd.Name
	}
	return "/" + label
}

func (cs *Commands) report(ctx *Ctx, err error) {
	var ue *UsageError
	switch {
	case errors.Is(err, ErrUsage):
	case errors.As(err, &ue):
		if ue.Msg.Key == "" {
			ctx.SendRaw("§cError: " + ctx.Format(ue.Msg))
		} else {
			ctx.Error(ue.Msg)
		}
	case errors.Is(err, ErrHandled):
		return
	default:
		ctx.Fail(err)
		return
	}
	usage := "§eUsage: §a" + ctx.usagePath()
	if syntax := strings.TrimSpace(ctx.Cmd.Syntax); syntax != "" {
		usage += "§f " + syntax
	}
	ctx.SendRaw(usage)
}

// ErrHandled is what Run answers when it has already told the sender.
var ErrHandled = errors.New("handled")

func (cs *Commands) allowed(sender plugin.CommandSender, cmd *Cmd) bool {
	if cmd.Permission == "" {
		return true
	}
	if check, ok := cs.dynamic[cmd.Permission]; ok {
		return check(sender)
	}
	return sender.HasPermission(cmd.Permission)
}

// parse splits words into positional arguments, which end at the first
// word that is a flag, and flags. MV: CommandFlagsParser
func (ctx *Ctx) parse(words []string) error {
	ctx.flags, ctx.set = map[string]string{}, map[string]bool{}
	i := 0
	for ; i < len(words); i++ {
		if ctx.flagOf(words[i]) != nil {
			break
		}
		if ctx.Cmd.MaxArgs >= 0 && len(ctx.Args) >= ctx.Cmd.MaxArgs {
			if len(ctx.Cmd.Flags) == 0 {
				break
			}
			return InvalidArg(Lit(words[i] + " is not a valid flag."))
		}
		ctx.Args = append(ctx.Args, words[i])
	}
	var current *Flag
	for ; i < len(words); i++ {
		w := words[i]
		if f := ctx.flagOf(w); f != nil {
			if current != nil && !current.Optional {
				return InvalidArg(Lit(current.Name + " requires a value!"))
			}
			ctx.set[f.Name] = true
			current = nil
			if f.Value {
				current = f
			}
			continue
		}
		if current == nil {
			return InvalidArg(Lit(w + " is not a valid flag."))
		}
		ctx.flags[current.Name] = w
		current = nil
	}
	if current != nil && !current.Optional {
		return InvalidArg(Lit(current.Name + " requires a value!"))
	}
	return nil
}

func (ctx *Ctx) flagOf(word string) *Flag {
	for _, f := range ctx.Cmd.Flags {
		if f.matches(word) {
			return f
		}
	}
	return nil
}

// Suggest completes a /mv line: the greedy text after the root, answered as
// whole replacements of it.
func (cs *Commands) Suggest(sender plugin.CommandSender, only []*Cmd, partial string) []string {
	if only != nil {
		return cs.suggestLegacy(sender, only, partial)
	}
	return cs.suggestIn(sender, cs.list, partial, true)
}

func (cs *Commands) suggestIn(sender plugin.CommandSender, cmds []*Cmd, partial string, help bool) []string {
	words := strings.Split(partial, " ")
	// Completing the subcommand's own words.
	var names []string
	for _, cmd := range cmds {
		if !cs.allowed(sender, cmd) {
			continue
		}
		for _, path := range append([]string{cmd.Name}, cmd.Aliases...) {
			parts := strings.Fields(path)
			if len(words) <= len(parts) && equalFoldAll(words[:len(words)-1], parts[:len(words)-1]) {
				names = append(names, parts[len(words)-1])
			}
		}
	}
	cmd, rest := find(cmds, words[:len(words)-1])
	if cmd == nil || len(rest) == 0 && len(words) == 1 {
		return completeWith(words, names)
	}
	if !cs.allowed(sender, cmd) {
		return nil
	}
	return completeWith(words, append(names, cs.complete(cs.core.Issuer(sender), cmd, append(rest, words[len(words)-1]))...))
}

func (cs *Commands) suggestLegacy(sender plugin.CommandSender, only []*Cmd, partial string) []string {
	words := strings.Split(partial, " ")
	var subs []string
	for _, cmd := range only {
		for _, l := range cmd.rootPaths() {
			_, sub, _ := strings.Cut(l, " ")
			if sub == "" {
				if !cs.allowed(sender, cmd) {
					return nil
				}
				return completeWith(words, cs.complete(cs.core.Issuer(sender), cmd, words))
			}
			if !cs.allowed(sender, cmd) {
				continue
			}
			if len(words) == 1 {
				subs = append(subs, sub)
			} else if strings.EqualFold(words[0], sub) {
				return completeWith(words, cs.complete(cs.core.Issuer(sender), cmd, words[1:]))
			}
		}
	}
	return completeWith(words, subs)
}

// complete answers the candidates for the last of words, the arguments
// after cmd's path: a flag's value, a flag, or a positional argument.
func (cs *Commands) complete(is *Issuer, cmd *Cmd, words []string) []string {
	ctx := &Ctx{Issuer: is, Core: cs.core, Cmd: cmd, Label: "mv", Words: words}
	last := words[len(words)-1]
	before := words[:len(words)-1]
	pos := 0
	inFlags := false
	used := map[string]bool{}
	var pending *Flag
	for _, w := range before {
		if f := ctx.flagOf(w); f != nil {
			inFlags = true
			used[f.Name] = true
			pending = nil
			if f.Value {
				pending = f
			}
			continue
		}
		if pending != nil {
			pending = nil
			continue
		}
		if !inFlags {
			ctx.Args = append(ctx.Args, w)
			pos++
		}
	}
	var out []string
	if pending != nil && pending.Complete != nil {
		out = append(out, pending.Complete(ctx, last)...)
		if !pending.Optional {
			return out
		}
	}
	flagsOnly := inFlags || cmd.MaxArgs >= 0 && pos >= cmd.MaxArgs
	if !flagsOnly && cmd.Complete != nil {
		out = append(out, cmd.Complete(ctx, pos)...)
	}
	if flagsOnly || pos >= cmd.MinArgs || strings.HasPrefix(last, "-") {
		for _, f := range cmd.Flags {
			if !used[f.Name] {
				out = append(out, f.Name)
			}
		}
	}
	return out
}

// completeWith keeps the candidates that start with the last word (ignoring
// case) and answers each as the whole line.
func completeWith(words []string, candidates []string) []string {
	last := strings.ToLower(words[len(words)-1])
	prefix := strings.Join(words[:len(words)-1], " ")
	if prefix != "" {
		prefix += " "
	}
	var out []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), last) {
			out = append(out, prefix+c)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// rootPaths answers the root commands (and legacy subcommand paths) that
// run cmd outside /mv.
func (cmd *Cmd) rootPaths() []string { return append(slices.Clone(cmd.Roots), cmd.Legacy...) }
