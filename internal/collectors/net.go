package collectors

import (
	"net"
	"strconv"
	"strings"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

var excludePrefixes = []string{
	"lo", "docker", "veth", "virbr", "tun", "tap", "br-", "vmnet", "vboxnet", "lxc", "lxd", "cni",
	"flannel", "cali", "kube", "podman", "zt", "utun", "awdl", "llw", "gif", "stf", "anpi", "bridge",
	"isatap", "teredo",
}

var excludeSubstrings = []string{"loopback", "pseudo-interface", "bluetooth", "vethernet", "virtualbox", "vmware",
	"hyper-v", "wsl"}

const redetectEvery = 30 * time.Second

// IsExcludedIface reports virtual/loopback interfaces that are never "primary".
func IsExcludedIface(name string) bool {
	low := strings.ToLower(name)
	for _, p := range excludePrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	for _, s := range excludeSubstrings {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

// IfCounters are cumulative byte counters for one interface.
type IfCounters struct{ Rx, Tx uint64 }

// PickPrimary chooses the primary interface: the default-route interface, else the one
// holding the default-route source IP, else the one with the most traffic. Virtual and
// down interfaces are skipped. up[name] missing means "unknown" (treated as up).
func PickPrimary(counters map[string]IfCounters, addrs map[string][]string, up map[string]bool,
	defaultIface, defaultIP string, order []string) string {
	usable := func(name string) bool {
		if IsExcludedIface(name) {
			return false
		}
		if _, ok := counters[name]; !ok {
			return false
		}
		if u, ok := up[name]; ok && !u {
			return false
		}
		return true
	}
	if defaultIface != "" && usable(defaultIface) {
		return defaultIface
	}
	if defaultIP != "" {
		for _, name := range order {
			if !usable(name) {
				continue
			}
			for _, a := range addrs[name] {
				if a == defaultIP {
					return name
				}
			}
		}
	}
	best, bestBytes := "", int64(-1)
	for _, name := range order {
		c, ok := counters[name]
		if !ok || !usable(name) {
			continue
		}
		if total := int64(c.Rx + c.Tx); total > bestBytes {
			best, bestBytes = name, total
		}
	}
	return best
}

// localIPv4ForDefaultRoute is the IP the OS would use to reach the internet
// (a UDP "connect" sends no packets, it only does a route lookup).
func localIPv4ForDefaultRoute() string {
	c, err := net.Dial("udp4", "192.0.2.1:9") // TEST-NET-1
	if err != nil {
		return ""
	}
	defer c.Close()
	if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}

type ifInfo struct {
	addrs map[string][]string
	up    map[string]bool
}

func interfaceInfo() ifInfo {
	info := ifInfo{addrs: map[string][]string{}, up: map[string]bool{}}
	ifs, err := net.Interfaces()
	if err != nil {
		return info
	}
	for _, i := range ifs {
		info.up[i.Name] = i.Flags&net.FlagUp != 0 && i.Flags&net.FlagRunning != 0
		as, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range as {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
				info.addrs[i.Name] = append(info.addrs[i.Name], ip4.String())
			}
		}
	}
	return info
}

// Network: primary interface name, IPv4 and rx/tx byte rates.
type Network struct {
	Iface   string // force this interface (matched ignoring case); "" = detect the primary one
	iface   string
	checked time.Time
	rates   RateTracker
}

func (*Network) Name() string { return "net" }

func (n *Network) Prime() error { return n.Collect(frame.New()) }

func (n *Network) detect(counters map[string]IfCounters, order []string) {
	info := interfaceInfo()
	def := defaultRouteIface()
	ip := ""
	if def == "" {
		ip = localIPv4ForDefaultRoute()
	}
	if nw := PickPrimary(counters, info.addrs, info.up, def, ip, order); nw != n.iface {
		n.iface = nw
		n.rates.Reset()
	}
	n.checked = time.Now()
}

// forcedIface is the configured interface as the OS names it, or "" if none is configured
// or it isn't present (then the primary interface is detected as usual).
func (n *Network) forcedIface(present []string) string {
	for _, name := range present {
		if n.Iface != "" && strings.EqualFold(name, n.Iface) {
			return name
		}
	}
	return ""
}

func (n *Network) Collect(out *frame.Obj) error {
	stats, err := psnet.IOCounters(true)
	if err != nil {
		return err
	}
	counters := map[string]IfCounters{}
	order := make([]string, 0, len(stats))
	for _, s := range stats {
		counters[s.Name] = IfCounters{Rx: s.BytesRecv, Tx: s.BytesSent}
		order = append(order, s.Name)
	}
	if forced := n.forcedIface(order); forced != "" {
		if n.iface != forced {
			n.iface = forced
			n.rates.Reset()
		}
	} else if _, ok := counters[n.iface]; !ok || time.Since(n.checked) > redetectEvery {
		n.detect(counters, order)
	}
	if n.iface == "" {
		return nil
	}
	c := counters[n.iface]
	o := frame.New("iface", n.iface)
	if addrs := interfaceInfo().addrs[n.iface]; len(addrs) > 0 {
		o.Set("ip", addrs[0])
	}
	if r := n.rates.Update(map[string]float64{"rx": float64(c.Rx), "tx": float64(c.Tx)}); r != nil {
		o.Set("rx", r["rx"])
		o.Set("tx", r["tx"])
	}
	out.Set("net", o)
	return nil
}

func parseProcNetRoute(text string) string {
	best, bestMetric := "", -1
	lines := strings.Split(text, "\n")
	for _, line := range lines[min(1, len(lines)):] {
		f := strings.Fields(line)
		if len(f) < 8 || f[1] != "00000000" || f[7] != "00000000" {
			continue
		}
		metric, err := strconv.Atoi(f[6])
		if err != nil || IsExcludedIface(f[0]) {
			continue
		}
		if bestMetric < 0 || metric < bestMetric {
			best, bestMetric = f[0], metric
		}
	}
	return best
}
