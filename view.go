package main

import (
	"github.com/gdamore/tcell/v2"
)

type View interface {
	SetPos(x, y, width, height int)
	Pos() (x1, y1, width, height int)
	Draw()
	FixedSize() bool
	// HandleKey is used to operate inside a view.
	// When interacting with multiple views, use [baseView.Handle] instead.
	HandleKey(*tcell.EventKey)
	OnFocus()
	OnBlur()
	OnClick(x, y int)
}

type baseView struct {
	x, y          int
	width, height int
	fixedSize     bool
	focused       bool
	keymap        map[tcell.Key]func(*tcell.EventKey)
}

func (v *baseView) SetPos(x, y, width, height int) {
	v.x = x
	v.y = y
	v.width = width
	v.height = height
}

func (v *baseView) Pos() (int, int, int, int) { return v.x, v.y, v.width, v.height }
func (v *baseView) FixedSize() bool           { return v.fixedSize }
func (v *baseView) OnFocus()                  { v.focused = true }
func (v *baseView) OnBlur()                   { v.focused = false }
func (v *baseView) Focused() bool             { return v.focused }

// TODO: focus
func (v *baseView) OnClick(int, int) {}

// Handle register callback function for the given key,
// it is intended to be used for interaction between multiple views.
func (v *baseView) Handle(k tcell.Key, f func(*tcell.EventKey)) {
	if v.keymap == nil {
		v.keymap = make(map[tcell.Key]func(*tcell.EventKey))
	}
	if _, ok := v.keymap[k]; ok {
		panic("repeated key handler")
	}
	v.keymap[k] = f
}

func (v *baseView) HandleKey(k *tcell.EventKey) {
	if v.keymap == nil {
		return
	}
	cb, ok := v.keymap[k.Key()]
	if ok {
		cb(k)
	}
}

type vstack struct {
	baseView
	Views []View
}

func VStack(v ...View) *vstack {
	return &vstack{Views: v}
}

func (v *vstack) OnClick(x, y int) {
	for _, view := range v.Views {
		if inView(view, x, y) {
			view.OnClick(x, y)
			return
		}
	}
}

func (v *vstack) Draw() {
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
		view.Draw()
	}
}

type hstack struct {
	baseView
	Views []View
}

func HStack(v ...View) *hstack {
	return &hstack{Views: v}
}

func (h *hstack) Draw() {
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
		view.Draw()
	}
}

func (h *hstack) OnClick(x, y int) {
	for _, v := range h.Views {
		if inView(v, x, y) {
			v.OnClick(x, y)
			return
		}
	}
}

func inView(v View, x, y int) bool {
	x1, y1, w, h := v.Pos()
	return x1 <= x && x < x1+w && y1 <= y && y < y1+h
}
