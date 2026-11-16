import React from 'react'
import { Sparkline, LineChart } from './charts.jsx'
import { fmtCompact, fmtInt, pct, fmtClock, TYPE_COLORS, MODES } from '../util.js'

function rainBandsOf(series) {
  const bands = []
  let start = null
  series.forEach((p) => {
    if (p.rain && start == null) start = p.hour
    if (!p.rain && start != null) { bands.push([start, p.hour]); start = null }
  })
  if (start != null && series.length) bands.push([start, series[series.length - 1].hour])
  return bands
}

function Tile({ label, value, sub, data, color, domain }) {
  return (
    <div className="tile">
      <div className="tile-l">{label}</div>
      <div className="tile-v">{value}</div>
      <div className="tile-s">{sub}</div>
      <Sparkline data={data} color={color} domain={domain} />
    </div>
  )
}

export function Kpis({ series }) {
  const last = series[series.length - 1]
  const col = (k) => series.map((p) => p[k])
  if (!last) return <div className="tiles skeleton" />
  return (
    <div className="tiles">
      <Tile label="Fulfilment" value={pct(last.fulfil, 1)} sub={`${fmtInt(last.completed)} of ${fmtInt(last.requests)} requests`} data={col('fulfil')} color="#4cd6a7" />
      <Tile label="Avg pickup ETA" value={last.eta.toFixed(1) + ' min'} sub="of completed rides" data={col('eta')} color="#79b4ff" />
      <Tile label="Captain earnings" value={'₹' + Math.round(last.cap_net_hr) + '/hr'} sub="net, per online hour" data={col('cap_net_hr')} color="#ffc233" />
      <Tile label="Platform net revenue" value={'₹' + fmtCompact(last.revenue)} sub={`after ₹${fmtCompact(last.incentive)} captain incentives`} data={col('revenue')} color="#c58bff" />
      <Tile label="Captains online" value={fmtInt(last.online)} sub={`${fmtInt(last.idle)} idle · ${fmtInt(last.busy)} on trip`} data={col('online')} color="#ff9d5c" />
      <Tile label="Rider loyalty" value={last.loyalty.toFixed(2)} sub="city mean (1.0 = baseline)" data={col('loyalty')} color="#ff6b8a" domain={[0.7, 1.25]} />
    </div>
  )
}

