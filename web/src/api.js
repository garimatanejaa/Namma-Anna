const j = async (r) => {
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || r.statusText)
  return r.status === 204 ? null : r.json()
}
export const getCity = () => fetch('/api/city').then(j)
export const getGeo = () => fetch('/api/wards.geojson').then(j)
export const getPolicies = () => fetch('/api/policies').then(j)
export const getBenchmark = () => fetch('/api/benchmark').then(j)
export const getTraining = () => fetch('/api/training').then(j)
export const createSession = (body) =>
  fetch('/api/sessions', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }).then(j)
export const sendCmd = (id, c) =>
  fetch(`/api/sessions/${id}/cmd`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(c) }).catch(() => {})
export const closeSession = (id) => fetch(`/api/sessions/${id}`, { method: 'DELETE', keepalive: true }).catch(() => {})
export const runBenchmark = (body) =>
  fetch('/api/benchmark/run', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }).then(j)
export const benchStatus = () => fetch('/api/benchmark/status').then(j)
export const getDayBench = () => fetch('/api/benchmark/daytypes').then(j)
