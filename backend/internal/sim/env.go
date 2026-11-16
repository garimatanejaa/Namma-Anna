package sim

import (
	"math"
	"math/rand/v2"
	"sort"

	"riderl/internal/city"
)

type nearEntry struct {
	w    int32
	dist float32
}

// Static is the immutable, shareable part of the world (built once per city+config).
type Static struct {
	City   *city.City
	Z      int
	near   [][]nearEntry // per ward: wards sorted by distance (within pickup range)
	odCum  [][]float32   // per origin: cumulative destination probabilities
	shares [4][]float64  // normalised component weights per ward
	prof   [4][]float64  // normalised time profiles over the day
	area   []float64
	zw     []float64 // captain home-ward weights
}

func NewStatic(c *city.City) *Static {
	z := len(c.Wards)
	s := &Static{City: c, Z: z, area: make([]float64, z)}
	// ward component shares
	for k := range s.shares {
		s.shares[k] = make([]float64, z)
	}
	for i, w := range c.Wards {
		s.shares[0][i], s.shares[1][i], s.shares[2][i], s.shares[3][i] = w.Resid, w.Employ, w.Transit, w.Retail
		s.area[i] = w.AreaKm2
	}
	// time profiles
	fn := [4]func(float64) float64{profResid, profEmploy, profTransit, profRetail}
	for k := 0; k < 4; k++ {
		s.prof[k] = make([]float64, StepsPerDay)
		sum := 0.0
		for t := 0; t < StepsPerDay; t++ {
			s.prof[k][t] = fn[k](StartHour + float64(t)*DtMin/60)
			sum += s.prof[k][t]
		}
		for t := range s.prof[k] {
			s.prof[k][t] /= sum
		}
	}
	// nearest-ward lists (pickup candidates) sorted by distance, within 9 km
	s.near = make([][]nearEntry, z)
	for i := 0; i < z; i++ {
		var l []nearEntry
		for j := 0; j < z; j++ {
			d := c.DistKm[i][j]
			if d <= 9 {
				l = append(l, nearEntry{int32(j), d})
			}
		}
		sort.Slice(l, func(a, b int) bool { return l[a].dist < l[b].dist })
		s.near[i] = l
	}
	// destination attractiveness and gravity OD
	attr := make([]float64, z)
	for i, w := range c.Wards {
		attr[i] = 0.35*w.Resid + 0.35*w.Employ + 0.2*w.Transit + 0.1*w.Retail + 1e-4
	}
	s.odCum = make([][]float32, z)
	for i := 0; i < z; i++ {
		row := make([]float64, z)
		tot := 0.0
		for j := 0; j < z; j++ {
			d := float64(c.DistKm[i][j])
			if i == j {
				d = 1.5
			}
			row[j] = attr[j] * math.Exp(-d/5.5)
			tot += row[j]
		}
		cum := make([]float32, z)
		acc := 0.0
		for j := 0; j < z; j++ {
			acc += row[j] / tot
			cum[j] = float32(acc)
		}
		cum[z-1] = 1
		s.odCum[i] = cum
	}
	// captains live mostly in residential wards
	s.zw = make([]float64, z)
	for i, w := range c.Wards {
		s.zw[i] = 0.7*w.Resid + 0.2*w.Transit + 0.1*w.Employ + 1e-4
	}
	return s
}

// Env is one simulated day. Not safe for concurrent use; make one per goroutine.
type Env struct {
	S   *Static
	Cfg Config
	rng *rand.Rand
	T   int

	// captains
	capZone, capDest                    []int32
	capState                            []uint8 // 0 off, 1 idle, 2 busy
	capFreeAt                           []int32
	capStart                            []int32
	capMaxOnline                        []int32
	capOnline                           []int32
	capResv, capTarget, capEarn, capFee []float64
	capFlex, capLogged                  []bool
	capMode                             []uint8
	idleM                               [NumModes][][]int32
	Level, Inc                          []int
	prem                                []float64

	demandMult            float64
	day                   *DayType
	compMult              [StepsPerDay][4]float64
	evMult                [][]float32
	rainStart, rainEnd    int
	Loyalty, Mult         []float64
	reqEWMA, unservedEWMA []float64
	fulfilEWMA            float64
	lam                   []float64
	obs                   []float32
	Reward                []float64
	Stats                 Stats
	Satisfied             float64

	// per-step output for the UI
	StepReq, StepDone, StepUnmet []int32
	inbound                      []float64
}

