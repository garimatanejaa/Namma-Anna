import React, { useEffect, useState } from 'react'
import { CIChart, LineChart } from './charts.jsx'
import { getBenchmark, getTraining, runBenchmark, benchStatus, getDayBench, getPolicies } from '../api.js'
import { mean, pct, fmtCompact } from '../util.js'

const COLORS = ['#7b8499', '#8a93a8', '#a0a8ba', '#79b4ff', '#4cd6a7', '#ffc233']

export default function Compare() {
  const [bench, setBench] = useState(null)
  const [train, setTrain] = useState([])
  const [running, setRunning] = useState(false)
  const [progress, setProgress] = useState(0)
  const [seeds, setSeeds] = useState(12)
  const [rain, setRain] = useState(false)
  const [err, setErr] = useState('')
  const [dayb, setDayb] = useState(null)
  const [dts, setDts] = useState([])

  useEffect(() => { getBenchmark().then(setBench); getTraining().then(setTrain).catch(() => {}); getDayBench().then(setDayb).catch(() => {}); getPolicies().then((p) => setDts(p.day_types || [])) }, [])

  const run = async () => {
    setErr(''); setRunning(true); setProgress(0)
    try {
      await runBenchmark({ seeds, rain })
      const t = setInterval(async () => {
        const s = await benchStatus()
        setProgress(s.pct)
        if (!s.running) { clearInterval(t); setBench(await getBenchmark()); setRunning(false) }
      }, 700)
    } catch (e) { setErr(e.message); setRunning(false) }
  }

  const entries = bench?.entries || []
  const rows = entries.filter((e) => e.name !== bench.baseline).map((e, i) => ({
    name: e.name, mean: e.vs_flat[0], lo: e.vs_flat[1], hi: e.vs_flat[2], color: e.name === 'Neural policy' ? '#ffc233' : e.name.startsWith('Rule') ? '#4cd6a7' : '#79b4ff',
    highlight: e.name === 'Neural policy',
  }))
  if (bench?.baseline) rows.unshift({ name: bench.baseline + ' (ref)', mean: 0, lo: 0, hi: 0, ref: true, color: '#7b8499' })
  const neural = entries.find((e) => e.name === 'Neural policy')
  const verdict = neural
    ? neural.vs_rule[1] > 0 ? 'The neural policy beats the tuned rule: its paired 95% interval is above zero.'
    : neural.vs_rule[2] < 0 ? 'The neural policy is worse than the tuned rule: its paired 95% interval is below zero.'
    : 'The neural policy is statistically tied with the tuned rule: the paired 95% interval around the difference contains zero.'
    : ''
  const curve = [
    { name: 'ES iterate', color: '#ffc233', data: train.map((t, i) => ({ x: i, y: t.center })) },
    { name: 'Population mean', color: '#79b4ff', data: train.map((t, i) => ({ x: i, y: t.pop_mean })), dash: '4 3' },
  ]

  return (
    <div className="page">
      <div className="page-h">
        <div>
          <h2>Does learning beat a tuned rule?</h2>
          <p>Every policy plays the same {bench?.seeds || '…'} simulated days (paired seeds). Bars show the improvement over charging no dynamic fee, with 95% bootstrap intervals.</p>
        </div>
        <div className="runbox">
          <label>Days <select value={seeds} onChange={(e) => setSeeds(+e.target.value)} disabled={running}>{[8, 12, 20, 30].map((n) => <option key={n}>{n}</option>)}</select></label>
          <label className="check"><input type="checkbox" checked={rain} onChange={(e) => setRain(e.target.checked)} disabled={running} /> rainy days</label>
          <button className="primary" onClick={run} disabled={running}>{running ? `Running… ${progress}%` : 'Re-run on fresh days'}</button>
        </div>
      </div>
      {err && <div className="err">{err}</div>}
      {verdict && <div className="verdict"><b>Honest read.</b> {verdict}</div>}
      <div className="grid2">
        <section className="card">
          <h3>Return vs. no-dynamic-fee baseline <small>(paired difference, mean ± 95% CI)</small></h3>
          {rows.length ? <CIChart rows={rows} /> : <div className="skeleton tall" />}
        </section>
        <section className="card">
          <h3>Evolution-strategies training <small>(return per simulated day)</small></h3>
          {train.length ? <LineChart series={curve} xDomain={[0, Math.max(1, train.length - 1)]} xTicks={[0, 10, 20, 30, train.length - 1].filter((v) => v < train.length)} tickFmt={(v) => String(v)} xFmt={(x) => 'iteration ' + Math.round(x)} yFmt={(v) => (v / 1000).toFixed(0) + 'k'} height={190} w={560} /> : <div className="skeleton tall" />}
          <p className="note">Each iteration scores a perturbation on a fresh set of days, so this curve is noisy and mostly flat: the fine-tuning barely moves the cloned rule. The paired comparison on the left is the real evidence.</p>
        </section>
      </div>
      <section className="card">
        <h3>Outcomes by policy <small>(means over the evaluation days)</small></h3>
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Policy</th><th>Fulfilment</th><th>Pickup ETA</th><th>Extra paid</th><th>Captain ₹/hr</th><th>Earnings Gini</th><th>Revenue</th><th>Mean return</th></tr></thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={e.name} className={e.name === 'Neural policy' ? 'hl' : ''}>
                  <td><i style={{ background: COLORS[i % COLORS.length] }} />{e.name}<small>{e.description}</small></td>
                  <td>{pct(e.mean.fulfilment)}</td>
                  <td>{e.mean.avg_eta_min.toFixed(1)} min</td>
                  <td>×{e.mean.avg_surge_paid.toFixed(2)}</td>
                  <td>₹{Math.round(e.mean.captain_net_per_hour)}</td>
                  <td>{e.mean.earnings_gini.toFixed(3)}</td>
                  <td>₹{fmtCompact(e.mean.platform_revenue)}</td>
                  <td>{fmtCompact(mean(e.returns))}<small>±{fmtCompact(e.return_sd)} sd</small></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {dayb?.order && (
        <section className="card">
          <h3>Does it hold on weekends and festivals? <small>(change in return per day, paired 95% CI, {dayb.seeds} days each)</small></h3>
          <div className="tbl-wrap">
            <table>
              <thead><tr><th>Day type</th><th>Fulfilment, no fee</th><th>Fulfilment, tuned rule</th><th>Tuned rule vs no fee</th><th>Neural vs tuned rule</th></tr></thead>
              <tbody>
                {dayb.order.map((k) => {
                  const b = dayb.days[k]
                  const label = dts.find((d) => d.key === k)?.label || k
                  const flat = b.entries.find((e) => e.name === 'No dynamic fee')
                  const rule = b.entries.find((e) => e.name === 'Rule (tuned)')
                  const nn = b.entries.find((e) => e.name === 'Neural policy')
                  const verdict = !nn ? '' : nn.vs_rule[1] > 0 ? 'better' : nn.vs_rule[2] < 0 ? 'worse' : 'tied'
                  return (
                    <tr key={k}>
                      <td>{label}<small>{dts.find((d) => d.key === k)?.desc}</small></td>
                      <td>{pct(flat.mean.fulfilment)}</td>
                      <td>{pct(rule.mean.fulfilment)}</td>
                      <td>+{fmtCompact(rule.vs_flat[0])}<small>[{fmtCompact(rule.vs_flat[1])}, {fmtCompact(rule.vs_flat[2])}]</small></td>
                      <td><span className={'pill ' + verdict}>{verdict}</span><small>{nn.vs_rule[0] >= 0 ? '+' : ''}{Math.round(nn.vs_rule[0])} [{Math.round(nn.vs_rule[1])}, {Math.round(nn.vs_rule[2])}]</small></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <p className="note">Policies were tuned and trained on ordinary weekdays only and are evaluated unchanged here. Day-type multipliers are modelling assumptions, not measured Bengaluru data.</p>
        </section>
      )}
    </div>
  )
}
