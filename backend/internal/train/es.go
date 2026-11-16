package train

import (
	"math"
	"math/rand/v2"
	"time"

	"riderl/internal/policy"
	"riderl/internal/sim"
)

// HistPoint is one ES iteration (for the training curve shown in the UI).
type HistPoint struct {
	Iter    int     `json:"iter"`
	Center  float64 `json:"center"`
	PopMean float64 `json:"pop_mean"`
	Best    float64 `json:"best"`
}

// ES fine-tunes the MLP with antithetic evolution strategies and common random numbers:
// every candidate in an iteration faces the same simulated days, so differences are low-variance.
func ES(st *sim.Static, cfg sim.Config, m *policy.MLP, iters, pairs, seedsPerIter int, sigma, lr float64, log func(string, ...any)) []HistPoint {
	theta := m.Flatten()
	n := len(theta)
	r := rand.New(rand.NewPCG(21, 22))
	mom, vel := make([]float64, n), make([]float64, n)
	var hist []HistPoint
	t0 := time.Now()
	for it := 0; it < iters; it++ {
		seeds := Seeds(300000+uint64(it*seedsPerIter), seedsPerIter)
		eps := make([][]float64, pairs)
		cands := [][]float64{theta}
		for p := 0; p < pairs; p++ {
			e := make([]float64, n)
			for i := range e {
				e[i] = r.NormFloat64()
			}
			eps[p] = e
			plus, minus := make([]float64, n), make([]float64, n)
			for i := range e {
				plus[i] = theta[i] + sigma*e[i]
				minus[i] = theta[i] - sigma*e[i]
			}
			cands = append(cands, plus, minus)
		}
		fit := make([]float64, len(cands))
		done := make(chan struct{})
		go func() {
			// evaluate candidates sequentially; each Evaluate call already parallelises over seeds
			for i, c := range cands {
				c := c
				f := func() policy.Policy { cl := m.Clone(); cl.SetFlat(c); return cl }
				fit[i] = MeanReturn(Evaluate(st, cfg, f, seeds))
			}
			close(done)
		}()
		<-done
		adv := make([]float64, pairs)
		sd := 0.0
		for p := 0; p < pairs; p++ {
			adv[p] = fit[1+2*p] - fit[2+2*p]
			sd += adv[p] * adv[p]
		}
		sd = math.Sqrt(sd/float64(pairs)) + 1e-8
		grad := make([]float64, n)
		for p := 0; p < pairs; p++ {
			a := adv[p] / sd
			for i := range grad {
				grad[i] += a * eps[p][i] / (float64(pairs) * sigma) / 2
			}
		}
		c1, c2 := 1-math.Pow(0.9, float64(it+1)), 1-math.Pow(0.999, float64(it+1))
		for i := range theta {
			mom[i] = 0.9*mom[i] + 0.1*grad[i]
			vel[i] = 0.999*vel[i] + 0.001*grad[i]*grad[i]
			theta[i] += lr * (mom[i] / c1) / (math.Sqrt(vel[i]/c2) + 1e-8) * sigma
		}
		mean, best := 0.0, math.Inf(-1)
		for _, f := range fit {
			mean += f / float64(len(fit))
			best = math.Max(best, f)
		}
		hist = append(hist, HistPoint{it, fit[0], mean, best})
		log("ES %3d/%d  center %.0f  pop %.0f  best %.0f  (%.0fs)", it+1, iters, fit[0], mean, best, time.Since(t0).Seconds())
	}
	m.SetFlat(theta)
	return hist
}
