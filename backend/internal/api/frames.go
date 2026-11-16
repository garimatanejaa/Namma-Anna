package api

import "riderl/internal/sim"

// KPI is one point of the live time series.
type KPI struct {
	T          int        `json:"t"`
	Hour       float64    `json:"hour"`
	Rain       bool       `json:"rain"`
	Requests   float64    `json:"requests"`
	Completed  float64    `json:"completed"`
	Fulfil     float64    `json:"fulfil"`      // cumulative
	FulfilStep float64    `json:"fulfil_step"` // this step
	ETA        float64    `json:"eta"`
	SurgePaid  float64    `json:"surge_paid"`
	CapNetHr   float64    `json:"cap_net_hr"`
	Online     int        `json:"online"`
	IdleM      [3][]int16 `json:"idle_m"`
	Idle       int        `json:"idle"`
	Busy       int        `json:"busy"`
	Revenue    float64    `json:"revenue"`
	MeanMult   float64    `json:"mean_mult"`
	Loyalty    float64    `json:"loyalty"`
	Unmet      float64    `json:"unmet"` // cumulative share of requests not served
	DoneM      [3]float64 `json:"done_m"`
	NetM       [3]float64 `json:"net_m"`
	OnlineM    [3]float64 `json:"online_m"`
	Incentive  float64    `json:"incentive"`
}

// Frame is what the map renders: per-ward arrays plus the KPI points since the last frame.
type Frame struct {
	T         int          `json:"t"`
	Hour      float64      `json:"hour"`
	Rain      bool         `json:"rain"`
	Done      bool         `json:"done"`
	Policy    string       `json:"policy"`
	Day       string       `json:"day"`
	Playing   bool         `json:"playing"`
	SPS       float64      `json:"sps"`
	Mult      []float32    `json:"mult"`
	Level     []int8       `json:"level"`
	Inc       []int8       `json:"inc"`
	Idle      []int16      `json:"idle"`
	IdleM     [3][]int16   `json:"idle_m"`
	Busy      []int16      `json:"busy"`
	Req       []int16      `json:"req"`
	Done_     []int16      `json:"served"`
	Unmet     []int16      `json:"unmet"`
	Loyalty   []float32    `json:"loyalty"`
	Series    []KPI        `json:"series"`
	Final     *sim.Metrics `json:"final,omitempty"`
	Events    []Event      `json:"events,omitempty"`
	Overrides map[int]int  `json:"overrides,omitempty"`
}

type Event struct {
	T    int    `json:"t"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}
