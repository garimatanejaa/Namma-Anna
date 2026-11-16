package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"riderl/internal/policy"
	"riderl/internal/sim"
)

type cmd struct {
	Action string  `json:"action"`
	Value  float64 `json:"value"`
	Name   string  `json:"name"`
	Ward   int     `json:"ward"`
	Level  int     `json:"level"`
	Seed   uint64  `json:"seed"`
	Day    string  `json:"day_type"`
}

type Session struct {
	ID      string
	env     *sim.Env
	pol     policy.Policy
	polName string
	factory func(string) (policy.Policy, bool)
	cmds    chan cmd
	ctx     context.Context
	cancel  context.CancelFunc

	mu       sync.Mutex
	subs     map[chan []byte]struct{}
	hello    []byte
	lastUsed time.Time

	playing   bool
	sps       float64
	series    []KPI
	events    []Event
	overrides map[int]int
	act       []int
	prevRain  bool
	nWards    int
}

func newSession(id string, st *sim.Static, cfg sim.Config, seed uint64, polName string, factory func(string) (policy.Policy, bool)) (*Session, error) {
	pol, ok := factory(polName)
	if !ok {
		return nil, fmt.Errorf("unknown policy %q", polName)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: id, env: sim.NewEnv(st, cfg), pol: pol, polName: polName, factory: factory,
		cmds: make(chan cmd, 64), ctx: ctx, cancel: cancel, subs: map[chan []byte]struct{}{},
		sps: 6, playing: true, overrides: map[int]int{}, act: make([]int, st.Z), nWards: st.Z, lastUsed: time.Now()}
	s.env.Reset(seed)
	if d := sim.DayTypeByKey(cfg.DayType); d.Key != "weekday" {
		s.events = append(s.events, Event{T: 0, Kind: "day", Text: d.Label + ": " + d.Desc})
	}
	s.snapshot(nil)
	go s.run()
	return s, nil
}

func (s *Session) touch() { s.mu.Lock(); s.lastUsed = time.Now(); s.mu.Unlock() }
func (s *Session) idleFor() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.subs) > 0 {
		return 0
	}
	return time.Since(s.lastUsed)
}

func (s *Session) send(c cmd) bool {
	select {
	case s.cmds <- c:
		return true
	default:
		return false
	}
}

func (s *Session) subscribe() (chan []byte, func()) {
	ch := make(chan []byte, 8)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	hello := s.hello
	s.mu.Unlock()
	if hello != nil {
		ch <- hello
	}
	return ch, func() { s.mu.Lock(); delete(s.subs, ch); s.lastUsed = time.Now(); s.mu.Unlock() }
}

func (s *Session) broadcast(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hello = b
	for ch := range s.subs {
		select {
		case ch <- b:
		default: // slow client: drop this frame
		}
	}
}

func (s *Session) run() {
	const tick = 50 * time.Millisecond
	t := time.NewTicker(tick)
	defer t.Stop()
	carry := 0.0
	for {
		select {
		case <-s.ctx.Done():
			return
		case c := <-s.cmds:
			s.apply(c)
			s.snapshot(nil)
		case <-t.C:
			if !s.playing || s.env.Done() {
				continue
			}
			carry += s.sps * tick.Seconds()
			n := int(carry)
			if n < 1 {
				continue
			}
			carry -= float64(n)
			var pts []KPI
			for i := 0; i < n && !s.env.Done(); i++ {
				pts = append(pts, s.stepOnce())
			}
			s.snapshot(pts)
		}
	}
}

func (s *Session) stepOnce() KPI {
	e := s.env
	obs := e.Observe()
	s.pol.Act(obs, e.S.Z, s.act)
	for w, lv := range s.overrides {
		if w >= 0 && w < len(s.act) {
			s.act[w] = lv + sim.NumFee*(s.act[w]/sim.NumFee) // keep the policy's incentive choice
		}
	}
	rainNow := e.Raining()
	if rainNow != s.prevRain {
		kind, txt := "rain", "Rain starts: demand up, captains log off"
		if !rainNow {
			txt = "Rain clears"
		}
		s.events = append(s.events, Event{T: e.T, Kind: kind, Text: txt})
		s.prevRain = rainNow
	}
	e.Step(s.act)
	k := s.kpi()
	if e.Done() {
		s.events = append(s.events, Event{T: e.T, Kind: "end", Text: "Day complete"})
	}
	s.series = append(s.series, k)
	return k
}

