package sim

import "math"

// DayType describes how a kind of day reshapes the baseline weekday demand and captain supply.
// These multipliers are modelling assumptions (no real trip data), documented in the UI.
type DayType struct {
	Key    string                     `json:"key"`
	Label  string                     `json:"label"`
	Desc   string                     `json:"desc"`
	Demand float64                    `json:"demand"` // overall demand multiplier
	Supply float64                    `json:"supply"` // share of captains who work today
	comp   func(h float64) [4]float64 // residential, employment, transit, retail multipliers by hour
	event  *eventSpec
}

type eventSpec struct {
	lat, lng, sigmaKm float64
	peaks             [][3]float64 // hour, width, amplitude
}

func g(h, mu, s float64) float64 { return math.Exp(-(h - mu) * (h - mu) / (2 * s * s)) }

var DayTypes = []DayType{
	{Key: "weekday", Label: "Weekday", Desc: "Baseline: sharp morning and evening commute peaks.", Demand: 1, Supply: 1,
		comp: func(h float64) [4]float64 { return [4]float64{1, 1, 1, 1} }},
	{Key: "saturday", Label: "Saturday", Desc: "Half-working day: weaker office peaks, strong shopping and leisure from late morning.", Demand: 0.95, Supply: 0.95,
		comp: func(h float64) [4]float64 {
			return [4]float64{1.05, 0.55, 0.9, 1.25 + 0.5*g(h, 18.5, 2.2)}
		}},
	{Key: "sunday", Label: "Sunday", Desc: "No commute: flat residential trips, afternoon and evening leisure, fewer captains.", Demand: 0.85, Supply: 0.85,
		comp: func(h float64) [4]float64 {
			return [4]float64{0.95, 0.22, 0.75, 1.4 + 0.7*g(h, 18, 2.5)}
		}},
	{Key: "festival", Label: "Festival evening", Desc: "A major festival (Diwali or Ganesh Chaturthi eve): offices thin out, shopping and family trips spike from 16:00, many captains take the evening off.", Demand: 1.05, Supply: 0.8,
		comp: func(h float64) [4]float64 {
			return [4]float64{1.1 + 0.5*g(h, 18.5, 1.8), 0.5, 1.05, 1.1 + 1.6*g(h, 18.5, 1.9)}
		}},
	{Key: "ipl", Label: "IPL match night", Desc: "A match at Chinnaswamy Stadium: a demand surge around MG Road and Shivajinagar before the toss and a larger one when the crowd leaves.", Demand: 1, Supply: 1,
		comp:  func(h float64) [4]float64 { return [4]float64{1, 1, 1, 1} },
		event: &eventSpec{lat: 12.9788, lng: 77.5996, sigmaKm: 2.2, peaks: [][3]float64{{17.6, 0.9, 1.6}, {21.5, 0.6, 3.2}}}},
	{Key: "bandh", Label: "Bandh / strike", Desc: "A city-wide strike: far fewer captains on the road and fewer riders willing to travel.", Demand: 0.55, Supply: 0.5,
		comp: func(h float64) [4]float64 { return [4]float64{1, 0.6, 1, 0.7} }},
}

// DayTypeByKey falls back to the weekday baseline.
func DayTypeByKey(k string) *DayType {
	for i := range DayTypes {
		if DayTypes[i].Key == k {
			return &DayTypes[i]
		}
	}
	return &DayTypes[0]
}

// setupDay precomputes the per-step component multipliers and the per-ward event boost.
func (e *Env) setupDay() {
	dt := DayTypeByKey(e.Cfg.DayType)
	e.day = dt
	for t := 0; t < StepsPerDay; t++ {
		e.compMult[t] = dt.comp(StartHour + float64(t)*DtMin/60)
	}
	e.evMult = nil
	if dt.event == nil {
		return
	}
	z := len(e.S.City.Wards)
	e.evMult = make([][]float32, z)
	for w := 0; w < z; w++ {
		wd := e.S.City.Wards[w]
		dx := (wd.Lng - dt.event.lng) * 111.32 * math.Cos(dt.event.lat*math.Pi/180)
		dy := (wd.Lat - dt.event.lat) * 110.57
		prox := math.Exp(-(dx*dx + dy*dy) / (2 * dt.event.sigmaKm * dt.event.sigmaKm))
		if prox < 0.02 {
			continue
		}
		row := make([]float32, StepsPerDay)
		for t := range row {
			h := StartHour + float64(t)*DtMin/60
			b := 0.0
			for _, p := range dt.event.peaks {
				b += p[2] * g(h, p[0], p[1])
			}
			row[t] = float32(1 + prox*b)
		}
		e.evMult[w] = row
	}
}
