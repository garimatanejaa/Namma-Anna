import { useCallback, useEffect, useRef, useState } from 'react'
import { createSession, sendCmd, closeSession } from './api.js'

// Owns one simulation session: creates it, streams frames over SSE, accumulates the KPI series.
export default function useSession(initial) {
  const [frame, setFrame] = useState(null)
  const [series, setSeries] = useState([])
  const [status, setStatus] = useState('connecting') // connecting | live | error
  const [error, setError] = useState('')
  const idRef = useRef(null)
  const esRef = useRef(null)
  const seriesRef = useRef([])
  const optsRef = useRef(initial)
  const rafRef = useRef(0)
  const pending = useRef(null)

  const flush = useCallback(() => {
    rafRef.current = 0
    if (pending.current) {
      setFrame(pending.current)
      setSeries(seriesRef.current.slice())
      pending.current = null
    }
  }, [])

  const stop = useCallback(() => {
    if (esRef.current) esRef.current.close()
    esRef.current = null
    if (idRef.current) closeSession(idRef.current)
    idRef.current = null
  }, [])

  const start = useCallback(
    async (patch = {}) => {
      stop()
      optsRef.current = { ...optsRef.current, ...patch }
      seriesRef.current = []
      setSeries([])
      setFrame(null)
      setStatus('connecting')
      try {
        const { id } = await createSession(optsRef.current)
        idRef.current = id
        const es = new EventSource(`/api/sessions/${id}/stream`)
        esRef.current = es
        let first = true
        es.onmessage = (m) => {
          const f = JSON.parse(m.data)
          if (first) {
            seriesRef.current = f.series || []
            first = false
          } else if (f.series && f.series.length) {
            seriesRef.current = seriesRef.current.concat(f.series)
          }
          pending.current = f
          setStatus('live')
          if (!rafRef.current) rafRef.current = requestAnimationFrame(flush)
        }
        es.onerror = () => {
          if (es.readyState === 2) {
            setStatus('error')
            setError('Stream closed — the session may have expired. Start a new day.')
          }
        }
      } catch (e) {
        setStatus('error')
        setError(String(e.message || e))
      }
    },
    [flush, stop],
  )

  useEffect(() => {
    start()
    return stop
  }, []) // eslint-disable-line

  const cmd = useCallback((c) => idRef.current && sendCmd(idRef.current, c), [])
  return { frame, series, status, error, cmd, start, opts: optsRef }
}
