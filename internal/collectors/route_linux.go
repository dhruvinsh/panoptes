package collectors

import (
	"os"
)

// defaultRouteIface reads /proc/net/route and returns the lowest-metric default route's
// interface (skipping virtual ones).
func defaultRouteIface() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	return parseProcNetRoute(string(b))
}
