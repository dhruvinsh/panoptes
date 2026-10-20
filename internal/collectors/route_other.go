//go:build !linux

package collectors

// defaultRouteIface is Linux-only; elsewhere the default-route source IP is used instead.
func defaultRouteIface() string { return "" }
