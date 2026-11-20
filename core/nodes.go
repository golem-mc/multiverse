package core

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Kind is the type of a node's value.
type Kind uint8

const (
	// KindHeader is a section with comments and no value of its own.
	KindHeader Kind = iota
	KindBool
	KindInt
	KindLong
	KindDouble
	KindString
	// KindEnum is a string from Enum, stored lowercase.
	KindEnum
	// KindList is a list of strings.
	KindList
	// KindCustom is a value the node's Decode and Encode handle.
	KindCustom
)

// Node is one setting of config.yml or of a world in worlds.yml: where it
// sits in the YAML, the name commands use for it, its comments, type and
// default. MV: ConfigNode, ListConfigNode, ConfigHeaderNode
type Node struct {
	// Path is the dotted YAML path, e.g. "world.enforce-access".
	Path string
	// Name is what /mv config and /mv modify call it; empty for the path.
	Name string
	// Aliases are other names the commands accept.
	Aliases  []string
	Comments []string
	Kind     Kind
	// Default is the value when the file has none; DefaultFn, when set,
	// computes it instead.
	Default   any
	DefaultFn func(v *Values) any
	// Enum lists a KindEnum node's values, lowercase.
	Enum []string
	// Hidden nodes are saved but cannot be read or changed by name.
	Hidden bool
	// Validate refuses a new value with a message.
	Validate func(v any) error
	// Suggest completes a value being typed; nil uses the kind's.
	Suggest func(input string) []string
	// Parse reads a value typed in a command; nil uses the kind's parser.
	Parse func(input string) (any, error)
	// Decode and Encode convert a KindCustom value from and to its YAML form.
	Decode func(raw any) any
	Encode func(v any) any
	// OnChange runs after the value changes through Set or Reset.
	OnChange func(old, new any)
}

// NameOrPath answers the name commands use for n.
func (n *Node) NameOrPath() string {
	if n.Name != "" {
		return n.Name
	}
	return n.Path
}

// Nodes is an ordered set of nodes. MV: NodeGroup
type Nodes struct {
	list   []*Node
	byName map[string]*Node
}

// NewNodes indexes nodes by path, name and alias.
func NewNodes(nodes ...*Node) *Nodes {
	ns := &Nodes{byName: map[string]*Node{}}
	for _, n := range nodes {
		ns.Add(n)
	}
	return ns
}

// Add appends a node, as a module adds its own.
func (ns *Nodes) Add(n *Node) {
	ns.list = append(ns.list, n)
	if n.Kind == KindHeader {
		return
	}
	ns.byName[n.Path] = n
	ns.byName[n.NameOrPath()] = n
	for _, a := range n.Aliases {
		ns.byName[a] = n
	}
}

// All answers the nodes in order.
func (ns *Nodes) All() []*Node { return ns.list }

// Find answers the visible value node called name.
func (ns *Nodes) Find(name string) (*Node, bool) {
	n, ok := ns.byName[name]
	if !ok || n.Hidden {
		return nil, false
	}
	return n, true
}

// Names answers the names of the visible value nodes, in order. MV:
// NodeGroup.getNames
func (ns *Nodes) Names() []string {
	var out []string
	for _, n := range ns.list {
		if n.Kind != KindHeader && !n.Hidden {
			out = append(out, n.NameOrPath())
		}
	}
	return out
}

// ListNames answers the names of the visible list nodes, which add and
// remove change.
func (ns *Nodes) ListNames() []string {
	var out []string
	for _, n := range ns.list {
		if n.Kind == KindList && !n.Hidden {
			out = append(out, n.NameOrPath())
		}
	}
	return out
}

// Values holds the values of a node set, by path.
type Values struct {
	nodes *Nodes
	vals  map[string]any
	// extra keeps the keys of the file no node reads, so they survive a save.
	extra map[string]any
}

// NewValues answers a value set with every node at its default.
func NewValues(nodes *Nodes) *Values {
	v := &Values{nodes: nodes, vals: map[string]any{}}
	return v
}

// Nodes answers the node set.
func (v *Values) Nodes() *Nodes { return v.nodes }

// Get answers the node's value, or its default.
func (v *Values) Get(n *Node) any {
	if val, ok := v.vals[n.Path]; ok {
		return val
	}
	return v.def(n)
}

func (v *Values) def(n *Node) any {
	if n.DefaultFn != nil {
		return n.DefaultFn(v)
	}
	if l, ok := n.Default.([]string); ok {
		return slices.Clone(l)
	}
	return n.Default
}

// GetPath answers the value at path; nil for an unknown path.
func (v *Values) GetPath(path string) any {
	n, ok := v.nodes.byName[path]
	if !ok {
		return nil
	}
	return v.Get(n)
}

