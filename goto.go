package main

import (
	"log"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
)

type gotoBar struct {
	BaseView
	keyword []rune
	index   int
	options []string
}

func (g *gotoBar) Draw(screen tcell.Screen) {
	once.Do(loadFileList)
	g.height = 1
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := g.y; y < g.y+g.height; y++ {
		for x := g.x; x < g.x+g.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(g.keyword) == 0 {
		hint := "search files by name"
		for i, c := range hint {
			screen.SetContent(g.x+i, g.y, c, nil, style.Foreground(tcell.ColorGray))
		}
	}
	for i, c := range g.keyword {
		screen.SetContent(g.x+i, g.y, c, nil, style)
	}
	g.cursorX = g.x + len(g.keyword)
	g.cursorY = g.y

	if g.Focused() {
		screen.ShowCursor(g.cursorX, g.cursorY)
	}

	if len(g.keyword) > 0 && g.keyword[0] == ':' {
		return
	}

	if len(g.keyword) == 0 {
		g.options = files
	}
	g.height += len(g.options)
	for i, name := range g.options {
		selectedStyle := style
		if i == g.index {
			selectedStyle = style.Background(tcell.ColorLightBlue)
		}
		for j, c := range name {
			screen.SetContent(g.x+j, g.y+1+i, c, nil, selectedStyle)
		}
		// padding
		for j := 0; j < optionWidth-len(name); j++ {
			screen.SetContent(g.x+len(name)+j, g.y+1+i, ' ', nil, selectedStyle)
		}
	}
}

const optionWidth = 40

var files []string
var once sync.Once

func loadFileList() {
	dirs, err := os.ReadDir(".")
	if err != nil {
		log.Print(err)
		return
	}
	for _, d := range dirs {
		if d.Name()[0] == '.' {
			continue
		}
		if d.IsDir() {
			continue
		}
		files = append(files, d.Name())
	}
}
