package main

import "github.com/gdamore/tcell/v2"

type findBar struct {
	s        tcell.Screen
	row, col int // cursor position when starting search

	x1, y1  int
	x2, y2  int
	keyword []rune
	match   [][2]int // [][2]int{lineIndex, columnIndex}
	i       int      // index of the matching result
	cx, cy  int      // cursor position
}

func newFindBar(s tcell.Screen, row, col int) *findBar {
	return &findBar{s: s, row: row, col: col}
}

var findBarStyle = tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)

func (f *findBar) draw() {
	width, _ := f.s.Size()
	// align right
	f.x1, f.y1 = width-30, 1
	f.x2, f.y2 = width-1, 1
	for y := f.y1; y <= f.y2; y++ {
		for x := f.x1; x <= f.x2; x++ {
			f.s.SetContent(x, y, ' ', nil, findBarStyle)
		}
	}

	s := "search:" + string(f.keyword)
	for i, c := range s {
		f.s.SetContent(f.x1+i, f.y1, c, nil, findBarStyle)
	}
	f.cx, f.cy = f.x1+len(s), f.y1
	f.s.ShowCursor(f.cx, f.cy)
}

func (f *findBar) insert(r rune) {
	f.keyword = append(f.keyword, r)
}

func (f *findBar) deleteLeft() {
	if len(f.keyword) == 0 {
		return
	}
	f.keyword = f.keyword[:len(f.keyword)-1]
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