// Bool, Int, Long, Double, String and List read a value at path.
func (v *Values) Bool(path string) bool       { b, _ := v.GetPath(path).(bool); return b }
func (v *Values) Int(path string) int         { i, _ := v.GetPath(path).(int); return i }
func (v *Values) Long(path string) int64      { i, _ := v.GetPath(path).(int64); return i }
func (v *Values) Double(path string) float64  { f, _ := v.GetPath(path).(float64); return f }
func (v *Values) String(path string) string   { s, _ := v.GetPath(path).(string); return s }
func (v *Values) List(path string) []string   { l, _ := v.GetPath(path).([]string); return l }
func (v *Values) Custom(path string) any      { return v.GetPath(path) }
func (v *Values) isSet(path string) bool      { _, ok := v.vals[path]; return ok }
func (v *Values) setRaw(path string, val any) { v.vals[path] = val }

// Set validates and stores val, then runs the node's OnChange.
func (v *Values) Set(n *Node, val any) error {
	if n.Validate != nil {
		if err := n.Validate(val); err != nil {
			return err
		}
	}
	old := v.Get(n)
	v.vals[n.Path] = val
	if n.OnChange != nil {
		n.OnChange(old, val)
	}
	return nil
}

// Reset puts the node back to its default.
func (v *Values) Reset(n *Node) error {
	return v.Set(n, v.def(n))
}

// Add appends item to a list node, Remove takes it out.
func (v *Values) Add(n *Node, item string) error {
	return v.Set(n, append(slices.Clone(v.List(n.Path)), item))
}

// Remove takes item out of a list node; it fails when the list lacks it.
func (v *Values) Remove(n *Node, item string) error {
	l := slices.Clone(v.List(n.Path))
	i := slices.Index(l, item)
	if i < 0 {
		return fmt.Errorf("value %s not found in list", item)
	}
	return v.Set(n, slices.Delete(l, i, i+1))
}

// ParseString reads input as a value of n. MV: NodeStringParser
func ParseString(n *Node, input string) (any, error) {
	if n.Parse != nil {
		return n.Parse(input)
	}
	switch n.Kind {
	case KindBool:
		switch strings.ToLower(input) {
		case "t", "true", "on", "y", "yes", "1", "allow":
			return true, nil
		case "f", "false", "off", "n", "no", "0", "deny":
			return false, nil
		}
		return nil, MsgError(M("mv-core.config.string.parser.invalidboolean", "{input}", input))
	case KindInt:
		i, err := parseJavaInt(input, 32)
		if err != nil {
			return nil, MsgError(M("mv-core.config.string.parser.invalidinteger", "{input}", input))
		}
		return int(i), nil
	case KindLong:
		i, err := parseJavaInt(input, 64)
		if err != nil {
			return nil, MsgError(M("mv-core.config.string.parser.invalidlong", "{input}", input))
		}
		return i, nil
	case KindDouble:
		f, err := strconv.ParseFloat(strings.ReplaceAll(input, ",", ""), 64)
		if err != nil {
			return nil, MsgError(M("mv-core.config.string.parser.invaliddouble", "{input}", input))
		}
		return f, nil
	case KindEnum:
		s := strings.ToLower(input)
		if !slices.Contains(n.Enum, s) {
			return nil, MsgError(Lit("No enum constant " + strings.ToUpper(input)))
		}
		return s, nil
	case KindList:
		return []string{input}, nil
	}
	return input, nil
}

// parseJavaInt parses as ACFUtil.parseInt does: commas ignored, a decimal
// part dropped.
func parseJavaInt(input string, bits int) (int64, error) {
	s := strings.ReplaceAll(input, ",", "")
	if whole, _, ok := strings.Cut(s, "."); ok {
		s = whole
	}
	return strconv.ParseInt(s, 10, bits)
}

// SuggestValues completes a value of n.
func SuggestValues(n *Node, input string) []string {
	if n.Suggest != nil {
		return n.Suggest(input)
	}
	switch n.Kind {
	case KindBool:
		return []string{"true", "false"}
	case KindEnum:
		return slices.Clone(n.Enum)
	}
	return nil
}

// FormatValue writes a value as Multiverse shows it: Java's String.valueOf.
func FormatValue(val any) string {
	switch v := val.(type) {
	case nil:
		return "null"
	case float64:
		return JavaDouble(v)
	case []string:
		return "[" + strings.Join(v, ", ") + "]"
	case fmt.Stringer:
		return v.String()
	}
	return fmt.Sprint(val)
}

// JavaDouble formats f as Java's Double.toString does: always with a
// decimal point, and as 1.5E7 below 10^-3 and from 10^7.
func JavaDouble(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if a := math.Abs(f); a != 0 && (a < 1e-3 || a >= 1e7) {
		mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'E', -1, 64), "E")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		e, _ := strconv.Atoi(exp)
		return mant + "E" + strconv.Itoa(e)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// decodeValue converts a value read from YAML to n's kind, falling back to
