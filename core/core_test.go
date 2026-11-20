package core

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
)

func TestParseProperties(t *testing.T) {
	got := parseProperties("# c\n! c\na=b\nkey = value \\\n   continued\nesc=\\u0041\\ \\ x\\n\nspaced: v\nnl=line\\nnext\n")
	want := map[string]string{"a": "b", "key": "value continued", "esc": "A  x\n", "spaced": "v", "nl": "line\nnext"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestEnglishBundle(t *testing.T) {
	l := newLocales()
	if got := l.Format(nil, M("mv-core.create.success", "{world}", "w")); got != "&aWorld 'w' created!" {
		t.Errorf("create.success = %q", got)
	}
	if got := l.Raw(nil, "mv-core.entityspawnconfig.info.spawn"); got != "  spawn: {spawn}" {
		t.Errorf("escaped spaces = %q", got)
	}
}

func TestLegacyText(t *testing.T) {
	txt := Legacy("&aGreen &lbold§r plain &zx")
	if got := txt.PlainString(); got != "Green bold plain &zx" {
		t.Errorf("plain = %q", got)
	}
	if c := txt.Children[0].Style.Color; c != "green" {
		t.Errorf("first colour = %q", c)
	}
	if b := txt.Children[1].Style.Bold; b == nil || !*b || txt.Children[1].Style.Color != "green" {
		t.Errorf("bold keeps the colour: %+v", txt.Children[1].Style)
	}
	if got := StripColor("&6Gold §bAqua & co"); got != "Gold Aqua & co" {
		t.Errorf("strip = %q", got)
	}
}

func TestDecimalFormat(t *testing.T) {
	for f, want := range map[float64]string{0: "0", 1.5: "1.5", -64: "-64", 1234.567: "1,234.57", 1000000: "1,000,000", 0.125: "0.12"} {
		if got := DecimalFormat(f); got != want {
			t.Errorf("DecimalFormat(%v) = %q, want %q", f, got, want)
		}
	}
	if got := JavaDouble(8); got != "8.0" {
		t.Errorf("JavaDouble(8) = %q", got)
	}
}

func TestFilter(t *testing.T) {
	f := ParseFilter("nether")
	if !f.Match("§aworld_nether - NETHER") || f.Match("world - NORMAL") {
		t.Error("substring filter")
	}
	if !ParseFilter("a.b").Match("xa.bx") || ParseFilter("a.b").Match("axb") {
		t.Error("a plain filter escapes regex characters")
	}
	if !ParseFilter("r=^w.*d$").Match("world") {
		t.Error("regex filter")
	}
}

func TestNameUUID(t *testing.T) {
	// Java: UUID.nameUUIDFromBytes("hello".getBytes())
	if got := NameUUID("hello").String(); got != "5d41402a-bc4b-3a76-b971-9d911017c592" {
		t.Errorf("NameUUID = %s", got)
	}
}

func TestParseFlags(t *testing.T) {
	cmd := &Cmd{Name: "create", MinArgs: 2, MaxArgs: 2, Flags: []*Flag{
		{Name: "--seed", Aliases: []string{"-s"}, Value: true},
		{Name: "--no-structures", Aliases: []string{"-a"}},
		{Name: "--remove-players", Value: true, Optional: true},
	}}
	ctx := &Ctx{Cmd: cmd}
	if err := ctx.parse([]string{"w", "normal", "-s", "42", "-a"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Arg(0) != "w" || ctx.Arg(1) != "normal" || ctx.FlagOr("--seed", "") != "42" || !ctx.Has("--no-structures") {
		t.Errorf("parsed %v %v %v", ctx.Args, ctx.flags, ctx.set)
	}
	ctx = &Ctx{Cmd: cmd}
	if err := ctx.parse([]string{"w", "normal", "--seed"}); err == nil {
		t.Error("a value flag without its value parsed")
	}
	ctx = &Ctx{Cmd: cmd}
	if err := ctx.parse([]string{"w", "normal", "--remove-players"}); err != nil || !ctx.Has("--remove-players") {
		t.Errorf("optional value: %v", err)
	}
	ctx = &Ctx{Cmd: cmd}
	if err := ctx.parse([]string{"w", "normal", "extra"}); err == nil {
		t.Error("an extra word parsed")
	}
}

func TestWorldRef(t *testing.T) {
	c := &Core{levelName: "survival"}
	c.Config = newConfig(t.TempDir())
	for in, want := range map[string]string{"survival": "minecraft:overworld", "SURVIVAL_nether": "minecraft:the_nether",
		"survival_the_end": "minecraft:the_end", "Creative": "minecraft:creative"} {
		r, err := c.ParseWorldRef(in)
		if err != nil || r.Key.String() != want || r.Name != in {
			t.Errorf("%s: %v %v", in, r, err)
		}
	}
	if r := c.RefOfKey(plugin.Key{Namespace: "myplugin", Path: "arena"}); r.Name != "myplugin_arena" {
		t.Errorf("custom key name = %s", r.Name)
	}
	if r := c.RefOfKey(plugin.MinecraftKey("the_nether")); r.Name != "survival_nether" || r.Serialise() != "minecraft:the_nether" {
		t.Errorf("nether ref = %+v", r)
	}
	for _, bad := range []string{"", "a b", "Bad:Key", "a:b:c"} {
		if _, err := c.ParseWorldRef(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

// A seed that is not a number hashes as Java's String.hashCode.
func TestParseSeed(t *testing.T) {
	for in, want := range map[string]int64{"42": 42, "-5": -5, "abc": 96354, "Multiverse": -1031087172, "é😀": 1996812} {
		if got := ParseSeed(in); got != want {
			t.Errorf("ParseSeed(%q) = %d, want %d", in, got, want)
		}
	}
	if ParseSeed("") == 0 {
		t.Error("a random seed is 0")
	}
}
