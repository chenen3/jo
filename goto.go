package main

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

type gotoBar struct {
	jo               *Jo
	keyword          []rune
	x1, y1           int
	x2, y2           int
	cursorX, cursorY int

	index   int
	options []string
}

func newGotoBar(j *Jo) *gotoBar {
	once.Do(initFilenames)
	return &gotoBar{jo: j}
}

func (g *gotoBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	width, height := g.jo.Size()
	g.x1, g.y1 = 0, height-1
	g.x2, g.y2 = width-1, height-1
	for y := g.y1; y <= g.y2; y++ {
		for x := g.x1; x <= g.x2; x++ {
			g.jo.SetContent(x, y, ' ', nil, style)
		}
	}
	g.cursorX = g.x1
	g.cursorY = g.y1

	if len(g.keyword) == 0 {
		placeholder := "search files by name (append : to go to line or @ to go to symbol)"
		for i, c := range placeholder {
			g.jo.SetContent(g.cursorX+i, g.y1, c, nil, style.Foreground(tcell.ColorGray))
		}
	}
	for _, c := range g.keyword {
		g.jo.SetContent(g.cursorX, g.y1, c, nil, style)
		g.cursorX++
	}

	if len(g.keyword) > 0 && (g.keyword[0] == ':' || g.keyword[0] == '@') {
		// TODO
		return
	}

	if len(g.keyword) == 0 {
		g.options = projectFiles
	}
	for i, name := range g.options {
		optionStyle := style
		if i == g.index {
			optionStyle = style.Background(tcell.ColorYellow)
		}
		for j, c := range name {
			g.jo.SetContent(g.x1+j, g.y1-1-i, c, nil, optionStyle)
		}
		// padding
		for j := 0; j < 40-len(name); j++ {
			g.jo.SetContent(g.x1+len(name)+j, g.y1-1-i, ' ', nil, optionStyle)
		}
	}

}

func (g *gotoBar) Range() (x1, y1, x2, y2 int) { return g.x1, g.y1, g.x2, g.y2 }

func (g *gotoBar) ShowCursor() {
	g.jo.ShowCursor(g.cursorX, g.cursorY)
}

func (g *gotoBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}
	switch k.Key() {
	case tcell.KeyRune:
		g.jo.editor.Draw() // clear previous options
		g.keyword = append(g.keyword, k.Rune())
		if g.keyword[0] == ':' {
			return
		}
		if g.keyword[0] == '@' {
			// TODO
			return
		}
		g.options = findFile(string(g.keyword))
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(g.keyword) == 0 {
			return
		}
		g.jo.editor.Draw() // clear previous options
		g.keyword = g.keyword[:len(g.keyword)-1]
		if len(g.keyword) == 0 {
			return
		}
		if g.keyword[0] == ':' {
			return
		}
		if g.keyword[0] == '@' {
			// TODO
			return
		}
		g.options = findFile(string(g.keyword))
	case tcell.KeyEnter:
		if len(g.keyword) > 0 && g.keyword[0] == ':' {
			line, err := strconv.Atoi(string(g.keyword[1:]))
			if err != nil {
				logger.Printf("goto: invalid line number: %s", err)
				return
			}
			if line < 1 || line > len(g.jo.editor.buf)+1 {
				logger.Printf("goto: line number out of range")
				return
			}
			g.jo.editor.line = line
			g.jo.editor.column = 1
			if line <= g.jo.editor.PageSize()/2 {
				g.jo.editor.startLine = 1
			} else {
				g.jo.editor.startLine = line - g.jo.editor.PageSize()/2
			}
			g.jo.editor.Draw()
			g.jo.focus = g.jo.editor
			g.jo.statusBar = newStatusBar(g.jo)
			return
		}
		if len(g.keyword) > 0 && g.keyword[0] == '@' {
			// TODO
			return
		}
		if len(g.options) > 0 {
			g.jo.titleBar = newTitleBar(g.jo, g.options[g.index])
			g.jo.titleBar.Draw()
			g.jo.editor = newEditor(g.jo, g.options[g.index])
			g.jo.editor.Draw()
			g.jo.statusBar = newStatusBar(g.jo)
			g.jo.focus = g.jo.editor
		}
	case tcell.KeyUp:
		if g.index == len(projectFiles)-1 {
			g.index = 0
			return
		}
		g.index++
	case tcell.KeyDown:
		if g.index == 0 {
			g.index = len(projectFiles) - 1
			return
		}
		g.index--
	case tcell.KeyESC:
		g.jo.statusBar = newStatusBar(g.jo)
		g.jo.editor.Draw()
		g.jo.focus = g.jo.editor
	}
}

func (g *gotoBar) LostFocus() {
	g.jo.statusBar = newStatusBar(g.jo)
	g.jo.editor.Draw()
}

var projectFiles []string
var once sync.Once

func initFilenames() {
	dirs, err := os.ReadDir(".")
	if err != nil {
		logger.Print(err)
		return
	}
	for _, d := range dirs {
		if d.Name()[0] == '.' {
			continue
		}
		if d.IsDir() {
			continue
		}
		projectFiles = append(projectFiles, d.Name())
	}
}

func findFile(name string) []string {
	var s []string
	for _, f := range projectFiles {
		if strings.Contains(strings.ToLower(f), name) {
			s = append(s, f)
		}
	}
	return s
}
