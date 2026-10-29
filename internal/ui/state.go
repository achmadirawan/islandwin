// Package ui implements the state machine for the island overlay.
// Pure Go — independent of Win32, testable with go test.
package ui

type State int

const (
	StateCollapsed State = iota
	StateExpandedMusic
	StateExpandedTodo
	StateExpandedPrayer
)

func (s State) String() string {
	switch s {
	case StateCollapsed:
		return "Collapsed"
	case StateExpandedMusic:
		return "ExpandedMusic"
	case StateExpandedTodo:
		return "ExpandedTodo"
	case StateExpandedPrayer:
		return "ExpandedPrayer"
	default:
		return "Unknown"
	}
}

// Dimensions returns target logical width and height for a state.
func (s State) Dimensions() (float64, float64) {
	switch s {
	case StateCollapsed:
		return 120, 32
	case StateExpandedMusic:
		return 420, 140
	case StateExpandedTodo:
		return 420, 260
	case StateExpandedPrayer:
		return 420, 210
	default:
		return 120, 32
	}
}

// Machine manages island states, pinned modes, and transitions.
type Machine struct {
	Current      State
	Pinned       bool
	LastExpanded State // remembers whether user prefers music or todo when expanding
}

func NewMachine() *Machine {
	return &Machine{
		Current:      StateCollapsed,
		Pinned:       false,
		LastExpanded: StateExpandedMusic,
	}
}

// HoverEnter triggers expansion to LastExpanded if not pinned.
func (m *Machine) HoverEnter() (State, bool) {
	if m.Current == StateCollapsed {
		m.Current = m.LastExpanded
		return m.Current, true
	}
	return m.Current, false
}

// HoverLeave collapses back if not pinned.
func (m *Machine) HoverLeave() (State, bool) {
	if !m.Pinned && m.Current != StateCollapsed {
		m.Current = StateCollapsed
		return m.Current, true
	}
	return m.Current, false
}

// TogglePin pins or unpins current expanded state.
func (m *Machine) TogglePin() bool {
	if m.Current == StateCollapsed {
		// Clicking while collapsed expands and pins.
		m.Current = m.LastExpanded
		m.Pinned = true
		return true
	}
	m.Pinned = !m.Pinned
	return m.Pinned
}

// UnpinAndCollapse forces collapse and clears pinned status (e.g. on Esc key or clicking outside).
func (m *Machine) UnpinAndCollapse() (State, bool) {
	m.Pinned = false
	if m.Current != StateCollapsed {
		m.Current = StateCollapsed
		return m.Current, true
	}
	return m.Current, false
}

// SwitchExpanded cycles between Music -> Todo -> Prayer -> Music when expanded.
func (m *Machine) SwitchExpanded() (State, bool) {
	switch m.Current {
	case StateExpandedMusic:
		m.Current = StateExpandedTodo
		m.LastExpanded = StateExpandedTodo
		return m.Current, true
	case StateExpandedTodo:
		m.Current = StateExpandedPrayer
		m.LastExpanded = StateExpandedPrayer
		return m.Current, true
	case StateExpandedPrayer:
		m.Current = StateExpandedMusic
		m.LastExpanded = StateExpandedMusic
		return m.Current, true
	}
	return m.Current, false
}

// ToggleExpand toggles between collapsed and expanded (for hotkey trigger).
func (m *Machine) ToggleExpand() (State, bool) {
	if m.Current == StateCollapsed {
		m.Current = m.LastExpanded
		m.Pinned = true
		return m.Current, true
	}
	m.Pinned = false
	m.Current = StateCollapsed
	return m.Current, true
}
