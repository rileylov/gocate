// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// DragState represents the current state of a drag operation
type DragState int

const (
	DragStateIdle DragState = iota
	DragStateDragging
)

// DragHandler manages mouse drag operations for resizable components
type DragHandler struct {
	state        DragState
	dragHandleID string // ID of the handle being dragged
	lastX        int    // Last known X position during drag
}

// NewDragHandler creates a new drag handler
func NewDragHandler() *DragHandler {
	return &DragHandler{
		state: DragStateIdle,
	}
}

// HandleMouseEvent processes mouse events for drag operations
// Returns true if the event was handled (consumed), false otherwise
func (d *DragHandler) HandleMouseEvent(msg tea.MouseMsg, handleIDs []string) (bool, string, int) {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			// Check if mouse is over any drag handle
			for _, handleID := range handleIDs {
				if zone.Get(handleID).InBounds(msg) {
					d.startDrag(handleID, msg.X)
					return true, handleID, 0
				}
			}
		}
	case tea.MouseActionMotion:
		if d.state == DragStateDragging {
			deltaX := msg.X - d.lastX
			d.lastX = msg.X
			return true, d.dragHandleID, deltaX
		}
	case tea.MouseActionRelease:
		if d.state == DragStateDragging && msg.Button == tea.MouseButtonLeft {
			handleID := d.dragHandleID
			d.stopDrag()
			return true, handleID, 0
		}
	}
	return false, "", 0
}

// IsDragging returns true if currently in a drag operation
func (d *DragHandler) IsDragging() bool {
	return d.state == DragStateDragging
}

// GetDragHandleID returns the ID of the handle currently being dragged
func (d *DragHandler) GetDragHandleID() string {
	return d.dragHandleID
}

// startDrag begins a drag operation
func (d *DragHandler) startDrag(handleID string, x int) {
	d.state = DragStateDragging
	d.dragHandleID = handleID
	d.lastX = x
}

// stopDrag ends the current drag operation
func (d *DragHandler) stopDrag() {
	d.state = DragStateIdle
	d.dragHandleID = ""
	d.lastX = 0
}
