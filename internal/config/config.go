package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type AppConfig struct {
	Input    InputDeviceConfig `toml:"device"`
	Scroll   ScrollSettings    `toml:"wheelpad"`
	Dynamics DynamicsConfig    `toml:"speed"`
	Inertia  InertiaSettings   `toml:"inertia"`
}

type InputDeviceConfig struct {
	DeviceName string `toml:"name"`
}

type ScrollSettings struct {
	CenterX       int     `toml:"center_x"`
	CenterY       int     `toml:"center_y"`
	Deadzone      int     `toml:"deadzone"`
	Sensitivity   float64 `toml:"sensitivity"`
	HiresStep     int     `toml:"hires_step"`
	NaturalScroll bool    `toml:"natural_scroll"`
}

type DynamicThreshold struct {
	MinVelocity float64 `toml:"velocity"`
	Multiplier  int     `toml:"multiplier"`
}

type DynamicsConfig struct {
	Thresholds []DynamicThreshold `toml:"thresholds"`
}

type InertiaSettings struct {
	Enabled      bool    `toml:"enabled"`
	Friction     float64 `toml:"friction"`
	StopVelocity float64 `toml:"min_velocity"`
	TickInterval int     `toml:"interval_ms"`
}

func GetDefaultConfig() AppConfig {
	return AppConfig{
		Input: InputDeviceConfig{
			DeviceName: "SynPS/2 Synaptics TouchPad",
		},
		Scroll: ScrollSettings{
			CenterX:       3495,
			CenterY:       2965,
			Deadzone:      1800,
			Sensitivity:   0.5,
			HiresStep:     60,
			NaturalScroll: false,
		},
		Dynamics: DynamicsConfig{
			Thresholds: []DynamicThreshold{},
		},
		Inertia: InertiaSettings{
			Enabled:      false,
			Friction:     0.85,
			StopVelocity: 0.5,
			TickInterval: 16,
		},
	}
}

func LoadConfig(path string) (AppConfig, error) {
	cfg := GetDefaultConfig()

	// 検索パスの候補
	searchPaths := []string{}
	if path != "" {
		searchPaths = append(searchPaths, path)
	} else {
		home, _ := os.UserHomeDir()
		searchPaths = append(searchPaths, filepath.Join(home, ".config", "wheelpad", "config.toml"))
		searchPaths = append(searchPaths, "/etc/wheelpad/config.toml")
	}

	var data []byte
	var err error
	for _, p := range searchPaths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}

	if err != nil {
		return cfg, fmt.Errorf("could not find config file in any search path")
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
