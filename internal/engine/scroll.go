package engine

import (
	"math"
	"time"

	"github.com/holoplot/go-evdev"
	"github.com/atmatjp/lets-wheelpad/internal/config"
)

type ScrollEngine struct {
	vWheel     *evdev.InputDevice
	inertia    *InertiaManager
	settings   config.AppConfig
	scrollSign float64
	scaleY     float64

	curX, curY          int
	lastAngle           float64
	hasLastAngle        bool
	lastTime            time.Time
	scrollMode          bool
	isTouching          bool
	fingerCount         int
	lastDir             int32
	lastIsHorizontal    bool
	lastAngularVelocity float64
}

func NewScrollEngine(vWheel *evdev.InputDevice, settings config.AppConfig) *ScrollEngine {
	scrollSign := 1.0
	if settings.Scroll.NaturalScroll {
		scrollSign = -1.0
	}

	return &ScrollEngine{
		vWheel:     vWheel,
		inertia:    NewInertiaManager(vWheel, settings.Inertia, settings.Scroll.HiresStep),
		settings:   settings,
		scrollSign: scrollSign,
		scaleY:     1.33,
	}
}

func (s *ScrollEngine) ProcessFrame(events []*evdev.InputEvent) bool {
	// 1. Update tracking state from the frame
	for _, e := range events {
		if e.Type == evdev.EV_ABS {
			if e.Code == evdev.ABS_X || e.Code == evdev.ABS_MT_POSITION_X {
				s.curX = int(e.Value)
			} else if e.Code == evdev.ABS_Y || e.Code == evdev.ABS_MT_POSITION_Y {
				s.curY = int(e.Value)
			}
		}
	}

	// 2. Detect touch transitions
	for _, e := range events {
		if e.Type == evdev.EV_KEY {
			switch e.Code {
			case evdev.BTN_TOOL_FINGER:
				if e.Value > 0 {
					s.fingerCount = 1
				} else {
					s.fingerCount = 0
				}
			case evdev.BTN_TOOL_DOUBLETAP:
				if e.Value > 0 {
					s.fingerCount = 2
				}
			case evdev.BTN_TOUCH:
				prevTouching := s.isTouching
				s.isTouching = e.Value > 0
				if s.isTouching && !prevTouching {
					s.handleTouchStart()
				} else if !s.isTouching && prevTouching {
					s.handleTouchEnd()
				}
			}
		}
	}

	// 3. Perform scrolling logic if in mode
	if s.scrollMode && s.isTouching {
		hasCoords := false
		for _, e := range events {
			if e.Type == evdev.EV_ABS && (e.Code == evdev.ABS_X || e.Code == evdev.ABS_Y ||
				e.Code == evdev.ABS_MT_POSITION_X || e.Code == evdev.ABS_MT_POSITION_Y) {
				hasCoords = true
				break
			}
		}

		if hasCoords {
			s.handleScrollMove()
		}
		return true // Events handled by engine
	}

	return false // Events should be forwarded to virtual pad
}

func (s *ScrollEngine) handleTouchStart() {
	s.inertia.Halt()
	dx := float64(s.curX - s.settings.Scroll.CenterX)
	dy := float64(s.curY-s.settings.Scroll.CenterY) * s.scaleY
	dist := math.Hypot(dx, dy)

	if dist > float64(s.settings.Scroll.Deadzone) {
		s.scrollMode = true
		s.lastAngle = math.Atan2(dy, dx)
		s.hasLastAngle = true
		s.lastTime = time.Now()
		s.lastAngularVelocity = 0
	} else {
		s.scrollMode = false
	}
}

func (s *ScrollEngine) handleTouchEnd() {
	if s.scrollMode {
		s.inertia.Start(s.lastAngularVelocity, s.lastDir, s.lastIsHorizontal)
	}
	s.scrollMode = false
	s.hasLastAngle = false
}

func (s *ScrollEngine) handleScrollMove() {
	dx := float64(s.curX - s.settings.Scroll.CenterX)
	dy := float64(s.curY-s.settings.Scroll.CenterY) * s.scaleY
	angle := math.Atan2(dy, dx)
	now := time.Now()

	if !s.hasLastAngle {
		s.lastAngle = angle
		s.lastTime = now
		s.hasLastAngle = true
		return
	}

	diff := angle - s.lastAngle
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}

	if math.Abs(diff) >= s.settings.Scroll.Sensitivity {
		dt := now.Sub(s.lastTime).Seconds()
		velocity := 0.0
		if dt > 0 {
			velocity = math.Abs(diff) / dt
		}

		multiplier := s.calcMultiplier(velocity)
		direction := int32(1)
		if diff > 0 {
			direction = -1
		}

		finalDir := int32(float64(direction) * s.scrollSign)
		hiresVal := finalDir * int32(s.settings.Scroll.HiresStep) * multiplier
		isHorizontal := s.fingerCount >= 2

		s.lastDir = finalDir
		s.lastIsHorizontal = isHorizontal
		s.lastAngularVelocity = velocity

		if isHorizontal {
			s.vWheel.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: finalDir * multiplier})
			s.vWheel.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL_HI_RES, Value: hiresVal})
		} else {
			s.vWheel.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: finalDir * multiplier})
			s.vWheel.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: hiresVal})
		}
		s.vWheel.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})

		s.lastAngle = angle
		s.lastTime = now
	}
}

func (s *ScrollEngine) calcMultiplier(velocity float64) int32 {
	for _, t := range s.settings.Dynamics.Thresholds {
		if velocity >= t.MinVelocity {
			return int32(t.Multiplier)
		}
	}
	return 1
}
