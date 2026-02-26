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
      // Writing from 0x0 as a single operation mirrors the WLED web installer
      // approach and is the correct way to fully re-flash an ESP32-C3.
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
      // is the correct approach for a full re-flash (bootloader + partitions +
      // otadata + app). This is what WLED web installer does.
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

      // Verify the app partition was actually written by reading back the first
      // 8 bytes of app0 and comparing them byte-for-byte with what we wrote.
      // The stub is still running after writeFlash so we can call readFlash.
      // Unlike checking only the magic byte (0xE9 is the same for any firmware),
      // this comparison catches the case where the old image is still present.
      try {
        const readBack = await loader.readFlash(0x10000, 8)
        const expected = uint8.slice(0x10000, 0x10008)
        const match = readBack.every((b, i) => b === expected[i])
        if (match) {
          addFlashLog('✓ Flash vérifié — app0 identique au fichier source')
        } else {
          addFlashLog('⚠ Flash suspect — contenu app0 différent du fichier source')
          addFlashLog('  Lu:      ' + Array.from(readBack).map(b => b.toString(16).padStart(2, '0')).join(' '))
          addFlashLog('  Attendu: ' + Array.from(expected).map(b => b.toString(16).padStart(2, '0')).join(' '))
        }
      } catch (verifyErr) {
        addFlashLog('Vérification flash: ' + verifyErr.message)
      }

      // Tell the stub to leave flash mode and reboot into user code.
      // usingUsbOtg=true → UsbJtagSerialReset for ESP32-C3 native USB.
      addFlashLog('Redémarrage du buzzer...')
      try {
        await loader.after('hard_reset', true)
      } catch (_) {}
      try {
        await transport.disconnect()
      } catch (_) {}
      addFlashLog('Flash terminé.')
      return true
    } catch (err) {
      addFlashLog('Erreur: ' + err.message)
      return false
    } finally {
      setFlashing(false)
    }
  }, [addFlashLog])

  return { flashing, flashProgress, flashLogs, pushFlashLog: addFlashLog, flashFirmware, clearFlashLogs }
}
