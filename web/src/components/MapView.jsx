import React, { useEffect, useRef } from 'react'
import maplibregl from 'maplibre-gl'
import { LAYERS, rampStops, clamp, TYPE_COLORS } from '../util.js'

const SRC = 'wards'
const ANCHOR_COLOR = { employ: '#c58bff', transit: '#4cd6a7', retail: '#ff9d5c' }

const supplyIdle = (frame, i, mode) => (mode >= 0 ? frame.idle_m[mode][i] : frame.idle[i])

// Map value → 0..1 for the current layer.
function normalise(layer, frame, city, scale, supplyMode) {
  const z = city.wards.length
  const out = new Float32Array(z)
  for (let i = 0; i < z; i++) {
    const a = city.wards[i].area_km2 || 1
    let v = 0
    switch (layer) {
      case 'surge': v = frame.level[i] / 4; break
      case 'incentive': v = frame.inc[i] / 2; break
      case 'demand': v = Math.sqrt(frame.req[i] / a) / scale.demand; break
      case 'unmet': { const r = frame.req[i]; v = r > 0 ? frame.unmet[i] / r : 0; break }
      case 'supply': v = Math.sqrt(supplyIdle(frame, i, supplyMode) / a) / scale.supply; break
      case 'loyalty': v = (frame.loyalty[i] - 0.7) / (1.25 - 0.7); break
      default: v = 0
    }
    out[i] = clamp(v, 0, 1)
  }
  return out
}

