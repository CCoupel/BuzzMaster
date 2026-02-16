import { useState, useRef, useCallback, useEffect } from 'react'

const BAUD_RATE = 115200

export default function useWebSerial() {
  const [connected, setConnected] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [logs, setLogs] = useState([])
  const portRef = useRef(null)
  const readerRef = useRef(null)
  const writerRef = useRef(null)
  const readLoopRef = useRef(false)
  const mountedRef = useRef(true)

  // Track mounted state to avoid state updates after unmount
  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const addLog = useCallback((text, type = 'info') => {
    setLogs(prev => [...prev, { text, type, time: Date.now() }])
  }, [])

  const readLoop = useCallback(async () => {
    const decoder = new TextDecoderStream()
    const inputDone = portRef.current.readable.pipeTo(decoder.writable)
    const reader = decoder.readable.getReader()
    readerRef.current = reader
    readLoopRef.current = true

    let buffer = ''
    try {
      while (readLoopRef.current) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += value
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''
        for (const line of lines) {
          const trimmed = line.trim()
          if (trimmed) {
            const isError = trimmed.startsWith('ERROR')
            addLog(trimmed, isError ? 'error' : 'response')
          }
        }
      }
    } catch (err) {
      if (readLoopRef.current) {
        addLog('Lecture interrompue: ' + err.message, 'error')
      }
    } finally {
      try { reader.releaseLock() } catch (_) {}
      try { await inputDone } catch (_) {}
    }
  }, [addLog])

  const openPort = useCallback(async (port) => {
    await port.open({ baudRate: BAUD_RATE })
    portRef.current = port

    const encoder = new TextEncoderStream()
    const outputDone = encoder.readable.pipeTo(port.writable)
    writerRef.current = encoder.writable.getWriter()
    writerRef.current._outputDone = outputDone

    readLoop()

    setConnected(true)
    addLog('Connecte au buzzer (115200 baud)', 'success')
  }, [addLog, readLoop])

  const connect = useCallback(async () => {
    if (!('serial' in navigator)) {
      addLog('Web Serial API non disponible. Utilisez Chrome/Edge sur localhost.', 'error')
      return false
    }

    setConnecting(true)
    try {
      const port = await navigator.serial.requestPort()
      await openPort(port)
      return true
    } catch (err) {
      addLog('Connexion echouee: ' + err.message, 'error')
      return false
    } finally {
      setConnecting(false)
    }
  }, [addLog, openPort])

  const connectToPort = useCallback(async (port) => {
    setConnecting(true)
    try {
      await openPort(port)
      return true
    } catch (err) {
      addLog('Connexion echouee: ' + err.message, 'error')
      return false
    } finally {
      setConnecting(false)
    }
  }, [addLog, openPort])

  const disconnect = useCallback(async () => {
    // Capture refs before any async work — they may be nulled by React unmount
    const port = portRef.current
    const writer = writerRef.current
    const reader = readerRef.current

    // Signal read loop to stop
    readLoopRef.current = false

    // Clear refs immediately so concurrent calls are no-ops
    portRef.current = null
    writerRef.current = null
    readerRef.current = null

    try {
      // 1. Close writer stream
      if (writer) {
        try { await writer.close() } catch (_) {}
        try { await writer._outputDone } catch (_) {}
      }
      // 2. Cancel reader (this also unblocks the readLoop)
      if (reader) {
        try { await reader.cancel() } catch (_) {}
      }
      // 3. Close the actual serial port — small delay to let streams settle
      if (port) {
        await new Promise(r => setTimeout(r, 50))
        try { await port.close() } catch (_) {}
      }
    } catch (err) {
      // Ignore close errors
    }
    if (mountedRef.current) {
      setConnected(false)
      addLog('Deconnecte', 'info')
    }
  }, [addLog])

  const sendCommand = useCallback(async (cmd) => {
    if (!writerRef.current) {
      addLog('Non connecte', 'error')
      return
    }
    addLog('> ' + cmd, 'command')
    await writerRef.current.write(cmd + '\n')
  }, [addLog])

  const clearLogs = useCallback(() => {
    setLogs([])
  }, [])

  return {
    connected,
    connecting,
    logs,
    connect,
    connectToPort,
    disconnect,
    sendCommand,
    clearLogs
  }
}
