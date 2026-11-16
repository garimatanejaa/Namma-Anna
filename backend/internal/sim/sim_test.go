package sim

import (
	"math"
	"testing"

	"riderl/internal/city"
)

func setup(t *testing.T) (*Static, Config) {
	c, err := city.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewStatic(c), DefaultConfig()
}

func runDay(st *Static, cfg Config, seed uint64, level int) Metrics {
	e := NewEnv(st, cfg)
	e.Reset(seed)
	act := make([]int, st.Z)
	for i := range act {
		act[i] = level
	}
	for !e.Done() {
		e.Step(act)
	}
	return e.Metrics()
}

func TestDeterministic(t *testing.T) {
	st, cfg := setup(t)
	a, b := runDay(st, cfg, 7, 0), runDay(st, cfg, 7, 0)
	if a != b {
		t.Fatalf("same seed gave different days:\n%+v\n%+v", a, b)
	}
	if c := runDay(st, cfg, 8, 0); c == a {
		t.Fatal("different seeds gave identical days")
	}
}

func TestAccounting(t *testing.T) {
	st, cfg := setup(t)
	m := runDay(st, cfg, 3, 0)
	if m.Requests < 20000 || m.Requests > 400000 {
		t.Fatalf("implausible request volume %v", m.Requests)
	}
	if m.Completed > m.Requests {
		t.Fatalf("completed %v > requests %v", m.Completed, m.Requests)
	}
	sum := m.Fulfilment + m.NoSupplyRate + m.PriceDeclineRate + m.RejectedRate + m.CancelRate
	if math.Abs(sum-1) > 0.02 {
		t.Fatalf("outcome shares sum to %v, want ~1", sum)
	}
}

func TestSurgeCostsDemand(t *testing.T) {
	st, cfg := setup(t)
	lo, hi := runDay(st, cfg, 5, 0), runDay(st, cfg, 5, 4)
	if hi.PriceDeclineRate <= lo.PriceDeclineRate {
		t.Fatalf("x1.8 everywhere should raise price declines: %v vs %v", hi.PriceDeclineRate, lo.PriceDeclineRate)
	}
	if hi.AvgSurgePaid <= lo.AvgSurgePaid {
		t.Fatal("surge paid should rise with surge")
	}
}

func TestFeatureLayout(t *testing.T) {
	st, cfg := setup(t)
	e := NewEnv(st, cfg)
	e.Reset(1)
	if got := len(e.Observe()); got != st.Z*NumFeatures {
		t.Fatalf("obs len %d, want %d", got, st.Z*NumFeatures)
	}
}

func TestDayTypes(t *testing.T) {
	st, cfg := setup(t)
	by := map[string]Metrics{}
	for _, d := range DayTypes {
		c := cfg
		c.DayType = d.Key
		c.Rain = 0
		by[d.Key] = runDay(st, c, 11, 0)
	}
	if by["sunday"].Requests >= by["weekday"].Requests {
		t.Fatal("sunday should have fewer requests than a weekday")
	}
	if by["bandh"].Requests >= 0.7*by["weekday"].Requests {
		t.Fatal("bandh should cut demand sharply")
	}
	if by["bandh"].CaptainsOnline >= 0.7*by["weekday"].CaptainsOnline {
		t.Fatalf("bandh should cut supply: %v vs %v", by["bandh"].CaptainsOnline, by["weekday"].CaptainsOnline)
	}
	if by["ipl"].Requests <= by["weekday"].Requests {
		t.Fatal("ipl night should add requests")
	}
}
