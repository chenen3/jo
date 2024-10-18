// Framework stuff
package main

import "github.com/gdamore/tcell/v2"

type App struct {
	body View

	screen tcell.Screen
	focus  View
	done   chan struct{}
	mouseX int
	mouseY int
	keymap map[tcell.Key]func(*tcell.EventKey)
}

func NewApp() (*App, error) {
	a := &App{
		done:   make(chan struct{}),
		keymap: make(map[tcell.Key]func(*tcell.EventKey)),
	}

	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err = s.Init(); err != nil {
		return nil, err
	}
	s.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	s.EnableMouse()
	s.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	s.EnablePaste()
	s.Clear()
	a.screen = s
	return a, nil
}

func (a *App) Screen() tcell.Screen { return a.screen }

func (a *App) SetBody(v View) {
	width, height := a.screen.Size()
	v.SetPos(0, 0, width, height)
	a.body = v
}

func (a *App) Redraw() {
	a.body.Draw(a.screen)
}

// Release resources and stop Run
func (a *App) Close() {
	close(a.done)
	a.screen.Fini()
}

// Focus focus on view and show cursor,
// while blurs previous focused view, if any.
func (a *App) Focus(v View) {
	cursorX, cursorY := v.Focus()
	a.screen.ShowCursor(cursorX, cursorY)

	if a.focus == v {
		return
	}
	if a.focus != nil {
		a.focus.Blur()
	}
	a.focus = v
}

func (a *App) GetHover() View {
	return getHover(a.body, a.mouseX, a.mouseY)
}

func getHover(view View, x, y int) View {
	if !inView(view, x, y) {
		return nil
	}

	if s, ok := view.(*vstack); ok {
		for _, v := range s.Views {
			hover := getHover(v, x, y)
			if hover != nil {
				return hover
			}
		}
	}
	if s, ok := view.(*hstack); ok {
		for _, v := range s.Views {
			hover := getHover(v, x, y)
			if hover != nil {
				return hover
			}
		}
	}
	return view
}

func (a *App) Handle(key tcell.Key, f func(*tcell.EventKey)) {
	a.keymap[key] = f
}

// Run will not stop until Close
func (a *App) Run() {
	a.body.Draw(a.screen)
	for {
		select {
		case <-a.done:
			return
		default:
		}
		a.screen.Show()

		ev := a.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := a.screen.Size()
			a.body.SetPos(0, 0, width, height)
			a.body.Draw(a.screen)
			a.screen.Sync()
			continue
		case *tcell.EventMouse:
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1:
				view := a.GetHover()
				view.Click(x, y)
				a.Focus(view)
			case tcell.WheelUp:
				view := a.GetHover()
				delta := int(float32(y) * scrollSensitivity)
				if view.ScrollUp(delta) {
					view.Draw(a.screen)
				}
			case tcell.WheelDown:
				view := a.GetHover()
				delta := int(float32(y) * scrollSensitivity)
				if view.ScrollDown(delta) {
					view.Draw(a.screen)
				}
			default:
				a.mouseX = x
				a.mouseY = y
				// do not render on mouse motion
				continue
			}
		case *tcell.EventKey:
			if f, ok := a.keymap[ev.Key()]; ok {
				f(ev)
			} else {
				a.focus.HandleEventKey(ev, a.screen)
			}
		}
	}
}

type View interface {
	SetPos(x, y, width, height int)
	Pos() (x1, y1, width, height int)
	Draw(screen tcell.Screen)
	FixedSize() bool

	// HandleEventKey is used to operate inside a view.
	// When interacting with multiple views, use [baseView.Handle] instead.
	HandleEventKey(*tcell.EventKey, tcell.Screen)
	Focus() (cursorX, cursorY int)
	Blur()

	Click(x, y int)
	ScrollUp(delta int) (ok bool)
	ScrollDown(delta int) (ok bool)
}

type BaseView struct {
	x, y      int
	width     int
	height    int
	fixedSize bool
	focused   bool
	cursorX   int
	cursorY   int
	keymap    map[tcell.Key]func(*tcell.EventKey, tcell.Screen)
	onClick   func()
}

