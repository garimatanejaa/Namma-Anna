import React, { useEffect, useMemo, useState } from 'react'
import useSession from './useSession.js'
import { getCity, getGeo, getPolicies } from './api.js'
import MapView from './components/MapView.jsx'
import { Kpis, Charts, Controls, Transport, WardCard, EventLog, ModeTable } from './components/LivePanel.jsx'
import Compare from './components/Compare.jsx'
import Method from './components/Method.jsx'
import { LAYERS, RAMPS, MODES } from './util.js'

function Legend({ layer }) {
  const L = LAYERS.find((l) => l.key === layer)
  const g = `linear-gradient(90deg, ${RAMPS[L.ramp].join(',')})`
  return (
    <div className="legend-map">
      <div className="lg-t">{L.label} <span>{L.hint}</span></div>
      <div className="lg-bar" style={{ background: g }} />
      <div className="lg-l"><span>{L.lo}</span><span>{L.hi}</span></div>
    </div>
  )
}

function Live({ city, geo, policies }) {
  const { frame, series, status, error, cmd, start, opts } = useSession({ policy: 'Rule (tuned)', speed: 6 })
  const [layer, setLayer] = useState('demand')
  const [extrude, setExtrude] = useState(false)
  const [landmarks, setLandmarks] = useState(true)
  const [sel, setSel] = useState(null)
  const [supplyMode, setSupplyMode] = useState(-1)
  const [hover, setHover] = useState(null)
  const hw = hover != null ? city.wards[hover.id] : null
  return (
    <div className="live">
      <div className="mapwrap">
        <MapView city={city} geo={geo} frame={frame} layer={layer} extrude={extrude} landmarks={landmarks} selected={sel} onSelect={setSel} onHover={setHover} supplyMode={supplyMode} />
        <div className="layers">
          {LAYERS.map((l) => (
            <button key={l.key} className={layer === l.key ? 'on' : ''} onClick={() => setLayer(l.key)} title={l.hint}>{l.label}</button>
          ))}
          <span className="sep" />
          <button className={extrude ? 'on' : ''} onClick={() => setExtrude(!extrude)}>3D</button>
          <button className={landmarks ? 'on' : ''} onClick={() => setLandmarks(!landmarks)}>Landmarks</button>
        </div>
        <Legend layer={layer} />
        {layer === 'supply' && (
          <div className="modepick">{[{ label: 'All', key: 'all' }, ...MODES].map((m, i) => <button key={m.key} className={supplyMode === i - 1 ? 'on' : ''} onClick={() => setSupplyMode(i - 1)}>{m.label}</button>)}</div>
        )}
        {sel != null && <div className="pinned"><WardCard cfg={policies?.config} ward={city.wards[sel]} frame={frame} pinned onClose={() => setSel(null)} onOverride={(w, k) => cmd({ action: 'override', ward: w, level: k })} /></div>}
        {hw && hover && sel == null && <WardCard cfg={policies?.config} floating ward={hw} frame={frame} style={{ left: Math.min(hover.x + 16, 9999), top: hover.y + 16 }} />}
        <Transport frame={frame} series={series} cmd={cmd} />
        {status === 'connecting' && <div className="overlay">Spinning up Bengaluru…</div>}
        {status === 'error' && <div className="overlay err">{error}<button className="primary" onClick={() => start()}>Start a new day</button></div>}
      </div>
      <aside className="side">
        <Controls frame={frame} policies={policies} cmd={cmd} opts={opts} onNewDay={(p) => start(p)} />
        <Kpis series={series} />
        <ModeTable series={series} />
        <Charts series={series} />
        <div className="ch-h">Event log</div>
        <EventLog events={frame?.events} />
        {frame?.final && (
          <div className="daydone"><b>Day complete.</b> {Math.round(frame.final.fulfilment * 1000) / 10}% of {frame.final.requests.toLocaleString('en-IN')} requests served; captains netted ₹{Math.round(frame.final.captain_net_per_hour)}/hr. Switch the policy and hit ↻ to compare.</div>
        )}
      </aside>
    </div>
  )
}

export default function App() {
  const [tab, setTab] = useState(() => (location.hash.slice(1) || 'live'))
  const [city, setCity] = useState(null)
  const [geo, setGeo] = useState(null)
  const [policies, setPolicies] = useState(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    Promise.all([getCity(), getGeo(), getPolicies()]).then(([c, g, p]) => { setCity(c); setGeo(g); setPolicies(p) }).catch((e) => setErr(String(e)))
  }, [])
  useEffect(() => { location.hash = tab; if (tab === 'live') setTimeout(() => window.dispatchEvent(new Event('resize')), 50) }, [tab])
  const tabs = [['live', 'Live city'], ['compare', 'Compare policies'], ['method', 'Method & data']]
  return (
    <div className="app">
      <header className="top">
        <div className="brand">
          <svg width="26" height="26" viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="#161a24" /><path d="M7 21 L13 11 L18 17 L25 8" fill="none" stroke="#ffc233" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" /><circle cx="25" cy="8" r="2.6" fill="#ffc233" /></svg>
          <div><b>Namma</b><span>Anna</span></div>
        </div>
        <nav>{tabs.map(([k, l]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>{l}</button>)}</nav>
        <div className="meta">Bengaluru · 243 wards · bikes, scooters & autos · simulated</div>
      </header>
      <main>
        {err && <div className="overlay err">Could not reach the server: {err}</div>}
        {!city && !err && <div className="overlay">Loading the city…</div>}
        {city && geo && <div style={{ display: tab === 'live' ? 'contents' : 'none' }}><Live city={city} geo={geo} policies={policies} /></div>}
        {city && tab === 'compare' && <Compare />}
        {city && tab === 'method' && <Method city={city} policies={policies} />}
      </main>
    </div>
  )
}
