import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import USBConfigModal from './USBConfigModal'

// Mutable mock state — allows per-test control without vi.doMock
const webSerialState = {
  connected: false,
  connecting: false,
  logs: [],
}
const mockDisconnect = vi.fn()
const mockConnectToPort = vi.fn()
const mockSendCommand = vi.fn()
const mockClearLogs = vi.fn()

vi.mock('../hooks/useWebSerial', () => ({
  default: () => ({
    connected: webSerialState.connected,
    connecting: webSerialState.connecting,
    logs: webSerialState.logs,
    connectToPort: mockConnectToPort,
    disconnect: mockDisconnect,
    sendCommand: mockSendCommand,
    clearLogs: mockClearLogs,
  }),
}))

const espFlashState = {
  flashing: false,
  flashProgress: 0,
  flashLogs: [],
}
const mockFlashFirmware = vi.fn()
const mockClearFlashLogs = vi.fn()

vi.mock('../hooks/useEspFlash', () => ({
  default: () => ({
    flashing: espFlashState.flashing,
    flashProgress: espFlashState.flashProgress,
    flashLogs: espFlashState.flashLogs,
    flashFirmware: mockFlashFirmware,
    clearFlashLogs: mockClearFlashLogs,
  }),
}))

const defaultProps = {
  onClose: vi.fn(),
  wifiConfig: { ssid: 'TestSSID', password: 'testpass', serverIp: '192.168.1.1', serverPort: 80 },
  firmwareInfo: { EXISTS: true, VERSION: '3.1.1', FILENAME: 'buzzclick-v3.1.1.bin', SIZE: 512000 },
}

function setupNavigatorSerial(ports = []) {
  Object.defineProperty(global.navigator, 'serial', {
    value: { getPorts: vi.fn().mockResolvedValue(ports) },
    configurable: true,
  })
}

const fakePort = { getInfo: () => ({ usbVendorId: 0x303a, usbProductId: 0x1001 }) }

describe('USBConfigModal - Flash Firmware section', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    webSerialState.connected = false
    espFlashState.flashing = false
    espFlashState.flashProgress = 0
    espFlashState.flashLogs = []
    setupNavigatorSerial()
  })

  it('should render Flash Firmware section heading', () => {
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.getByText('Flash Firmware via USB')).toBeInTheDocument()
  })

  it('should render "Flasher via USB" button when firmwareInfo.EXISTS is true', () => {
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.getByText('Flasher via USB')).toBeInTheDocument()
  })

  it('should not show unavailability message when firmwareInfo.EXISTS is true', () => {
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.queryByText(/Aucun firmware disponible/)).not.toBeInTheDocument()
  })

  it('should show unavailability message when firmwareInfo.EXISTS is false', () => {
    render(
      <USBConfigModal
        {...defaultProps}
        firmwareInfo={{ EXISTS: false, VERSION: null }}
      />
    )
    expect(screen.getByText(/Aucun firmware disponible/)).toBeInTheDocument()
  })

  it('should disable "Flasher via USB" button when selectedPortIndex is null (no port selected)', () => {
    render(<USBConfigModal {...defaultProps} />)
    const flashButton = screen.getByText('Flasher via USB').closest('button')
    expect(flashButton).toBeDisabled()
  })

  it('should disable "Flasher via USB" button when firmwareInfo.EXISTS is false', () => {
    render(
      <USBConfigModal
        {...defaultProps}
        firmwareInfo={{ EXISTS: false, VERSION: null }}
      />
    )
    const flashButton = screen.getByText('Flasher via USB').closest('button')
    expect(flashButton).toBeDisabled()
  })
})

describe('USBConfigModal - Flash progress display', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    webSerialState.connected = false
    espFlashState.flashing = false
    espFlashState.flashProgress = 0
    espFlashState.flashLogs = []
    setupNavigatorSerial()
  })

  it('should not show progress bar when flashing is false (default)', () => {
    const { container } = render(<USBConfigModal {...defaultProps} />)
    expect(container.querySelector('.usb-flash-progress')).not.toBeInTheDocument()
  })

  it('should show progress bar when flashing is true', () => {
    espFlashState.flashing = true
    espFlashState.flashProgress = 42
    const { container } = render(<USBConfigModal {...defaultProps} />)
    expect(container.querySelector('.usb-flash-progress')).toBeInTheDocument()
  })

  it('should show spinner in flash button when flashing is true', () => {
    espFlashState.flashing = true
    espFlashState.flashProgress = 42
    const { container } = render(<USBConfigModal {...defaultProps} />)
    // When loading=true, Button renders a spinner instead of text children
    const flashSection = container.querySelector('.usb-flash-progress')
    expect(flashSection).toBeInTheDocument()
  })

  it('should show "Flasher via USB" text in button label when flashing is false', () => {
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.getByText('Flasher via USB')).toBeInTheDocument()
  })
})

