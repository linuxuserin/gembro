package main

import (
	"fmt"

	"git.sr.ht/~rafael/gembro/text"
	tea "github.com/charmbracelet/bubbletea"
)

type Message struct {
	Message     string
	Type        int
	WithConfirm bool
	Payload     string
}

func (m Message) Update(msg tea.Msg) (Message, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch skey := msg.String(); skey {
		case "y", "n", "enter", "q":
			cmds = append(cmds, fireEvent(MessageEvent{Response: skey == "y", Type: m.Type, Payload: m.Payload}))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Message) View() string {
	msg := text.Wrap(m.Message, 80)
	if m.WithConfirm {
		return fmt.Sprintf("%s\n\n(Y)es or (N)o", msg)
	}
	return fmt.Sprintf("%s\n\nPress ENTER or q to continue", msg)
}
