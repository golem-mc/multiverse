package core

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// SpawnCategories are Bukkit's spawn categories, in its order, as worlds.yml
// names them. MV: SpawnCategory
var SpawnCategories = []string{"monster", "animal", "water_animal", "water_ambient",
	"water_underground_creature", "ambient", "axolotl", "misc"}

// Spawn values with a meaning of their own: '@unset' leaves the server's
// value alone, '@bukkit' asks for the server default. MV:
// SpawnCategoryConfig.SpawnValueSerializer
const (
	SpawnUnset  = -2
	SpawnServer = -1
)

// spawnCategoryNodes are one category's settings under spawning.<category>.
// MV: SpawnCategoryConfig.Nodes
func spawnCategoryNodes() *Nodes {
	spawnValue := func(path string) *Node {
		return &Node{Path: path, Kind: KindCustom, Default: SpawnUnset,
			Parse:   parseSpawnValue,
			Suggest: func(string) []string { return []string{"@unset", "@bukkit", "10", "100", "400", "1000"} },
			Decode: func(raw any) any {
				v, err := parseSpawnValue(fmt.Sprint(raw))
				if err != nil {
					return nil
				}
				return v
			},
			Encode: func(v any) any {
				switch v {
				case SpawnUnset:
					return "@unset"
				case SpawnServer:
					return "@bukkit"
				}
				return v
			},
		}
	}
	return NewNodes(
		&Node{Path: "exceptions", Kind: KindList, Default: []string{}},
		&Node{Path: "spawn", Kind: KindBool, Default: true},
		spawnValue("spawn-limit"),
		spawnValue("tick-rate"),
	)
}

func parseSpawnValue(input string) (any, error) {
	s := strings.TrimSpace(input)
	switch strings.ToLower(s) {
	case "@unset":
		return SpawnUnset, nil
	case "@bukkit":
		return SpawnServer, nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil, MsgError(M("mv-core.config.string.parser.invalidinteger", "{input}", input))
	}
	return i, nil
}

// Spawning is a world's spawning section: per category, whether mobs of it
// spawn, the exceptions, and the tick rate and limit. MV: EntitySpawnConfig
type Spawning struct {
	Categories map[string]*Values
}

// NewSpawning answers the default: everything spawns.
func NewSpawning() *Spawning {
	s := &Spawning{Categories: map[string]*Values{}}
	for _, c := range SpawnCategories {
		s.Categories[c] = NewValues(spawnCategoryNodes())
	}
	return s
}

// Category answers one category's settings.
func (s *Spawning) Category(name string) *Values {
	v, ok := s.Categories[name]
	if !ok {
		v = NewValues(spawnCategoryNodes())
		s.Categories[name] = v
	}
	return v
}

// Allows reports whether a mob of entityType (its registry key path, e.g.
// "zombie") in category may spawn: the category's spawn flag, flipped for
// the types in its exceptions. MV: SpawnCategoryConfig.shouldAllowSpawn
func (s *Spawning) Allows(category, entityType string) bool {
	v := s.Category(category)
	excepted := slices.ContainsFunc(v.List("exceptions"), func(e string) bool {
		return strings.EqualFold(e, entityType)
	})
	return v.Bool("spawn") != excepted
}

func decodeSpawning(raw any) *Spawning {
	s := NewSpawning()
	m, ok := raw.(map[string]any)
	if !ok {
		return s
	}
	for name, sec := range m {
		if sm, ok := sec.(map[string]any); ok {
			s.Category(strings.ToLower(name)).Load(sm)
		}
	}
	return s
}

func (s *Spawning) yaml() *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	names := slices.Clone(SpawnCategories)
	for name := range s.Categories {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range names {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, s.Category(name).YAML(false))
	}
	return n
}
