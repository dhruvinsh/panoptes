package collectors

import (
	"errors"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

const (
	gib          = 1024 * 1024 * 1024
	MaxMounts    = 4
	rescanMounts = 30 * time.Second
)

var skipFSTypes = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`tmpfs devtmpfs overlay squashfs ramfs proc sysfs cgroup cgroup2 devpts
		autofs efivarfs iso9660 udf nfs nfs4 cifs smbfs smb3 sshfs fuse.sshfs fuse.portal fuse.gvfsd-fuse
		fuse.rclone nsfs tracefs debugfs securityfs bpf pstore mqueue hugetlbfs configfs fusectl binfmt_misc
		afpfs webdav devfs nullfs fuse.snapfuse zram`) {
		skipFSTypes[t] = true
	}
}

var skipMountPrefixes = []string{
	"/snap", "/var/snap", "/var/lib/docker", "/var/lib/containers", "/proc", "/sys", "/dev", "/run/user",
	"/run/credentials", "/System/Volumes", "/private/var/vm", "/Volumes/Recovery",
}

var linuxSkipDisks = []string{"loop", "ram", "zram", "dm-", "md", "sr", "fd"}

// Partition is the subset of a mounted filesystem SelectPartitions needs.
type Partition struct {
	Device, Mountpoint, Fstype, Opts string
}

func mountPriority(m string) int {
	switch {
	case m == "/" || strings.EqualFold(m, `C:\`) || strings.EqualFold(m, "C:"):
		return 0
	case m == "/home":
		return 1
	case strings.HasPrefix(m, "/boot") || m == "/efi":
		return 9
	}
	return 5
}

// SelectPartitions picks up to MaxMounts real filesystems (pure, for testability).
func SelectPartitions(parts []Partition, goos string) []string {
	sorted := append([]Partition(nil), parts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := mountPriority(sorted[i].Mountpoint), mountPriority(sorted[j].Mountpoint)
		if pi != pj {
			return pi < pj
		}
		return sorted[i].Mountpoint < sorted[j].Mountpoint
	})
	seen := map[string]bool{}
	var chosen []string
	for _, p := range sorted {
		fstype := strings.ToLower(p.Fstype)
		if goos == "windows" {
			opts := strings.ToLower(p.Opts)
			if strings.Contains(opts, "cdrom") || fstype == "" || fstype == "cdfs" || fstype == "udf" {
				continue
			}
		} else {
			if skipFSTypes[fstype] || strings.HasPrefix(fstype, "fuse.") { // fuseblk (ntfs-3g) is kept
				continue
			}
			if strings.HasPrefix(p.Device, "/dev/loop") {
				continue
			}
			skip := false
			for _, pre := range skipMountPrefixes {
				if p.Mountpoint == pre || strings.HasPrefix(p.Mountpoint, pre+"/") {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		dev := p.Device
		if dev == "" {
			dev = p.Mountpoint
		}
		if seen[dev] { // btrfs subvolumes, bind mounts
			continue
		}
		seen[dev] = true
		chosen = append(chosen, p.Mountpoint)
		if len(chosen) >= MaxMounts {
			break
		}
	}
	return chosen
}

// DisplayMount shortens a mountpoint for the screen.
func DisplayMount(m, goos string) string {
	if goos == "windows" {
		return strings.TrimRight(m, `\`)
	}
	// Removable media: show the volume label, not /run/media/<user>/<label>.
	for _, prefix := range []string{"/run/media/", "/media/", "/Volumes/"} {
		if strings.HasPrefix(m, prefix) {
			t := strings.TrimRight(m, "/")
			return t[strings.LastIndex(t, "/")+1:]
		}
	}
	return m
}

// DiskMounts: disk.mounts (GiB), rescanned every 30 s.
type DiskMounts struct {
	noPrime
	Only    []string // from the config: show exactly these instead of picking filesystems
	mounts  []string
	scanned time.Time
}

func (*DiskMounts) Name() string { return "disk.mounts" }

// normalizeMounts cleans configured mount points: at most MaxMounts, no blanks, and
// Windows drive letters in the "C:\" form gopsutil expects.
func normalizeMounts(in []string, goos string) []string {
	var out []string
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if goos == "windows" && !strings.HasSuffix(m, `\`) && len(m) == 2 && m[1] == ':' {
			m += `\`
		}
		out = append(out, m)
	}
	if len(out) > MaxMounts {
		out = out[:MaxMounts]
	}
	return out
}

func (d *DiskMounts) Collect(out *frame.Obj) error {
	if d.Only != nil {
		d.mounts = d.Only
	} else if d.scanned.IsZero() || time.Since(d.scanned) > rescanMounts {
		parts, err := disk.Partitions(false)
		if err != nil && len(parts) == 0 {
			return err
		}
		var ps []Partition
		for _, p := range parts {
			mp := p.Mountpoint
			if runtime.GOOS == "windows" {
				if !windowsLocalDrive(mp) {
					continue
				}
				if !strings.HasSuffix(mp, `\`) {
					mp += `\`
				}
			}
			ps = append(ps, Partition{Device: p.Device, Mountpoint: mp, Fstype: p.Fstype, Opts: strings.Join(p.Opts, ",")})
		}
		d.mounts = SelectPartitions(ps, runtime.GOOS)
		d.scanned = time.Now()
	}
	var mounts []*frame.Obj
	for _, m := range d.mounts {
		u, err := disk.Usage(m)
		if err != nil || u.Total == 0 {
			continue
		}
		mounts = append(mounts, frame.New("m", DisplayMount(m, runtime.GOOS), "pct", u.UsedPercent,
			"used", float64(u.Used)/gib, "total", float64(u.Total)/gib))
	}
	if len(mounts) > 0 {
		out.Sub("disk").Set("mounts", mounts)
	}
	return nil
}

func linuxPhysicalDisks() map[string]bool {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, e := range entries {
		skip := false
		for _, p := range linuxSkipDisks {
			if strings.HasPrefix(e.Name(), p) {
				skip = true
				break
			}
		}
		if !skip {
			out[e.Name()] = true
		}
	}
	return out
}

// DiskIO: disk.r / disk.w byte rates (whole physical disks on Linux, all disks elsewhere).
type DiskIO struct {
	rates RateTracker
}

func (*DiskIO) Name() string { return "disk.io" }

func (*DiskIO) totals() (float64, float64, error) {
	per, err := disk.IOCounters()
	if err != nil && len(per) == 0 {
		return 0, 0, err
	}
	if len(per) == 0 {
		return 0, 0, errors.New("no disk I/O counters")
	}
	var wanted map[string]bool
	if runtime.GOOS == "linux" {
		wanted = linuxPhysicalDisks()
	}
	var r, w float64
	for name, c := range per {
		if wanted != nil && !wanted[name] {
			continue
		}
		r += float64(c.ReadBytes)
		w += float64(c.WriteBytes)
	}
	return r, w, nil
}

func (d *DiskIO) Prime() error {
	r, w, err := d.totals()
	if err != nil {
		return err
	}
	d.rates.Update(map[string]float64{"r": r, "w": w})
	return nil
}

func (d *DiskIO) Collect(out *frame.Obj) error {
	r, w, err := d.totals()
	if err != nil {
		return err
	}
	if rates := d.rates.Update(map[string]float64{"r": r, "w": w}); rates != nil {
		o := out.Sub("disk")
		o.Set("r", rates["r"])
		o.Set("w", rates["w"])
	}
	return nil
}
