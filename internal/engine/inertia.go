package engine

import (
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
	"github.com/atmatjp/lets-wheelpad/internal/config"
)

type InertiaManager struct {
	output    *evdev.InputDevice
	settings  config.InertiaSettings
	stopSignal chan struct{}
	isActive  bool
	mu        sync.Mutex
	step      int
}

func NewInertiaManager(vWheel *evdev.InputDevice, settings config.InertiaSettings, step int) *InertiaManager {
	return &InertiaManager{
		output:    vWheel,
		settings:  settings,
		step:      step,
	}
}

func (im *InertiaManager) Start(velocity float64, dir int32, isHorizontal bool) {
	if !im.settings.Enabled {
		return
	}

	im.Halt()

	im.mu.Lock()
	im.stopSignal = make(chan struct{})
	im.isActive = true
	im.mu.Unlock()

	go im.loop(velocity, dir, isHorizontal)
}

func (im *InertiaManager) Halt() {
	im.mu.Lock()
	if im.isActive {
		close(im.stopSignal)
		im.isActive = false
	}
	im.mu.Unlock()
}

func (im *InertiaManager) loop(velocity float64, dir int32, isHorizontal bool) {
	ticker := time.NewTicker(time.Duration(im.settings.TickInterval) * time.Millisecond)
	defer ticker.Stop()

	for velocity > im.settings.StopVelocity {
		select {
		case <-im.stopSignal:
			return
		case <-ticker.C:
			hires := dir * int32(float64(im.step)*(velocity/2.0))
			if hires == 0 {
				hires = dir
			}

			if isHorizontal {
				im.output.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL, Value: dir})
				im.output.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_HWHEEL_HI_RES, Value: hires})
			} else {
				im.output.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: dir})
				im.output.WriteOne(&evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL_HI_RES, Value: hires})
			}
			im.output.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})

			velocity *= im.settings.Friction
		}
	}

	im.mu.Lock()
	im.isActive = false
	im.mu.Unlock()
}
