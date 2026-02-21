import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import USBConfigModal from '../components/USBConfigModal'
import { OtaAllModal } from '../components/TeamCard'
import useEspFlash from '../hooks/useEspFlash'
import './ConfigPage.css'

function getPortLabel(port, index) {
  const info = port.getInfo()
  if (info.usbVendorId) {
    return `Port USB #${index + 1} (VID:${info.usbVendorId.toString(16).toUpperCase()} PID:${info.usbProductId?.toString(16).toUpperCase() || '?'})`
  }
  return `Port serie #${index + 1}`
}

export default function ConfigPage() {
  const { teams, bumpers, gameState, updateConfig, sendMessage, version, firmwareInfo: wsFirmwareInfo } = useGame()
  const dualRangeTrackRef = useRef(null)

  // Count outdated physical WebSocket buzzers only (those with FIRMWARE_VERSION set).
  // Excludes TCP-only/simulated bumpers (no FIRMWARE_VERSION) and VJoueurs.
  const outdatedCount = useMemo(() =>
    Object.values(bumpers).filter(b => b.IS_OUTDATED === true && b.FIRMWARE_VERSION).length
  , [bumpers])

  // Neon effect configuration
  const [neonConfig, setNeonConfig] = useState({
    enabled: false,
    mode: 'bar',
    arc_width: 60,
    intensity_gap: 80,
    rotation_speed: 4,
    glow_pulse_speed: 2,
    glow_pulse_min: 30,
    glow_pulse_max: 50,
    bar_offset: 20,
    bar_thickness: 4,
    arc_blur: 100
  })
  const [savingNeon, setSavingNeon] = useState(false)

  const [loadingDemo, setLoadingDemo] = useState(false)
  const [showUSBModal, setShowUSBModal] = useState(false)

  // Server parameters
  const [serverParams, setServerParams] = useState({
    auto_open_browsers: false,
    debug: false
  })
  const [savingParams, setSavingParams] = useState(false)

  // WiFi config
  const [wifiSsid, setWifiSsid] = useState('')
  const [wifiPassword, setWifiPassword] = useState('')
  const [wifiSsid2, setWifiSsid2] = useState('')
  const [wifiPassword2, setWifiPassword2] = useState('')
  const [wifiServerIp, setWifiServerIp] = useState('')
  const [wifiServerPort, setWifiServerPort] = useState(80)
  const [savingWifi, setSavingWifi] = useState(false)
  const [wifiToast, setWifiToast] = useState(null)
  const [broadcastingWifi, setBroadcastingWifi] = useState(false)

  // Firmware section
  const [firmwareInfo, setFirmwareInfo] = useState(null) // { VERSION, FILENAME, SIZE, EXISTS, EMBEDDED_VERSION }
  const [uploadingFirmware, setUploadingFirmware] = useState(false)
  const [restoringEmbedded, setRestoringEmbedded] = useState(false)
  const [showOtaAllModal, setShowOtaAllModal] = useState(false)
  const [firmwareToast, setFirmwareToast] = useState(null) // { message, type }
  const firmwareFileRef = useRef(null)

  // USB Flash section
  const [flashPorts, setFlashPorts] = useState([])
  const [selectedFlashPortIdx, setSelectedFlashPortIdx] = useState(null)
  const { flashing, flashProgress, flashLogs, flashFirmware, clearFlashLogs } = useEspFlash()

  // WiFi toast auto-hide
  useEffect(() => {
    if (wifiToast) {
      const timer = setTimeout(() => setWifiToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [wifiToast])

  // Firmware toast auto-hide
  useEffect(() => {
    if (firmwareToast) {
      const timer = setTimeout(() => setFirmwareToast(null), 4000)
      return () => clearTimeout(timer)
    }
  }, [firmwareToast])

  // Load neon config and server parameters from server on mount
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await fetch('/config.json')
        if (response.ok) {
          const data = await response.json()
          if (data.neon_effect) {
            setNeonConfig(data.neon_effect)
          }
          if (data.server) {
            setServerParams({
              auto_open_browsers: data.server.auto_open_browsers || false,
              debug: data.server.debug || false
            })
          }
        }
      } catch (error) {
        console.error('Failed to fetch config:', error)
      }
    }
    fetchConfig()
  }, [])

  // Load WiFi defaults on mount
  useEffect(() => {
    const fetchWifiDefaults = async () => {
      try {
        const res = await fetch('/api/wifi/defaults')
        if (res.ok) {
          const data = await res.json()
          if (data.ssid) setWifiSsid(data.ssid)
          if (data.password) setWifiPassword(data.password)
          if (data.ssid2) setWifiSsid2(data.ssid2)
          if (data.password2) setWifiPassword2(data.password2)
          if (data.server_ip) setWifiServerIp(data.server_ip)
          if (data.server_port) setWifiServerPort(data.server_port)
        }
      } catch (err) {
        // Defaults not available
      }
    }
    fetchWifiDefaults()
  }, [])

  // Load authorized USB ports for the flash section on mount
  useEffect(() => {
    if ('serial' in navigator) {
      navigator.serial.getPorts().then(setFlashPorts).catch(() => {})
    }
  }, [])

  // Fetch firmware info on mount
  useEffect(() => {
    const fetchFirmwareInfo = async () => {
      try {
        const res = await fetch('/api/firmware/buzzclick/version')
        if (res.ok) {
          const data = await res.json()
          setFirmwareInfo(data)
        }
      } catch {
        // Firmware endpoint not available (ignore)
      }
    }
    fetchFirmwareInfo()
  }, [])

  // Update firmware info from WebSocket broadcast (after upload)
  useEffect(() => {
    if (wsFirmwareInfo) {
      setFirmwareInfo(wsFirmwareInfo)
    }
  }, [wsFirmwareInfo])

  // Update local state when gameState.neonEffect changes (from WebSocket)
  useEffect(() => {
    if (gameState?.neonEffect) {
      setNeonConfig(gameState.neonEffect)
    }
  }, [gameState?.neonEffect])

  const handleSaveNeonConfig = async () => {
    setSavingNeon(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ neon_effect: neonConfig })
      })
      if (!response.ok) {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Save neon config failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setSavingNeon(false)
    }
  }

  const handleSaveServerParams = async () => {
    setSavingParams(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          server: {
            auto_open_browsers: serverParams.auto_open_browsers,
            debug: serverParams.debug
          }
        })
      })
      if (!response.ok) {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Save server params failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setSavingParams(false)
    }
  }

  const handleResetScores = () => {
    if (!window.confirm('Remettre tous les scores a zero ?')) return
    sendMessage('RAZ', {})
  }

  const handleSaveWifiDefaults = async () => {
    setSavingWifi(true)
    try {
      const res = await fetch('/api/wifi/defaults', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ssid: wifiSsid,
          password: wifiPassword,
          ssid2: wifiSsid2,
          password2: wifiPassword2,
          server_ip: wifiServerIp,
          server_port: wifiServerPort
        })
      })
      if (res.ok) {
        setWifiToast({ message: 'Configuration WiFi sauvegardee', type: 'success' })
      } else {
        const text = await res.text()
        setWifiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (err) {
      setWifiToast({ message: 'Erreur: ' + err.message, type: 'error' })
    } finally {
      setSavingWifi(false)
    }
  }

  const handleBroadcastWifi = async () => {
    setBroadcastingWifi(true)
    try {
      const res = await fetch('/api/buzzer/wifi-config', {
        method: 'POST'
      })
      if (res.ok) {
        const data = await res.json()
        setWifiToast({ message: `Config WiFi envoyee a ${data.count} buzzer(s)`, type: 'success' })
      } else {
        const text = await res.text()
        setWifiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (err) {
      setWifiToast({ message: 'Erreur: ' + err.message, type: 'error' })
    } finally {
      setBroadcastingWifi(false)
    }
  }

  const handleFirmwareUpload = async () => {
    const file = firmwareFileRef.current?.files?.[0]
    if (!file) {
      setFirmwareToast({ message: 'Veuillez selectionner un fichier .bin', type: 'error' })
      return
    }
    // Client-side validation
    if (!file.name.endsWith('.bin')) {
      setFirmwareToast({ message: 'Le fichier doit etre au format .bin', type: 'error' })
      return
    }
    const sizeMB = file.size / (1024 * 1024)
    if (file.size < 200 * 1024) {
      setFirmwareToast({ message: 'Fichier trop petit (minimum 200 Ko)', type: 'error' })
      return
    }
    if (sizeMB > 2) {
      setFirmwareToast({ message: 'Fichier trop grand (maximum 2 Mo)', type: 'error' })
      return
    }

    setUploadingFirmware(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/firmware/buzzclick/upload', {
        method: 'POST',
        body: formData,
      })
      const data = await res.json()
      if (res.ok && data.status === 'ok') {
        setFirmwareInfo({ VERSION: data.version, SIZE: data.size, EXISTS: true, FILENAME: file.name })
        setFirmwareToast({ message: `Firmware ${data.version} uploade avec succes`, type: 'success' })
        if (firmwareFileRef.current) firmwareFileRef.current.value = ''
      } else {
        setFirmwareToast({ message: 'Erreur: ' + (data.message || 'Upload echoue'), type: 'error' })
      }
    } catch (err) {
      setFirmwareToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setUploadingFirmware(false)
    }
  }

  const handleRestoreEmbedded = async () => {
    setRestoringEmbedded(true)
    try {
      const res = await fetch('/api/firmware/buzzclick/restore-embedded', { method: 'POST' })
      const data = await res.json()
      if (res.ok && data.status === 'ok') {
        setFirmwareInfo(prev => ({ ...prev, VERSION: data.version, SIZE: data.size, EXISTS: true, FILENAME: data.filename }))
        setFirmwareToast({ message: `Firmware v${data.version} restaure (firmware embarque)`, type: 'success' })
      } else {
        setFirmwareToast({ message: 'Erreur: ' + (data.message || 'Restauration echouee'), type: 'error' })
      }
    } catch (err) {
      setFirmwareToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setRestoringEmbedded(false)
    }
  }

  const handleUpdateAll = () => {
    setShowOtaAllModal(true)
  }

  const refreshFlashPorts = async () => {
    if (!('serial' in navigator)) return
    try {
      const ports = await navigator.serial.getPorts()
      setFlashPorts(ports)
      setSelectedFlashPortIdx(null)
    } catch (_) {}
  }

  const handleRequestFlashPort = async () => {
    if (!('serial' in navigator)) return
    try {
      await navigator.serial.requestPort()
      await refreshFlashPorts()
    } catch (_) {}
  }

  const handleFlashViaUSB = async () => {
    if (selectedFlashPortIdx === null || !flashPorts[selectedFlashPortIdx]) return
    if (!firmwareInfo?.EXISTS) return
    clearFlashLogs()
    await flashFirmware(flashPorts[selectedFlashPortIdx])
  }

  const handleLoadDemo = async () => {
    if (!window.confirm('Charger les donnees de demonstration ? Les donnees actuelles seront remplacees.')) return

    setLoadingDemo(true)
    try {
      const response = await fetch('/load-demo', { method: 'POST' })
      if (response.ok) {
        window.location.reload()
      } else {
        const data = await response.json()
        alert('Erreur: ' + (data.message || 'Echec du chargement'))
      }
    } catch (error) {
      console.error('Load demo failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setLoadingDemo(false)
    }
  }


  return (
    <div className="config-page page">
      <header className="page-header">
        <h1 className="page-title">Configuration</h1>
        <p className="page-subtitle">Parametres du systeme</p>
      </header>

      <div className="config-layout">
        {/* System Section */}
        <section className="system-section">
          <h2 className="section-title">Systeme</h2>

          <Card padding="lg" className="system-card">
            <div className="system-info">
              {version && (
                <div className="info-item">
                  <span className="info-label">Version serveur</span>
                  <span className="info-value">{version}</span>
                </div>
              )}
              <div className="info-item">
                <span className="info-label">Equipes</span>
                <span className="info-value">{Object.keys(teams).length}</span>
              </div>
              <div className="info-item">
                <span className="info-label">Buzzers</span>
                <span className="info-value">{Object.keys(bumpers).length}</span>
              </div>
            </div>

            <div className="system-actions">
              <Button variant="secondary" onClick={handleResetScores}>
                Remettre les scores a zero
              </Button>
            </div>

            {/* Server Parameters Section */}
            <div className="config-section">
              <h3 className="config-section-title">Parametres serveur</h3>
              <p className="config-section-hint">
                Configuration du comportement du serveur au demarrage et mode de fonctionnement.
              </p>

              <label className="checkbox-item">
                <input
                  type="checkbox"
                  checked={serverParams.auto_open_browsers}
                  onChange={(e) => setServerParams(prev => ({ ...prev, auto_open_browsers: e.target.checked }))}
                />
                <span>Ouvrir les navigateurs automatiquement</span>
              </label>

              <label className="checkbox-item">
                <input
                  type="checkbox"
                  checked={serverParams.debug}
                  onChange={(e) => setServerParams(prev => ({ ...prev, debug: e.target.checked }))}
                />
                <span>Mode debug</span>
              </label>

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveServerParams} loading={savingParams}>
                  Enregistrer
                </Button>
              </div>
            </div>

            {/* WiFi Config Section */}
            <div className="config-section">
              <h3 className="config-section-title">Configuration du WiFi</h3>
              <p className="config-section-hint">
                Parametres WiFi par defaut pour les buzzers. Ces valeurs seront envoyees aux buzzers lors de la configuration USB.
              </p>

              <div className="wifi-form">
                <label className="wifi-field">
                  <span>SSID WiFi</span>
                  <input
                    type="text"
                    value={wifiSsid}
                    onChange={(e) => setWifiSsid(e.target.value)}
                    placeholder="Nom du reseau WiFi"
                    maxLength={32}
                  />
                </label>
                <label className="wifi-field">
                  <span>Mot de passe WiFi</span>
                  <input
                    type="password"
                    value={wifiPassword}
                    onChange={(e) => setWifiPassword(e.target.value)}
                    placeholder="Mot de passe (min 8 car.)"
                    maxLength={63}
                  />
                </label>
                <label className="wifi-field">
                  <span>IP Serveur</span>
                  <input
                    type="text"
                    value={wifiServerIp}
                    onChange={(e) => setWifiServerIp(e.target.value)}
                    placeholder="ex: 192.168.1.100"
                  />
                </label>
                <label className="wifi-field">
                  <span>Port Serveur</span>
                  <input
                    type="number"
                    value={wifiServerPort}
                    onChange={(e) => setWifiServerPort(parseInt(e.target.value) || 0)}
                    min={1}
                    max={65535}
                  />
                </label>
              </div>

              <div className="wifi-fallback-section">
                <h4 className="wifi-fallback-title">WiFi de secours (optionnel)</h4>
                <div className="wifi-form">
                  <label className="wifi-field">
                    <span>SSID WiFi 2 (optionnel)</span>
                    <input
                      type="text"
                      value={wifiSsid2}
                      onChange={(e) => setWifiSsid2(e.target.value)}
                      placeholder="Nom du reseau WiFi secondaire"
                      maxLength={32}
                    />
                  </label>
                  <label className="wifi-field">
                    <span>Mot de passe WiFi 2</span>
                    <input
                      type="password"
                      value={wifiPassword2}
                      onChange={(e) => setWifiPassword2(e.target.value)}
                      placeholder="Mot de passe (min 8 car.)"
                      maxLength={63}
                    />
                  </label>
                </div>
              </div>

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveWifiDefaults} loading={savingWifi}>
                  Sauvegarder
                </Button>
                <Button variant="secondary" onClick={() => setShowUSBModal(true)}>
                  Configuration via USB
                </Button>
              </div>

              <div className="wifi-broadcast-section">
                <div className="wifi-broadcast-warning">
                  <span className="warning-icon">⚠️</span>
                  <span>Les buzzers qui changent de reseau WiFi vont redemarrer automatiquement.</span>
                </div>
                <Button variant="secondary" onClick={handleBroadcastWifi} loading={broadcastingWifi}>
                  Appliquer a tous les buzzers connectes
                </Button>
              </div>
            </div>

            {/* Firmware Buzzers Section */}
            <div className="config-section">
              <h3 className="config-section-title">Firmware Buzzers</h3>
              <p className="config-section-hint">
                Gerez le firmware des buzzers BuzzClick. Uploadez un fichier .bin de reference
                et lancez les mises a jour OTA sur les buzzers connectes.
              </p>

              {/* Current firmware info */}
              <div className="firmware-info-grid">
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Version de reference</span>
                  <span className="firmware-info-value">
                    {firmwareInfo?.VERSION || <span className="firmware-none">aucune</span>}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Fichier</span>
                  <span className="firmware-info-value firmware-filename">
                    {firmwareInfo?.FILENAME || '-'}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Taille</span>
                  <span className="firmware-info-value">
                    {firmwareInfo?.EXISTS ? `${Math.round(firmwareInfo.SIZE / 1024)} Ko` : '—'}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Etat</span>
                  <span className={`firmware-status-badge ${firmwareInfo?.EXISTS ? 'exists' : firmwareInfo?.VERSION ? 'pending' : 'missing'}`}>
                    {firmwareInfo?.EXISTS ? 'Disponible' : firmwareInfo?.VERSION ? 'Non uploade' : 'Absent'}
                  </span>
                </div>
                {firmwareInfo?.EMBEDDED_VERSION && (
                  <div className="firmware-info-item">
                    <span className="firmware-info-label">Version embarquee</span>
                    <span className="firmware-info-value">{firmwareInfo.EMBEDDED_VERSION}</span>
                  </div>
                )}
              </div>

              {/* File upload */}
              <div className="firmware-upload-row">
                <input
                  ref={firmwareFileRef}
                  type="file"
                  accept=".bin"
                  className="firmware-file-input"
                  id="firmware-file-input"
                />
                <label htmlFor="firmware-file-input" className="firmware-file-label">
                  Choisir un fichier .bin (200 Ko - 2 Mo)
                </label>
              </div>

              <div className="config-section-actions">
                <Button
                  variant="primary"
                  onClick={handleFirmwareUpload}
                  loading={uploadingFirmware}
                >
                  Uploader le firmware
                </Button>
                {firmwareInfo?.EMBEDDED_VERSION && firmwareInfo?.VERSION !== firmwareInfo?.EMBEDDED_VERSION && (
                  <Button
                    variant="secondary"
                    onClick={handleRestoreEmbedded}
                    loading={restoringEmbedded}
                  >
                    Restaurer v{firmwareInfo.EMBEDDED_VERSION}
                  </Button>
                )}
                <Button
                  variant="secondary"
                  onClick={handleUpdateAll}
                  disabled={!firmwareInfo?.EXISTS || outdatedCount === 0}
                >
                  {outdatedCount > 0
                    ? `Mettre a jour les ${outdatedCount} buzzer${outdatedCount > 1 ? 's' : ''} obsoletes`
                    : 'Mettre a jour les buzzers obsoletes'}
                </Button>
              </div>

              {/* Flash via USB */}
              <div className="firmware-flash-usb">
                <h4 className="firmware-flash-title">Flash via USB</h4>
                {'serial' in navigator ? (
                  <>
                    <div className="firmware-flash-ports">
                      {flashPorts.length === 0 ? (
                        <div className="firmware-flash-no-ports">Aucun port USB autorise</div>
                      ) : (
                        flashPorts.map((port, i) => (
                          <div
                            key={i}
                            className={`firmware-flash-port-item ${selectedFlashPortIdx === i ? 'selected' : ''}`}
                            onClick={() => setSelectedFlashPortIdx(i)}
                          >
                            {getPortLabel(port, i)}
                          </div>
                        ))
                      )}
                    </div>
                    <div className="firmware-flash-port-actions">
                      <Button variant="secondary" size="sm" onClick={handleRequestFlashPort}>
                        Ajouter un port
                      </Button>
                      <Button variant="secondary" size="sm" onClick={refreshFlashPorts}>
                        Rafraichir
                      </Button>
                    </div>
                    <div className="config-section-actions">
                      <Button
                        variant="warning"
                        onClick={handleFlashViaUSB}
                        disabled={selectedFlashPortIdx === null || flashing || !firmwareInfo?.EXISTS}
                        loading={flashing}
                      >
                        {flashing ? `Flash en cours... ${flashProgress}%` : 'Flasher via USB'}
                      </Button>
                    </div>
                    {flashing && (
                      <div className="firmware-flash-progress">
                        <div className="firmware-flash-progress-bar" style={{ width: `${flashProgress}%` }} />
                      </div>
                    )}
                    {flashLogs.length > 0 && (
                      <div className="firmware-flash-logs">
                        {flashLogs.map((line, i) => (
                          <div key={i} className="firmware-flash-log">{line}</div>
                        ))}
                      </div>
                    )}
                  </>
                ) : (
                  <p className="firmware-flash-unavailable">
                    Web Serial non disponible. Utilisez Chrome/Edge sur localhost.
                  </p>
                )}
              </div>
            </div>

            {/* Demo Section */}
            <div className="config-section">
              <h3 className="config-section-title">Mode Demo</h3>
              <p className="config-section-hint">
                Charge des donnees de demonstration: equipes, joueurs, questions (QCM, Memory, Normal) et historique.
              </p>
              <div className="config-section-actions">
                <Button variant="primary" onClick={handleLoadDemo} loading={loadingDemo}>
                  Charger la demo
                </Button>
              </div>
            </div>

            {/* Neon Effect Section */}
            <div className="config-section">
              <h3 className="config-section-title">Effet Neon</h3>
              <p className="config-section-hint">
                Bordure lumineuse animee autour de l'ecran TV et VJoueur, avec la couleur de la categorie.
              </p>

              <label className="checkbox-item neon-toggle">
                <input
                  type="checkbox"
                  checked={neonConfig.enabled}
                  onChange={(e) => setNeonConfig(prev => ({ ...prev, enabled: e.target.checked }))}
                />
                <span>Activer l'effet neon</span>
              </label>

              {neonConfig.enabled && (
                <div className="neon-sliders">
                  {/* Mode selector */}
                  <div className="slider-row">
                    <label>Mode d'affichage</label>
                    <div className="mode-selector">
                      <button
                        className={`mode-btn ${neonConfig.mode !== 'halo' ? 'active' : ''}`}
                        onClick={() => setNeonConfig(prev => ({ ...prev, mode: 'bar' }))}
                      >
                        Neon
                      </button>
                      <button
                        className={`mode-btn ${neonConfig.mode === 'halo' ? 'active' : ''}`}
                        onClick={() => setNeonConfig(prev => ({ ...prev, mode: 'halo' }))}
                      >
                        Halo
                      </button>
                    </div>
                  </div>

                  {/* Bar mode specific settings */}
                  {neonConfig.mode !== 'halo' && (
                    <>
                      <div className="slider-row">
                        <label>Distance du bord</label>
                        <div className="slider-control">
                          <input
                            type="range"
                            min="10"
                            max="100"
                            value={neonConfig.bar_offset || 20}
                            onChange={(e) => setNeonConfig(prev => ({ ...prev, bar_offset: parseInt(e.target.value) }))}
                          />
                          <span className="slider-value">{neonConfig.bar_offset || 20}px</span>
                        </div>
                      </div>

                      <div className="slider-row">
                        <label>Epaisseur de la barre</label>
                        <div className="slider-control">
                          <input
                            type="range"
                            min="2"
                            max="20"
                            value={neonConfig.bar_thickness || 4}
                            onChange={(e) => setNeonConfig(prev => ({ ...prev, bar_thickness: parseInt(e.target.value) }))}
                          />
                          <span className="slider-value">{neonConfig.bar_thickness || 4}px</span>
                        </div>
                      </div>
                    </>
                  )}

                  {/* Glow section - grouped */}
                  <div className="neon-glow-section">
                    <h4 className="neon-subsection-title">Glow</h4>

                    <div className="slider-row">
                      <label>Vitesse pulsation</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0.5"
                          max="5"
                          step="0.1"
                          value={neonConfig.glow_pulse_speed || 2}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, glow_pulse_speed: parseFloat(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.glow_pulse_speed || 2}s</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Amplitude pulsation</label>
                      <div className="dual-range-container">
                        <div className="dual-range-track" ref={dualRangeTrackRef}>
                          <div
                            className="dual-range-fill"
                            style={{
                              left: `${neonConfig.glow_pulse_min || 30}%`,
                              width: `${(neonConfig.glow_pulse_max || 50) - (neonConfig.glow_pulse_min || 30)}%`,
                              background: `linear-gradient(to right,
                                rgba(0, 200, 200, ${(neonConfig.glow_pulse_min || 30) / 100}),
                                rgba(0, 200, 200, ${(neonConfig.glow_pulse_max || 50) / 100}))`
                            }}
                            onMouseDown={(e) => {
                              e.preventDefault()
                              const track = dualRangeTrackRef.current
                              if (!track) return
                              const trackRect = track.getBoundingClientRect()
                              const startX = e.clientX
                              const startMin = neonConfig.glow_pulse_min || 30
                              const startMax = neonConfig.glow_pulse_max || 50
                              const gap = startMax - startMin

                              const onMouseMove = (moveEvent) => {
                                const deltaX = moveEvent.clientX - startX
                                const deltaPercent = (deltaX / trackRect.width) * 100
                                let newMin = Math.round(startMin + deltaPercent)
                                let newMax = Math.round(startMax + deltaPercent)

                                // Clamp to boundaries - min cannot go below 1%
                                if (newMin < 1) {
                                  newMin = 1
                                  newMax = 1 + gap
                                }
                                // max cannot go above 100
                                if (newMax > 100) {
                                  newMax = 100
                                  newMin = 100 - gap
                                }

                                // Final safety clamp (min at least 1%)
                                newMin = Math.max(1, Math.min(100, newMin))
                                newMax = Math.max(1, Math.min(100, newMax))

                                setNeonConfig(prev => ({
                                  ...prev,
                                  glow_pulse_min: newMin,
                                  glow_pulse_max: newMax
                                }))
                              }

                              const onMouseUp = () => {
                                document.removeEventListener('mousemove', onMouseMove)
                                document.removeEventListener('mouseup', onMouseUp)
                              }

                              document.addEventListener('mousemove', onMouseMove)
                              document.addEventListener('mouseup', onMouseUp)
                            }}
                          />
                          <input
                            type="range"
                            className="dual-range-input dual-range-min"
                            min="1"
                            max="100"
                            value={neonConfig.glow_pulse_min || 30}
                            onChange={(e) => {
                              const val = parseInt(e.target.value)
                              const max = neonConfig.glow_pulse_max || 50
                              setNeonConfig(prev => ({
                                ...prev,
                                glow_pulse_min: Math.max(1, Math.min(val, max - 5))
                              }))
                            }}
                          />
                          <input
                            type="range"
                            className="dual-range-input dual-range-max"
                            min="0"
                            max="100"
                            value={neonConfig.glow_pulse_max || 50}
                            onChange={(e) => {
                              const val = parseInt(e.target.value)
                              const min = neonConfig.glow_pulse_min || 30
                              setNeonConfig(prev => ({
                                ...prev,
                                glow_pulse_max: Math.max(val, min + 5)
                              }))
                            }}
                          />
                        </div>
                        <span className="slider-value">{neonConfig.glow_pulse_min || 30}% - {neonConfig.glow_pulse_max || 50}%</span>
                      </div>
                    </div>
                  </div>

                  {/* Arc section - grouped */}
                  <div className="neon-arc-section">
                    <h4 className="neon-subsection-title">Arc lumineux</h4>

                    <div className="slider-row">
                      <label>Intensite</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0"
                          max="100"
                          value={neonConfig.intensity_gap}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, intensity_gap: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.intensity_gap}%</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Largeur</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="30"
                          max="180"
                          value={neonConfig.arc_width}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, arc_width: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.arc_width}°</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Epaisseur</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0"
                          max="200"
                          step="10"
                          value={neonConfig.arc_blur !== undefined ? neonConfig.arc_blur : 100}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, arc_blur: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.arc_blur !== undefined ? neonConfig.arc_blur : 100}%</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Vitesse</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="1"
                          max="10"
                          step="0.5"
                          value={neonConfig.rotation_speed}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, rotation_speed: parseFloat(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.rotation_speed}s</span>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveNeonConfig} loading={savingNeon}>
                  Enregistrer
                </Button>
              </div>
            </div>

          </Card>
        </section>
      </div>

      {showUSBModal && (
        <USBConfigModal
          onClose={() => setShowUSBModal(false)}
          wifiConfig={{ ssid: wifiSsid, password: wifiPassword, serverIp: wifiServerIp, serverPort: wifiServerPort }}
        />
      )}

      {wifiToast && (
        <div className={`wifi-toast wifi-toast-${wifiToast.type}`}>
          {wifiToast.message}
        </div>
      )}

      {firmwareToast && (
        <div className={`wifi-toast wifi-toast-${firmwareToast.type}`}>
          {firmwareToast.message}
        </div>
      )}

      {showOtaAllModal && (
        <OtaAllModal
          bumpers={bumpers}
          onClose={() => setShowOtaAllModal(false)}
        />
      )}
    </div>
  )
}
