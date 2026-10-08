// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import "github.com/go-widgets/toolkit"

// Every pointer event enters through the root box, in the root's own
// coordinates: the boxes translate it for each child and move keyboard focus
// on a click (toolkit's focus system), so the View does no hit-testing of its
// own. Only the open popovers come first, because they float above the boxes.

// rootEvent re-bases a surface-space event onto the root box.
func (s *State) rootEvent(kind toolkit.EventKind, x, y int) toolkit.Event {
	r := s.root.Bounds()
	return toolkit.Event{Kind: kind, X: x - r.X, Y: y - r.Y}
}

// HandleClick routes a primary-button press at surface (x, y).
func (s *State) HandleClick(x, y int) bool {
	for _, d := range s.dropdowns {
		if d.PopoverClick(x, y) {
			s.dirty = true
			return true
		}
	}
	s.dragTarget = s.hit(x, y)
	s.root.OnEvent(s.rootEvent(toolkit.EventClick, x, y))
	s.dirty = true
	return true
}

// hit finds the leaf widget under (x, y), for the pointer capture.
func (s *State) hit(x, y int) toolkit.Widget {
	for _, w := range []toolkit.Widget{s.steps, s.table, s.code} {
		if w.Bounds().Contains(x, y) {
			return w
		}
	}
	return nil
}

// HandleMove routes a pointer move: a drag of the captured widget (a
// scrollbar thumb) while the button is held, else a hover.
func (s *State) HandleMove(x, y int) bool {
	if s.dragTarget != nil {
		r := s.dragTarget.Bounds()
		s.dragTarget.OnEvent(toolkit.Event{Kind: toolkit.EventMouseDrag, X: x - r.X, Y: y - r.Y})
		s.dirty = true
		return true
	}
	s.root.OnEvent(s.rootEvent(toolkit.EventMouseMove, x, y))
	return s.dirty
}

// HandleRelease ends a drag.
func (s *State) HandleRelease(x, y int) bool {
	if s.dragTarget == nil {
		return false
	}
	r := s.dragTarget.Bounds()
	s.dragTarget.OnEvent(toolkit.Event{Kind: toolkit.EventMouseUp, X: x - r.X, Y: y - r.Y})
	s.dragTarget = nil
	s.dirty = true
	return true
}

// HandleScroll routes a wheel of dy rows (dx sideways) at (x, y): to an open
// popover's list first, else to the widget under the pointer.
func (s *State) HandleScroll(x, y, dx, dy int) bool {
	for _, d := range s.dropdowns {
		if d.PopoverOpen() && d.PopoverBounds().Contains(x, y) {
			d.OnEvent(toolkit.Event{Kind: toolkit.EventScroll, Delta: dy})
			s.dirty = true
			return true
		}
	}
	e := s.rootEvent(toolkit.EventScroll, x, y)
	e.Delta, e.DeltaX = dy, dx
	s.root.OnEvent(e)
	s.dirty = true
	return true
}

// HandleChar types a printable character into the focused widget.
func (s *State) HandleChar(ch string) bool {
	s.root.OnEvent(toolkit.Event{Kind: toolkit.EventChar, Code: ch})
	s.dirty = true
	return true
}

// HandleKeyDown delivers a named key (Enter, Backspace, arrows, Tab, Escape)
// to the focused widget; Escape also closes any open popover.
func (s *State) HandleKeyDown(code string) bool {
	if code == "Escape" {
		for _, d := range s.dropdowns {
			d.Open().Set(false)
		}
	}
	s.root.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: code})
	s.dirty = true
	return true
}
