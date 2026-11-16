package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"riderl/internal/assets"
	"riderl/internal/city"
	"riderl/internal/policy"
	"riderl/internal/sim"
	"riderl/internal/train"
)

type Server struct {
	City  *city.City
	St    *sim.Static
	Cfg   sim.Config
	rule  policy.Rule
	mlp   *policy.MLP
	cands []train.Candidate

	mu       sync.Mutex
	sessions map[string]*Session
	maxSess  int

	benchMu  sync.Mutex
	benchRun bool
	benchPct int
	benchOut []byte
}

func New() (*Server, error) {
	c, err := city.Load()
	if err != nil {
		return nil, err
	}
	s := &Server{City: c, St: sim.NewStatic(c), Cfg: sim.DefaultConfig(), sessions: map[string]*Session{}, maxSess: 24}
	if b, err := assets.Read("rule.json"); err == nil {
		if r, err := policy.LoadRule(b); err == nil {
			s.rule = r
		}
	}
	if len(s.rule.Thresholds) == 0 {
		s.rule = policy.Rule{Label: "Rule (tuned)", Thresholds: []float64{0.6, 1.0, 1.5, 2.2}}
	}
	if b, err := assets.Read("mlp.json"); err == nil {
		if m, err := policy.LoadMLP(b); err == nil && len(m.Sizes) > 0 {
			s.mlp = m
		}
	}
	s.cands = train.StandardCandidates(s.rule, s.mlp)
	s.benchOut, _ = assets.Read("benchmark.json")
	go s.reaper()
	return s, nil
}

func (s *Server) factory(name string) (policy.Policy, bool) {
	for _, c := range s.cands {
		if c.Name == name {
			return c.F(), true
		}
	}
	return nil, false
}

