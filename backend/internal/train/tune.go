package train

import (
	"math/rand/v2"
	"sort"

	"riderl/internal/policy"
	"riderl/internal/sim"
)

// TuneRule random-searches the fee (4) and incentive (2) ratio thresholds of the demand/supply rule on tuning seeds.
func TuneRule(st *sim.Static, cfg sim.Config, candidates int, seedBase uint64, log func(string, ...any)) policy.Rule {
	r := rand.New(rand.NewPCG(1, 2))
	tune := Seeds(seedBase, 8)
	best := policy.Rule{Label: "rule (tuned)", Thresholds: []float64{0.6, 1.0, 1.5, 2.2}, IncThresholds: []float64{3, 3}}
	bestR := MeanReturn(Evaluate(st, cfg, func() policy.Policy { return best }, tune))
	for i := 0; i < candidates; i++ {
		th := make([]float64, 4)
		for k := range th {
			th[k] = 0.1 + 2.9*r.Float64()
		}
		sort.Float64s(th)
		ic := []float64{0.3 + 3*r.Float64(), 0.3 + 3*r.Float64()}
		sort.Float64s(ic)
		cand := policy.Rule{Label: "rule (tuned)", Thresholds: th, IncThresholds: ic}
		ret := MeanReturn(Evaluate(st, cfg, func() policy.Policy { return cand }, tune))
		if ret > bestR {
			best, bestR = cand, ret
			log("rule search %3d/%d  best %.0f  fee %.2f inc %.2f", i+1, candidates, bestR, th, ic)
		}
	}
	return best
}
