package anim

import (
	"testing"
)

func TestSpringSettles(t *testing.T) {
	s := New(0)
	s.SetTarget(100)
	for i := 0; i < 10000; i++ {
		if !s.Tick(1.0 / 60.0) {
			break
		}
	}
	if s.Value != 100 {
		t.Fatalf("spring did not settle: value=%.2f", s.Value)
	}
}

func TestSpringNoMovementWhenAtTarget(t *testing.T) {
	s := New(50)
	moved := s.Tick(1.0 / 60.0)
	if moved {
		t.Fatal("spring at target should not move")
	}
}
