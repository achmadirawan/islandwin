// Package anim implements a critically-damped spring animator.
// No Win32 dependencies — fully testable with go test.
package anim

import "math"

// Spring simulates a critically-damped spring for smooth UI animation.
// Call Tick each frame; stop calling when Tick returns false (settled).
type Spring struct {
	Value     float64
	Target    float64
	Velocity  float64
	Stiffness float64 // default 300
	Damping   float64 // default 30
}

// New returns a Spring initialised at the given value.
func New(initial float64) *Spring {
	return &Spring{
		Value:     initial,
		Target:    initial,
		Stiffness: 240,
		Damping:   27,
	}
}

// SetTarget updates the target without resetting velocity (smooth redirect).
func (s *Spring) SetTarget(t float64) {
	s.Target = t
}

// Tick advances the spring by dt seconds.
// Returns true while still moving, false when settled.
func (s *Spring) Tick(dt float64) bool {
	force := -s.Stiffness*(s.Value-s.Target) - s.Damping*s.Velocity
	s.Velocity += force * dt
	s.Value += s.Velocity * dt
	if math.Abs(s.Value-s.Target) < 0.25 && math.Abs(s.Velocity) < 0.25 {
		s.Value = s.Target
		s.Velocity = 0
		return false
	}
	return true
}
