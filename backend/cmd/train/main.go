// Command train tunes the rule policy, trains the neural policy (BC + ES) and writes the
// embedded assets (rule.json, mlp.json, benchmark.json, training.json).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"riderl/internal/city"
	"riderl/internal/policy"
	"riderl/internal/sim"
	"riderl/internal/train"
)

func main() {
	out := flag.String("out", "internal/assets", "output directory")
	esIters := flag.Int("es", 40, "ES iterations")
	ruleCands := flag.Int("rule", 80, "rule-search candidates")
	bcEps := flag.Int("bc-episodes", 24, "behaviour-cloning rollouts")
	evalSeeds := flag.Int("seeds", 40, "held-out evaluation days")
	flag.Parse()
	log := func(f string, a ...any) { fmt.Printf(f+"\n", a...) }

	c, err := city.Load()
	must(err)
	st := sim.NewStatic(c)
	cfg := sim.DefaultConfig()
	t0 := time.Now()

	log("== 1/4 tuning the rule policy")
	rule := train.TuneRule(st, cfg, *ruleCands, 2000, log)
	must(policy.SaveJSON(filepath.Join(*out, "rule.json"), rule))

	log("== 2/4 behaviour cloning the neural policy from the tuned rule")
	rng := rand.New(rand.NewPCG(3, 4))
	mlp := policy.NewMLP("Neural policy", []int{sim.NumFeatures, 32, 32, sim.NumActions}, rng.NormFloat64)
	X, Y := train.CollectBC(st, cfg, rule, *bcEps, 150000, 7000)
	log("collected %d rows", len(X))
	train.TrainBC(mlp, X, Y, 16, 0.12, log)
	must(policy.SaveJSON(filepath.Join(*out, "mlp_bc.json"), mlp))

	log("== 3/4 evolution-strategies fine-tuning")
	hist := train.ES(st, cfg, mlp, *esIters, 12, 4, 0.04, 0.03, log)
	must(policy.SaveJSON(filepath.Join(*out, "mlp.json"), mlp))
	must(policy.SaveJSON(filepath.Join(*out, "training.json"), hist))

	log("== 4/4 benchmark on %d held-out days", *evalSeeds)
	cands := train.StandardCandidates(rule, mlp)
	b := train.RunBenchmark(st, cfg, cands, train.Seeds(5000, *evalSeeds), "No dynamic fee", "Rule (tuned)", func(d, t int, n string) { log("  %d/%d %s", d, t, n) })
	must(policy.SaveJSON(filepath.Join(*out, "benchmark.json"), b))
	for _, e := range b.Entries {
		fmt.Printf("%-16s return %8.0f  fulfil %.3f  vsFlat %+8.0f [%+.0f,%+.0f]  vsRule %+7.0f [%+.0f,%+.0f]\n", e.Name, e.Mean.Return, e.Mean.Fulfilment, e.VsFlat[0], e.VsFlat[1], e.VsFlat[2], e.VsRule[0], e.VsRule[1], e.VsRule[2])
	}
	log("done in %.0fs", time.Since(t0).Seconds())
	_ = json.Marshal
	_ = os.Stdout
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