func (v *BaseView) SetPos(x, y, width, height int) {
	v.x = x
	v.y = y
	v.width = width
	v.height = height
}

func (v *BaseView) Pos() (int, int, int, int) { return v.x, v.y, v.width, v.height }
func (v *BaseView) FixedSize() bool           { return v.fixedSize }

func (v *BaseView) Focus() (int, int) {
	v.focused = true
	return v.cursorX, v.cursorY
}

func (v *BaseView) Blur()         { v.focused = false }
func (v *BaseView) Focused() bool { return v.focused }

// OnClick register the callback function that will be call on click
func (v *BaseView) OnClick(f func()) {
	v.onClick = f
}

func (v *BaseView) Click(x, y int) {
	if v.onClick != nil {
		v.onClick()
	}
}

func (v *BaseView) ScrollUp(delta int) bool   { return false }
func (v *BaseView) ScrollDown(delta int) bool { return false }

// Handle register callback function for the given key,
// it is intended to be used for interaction between multiple views.
func (v *BaseView) Handle(k tcell.Key, f func(*tcell.EventKey, tcell.Screen)) {
	if v.keymap == nil {
		v.keymap = make(map[tcell.Key]func(*tcell.EventKey, tcell.Screen))
	}
	if _, ok := v.keymap[k]; ok {
		panic("repeated key handler")
	}
	v.keymap[k] = f
}

func (v *BaseView) HandleEventKey(k *tcell.EventKey, screen tcell.Screen) {
	if v.keymap == nil {
		return
	}
	cb, ok := v.keymap[k.Key()]
	if ok {
		cb(k, screen)
	}
}

type vstack struct {
	BaseView
	Views []View
}

// VStack arranges views vertically
func VStack(v ...View) *vstack {
	return &vstack{Views: v}
}

func (v *vstack) Click(x, y int) {
	for _, view := range v.Views {
		if inView(view, x, y) {
			view.Click(x, y)
			return
		}
	}
}

func (v *vstack) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := v.y; y < v.y+v.height; y++ {
		for x := v.x; x < v.x+v.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(v.Views) == 0 {
		return
	}

	var fixed int
	var remainH = v.height
	for _, view := range v.Views {
		_, _, _, h := view.Pos()
		if view.FixedSize() {
			fixed++
			remainH -= h
		}
	}
	var avgH int
	if fixed != len(v.Views) {
		avgH = remainH / (len(v.Views) - fixed)
	}

	y := v.y
	for _, view := range v.Views {
		_, _, _, h := view.Pos()
		if view.FixedSize() {
			view.SetPos(v.x, y, v.width, h)
			y += h
		} else {
			view.SetPos(v.x, y, v.width, avgH)
			y += avgH
		}
		view.Draw(screen)
	}
}

type hstack struct {
	BaseView
	Views []View
}

// HStack arranges views horizontally
func HStack(v ...View) *hstack {
	return &hstack{Views: v}
}

func (h *hstack) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := h.y; y < h.y+h.height; y++ {
		for x := h.x; x < h.x+h.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(h.Views) == 0 {
		return
	}

	var fixed int
	var remainW = h.width
	for _, view := range h.Views {
		_, _, w, _ := view.Pos()
		if view.FixedSize() {
			fixed++
			remainW -= w
		}
	}
	avgW := remainW / (len(h.Views) - fixed)

	x := h.x
	for _, view := range h.Views {
		_, _, w, _ := view.Pos()
		if view.FixedSize() {
			view.SetPos(x, h.y, w, h.height)
			x += w
		} else {
			view.SetPos(x, h.y, avgW, h.height)
			x += avgW
		}
		view.Draw(screen)
	}
}

func (h *hstack) Click(x, y int) {
	for _, v := range h.Views {
		if inView(v, x, y) {
			v.Click(x, y)
			return
		}
	}
}

func inView(v View, x, y int) bool {
	x1, y1, w, h := v.Pos()
	return x1 <= x && x < x1+w && y1 <= y && y < y1+h
}