// the default when it does not fit. MV: NodeSerializer.deserialize
func decodeValue(n *Node, raw any) (any, bool) {
	if raw == nil {
		return nil, false
	}
	switch n.Kind {
	case KindCustom:
		if n.Decode == nil {
			return raw, true
		}
		v := n.Decode(raw)
		return v, v != nil
	case KindList:
		l, ok := raw.([]any)
		if !ok {
			return nil, false
		}
		out := make([]string, 0, len(l))
		for _, e := range l {
			out = append(out, fmt.Sprint(e))
		}
		return out, true
	}
	v, err := ParseString(n, scalarString(raw))
	if err != nil {
		return nil, false
	}
	return v, true
}

func scalarString(raw any) string {
	if f, ok := raw.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(raw)
}

// encodeValue converts a value to what is written to YAML.
func encodeValue(n *Node, val any) any {
	if n.Kind == KindCustom && n.Encode != nil {
		return n.Encode(val)
	}
	if n.Kind == KindList {
		if l, ok := val.([]string); ok && l == nil {
			return []string{}
		}
	}
	return val
}

// Load reads the values from a decoded YAML mapping; a key no node reads is
// kept for the next save.
func (v *Values) Load(m map[string]any) {
	v.vals = map[string]any{}
	v.extra = map[string]any{}
	for _, n := range v.nodes.list {
		if n.Kind == KindHeader {
			continue
		}
		raw, ok := lookupPath(m, n.Path)
		if !ok {
			continue
		}
		if val, ok := decodeValue(n, raw); ok {
			v.vals[n.Path] = val
		}
	}
	collectExtra(m, "", v.nodes, v.extra)
}

// collectExtra finds the leaf keys no node covers.
func collectExtra(m map[string]any, prefix string, nodes *Nodes, out map[string]any) {
	for k, val := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if n, ok := nodes.byName[path]; ok && n.Path == path {
			continue
		}
		if sub, ok := val.(map[string]any); ok && nodes.hasPrefix(path+".") {
			collectExtra(sub, path, nodes, out)
			continue
		}
		out[path] = val
	}
}

func (ns *Nodes) hasPrefix(prefix string) bool {
	for _, n := range ns.list {
		if strings.HasPrefix(n.Path, prefix) {
			return true
		}
	}
	return false
}

func lookupPath(m map[string]any, path string) (any, bool) {
	cur := any(m)
	for part := range strings.SplitSeq(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = mm[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// YAML builds the mapping node to save: every node in order with its
// value (defaults included, as Multiverse writes them), its comments when
// comments is set, and the keys no node reads at the end.
func (v *Values) YAML(comments bool) *yaml.Node {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, n := range v.nodes.list {
		parent, key := ensureSection(root, n.Path, n.Kind == KindHeader)
		var head string
		if comments && len(n.Comments) > 0 {
			head = strings.Join(n.Comments, "\n")
		}
		if n.Kind == KindHeader {
			if head != "" {
				key.HeadComment = head
			}
			continue
		}
		val, err := encodeYAML(encodeValue(n, v.Get(n)))
		if err != nil {
			continue
		}
		k := &yaml.Node{Kind: yaml.ScalarNode, Value: lastPart(n.Path), HeadComment: head}
		parent.Content = append(parent.Content, k, val)
	}
	for path, raw := range v.extra {
		parent, _ := ensureSection(root, path, false)
		if val, err := encodeYAML(raw); err == nil {
			parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: lastPart(path)}, val)
		}
	}
	return root
}

func lastPart(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// ensureSection answers the mapping that holds path's last key, creating the
// sections before it; with header set it creates path itself as a section
// and answers its key node too.
func ensureSection(root *yaml.Node, path string, header bool) (*yaml.Node, *yaml.Node) {
	parts := strings.Split(path, ".")
	if !header {
		parts = parts[:len(parts)-1]
	}
	cur := root
	var key *yaml.Node
	for _, p := range parts {
		var next *yaml.Node
		for i := 0; i+1 < len(cur.Content); i += 2 {
			if cur.Content[i].Value == p {
				key, next = cur.Content[i], cur.Content[i+1]
				break
			}
		}
		if next == nil {
			key = &yaml.Node{Kind: yaml.ScalarNode, Value: p}
			next = &yaml.Node{Kind: yaml.MappingNode}
			cur.Content = append(cur.Content, key, next)
		}
		cur = next
	}
	return cur, key
}

// encodeYAML converts a value to a YAML node, writing a float64 as Java
// does (1.0, not 1) and passing a prepared node through.
func encodeYAML(v any) (*yaml.Node, error) {
	switch x := v.(type) {
	case *yaml.Node:
		return x, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: JavaDouble(x)}, nil
	}
	n := &yaml.Node{}
	err := n.Encode(v)
	return n, err
}
