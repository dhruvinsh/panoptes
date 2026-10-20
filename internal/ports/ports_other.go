//go:build !linux && !windows && !darwin

package ports

import "go.bug.st/serial"

func osPorts() ([]Port, error) {
	names, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]Port, 0, len(names))
	for _, n := range names {
		out = append(out, Port{Device: n, Description: "n/a", HWID: "n/a"})
	}
	return out, nil
}
