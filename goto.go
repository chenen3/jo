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
	x, y             int
	width            int
	height           int
	cursorX, cursorY int

	index   int
	options []string
}

func newGotoBar(j *Jo, keyword string) *gotoBar {
	once.Do(initFilenames)
	return &gotoBar{jo: j, keyword: []rune(keyword), height: 1}
}

func (g *gotoBar) SetPos(x, y, width, height int) {
	g.x = x
	g.y = y
	g.width = width
}

func (g *gotoBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	for y := g.y; y < g.y+g.height; y++ {
		for x := g.x; x < g.x+g.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}
	g.cursorX = g.x
	g.cursorY = g.y

	if len(g.keyword) == 0 {
		placeholder := "search files by name (append : to go to line or @ to go to symbol)"
		for i, c := range placeholder {
			screen.SetContent(g.cursorX+i, g.y, c, nil, style.Foreground(tcell.ColorGray))
		}
	}
	for _, c := range g.keyword {
		screen.SetContent(g.cursorX, g.y, c, nil, style)
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
			screen.SetContent(g.x+j, g.y-1-i, c, nil, optionStyle)
		}
		// padding
		for j := 0; j < 40-len(name); j++ {
			screen.SetContent(g.x+len(name)+j, g.y-1-i, ' ', nil, optionStyle)
		}
	}

}

func (g *gotoBar) Pos() (x1, y1, width, height int) { return g.x, g.y, g.width, g.height }

func (g *gotoBar) ShowCursor() {
	screen.ShowCursor(g.cursorX, g.cursorY)
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
			g.gotoLine(line)
			return
		}
		if len(g.keyword) > 0 && g.keyword[0] == '@' {
			// TODO
			return
		}
		if len(g.options) > 0 {
			g.gotoFile(g.options[g.index])
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
	}
}

func (g *gotoBar) gotoLine(line int) {
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
	g.jo.replaceStatus(newStatusBar(g.jo))
}

func (g *gotoBar) gotoFile(name string) {
	g.jo.titleBar.Set(name)
	g.jo.titleBar.Draw()

	g.jo.editor.Load(name)
	g.jo.editor.Draw()
	g.jo.focus = g.jo.editor

	g.jo.replaceStatus(newStatusBar(g.jo))
}

func (g *gotoBar) LostFocus() {
	g.jo.replaceStatus(newStatusBar(g.jo))
	g.jo.editor.Draw()
}
func (g *gotoBar) Fixed() bool {
	return true
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
