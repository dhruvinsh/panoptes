package frame

// step mutates a frame copy and reports whether it removed anything.
type step func(f *Obj) bool

func dropKey(key string) step {
	return func(f *Obj) bool { return f.Delete(key) }
}

func trimList(get func(f *Obj) (*Obj, string), n int) step {
	return func(f *Obj) bool {
		parent, key := get(f)
		if parent == nil {
			return false
		}
		v, _ := parent.Get(key)
		switch l := v.(type) {
		case []*Obj:
			if len(l) > n {
				parent.Set(key, l[:n])
				return true
			}
		case []any:
			if len(l) > n {
				parent.Set(key, l[:n])
				return true
			}
		}
		return false
	}
}

func dropCores(f *Obj) bool {
	cpu := f.Obj("cpu")
	return cpu != nil && cpu.Delete("cores")
}

// truncationSteps is the documented drop order (procs -> fans -> temps beyond 3 ->
// per-core values), followed by emergency fallbacks that sane input never reaches.
var truncationSteps = []step{
	dropKey("procs"),
	dropKey("fans"),
	trimList(func(f *Obj) (*Obj, string) { return f, "temps" }, 3),
	dropCores,
	trimList(func(f *Obj) (*Obj, string) { return f.Obj("disk"), "mounts" }, 1),
	dropKey("temps"),
	dropKey("disk"),
	dropKey("gpu"),
	dropKey("net"),
}

var essential = []string{"t", "ts", "tz", "up", "cpu", "mem"}

// Fit encodes a stats frame, progressively dropping detail until it fits in limit bytes.
// The input frame is never modified.
func Fit(f *Obj, limit int) []byte {
	data := Encode(f)
	if len(data) <= limit {
		return data
	}
	c := f.Clone()
	for _, s := range truncationSteps {
		if s(c) {
			data = Encode(c)
			if len(data) <= limit {
				return data
			}
		}
	}
	// Last resort: only the essentials, then only the timestamp.
	e := New()
	for _, k := range essential {
		if v, ok := c.Get(k); ok {
			e.Set(k, v)
		}
	}
	if cpu := e.Obj("cpu"); cpu != nil {
		cpu.Delete("name")
	}
	data = Encode(e)
	if len(data) <= limit {
		return data
	}
	ts, _ := f.Get("ts")
	return Encode(New("t", "s", "ts", ts))
}

// FitFrame is Fit with the protocol limit.
func FitFrame(f *Obj) []byte { return Fit(f, MaxLine) }