export function ModeTable({ series }) {
  const last = series[series.length - 1]
  if (!last) return null
  const tot = last.done_m.reduce((a, b) => a + b, 0) || 1
  return (
    <div className="modes">
      <div className="ch-h">By vehicle</div>
      <table>
        <thead><tr><th></th><th>Trips</th><th>Share</th><th>Captain ₹/hr</th><th>Online</th></tr></thead>
        <tbody>
          {MODES.map((m, i) => (
            <tr key={m.key}>
              <td><i style={{ background: m.color }} />{m.label}</td>
              <td>{fmtInt(last.done_m[i])}</td>
              <td>{pct(last.done_m[i] / tot, 0)}</td>
              <td>₹{Math.round(last.net_m[i])}</td>
              <td>{fmtInt(last.online_m[i])}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function Charts({ series }) {
  const bands = rainBandsOf(series)
  const f = [{ name: 'Fulfilment (step)', color: '#4cd6a7', data: series.map((p) => ({ x: p.hour, y: p.fulfil_step })) }]
  const m = [
    { name: 'Avg bike/scooter fare ×', color: '#ffc233', data: series.map((p) => ({ x: p.hour, y: p.mean_mult })) },
    { name: 'Rider loyalty', color: '#ff6b8a', data: series.map((p) => ({ x: p.hour, y: p.loyalty })), dash: '4 3' },
  ]
  return (
    <div className="charts">
      <div className="ch-h">Fulfilment through the day</div>
      <LineChart series={f} yDomain={[0, 1]} yFmt={(v) => Math.round(v * 100) + '%'} rainBands={bands} xFmt={fmtClock} height={130} />
      <div className="ch-h">Fees vs loyalty</div>
      <LineChart series={m} yFmt={(v) => v.toFixed(2)} rainBands={bands} xFmt={fmtClock} height={130} />
    </div>
  )
}

export function Controls({ frame, policies, cmd, opts, onNewDay }) {
  const [demand, setDemand] = React.useState(1)
  const [sens, setSens] = React.useState(1)
  if (!frame) return null
  const names = policies?.policies || []
  const desc = names.find((p) => p.Name === frame.Policy || p.Name === frame.policy)?.Description
  return (
    <div className="controls">
      <label className="field">
        <span>Pricing policy</span>
        <select value={frame.policy} onChange={(e) => cmd({ action: 'policy', name: e.target.value })}>
          {names.map((p) => <option key={p.Name} value={p.Name}>{p.Name}</option>)}
        </select>
        <em>{desc}</em>
      </label>
      <label className="field">
        <span>Day type</span>
        <select value={frame.day || 'weekday'} onChange={(e) => { cmd({ action: 'reset', day_type: e.target.value, seed: Math.floor(Math.random() * 90000) + 1 }); setDemand(1); setSens(1) }}>
          {(policies?.day_types || []).map((d) => <option key={d.key} value={d.key}>{d.label}</option>)}
        </select>
        <em>{(policies?.day_types || []).find((d) => d.key === (frame.day || 'weekday'))?.desc}</em>
      </label>
      <div className="row2">
        <label className="field">
          <span>Demand <b>×{demand.toFixed(2)}</b></span>
          <input type="range" min="0.5" max="1.6" step="0.05" value={demand} onChange={(e) => { setDemand(+e.target.value); cmd({ action: 'demand', value: +e.target.value }) }} />
        </label>
        <label className="field">
          <span>Price sensitivity <b>×{sens.toFixed(2)}</b></span>
          <input type="range" min="0.5" max="2" step="0.05" value={sens} onChange={(e) => { setSens(+e.target.value); cmd({ action: 'price_sens', value: +e.target.value }) }} />
        </label>
      </div>
      <div className="row2 btns">
        <button className={'chip' + (frame.rain ? ' on' : '')} onClick={() => cmd({ action: 'rain', value: frame.rain ? 0 : 1 })}>
          {frame.rain ? '🌧 Raining · clear' : '☂ Make it rain'}
        </button>
      </div>
      <div className="regimes">
        <div className="reg"><span>Autos</span>
          <div className="seg small">{[['subscription', 'Flat fee'], ['commission', 'Commission']].map(([k, l]) => <button key={k} className={(opts.current.regime_auto || 'subscription') === k ? 'on' : ''} onClick={() => onNewDay({ regime_auto: k })}>{l}</button>)}</div></div>
        <div className="reg"><span>Bikes & scooters</span>
          <div className="seg small">{[['commission', 'Commission'], ['subscription', 'Flat fee']].map(([k, l]) => <button key={k} className={(opts.current.regime_two || 'commission') === k ? 'on' : ''} onClick={() => onNewDay({ regime_two: k })}>{l}</button>)}</div></div>
        <div className="reg"><span>Auto add-on cap</span>
          <div className="seg small">{[['typical', 'Up to ₹20'], ['strict', '10% of meter']].map(([k, l]) => <button key={k} className={(opts.current.auto_cap || 'typical') === k ? 'on' : ''} onClick={() => onNewDay({ auto_cap: k })}>{l}</button>)}</div></div>
      </div>
      <div className="row2 btns">
        <button className="chip" onClick={() => { cmd({ action: 'reset', seed: Math.floor(Math.random() * 90000) + 1 }); setDemand(1); setSens(1) }}>↻ New day</button>
        {Object.keys(frame.overrides || {}).length > 0 && (
          <button className="chip warn" onClick={() => cmd({ action: 'clear_overrides' })}>Clear {Object.keys(frame.overrides).length} overrides</button>
        )}
      </div>
    </div>
  )
}

export function Transport({ frame, series, cmd }) {
  const [sps, setSps] = React.useState(6)
  if (!frame) return null
  const bands = rainBandsOf(series)
  const prog = frame.t / 192
  const speeds = [2, 6, 16, 40]
  return (
    <div className="transport">
      <button className="play" onClick={() => (frame.done ? cmd({ action: 'reset', seed: Math.floor(Math.random() * 90000) + 1 }) : cmd({ action: frame.playing ? 'pause' : 'play' }))} aria-label="play/pause">
        {frame.done ? '↻' : frame.playing ? '❚❚' : '▶'}
      </button>
      <div className="clock">{fmtClock(frame.hour)}</div>
      <div className="timeline">
        <div className="tl-rain">{bands.map(([a, b], i) => <i key={i} style={{ left: ((a - 6) / 16) * 100 + '%', width: ((b - a) / 16) * 100 + '%' }} />)}</div>
        <div className="tl-fill" style={{ width: prog * 100 + '%' }} />
        <div className="tl-ticks">{[6, 9, 12, 15, 18, 22].map((h) => <span key={h} style={{ left: ((h - 6) / 16) * 100 + '%' }}>{h}</span>)}</div>
      </div>
      <div className="seg small">
        {speeds.map((s) => <button key={s} className={sps === s ? 'on' : ''} onClick={() => { setSps(s); cmd({ action: 'speed', value: s }) }}>{s}×</button>)}
      </div>
    </div>
  )
}

export function WardCard({ ward, frame, onOverride, pinned, onClose, floating, style, cfg }) {
  if (!ward || !frame) return null
  const i = ward.id
  const ov = frame.overrides?.[i]
  const lvl = frame.level[i]
  const two = cfg?.two_levels || [1, 1.1, 1.2, 1.35, 1.5]
  const boost = cfg?.auto_boost || [0, 5, 10, 15, 20]
  const inc = cfg?.incentive || [0, 8, 16]
  const r = frame.req[i], u = frame.unmet[i]
  return (
    <div className={'ward-card' + (floating ? ' floating' : '')} style={style}>
      <div className="wc-h">
        <div>
          <div className="wc-n">{ward.name}</div>
          <div className="wc-s"><i style={{ background: TYPE_COLORS[ward.type] }} />{ward.type} · ward {ward.no} · {ward.area_km2.toFixed(1)} km²</div>
        </div>
        {pinned && <button className="x" onClick={onClose}>×</button>}
      </div>
      <div className="wc-grid">
        <div><b>×{two[lvl].toFixed(2)}</b><span>bike fare{ov != null ? ' (yours)' : ''}</span></div>
        <div><b>+₹{boost[lvl]}</b><span>auto add-on</span></div>
        <div><b>₹{inc[frame.inc[i]]}</b><span>captain bonus</span></div>
        <div><b>{r}</b><span>requests / 5 min</span></div>
        <div><b>{r ? Math.round((u / r) * 100) + '%' : '–'}</b><span>unmet</span></div>
        <div><b>{frame.idle_m[0][i]}/{frame.idle_m[1][i]}/{frame.idle_m[2][i]}</b><span>idle bike/scooter/auto</span></div>
        <div><b>{frame.busy[i]}</b><span>on trip</span></div>
        <div><b>{frame.loyalty[i].toFixed(2)}</b><span>loyalty</span></div>
      </div>
      {pinned && (
        <div className="wc-ov">
          <div className="wc-ovl">Override the fee level in this ward</div>
          <div className="seg">
            {two.map((x, k) => (
              <button key={k} className={ov === k ? 'on' : ''} onClick={() => onOverride(i, k)}>L{k}</button>
            ))}
            <button className={ov == null ? 'on' : ''} onClick={() => onOverride(i, -1)}>Auto</button>
          </div>
        </div>
      )}
    </div>
  )
}

export function EventLog({ events }) {
  const list = (events || []).slice(-6).reverse()
  if (!list.length) return <div className="events empty">Events appear here: rain, policy changes, end of day.</div>
  return (
    <div className="events">
      {list.map((e, k) => (
        <div key={k} className={'ev ' + e.kind}><span>{fmtClock(6 + (e.t * 5) / 60)}</span>{e.text}</div>
      ))}
    </div>
  )
}
