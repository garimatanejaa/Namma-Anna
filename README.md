# Namma Anna

A Rapido-style marketplace for **bikes, scooters and autos**, simulated over **all 243 BBMP wards of Bengaluru**, with a pricing-and-incentive control problem on top and a policy lab to study it.
Go backend (simulator, training, API, SSE) + React/MapLibre frontend. Ships as one binary or a Docker image.

## What it does
- **Live city**: watch a full simulated day (06:00–22:00, 5-minute steps) on a ward-level map. Layers: fee level, captain incentive, demand, unmet, supply (filter by vehicle), loyalty; 3D mode. Click a ward and override its fee level. Switch day type (weekday, Saturday, Sunday, festival evening, IPL night, bandh), rain, regimes (flat fee vs commission, per vehicle class) and the auto add-on cap.
- **Compare policies**: no dynamic fee, flat fee, flat incentive, a demand/supply rule (default and tuned) and a neural policy on identical days with paired bootstrap 95% CIs; re-run live; per-day-type table.
- **Method & data**: state / action / reward, what is sourced vs assumed, data sources and licences.

## The control problem
Each step, for every ward the policy picks a **fee level** (bikes/scooters ×1.0–1.5; autos +₹0–20 on top of the fixed meter fare) and a **captain incentive** (₹0/8/16 per trip): 15 actions. Riders choose bike, scooter, auto or nothing by price and pickup time; captains accept by net earnings, work shifts, chase premiums and log in reactively. Loyalty erodes after failures and price shocks.

## Grounded in public facts vs assumed
| Sourced | Used as |
|---|---|
| Autos: zero-commission SaaS since Feb 2024, daily login fee ₹9–29 (Outlook Business) | autos default to a ₹19 flat daily fee |
| BBMP auto tariff from 1 Aug 2025: ₹36 / 2 km, ₹18 / km, +50% at night (The Logical Indian) | exact meter fare (the day ends at 22:00, so no night surcharge) |
| Aggregator autos add a ~₹10 pickup fee; reported cap of 10% over the rate + GST (Inc42) | ₹10 pickup fee + a dynamic add-on, optionally capped at 10% of meter |
| Bike taxis ~₹10/km pre-ban (The Week); ban 16 Jun 2025, back Aug 2025, High Court lifted ban 23 Jan 2026 (Business Standard) | ₹15 + ₹7/km, band ×1.0–1.5; permits/conditions **not** modelled |

**Assumed:** fleet mix (40% bike / 17% scooter / 43% auto), scooter-vs-bike preferences, rider types, captain reservation rates and targets, incentive amounts, 15% two-wheeler commission, demand levels and day-type multipliers. No real Rapido trip data is used and there is no affiliation. A "request" is an app intent; only about 30% convert to completed trips in the default setting.

## Honest results (40 held-out days, default config)
| Policy | Fulfilment | Return | vs no dynamic fee (95% CI) |
|---|---|---|---|
| No dynamic fee | 26.9% | 22.5k | – |
| Flat mid fee | 28.5% | 28.3k | +5.8k |
| Flat incentive | 26.8% | 21.4k | −1.0k |
| Rule (tuned) | 30.5% | 32.5k | +10.0k [+9.5k, +10.5k] |
| Neural (BC + ES) | 30.5% | 32.5k | +10.1k [+9.6k, +10.6k] |

The neural policy is **statistically tied** with the tuned rule (+73, CI [−100, +247]). The rule beats no-fee on all six day types. A flat incentive loses money. That is the finding, not a bug.

## Run
```bash
make run                      # builds UI + Go binary, serves on :8080
make dev-api / make dev-web   # hot-reload dev (Go :8080, Vite :5173)
make test                     # Go tests
make train                    # retrain policies (~10 min) and refresh benchmark
cd backend && go run ./cmd/benchdays   # re-run benchmarks for every day type
docker build -t namma-anna . && docker run -p 8080:8080 namma-anna
```
Needs Go ≥ 1.22 and Node ≥ 18. `PORT` sets the port; `/healthz` is the health check. Sessions are in-memory (cap 24, reaped after 5 idle minutes). Behind a proxy, don't buffer SSE.

## Layout
```
backend/cmd/server      HTTP server (REST + SSE, embeds the UI)
backend/cmd/train       rule tuning → behaviour cloning → evolution strategies → benchmark
backend/cmd/benchdays   benchmarks (main + every day type)
backend/internal/sim    marketplace simulator (modes, day types)
backend/internal/policy flat / rule / MLP policies
backend/internal/train  evaluation, paired CIs, BC, ES
backend/internal/city   embedded ward model
data/pipeline           build_city.py: raw data → ward model
web/                    React + Vite + MapLibre
```

## Data
| Source | Used for | Licence |
|---|---|---|
| BBMP 2022 wards, KSRSAC via DataMeet | polygons, areas, adjacency | CC BY / CC BY-SA 2.5 India |
| Schools & pre-schools, Open Bangalore | residential-demand proxy | ODbL |
| Hand-curated landmark coordinates | employment / transit / retail priors | n/a (approximate) |
