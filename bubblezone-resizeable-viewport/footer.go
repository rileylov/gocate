// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

var (
	footerStyle = lipgloss.NewStyle().
			Background(subtle).
			Foreground(lipgloss.AdaptiveColor{Light: "#666", Dark: "#AAA"}).
			Padding(0, 0, 0, 0).
			Height(1)

	debugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"})
)

type footer struct {
	id             string
	width          int
	height         int
	terminalWidth  int
	terminalHeight int
}

func newFooter() *footer {
	return &footer{
		id:     zone.NewPrefix(),
		height: 1,
	}
}

func (f *footer) Init() tea.Cmd {
	return nil
}

func (f *footer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.terminalWidth = msg.Width
		f.terminalHeight = msg.Height
	}
	return f, nil
}

func (f *footer) View() string {
	// Get current mouse position if available
	mouseInfo := ""
	if zone.Enabled() {
		mouseInfo = "Mouse: enabled"
	} else {
		mouseInfo = "Mouse: disabled"
	}
	// Create debug information with real-time terminal size
	debugInfo := fmt.Sprintf("Terminal: %dx%d | Component: %dx%d | %s | Ctrl+E=mouse | Ctrl+C=quit",
		f.terminalWidth, f.terminalHeight, f.width, f.height, mouseInfo)
	content := debugStyle.Render(debugInfo)
	return footerStyle.Width(f.width).Render(content)
}
