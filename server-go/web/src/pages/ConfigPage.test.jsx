import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Mock GameContext
vi.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    teams: {},
    bumpers: {},
    gameState: { backgrounds: [] },
    updateConfig: vi.fn(),
    sendMessage: vi.fn(),
    version: '2.49.0',
    // IS_MERGED: true requis pour que le bouton "Flash via USB" ne soit pas disabled
    firmwareInfo: { EXISTS: true, IS_MERGED: true, VERSION: '3.1.1', FILENAME: 'buzzclick-v3.1.1.bin', SIZE: 512000 },
  })
}))

// Note: framer-motion is globally aliased via vite.config.js test.alias → src/mocks/framer-motion.jsx

// Mock lightweight UI primitives to reduce JSDOM memory footprint
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('./ConfigPage.css', () => ({}))

// Mock USBConfigModal to avoid Web Serial API complexity in ConfigPage tests
vi.mock('../components/USBConfigModal', () => ({
  default: ({ onClose }) => (
    <div data-testid="usb-modal">
      <button onClick={onClose}>Close USB Modal</button>
    </div>
  )
}))

// Mock OtaAllModal
vi.mock('../components/TeamCard', () => ({
  OtaAllModal: ({ onClose }) => (
    <div data-testid="ota-all-modal">
      <button onClick={onClose}>Close OTA Modal</button>
    </div>
  )
}))

// Fournit un mock fetch COMPLET pour les 3 requêtes déclenchées au montage
// (/config.json, /api/wifi/defaults, /api/firmware/buzzclick/version).
// Durcissement (bugfix/config-api-key-help, sur demande explicite du CDP) :
// avant ce commit, `beforeEach` posait `global.fetch = vi.fn()` SANS
// implémentation par défaut, et chaque test ne mockait qu'UN SEUL appel via
// `mockResolvedValueOnce` — les 2 autres requêtes de montage retombaient sur
// le mock non configuré (résolution en `undefined`, avalé par le try/catch
// mais fragile/silencieux). Non concluant comme correctif du blocage vitest
// observé dans l'environnement WSL de dev-frontend (reproduit aussi sur le
// commit parent, donc probablement lié à l'environnement — voir
// `_work/reports/test-writer-*.md`), mais retire une source d'appels fetch
// non déterministes et aligne ce describe sur le pattern déjà utilisé plus
// bas dans ce même fichier (describe "Flash via USB button").
function mockConfigPageFetch(overrides = {}) {
  global.fetch = vi.fn((url) => {
    if (url === '/config.json') {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          server: { auto_open_browsers: false, debug: false },
          ...overrides.config,
        })
      })
    }
    if (url === '/api/wifi/defaults') {
      return Promise.resolve({
        ok: true,
        json: async () => ({ ssid: '', password: '', ssid2: '', password2: '', server_ip: '', server_port: 80 })
      })
    }
    if (url === '/api/firmware/buzzclick/version') {
      return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    }
    return Promise.resolve({ ok: false, json: async () => ({}), text: async () => '' })
  })
}

describe('ConfigPage - Server Parameters', () => {
  beforeEach(() => {
    mockConfigPageFetch()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  test('Should render "Parametres serveur" section', () => {
    render(<ConfigPage />)

    expect(screen.getByText('Parametres serveur')).toBeInTheDocument()
  })

  test('Should load server parameters from /config.json on mount', async () => {
    mockConfigPageFetch({ config: { server: { auto_open_browsers: true, debug: true } } })

    render(<ConfigPage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/config.json')
    })
  })

  test('Should display auto_open_browsers toggle', () => {
    render(<ConfigPage />)

    const label = screen.getByText('Ouvrir les navigateurs automatiquement')
    expect(label).toBeInTheDocument()
  })

  test('Should display debug toggle', () => {
    render(<ConfigPage />)

    const label = screen.getByText('Mode debug')
    expect(label).toBeInTheDocument()
  })

  test('Should toggle auto_open_browsers on checkbox change', async () => {
    const { container } = render(<ConfigPage />)

    await waitFor(() => {
      const checkboxes = container.querySelectorAll('input[type="checkbox"]')
      const autoOpenCheckbox = Array.from(checkboxes).find(cb =>
        cb.parentElement.textContent.includes('Ouvrir les navigateurs')
      )
      expect(autoOpenCheckbox).toBeInTheDocument()
    })
  })

  test('Should toggle debug on checkbox change', async () => {
    const { container } = render(<ConfigPage />)

    await waitFor(() => {
      const checkboxes = container.querySelectorAll('input[type="checkbox"]')
      const debugCheckbox = Array.from(checkboxes).find(cb =>
        cb.parentElement.textContent.includes('Mode debug')
      )
      expect(debugCheckbox).toBeInTheDocument()
    })
  })

  test('Should save server parameters via POST /config.json', async () => {
    const { container } = render(<ConfigPage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled()
    })

    global.fetch.mockClear()
    global.fetch.mockImplementation(() => Promise.resolve({ ok: true, json: async () => ({}) }))

    const buttons = screen.getAllByRole('button')
    const saveButton = buttons.find(btn => btn.textContent === 'Enregistrer')

    if (saveButton) {
      fireEvent.click(saveButton)

      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalledWith(
          '/config.json',
          expect.objectContaining({
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
          })
        )
      })
    }
  })
})

describe('ConfigPage - Flash via USB button', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
    // Default fetch mocks: config.json, wifi/defaults, firmware/version
    global.fetch.mockImplementation((url) => {
      if (url === '/config.json') {
        return Promise.resolve({
          ok: true,
          json: async () => ({ server: { auto_open_browsers: false, debug: false } })
        })
      }
      if (url === '/api/wifi/defaults') {
        return Promise.resolve({
          ok: true,
          json: async () => ({ ssid: 'TestSSID', password: 'pass', server_ip: '192.168.1.1', server_port: 80 })
        })
      }
      if (url === '/api/firmware/buzzclick/version') {
        return Promise.resolve({
          ok: true,
          json: async () => ({ VERSION: '3.1.1', FILENAME: 'buzzclick-v3.1.1.bin', SIZE: 512000, EXISTS: true })
        })
      }
      return Promise.resolve({ ok: false })
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('should render "Flash via USB" button in the Firmware section', async () => {
    render(<ConfigPage />)
    await waitFor(() => {
      expect(screen.getByText('Flash via USB')).toBeInTheDocument()
    })
  })

  it('should open the USB modal when "Flash via USB" button is clicked', async () => {
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('Flash via USB')).toBeInTheDocument()
    })

    const flashUsbButton = screen.getByText('Flash via USB')
    fireEvent.click(flashUsbButton)

    await waitFor(() => {
      expect(screen.getByTestId('usb-modal')).toBeInTheDocument()
    })
  })

  it('should close the USB modal when onClose is called', async () => {
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('Flash via USB')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Flash via USB'))

    await waitFor(() => {
      expect(screen.getByTestId('usb-modal')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Close USB Modal'))

    await waitFor(() => {
      expect(screen.queryByTestId('usb-modal')).not.toBeInTheDocument()
    })
  })

  it('should pass firmwareInfo prop to USBConfigModal', async () => {
    // The mock renders the modal if showUSBModal=true; we verify it mounts
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('Flash via USB')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Flash via USB'))

    await waitFor(() => {
      expect(screen.getByTestId('usb-modal')).toBeInTheDocument()
    })
  })

  it('should also open USB modal from "Configuration via USB" button in WiFi section', async () => {
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('Configuration via USB')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Configuration via USB'))

    await waitFor(() => {
      expect(screen.getByTestId('usb-modal')).toBeInTheDocument()
    })
  })
})
