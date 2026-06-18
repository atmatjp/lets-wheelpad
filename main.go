package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/holoplot/go-evdev"
	"github.com/atmatjp/lets-wheelpad/internal/config"
	"github.com/atmatjp/lets-wheelpad/internal/device"
	"github.com/atmatjp/lets-wheelpad/internal/engine"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Warning: failed to load config: %v, using defaults", err)
	}

	physDev, err := device.FindDevice(cfg.Input.DeviceName)
	if err != nil {
		log.Fatalf("Fatal: %v", err)
	}
	defer physDev.Close()

	if err := physDev.Grab(); err != nil {
		log.Fatalf("Fatal: failed to grab device: %v", err)
	}
	defer physDev.Ungrab()

	vPad, err := device.CreateVirtualTouchpad(physDev)
	if err != nil {
		log.Fatalf("Fatal: failed to create virtual pad: %v", err)
	}
	defer vPad.Close()

	// Virtual Wheel for scroll events
	caps := map[evdev.EvType][]evdev.EvCode{
		evdev.EV_REL: {
			evdev.REL_WHEEL,
			evdev.REL_WHEEL_HI_RES,
			evdev.REL_HWHEEL,
			evdev.REL_HWHEEL_HI_RES,
		},
	}
	id, _ := physDev.InputID()
	vWheel, err := evdev.CreateDevice("LetsNote-Virtual-Wheel", id, caps)
	if err != nil {
		log.Fatalf("Fatal: failed to create virtual wheel: %v", err)
	}
	defer vWheel.Close()

	scrollEngine := engine.NewScrollEngine(vWheel, cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Wheelpad Daemon (Modular) started for %s", cfg.Input.DeviceName)

	var frameEvents []*evdev.InputEvent

	eventChan := make(chan *evdev.InputEvent)
	errChan := make(chan error, 1)

	go func() {
		for {
			e, err := physDev.ReadOne()
			if err != nil {
				errChan <- err
				return
			}
			eventChan <- e
		}
	}()

	for {
		select {
		case <-sigChan:
			log.Println("Received termination signal, shutting down...")
			return
		case err := <-errChan:
			log.Printf("Read error: %v", err)
			return
		case e := <-eventChan:
			frameEvents = append(frameEvents, e)

			if e.Type == evdev.EV_SYN && e.Code == evdev.SYN_REPORT {
				handled := scrollEngine.ProcessFrame(frameEvents)

				if !handled {
					for _, fe := range frameEvents {
						vPad.Emit(fe)
					}
				}

				frameEvents = nil
			}
		}
	}
}
