import React, { useMemo, useRef, useState } from 'react'

// Sparkline with area fill; values array, optional fixed domain.
export function Sparkline({ data, color = 'var(--accent)', height = 34, domain, className }) {
  const w = 120
  const { d, a } = useMemo(() => {
    if (!data || data.length < 2) return { d: '', a: '' }
    let lo = domain ? domain[0] : Math.min(...data)
    let hi = domain ? domain[1] : Math.max(...data)
    if (hi - lo < 1e-9) { hi += 1; lo -= 1 }
    const pts = data.map((v, i) => [(i / (data.length - 1)) * w, height - 3 - ((v - lo) / (hi - lo)) * (height - 6)])
    const d = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join(' ')
    return { d, a: d + ` L${w} ${height} L0 ${height} Z` }
  }, [data, height, domain])
  return (
    <svg className={className} viewBox={`0 0 ${w} ${height}`} preserveAspectRatio="none" width="100%" height={height}>
      <path d={a} fill={color} opacity="0.12" />
      <path d={d} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

// Multi-series line chart over the 192-step day with hover crosshair. One y-axis only.
export function LineChart({ series, xKey = 'hour', xDomain = [6, 22], height = 150, w = 400, yFmt = (v) => v.toFixed(2), yDomain, rainBands = [], xFmt, xTicks, tickFmt }) {
  const ref = useRef(null)
  const [hover, setHover] = useState(null)
  const W = w, H = height, L = 40, R = 10, T = 8, B = 20
  const all = series.flatMap((s) => s.data.map((p) => p.y))
  let lo = yDomain ? yDomain[0] : all.length ? Math.min(...all) : 0
  let hi = yDomain ? yDomain[1] : all.length ? Math.max(...all) : 1
  if (hi - lo < 1e-9) { hi = lo + 1 }
  if (!yDomain) { const pad = (hi - lo) * 0.08; lo -= pad; hi += pad }
  const sx = (x) => L + ((x - xDomain[0]) / (xDomain[1] - xDomain[0])) * (W - L - R)
  const sy = (y) => T + (1 - (y - lo) / (hi - lo)) * (H - T - B)
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((f) => lo + f * (hi - lo))
  const xt = xTicks || [6, 10, 14, 18, 22]
  const onMove = (e) => {
    const r = ref.current.getBoundingClientRect()
    const x = ((e.clientX - r.left) / r.width) * W
    const hx = xDomain[0] + ((x - L) / (W - L - R)) * (xDomain[1] - xDomain[0])
    setHover(hx)
  }
  const pick = (s) => {
    if (hover == null || !s.data.length) return null
    let best = s.data[0]
    for (const p of s.data) if (Math.abs(p.x - hover) < Math.abs(best.x - hover)) best = p
    return Math.abs(best.x - hover) < 0.5 ? best : null
  }
  return (
    <div className="chart">
      <svg ref={ref} viewBox={`0 0 ${W} ${H}`} width="100%" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
        {rainBands.map(([a, b], i) => (
          <rect key={i} x={sx(a)} y={T} width={Math.max(1, sx(b) - sx(a))} height={H - T - B} fill="#5b8cff" opacity="0.10" />
        ))}
        {ticks.map((t, i) => (
          <g key={i}>
            <line x1={L} x2={W - R} y1={sy(t)} y2={sy(t)} stroke="var(--grid)" strokeWidth="1" />
            <text x={L - 6} y={sy(t) + 3} textAnchor="end" className="axis">{yFmt(t)}</text>
          </g>
        ))}
        {xt.map((t) => (
          <text key={t} x={sx(t)} y={H - 5} textAnchor="middle" className="axis">{tickFmt ? tickFmt(t) : String(t).padStart(2, '0') + ':00'}</text>
        ))}
        {series.map((s) => (
          <path
            key={s.name}
            d={s.data.map((p, i) => (i ? 'L' : 'M') + sx(p.x).toFixed(1) + ' ' + sy(p.y).toFixed(1)).join(' ')}
            fill="none" stroke={s.color} strokeWidth="2" strokeLinejoin="round" strokeLinecap="round"
            strokeDasharray={s.dash || undefined} opacity={s.opacity ?? 1}
          />
        ))}
        {hover != null && hover >= xDomain[0] && hover <= xDomain[1] && (
          <g>
            <line x1={sx(hover)} x2={sx(hover)} y1={T} y2={H - B} stroke="var(--ink-3)" strokeDasharray="3 3" />
            {series.map((s) => {
              const p = pick(s)
              return p ? <circle key={s.name} cx={sx(p.x)} cy={sy(p.y)} r="4" fill={s.color} stroke="var(--surface)" strokeWidth="2" /> : null
            })}
          </g>
        )}
      </svg>
      {hover != null && (
        <div className="chart-tip" style={{ left: `${(Math.min(Math.max(sx(hover), 60), W - 90) / W) * 100}%` }}>
          <div className="tip-h">{xFmt ? xFmt(hover) : hover.toFixed(1)}</div>
          {series.map((s) => {
            const p = pick(s)
            return p ? (
              <div key={s.name} className="tip-r"><i style={{ background: s.color }} />{s.name}<b>{yFmt(p.y)}</b></div>
            ) : null
          })}
        </div>
      )}
      <div className="legend">
        {series.map((s) => (
          <span key={s.name}><i style={{ background: s.color }} />{s.name}</span>
        ))}
        {rainBands.length > 0 && <span><i style={{ background: '#5b8cff', opacity: 0.4 }} />rain</span>}
      </div>
    </div>
  )
}

// Dot-and-whisker chart: estimate with 95% CI around zero line.
export function CIChart({ rows, unit = '', width = 520 }) {
  const rowH = 30, T = 8, L = 128, R = 20, W = width
  const H = T + rows.length * rowH + 26
  const lo = Math.min(0, ...rows.map((r) => r.lo)), hi = Math.max(0, ...rows.map((r) => r.hi))
  const pad = (hi - lo) * 0.08 || 1
  const sx = (v) => L + ((v - (lo - pad)) / (hi + pad - (lo - pad))) * (W - L - R)
  const [h, setH] = useState(null)
  const ticks = 5
  const step = (hi - lo) / (ticks - 1)
  return (
    <div className="chart">
      <svg viewBox={`0 0 ${W} ${H}`} width="100%">
        {Array.from({ length: ticks }, (_, i) => lo + i * step).map((t, i) => (
          <g key={i}>
            <line x1={sx(t)} x2={sx(t)} y1={T} y2={H - 22} stroke="var(--grid)" />
            <text x={sx(t)} y={H - 8} textAnchor="middle" className="axis">{(t / 1000).toFixed(0)}k</text>
          </g>
        ))}
        <line x1={sx(0)} x2={sx(0)} y1={T} y2={H - 22} stroke="var(--ink-3)" strokeDasharray="4 3" />
        {rows.map((r, i) => {
          const y = T + i * rowH + rowH / 2
          const sig = r.lo > 0 || r.hi < 0
          return (
            <g key={r.name} onMouseEnter={() => setH(i)} onMouseLeave={() => setH(null)} opacity={h == null || h === i ? 1 : 0.45}>
              <rect x="0" y={y - rowH / 2} width={W} height={rowH} fill="transparent" />
              <text x={L - 10} y={y + 4} textAnchor="end" className={'rowlab' + (r.highlight ? ' hl' : '')}>{r.name}</text>
              {r.ref ? (
                <text x={sx(0) + 8} y={y + 4} className="axis">reference</text>
              ) : (
                <>
                  <line x1={sx(r.lo)} x2={sx(r.hi)} y1={y} y2={y} stroke={r.color} strokeWidth="2.5" strokeLinecap="round" />
                  <circle cx={sx(r.mean)} cy={y} r="5.5" fill={sig ? r.color : 'var(--surface)'} stroke={r.color} strokeWidth="2" />
                </>
              )}
              {h === i && !r.ref && (
                <text x={Math.min(sx(r.hi) + 8, W - 130)} y={y - 9} className="tipsvg">
                  {(r.mean / 1000).toFixed(1)}k  [{(r.lo / 1000).toFixed(1)}, {(r.hi / 1000).toFixed(1)}]{unit}
                </text>
              )}
            </g>
          )
        })}
      </svg>
    </div>
  )
}
