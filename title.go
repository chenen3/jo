package main

import (
	"github.com/gdamore/tcell/v2"
)

type titleBar struct {
	BaseView
	names []string
	i     int
}

func newTitleBar(name string) *titleBar {
	t := &titleBar{}
	t.height = 1
	if name != "" {
		t.names = append(t.names, name)
	}
	return t
}

func (t *titleBar) Click(x, y int) {
	t.BaseView.Click(x, y)
	start := t.x
	for i := range t.names {
		end := start + len(t.names[i]) + len(" |")
		if start <= x && x <= end {
			t.i = i
			return
		}
		start = end + 1
	}
}

func (t *titleBar) FixedSize() bool { return true }

func (t *titleBar) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := t.y; y < t.y+t.height; y++ {
		for x := t.x; x < t.x+t.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(t.names) == 0 {
		for i, c := range "untitled" {
			screen.SetContent(t.x+i, t.y, c, nil, style)
		}
		return
	}

	var i int
	for j, name := range t.names {
		newstyle := style
		if j == t.i {
			newstyle = newstyle.Background(tcell.ColorLightGray).Italic(true)
		}
		for _, c := range name {
			screen.SetContent(t.x+i, t.y, c, nil, newstyle)
			i++
		}
		if j != len(t.names)-1 {
			screen.SetContent(t.x+i, t.y, ' ', nil, style)
			screen.SetContent(t.x+i+1, t.y, '|', nil, style)
			screen.SetContent(t.x+i+2, t.y, ' ', nil, style)
			i += 3
		}
	}
}

func (t *titleBar) Add(s string) {
	for i, name := range t.names {
		if name == s {
			t.i = i
			return
		}
	}
	t.names = append(t.names, s)
	t.i = len(t.names) - 1
}

// delete current name
func (t *titleBar) Del() {
	if len(t.names) == 0 {
		return
	}
	if len(t.names) == 1 {
		t.names = nil
		return
	}

	if t.i == len(t.names)-1 {
		t.names = t.names[:len(t.names)-1]
	} else {
		t.names = append(t.names[:t.i], t.names[t.i+1:]...)
	}
	if t.i > len(t.names)-1 {
		t.i = len(t.names) - 1
	}
}
