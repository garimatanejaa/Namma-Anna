// Package train contains Go-native policy search: rule tuning, behaviour cloning and
// evolution strategies, plus the paired benchmark used by the API and the UI.
package train

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"

	"riderl/internal/policy"
	"riderl/internal/sim"
)

// Factory builds an independent policy instance per worker (MLPs hold scratch buffers).
type Factory func() policy.Policy

// RunEpisode plays one day and returns its metrics (with Return filled in).
func RunEpisode(env *sim.Env, pol policy.Policy, seed uint64) sim.Metrics {
	obs := env.Reset(seed)
	act := make([]int, env.S.Z)
	ret := 0.0
	for !env.Done() {
		pol.Act(obs, env.S.Z, act)
		env.Step(act)
		for _, r := range env.Reward {
			ret += r
		}
		obs = env.Observe()
	}
	m := env.Metrics()
	m.Return = ret
	return m
}

// Evaluate runs seeds in parallel; results are in seed order.
func Evaluate(st *sim.Static, cfg sim.Config, f Factory, seeds []uint64) []sim.Metrics {
	out := make([]sim.Metrics, len(seeds))
	w := runtime.NumCPU()
	if w > len(seeds) {
		w = len(seeds)
	}
	var wg sync.WaitGroup
	ch := make(chan int)
	for k := 0; k < w; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			env := sim.NewEnv(st, cfg)
			pol := f()
			for i := range ch {
				out[i] = RunEpisode(env, pol, seeds[i])
			}
		}()
	}
	for i := range seeds {
		ch <- i
	}
	close(ch)
	wg.Wait()
	return out
}

func MeanReturn(ms []sim.Metrics) float64 {
	s := 0.0
	for _, m := range ms {
		s += m.Return
	}
	return s / float64(len(ms))
}

func Seeds(start uint64, n int) []uint64 {
	s := make([]uint64, n)
	for i := range s {
		s[i] = start + uint64(i)
	}
	return s
}

// MeanMetrics averages every field.
func MeanMetrics(ms []sim.Metrics) sim.Metrics {
	var o sim.Metrics
	n := float64(len(ms))
	for _, m := range ms {
		o.Requests += m.Requests / n
		o.Completed += m.Completed / n
		o.Fulfilment += m.Fulfilment / n
		o.NoSupplyRate += m.NoSupplyRate / n
		o.PriceDeclineRate += m.PriceDeclineRate / n
		o.RejectedRate += m.RejectedRate / n
		o.CancelRate += m.CancelRate / n
		o.AvgETAMin += m.AvgETAMin / n
		o.AvgSurgePaid += m.AvgSurgePaid / n
		o.AvgQuoteMult += m.AvgQuoteMult / n
		o.GMV += m.GMV / n
		o.CaptainNetPerHour += m.CaptainNetPerHour / n
		o.CaptainsOnline += m.CaptainsOnline / n
		o.EarningsGini += m.EarningsGini / n
		o.PlatformRevenue += m.PlatformRevenue / n
		o.RevPerTrip += m.RevPerTrip / n
		o.MeanLoyalty += m.MeanLoyalty / n
		o.SatisfiedCaptains += m.SatisfiedCaptains / n
		o.Return += m.Return / n
	}
	return o
}

// PairedCI returns mean(a-b) and a bootstrap 95% interval.
func PairedCI(a, b []float64, nBoot int) (mean, lo, hi float64) {
	d := make([]float64, len(a))
	for i := range a {
		d[i] = a[i] - b[i]
		mean += d[i] / float64(len(a))
	}
	r := rand.New(rand.NewPCG(7, 11))
	bs := make([]float64, nBoot)
	for k := range bs {
		s := 0.0
		for range d {
			s += d[r.IntN(len(d))]
		}
		bs[k] = s / float64(len(d))
	}
	sort.Float64s(bs)
	return mean, bs[int(0.025*float64(nBoot))], bs[int(0.975*float64(nBoot))-1]
}

func StdDev(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		m += x / float64(len(v))
	}
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(maxI(1, len(v)-1)))
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
