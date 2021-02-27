package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Input struct {
	Message string
	Type    int
	input   textinput.Model
}

func NewInput() Input {
	ti := textinput.NewModel()
	ti.Placeholder = ""
	ti.CharLimit = 156
	ti.Width = 50

	return Input{
		Message: "",
		input:   ti,
	}
}

func (inp Input) Show(msg, val string, typ int) Input {
	inp.input.Focus()
	inp.Message = msg
	inp.Type = typ
	inp.input.SetValue(val)
	inp.input.CursorEnd()
	return inp
}

func (inp Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			inp.input.Blur()
			cmds = append(cmds, fireEvent(InputEvent{Value: inp.input.Value(), Type: inp.Type}))
		case "ctrl+q":
			inp.input.Blur()
			cmds = append(cmds, fireEvent(CloseInputEvent{}))
		}
	}

	inp.input, cmd = inp.input.Update(msg)
	cmds = append(cmds, cmd)
	return inp, tea.Batch(cmds...)
}

func (inp Input) View() string {
	return fmt.Sprintf("%s %s\n\nPress ENTER to continue or ctrl+q to cancel", inp.Message, inp.input.View())
}
