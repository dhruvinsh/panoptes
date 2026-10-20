package collectors

import (
	"context"
	"os/exec"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// macOS: no temperature/fan source without SMC access (same as the Python agent).
func readTemperatures() ([]*frame.Obj, *float64, error) { return nil, nil, nil }
func readFans() ([]*frame.Obj, error)                   { return nil, nil }

// readBattery parses `pmset -g batt`:
//
//	Now drawing from 'AC Power'
//	 -InternalBattery-0 (id=1234)	87%; charging; 0:42 remaining present: true
func readBattery() (*BatteryInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pmset", "-g", "batt").Output()
	if err != nil {
		return nil, err
	}
	return parsePmset(string(out)), nil
}
