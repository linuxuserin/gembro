package main

import tea "github.com/charmbracelet/bubbletea"

type SelectTabEvent struct {
	Tab int
}
type CloseCurrentTabEvent struct{}
type CloseTabEvent struct {
	Tab int
}
type OpenNewTabEvent struct {
	URL    string
	Switch bool
}

type MessageEvent struct {
	Response bool
	Type     int
}

type CloseInputEvent struct{}
type InputEvent struct {
	Value string
	Type  int
}

func fireEvent(msg tea.Msg) func() tea.Msg {
	return func() tea.Msg {
		return msg
	}
}
