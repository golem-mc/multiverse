package inventories

import (
	"fmt"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// The menu's slots: 0 to 35 the inventory, then the helmet, chestplate,
// leggings, boots and off hand, then the four stat displays.
// MV: InventoryGUIHelper.populateInventoryGUI
const (
	menuHelmet  = 36
	menuOffHand = 40
	menuSize    = 45
)

// readOnlyMenu and modifyMenu hold /mvinv view's and modify's menus.
// MV: ReadOnlyInventoryHolder, ModifiableInventoryHolder
type readOnlyMenu struct{}

type modifyMenu struct {
	target PlayerRef
	world  string
	typ    ProfileType
}

// inventoryData is what a menu shows. MV: PlayerInventoryData
type inventoryData struct {
	live     bool
	typ      ProfileType
	contents []plugin.ItemStack
	// armor is boots, leggings, chestplate, helmet.
	armor   []plugin.ItemStack
	offHand plugin.ItemStack
	// The stats are nil with no data.
	health, maxHealth, saturation, exp *float64
	level, food                        *int
	lastLocation                       string
}

var (
	fillerKey  = plugin.Key{Namespace: "multiverse-inventories", Path: "is_mvinv_filler"}
	displayKey = plugin.Key{Namespace: "multiverse-inventories", Path: "is_mvinv_display"}
)

// viewCommands are /mvinv group, view and modify. MV: GroupCommand,
// InventoryViewCommand, InventoryModifyCommand
func (m *Module) viewCommands() []*core.Cmd {
	complete := func(ctx *core.Ctx, pos int) []string {
		if pos == 0 {
			return m.suggestPlayers(lastWord(ctx))
		}
		return m.Core.WorldNames("both")
	}
	return []*core.Cmd{{
		Name: "group", Legacy: []string{"mvinvgroup", "mvinvg"}, Permission: "multiverse.inventories.group",
		Description: "Manage a world group with prompts!",
		Run: func(ctx *core.Ctx) error {
			p, ok := ctx.Player()
			if !ok {
				ctx.Error(core.M("mv-inventories.group.nonconversable"))
				return nil
			}
			m.conversations.begin(ctx.Issuer, p)
			return nil
		},
	}, {
		Name: "view", Permission: "multiverse.inventories.view", Syntax: "<player> <world>",
		Description: "View a player's inventory in a specific world.", MinArgs: 2, MaxArgs: 2, PlayerOnly: true,
		Complete: complete, Run: func(ctx *core.Ctx) error { return m.openMenu(ctx, false) },
	}, {
		Name: "modify", Permission: "multiverse.inventories.view.modify", Syntax: "<player> <world>",
		Description: "Modify a player's inventory in a specific world.", MinArgs: 2, MaxArgs: 2, PlayerOnly: true,
		Complete: complete, Run: func(ctx *core.Ctx) error { return m.openMenu(ctx, true) },
	}}
}

// openMenu shows a player's inventory in a world in a menu, which with
// modify saves into it when closed. MV: InventoryViewCommand, InventoryModifyCommand
func (m *Module) openMenu(ctx *core.Ctx, modify bool) error {
	viewer, _ := ctx.Player()
	target, err := m.offlinePlayer(ctx.Arg(0))
	if err != nil {
		return err
	}
	w, err := ctx.RequireWorld(1, false)
	if err != nil {
		return err
	}
	world := w.Name()
	ctx.Info(core.M("mv-inventories.inventory.loading", "{player}", target.Name()))
	data, err := m.inventoryData(target, world)
	if err != nil {
		ctx.Error(core.M("mv-inventories.inventory.loadfailed", "{error}", err))
		return nil
	}
	title := target.Name() + " @ " + world
	var holder any = readOnlyMenu{}
	if modify {
		if target.UUID() == viewer.UUID() && strings.EqualFold(m.worldName(viewer.World()), world) {
			ctx.Error(core.M("mv-inventories.inventory.modifyliveself"))
			return nil
		}
		title = "Modify " + title
		holder = &modifyMenu{target: target, world: world, typ: data.typ}
	}
	inv := m.Server.CreateInventory(plugin.InvChest, menuSize/9, plugin.Plain(title), holder)
	m.fillMenu(inv, data, modify)
	viewer.OpenInventory(inv)
	status := "mv-inventories.inventory.storedinventory"
	if data.live {
		status = "mv-inventories.inventory.liveinventory"
	}
	ctx.Info(core.M("mv-inventories.inventory.opened", "{status}", core.M(status, "{player}", target.Name(), "{world}", world)))
	return nil
}

func locationString(world string, x, y, z float64) string {
	return fmt.Sprintf("%s (%.1f, %.1f, %.1f)", world, x, y, z)
}

// inventoryData reads the player's live inventory when they are in the
// world, else the stored one of their first profile type with data.
// MV: InventoryDataProvider.loadPlayerInventoryData
func (m *Module) inventoryData(target PlayerRef, world string) (*inventoryData, error) {
	if p, ok := m.Server.Player(target.UUID()); ok && strings.EqualFold(m.worldName(p.World()), world) {
		health, maxHealth := float64(p.Health()), defaultHealth
		if a, ok := maxHealthAttribute(p); ok {
			maxHealth = a.Value()
		}
		exp, sat := float64(p.Progress()), float64(p.Saturation())
		level, food := p.Level(), p.Food()
		loc := p.Location()
		return &inventoryData{live: true, typ: m.profileType(p),
			contents: slotsOf(p.Inventory(), 0, inventorySize), armor: slotsOf(p.Inventory(), inventorySize, armorSize),
			offHand: p.Inventory().Item(offHandSlot), health: &health, maxHealth: &maxHealth, exp: &exp,
			saturation: &sat, level: &level, food: &food,
			lastLocation: locationString(m.worldName(loc.World), loc.Pos.X, loc.Pos.Y, loc.Pos.Z)}, nil
	}
	typ := ProfileType("")
	for _, t := range ProfileTypes() {
		if !m.store.Profile(key(ContainerWorld, world, t, target)).Empty() {
			typ = t
			break
		}
	}
	if typ == "" {
		return nil, core.MsgError(core.M("mv-inventories.inventory.nodatafound", "{player}", target.Name(), "{world}", world))
	}
	read := func(s *Sharable) any { return m.ReadShare(target, world, typ, s) }
	d := &inventoryData{typ: typ}
	d.contents, _ = read(Inventory).([]plugin.ItemStack)
	d.armor, _ = read(Armor).([]plugin.ItemStack)
	d.offHand, _ = read(OffHand).(plugin.ItemStack)
	float := func(s *Sharable) *float64 {
		switch v := read(s).(type) {
		case float64:
			return &v
		case float32:
			f := float64(v)
			return &f
		}
		return nil
	}
	integer := func(s *Sharable) *int {
		if v, ok := read(s).(int); ok {
			return &v
		}
		return nil
	}
	d.health, d.maxHealth, d.exp, d.saturation = float(Health), float(MaxHealth), float(Experience), float(Saturation)
	d.level, d.food = integer(Level), integer(FoodLevel)
	switch loc, _ := read(LastLocation).(*Loc); {
	case !m.activeOptionalShares().Has(LastLocation):
		d.lastLocation = "§cDisabled in Config"
	case loc == nil || loc.World == "":
		d.lastLocation = "N/A (No Location Data)"
	default:
		d.lastLocation = locationString(loc.World, loc.X, loc.Y, loc.Z)
	}
	if _, online := m.Server.Player(target.UUID()); !online {
		m.store.forgetPlayer(target.UUID())
	}
	return d, nil
}

// menuItem answers a named item marked with key, with legacy colour codes
// in its name and lore. MV: InventoryGUIHelper.createFillerItem, createDisplayItem
func menuItem(item registry.Item, mark plugin.Key, name string, lore ...string) plugin.ItemStack {
	s := plugin.NewStack(item, 1)
	n := core.Legacy(core.Colorize(name))
	s.Patch.CustomName = &n
	for _, l := range lore {
		s.Patch.Lore = append(s.Patch.Lore, core.Legacy(core.Colorize(l)))
	}
	s.Patch.Data = map[plugin.Key]any{mark: int8(1)}
	return s
}

func isFiller(s plugin.ItemStack) bool { _, ok := s.Patch.Data[fillerKey]; return ok }

// fillerFor answers the placeholder of an empty armour or off hand slot.
// MV: InventoryGUIHelper.createFillerItemForSlot
func fillerFor(slot int, modify bool) plugin.ItemStack {
	names := [...]string{"Helmet", "Chestplate", "Leggings", "Boots", "Off-Hand"}
	name := names[slot-menuHelmet]
	lore := "No " + name
	switch {
	case modify && slot == menuOffHand:
		lore = "Place Off-Hand Item Here"
	case slot == menuOffHand:
		lore = "No Off-Hand Item"
	case modify:
		lore = "Place " + name + " Here"
	}
	return menuItem(registry.ItemGrayStainedGlassPane, fillerKey, "&6"+name+" Slot", "&7"+lore)
}

// fillMenu puts the data in the menu. MV: InventoryGUIHelper.populateInventoryGUI
func (m *Module) fillMenu(inv plugin.Inventory, d *inventoryData, modify bool) {
	for i := 0; i < len(d.contents) && i < inventorySize; i++ {
		inv.SetItem(i, d.contents[i])
	}
	for i := range armorSize {
		var s plugin.ItemStack
		if i < len(d.armor) {
			s = d.armor[armorSize-1-i]
		}
		if s.Empty() {
			s = fillerFor(menuHelmet+i, modify)
		}
		inv.SetItem(menuHelmet+i, s)
	}
	off := d.offHand
	if off.Empty() {
		off = fillerFor(menuOffHand, modify)
	}
	inv.SetItem(menuOffHand, off)
	noData := "&cN/A (No Data)"
	health := []string{noData}
	if d.health != nil {
		health = []string{fmt.Sprintf("&fCurrent: &c%.1f&f / &c%.1f", *d.health, deref(d.maxHealth))}
	}
	inv.SetItem(41, menuItem(registry.ItemRedDye, displayKey, "&bHealth", health...))
	var food []string
	if d.food == nil {
		food = append(food, noData)
	}
	food = append(food, "&fFood: &6"+intOrNull(d.food)+"&f / &620",
		fmt.Sprintf("&fSaturation: &d%.1f", deref(d.saturation)))
	inv.SetItem(42, menuItem(registry.ItemCookedBeef, displayKey, "&bFood & Saturation", food...))
	level := []string{noData}
	if d.level != nil {
		level = []string{fmt.Sprintf("&fLevel: &a%d", *d.level), fmt.Sprintf("&fProgress: &b%.1f%%", deref(d.exp)*100)}
	}
	inv.SetItem(43, menuItem(registry.ItemExperienceBottle, displayKey, "&bExperience", level...))
	inv.SetItem(44, menuItem(registry.ItemCompass, displayKey, "&bLast Location", "&fLast Location:", "&e"+d.lastLocation))
}

func deref(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func intOrNull(i *int) string {
	if i == nil {
		return "null"
	}
	return fmt.Sprint(*i)
}

// validFor reports whether a stack may go in a menu slot: armour only in
// its own slot, by its name as Multiverse-Inventories checks it.
// MV: InventoryGUIHelper.isValidItemForSlot
func validFor(s plugin.ItemStack, slot int) bool {
	if s.Empty() || isFiller(s) {
		return true
	}
	name := s.Item.Name()
	switch slot {
	case 36:
		return strings.HasSuffix(name, "_helmet")
	case 37:
		return strings.HasSuffix(name, "_chestplate")
	case 38:
		return strings.HasSuffix(name, "_leggings")
	case 39:
		return strings.HasSuffix(name, "_boots")
	case menuOffHand:
		return true
	}
	return slot < menuHelmet
}

// listenMenus keeps view's menus read-only and modify's displays and
// placeholders in place, saving modify's menu when it closes.
// MV: InventoryViewListener
func (m *Module) listenMenus() {
	holder := func(v plugin.InventoryView) any {
		if v == nil || v.Top() == nil {
			return nil
		}
		return v.Top().Holder()
	}
	refill := func(inv plugin.Inventory, slots []int) {
		m.Server.Scheduler().RunLater(1, func() {
			for _, slot := range slots {
				if slot >= menuHelmet && slot <= menuOffHand && inv.Item(slot).Empty() {
					inv.SetItem(slot, fillerFor(slot, true))
				}
			}
		})
	}
	plugin.Listen(m.Server, func(e *plugin.InventoryClickEvent) {
		switch holder(e.View).(type) {
		case readOnlyMenu:
			e.SetCancelled(true)
			return
		case *modifyMenu:
		default:
			return
		}
		slot := e.RawSlot
		switch {
		case slot > menuOffHand && slot < menuSize:
			e.SetCancelled(true)
		case slot < menuHelmet || slot >= menuSize:
		case isFiller(e.Current) && e.Cursor.Empty():
			e.SetCancelled(true)
		case !e.Cursor.Empty() && !validFor(e.Cursor, slot):
			e.SetCancelled(true)
		case !e.Cursor.Empty():
			// Placing swaps by hand so that a placeholder never reaches the cursor.
			e.SetCancelled(true)
			e.View.Top().SetItem(slot, e.Cursor)
			if !e.Current.Empty() && !isFiller(e.Current) {
				e.Player.SetCursor(e.Current)
			} else {
				e.Player.SetCursor(plugin.ItemStack{})
			}
		default:
			refill(e.View.Top(), []int{slot})
		}
	}, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(m.Server, func(e *plugin.InventoryDragEvent) {
		switch holder(e.View).(type) {
		case readOnlyMenu:
			e.SetCancelled(true)
			return
		case *modifyMenu:
		default:
			return
		}
		var slots []int
		for slot, s := range e.Slots {
			if slot > menuOffHand && slot < menuSize || slot >= menuHelmet && slot <= menuOffHand && !validFor(s, slot) {
				e.SetCancelled(true)
				return
			}
			slots = append(slots, slot)
		}
		refill(e.View.Top(), slots)
	}, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(m.Server, func(e *plugin.InventoryCloseEvent) {
		if menu, ok := holder(e.View).(*modifyMenu); ok {
			m.saveMenu(menu, e.View.Top())
		}
	}, plugin.ListenOptions{Priority: plugin.Normal})
}

// saveMenu writes a modify menu into the player's profiles and, when they
// are in that world, their live inventory; an offline player has it
// applied when they join. MV: InventoryDataProvider.savePlayerInventoryData
func (m *Module) saveMenu(menu *modifyMenu, inv plugin.Inventory) {
	contents := slotsOf(inv, 0, inventorySize)
	armor := make([]plugin.ItemStack, armorSize)
	for i := range armorSize {
		if s := inv.Item(menuHelmet + i); !isFiller(s) {
			armor[armorSize-1-i] = s
		}
	}
	off := inv.Item(menuOffHand)
	if isFiller(off) {
		off = plugin.ItemStack{}
	}
	m.writeShare(menu.target, menu.world, menu.typ, Inventory, contents, true)
	m.writeShare(menu.target, menu.world, menu.typ, Armor, armor, true)
	m.writeShare(menu.target, menu.world, menu.typ, OffHand, off, true)
	m.Log.Info("Inventory for player " + menu.target.Name() + " in world " + menu.world + " has been modified and saved.")
	p, online := m.Server.Player(menu.target.UUID())
	if !online {
		m.loadOnLogin(key(ContainerWorld, menu.world, menu.typ, menu.target))
		m.store.forgetPlayer(menu.target.UUID())
		return
	}
	if !strings.EqualFold(m.worldName(p.World()), menu.world) {
		return
	}
	setSlots(p.Inventory(), 0, inventorySize, contents)
	setSlots(p.Inventory(), inventorySize, armorSize, armor)
	p.Inventory().SetItem(offHandSlot, off)
	m.Log.Info("Updated live inventory for online player " + p.Name() + " in world " + menu.world)
}
