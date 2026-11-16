import React from 'react'
import { TYPE_COLORS } from '../util.js'

const Fact = ({ t, url, children }) => (
  <div className="fact"><span className={'tag ' + t}>{t === 'src' ? 'SOURCED' : 'ASSUMED'}</span><div>{children}{url && <> <a href={url} target="_blank" rel="noreferrer">source</a></>}</div></div>
)

const Step = ({ n, t, children }) => (
  <div className="step"><div className="step-n">{n}</div><div><h4>{t}</h4><p>{children}</p></div></div>
)

export default function Method({ city, policies }) {
  const m = city.meta
  const counts = {}
  city.wards.forEach((w) => (counts[w.type] = (counts[w.type] || 0) + 1))
  const cfg = policies?.config
  return (
    <div className="page narrow">
      <div className="page-h"><div>
        <h2>How Namma Anna works</h2>
        <p>A marketplace simulator for Rapido-style bikes, scooters and autos across all {m.wards} BBMP wards of Bengaluru, a pricing-and-incentive control problem on top of it, and three families of policies compared on identical days.</p>
      </div></div>

      <section className="card">
        <h3>What is grounded in public facts, and what is assumed</h3>
        <div className="facts">
          <Fact t="src" url="https://www.outlookbusiness.com/corporate/rapido-aims-to-attract-auto-drivers-with-zero-commission">Rapido moved autos to a zero-commission SaaS model in Feb 2024: a flat daily login fee, ₹9–29 depending on city demand, and captains keep the fare. <b>Used:</b> autos default to a flat daily fee (₹19).</Fact>
          <Fact t="src" url="https://thelogicalindian.com/bengaluru-auto-fares-hiked-after-4-years-see-what-commuters-will-pay-from-august-1">Bengaluru's regulated auto tariff since 1 Aug 2025 is ₹36 for the first 2 km and ₹18 per km after, with a 50% night surcharge. <b>Used:</b> the meter fare is exact; the simulated day ends at 22:00 so the night surcharge never applies.</Fact>
          <Fact t="src" url="https://inc42.com/buzz/rapido-rolls-out-guaranteed-auto-rides-in-bengaluru-at-steep-prices/">Aggregator autos add a pickup fee (reported as ₹10) on top of the meter fare, and reports cite a cap of 10% over the rate plus GST. <b>Used:</b> a ₹10 pickup fee, plus a dynamic add-on of ₹0–20 that you can restrict to 10% of the meter fare.</Fact>
          <Fact t="src" url="https://www.theweek.in/news/biz-tech/2025/04/03/rapido-other-bike-taxis-to-go-off-bengaluru-roads-in-six-weeks.html">Bike taxis were popular at roughly ₹10 per km. <b>Used:</b> a base fare of ₹15 plus ₹7 per km, a fare band of ×1.0 to ×1.5.</Fact>
          <Fact t="src" url="https://www.business-standard.com/industry/news/karnataka-hc-lifts-bike-taxi-ban-uber-rapido-ola-126012301206_1.html">Bike taxis were paused in Karnataka from June 2025 and came back in August; on 23 January 2026 the High Court lifted the ban, with the state able to set conditions. <b>Not modelled:</b> permits or caps; I have not confirmed the current licensing rules.</Fact>
          <Fact t="asm">Fleet mix (40% bike, 17% scooter, 43% auto), scooter vs bike preferences, rider types, captain reservation rates and earning targets, the captain incentive amounts (₹0, 8, 16 per trip), a 15% commission for two-wheelers, and all demand levels. <b>These are assumptions</b>, not Rapido data, and the day-type multipliers likewise.</Fact>
        </div>
      </section>

      <section className="card">
        <h3>The control problem</h3>
        <div className="steps">
          <Step n="1" t="State">Each 5-minute step, every ward is described by 26 features: time of day, rain, recent demand and unserved demand, idle captains by vehicle class locally and in neighbouring wards, inbound captains, current fee and incentive, rider loyalty, city-wide fulfilment, and a demand/supply ratio.</Step>
          <Step n="2" t="Action">Two levers per ward, chosen together (15 combinations). A <b>fee level</b> (five steps) that maps to a fare multiplier for bikes and scooters (×1.0–1.5) and an add-on for autos (₹0–20) on top of the fixed meter fare. A <b>captain incentive</b> (₹0, 8 or 16 per trip). One shared policy acts on all {m.wards} wards.</Step>
          <Step n="3" t="Dynamics">A rider picks the vehicle class that suits them, or none, using a logit over price and pickup time (three latent rider types: commuter, price-sensitive, occasional). Captains in each class accept offers by net earnings against a reservation rate, work shifts, chase premiums and log in reactively. Failures and price shocks erode a per-ward loyalty state, so aggressive pricing has a delayed cost.</Step>
          <Step n="4" t="Reward">Completed trips, captain earnings and waiting time, minus lost and cancelled rides, rider add-ons and the platform's incentive spend, plus a captain-retention bonus at the end of the day. Rewards are per ward, which makes credit assignment tractable.</Step>
          <Step n="5" t="Regimes">Autos default to the flat daily fee, bikes and scooters to commission. Each can be switched from the Live panel.</Step>
        </div>
      </section>

      <section className="card">
        <h3>Day types</h3>
        <div className="steps">
          {(policies?.day_types || []).map((d) => (
            <Step key={d.key} n={d.label.slice(0, 1)} t={d.label}>{d.Desc || d.desc} Demand ×{d.demand}, captains working ×{d.supply}.</Step>
          ))}
        </div>
        <p className="note">These shapes are assumptions chosen to be plausible, not fitted to trip data. The policies are tuned on weekdays only, so the other days test how well they generalise.</p>
      </section>

      <section className="card">
        <h3>The policies</h3>
        <div className="steps">
          <Step n="A" t="Flat baselines">No dynamic fee, a flat mid-level fee everywhere, or a flat captain incentive everywhere.</Step>
          <Step n="B" t="Rule on demand/supply">Fee and incentive rise in steps as the local demand-to-supply ratio grows. The default thresholds are textbook values; the tuned version searches all six thresholds by random search on separate days.</Step>
          <Step n="C" t="Neural policy">A small MLP (26 → 32 → 32 → 15) shared across wards, behaviour-cloned from the tuned rule and then fine-tuned with antithetic evolution strategies on common random numbers. Everything, including the backprop, is hand-written Go with no ML framework. Evaluation is paired by seed with bootstrap intervals.</Step>
        </div>
      </section>

      <div className="grid2">
        <section className="card">
          <h3>Ward model</h3>
          <div className="mix">
            {Object.entries(counts).map(([k, v]) => (
              <div key={k}><i style={{ background: TYPE_COLORS[k] }} />{k}<b>{v}</b></div>
            ))}
          </div>
          <p className="note">Each ward blends four demand components, residential, employment, transit and retail, with their own time-of-day profiles. Trips follow a gravity model: destination attractiveness decays with distance (scale about 5.5 km). Labels are the dominant component in percentile terms.</p>
        </section>
        <section className="card">
          <h3>Scale of one simulated day</h3>
          <div className="mix">
            <div>Captains<b>{cfg?.captains?.toLocaleString('en-IN') ?? '—'}</b></div>
            <div>Ride requests (app intents)<b>{cfg?.daily_requests?.toLocaleString('en-IN') ?? '—'}</b></div>
            <div>Steps<b>192 × 5 min</b></div>
            <div>Window<b>06:00 – 22:00</b></div>
            <div>Auto fare<b>₹36 + ₹18/km after 2 km + ₹10</b></div>
            <div>Bike fare<b>₹15 + ₹7/km</b></div>
          </div>
          <p className="note">Counts are sized so that a no-surge day is supply-constrained, which is where pricing decisions matter.</p>
        </section>
      </div>

      <section className="card">
        <h3>Data sources</h3>
        <div className="tbl-wrap"><table className="src">
          <thead><tr><th>Source</th><th>Used for</th><th>Licence</th></tr></thead>
          <tbody>{m.sources.map((s) => (
            <tr key={s.name}><td>{s.url ? <a href={s.url} target="_blank" rel="noreferrer">{s.name}</a> : s.name}<small>{s.by}</small></td><td>{s.used_for}</td><td>{s.licence}</td></tr>
          ))}</tbody>
        </table></div>
      </section>

      <section className="card caveats">
        <h3>What this is not</h3>
        <ul>{m.caveats.map((c, i) => <li key={i}>{c}</li>)}
          <li>Rider and captain behaviour parameters are plausible, not fitted to real data. The goal is to compare pricing policies in a controlled, reproducible world, not to predict Rapido's real numbers.</li>
          <li>Rapido is named only because its public business model is the reference; this project has no affiliation with it.</li>
        </ul>
      </section>
    </div>
  )
}
