package main

import (
	"github.com/gdamore/tcell/v2"
)

type View interface {
	SetPos(x, y, width, height int)
	Pos() (x1, y1, x2, y2 int)
	Render()
	HandleEvent(tcell.Event)
	ShowCursor()
	LostFocus()
}

type HStack struct {
	x, y          int
	width, height int
	Views         []View
}

func (h *HStack) SetPos(x, y, width, height int) {
	h.x = x
	h.y = y
	h.width = width
	h.height = height
}

func (h *HStack) Render() {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := h.y; y < h.y+h.height; y++ {
		for x := h.x; x < h.x+h.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(h.Views) == 0 {
		return
	}

	avg := h.width / len(h.Views)
	for i := range h.Views {
		h.Views[i].SetPos(h.x+i*avg, h.y, avg, h.height)
		h.Views[i].Render()
	}
}

func (h *HStack) Pos() (x1, y1, x2, y2 int) { return h.x, h.y, h.x + h.width - 1, h.y + h.height - 1 }
func (h *HStack) HandleEvent(tcell.Event)   {}
func (h *HStack) ShowCursor()               {}
func (h *HStack) LostFocus()                {}
