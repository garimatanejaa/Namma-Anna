// Package sim is a ward-level ride-hailing marketplace simulator for Bengaluru, inspired by
// Rapido's captain-subscription model. One step = 5 minutes, one episode = 06:00-22:00.
package sim

import "math"

const (
	StepsPerDay = 192
	DtMin       = 5.0
	StartHour   = 6.0
	NumFeatures = 26
)

// Vehicle classes on the Rapido platform that this lab models.
const (
	ModeBike = iota
	ModeScooter
	ModeAuto
	NumModes
)

// Action = fee level (0..NumFee-1) + NumFee * incentive level (0..NumInc-1).
const (
	NumFee     = 5
	NumInc     = 3
	NumActions = NumFee * NumInc
)

// ModeCfg describes one vehicle class: fleet, economics and captain behaviour.
type ModeCfg struct {
	Name      string  `json:"name"`
	Share     float64 `json:"share"`       // share of the captain fleet
	CostPerKm float64 `json:"cost_per_km"` // fuel + wear, rupees
	SpeedMul  float64 `json:"speed_mul"`   // vs traffic speed
	Resv      float64 `json:"resv"`        // mean reservation rate, rupees per minute
	Target    float64 `json:"target"`      // mean daily earning target
	ASC       float64 `json:"asc"`         // rider preference for this mode
}

// Config holds every knob the UI and trainers can turn.
type Config struct {
	Captains      int               `json:"captains"`
	DailyRequests float64           `json:"daily_requests"` // expected ride intents per day at demand scale 1
	DemandScale   float64           `json:"demand_scale"`
	PriceSens     float64           `json:"price_sens"`  // multiplier on riders' price sensitivity
	RegimeAuto    string            `json:"regime_auto"` // "subscription" | "commission"
	RegimeTwo     string            `json:"regime_two"`  // bikes and scooters
	Commission    float64           `json:"commission"`
	FeeAuto       float64           `json:"fee_auto"` // daily access fee, rupees (Rapido: 9-29)
	FeeTwo        float64           `json:"fee_two"`
	Rain          int               `json:"rain"` // -1 random, 0 never, 1 forced (a ~3h shower)
	RainProb      float64           `json:"rain_prob"`
	TwoLevels     []float64         `json:"two_levels"` // fare multiplier band for bikes/scooters
	AutoBoost     []float64         `json:"auto_boost"` // rupee add-on levels for autos (pickup/boost fee)
	Incentive     []float64         `json:"incentive"`  // rupee bonus per trip paid to captains
	AutoCap       string            `json:"auto_cap"`   // "typical" | "strict" (boost <= 10% of meter fare)
	Modes         [NumModes]ModeCfg `json:"modes"`
	DayType       string            `json:"day_type"` // see DayTypes; "" = weekday

	// reward weights
	WComplete   float64 `json:"-"`
	WEarn       float64 `json:"-"`
	WSurcharge  float64 `json:"-"`
	WLost       float64 `json:"-"`
	WCancel     float64 `json:"-"`
	WWait       float64 `json:"-"`
	WInc        float64 `json:"-"`
	WRetention  float64 `json:"-"`
	PickupRange float64 `json:"-"` // max pickup ETA (minutes) a dispatch will consider
}

// Auto meter tariff inside BBMP limits (effective 1 Aug 2025) plus Rapido's flat pickup fee.
const (
	AutoMinFare   = 36.0
	AutoMinKm     = 2.0
	AutoPerKm     = 18.0
	AutoPickupFee = 10.0
	TwoBaseFare   = 15.0
	TwoPerKm      = 7.0
)

func DefaultConfig() Config {
	return Config{
		Captains: 7000, DailyRequests: 150000, DemandScale: 1, PriceSens: 1,
		RegimeAuto: "subscription", RegimeTwo: "commission", Commission: 0.15, FeeAuto: 19, FeeTwo: 15,
		Rain: -1, RainProb: 0.3,
		TwoLevels: []float64{1.0, 1.1, 1.2, 1.35, 1.5},
		AutoBoost: []float64{0, 5, 10, 15, 20},
		Incentive: []float64{0, 8, 16},
		AutoCap:   "typical",
		Modes: [NumModes]ModeCfg{
			{Name: "Bike", Share: 0.40, CostPerKm: 2.0, SpeedMul: 1.15, Resv: 1.1, Target: 480, ASC: 0.55},
			{Name: "Scooter", Share: 0.17, CostPerKm: 1.3, SpeedMul: 1.0, Resv: 1.0, Target: 430, ASC: 0.45},
			{Name: "Auto", Share: 0.43, CostPerKm: 4.5, SpeedMul: 0.85, Resv: 2.0, Target: 800, ASC: 0.25},
		},
		WComplete: 1, WEarn: 0.004, WSurcharge: -0.004, WLost: -0.5, WCancel: -0.5, WWait: -0.02, WInc: -0.007,
		WRetention: 0.05, PickupRange: 22,
	}
}

// Rider type preferences: kappa (baseline appetite), gamma (price per Rs50), delta (wait per 10 min), patience.
type riderParams struct{ kappa, gamma, delta, patience float64 }

var riderTypes = [3]riderParams{
	{3.3, 0.7, 0.9, 9},  // commuter
	{2.9, 1.8, 0.6, 10}, // price-sensitive
	{2.4, 1.1, 1.2, 7},  // occasional
}

// P(rider type | ward type); ward types: residential, tech, transit, retail, mixed
var typeMix = [5][3]float64{
	{0.30, 0.40, 0.30},
	{0.62, 0.14, 0.24},
	{0.35, 0.25, 0.40},
	{0.25, 0.35, 0.40},
	{0.30, 0.38, 0.32},
}

func sigmoid(x float64) float64 {
	if x >= 0 {
		return 1 / (1 + math.Exp(-x))
	}
	e := math.Exp(x)
	return e / (1 + e)
}

func isPeak(h float64) bool { return (h >= 8 && h < 10.5) || (h >= 17 && h < 20) }

func gauss(h, mu, sd float64) float64 { d := (h - mu) / sd; return math.Exp(-0.5 * d * d) }

// Time-of-day demand profiles per origin component (unnormalised).
func profResid(h float64) float64 {
	return 0.2 + 1.0*gauss(h, 8.7, 1.1) + 0.45*gauss(h, 19.5, 1.8)
}
func profEmploy(h float64) float64 {
	return 0.2 + 0.35*gauss(h, 9.8, 1.2) + 1.0*gauss(h, 18.4, 1.2) + 0.2*gauss(h, 13.2, 1.5)
}
func profTransit(h float64) float64 {
	return 0.35 + 0.8*gauss(h, 9.0, 1.3) + 0.8*gauss(h, 18.5, 1.4)
}
func profRetail(h float64) float64 {
	return 0.15 + 0.3*gauss(h, 13.0, 2.0) + 0.8*gauss(h, 19.3, 1.5)
}

// share of daily demand by origin component
var compWeights = [4]float64{0.38, 0.32, 0.15, 0.15}
