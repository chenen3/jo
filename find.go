package main

import "github.com/gdamore/tcell/v2"

type findBar struct {
	s       tcell.Screen
	x1, y1  int
	x2, y2  int
	keyword []rune
	match   [][2]int // [][2]int{lineIndex, columnIndex}
	i       int      // index of match
	cx, cy  int      // cursor position
}

func newFindBar(s tcell.Screen) *findBar {
	return &findBar{s: s}
}

var findBarStyle = tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)

func (f *findBar) draw() {
	width, _ := f.s.Size()
	// align right
	f.x1, f.y1 = width-20, 1
	f.x2, f.y2 = width-1, 1
	for y := f.y1; y <= f.y2; y++ {
		for x := f.x1; x <= f.x2; x++ {
			f.s.SetContent(x, y, ' ', nil, findBarStyle)
		}
	}
	for i := range f.keyword {
		f.s.SetContent(f.x1+i, f.y1, f.keyword[i], nil, findBarStyle)
	}
	f.cx, f.cy = f.x1+len(f.keyword), f.y1
	f.s.ShowCursor(f.cx, f.cy)
}

func (f *findBar) insert(r rune) {
	f.keyword = append(f.keyword, r)
	f.s.SetContent(f.cx, f.cy, r, nil, findBarStyle)
	f.cx++
	f.s.ShowCursor(f.cx, f.cy)
}

func (f *findBar) deleteLeft() {
	if len(f.keyword) == 0 {
		return
	}
	f.keyword = f.keyword[:len(f.keyword)-1]
	f.draw()
}

func (f *findBar) next() (int, int) {
	if f.i == len(f.match)-1 {
		f.i = 0
	} else {
		f.i++
	}
	return f.match[f.i][0], f.match[f.i][1]
}

func (f *findBar) prev() (int, int) {
	if f.i == 0 {
		f.i = len(f.match) - 1
	} else {
		f.i--
	}
	return f.match[f.i][0], f.match[f.i][1]
}
