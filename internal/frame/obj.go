// Package frame holds the ordered message type used for every protocol message and the
// encoder that enforces the PROTOCOL.md line rules (compact JSON, floats rounded to one
// decimal, non-finite values dropped, strings capped, at most 2048 bytes per line).
package frame

// Obj is an insertion-ordered JSON object. Keys keep the order in which they were first
// set, so encoded frames are stable from one second to the next.
//
// Supported value types: nil, bool, int, int64, uint64, float64, string, *Obj, []any,
// []float64, []*Obj and []string. Anything else is encoded as its fmt.Sprint string.
type Obj struct {
	keys []string
	m    map[string]any
}

// New builds an object from alternating key/value arguments.
func New(kv ...any) *Obj {
	o := &Obj{m: make(map[string]any, len(kv)/2)}
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

func (o *Obj) init() {
	if o.m == nil {
		o.m = map[string]any{}
	}
}

// Set stores v under k (keeping k's original position if it already exists).
func (o *Obj) Set(k string, v any) *Obj {
	o.init()
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
	return o
}

// Get returns the value under k.
func (o *Obj) Get(k string) (any, bool) {
	if o == nil || o.m == nil {
		return nil, false
	}
	v, ok := o.m[k]
	return v, ok
}

// Has reports whether k is present.
func (o *Obj) Has(k string) bool {
	_, ok := o.Get(k)
	return ok
}

// Obj returns the nested object under k, or nil.
func (o *Obj) Obj(k string) *Obj {
	v, _ := o.Get(k)
	sub, _ := v.(*Obj)
	return sub
}

// Sub returns the nested object under k, creating it if needed (like dict.setdefault).
func (o *Obj) Sub(k string) *Obj {
	if sub := o.Obj(k); sub != nil {
		return sub
	}
	sub := New()
	o.Set(k, sub)
	return sub
}

// Delete removes k and reports whether it held a non-nil value (like dict.pop(k) is not None).
func (o *Obj) Delete(k string) bool {
	if o == nil || o.m == nil {
		return false
	}
	v, ok := o.m[k]
	if !ok {
		return false
	}
	delete(o.m, k)
	for i, key := range o.keys {
		if key == k {
			o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
			break
		}
	}
	return v != nil
}

// Keys returns the keys in insertion order.
func (o *Obj) Keys() []string {
	if o == nil {
		return nil
	}
	return append([]string(nil), o.keys...)
}

// Len is the number of keys.
func (o *Obj) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

// Clone returns a deep copy.
func (o *Obj) Clone() *Obj {
	if o == nil {
		return nil
	}
	c := &Obj{keys: append([]string(nil), o.keys...), m: make(map[string]any, len(o.m))}
	for k, v := range o.m {
		c.m[k] = cloneValue(v)
	}
	return c
}

func cloneValue(v any) any {
	switch x := v.(type) {
	case *Obj:
		return x.Clone()
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = cloneValue(e)
		}
		return out
	case []*Obj:
		out := make([]*Obj, len(x))
		for i, e := range x {
			out[i] = e.Clone()
		}
		return out
	case []float64:
		return append([]float64(nil), x...)
	case []string:
		return append([]string(nil), x...)
	}
	return v
}

// Merge deep-merges src into dst: nested objects are merged, everything else replaced.
func Merge(dst, src *Obj) {
	for _, k := range src.keys {
		v := src.m[k]
		if sv, ok := v.(*Obj); ok {
			if dv := dst.Obj(k); dv != nil {
				Merge(dv, sv)
				continue
			}
		}
		dst.Set(k, v)
	}
}

// Value returns the value under k, or nil.
func (o *Obj) Value(k string) any {
	v, _ := o.Get(k)
	return v
}
