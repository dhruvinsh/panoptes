// Package gpu reads GPU stats: NVIDIA via nvidia-smi (Linux/Windows), AMD via sysfs
// (Linux). Intel GPUs and macOS are not reported (the `gpu` key is omitted).
package gpu

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

const (
	mib            = 1024 * 1024
	nvsmiQuery     = "name,utilization.gpu,temperature.gpu,memory.used,memory.total,power.draw"
	nvsmiTimeout   = 2 * time.Second
	retryAfterFail = 30 * time.Second
)

// ShortenName strips vendor noise: "NVIDIA GeForce RTX 3070" -> "RTX 3070".
func ShortenName(name string) string {
	for _, junk := range []string{"NVIDIA ", "GeForce ", "AMD ", "Advanced Micro Devices, Inc. ", "[AMD/ATI] ",
		"Radeon ", " Laptop GPU"} {
		name = strings.ReplaceAll(name, junk, "")
	}
	return strings.TrimSpace(name)
}

func num(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) { // "[N/A]", "[Not Supported]"
		return 0, false
	}
	return f, true
}

// ParseNvidiaSMI parses the first GPU line of
// `nvidia-smi --query-gpu=... --format=csv,noheader,nounits`.
func ParseNvidiaSMI(text string) *frame.Obj {
	for _, line := range strings.Split(text, "\n") {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		g := frame.New("name", ShortenName(parts[0]))
		for i, key := range []string{"pct", "temp", "mem_used", "mem_total"} {
			if v, ok := num(parts[i+1]); ok {
				g.Set(key, int(math.RoundToEven(v)))
			}
		}
		if v, ok := num(parts[5]); ok {
			g.Set("power", v)
		}
		return g
	}
	return nil
}

// Nvidia runs nvidia-smi (binary lookup cached; 30 s back-off after a failure).
type Nvidia struct {
	looked  bool
	exe     string
	retryAt time.Time
}

func findNvidiaSMI() string {
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		sysroot := os.Getenv("SystemRoot")
		if sysroot == "" {
			sysroot = `C:\Windows`
		}
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		for _, dir := range []string{filepath.Join(sysroot, "System32"), filepath.Join(pf, "NVIDIA Corporation", "NVSMI")} {
			c := filepath.Join(dir, "nvidia-smi.exe")
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	return ""
}

// Available reports whether nvidia-smi exists.
func (n *Nvidia) Available() bool {
	if !n.looked {
		n.looked = true
		n.exe = findNvidiaSMI()
	}
	return n.exe != ""
}

// Read returns the GPU object, nil if unavailable, or an error.
func (n *Nvidia) Read() (*frame.Obj, error) {
	if !n.Available() || time.Now().Before(n.retryAt) {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), nvsmiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, n.exe, "--query-gpu="+nvsmiQuery, "--format=csv,noheader,nounits")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		n.retryAt = time.Now().Add(retryAfterFail)
		return nil, fmt.Errorf("nvidia-smi: %w: %.100s", err, strings.TrimSpace(string(out)))
	}
	return ParseNvidiaSMI(string(out)), nil
}

func readStr(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func readInt(p string) (int64, bool) {
	s, ok := readStr(p)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

// pciName looks up a marketing name in pci.ids (e.g. "Radeon RX 6800/6800 XT / 6900 XT").
func pciName(vendor, device string) string {
	vendor = strings.TrimPrefix(strings.ToLower(vendor), "0x")
	device = strings.TrimPrefix(strings.ToLower(device), "0x")
	for _, path := range []string{"/usr/share/hwdata/pci.ids", "/usr/share/misc/pci.ids", "/usr/share/pci.ids"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		name := scanPCIIDs(bufio.NewScanner(f), vendor, device)
		f.Close()
		return name
	}
	return ""
}

func scanPCIIDs(sc *bufio.Scanner, vendor, device string) string {
	inVendor := false
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			inVendor = strings.HasPrefix(strings.ToLower(line), vendor+" ")
			continue
		}
		if inVendor && !strings.HasPrefix(line, "\t\t") && strings.HasPrefix(strings.ToLower(line[1:]), device+" ") {
			_, name, _ := strings.Cut(line[1:], " ")
			name = strings.TrimSpace(name)
			if i, j := strings.Index(name, "["), strings.LastIndex(name, "]"); i >= 0 && j > i {
				name = name[i+1 : j]
			}
			return name
		}
	}
	return ""
}

// AMD reads an amdgpu card from sysfs (Linux only), preferring the one with most VRAM.
type AMD struct {
	scanned bool
	dev     string
	name    string
}

func (a *AMD) scan() {
	a.scanned = true
	busy, _ := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	var best string
	var bestVRAM int64 = -1
	for _, b := range busy {
		dev := filepath.Dir(b)
		if v, _ := readStr(filepath.Join(dev, "vendor")); v != "0x1002" {
			continue
		}
		vram, _ := readInt(filepath.Join(dev, "mem_info_vram_total"))
		if vram > bestVRAM { // prefer the discrete card
			best, bestVRAM = dev, vram
		}
	}
	if best == "" {
		return
	}
	a.dev = best
	name, _ := readStr(filepath.Join(best, "product_name"))
	if name == "" {
		d, _ := readStr(filepath.Join(best, "device"))
		name = pciName("1002", d)
	}
	if name != "" {
		a.name = ShortenName(name)
	} else {
		a.name = "AMD GPU"
	}
}

// Read returns the GPU object, nil if there is no AMD GPU, or an error.
func (a *AMD) Read() (*frame.Obj, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	if !a.scanned {
		a.scan()
	}
	if a.dev == "" {
		return nil, nil
	}
	d := a.dev
	g := frame.New("name", a.name)
	pct, ok := readInt(filepath.Join(d, "gpu_busy_percent"))
	if !ok {
		return nil, fmt.Errorf("cannot read %s/gpu_busy_percent", d)
	}
	g.Set("pct", int(pct))
	used, okU := readInt(filepath.Join(d, "mem_info_vram_used"))
	total, okT := readInt(filepath.Join(d, "mem_info_vram_total"))
	if okU && okT && total > 0 {
		g.Set("mem_used", used/mib)
		g.Set("mem_total", total/mib)
	}
	if hw, _ := filepath.Glob(filepath.Join(d, "hwmon", "hwmon*")); len(hw) > 0 {
		h := hw[0]
		if t, ok := readInt(filepath.Join(h, "temp1_input")); ok {
			g.Set("temp", int(math.RoundToEven(float64(t)/1000)))
		}
		p, ok := readInt(filepath.Join(h, "power1_average"))
		if !ok {
			p, ok = readInt(filepath.Join(h, "power1_input"))
		}
		if ok {
			g.Set("power", float64(p)/1e6)
		}
	}
	return g, nil
}

// Reader is NVIDIA first, then AMD.
type Reader struct {
	Nvidia Nvidia
	AMD    AMD
}

// Read returns the first backend's GPU, or the last error if all failed.
func (r *Reader) Read() (*frame.Obj, error) {
	var lastErr error
	for _, b := range []interface{ Read() (*frame.Obj, error) }{&r.Nvidia, &r.AMD} {
		g, err := b.Read()
		if err != nil {
			lastErr = err
			continue
		}
		if g != nil {
			return g, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}
