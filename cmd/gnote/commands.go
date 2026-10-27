//go:build windows

package main

// commandID 应用命令的唯一标识。菜单、工具栏、右键菜单与快捷键都通过它分发，
// 避免同一个动作在多处重复实现、状态各说各话。
type commandID string

const (
	cmdFileNew    commandID = "file.new"
	cmdFileOpen   commandID = "file.open"
	cmdFileSave   commandID = "file.save"
	cmdFileSaveAs commandID = "file.saveAs"
	cmdFileQuit   commandID = "file.quit"

	cmdEditUndo      commandID = "edit.undo"
	cmdEditRedo      commandID = "edit.redo"
	cmdEditCut       commandID = "edit.cut"
	cmdEditCopy      commandID = "edit.copy"
	cmdEditPaste     commandID = "edit.paste"
	cmdEditSelectAll commandID = "edit.selectAll"
	cmdEditFind      commandID = "edit.find"
	cmdEditReplace   commandID = "edit.replace"
	cmdEditFormat    commandID = "edit.format"

	cmdViewLineNumbers commandID = "view.lineNumbers"
	cmdViewWordWrap    commandID = "view.wordWrap"
	cmdViewHighlight   commandID = "view.highlight"
	cmdViewZoomIn      commandID = "view.zoomIn"
	cmdViewZoomOut     commandID = "view.zoomOut"
	cmdViewZoomReset   commandID = "view.zoomReset"
	cmdViewTheme       commandID = "view.theme"

	cmdHelpShortcuts commandID = "help.shortcuts"
	cmdHelpAbout     commandID = "help.about"

	cmdAppSettings   commandID = "app.settings"
	cmdAppOpenConfig commandID = "app.openConfig"
)

// themeCommandID 返回某个内置主题对应的命令标识。
func themeCommandID(themeID string) commandID {
	return cmdViewTheme + commandID("."+themeID)
}

// Command 一条可由菜单/工具栏/快捷键执行的命令。
// Enabled/Checked 在每次打开菜单或查询状态时求值，保证撤销栈、选区、
// 主题等状态不会缓存在另一份 UI 字段里。
type Command struct {
	ID      commandID
	Title   string
	Accel   string
	Run     func()
	Enabled func() bool
	Checked func() bool
}

func (c *Command) isEnabled() bool {
	return c != nil && (c.Enabled == nil || c.Enabled())
}

func (c *Command) isChecked() bool {
	return c != nil && c.Checked != nil && c.Checked()
}

// commandRegistry 命令注册表。注册顺序即未来菜单/快捷键文档的展示顺序。
type commandRegistry struct {
	order []*Command
	byID  map[commandID]*Command
}

func newCommandRegistry() *commandRegistry {
	return &commandRegistry{byID: make(map[commandID]*Command)}
}

func (r *commandRegistry) Register(c *Command) {
	if c == nil || c.ID == "" {
		panic("command registry: nil command or empty ID")
	}
	if _, exists := r.byID[c.ID]; exists {
		panic("command registry: duplicate command ID " + string(c.ID))
	}
	r.order = append(r.order, c)
	r.byID[c.ID] = c
}

func (r *commandRegistry) Get(id commandID) *Command {
	if r == nil {
		return nil
	}
	return r.byID[id]
}

// Run 执行命令；命令不存在或被禁用时返回 false。
func (r *commandRegistry) Run(id commandID) bool {
	c := r.Get(id)
	if c == nil || !c.isEnabled() {
		return false
	}
	c.Run()
	return true
}

func (r *commandRegistry) Enabled(id commandID) bool {
	return r.Get(id).isEnabled()
}

func (r *commandRegistry) Checked(id commandID) bool {
	return r.Get(id).isChecked()
}