export default function MapView({ city, geo, frame, layer, extrude, landmarks, selected, onSelect, onHover, supplyMode }) {
  const el = useRef(null)
  const map = useRef(null)
  const ready = useRef(false)
  const scale = useRef({ demand: 1, supply: 1 })
  const markers = useRef([])
  const hoverId = useRef(null)
  const lastSel = useRef(null)
  const cb = useRef({})
  cb.current = { onSelect, onHover, frame, layer, city, supplyMode }

  // init once
  useEffect(() => {
    const m = new maplibregl.Map({
      container: el.current,
      style: { version: 8, sources: {}, layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#080a0f' } }] },
      center: city.meta.origin, zoom: 9.9, minZoom: 8.8, maxZoom: 14, pitch: 0, attributionControl: false, dragRotate: true,
      maxBounds: [[77.1, 12.6], [78.1, 13.4]],
    })
    map.current = m
    m.addControl(new maplibregl.NavigationControl({ visualizePitch: true, showCompass: true }), 'bottom-right')
    m.addControl(new maplibregl.AttributionControl({ compact: true, customAttribution: 'Wards: KSRSAC/DataMeet · Schools: Open Bangalore (ODbL)' }), 'bottom-right')
    const lngs = city.wards.map((w) => w.lng), lats = city.wards.map((w) => w.lat)
    m.fitBounds([[Math.min(...lngs) - 0.02, Math.min(...lats) - 0.02], [Math.max(...lngs) + 0.02, Math.max(...lats) + 0.02]], { padding: { top: 70, left: 30, right: 30, bottom: 110 }, duration: 0 })
    m.on('load', () => {
      m.addSource(SRC, { type: 'geojson', data: geo, promoteId: 'id' })
      const v = ['coalesce', ['feature-state', 'v'], 0]
      m.addLayer({
        id: 'fill', type: 'fill', source: SRC,
        paint: { 'fill-color': ['interpolate', ['linear'], v, ...rampStops('surge')], 'fill-opacity': 0.94 },
      })
      m.addLayer({
        id: 'lines', type: 'line', source: SRC,
        paint: { 'line-color': '#0a0c11', 'line-width': ['interpolate', ['linear'], ['zoom'], 9, 0.4, 13, 1.2], 'line-opacity': 0.9 },
      })
      m.addLayer({
        id: 'hover', type: 'line', source: SRC,
        paint: {
          'line-color': '#ffffff',
          'line-width': ['case', ['boolean', ['feature-state', 'sel'], false], 3, ['boolean', ['feature-state', 'hov'], false], 2, 0],
          'line-opacity': ['case', ['boolean', ['feature-state', 'sel'], false], 1, 0.85],
        },
      })
      m.addLayer({
        id: 'ext', type: 'fill-extrusion', source: SRC, layout: { visibility: 'none' },
        paint: {
          'fill-extrusion-color': ['interpolate', ['linear'], v, ...rampStops('surge')],
          'fill-extrusion-height': ['+', 250, ['*', v, 5200]],
          'fill-extrusion-opacity': 0.96,
        },
      })
      m.on('mousemove', 'fill', (e) => {
        m.getCanvas().style.cursor = 'pointer'
        const id = e.features[0]?.id
        if (id == null) return
        if (hoverId.current !== id) {
          if (hoverId.current != null) m.setFeatureState({ source: SRC, id: hoverId.current }, { hov: false })
          hoverId.current = id
          m.setFeatureState({ source: SRC, id }, { hov: true })
        }
        cb.current.onHover({ id, x: e.point.x, y: e.point.y })
      })
      m.on('mouseleave', 'fill', () => {
        m.getCanvas().style.cursor = ''
        if (hoverId.current != null) m.setFeatureState({ source: SRC, id: hoverId.current }, { hov: false })
        hoverId.current = null
        cb.current.onHover(null)
      })
      m.on('click', (e) => {
        const f = m.queryRenderedFeatures(e.point, { layers: [m.getLayoutProperty('ext', 'visibility') === 'visible' ? 'ext' : 'fill'] })
        cb.current.onSelect(f[0]?.id ?? null)
      })
      ready.current = true
      paint()
    })
    // landmarks
    for (const a of city.meta.anchors) {
      if (a.weight < 0.8) continue
      const d = document.createElement('div')
      d.className = 'lm'
      d.innerHTML = `<i style="background:${ANCHOR_COLOR[a.kind]}"></i><span>${a.name.replace(/ \(.*\)/, '').replace(/ - .*/, '')}</span>`
      const mk = new maplibregl.Marker({ element: d, anchor: 'left' }).setLngLat([a.lng, a.lat]).addTo(m)
      markers.current.push(d)
    }
    return () => m.remove()
  }, []) // eslint-disable-line

  function paint() {
    const m = map.current
    const { frame, layer, city } = cb.current
    if (!ready.current || !frame) return
    // adapt demand/supply scales slowly so the map stays readable all day
    const z = city.wards.length
    let dmax = 0, smax = 0
    const ds = [], ss = []
    for (let i = 0; i < z; i++) {
      const a = city.wards[i].area_km2 || 1
      ds.push(Math.sqrt(frame.req[i] / a)); ss.push(Math.sqrt(supplyIdle(frame, i, cb.current.supplyMode) / a))
    }
    ds.sort((a, b) => a - b); ss.sort((a, b) => a - b)
    dmax = ds[Math.floor(z * 0.97)] || 1; smax = ss[Math.floor(z * 0.97)] || 1
    scale.current.demand = Math.max(0.25, scale.current.demand * 0.85 + dmax * 0.15)
    scale.current.supply = Math.max(0.25, scale.current.supply * 0.85 + smax * 0.15)
    const vals = normalise(layer, frame, city, scale.current, cb.current.supplyMode)
    for (let i = 0; i < z; i++) m.setFeatureState({ source: SRC, id: i }, { v: vals[i] })
  }

  useEffect(paint, [frame, layer, supplyMode]) // eslint-disable-line

  // switch colour ramp / 3D
  useEffect(() => {
    const m = map.current
    if (!m || !ready.current) return
    const L = LAYERS.find((l) => l.key === layer)
    const v = ['coalesce', ['feature-state', 'v'], 0]
    const expr = ['interpolate', ['linear'], v, ...rampStops(L.ramp)]
    m.setPaintProperty('fill', 'fill-color', expr)
    m.setPaintProperty('ext', 'fill-extrusion-color', expr)
  }, [layer])

  useEffect(() => {
    const m = map.current
    const go = () => {
      m.setLayoutProperty('ext', 'visibility', extrude ? 'visible' : 'none')
      m.setLayoutProperty('fill', 'visibility', extrude ? 'none' : 'visible')
      m.setLayoutProperty('lines', 'visibility', extrude ? 'none' : 'visible')
      m.easeTo({ pitch: extrude ? 58 : 0, bearing: extrude ? -18 : 0, duration: 900 })
    }
    if (m && ready.current) go()
    else m && m.once('load', go)
  }, [extrude])

  useEffect(() => {
    markers.current.forEach((d) => (d.style.display = landmarks ? '' : 'none'))
  }, [landmarks])

  useEffect(() => {
    const m = map.current
    if (!m || !ready.current) return
    if (lastSel.current != null) m.setFeatureState({ source: SRC, id: lastSel.current }, { sel: false })
    if (selected != null) m.setFeatureState({ source: SRC, id: selected }, { sel: true })
    lastSel.current = selected
  }, [selected])

  return <div ref={el} className="map" />
}
