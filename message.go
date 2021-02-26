package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type CloseMessageEvent struct{}

type Message struct {
	Message string
}

func (m Message) Update(msg tea.Msg) (Message, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "q":
			cmds = append(cmds, fireEvent(CloseMessageEvent{}))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Message) View() string {
	return fmt.Sprintf("%s\n\nPress ENTER or q to continue", m.Message)
}
