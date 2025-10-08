// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

var (
	headerStyle = lipgloss.NewStyle().
			Background(subtle).
			Foreground(lipgloss.AdaptiveColor{Light: "#333", Dark: "#FFF"}).
			Height(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			Background(subtle)

	headerButtonStyle = lipgloss.NewStyle().
				Background(highlight).
				Foreground(lipgloss.AdaptiveColor{Light: "#FFF", Dark: "#FFF"}).
				Margin(0, 1).
				Padding(0, 1)

	headerButtonActiveStyle = headerButtonStyle.
				Copy().
				Background(special).
				Bold(true)
)

type header struct {
	id      string
	width   int
	height  int
	title   string
	buttons []headerButton
}

type headerButton struct {
	label  string
	active bool
}

func newHeader(title string) *header {
	return &header{
		id:     zone.NewPrefix(),
		height: 1,
		title:  title,
		buttons: []headerButton{
			{label: "Settings", active: false},
			{label: "Help", active: false},
			{label: "About", active: false},
		},
	}
}

func (h *header) Init() tea.Cmd {
	return nil
}

func (h *header) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			return h, nil
		}
		// Check if any button was clicked
		for i := range h.buttons {
			buttonID := h.getButtonID(i)
			if zone.Get(buttonID).InBounds(msg) {
				// Toggle the clicked button
				h.buttons[i].active = !h.buttons[i].active
				break
			}
		}
	}
	return h, nil
}
func (h *header) View() string {
	// Create buttons on the right
	var buttonViews []string
	for i, button := range h.buttons {
		buttonID := h.getButtonID(i)
		style := headerButtonStyle
		if button.active {
			style = headerButtonActiveStyle
		}
		buttonViews = append(buttonViews, zone.Mark(buttonID, style.Render(button.label)))
	}
	buttonsSection := lipgloss.JoinHorizontal(lipgloss.Center, buttonViews...)
	buttonsWidth := lipgloss.Width(buttonsSection)
	// Compute how much room is available for the title accounting for
	// header padding (left+right = 2) and a space between title and buttons.
	maxTitleWidth := h.width - buttonsWidth - 2
	if maxTitleWidth < 0 {
		maxTitleWidth = 0
	}
	// Truncate the title with an ellipsis if it won't fit.
	titleText := h.title
	if lipgloss.Width(titleText) > maxTitleWidth {
		runes := []rune(titleText)
		for len(runes) > 0 && lipgloss.Width(string(runes))+1 > maxTitleWidth {
			runes = runes[:len(runes)-1]
		}
		if maxTitleWidth > 0 {
			titleText = string(runes) + "…"
		} else {
			titleText = ""
		}
	}
	title := titleStyle.Render(titleText)
	titleWidth := lipgloss.Width(title)
	// Calculate spacing to push buttons to the right
	spacingWidth := h.width - titleWidth - buttonsWidth - 2
	if spacingWidth < 0 {
		spacingWidth = 0
	}
	spacing := lipgloss.NewStyle().Background(subtle).Width(spacingWidth).Render("")
	// Combine title, spacing, and buttons
	content := lipgloss.JoinHorizontal(lipgloss.Center, title, spacing, buttonsSection)
	return headerStyle.Width(h.width).Render(content)
}

func (h *header) getButtonID(index int) string {
	return h.id + "button_" + string(rune('0'+index))
}
