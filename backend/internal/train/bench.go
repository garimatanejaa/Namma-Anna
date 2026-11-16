package train

import (
	"riderl/internal/policy"
	"riderl/internal/sim"
)

type Entry struct {
	Name        string      `json:"name"`
	Mean        sim.Metrics `json:"mean"`
	ReturnSD    float64     `json:"return_sd"`
	Returns     []float64   `json:"returns"`
	VsFlat      [3]float64  `json:"vs_flat"` // mean, lo, hi of paired return difference
	VsRule      [3]float64  `json:"vs_rule"`
	Description string      `json:"description"`
}

type Benchmark struct {
	Seeds    int        `json:"seeds"`
	Config   sim.Config `json:"config"`
	Baseline string     `json:"baseline"`
	RuleName string     `json:"rule_name"`
	Entries  []Entry    `json:"entries"`
}

type Candidate struct {
	Name, Description string
	F                 Factory
}

// RunBenchmark evaluates every candidate on the same seeds and attaches paired CIs.
// progress (optional) is called after each candidate finishes.
func RunBenchmark(st *sim.Static, cfg sim.Config, cands []Candidate, seeds []uint64, baseline, rule string, progress func(done, total int, name string)) Benchmark {
	b := Benchmark{Seeds: len(seeds), Config: cfg, Baseline: baseline, RuleName: rule}
	all := map[string][]float64{}
	for i, c := range cands {
		ms := Evaluate(st, cfg, c.F, seeds)
		rets := make([]float64, len(ms))
		for k, m := range ms {
			rets[k] = m.Return
		}
		all[c.Name] = rets
		b.Entries = append(b.Entries, Entry{Name: c.Name, Description: c.Description, Mean: MeanMetrics(ms), ReturnSD: StdDev(rets), Returns: rets})
		if progress != nil {
			progress(i+1, len(cands), c.Name)
		}
	}
	for i := range b.Entries {
		e := &b.Entries[i]
		if base, ok := all[baseline]; ok {
			m, lo, hi := PairedCI(e.Returns, base, 2000)
			e.VsFlat = [3]float64{m, lo, hi}
		}
		if r, ok := all[rule]; ok {
			m, lo, hi := PairedCI(e.Returns, r, 2000)
			e.VsRule = [3]float64{m, lo, hi}
		}
	}
	return b
}

// StandardCandidates builds the policy line-up used everywhere (CLI, API, UI).
func StandardCandidates(rule policy.Rule, mlp *policy.MLP) []Candidate {
	def := []float64{0.6, 1.0, 1.5, 2.2}
	c := []Candidate{
		{"No dynamic fee", "Fare stays at the meter fare (autos) or base fare (bikes); no captain incentive.", func() policy.Policy { return policy.Flat{Level: 0} }},
		{"Flat mid fee", "A constant mid-level fee everywhere: x1.2 on bikes and scooters, +Rs10 boost on autos.", func() policy.Policy { return policy.Flat{Level: 2} }},
		{"Flat incentive", "No fee, but every captain trip gets a flat Rs8 incentive everywhere.", func() policy.Policy { return policy.Flat{Level: sim.NumFee} }},
		{"Rule (default)", "Fee and incentive follow the local demand/supply ratio, textbook thresholds.", func() policy.Policy {
			return policy.Rule{Label: "Rule (default)", Thresholds: def, IncThresholds: []float64{1.5, 2.5}}
		}},
		{"Rule (tuned)", "Same rule, six thresholds found by random search on separate days.", func() policy.Policy {
			r := rule
			r.Label = "Rule (tuned)"
			return r
		}},
	}
	if mlp != nil {
		c = append(c, Candidate{"Neural policy", "Shared MLP over ward features choosing fee and incentive level: behaviour-cloned from the rule, then evolution-strategies fine-tuned.", func() policy.Policy {
			cl := mlp.Clone()
			cl.Label = "Neural policy"
			return cl
		}})
	}
	return c
}
