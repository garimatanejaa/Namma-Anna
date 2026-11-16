package train

import (
	"math"
	"math/rand/v2"

	"riderl/internal/policy"
	"riderl/internal/sim"
)

// CollectBC rolls out a teacher (with some exploration) and records (features, action) rows.
func CollectBC(st *sim.Static, cfg sim.Config, teacher policy.Policy, episodes int, maxRows int, seedBase uint64) ([][]float32, []int) {
	env := sim.NewEnv(st, cfg)
	r := rand.New(rand.NewPCG(5, 6))
	var X [][]float32
	var Y []int
	z := st.Z
	act := make([]int, z)
	for ep := 0; ep < episodes; ep++ {
		obs := env.Reset(seedBase + uint64(ep))
		eps := 0.0
		if ep%2 == 1 {
			eps = 0.3
		}
		for !env.Done() {
			teacher.Act(obs, z, act)
			for w := 0; w < z; w++ {
				// subsample rows to keep the dataset manageable
				if r.Float64() < float64(maxRows)/float64(episodes*z*sim.StepsPerDay) {
					row := make([]float32, sim.NumFeatures)
					copy(row, obs[w*sim.NumFeatures:(w+1)*sim.NumFeatures])
					X = append(X, row)
					Y = append(Y, act[w])
				}
			}
			if eps > 0 {
				for w := 0; w < z; w++ {
					if r.Float64() < eps {
						act[w] = r.IntN(sim.NumActions)
					}
				}
			}
			env.Step(act)
			obs = env.Observe()
		}
	}
	return X, Y
}

// TrainBC fits the actor to the teacher with label-smoothed cross-entropy (Adam, per-sample backprop).
func TrainBC(m *policy.MLP, X [][]float32, Y []int, epochs int, smoothing float64, log func(string, ...any)) float64 {
	L := len(m.Sizes) - 1
	nOut := m.Sizes[L]
	r := rand.New(rand.NewPCG(9, 10))
	// Adam state
	mW, vW, mB, vB := zerosLike(m.W), zerosLike(m.W), zerosLike(m.B), zerosLike(m.B)
	gW, gB := zerosLike(m.W), zerosLike(m.B)
	acts := make([][]float64, L+1)
	for i, s := range m.Sizes {
		acts[i] = make([]float64, s)
	}
	deltas := make([][]float64, L+1)
	for i, s := range m.Sizes {
		deltas[i] = make([]float64, s)
	}
	probs := make([]float64, nOut)
	idx := r.Perm(len(X))
	const bs = 512
	step := 0
	lr := 2e-3
	acc := 0.0
	for ep := 0; ep < epochs; ep++ {
		r.Shuffle(len(idx), func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
		loss, correct := 0.0, 0.0
		for s := 0; s < len(idx); s += bs {
			e := s + bs
			if e > len(idx) {
				e = len(idx)
			}
			clear2(gW)
			clear2(gB)
			for _, ix := range idx[s:e] {
				for i, v := range X[ix] {
					acts[0][i] = float64(v)
				}
				for l := 0; l < L; l++ {
					in, out := m.Sizes[l], m.Sizes[l+1]
					for j := 0; j < out; j++ {
						sum := m.B[l][j]
						for i := 0; i < in; i++ {
							sum += acts[l][i] * m.W[l][i*out+j]
						}
						if l < L-1 {
							sum = math.Tanh(sum)
						}
						acts[l+1][j] = sum
					}
				}
				mx := acts[L][0]
				for _, v := range acts[L] {
					mx = math.Max(mx, v)
				}
				tot := 0.0
				for k := 0; k < nOut; k++ {
					probs[k] = math.Exp(acts[L][k] - mx)
					tot += probs[k]
				}
				best := 0
				for k := 0; k < nOut; k++ {
					probs[k] /= tot
					if probs[k] > probs[best] {
						best = k
					}
				}
				if best == Y[ix] {
					correct++
				}
				for k := 0; k < nOut; k++ {
					t := smoothing / float64(nOut)
					if k == Y[ix] {
						t += 1 - smoothing
					}
					deltas[L][k] = probs[k] - t
					loss -= t * math.Log(probs[k]+1e-12)
				}
				for l := L - 1; l >= 0; l-- {
					in, out := m.Sizes[l], m.Sizes[l+1]
					for i := 0; i < in; i++ {
						d := 0.0
						for j := 0; j < out; j++ {
							gW[l][i*out+j] += acts[l][i] * deltas[l+1][j]
							d += m.W[l][i*out+j] * deltas[l+1][j]
						}
						if l > 0 {
							d *= 1 - acts[l][i]*acts[l][i]
						}
						deltas[l][i] = d
					}
					for j := 0; j < out; j++ {
						gB[l][j] += deltas[l+1][j]
					}
				}
			}
			step++
			n := float64(e - s)
			b1, b2 := 0.9, 0.999
			c1, c2 := 1-math.Pow(b1, float64(step)), 1-math.Pow(b2, float64(step))
			for l := 0; l < L; l++ {
				for i := range m.W[l] {
					g := gW[l][i] / n
					mW[l][i] = b1*mW[l][i] + (1-b1)*g
					vW[l][i] = b2*vW[l][i] + (1-b2)*g*g
					m.W[l][i] -= lr * (mW[l][i] / c1) / (math.Sqrt(vW[l][i]/c2) + 1e-8)
				}
				for i := range m.B[l] {
					g := gB[l][i] / n
					mB[l][i] = b1*mB[l][i] + (1-b1)*g
					vB[l][i] = b2*vB[l][i] + (1-b2)*g*g
					m.B[l][i] -= lr * (mB[l][i] / c1) / (math.Sqrt(vB[l][i]/c2) + 1e-8)
				}
			}
		}
		acc = correct / float64(len(idx))
		if ep%4 == 0 || ep == epochs-1 {
			log("BC epoch %2d  loss %.4f  acc %.3f", ep, loss/float64(len(idx)), acc)
		}
	}
	return acc
}

func zerosLike(a [][]float64) [][]float64 {
	o := make([][]float64, len(a))
	for i := range a {
		o[i] = make([]float64, len(a[i]))
	}
	return o
}

func clear2(a [][]float64) {
	for i := range a {
		for j := range a[i] {
			a[i][j] = 0
		}
	}
}
