package main

import (
	"github.com/gdamore/tcell/v2"
)

type View interface {
	SetPos(x, y, width, height int)
	Pos() (x1, y1, width, height int)
	Draw()
	HandleEvent(tcell.Event)
	ShowCursor()
	LostFocus()
	Fixed() bool
}

type vstack struct {
	x, y          int
	width, height int
	Views         []View
}

func VStack(v ...View) *vstack {
	return &vstack{Views: v}
}

func (v *vstack) SetPos(x, y, width, height int) {
	v.x = x
	v.y = y
	v.width = width
	v.height = height
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
		if view.Fixed() {
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
		if view.Fixed() {
			view.SetPos(v.x, y, v.width, h)
			y += h
		} else {
			view.SetPos(v.x, y, v.width, avgH)
			y += avgH
		}
		view.Draw()
	}
}

func (v *vstack) Pos() (x1, y1, x2, y2 int) {
	return v.x, v.y, v.x + v.width - 1, v.y + v.height - 1
}
func (v *vstack) HandleEvent(tcell.Event) {}
func (v *vstack) ShowCursor()             {}
func (v *vstack) LostFocus()              {}

type hstack struct {
	x, y          int
	width, height int
	Views         []View
}

func HStack(v ...View) *hstack {
	return &hstack{Views: v}
}

func (h *hstack) SetPos(x, y, width, height int) {
	h.x = x
	h.y = y
	h.width = width
	h.height = height
}

func (h *hstack) Render() {
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
		if view.Fixed() {
			fixed++
			remainW -= w
		}
	}
	avgW := remainW / (len(h.Views) - fixed)

	x := h.x
	for _, view := range h.Views {
		_, _, w, _ := view.Pos()
		if view.Fixed() {
			view.SetPos(x, h.y, w, h.height)
			x += w
		} else {
			view.SetPos(x, h.y, avgW, h.height)
			x += avgW
		}
		view.Draw()
	}
}

func (h *hstack) Pos() (x1, y1, x2, y2 int) { return h.x, h.y, h.x + h.width - 1, h.y + h.height - 1 }
func (h *hstack) HandleEvent(tcell.Event)   {}
func (h *hstack) ShowCursor()               {}
func (h *hstack) LostFocus()                {}
