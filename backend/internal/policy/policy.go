// Package policy holds surge-pricing policies: flat, demand/supply rule, and an MLP.
package policy

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"

	"riderl/internal/sim"
)

// Policy maps the per-ward observation matrix to an action per ward: fee level + NumFee*incentive level.
type Policy interface {
	Name() string
	Act(obs []float32, z int, out []int)
}

type Flat struct{ Level int }

func (p Flat) Name() string { return fmt.Sprintf("flat action %d", p.Level) }
func (p Flat) Act(_ []float32, z int, out []int) {
	for i := 0; i < z; i++ {
		out[i] = p.Level
	}
}

// Rule is the classic demand/supply-ratio surge heuristic with tunable thresholds.
type Rule struct {
	Label         string    `json:"label"`
	Thresholds    []float64 `json:"thresholds"`     // fee level from the demand/supply ratio (4 cut-points)
	IncThresholds []float64 `json:"inc_thresholds"` // captain incentive level (2 cut-points)
}

func (p Rule) Name() string {
	if p.Label != "" {
		return p.Label
	}
	return "rule"
}
func (p Rule) Act(obs []float32, z int, out []int) {
	for i := 0; i < z; i++ {
		r := float64(obs[i*sim.NumFeatures+sim.FeatureRatio])
		fee := sort.SearchFloat64s(p.Thresholds, r+1e-9)
		inc := sort.SearchFloat64s(p.IncThresholds, r+1e-9)
		out[i] = fee + sim.NumFee*inc
	}
}

// MLP is a feed-forward (tanh) actor, greedy argmax. Weights are row-major [in][out].
type MLP struct {
	Label string      `json:"label"`
	Sizes []int       `json:"sizes"`
	W     [][]float64 `json:"w"`
	B     [][]float64 `json:"b"`
	buf   [][]float64
}

func NewMLP(label string, sizes []int, seed func() float64) *MLP {
	m := &MLP{Label: label, Sizes: sizes}
	for l := 0; l < len(sizes)-1; l++ {
		in, out := sizes[l], sizes[l+1]
		scale := math.Sqrt(2 / float64(in))
		if l == len(sizes)-2 {
			scale = 0.01
		}
		w := make([]float64, in*out)
		for i := range w {
			w[i] = seed() * scale
		}
		m.W = append(m.W, w)
		m.B = append(m.B, make([]float64, out))
	}
	return m
}

func (m *MLP) Name() string {
	if m.Label != "" {
		return m.Label
	}
	return "mlp"
}

func (m *MLP) ensure() {
	if len(m.buf) == len(m.Sizes) {
		return
	}
	m.buf = make([][]float64, len(m.Sizes))
	for i, s := range m.Sizes {
		m.buf[i] = make([]float64, s)
	}
}

// Logits runs a forward pass for one feature row. Not safe for concurrent use of one MLP.
func (m *MLP) Logits(x []float32) []float64 {
	m.ensure()
	for i, v := range x {
		m.buf[0][i] = float64(v)
	}
	for l := 0; l < len(m.Sizes)-1; l++ {
		in, out := m.Sizes[l], m.Sizes[l+1]
		src, dst, w, b := m.buf[l], m.buf[l+1], m.W[l], m.B[l]
		for j := 0; j < out; j++ {
			s := b[j]
			for i := 0; i < in; i++ {
				s += src[i] * w[i*out+j]
			}
			if l < len(m.Sizes)-2 {
				s = math.Tanh(s)
			}
			dst[j] = s
		}
	}
	return m.buf[len(m.Sizes)-1]
}

func (m *MLP) Act(obs []float32, z int, out []int) {
	for i := 0; i < z; i++ {
		lg := m.Logits(obs[i*sim.NumFeatures : (i+1)*sim.NumFeatures])
		best := 0
		for k := 1; k < len(lg); k++ {
			if lg[k] > lg[best] {
				best = k
			}
		}
		out[i] = best
	}
}

// Clone copies weights (separate scratch buffers => safe per goroutine).
func (m *MLP) Clone() *MLP {
	c := &MLP{Label: m.Label, Sizes: append([]int(nil), m.Sizes...)}
	for l := range m.W {
		c.W = append(c.W, append([]float64(nil), m.W[l]...))
		c.B = append(c.B, append([]float64(nil), m.B[l]...))
	}
	return c
}

// Flat/Unflat for evolution strategies.
func (m *MLP) Flatten() []float64 {
	var v []float64
	for l := range m.W {
		v = append(v, m.W[l]...)
		v = append(v, m.B[l]...)
	}
	return v
}
func (m *MLP) SetFlat(v []float64) {
	i := 0
	for l := range m.W {
		i += copy(m.W[l], v[i:])
		i += copy(m.B[l], v[i:])
	}
}

func SaveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func LoadMLP(b []byte) (*MLP, error) {
	var m MLP
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func LoadRule(b []byte) (Rule, error) {
	var r Rule
	err := json.Unmarshal(b, &r)
	return r, err
}