func (s *Session) kpi() KPI {
	e := s.env
	m := e.Metrics()
	idle, busy := make([]int16, s.nWards), make([]int16, s.nWards)
	e.BusyIdle(idle, busy)
	ni, nb, req, done := 0, 0, 0, 0
	mm, ml := 0.0, 0.0
	for i := range idle {
		ni += int(idle[i])
		nb += int(busy[i])
		req += int(e.StepReq[i])
		done += int(e.StepDone[i])
		mm += e.Mult[i]
		ml += e.Loyalty[i]
	}
	fs := 0.0
	if req > 0 {
		fs = float64(done) / float64(req)
	}
	return KPI{T: e.T, Hour: e.Hour(), Rain: e.Raining(), Requests: m.Requests, Completed: m.Completed,
		Fulfil: m.Fulfilment, FulfilStep: fs, ETA: m.AvgETAMin, SurgePaid: m.AvgSurgePaid, CapNetHr: m.CaptainNetPerHour,
		Online: ni + nb, Idle: ni, Busy: nb, Revenue: m.PlatformRevenue, MeanMult: mm / float64(s.nWards),
		Loyalty: ml / float64(s.nWards), Unmet: 1 - m.Fulfilment, DoneM: m.ModeDone, NetM: m.ModeNetHr, OnlineM: m.ModeOnline, Incentive: m.IncentiveSpend}
}

func (s *Session) apply(c cmd) {
	e := s.env
	switch c.Action {
	case "play":
		s.playing = true
	case "pause":
		s.playing = false
	case "speed":
		s.sps = math.Max(0.5, math.Min(80, c.Value))
	case "policy":
		if p, ok := s.factory(c.Name); ok {
			s.pol, s.polName = p, c.Name
			s.events = append(s.events, Event{T: e.T, Kind: "policy", Text: "Pricing policy → " + c.Name})
		}
	case "rain":
		e.SetRain(c.Value > 0)
		s.events = append(s.events, Event{T: e.T, Kind: "rain", Text: map[bool]string{true: "You forced a rain shower", false: "You cleared the rain"}[c.Value > 0]})
		s.prevRain = e.Raining()
	case "demand":
		e.SetDemandScale(math.Max(0.4, math.Min(2.0, c.Value)))
	case "price_sens":
		e.SetPriceSens(math.Max(0.4, math.Min(2.5, c.Value)))
	case "override":
		if c.Level < 0 {
			delete(s.overrides, c.Ward)
		} else {
			s.overrides[c.Ward] = c.Level
		}
	case "clear_overrides":
		s.overrides = map[int]int{}
	case "reset":
		s.series, s.events, s.overrides = nil, nil, map[int]int{}
		if c.Day != "" {
			e.Cfg.DayType = c.Day
		}
		e.Reset(c.Seed)
		s.prevRain = false
		s.playing = true
		if d := sim.DayTypeByKey(e.Cfg.DayType); d.Key != "weekday" {
			s.events = append(s.events, Event{T: 0, Kind: "day", Text: d.Label + ": " + d.Desc})
		}
	}
}

func (s *Session) snapshot(pts []KPI) {
	e := s.env
	z := s.nWards
	f := Frame{T: e.T, Hour: e.Hour(), Rain: e.Raining(), Done: e.Done(), Policy: s.polName, Day: e.Cfg.DayType, Playing: s.playing && !e.Done(), SPS: s.sps,
		Mult: make([]float32, z), Level: make([]int8, z), Inc: make([]int8, z), Idle: make([]int16, z), Busy: make([]int16, z), Req: make([]int16, z),
		Done_: make([]int16, z), Unmet: make([]int16, z), Loyalty: make([]float32, z), Series: pts}
	e.BusyIdle(f.Idle, f.Busy)
	for m := 0; m < 3; m++ {
		f.IdleM[m] = make([]int16, z)
		e.IdleMode(m, f.IdleM[m])
	}
	for i := 0; i < z; i++ {
		f.Mult[i] = float32(e.Mult[i])
		f.Level[i], f.Inc[i] = int8(e.Level[i]), int8(e.Inc[i])
		f.Req[i], f.Done_[i], f.Unmet[i] = int16(e.StepReq[i]), int16(e.StepDone[i]), int16(e.StepUnmet[i])
		f.Loyalty[i] = float32(e.Loyalty[i])
	}
	if len(s.overrides) > 0 {
		f.Overrides = s.overrides
	}
	if len(s.events) > 0 {
		f.Events = s.events
	}
	if e.Done() {
		m := e.Metrics()
		f.Final = &m
	}
	// a late subscriber must be able to rebuild the charts from the hello frame
	hello := f
	hello.Series = s.series
	hb, _ := json.Marshal(hello)
	s.mu.Lock()
	s.hello = hb
	s.mu.Unlock()
	fb, _ := json.Marshal(f)
	s.mu.Lock()
	for ch := range s.subs {
		select {
		case ch <- fb:
		default:
		}
	}
	s.mu.Unlock()
}