type Stats struct {
	Requests, NoSupply, DeclinedPrice, Rejected, Cancelled, Completed  float64
	WaitMin, GMV, Surcharge, CapEarn, PlatformRev, MultSum, ShockTrips float64
	Logins, IncSpend                                                   float64
	DoneM                                                              [NumModes]float64
}

func NewEnv(s *Static, cfg Config) *Env {
	e := &Env{S: s, Cfg: cfg}
	z, n := s.Z, cfg.Captains
	e.capZone, e.capDest = make([]int32, n), make([]int32, n)
	e.capState = make([]uint8, n)
	e.capFreeAt, e.capStart, e.capMaxOnline, e.capOnline = make([]int32, n), make([]int32, n), make([]int32, n), make([]int32, n)
	e.capResv, e.capTarget, e.capEarn, e.capFee = make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	e.capFlex, e.capLogged = make([]bool, n), make([]bool, n)
	for m := 0; m < NumModes; m++ {
		e.idleM[m] = make([][]int32, z)
	}
	e.capMode = make([]uint8, n)
	e.Level, e.Inc, e.prem = make([]int, z), make([]int, z), make([]float64, z)
	e.Loyalty, e.Mult = make([]float64, z), make([]float64, z)
	e.reqEWMA, e.unservedEWMA = make([]float64, z), make([]float64, z)
	e.lam = make([]float64, z)
	e.obs = make([]float32, z*NumFeatures)
	e.Reward = make([]float64, z)
	e.StepReq, e.StepDone, e.StepUnmet = make([]int32, z), make([]int32, z), make([]int32, z)
	e.inbound = make([]float64, z)
	return e
}

func (e *Env) normal() float64 { return e.rng.NormFloat64() }

func pick(r *rand.Rand, cum []float64) int {
	u := r.Float64() * cum[len(cum)-1]
	return sort.SearchFloat64s(cum, u)
}

// Reset starts a new day. Same seed => same day (demand, weather, captain population).
func (e *Env) Reset(seed uint64) []float32 {
	c, z, n := e.Cfg, e.S.Z, e.Cfg.Captains
	e.rng = rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	e.T = 0
	cumZW := make([]float64, z)
	acc := 0.0
	for i, w := range e.S.zw {
		acc += w
		cumZW[i] = acc
	}
	for i := 0; i < n; i++ {
		kind := e.rng.Float64()
		var mu float64
		switch {
		case kind < 0.40:
			mu = 6.5
		case kind < 0.75:
			mu = 9.5
		default:
			mu = 16.5
		}
		start := int((mu + e.normal() - StartHour) * 12)
		e.capStart[i] = int32(clampInt(start, 0, StepsPerDay-1))
		mo := 96
		if kind >= 0.75 {
			mo = 60
		}
		e.capMaxOnline[i] = int32(mo + e.rng.IntN(25) - 12)
		mode := modeOf(i, n, c)
		e.capMode[i] = uint8(mode)
		e.capResv[i] = clampF(c.Modes[mode].Resv+0.4*e.normal(), 0.8, 5)
		e.capTarget[i] = clampF(c.Modes[mode].Target+120*e.normal(), 250, 1300)
		e.capFlex[i] = e.rng.Float64() < 0.4
		if e.capFlex[i] {
			e.capStart[i] = 1 << 20
			e.capMaxOnline[i] = int32(36 + e.rng.IntN(25) - 12)
		}
		e.capState[i] = 0
		e.capZone[i] = int32(pick(e.rng, cumZW))
		e.capEarn[i], e.capFee[i], e.capOnline[i] = 0, 0, 0
		e.capLogged[i] = false
	}
	e.demandMult = math.Exp(0.12*e.normal()) * c.DemandScale
	e.setupDay()
	if e.day.Supply < 1 { // some captains stay home today
		for i := 0; i < n; i++ {
			if e.rng.Float64() > e.day.Supply {
				e.capStart[i], e.capFlex[i] = 1<<20, false
			}
		}
	}
	e.rainStart, e.rainEnd = -1, -1
	wantRain := c.Rain == 1 || (c.Rain < 0 && e.rng.Float64() < c.RainProb)
	if wantRain {
		e.rainStart = 20 + e.rng.IntN(StepsPerDay-80)
		e.rainEnd = e.rainStart + 24 + e.rng.IntN(16)
	}
	for i := 0; i < z; i++ {
		e.Loyalty[i], e.Mult[i], e.reqEWMA[i], e.unservedEWMA[i] = 1, 1, 0, 0
		e.Level[i], e.Inc[i], e.prem[i] = 0, 0, 0
	}
	e.fulfilEWMA = 0.8
	e.Stats = Stats{}
	e.Satisfied = 0
	e.refreshIdle()
	return e.Observe()
}