func (s *Server) reaper() {
	for range time.Tick(30 * time.Second) {
		s.mu.Lock()
		for id, ss := range s.sessions {
			if ss.idleFor() > 5*time.Minute {
				ss.cancel()
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

func j(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) Handler(web fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	mux.HandleFunc("GET /api/city", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(s.City)
	})
	mux.HandleFunc("GET /api/wards.geojson", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(city.GeoJSON())
	})
	mux.HandleFunc("GET /api/policies", func(w http.ResponseWriter, r *http.Request) {
		type P struct {
			Name, Description string
		}
		var out []P
		for _, c := range s.cands {
			out = append(out, P{c.Name, c.Description})
		}
		j(w, 200, map[string]any{"policies": out, "rule_thresholds": s.rule.Thresholds, "levels": s.Cfg.TwoLevels,
			"config": s.Cfg, "has_neural": s.mlp != nil, "day_types": sim.DayTypes})
	})
	mux.HandleFunc("GET /api/benchmark", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if len(s.benchOut) == 0 {
			w.Write([]byte("{}"))
			return
		}
		w.Write(s.benchOut)
	})
	mux.HandleFunc("GET /api/benchmark/daytypes", func(w http.ResponseWriter, r *http.Request) {
		b, err := assets.Read("benchmark_daytypes.json")
		if err != nil {
			b = []byte("{}")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	mux.HandleFunc("GET /api/training", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.Read("training.json")
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	// ---- sessions ----
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Policy      string  `json:"policy"`
			Seed        uint64  `json:"seed"`
			RegimeAuto  string  `json:"regime_auto"`
			RegimeTwo   string  `json:"regime_two"`
			AutoCap     string  `json:"auto_cap"`
			Rain        bool    `json:"rain"`
			DemandScale float64 `json:"demand_scale"`
			Speed       float64 `json:"speed"`
			DayType     string  `json:"day_type"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req)
		if req.Policy == "" {
			req.Policy = "Rule (tuned)"
		}
		if req.Seed == 0 {
			req.Seed = uint64(time.Now().UnixNano()) % 100000
		}
		cfg := s.Cfg
		if req.RegimeAuto != "" {
			cfg.RegimeAuto = req.RegimeAuto
		}
		if req.RegimeTwo != "" {
			cfg.RegimeTwo = req.RegimeTwo
		}
		if req.AutoCap != "" {
			cfg.AutoCap = req.AutoCap
		}
		if req.DayType != "" {
			cfg.DayType = req.DayType
		}
		if req.DemandScale > 0 {
			cfg.DemandScale = req.DemandScale
		}
		s.mu.Lock()
		if len(s.sessions) >= s.maxSess {
			s.mu.Unlock()
			j(w, 503, map[string]string{"error": "server is busy, try again in a minute"})
			return
		}
		b := make([]byte, 8)
		rand.Read(b)
		id := hex.EncodeToString(b)
		ss, err := newSession(id, s.St, cfg, req.Seed, req.Policy, s.factory)
		if err != nil {
			s.mu.Unlock()
			j(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if req.Speed > 0 {
			ss.sps = req.Speed
		}
		s.sessions[id] = ss
		s.mu.Unlock()
		if req.Rain {
			ss.send(cmd{Action: "rain", Value: 1})
		}
		j(w, 201, map[string]any{"id": id, "seed": req.Seed})
	})
	get := func(w http.ResponseWriter, r *http.Request) *Session {
		s.mu.Lock()
		ss := s.sessions[r.PathValue("id")]
		s.mu.Unlock()
		if ss == nil {
			j(w, 404, map[string]string{"error": "session expired"})
			return nil
		}
		ss.touch()
		return ss
	}
	mux.HandleFunc("POST /api/sessions/{id}/cmd", func(w http.ResponseWriter, r *http.Request) {
		ss := get(w, r)
		if ss == nil {
			return
		}
		var c cmd
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&c); err != nil {
			j(w, 400, map[string]string{"error": "bad json"})
			return
		}
		if !ss.send(c) {
			j(w, 429, map[string]string{"error": "too many commands"})
			return
		}
		j(w, 202, map[string]bool{"ok": true})
	})
	mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if ss := s.sessions[r.PathValue("id")]; ss != nil {
			ss.cancel()
			delete(s.sessions, r.PathValue("id"))
		}
		s.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /api/sessions/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
		ss := get(w, r)
		if ss == nil {
			return
		}
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		ch, off := ss.subscribe()
		defer off()
		ka := time.NewTicker(15 * time.Second)
		defer ka.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ss.ctx.Done():
				return
			case b := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", b)
				fl.Flush()
			case <-ka.C:
				fmt.Fprint(w, ": ka\n\n")
				fl.Flush()
			}
		}
	})

	// ---- live benchmark (re-runs the policy line-up on fresh seeds) ----
	mux.HandleFunc("POST /api/benchmark/run", func(w http.ResponseWriter, r *http.Request) {
		s.benchMu.Lock()
		if s.benchRun {
			s.benchMu.Unlock()
			j(w, 409, map[string]string{"error": "a benchmark is already running"})
			return
		}
		s.benchRun, s.benchPct = true, 0
		s.benchMu.Unlock()
		var req struct {
			Seeds int    `json:"seeds"`
			Rain  bool   `json:"rain"`
			Base  uint64 `json:"base"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req)
		if req.Seeds < 4 {
			req.Seeds = 12
		}
		if req.Seeds > 40 {
			req.Seeds = 40
		}
		if req.Base == 0 {
			req.Base = uint64(time.Now().UnixNano()) % 1000000
		}
		cfg := s.Cfg
		if req.Rain {
			cfg.RainProb = 1
		}
		go func() {
			seeds := train.Seeds(req.Base, req.Seeds)
			b := train.RunBenchmark(s.St, cfg, s.cands, seeds, "No dynamic fee", "Rule (tuned)", func(d, t int, n string) {
				s.benchMu.Lock()
				s.benchPct = 100 * d / t
				s.benchMu.Unlock()
			})
			out, _ := json.Marshal(b)
			s.benchMu.Lock()
			s.benchOut, s.benchRun, s.benchPct = out, false, 100
			s.benchMu.Unlock()
		}()
		j(w, 202, map[string]bool{"started": true})
	})
	mux.HandleFunc("GET /api/benchmark/status", func(w http.ResponseWriter, r *http.Request) {
		s.benchMu.Lock()
		defer s.benchMu.Unlock()
		j(w, 200, map[string]any{"running": s.benchRun, "pct": s.benchPct})
	})

	// ---- SPA ----
	if web != nil {
		fsrv := http.FileServer(http.FS(web))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			p := strings.TrimPrefix(r.URL.Path, "/")
			if p == "" {
				p = "index.html"
			}
			if f, err := web.Open(p); err == nil {
				f.Close()
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fsrv.ServeHTTP(w, r)
				return
			}
			r.URL.Path = "/"
			fsrv.ServeHTTP(w, r)
		})
	}
	return logging(cors(mux))
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		h.ServeHTTP(w, r)
		if !strings.HasSuffix(r.URL.Path, "/stream") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(t).Round(time.Millisecond))
		}
	})
}
