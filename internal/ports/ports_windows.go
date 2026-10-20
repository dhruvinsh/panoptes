package ports

import (
	"fmt"
	"strconv"

	"go.bug.st/serial/enumerator"
)

// osPorts uses SetupAPI via go.bug.st/serial/enumerator (pure Go, no ports are opened).
func osPorts() ([]Port, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]Port, 0, len(list))
	for _, d := range list {
		p := Port{Device: d.Name, Description: d.Product, SerialNumber: d.SerialNumber}
		if d.IsUSB {
			vid, _ := strconv.ParseInt(d.VID, 16, 32)
			pid, _ := strconv.ParseInt(d.PID, 16, 32)
			p.VID, p.PID = int(vid), int(pid)
			p.HWID = fmt.Sprintf("USB VID:PID=%04X:%04X", p.VID, p.PID)
			if d.SerialNumber != "" {
				p.HWID += " SER=" + d.SerialNumber
			}
		}
		out = append(out, p)
	}
	return out, nil
}
