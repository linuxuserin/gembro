package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Message struct {
	Message     string
	Type        int
	WithConfirm bool
}

func (m Message) Update(msg tea.Msg) (Message, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch skey := msg.String(); skey {
		case "y", "n", "enter", "q":
			cmds = append(cmds, fireEvent(MessageEvent{Response: skey == "y", Type: m.Type}))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Message) View() string {
	if m.WithConfirm {
		return fmt.Sprintf("%s\n\n(Y)es or (N)o", m.Message)
	}
	return fmt.Sprintf("%s\n\nPress ENTER or q to continue", m.Message)
}
