package core

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

//go:embed resources
var resources embed.FS

// Resource answers an embedded file under resources/.
func Resource(name string) []byte {
	data, err := resources.ReadFile("resources/" + name)
	if err != nil {
		panic(err)
	}
	return data
}

// Locales holds the message bundles, by language: Multiverse's
// multiverse-core_<lang>.properties and those modules add. MV: PluginLocales
type Locales struct {
	bundles map[string]map[string]string
	// def is the configured default language (config.yml messaging.default-locale).
	def string
	// perPlayer uses a player's client language when there is a bundle for it.
	perPlayer bool
}

func newLocales() *Locales {
	l := &Locales{bundles: map[string]map[string]string{}, def: "en", perPlayer: true}
	l.AddBundles(resources, "resources", "multiverse-core")
	return l
}

// AddBundles reads every <prefix>_<lang>.properties in dir of fsys into the
// bundles; a module calls it with its own embedded messages.
func (l *Locales) AddBundles(fsys fs.FS, dir, prefix string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		lang, ok := strings.CutPrefix(e.Name(), prefix+"_")
		if !ok {
			continue
		}
		lang, ok = strings.CutSuffix(lang, ".properties")
		if !ok {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			panic(err)
		}
		if l.bundles[lang] == nil {
			l.bundles[lang] = map[string]string{}
		}
		maps.Copy(l.bundles[lang], parseProperties(string(data)))
	}
}

// lang answers the bundle language for sender: the player's client language
// when per-player-locale is on and a bundle exists for it, else the default.
func (l *Locales) lang(sender plugin.CommandSender) string {
	if p, ok := sender.(plugin.Player); ok && l.perPlayer {
		loc := strings.ToLower(p.Locale())
		if _, ok := l.bundles[loc]; ok {
			return loc
		}
		if lang, _, _ := strings.Cut(loc, "_"); l.bundles[lang] != nil {
			return lang
		}
	}
	return l.def
}

// Raw answers the message for key in sender's language, falling back to
// English and then to the key itself, with no replacements.
func (l *Locales) Raw(sender plugin.CommandSender, key string) string {
	if v, ok := l.bundles[l.lang(sender)][key]; ok {
		return v
	}
	if v, ok := l.bundles["en"][key]; ok {
		return v
	}
	return key
}

// Message is a localizable message: a bundle key, or a literal when Key is
// empty, with {placeholder} replacements. MV: Message, MessageReplacement
type Message struct {
	Key     string
	Literal string
	// Repl holds placeholder and value pairs: "{world}", "w1". A value may be
	// a Message, which is formatted for the same sender, or anything else,
	// written with fmt's %v.
	Repl []any
}

// M is a message by bundle key with replacement pairs.
func M(key string, repl ...any) Message { return Message{Key: key, Repl: repl} }

// Lit is a literal message with replacement pairs.
func Lit(s string, repl ...any) Message { return Message{Literal: s, Repl: repl} }

// Format renders m for sender: the template in their language with the
// replacements, '&' codes left in place.
func (l *Locales) Format(sender plugin.CommandSender, m Message) string {
	s := m.Literal
	if m.Key != "" {
		s = l.Raw(sender, m.Key)
	}
	for i := 0; i+1 < len(m.Repl); i += 2 {
		key, _ := m.Repl[i].(string)
		var val string
		switch v := m.Repl[i+1].(type) {
		case Message:
			val = l.Format(sender, v)
		case string:
			val = v
		case error:
			val = v.Error()
		default:
			val = fmt.Sprint(v)
		}
		s = strings.ReplaceAll(s, key, val)
	}
	return s
}

// parseProperties reads a Java .properties file: key=value or key: value
// lines, # and ! comments, backslash continuations and \uXXXX, \n, \t escapes.
func parseProperties(data string) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var logical strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if logical.Len() == 0 {
			line = strings.TrimLeft(line, " \t\f")
			if line == "" || line[0] == '#' || line[0] == '!' {
				continue
			}
		} else {
			line = strings.TrimLeft(line, " \t\f")
		}
		if n := trailingBackslashes(line); n%2 == 1 {
			logical.WriteString(line[:len(line)-1])
			continue
		}
		logical.WriteString(line)
		key, value := splitProperty(logical.String())
		out[unescapeProperty(key)] = unescapeProperty(value)
		logical.Reset()
	}
	return out
}

func trailingBackslashes(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n
}

// splitProperty splits at the first unescaped '=', ':' or whitespace.
func splitProperty(line string) (string, string) {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '=', ':':
			return line[:i], strings.TrimLeft(line[i+1:], " \t\f")
		case ' ', '\t', '\f':
			rest := strings.TrimLeft(line[i:], " \t\f")
			if rest != "" && (rest[0] == '=' || rest[0] == ':') {
				rest = rest[1:]
			}
			return line[:i], strings.TrimLeft(rest, " \t\f")
		}
	}
	return line, ""
}

func unescapeProperty(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			out.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case 'f':
			out.WriteByte('\f')
		case 'u':
			if i+4 < len(s) {
				if r, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					out.WriteRune(rune(r))
					i += 4
					continue
				}
			}
			out.WriteByte('u')
		default:
			out.WriteByte(s[i])
		}
	}
	return out.String()
}
