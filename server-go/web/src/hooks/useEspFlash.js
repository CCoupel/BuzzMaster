import { useState, useCallback } from 'react'
import { ESPLoader, Transport } from 'esptool-js'

export default function useEspFlash() {
  const [flashing, setFlashing] = useState(false)
  const [flashProgress, setFlashProgress] = useState(0)
  const [flashLogs, setFlashLogs] = useState([])

  const addFlashLog = useCallback((text) => {
    setFlashLogs(prev => [...prev, String(text)])
  }, [])

  const clearFlashLogs = useCallback(() => {
    setFlashLogs([])
    setFlashProgress(0)
  }, [])

  // port: raw SerialPort object (from useWebSerial.getPort())
  const flashFirmware = useCallback(async (port) => {
    setFlashing(true)
    setFlashProgress(0)
    setFlashLogs([])

    try {
      // Fetch merged firmware binary from server.
      // The merged binary contains: bootloader (0x0) + partitions (0x8000)
      // + boot_app0/otadata (0xE000) + app firmware (0x10000).
      // Writing from 0x0 as a single operation avoids the silent write failures
      // that occur when esptool-js starts a write directly at an app partition
      // address (0x10000 or 0x150000). This mirrors the WLED web installer approach.
      addFlashLog('Téléchargement du firmware depuis le serveur...')
      const response = await fetch('/api/firmware/buzzclick/merged.bin')
      if (!response.ok) {
        if (response.status === 404) {
          throw new Error('Flash USB non disponible : le serveur ne contient pas le firmware complet. Utilisez une release CI.')
        }
        throw new Error('Firmware non disponible sur le serveur')
      }

      const arrayBuffer = await response.arrayBuffer()
      const uint8 = new Uint8Array(arrayBuffer)

      // esptool-js requires a binary string (not ArrayBuffer)
      let binaryString = ''
      for (let i = 0; i < uint8.length; i++) {
        binaryString += String.fromCharCode(uint8[i])
      }
      addFlashLog(`Firmware: ${(uint8.length / 1024).toFixed(1)} KB`)

      const terminal = {
        clean() {},
        writeLine(data) { addFlashLog(data) },
        write(data) { addFlashLog(data) },
      }

      const transport = new Transport(port, true)
      const loader = new ESPLoader({
        transport,
        baudrate: 460800,
        romBaudrate: 115200,
        terminal,
      })

      addFlashLog('Connexion au bootloader ROM...')
      const chipName = await loader.main()
      addFlashLog(`Puce: ${chipName}`)

      // Flash the merged binary at address 0x0.
      // A single contiguous write from 0x0 through the end of the app partition
      // is reliably handled by the esptool-js stub, unlike writes that start
      // directly at app partition addresses (0x10000 / 0x150000).
      addFlashLog('Flash en cours (0x0)...')
      await loader.writeFlash({
        fileArray: [{ data: binaryString, address: 0x0 }],
        flashSize: 'keep',
        flashMode: 'keep',
        flashFreq: 'keep',
        eraseAll: false,
        compress: true,
        reportProgress(_fileIndex, written, total) {
          setFlashProgress(Math.round((written / total) * 100))
        },
      })

      // Tell the stub to leave flash mode and reboot into user code.
      // usingUsbOtg=true → UsbJtagSerialReset (CDC break) for ESP32-C3 native USB.
      addFlashLog('Redémarrage du buzzer...')
      try {
        await loader.after('hard_reset', true)
      } catch (_) {}
      try {
        await transport.disconnect()
      } catch (_) {}
      addFlashLog('Flash terminé ! Le buzzer redémarre.')
      return true
    } catch (err) {
      addFlashLog('Erreur: ' + err.message)
      return false
    } finally {
      setFlashing(false)
    }
  }, [addFlashLog])

  return { flashing, flashProgress, flashLogs, flashFirmware, clearFlashLogs }
}
