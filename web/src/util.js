export const clamp = (x, a, b) => Math.max(a, Math.min(b, x))
export const fmtClock = (h) => {
  const hh = Math.floor(h), mm = Math.floor((h - hh) * 60)
  return String(hh).padStart(2, '0') + ':' + String(mm).padStart(2, '0')
}
export const fmtInt = (x) => Math.round(x).toLocaleString('en-IN')
export const fmtCompact = (x) => {
  const a = Math.abs(x)
  if (a >= 1e7) return (x / 1e7).toFixed(2) + ' Cr'
  if (a >= 1e5) return (x / 1e5).toFixed(2) + ' L'
  if (a >= 1e3) return (x / 1e3).toFixed(1) + 'k'
  return String(Math.round(x))
}
export const pct = (x, d = 1) => (x * 100).toFixed(d) + '%'
export const mean = (a) => (a.length ? a.reduce((s, x) => s + x, 0) / a.length : 0)

// Colour ramps: [stop, hex]. Interpolated in JS for the legend and in MapLibre for the map.
export const RAMPS = {
  surge: ['#1b2338', '#38356a', '#7b2d6b', '#d9453f', '#ffa927', '#fff0b0'],
  demand: ['#17202f', '#15486a', '#1a7f88', '#4cc9a2', '#d4ffb5'],
  unmet: ['#1b2338', '#4c2c42', '#9b2f4a', '#ff5a5f', '#ffd3c4'],
  supply: ['#17202f', '#1c3d75', '#336fd0', '#79b4ff', '#e3f1ff'],
  loyalty: ['#c23a52', '#7a3345', '#2a3043', '#2f7a63', '#43d697'],
}
export const rampStops = (name) => {
  const r = RAMPS[name]
  const out = []
  r.forEach((c, i) => out.push(i / (r.length - 1), c))
  return out
}
export const LAYERS = [
  { key: 'surge', label: 'Fee level', hint: 'Dynamic fee the policy sets: bikes ×1.0–1.5, autos +₹0–20', ramp: 'surge', lo: 'none', hi: 'max' },
  { key: 'incentive', label: 'Incentive', hint: 'Bonus per trip the policy pays captains (₹0 / 8 / 16)', ramp: 'demand', lo: '₹0', hi: '₹16' },
  { key: 'demand', label: 'Demand', hint: 'Ride requests per km² this step', ramp: 'demand', lo: 'quiet', hi: 'peak' },
  { key: 'unmet', label: 'Unmet', hint: 'Share of requests not served', ramp: 'unmet', lo: '0%', hi: '100%' },
  { key: 'supply', label: 'Supply', hint: 'Idle captains per km² (filter by vehicle)', ramp: 'supply', lo: 'none', hi: 'dense' },
  { key: 'loyalty', label: 'Loyalty', hint: 'Rider retention state (long-run effect of price shocks)', ramp: 'loyalty', lo: 'eroded', hi: 'loyal' },
]
export const TYPE_COLORS = {
  Residential: '#6ea8ff', 'Tech & offices': '#c58bff', 'Transit hub': '#4cd6a7', 'Retail & leisure': '#ff9d5c', Mixed: '#7b8499',
}
export const MODES = [
  { key: 'bike', label: 'Bike', color: '#ffc233' },
  { key: 'scooter', label: 'Scooter', color: '#4cd6a7' },
  { key: 'auto', label: 'Auto', color: '#79b4ff' },
]
