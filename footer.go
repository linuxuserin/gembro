package main

import (
	"fmt"
	"strings"

	"git.sr.ht/~rafael/gembro/text"
	tea "github.com/charmbracelet/bubbletea"
)

const footerHeight = 1

type FooterClickEvent struct {
	tea.MouseMsg
}

type Footer struct {
	height  int
	buttons []string
}

type ButtonClickEvent struct {
	Button string
}

func NewFooter(buttons ...string) Footer {
	return Footer{footerHeight, buttons}
}

func (f Footer) Update(msg tea.Msg) (Footer, tea.Cmd) {
	switch msg := msg.(type) {
	case FooterClickEvent:
		if b := f.GetButtonOnX(msg.X); b != "" {
			return f, fireEvent(ButtonClickEvent{b})
		}
	}
	return f, nil
}

func (f Footer) View() (string, int) {
	var buf strings.Builder
	var width int
	for _, b := range f.buttons {
		fmt.Fprintf(&buf, "[%s] ", text.Color(b, text.Clink))
		width += len(b) + 3
	}
	return buf.String(), width
}

func (f Footer) GetButtonOnX(x int) string {
	var curX int
	for _, b := range f.buttons {
		nextX := curX + len(b) + 3
		if curX <= x && x < nextX-1 {
			return b
		}
		curX = nextX
	}
	return ""
}
