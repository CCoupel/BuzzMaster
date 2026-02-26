import { useState, useRef, useEffect } from 'react'
import useWebSerial from '../hooks/useWebSerial'
import useEspFlash from '../hooks/useEspFlash'
import Button from './Button'
import './USBConfigModal.css'

// ANSI escape code to CSS class mapping
const ANSI_COLORS = {
  '30': 'ansi-black',
  '31': 'ansi-red',
  '32': 'ansi-green',
  '33': 'ansi-yellow',
  '34': 'ansi-blue',
  '35': 'ansi-magenta',
  '36': 'ansi-cyan',
  '37': 'ansi-white',
  '90': 'ansi-gray',
}

// Parse ANSI escape codes into an array of {text, classes} segments
// Handles: \x1b[0m (reset), \x1b[1m (bold), \x1b[Nm (colors), \x1b[1;Nm (bold+color)
// Also handles bracket-only codes like [1;36m (missing ESC byte from serial)
function parseAnsi(text) {
  // Match both \x1b[...m and bare [...m (firmware sometimes sends without ESC)
  const re = /\x1b\[([\d;]*)m|\[([\d;]+)m/g
  const segments = []
  let lastIndex = 0
  let currentClasses = []
  let match

  while ((match = re.exec(text)) !== null) {
    // Text before this escape code
    if (match.index > lastIndex) {
      const before = text.slice(lastIndex, match.index)
      if (before) {
        segments.push({ text: before, classes: [...currentClasses] })
      }
    }

    // Parse the code params (group 1 for ESC variant, group 2 for bare bracket)
    const raw = match[1] !== undefined ? match[1] : match[2]
    const params = raw.split(';').filter(Boolean)
    if (params.length === 0 || (params.length === 1 && params[0] === '0')) {
      currentClasses = []
    } else {
      for (const p of params) {
        if (p === '0') {
          currentClasses = []
        } else if (p === '1') {
          if (!currentClasses.includes('ansi-bold')) currentClasses.push('ansi-bold')
        } else if (ANSI_COLORS[p]) {
          currentClasses = currentClasses.filter(c => !c.startsWith('ansi-') || c === 'ansi-bold')
          currentClasses.push(ANSI_COLORS[p])
        }
      }
    }

    lastIndex = match.index + match[0].length
  }

  // Remaining text after last code
  if (lastIndex < text.length) {
    segments.push({ text: text.slice(lastIndex), classes: [...currentClasses] })
  }

  // If no ANSI codes found, return the whole text as one segment
  if (segments.length === 0) {
    return [{ text, classes: [] }]
  }

  return segments
}

function getPortLabel(port, index) {
  const info = port.getInfo()
  if (info.usbVendorId) {
    return `Port USB #${index + 1} (VID:${info.usbVendorId.toString(16).toUpperCase()} PID:${info.usbProductId?.toString(16).toUpperCase() || '?'})`
  }
  return `Port serie #${index + 1}`
}

export default function USBConfigModal({ onClose, wifiConfig, firmwareInfo }) {
  const { connected, connecting, logs, connectToPort, disconnect, sendCommand, sendCommandAndWait, clearLogs } = useWebSerial()
  const { flashing, flashProgress, flashLogs, pushFlashLog, flashFirmware, clearFlashLogs } = useEspFlash()

  const [sending, setSending] = useState(false)
  const [ports, setPorts] = useState([])
  const [selectedPortIndex, setSelectedPortIndex] = useState(null)
  const [flashVerified, setFlashVerified] = useState(false)

  const logsEndRef = useRef(null)
  const flashLogsEndRef = useRef(null)

  // List already-authorized ports on mount
  useEffect(() => {
    if ('serial' in navigator) {
      navigator.serial.getPorts().then(setPorts).catch(() => {})
    }
  }, [])

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  useEffect(() => {
    flashLogsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [flashLogs])

  // Disconnect serial port on unmount (modal close)
  useEffect(() => {
    return () => {
      disconnect()
    }
  }, [disconnect])

  // Close on Escape
  useEffect(() => {
    const handleKey = (e) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  const refreshPorts = async () => {
    if (!('serial' in navigator)) return
    try {
      const existing = await navigator.serial.getPorts()
      setPorts(existing)
      setSelectedPortIndex(null)
    } catch (err) {
      // ignore
    }
  }

  const handleRequestNewPort = async () => {
    if (!('serial' in navigator)) return
    try {
      await navigator.serial.requestPort()
      await refreshPorts()
    } catch (err) {
      // User cancelled the picker
    }
  }

  const handleConnectToSelected = async () => {
    if (selectedPortIndex === null || !ports[selectedPortIndex]) return
    const ok = await connectToPort(ports[selectedPortIndex])
    if (ok) {
      await sendCommand('AT')
      await new Promise(r => setTimeout(r, 300))
      await sendCommand('AT+SHOW')
    }
  }

  const handleSendConfig = async () => {
    if (!connected || !wifiConfig?.ssid) return
    setSending(true)
    try {
      await sendCommand('AT+SSID=' + wifiConfig.ssid)
      await new Promise(r => setTimeout(r, 200))
      if (wifiConfig.password) {
        await sendCommand('AT+PASS=' + wifiConfig.password)
        await new Promise(r => setTimeout(r, 200))
      }
      if (wifiConfig.serverIp) {
        await sendCommand('AT+SERVERIP=' + wifiConfig.serverIp)
        await new Promise(r => setTimeout(r, 200))
      }
      if (wifiConfig.serverPort > 0) {
        await sendCommand('AT+SERVERPORT=' + wifiConfig.serverPort)
        await new Promise(r => setTimeout(r, 200))
      }
      await sendCommand('AT+SAVE')
    } finally {
      setSending(false)
    }
  }

  const handleShowConfig = () => sendCommand('AT+SHOW')
  const handleGetVersion = () => sendCommand('AT+VERSION')
  const handleGetMac = () => sendCommand('AT+MAC')
  const handleFactoryReset = () => {
    if (window.confirm('Reinitialiser le buzzer aux parametres usine ? Le buzzer va redemarrer.')) {
      sendCommand('AT+FACTORY')
    }
  }

  const handleFlash = async () => {
    if (connected) {
      await disconnect()
      await new Promise(r => setTimeout(r, 100))
    }
    setFlashVerified(false)
    clearFlashLogs()
    const ok = await flashFirmware(ports[selectedPortIndex])
    if (!ok) return

    // Post-flash: verify firmware version using useWebSerial's proven connection path
    const expectedVersion = firmwareInfo?.VERSION
    const MAX_ATTEMPTS = 5
    let verified = false
    pushFlashLog('Vérification de la version firmware...')
    for (let attempt = 1; attempt <= MAX_ATTEMPTS && !verified; attempt++) {
      pushFlashLog(`Tentative ${attempt}/${MAX_ATTEMPTS}`)
      // Give device extra time on first attempt (fresh reboot after flash)
      await new Promise(r => setTimeout(r, attempt === 1 ? 1000 : 500))
      const portOk = await connectToPort(ports[selectedPortIndex])
      if (!portOk) { pushFlashLog('Reconnexion échouée, nouvelle tentative...'); continue }
      // Wait for AT command processor to initialise (USB subsystem ready ~2s after boot)
      await new Promise(r => setTimeout(r, 2000))
      const response = await sendCommandAndWait('AT+VERSION', 3000, line => /\d+\.\d+\.\d+/.test(line))
      await disconnect()
      if (!response) { pushFlashLog('Pas de réponse, nouvelle tentative...'); continue }
      const versionMatch = response.match(/(\d+\.\d+\.\d+)/)
      if (versionMatch) {
        const flashedVersion = versionMatch[1]
        if (expectedVersion && flashedVersion === expectedVersion) {
          pushFlashLog(`✓ Version vérifiée : ${flashedVersion}`)
        } else if (expectedVersion) {
          pushFlashLog(`⚠ Version lue : ${flashedVersion} (attendu : ${expectedVersion})`)
        } else {
          pushFlashLog(`Version firmware : ${flashedVersion}`)
        }
        verified = true
        setFlashVerified(true)
      }
    }
    if (!verified) pushFlashLog(`⚠ Version non vérifiée après ${MAX_ATTEMPTS} tentatives`)
    pushFlashLog(verified ? '✓ Flash terminé avec succès !' : 'Flash terminé.')
  }

  const isWebSerialSupported = 'serial' in navigator

  return (
    <div className="usb-modal-overlay" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="usb-modal">
        <div className="usb-modal-header">
          <h2>Configuration USB Buzzer</h2>
          <button className="usb-modal-close" onClick={onClose}>&times;</button>
        </div>

        <div className="usb-modal-body">
          {!isWebSerialSupported && (
            <div className="usb-warning">
              Web Serial API non disponible. Utilisez Chrome ou Edge sur localhost.
            </div>
          )}

          {/* USB Ports Detection */}
          <div className="usb-section">
            <div className="usb-section-header">
              <h3>Buzzers USB detectes</h3>
              <span className={`usb-status ${connected ? 'connected' : ''}`}>
                {connected ? 'Connecte' : 'Deconnecte'}
              </span>
            </div>

            {connected ? (
              <Button variant="secondary" onClick={() => disconnect()}>
                Deconnecter
              </Button>
            ) : (
              <>
                {ports.length > 0 ? (
                  <div className="usb-port-list">
                    {ports.map((port, i) => (
                      <div
                        key={i}
                        className={`usb-port-item ${selectedPortIndex === i ? 'selected' : ''}`}
                        onClick={() => setSelectedPortIndex(i)}
                      >
                        <span className="usb-port-label">{getPortLabel(port, i)}</span>
                        {selectedPortIndex === i && (
                          <span className="usb-port-selected-badge">Selectionne</span>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="usb-no-ports">Aucun buzzer detecte</div>
                )}
                <div className="usb-port-actions">
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleConnectToSelected}
                    loading={connecting}
                    disabled={!isWebSerialSupported || selectedPortIndex === null}
                  >
                    Connecter
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleRequestNewPort}
                    disabled={!isWebSerialSupported}
                  >
                    Ajouter un port USB
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={refreshPorts}
                    disabled={!isWebSerialSupported}
                  >
                    Rafraichir
                  </Button>
                </div>
              </>
            )}
          </div>

          {/* Config summary + send */}
          <div className="usb-section">
            <h3>Configuration a envoyer</h3>
            <div className="usb-config-summary">
              <div className="usb-config-row">
                <span className="usb-config-label">SSID</span>
                <span className="usb-config-value">{wifiConfig?.ssid || '(non defini)'}</span>
              </div>
              <div className="usb-config-row">
                <span className="usb-config-label">Mot de passe</span>
                <span className="usb-config-value">{wifiConfig?.password ? '********' : '(non defini)'}</span>
              </div>
              <div className="usb-config-row">
                <span className="usb-config-label">IP Serveur</span>
                <span className="usb-config-value">{wifiConfig?.serverIp || '(non defini)'}</span>
              </div>
              <div className="usb-config-row">
                <span className="usb-config-label">Port Serveur</span>
                <span className="usb-config-value">{wifiConfig?.serverPort || 80}</span>
              </div>
            </div>
            <div className="usb-form-actions">
              <Button
                variant="primary"
                onClick={handleSendConfig}
                disabled={!connected || !wifiConfig?.ssid}
                loading={sending}
              >
                Envoyer et configurer
              </Button>
            </div>
          </div>

          {/* Flash Firmware */}
          <div className="usb-section">
            <h3>Flash Firmware via USB</h3>
            <div className="usb-form-actions">
              <Button
                variant="warning"
                onClick={handleFlash}
                disabled={!firmwareInfo?.IS_MERGED || flashing || selectedPortIndex === null}
                loading={flashing}
              >
                {flashing ? `Flash en cours... ${flashProgress}%` : 'Flasher via USB'}
              </Button>
            </div>
            {(flashing || flashLogs.length > 0) && (
              <div className="usb-flash-progress">
                <div
                  className={`usb-flash-progress-bar${flashVerified ? ' verified' : ''}`}
                  style={{ width: `${flashProgress}%` }}
                />
              </div>
            )}
            {flashLogs.length > 0 && (
              <div className="usb-logs usb-flash-logs">
                {flashLogs.map((line, i) => (
                  <div key={i} className="usb-log">{line}</div>
                ))}
                <div ref={flashLogsEndRef} />
              </div>
            )}
            {firmwareInfo !== null && !firmwareInfo?.EXISTS && (
              <p className="usb-flash-unavailable">
                Aucun firmware disponible sur le serveur. Uploadez un fichier .bin dans la section Firmware.
              </p>
            )}
            {firmwareInfo !== null && firmwareInfo?.EXISTS && !firmwareInfo?.IS_MERGED && (
              <p className="usb-flash-unavailable">
                Le firmware stocké est un binaire app-only (non merged). Le flash USB nécessite un binaire merged (bootloader + partitions + app). Uploadez un firmware issu d&apos;une release CI.
              </p>
            )}
          </div>

          {/* Quick actions */}
          <div className={`usb-section ${!connected ? 'disabled' : ''}`}>
            <h3>Actions rapides</h3>
            <div className="usb-quick-actions">
              <Button variant="secondary" size="sm" onClick={handleShowConfig} disabled={!connected}>
                Voir config
              </Button>
              <Button variant="secondary" size="sm" onClick={handleGetVersion} disabled={!connected}>
                Version
              </Button>
              <Button variant="secondary" size="sm" onClick={handleGetMac} disabled={!connected}>
                Adresse MAC
              </Button>
              <Button variant="secondary" size="sm" onClick={handleFactoryReset} disabled={!connected}>
                Factory reset
              </Button>
            </div>
          </div>

          {/* AT Logs */}
          <div className="usb-section">
            <div className="usb-section-header">
              <h3>Console AT</h3>
              <button className="usb-clear-btn" onClick={clearLogs}>Effacer</button>
            </div>
            <div className="usb-logs">
              {logs.length === 0 && (
                <div className="usb-logs-empty">Connectez un buzzer pour commencer...</div>
              )}
              {logs.map((log, i) => {
                const segments = parseAnsi(log.text)
                const hasAnsi = segments.some(s => s.classes.length > 0)
                return (
                  <div key={i} className={`usb-log ${hasAnsi ? '' : `usb-log-${log.type}`}`}>
                    {hasAnsi ? segments.map((seg, j) => (
                      seg.classes.length > 0
                        ? <span key={j} className={seg.classes.join(' ')}>{seg.text}</span>
                        : seg.text
                    )) : log.text}
                  </div>
                )
              })}
              <div ref={logsEndRef} />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