func (e *Env) Hour() float64          { return StartHour + float64(e.T)*DtMin/60 }
func (e *Env) Raining() bool          { return e.T >= e.rainStart && e.T < e.rainEnd && e.rainStart >= 0 }
func (e *Env) RainWindow() (int, int) { return e.rainStart, e.rainEnd }
func (e *Env) Done() bool             { return e.T >= StepsPerDay }

func (e *Env) refreshIdle() {
	for m := 0; m < NumModes; m++ {
		for i := range e.idleM[m] {
			e.idleM[m][i] = e.idleM[m][i][:0]
		}
	}
	for i, st := range e.capState {
		if st == 1 {
			m := e.capMode[i]
			e.idleM[m][e.capZone[i]] = append(e.idleM[m][e.capZone[i]], int32(i))
		}
	}
}

// modeOf assigns captain i to a vehicle class by fleet share.
func modeOf(i, n int, c Config) int {
	f := (float64(i) + 0.5) / float64(n)
	acc := 0.0
	for m := 0; m < NumModes; m++ {
		acc += c.Modes[m].Share
		if f < acc {
			return m
		}
	}
	return NumModes - 1
}

// IdleCount is the number of idle captains of any vehicle class in a ward.
func (e *Env) IdleCount(w int) int {
	return len(e.idleM[0][w]) + len(e.idleM[1][w]) + len(e.idleM[2][w])
}

func (e *Env) OnlineCaptains() int {
	n := 0
	for _, s := range e.capState {
		if s > 0 {
			n++
		}
	}
	return n
}

// pickup candidates: nearest ward with an untried idle captain of this mode and ETA within range.
func (e *Env) nearest(mode, z int, tried []int32, speed float64) (cap int32, eta, pickKm float64, ok bool) {
	sp := speed * e.Cfg.Modes[mode].SpeedMul
	for _, ne := range e.S.near[z] {
		l := e.idleM[mode][ne.w]
		if len(l) == 0 {
			continue
		}
		d := float64(ne.dist)
		if ne.w == int32(z) {
			d = 0.4 * math.Sqrt(e.S.area[z])
		}
		pk := d * 1.3
		et := 1.5 + 60*pk/sp
		if et > e.Cfg.PickupRange {
			return 0, 0, 0, false // list is distance-sorted: nothing closer remains
		}
		off := e.rng.IntN(len(l))
		for k := 0; k < len(l); k++ {
			ci := l[(off+k)%len(l)]
			skip := false
			for _, t := range tried {
				if t == ci {
					skip = true
					break
				}
			}
			if !skip {
				return ci, et, pk, true
			}
		}
	}
	return 0, 0, 0, false
}

func (e *Env) removeIdle(mode int, w int32, ci int32) {
	l := e.idleM[mode][w]
	for i, v := range l {
		if v == ci {
			l[i] = l[len(l)-1]
			e.idleM[mode][w] = l[:len(l)-1]
			return
		}
	}
}

