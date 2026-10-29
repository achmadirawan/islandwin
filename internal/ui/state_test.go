package ui

import "testing"

func TestMachineFlow(t *testing.T) {
	m := NewMachine()

	if m.Current != StateCollapsed {
		t.Fatalf("expected initial state Collapsed, got %v", m.Current)
	}

	// Hover in
	state, changed := m.HoverEnter()
	if !changed || state != StateExpandedMusic {
		t.Fatalf("expected change to ExpandedMusic on hover, got %v", state)
	}

	// Switch to todo
	state, changed = m.SwitchExpanded()
	if !changed || state != StateExpandedTodo {
		t.Fatalf("expected switch to ExpandedTodo, got %v", state)
	}

	// Switch to prayer
	state, changed = m.SwitchExpanded()
	if !changed || state != StateExpandedPrayer {
		t.Fatalf("expected switch to ExpandedPrayer, got %v", state)
	}

	// Switch back to music
	state, changed = m.SwitchExpanded()
	if !changed || state != StateExpandedMusic {
		t.Fatalf("expected switch to ExpandedMusic, got %v", state)
	}

	// Re-enter todo
	m.SwitchExpanded()

	// Hover leave without pin collapses
	state, changed = m.HoverLeave()
	if !changed || state != StateCollapsed {
		t.Fatalf("expected collapse on unpinned hover leave, got %v", state)
	}

	// Next hover remembers Todo
	state, changed = m.HoverEnter()
	if !changed || state != StateExpandedTodo {
		t.Fatalf("expected expand to remembered Todo, got %v", state)
	}

	// Pin it
	pinned := m.TogglePin()
	if !pinned || !m.Pinned {
		t.Fatal("expected pinned to be true")
	}

	// Hover leave with pin should NOT collapse
	state, changed = m.HoverLeave()
	if changed || state != StateExpandedTodo {
		t.Fatal("expected no collapse when pinned")
	}

	// Unpin and collapse
	state, changed = m.UnpinAndCollapse()
	if !changed || state != StateCollapsed || m.Pinned {
		t.Fatal("expected unpin and collapse to succeed")
	}
}
