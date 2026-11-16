package sim

import "sort"

// Metrics summarises an episode (or the day so far).
type Metrics struct {
	Requests          float64           `json:"requests"`
	Completed         float64           `json:"completed"`
	Fulfilment        float64           `json:"fulfilment"`
	NoSupplyRate      float64           `json:"no_supply_rate"`
	PriceDeclineRate  float64           `json:"price_decline_rate"`
	RejectedRate      float64           `json:"rejected_rate"`
	CancelRate        float64           `json:"cancel_rate"`
	AvgETAMin         float64           `json:"avg_eta_min"`
	AvgSurgePaid      float64           `json:"avg_surge_paid"`
	AvgQuoteMult      float64           `json:"avg_quote_mult"`
	GMV               float64           `json:"gmv"`
	CaptainNetPerHour float64           `json:"captain_net_per_hour"`
	CaptainsOnline    float64           `json:"captains_online"`
	EarningsGini      float64           `json:"earnings_gini"`
	PlatformRevenue   float64           `json:"platform_revenue"`
	RevPerTrip        float64           `json:"platform_rev_per_trip"`
	MeanLoyalty       float64           `json:"mean_loyalty"`
	SatisfiedCaptains float64           `json:"satisfied_captains"`
	Return            float64           `json:"return"`
	IncentiveSpend    float64           `json:"incentive_spend"`
	ModeDone          [NumModes]float64 `json:"mode_done"`
	ModeNetHr         [NumModes]float64 `json:"mode_net_hr"`
	ModeOnline        [NumModes]float64 `json:"mode_online"`
}

func (e *Env) Metrics() Metrics {
	s := e.Stats
	req := maxF(1, s.Requests)
	done := maxF(1, s.Completed)
	var net []float64
	hours := 0.0
	var mNet, mHours, mOn [NumModes]float64
	for i := range e.capState {
		if e.capLogged[i] {
			net = append(net, e.capEarn[i]-e.capFee[i])
			h := float64(maxInt(1, int(e.capOnline[i]))) * DtMin / 60
			hours += h
			md := e.capMode[i]
			mNet[md] += e.capEarn[i] - e.capFee[i]
			mHours[md] += h
			mOn[md]++
		}
	}
	m := Metrics{
		Requests: s.Requests, Completed: s.Completed, Fulfilment: s.Completed / req,
		NoSupplyRate: s.NoSupply / req, PriceDeclineRate: s.DeclinedPrice / req,
		RejectedRate: s.Rejected / req, CancelRate: s.Cancelled / req,
		AvgETAMin: s.WaitMin / done, AvgSurgePaid: 1 + s.Surcharge/maxF(1, s.GMV-s.Surcharge),
		AvgQuoteMult: s.MultSum / req, GMV: s.GMV, CaptainsOnline: float64(len(net)),
		PlatformRevenue: s.PlatformRev, RevPerTrip: s.PlatformRev / done, SatisfiedCaptains: e.Satisfied,
	}
	m.IncentiveSpend = s.IncSpend
	for k := 0; k < NumModes; k++ {
		m.ModeDone[k] = s.DoneM[k]
		m.ModeOnline[k] = mOn[k]
		if mHours[k] > 0 {
			m.ModeNetHr[k] = mNet[k] / mHours[k]
		}
	}
	if len(net) > 0 {
		tot := 0.0
		for _, v := range net {
			tot += v
		}
		m.CaptainNetPerHour = tot / maxF(hours, 1e-9)
		sort.Float64s(net)
		n := float64(len(net))
		num := 0.0
		for i, v := range net {
			num += (2*float64(i+1) - n - 1) * v
		}
		if tot > 0 {
			m.EarningsGini = num / (n * tot)
		}
	}
	sum := 0.0
	for _, l := range e.Loyalty {
		sum += l
	}
	m.MeanLoyalty = sum / float64(len(e.Loyalty))
	return m
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