func poisson(r *rand.Rand, lam float64) int {
	if lam <= 0 {
		return 0
	}
	if lam > 30 {
		v := int(math.Round(lam + math.Sqrt(lam)*r.NormFloat64()))
		if v < 0 {
			v = 0
		}
		return v
	}
	l, k, p := math.Exp(-lam), 0, 1.0
	for {
		p *= r.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

func (e *Env) rainMult() float64 {
	if e.Raining() {
		return 1.45
	}
	return 1
}

// meterFare is the regulated Bengaluru auto tariff (BBMP limits, from 1 Aug 2025).
func meterFare(km float64) float64 {
	if km <= AutoMinKm {
		return AutoMinFare
	}
	return AutoMinFare + AutoPerKm*(km-AutoMinKm)
}

// quote returns what the rider pays for a trip in this mode at a ward's current fee level, and the
// part of it that is a dynamic add-on (the "surcharge").
func (e *Env) quote(mode, lvl int, km float64) (fare, extra float64) {
	c := e.Cfg
	if mode == ModeAuto {
		m := meterFare(km)
		boost := c.AutoBoost[lvl]
		if c.AutoCap == "strict" {
			boost = math.Min(boost, 0.10*m)
		}
		return m + AutoPickupFee + boost, boost
	}
	base := TwoBaseFare + TwoPerKm*km
	m := c.TwoLevels[lvl]
	return base * m, base * (m - 1)
}

// take is the platform's share of a mode's fare.
func (e *Env) take(mode int) float64 {
	reg := e.Cfg.RegimeTwo
	if mode == ModeAuto {
		reg = e.Cfg.RegimeAuto
	}
	if reg == "commission" {
		return e.Cfg.Commission
	}
	return 0
}

func (e *Env) dailyFee(mode int) float64 {
	if mode == ModeAuto {
		if e.Cfg.RegimeAuto == "subscription" {
			return e.Cfg.FeeAuto
		}
		return 0
	}
	if e.Cfg.RegimeTwo == "subscription" {
		return e.Cfg.FeeTwo
	}
	return 0
}

// Step applies one action per ward (fee level + NumFee*incentive level) and advances five minutes.
func (e *Env) Step(actions []int) (done bool) {
	c, z := e.Cfg, e.S.Z
	t, hour := e.T, e.Hour()
	rain := e.Raining()
	for i := 0; i < z; i++ {
		e.Reward[i] = 0
		e.StepReq[i], e.StepDone[i], e.StepUnmet[i] = 0, 0, 0
		a := clampInt(actions[i], 0, NumActions-1)
		e.Level[i], e.Inc[i] = a%NumFee, a/NumFee
		e.Mult[i] = c.TwoLevels[e.Level[i]]
		e.prem[i] = (e.Mult[i] - 1) + 0.04*float64(e.Inc[i])
	}

	// ---- captains: finish trips, log in/out, reposition
	for i, st := range e.capState {
		if st == 2 && int(e.capFreeAt[i]) <= t {
			e.capZone[i] = e.capDest[i]
			e.capState[i] = 1
		}
		if e.capState[i] > 0 {
			e.capOnline[i]++
		}
	}
	// reactive logins respond to the mean of the top-20 pay premiums (fee level and incentive)
	top := topMean(e.prem, 20)
	pReact := 0.004 + 0.30*math.Max(0, top-0.05)
	for i := range e.capState {
		if e.capLogged[i] {
			continue
		}
		login := false
		if int(e.capStart[i]) <= t && e.rng.Float64() < 0.5 {
			login = !rain || e.rng.Float64() < 0.7
		} else if e.capFlex[i] && e.rng.Float64() < pReact {
			login = true
		}
		if login {
			e.capState[i] = 1
			e.capLogged[i] = true
			e.Stats.Logins++
			f := e.dailyFee(int(e.capMode[i]))
			e.capFee[i] = f
			e.Stats.PlatformRev += f
		}
	}
	for i, st := range e.capState {
		if st != 1 {
			continue
		}
		net := e.capEarn[i] - e.capFee[i]
		if e.capOnline[i] >= e.capMaxOnline[i] ||
			(net >= e.capTarget[i] && e.capOnline[i] >= 48 && e.rng.Float64() < 0.2) ||
			(rain && e.rng.Float64() < 0.015) {
			e.capState[i] = 0
			continue
		}
		if e.rng.Float64() < 0.25 { // idle repositioning: chase premiums towards a neighbour
			z0 := int(e.capZone[i])
			nb := e.S.City.Wards[z0].Nbrs
			if len(nb) == 0 {
				continue
			}
			best, bestScore := z0, 4*e.prem[z0]+0.15*e.reqEWMA[z0]+0.5*e.normal()*0.3
			for _, w := range nb {
				sc := 4*e.prem[w] + 0.15*e.reqEWMA[w] + 0.3*e.normal()*0.5
				if sc > bestScore {
					best, bestScore = w, sc
				}
			}
			if e.rng.Float64() < 0.5 {
				e.capZone[i] = int32(best)
			}
		}
	}
	e.refreshIdle()

	// ---- demand
	cOrig := [4]float64{}
	totalReq := 0
	speed := 22.0
	if isPeak(hour) {
		speed = 15
	}
	if rain {
		speed *= 0.8
	}
	rm := e.rainMult()
	reqs := make([]int32, 0, 1024)
	for w := 0; w < z; w++ {
		for k := 0; k < 4; k++ {
			cOrig[k] = e.S.shares[k][w] * e.S.prof[k][t] * compWeights[k] * e.compMult[t][k]
		}
		lam := c.DailyRequests * (cOrig[0] + cOrig[1] + cOrig[2] + cOrig[3]) * e.Loyalty[w] * e.demandMult * rm * e.day.Demand
		if e.evMult != nil && e.evMult[w] != nil {
			lam *= float64(e.evMult[w][t])
		}
		n := poisson(e.rng, lam)
		e.StepReq[w] = int32(n)
		for j := 0; j < n; j++ {
			reqs = append(reqs, int32(w))
		}
		totalReq += n
	}
	e.rng.Shuffle(len(reqs), func(i, j int) { reqs[i], reqs[j] = reqs[j], reqs[i] })

	exp := make([]float64, z)
	fail := make([]float64, z)
	shock := make([]float64, z)
	completed := 0
	var tried [3]int32
	for _, o := range reqs {
		w := int(o)
		lvl := e.Level[w]
		incAmt := c.Incentive[e.Inc[w]]
		e.Stats.Requests++
		e.Stats.MultSum += e.Mult[w]

		// trip geometry (independent of mode)
		uOD := float32(e.rng.Float64())
		dest := sort.Search(z, func(i int) bool { return e.S.odCum[w][i] >= uOD })
		if dest >= z {
			dest = z - 1
		}
		var dkm float64
		if dest == w {
			dkm = clampF(0.8*math.Sqrt(e.S.area[w])*1.3, 1.2, 6)
		} else {
			dkm = clampF(float64(e.S.City.DistKm[w][dest])*1.3*math.Exp(0.15*e.normal()), 1.2, 30)
		}

		// which vehicle classes can reach this rider, with their ETA and fare
		var avail [NumModes]bool
		var etaM, fareM, extraM [NumModes]float64
		any := false
		for m := 0; m < NumModes; m++ {
			_, et, _, ok := e.nearest(m, w, nil, speed)
			if ok {
				avail[m], etaM[m] = true, et
				fareM[m], extraM[m] = e.quote(m, lvl, dkm)
				any = true
			}
		}
		if !any {
			e.Stats.NoSupply++
			e.Reward[w] += c.WLost
			fail[w]++
			exp[w]++
			e.StepUnmet[w]++
			continue
		}
		// rider type and mode choice (multinomial logit with an outside option)
		wt := e.S.City.Wards[w].TypeIdx
		u := e.rng.Float64()
		rt := 0
		mix := typeMix[wt]
		if u >= mix[0] {
			rt = 1
			if u >= mix[0]+mix[1] {
				rt = 2
			}
		}
		p := riderTypes[rt]
		var ex [NumModes]float64
		den := 1.0
		for m := 0; m < NumModes; m++ {
			if !avail[m] {
				continue
			}
			ut := c.Modes[m].ASC + p.kappa - p.gamma*c.PriceSens*fareM[m]/50 - p.delta*etaM[m]/10
			if rain {
				if m == ModeAuto {
					ut += 0.35
				} else {
					ut -= 0.25
				}
			}
			ex[m] = math.Exp(ut)
			den += ex[m]
		}
		r := e.rng.Float64() * den
		mode := -1
		acc := 0.0
		for m := 0; m < NumModes; m++ {
			acc += ex[m]
			if r < acc {
				mode = m
				break
			}
		}
		if mode < 0 {
			e.Stats.DeclinedPrice++
			if lvl >= 3 {
				shock[w]++
			}
			exp[w]++
			continue
		}
		fare, extra, eta := fareM[mode], extraM[mode], etaM[mode]
		dur := dkm / (speed * c.Modes[mode].SpeedMul) * 60
		take := e.take(mode)
		var accepted int32 = -1
		nt := 0
		for try := 0; try < 3; try++ {
			ci, et, pk, ok2 := e.nearest(mode, w, tried[:nt], speed)
			if !ok2 {
				break
			}
			netRate := (fare*(1-take) + incAmt - c.Modes[mode].CostPerKm*(dkm+pk)) / (et + dur)
			if e.rng.Float64() < sigmoid(2*(netRate-e.capResv[ci])) {
				accepted, eta = ci, et
				break
			}
			tried[nt] = ci
			nt++
		}
		if accepted < 0 {
			e.Stats.Rejected++
			e.Reward[w] += c.WLost
			fail[w]++
			exp[w]++
			e.StepUnmet[w]++
			continue
		}
		ci := accepted
		e.removeIdle(mode, e.capZone[ci], ci)
		e.capState[ci] = 2
		rp := riderTypes[rt]
		if e.rng.Float64() < sigmoid(0.55*(eta-rp.patience)) { // rider cancels while waiting
			e.Stats.Cancelled++
			e.capFreeAt[ci] = int32(t + maxInt(1, int(math.Ceil(eta/DtMin))))
			e.capDest[ci] = e.capZone[ci]
			e.Reward[w] += c.WCancel
			fail[w]++
			exp[w]++
			e.StepUnmet[w]++
			continue
		}
		e.capFreeAt[ci] = int32(t + maxInt(1, int(math.Ceil((eta+dur)/DtMin))))
		e.capDest[ci] = int32(dest)
		cut := fare*(1-take) + incAmt
		e.capEarn[ci] += cut - c.Modes[mode].CostPerKm*dkm
		e.Stats.Completed++
		e.Stats.DoneM[mode]++
		e.Stats.WaitMin += eta
		e.Stats.GMV += fare
		e.Stats.Surcharge += extra
		e.Stats.CapEarn += cut
		e.Stats.IncSpend += incAmt
		e.Stats.PlatformRev += fare*take - incAmt
		if lvl >= 3 {
			e.Stats.ShockTrips++
			shock[w] += 0.5
		}
		completed++
		e.StepDone[w]++
		e.Reward[w] += c.WComplete + c.WEarn*cut + c.WSurcharge*extra + c.WWait*eta + c.WInc*incAmt
		exp[w]++
		if !(eta <= 6 && lvl < 3) {
			exp[w]--
		}
	}

	// ---- loyalty dynamics
	for w := 0; w < z; w++ {
		nReq := float64(e.StepReq[w])
		nf := math.Max(nReq, 1)
		score := (exp[w] - 1.5*fail[w] - shock[w]) / nf
		drift := 0.05 * clampF(score-0.3, -1.5, 0.7) * math.Min(1, nReq/4)
		e.Loyalty[w] = clampF(e.Loyalty[w]+drift+0.01*(1-e.Loyalty[w]), 0.5, 1.25)
		e.reqEWMA[w] = 0.7*e.reqEWMA[w] + 0.3*nReq
		e.unservedEWMA[w] = 0.7*e.unservedEWMA[w] + 0.3*fail[w]
	}
	if totalReq > 0 {
		e.fulfilEWMA = 0.9*e.fulfilEWMA + 0.1*float64(completed)/float64(totalReq)
	}
	e.T++
	if e.T >= StepsPerDay {
		sat := 0.0
		for i := range e.capState {
			if e.capLogged[i] && e.capEarn[i]-e.capFee[i] >= e.capResv[i]*8*60*0.45 {
				sat++
			}
		}
		e.Satisfied = sat
		for w := 0; w < z; w++ {
			e.Reward[w] += c.WRetention * sat / float64(z)
		}
		return true
	}
	return false
}

// Observe fills the per-ward feature matrix (Z x NumFeatures, row-major).
func (e *Env) Observe() []float32 {
	z := e.S.Z
	h := e.Hour()
	sinT, cosT := float32(math.Sin(2*math.Pi*h/24)), float32(math.Cos(2*math.Pi*h/24))
	rain := float32(0)
	if e.Raining() {
		rain = 1
	}
	for i := range e.inbound {
		e.inbound[i] = 0
	}
	online, idleTot := 0.0, 0.0
	for i, st := range e.capState {
		if st > 0 {
			online++
		}
		if st == 2 && int(e.capFreeAt[i]) <= e.T+2 {
			e.inbound[e.capDest[i]]++
		}
	}
	for w := 0; w < z; w++ {
		idleTot += float64(e.IdleCount(w))
	}
	gIdle := float32(idleTot / math.Max(1, online))
	for w := 0; w < z; w++ {
		o := e.obs[w*NumFeatures : (w+1)*NumFeatures]
		for k := 0; k < 5; k++ {
			o[k] = 0
		}
		o[e.S.City.Wards[w].TypeIdx] = 1
		o[5], o[6], o[7] = sinT, cosT, rain
		o[8] = float32(e.reqEWMA[w] / 5)
		o[9] = float32(e.unservedEWMA[w] / 3)
		idleLocal := float64(e.IdleCount(w))
		o[10] = float32(idleLocal / 3)
		adj := 0.0
		for _, nb := range e.S.City.Wards[w].Nbrs {
			adj += float64(e.IdleCount(nb))
		}
		o[11] = float32(adj / 8)
		o[12] = float32(e.inbound[w] / 3)
		o[13] = float32(e.Loyalty[w])
		o[14] = float32(e.Mult[w] - 1)
		o[15] = gIdle
		o[16] = float32(e.fulfilEWMA)
		o[17] = float32(online / float64(e.Cfg.Captains))
		o[18] = float32(clampF(e.reqEWMA[w]/(idleLocal+e.inbound[w]+adj*0.35+1)/2, 0, 3))
		o[19] = float32(e.T) / StepsPerDay
		o[20] = float32(len(e.idleM[ModeBike][w])) / 3
		o[21] = float32(len(e.idleM[ModeScooter][w])) / 3
		o[22] = float32(len(e.idleM[ModeAuto][w])) / 3
		o[23] = float32(e.Inc[w]) / 2
		o[24] = float32(e.Level[w]) / (NumFee - 1)
		o[25] = float32(e.prem[w])
	}
	return e.obs
}

// FeatureRatio is the index of the demand/supply pressure feature used by the rule policy.
const FeatureRatio = 18

func topMean(v []float64, k int) float64 {
	tmp := make([]float64, len(v))
	copy(tmp, v)
	sort.Float64s(tmp)
	s := 0.0
	for i := len(tmp) - k; i < len(tmp); i++ {
		s += tmp[i]
	}
	return s / float64(k)
}

func clampF(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func clampInt(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- live controls (called between steps by the session goroutine) ----

// SetRain forces a shower starting now (3 hours) or clears it.
func (e *Env) SetRain(on bool) {
	if on {
		e.rainStart, e.rainEnd = e.T, e.T+36
	} else {
		e.rainStart, e.rainEnd = -1, -1
	}
}

func (e *Env) SetDemandScale(x float64) {
	// demandMult already contains the (random) day factor; rescale relative to the old scale.
	if e.Cfg.DemandScale > 0 {
		e.demandMult *= x / e.Cfg.DemandScale
	}
	e.Cfg.DemandScale = x
}

func (e *Env) SetPriceSens(x float64) { e.Cfg.PriceSens = x }

// BusyIdle fills per-ward idle and busy captain counts (all vehicle classes).
func (e *Env) BusyIdle(idle, busy []int16) {
	for i := range idle {
		idle[i], busy[i] = int16(e.IdleCount(i)), 0
	}
	for i, s := range e.capState {
		if s == 2 {
			busy[e.capZone[i]]++
		}
	}
}

// IdleMode fills per-ward idle counts for one vehicle class.
func (e *Env) IdleMode(m int, out []int16) {
	for i := range out {
		out[i] = int16(len(e.idleM[m][i]))
	}
}
