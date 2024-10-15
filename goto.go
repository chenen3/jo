package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

type gotoBar struct {
	baseView
	jo               *Jo
	keyword          []rune
	cursorX, cursorY int

	index   int
	options []string
}

// TODO: go to symbol or call command
func newGotoBar(j *Jo, keyword string) *gotoBar {
	once.Do(loadFileList)
	b := &gotoBar{jo: j, keyword: []rune(keyword)}
	b.height = 1
	return b
}

func (g *gotoBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := g.y; y < g.y+g.height; y++ {
		for x := g.x; x < g.x+g.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(g.keyword) == 0 {
		placeholder := "search files by name (append : to go to line)"
		for i, c := range placeholder {
			screen.SetContent(g.x+i, g.y, c, nil, style.Foreground(tcell.ColorGray))
		}
	}
	for i, c := range g.keyword {
		screen.SetContent(g.x+i, g.y, c, nil, style)
	}
	g.cursorX = g.x + len(g.keyword)
	g.cursorY = g.y
	defer screen.ShowCursor(g.cursorX, g.cursorY)

	if len(g.keyword) > 0 && g.keyword[0] == ':' {
		return
	}

	if len(g.keyword) == 0 {
		g.options = files
	}
	for i, name := range g.options {
		selectedStyle := style
		if i == g.index {
			selectedStyle = style.Background(tcell.ColorLightBlue)
		}
		for j, c := range name {
			screen.SetContent(g.x+j, g.y-1-i, c, nil, selectedStyle)
		}
		// padding
		for j := 0; j < optionWidth-len(name); j++ {
			screen.SetContent(g.x+len(name)+j, g.y-1-i, ' ', nil, selectedStyle)
		}
	}
}

const optionWidth = 40

func (g *gotoBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}
	switch k.Key() {
	case tcell.KeyRune:
		defer g.Draw()
		g.jo.editors.Views[0].Draw() // clear previous options
		g.keyword = append(g.keyword, k.Rune())
		if g.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(g.keyword)) {
				options = append(options, f)
			}
		}
		g.options = options
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		defer g.Draw()
		if len(g.keyword) == 0 {
			return
		}
		g.jo.editors.Views[0].Draw() // clear previous options
		g.keyword = g.keyword[:len(g.keyword)-1]
		if len(g.keyword) == 0 {
			return
		}
		if g.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(g.keyword)) {
				options = append(options, f)
			}
		}
		g.options = options
	case tcell.KeyEnter:
		if len(g.keyword) > 0 && g.keyword[0] == ':' {
			line, err := strconv.Atoi(string(g.keyword[1:]))
			if err != nil {
				log.Printf("goto: invalid line number: %s", err)
				return
			}
			g.gotoLine(line)
			return
		}
		if len(g.options) > 0 {
			g.gotoFile(g.options[g.index])
			g.jo.Draw()
		}
	case tcell.KeyUp:
		defer g.Draw()
		if g.index == len(files)-1 {
			g.index = 0
			return
		}
		g.index++
	case tcell.KeyDown:
		defer g.Draw()
		if g.index == 0 {
			g.index = len(files) - 1
			return
		}
		g.index--
	case tcell.KeyESC:
		g.Defocus()
		g.jo.editor.Focus()
	case tcell.KeyCtrlBackslash:
		e := newEditor(g.jo, g.options[g.index])
		if g.jo.focus != nil {
			g.jo.focus.Defocus()
		}
		g.jo.focus = e
		g.jo.editor = e
		g.jo.editors.Views = append(g.jo.editors.Views, e)
		g.jo.status.Set(newStatusBar(g.jo))
		g.jo.Draw()
		e.ShowCursor()
	}
}

func (g *gotoBar) gotoLine(line int) {
	if line < 1 || line > len(g.jo.editor.buf)+1 {
		log.Printf("goto: line number out of range")
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
	g.jo.editor.Focus()
	g.jo.status.Set(newStatusBar(g.jo))
}

func (g *gotoBar) gotoFile(name string) {
	g.jo.editor.Load(name)
	g.jo.editor.Draw()
	g.jo.editor.Focus()
	g.jo.status.Set(newStatusBar(g.jo))
}

func (g *gotoBar) Defocus() {
	g.jo.status.Set(newStatusBar(g.jo))
	g.jo.status.Draw()
	g.jo.editors.Views[0].Draw() // hide gotoBar
}

func (g *gotoBar) FixedSize() bool {
	return true
}

func (g *gotoBar) OnClick(x, y int) {
	g.Focus()
}

func (g *gotoBar) Focus() {
	if g.jo.focus == g {
		return
	}
	g.jo.focus = g
	screen.ShowCursor(g.cursorX, g.cursorY)
}

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
