// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// This is a modified version of this example, supporting full screen, dynamic
// resizing, and clickable models (tabs, lists, dialogs, etc).
// 	https://github.com/charmbracelet/lipgloss/blob/master/example

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
)

type model struct {
	height             int
	width              int
	header             tea.Model
	footer             tea.Model
	resizableContainer *ResizableContainer
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) isInitialized() bool {
	return m.height != 0 && m.width != 0
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.isInitialized() {
		if _, ok := msg.(tea.WindowSizeMsg); !ok {
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Example of toggling mouse event tracking on/off.
		if msg.String() == "ctrl+e" {
			zone.SetEnabled(!zone.Enabled())
			return m, nil
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		msg.Height -= 2
		msg.Width -= 2
		return m.propagate(msg), nil
	}
	return m.propagate(msg), nil
}

func (m *model) propagate(msg tea.Msg) tea.Model {
	// Update header first so we can measure its rendered height.
	m.header, _ = m.header.Update(msg)
	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		// Measure header/footer heights, then allocate the remaining height
		// to the resizable middle container.
		headerH := lipgloss.Height(m.header.View())
		// Update footer with the container's interior width so it doesn't
		// overflow the border; we'll also use the same height baseline for
		// consistent layout measurements.
		originalMsg := tea.WindowSizeMsg{
			Width:  wmsg.Width,
			Height: wmsg.Height,
		}
		m.footer, _ = m.footer.Update(originalMsg)
		footerH := lipgloss.Height(m.footer.View())
		// Allocate remaining height to the middle container (no extra spacer line).
		middleMsg := wmsg
		middleMsg.Height = wmsg.Height - headerH - footerH
		if middleMsg.Height < 1 {
			middleMsg.Height = 1
		}
		updatedContainer, _ := m.resizableContainer.Update(middleMsg)
		m.resizableContainer = updatedContainer.(*ResizableContainer)
		return m
	}

	// Non-size messages: just propagate.
	updatedContainer, _ := m.resizableContainer.Update(msg)
	m.resizableContainer = updatedContainer.(*ResizableContainer)
	m.footer, _ = m.footer.Update(msg)
	return m
}

func (m model) View() string {
	if !m.isInitialized() {
		return ""
	}
	// Render a bordered container that occupies the full terminal size.
	// Update() already subtracted border+padding from content sizes so the
	// interior area fits without clipping.
	s := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(highlight).
		MaxHeight(m.height).
		MaxWidth(m.width).
		Margin(0, 0, 0, 0).
		Padding(0, 0, 0, 0)
	return zone.Scan(s.Render(lipgloss.JoinVertical(lipgloss.Top,
		m.header.View(),
		m.resizableContainer.View(),
		m.footer.View(),
	)))
}

func main() {
	// Initialize a global zone manager, so we don't have to pass around the manager
	// throughout components.
	zone.NewGlobal()

	// Create the individual components
	headerComponent := newHeader("Resizable Lipgloss Demo")
	footerComponent := newFooter()
	list1Component := &list{
		id:     zone.NewPrefix(),
		height: 8,
		title:  "Citrus Fruits to Try",
		items: []listItem{
			{name: "Grapefruit", done: true},
			{name: "Yuzu", done: false},
			{name: "Citron", done: false},
			{name: "Kumquat", done: true},
			{name: "Pomelo", done: false},
		},
	}
	list2Component := &list{
		id:     zone.NewPrefix(),
		height: 8,
		title:  "Actual Lip Gloss Vendors",
		items: []listItem{
			{name: "Glossier", done: true},
			{name: "Claire's Boutique", done: true},
			{name: "Nyx", done: false},
			{name: "Mac", done: false},
			{name: "Milk", done: false},
		},
	}
	// Create resizable container with the two middle components
	resizableContainer := NewResizableContainer(
		[]tea.Model{list1Component, list2Component},
		[]float64{0.5, 0.5}, // Initial proportions: 50/50 split
	)
	m := &model{
		header:             headerComponent,
		footer:             footerComponent,
		resizableContainer: resizableContainer,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
}
