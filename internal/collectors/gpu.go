package collectors

import (
	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/gpu"
)

// GPU: NVIDIA (nvidia-smi) first, then AMD (sysfs).
type GPU struct {
	noPrime
	r *gpu.Reader
}

// NewGPU returns the GPU collector.
func NewGPU() *GPU { return &GPU{r: &gpu.Reader{}} }

func (*GPU) Name() string { return "gpu" }

func (g *GPU) Collect(out *frame.Obj) error {
	o, err := g.r.Read()
	if err != nil {
		return err
	}
	if o != nil {
		out.Set("gpu", o)
	}
	return nil
}
