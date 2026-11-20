package core

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// legacyColours maps a colour code to its JSON text colour name.
var legacyColours = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua",
	'4': "dark_red", '5': "dark_purple", '6': "gold", '7': "gray",
	'8': "dark_gray", '9': "blue", 'a': "green", 'b': "aqua",
	'c': "red", 'd': "light_purple", 'e': "yellow", 'f': "white",
}

// Colorize turns '&' colour codes into '§' ones, as Bukkit's
// ChatColor.translateAlternateColorCodes('&', s) does: only before a valid
// code character.
func Colorize(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	b := []byte(s)
	var out strings.Builder
	for i := range b {
		if b[i] == '&' && i+1 < len(b) && isCode(b[i+1]) {
			out.WriteString("§")
			continue
		}
		out.WriteByte(b[i])
	}
	return out.String()
}

func isCode(c byte) bool {
	c |= 0x20
	_, colour := legacyColours[c]
	return colour || strings.IndexByte("klmnor", c) >= 0
}

// StripColor removes '§' and '&' colour codes. Bukkit: ChatColor.stripColor
// after translating '&'
func StripColor(s string) string {
	s = Colorize(s)
	if !strings.Contains(s, "§") {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "§") && i+2 < len(s) {
			i += 2 // the two bytes of '§' and the code byte
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// Legacy builds a text from a string with '§' (or '&') colour codes. A colour
// code resets the formats before it, as the legacy chat format does.
func Legacy(s string) plugin.Text {
	s = Colorize(s)
	if !strings.Contains(s, "§") {
		return plugin.Plain(s)
	}
	root := plugin.Plain("")
	var buf strings.Builder
	var style plugin.Style
	flush := func() {
		if buf.Len() > 0 {
			root = root.Append(plugin.Text{Literal: buf.String(), Style: style})
			buf.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		if !strings.HasPrefix(s[i:], "§") || i+2 >= len(s) {
			buf.WriteByte(s[i])
			continue
		}
		code := s[i+2] | 0x20
		i += 2
		flush()
		if c, ok := legacyColours[code]; ok {
			style = plugin.Style{Color: c}
			continue
		}
		switch code {
		case 'l':
			style.Bold = new(true)
		case 'o':
			style.Italic = new(true)
		case 'n':
			style.Underlined = new(true)
		case 'm':
			style.Strikethrough = new(true)
		case 'k':
			style.Obfuscated = new(true)
		case 'r':
			style = plugin.Style{Color: "white"}
		default:
			buf.WriteString("§")
			buf.WriteByte(s[i])
		}
	}
	flush()
	return root
}
