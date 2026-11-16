// Command benchdays evaluates the policy line-up on every day type (weekday, weekend, festival, ...)
// using the already-trained policies, and writes internal/assets/benchmark_daytypes.json.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"riderl/internal/city"
	"riderl/internal/policy"
	"riderl/internal/sim"
	"riderl/internal/train"
)

type Out struct {
	Seeds int                        `json:"seeds"`
	Days  map[string]train.Benchmark `json:"days"`
	Order []string                   `json:"order"`
}

func main() {
	dir := flag.String("assets", "internal/assets", "assets directory")
	n := flag.Int("seeds", 24, "days per day type")
	main40 := flag.Bool("main", true, "also rewrite benchmark.json")
	flag.Parse()
	c, err := city.Load()
	must(err)
	st := sim.NewStatic(c)
	rb, err := os.ReadFile(filepath.Join(*dir, "rule.json"))
	must(err)
	rule, err := policy.LoadRule(rb)
	must(err)
	mb, err := os.ReadFile(filepath.Join(*dir, "mlp.json"))
	must(err)
	mlp, err := policy.LoadMLP(mb)
	must(err)
	cands := train.StandardCandidates(rule, mlp)
	if *main40 {
		fmt.Println("== main benchmark (40 held-out days, default config)")
		b := train.RunBenchmark(st, sim.DefaultConfig(), cands, train.Seeds(5000, 40), "No dynamic fee", "Rule (tuned)", nil)
		bs, _ := json.Marshal(b)
		must(os.WriteFile(filepath.Join(*dir, "benchmark.json"), bs, 0o644))
		for _, e := range b.Entries {
			fmt.Printf("  %-16s fulfil %.3f return %8.0f vsFlat %+7.0f [%+.0f,%+.0f] vsRule %+6.0f [%+.0f,%+.0f]\n", e.Name, e.Mean.Fulfilment, mean(e.Returns), e.VsFlat[0], e.VsFlat[1], e.VsFlat[2], e.VsRule[0], e.VsRule[1], e.VsRule[2])
		}
	}
	out := Out{Seeds: *n, Days: map[string]train.Benchmark{}}
	for _, d := range sim.DayTypes {
		cfg := sim.DefaultConfig()
		cfg.DayType, cfg.Rain = d.Key, 0
		fmt.Printf("== %s\n", d.Label)
		b := train.RunBenchmark(st, cfg, cands, train.Seeds(9000, *n), "No dynamic fee", "Rule (tuned)", nil)
		for _, e := range b.Entries {
			fmt.Printf("  %-16s fulfil %.3f return %8.0f vsFlat %+7.0f [%+.0f,%+.0f] vsRule %+6.0f [%+.0f,%+.0f]\n", e.Name, e.Mean.Fulfilment, mean(e.Returns), e.VsFlat[0], e.VsFlat[1], e.VsFlat[2], e.VsRule[0], e.VsRule[1], e.VsRule[2])
		}
		out.Days[d.Key] = b
		out.Order = append(out.Order, d.Key)
	}
	bs, _ := json.Marshal(out)
	must(os.WriteFile(filepath.Join(*dir, "benchmark_daytypes.json"), bs, 0o644))
}

func mean(a []float64) float64 {
	s := 0.0
	for _, x := range a {
		s += x
	}
	return s / float64(len(a))
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
