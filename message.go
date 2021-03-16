package main

import (
	"fmt"
	"strings"

	"git.sr.ht/~rafael/gembro/text"
	tea "github.com/charmbracelet/bubbletea"
)

type Message struct {
	Message     string
	Type        int
	WithConfirm bool
	Payload     string
	actionY     int
}

func NewMessage(message string, typ int, withConfirm bool, payload string) Message {
	msg := text.Wrap(message, 80)
	actionY := strings.Count(msg, "\n") + headerHeight + 1
	return Message{msg, typ, withConfirm, payload, actionY}
}

func (m Message) Update(msg tea.Msg) (Message, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch skey := msg.String(); skey {
		case "y", "n", "enter", "q", "esc":
			cmds = append(cmds, fireEvent(MessageEvent{Response: skey == "y", Type: m.Type, Payload: m.Payload}))
		}
	case tea.MouseMsg:
		return m, m.handleClick(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m Message) handleClick(msg tea.MouseMsg) tea.Cmd {
	if msg.Y != m.actionY {
		return nil
	}
	if m.WithConfirm {
		yes := 1 <= msg.X && msg.X < 4
		no := 10 <= msg.X && msg.X < 12
		if yes || no {
			return fireEvent(MessageEvent{Response: yes, Type: m.Type, Payload: m.Payload})
		}
		return nil
	}
	if 1 <= msg.X && msg.X < 3 {
		return fireEvent(MessageEvent{Response: false, Type: m.Type, Payload: m.Payload})
	}
	return nil
}

func (m Message) View() string {
	if m.WithConfirm {
		return fmt.Sprintf("%s\n\n[%s] or [%s]",
			m.Message,
			text.Color("Yes", text.Clink),
			text.Color("No", text.Clink))
	}
	return fmt.Sprintf("%s\n\n[%s]", m.Message, text.Color("OK", text.Clink))
}
