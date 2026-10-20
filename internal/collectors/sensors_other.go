//go:build !linux && !darwin && !windows

package collectors

import "github.com/dhruvinsh/panoptes/internal/frame"

func readTemperatures() ([]*frame.Obj, *float64, error) { return nil, nil, nil }
func readFans() ([]*frame.Obj, error)                   { return nil, nil }
func readBattery() (*BatteryInfo, error)                { return nil, nil }
