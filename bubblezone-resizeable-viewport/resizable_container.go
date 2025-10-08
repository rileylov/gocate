// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const (
	handleWidth   = 1    // Width of drag handles in characters
	minWidthChars = 35   // Minimum width in characters for any component
	maxProportion = 0.70 // Maximum proportion (70%) for any component
)

var (
	// Style for drag handles
	handleStyle = lipgloss.NewStyle().
			Width(handleWidth).
			Background(subtle).
			Foreground(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#585858"})

	// Style for drag handles when being hovered/dragged
	handleActiveStyle = handleStyle.
				Background(highlight).
				Foreground(lipgloss.AdaptiveColor{Light: "#FFF", Dark: "#FFF"})
)

// ResizableContainer manages multiple child components with resizable boundaries
type ResizableContainer struct {
	id          string
	width       int
	height      int
	children    []tea.Model
	proportions []float64 // Width proportions for each child (sum = 1.0)
	dragHandler *DragHandler
}

// NewResizableContainer creates a new resizable container
func NewResizableContainer(children []tea.Model, initialProportions []float64) *ResizableContainer {
	if len(children) != len(initialProportions) {
		panic("children and proportions must have the same length")
	}

	// Normalize proportions to sum to 1.0
	total := 0.0
	for _, p := range initialProportions {
		total += p
	}
	for i := range initialProportions {
		initialProportions[i] /= total
	}

	return &ResizableContainer{
		id:          zone.NewPrefix(),
		children:    children,
		proportions: initialProportions,
		dragHandler: NewDragHandler(),
	}
}

func (r *ResizableContainer) Init() tea.Cmd {
	return nil
}

func (r *ResizableContainer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height

		// Calculate available width (subtract handle widths)
		availableWidth := r.width - ((len(r.children) - 1) * handleWidth)

		// Update child component sizes
		for i, child := range r.children {
			childWidth := int(float64(availableWidth) * r.proportions[i])
			childMsg := tea.WindowSizeMsg{
				Width:  childWidth,
				Height: r.height,
			}
			r.children[i], _ = child.Update(childMsg)
		}

	case tea.MouseMsg:
		// Handle drag operations
		handled, handleID, deltaX := r.dragHandler.HandleMouseEvent(msg, r.getHandleIDs())
		if handled && deltaX != 0 {
			// Determine which handle was dragged and update proportions
			r.updateProportionsFromDrag(handleID, deltaX)
			// Recalculate child sizes after proportion change
			availableWidth := r.width - ((len(r.children) - 1) * handleWidth)
			for i, child := range r.children {
				childWidth := int(float64(availableWidth) * r.proportions[i])
				childMsg := tea.WindowSizeMsg{
					Width:  childWidth,
					Height: r.height,
				}
				r.children[i], _ = child.Update(childMsg)
			}
			return r, nil
		}

		// If not handled by drag system, propagate to children
		if !handled {
			for i, child := range r.children {
				r.children[i], _ = child.Update(msg)
			}
		}

	default:
		// Propagate all other messages to children
		for i, child := range r.children {
			r.children[i], _ = child.Update(msg)
		}
	}

	return r, nil
}

func (r *ResizableContainer) View() string {
	if len(r.children) == 0 {
		return ""
	}

	var parts []string
	availableWidth := r.width - ((len(r.children) - 1) * handleWidth)

	for i, child := range r.children {
		// Render child component
		childWidth := int(float64(availableWidth) * r.proportions[i])
		childView := lipgloss.NewStyle().
			Width(childWidth).
			Height(r.height).
			Render(child.View())

		parts = append(parts, childView)

		// Add drag handle (except after the last child)
		if i < len(r.children)-1 {
			handleID := r.getHandleID(i)
			handleView := r.renderHandle(handleID)
			parts = append(parts, handleView)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// getHandleIDs returns all drag handle IDs
func (r *ResizableContainer) getHandleIDs() []string {
	var ids []string
	for i := 0; i < len(r.children)-1; i++ {
		ids = append(ids, r.getHandleID(i))
	}
	return ids
}

// getHandleID returns the zone ID for a specific handle
func (r *ResizableContainer) getHandleID(handleIndex int) string {
	return r.id + "handle_" + strconv.Itoa(handleIndex)
}

// renderHandle renders a drag handle with appropriate styling
func (r *ResizableContainer) renderHandle(handleID string) string {
	style := handleStyle

	// Highlight handle if it's being dragged or if mouse is over it
	if r.dragHandler.IsDragging() && r.dragHandler.GetDragHandleID() == handleID {
		style = handleActiveStyle
	}

	// Create vertical bars for the handle
	handleContent := ""
	for i := 0; i < r.height; i++ {
		if i > 0 {
			handleContent += "\n"
		}
		handleContent += "│"
	}

	return zone.Mark(handleID, style.Render(handleContent))
}

// updateProportionsFromDrag updates component proportions based on drag movement
func (r *ResizableContainer) updateProportionsFromDrag(handleID string, deltaX int) {
	// Find which handle was dragged
	handleIndex := -1
	for i := 0; i < len(r.children)-1; i++ {
		if r.getHandleID(i) == handleID {
			handleIndex = i
			break
		}
	}

	if handleIndex == -1 {
		return
	}

	// Calculate the change in proportion
	availableWidth := r.width - ((len(r.children) - 1) * handleWidth)
	if availableWidth <= 0 {
		return
	}

	deltaPercent := float64(deltaX) / float64(availableWidth)

	// Update proportions: left component grows/shrinks, right component shrinks/grows
	leftIndex := handleIndex
	rightIndex := handleIndex + 1

	newLeftProp := r.proportions[leftIndex] + deltaPercent
	newRightProp := r.proportions[rightIndex] - deltaPercent

	// Calculate minimum proportion based on character width
	minProportion := float64(minWidthChars) / float64(availableWidth)

	// Enforce constraints
	if newLeftProp < minProportion {
		newLeftProp = minProportion
		newRightProp = r.proportions[rightIndex] + (r.proportions[leftIndex] - newLeftProp)
	}
	if newLeftProp > maxProportion {
		newLeftProp = maxProportion
		newRightProp = r.proportions[rightIndex] + (r.proportions[leftIndex] - newLeftProp)
	}
	if newRightProp < minProportion {
		newRightProp = minProportion
		newLeftProp = r.proportions[leftIndex] + (r.proportions[rightIndex] - newRightProp)
	}
	if newRightProp > maxProportion {
		newRightProp = maxProportion
		newLeftProp = r.proportions[leftIndex] + (r.proportions[rightIndex] - newRightProp)
	}

	// Apply the new proportions
	r.proportions[leftIndex] = newLeftProp
	r.proportions[rightIndex] = newRightProp

	// Ensure proportions still sum to 1.0 (accounting for floating point precision)
	r.normalizeProportions()
}

// normalizeProportions ensures all proportions sum to 1.0
func (r *ResizableContainer) normalizeProportions() {
	total := 0.0
	for _, p := range r.proportions {
		total += p
	}

	if total > 0 {
		for i := range r.proportions {
			r.proportions[i] /= total
		}
	}
}