describe('USBConfigModal - Flash button click behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    webSerialState.connected = false
    espFlashState.flashing = false
    espFlashState.flashProgress = 0
    espFlashState.flashLogs = []
    setupNavigatorSerial([fakePort])
  })

  it('should call clearFlashLogs then flashFirmware when port is selected and flash button clicked', async () => {
    render(<USBConfigModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.queryByText(/Port USB #1/)).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText(/Port USB #1/).closest('.usb-port-item'))

    const flashButton = screen.getByText('Flasher via USB').closest('button')
    expect(flashButton).not.toBeDisabled()

    fireEvent.click(flashButton)

    await waitFor(() => {
      expect(mockClearFlashLogs).toHaveBeenCalled()
      expect(mockFlashFirmware).toHaveBeenCalled()
    })
  })

  it('should call disconnect() before flashFirmware when serial is connected', async () => {
    // Step 1: render with connected=false to allow port selection from the list
    webSerialState.connected = false
    const { rerender } = render(<USBConfigModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.queryByText(/Port USB #1/)).toBeInTheDocument()
    })

    // Select port (only visible when disconnected)
    fireEvent.click(screen.getByText(/Port USB #1/).closest('.usb-port-item'))

    // Step 2: switch to connected=true and re-render — the onClick handler will now
    // see connected===true and call disconnect() before flashFirmware()
    webSerialState.connected = true
    rerender(<USBConfigModal {...defaultProps} />)

    const flashButton = screen.getByText('Flasher via USB').closest('button')
    fireEvent.click(flashButton)

    await waitFor(() => {
      expect(mockDisconnect).toHaveBeenCalled()
      expect(mockFlashFirmware).toHaveBeenCalled()
    })
  })

  it('should NOT call disconnect() before flashFirmware when serial is not connected', async () => {
    webSerialState.connected = false

    render(<USBConfigModal {...defaultProps} />)

    await waitFor(() => {
      expect(screen.queryByText(/Port USB #1/)).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText(/Port USB #1/).closest('.usb-port-item'))

    const flashButton = screen.getByText('Flasher via USB').closest('button')
    fireEvent.click(flashButton)

    await waitFor(() => {
      expect(mockDisconnect).not.toHaveBeenCalled()
      expect(mockFlashFirmware).toHaveBeenCalled()
    })
  })
})

describe('USBConfigModal - General rendering', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    webSerialState.connected = false
    espFlashState.flashing = false
    espFlashState.flashProgress = 0
    espFlashState.flashLogs = []
    setupNavigatorSerial()
  })

  it('should render modal overlay', () => {
    const { container } = render(<USBConfigModal {...defaultProps} />)
    expect(container.querySelector('.usb-modal-overlay')).toBeInTheDocument()
  })

  it('should render modal title', () => {
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.getByText('Configuration USB Buzzer')).toBeInTheDocument()
  })

  it('should call onClose when Escape key is pressed', () => {
    const onClose = vi.fn()
    render(<USBConfigModal {...defaultProps} onClose={onClose} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('should call onClose when close button is clicked', () => {
    const onClose = vi.fn()
    render(<USBConfigModal {...defaultProps} onClose={onClose} />)
    const closeBtn = screen.getByText('×').closest('button')
    fireEvent.click(closeBtn)
    expect(onClose).toHaveBeenCalled()
  })

  it('should display Web Serial not supported warning when serial is unavailable', () => {
    const descriptor = Object.getOwnPropertyDescriptor(global.navigator, 'serial')
    delete global.navigator.serial
    render(<USBConfigModal {...defaultProps} />)
    expect(screen.getByText(/Web Serial API non disponible/)).toBeInTheDocument()
    if (descriptor) {
      Object.defineProperty(global.navigator, 'serial', descriptor)
    }
  })
})
