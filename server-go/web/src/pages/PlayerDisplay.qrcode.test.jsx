import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — PlayerDisplay imports (pattern identique aux tests existants)
// framer-motion est auto-mocké via l'alias vite.config.js
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div>,
}))

vi.mock('../components/Podium', () => ({
  default: () => <div data-testid="podium" />,
}))

vi.mock('../components/QRCodeOverlay', () => ({
  default: () => null,
}))

// QRCodeDisplay mock — expose l'URL comme attribut data pour les assertions
vi.mock('../components/QRCodeDisplay', () => ({
  default: ({ url }) => <div data-testid="qr-display" data-url={url} />,
}))

vi.mock('./QuestionsPage', () => ({
  CATEGORIES: [],
}))

vi.mock('../constants/colors', () => ({
  getCategoryColor: vi.fn(() => '#8b5cf6'),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))

vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

// ---------------------------------------------------------------------------
// Import après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Données de test
// ---------------------------------------------------------------------------
const BASE_GAME_STATE = {
  phase: 'ENROLL',
  remote: null,
  question: null,
  timer: 0,
  totalTime: 0,
  virtualPlayerCount: 3,
  virtualPlayerLimit: 20,
  backgrounds: [],
  currentBackgroundIndex: 0,
  memoryMatchedPairs: [],
  neonEffect: { enabled: false },
}

function mockUseGame(overrides = {}) {
  useGame.mockReturnValue({
    gameState: { ...BASE_GAME_STATE, ...overrides },
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: vi.fn(),
    selectMotionCard: vi.fn(),
  })
}

function mockFetchWifi({ ssid = 'TestWifi', password = 'testpass' } = {}) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ ssid, password }),
  })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('PlayerDisplay — QR codes ENROLL (non-régression #85)', () => {

  beforeEach(() => {
    mockUseGame()
    mockFetchWifi()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  // -------------------------------------------------------------------------
  // a) URL du QR "Rejoindre le jeu"
  // -------------------------------------------------------------------------
  describe('QR "Rejoindre le jeu" — URL', () => {

    it('affiche deux QR codes en phase ENROLL (TV)', async () => {
      render(<PlayerDisplay />)
      await waitFor(() => {
        expect(screen.getAllByTestId('qr-display')).toHaveLength(2)
      })
    })

    it("l'URL du QR jeu NE contient PAS '/player' (régression #85)", async () => {
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        // index 0 = WiFi, index 1 = Jeu
        const gameQrUrl = qrElements[1].getAttribute('data-url')
        expect(gameQrUrl).not.toContain('/player')
      })
    })

    it("l'URL du QR jeu se termine par '/' (racine, pas /player)", async () => {
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const gameQrUrl = qrElements[1].getAttribute('data-url')
        // Format attendu après fix : http://<host>/
        expect(gameQrUrl).toMatch(/http:\/\/.+\/$/)
      })
    })

    it("l'URL du QR jeu inclut le protocole http:// et le host", async () => {
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const gameQrUrl = qrElements[1].getAttribute('data-url')
        expect(gameQrUrl).toMatch(/^http:\/\//)
      })
    })

    it('ne rend PAS de QR code ENROLL si isVPlayer=true', () => {
      render(<PlayerDisplay isVPlayer={true} />)
      // La phase ENROLL ne s'affiche pas pour les VPlayers
      expect(screen.queryAllByTestId('qr-display')).toHaveLength(0)
    })

  })

  // -------------------------------------------------------------------------
  // b) QR WiFi — format et escaping
  // -------------------------------------------------------------------------
  describe('QR WiFi — format de l\'URL', () => {

    it("forme l'URL WiFi au format WIFI:T:WPA;S:...;P:...; avec SSID + mot de passe", async () => {
      mockFetchWifi({ ssid: 'TestWifi', password: 'testpass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toMatch(/^WIFI:T:WPA;S:/)
        expect(wifiUrl).toContain('TestWifi')
        expect(wifiUrl).toContain('testpass')
        expect(wifiUrl).toMatch(/;;$/)
      })
    })

    it("utilise 'nopass' si le mot de passe est vide", async () => {
      mockFetchWifi({ ssid: 'OpenWifi', password: '' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toContain('T:nopass')
        expect(wifiUrl).toContain('S:OpenWifi')
      })
    })

    it("utilise l'URL de fallback si aucun SSID configuré", async () => {
      mockFetchWifi({ ssid: '', password: '' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toBe('https://buzzcontrol.local/no-wifi-configured')
      })
    })

    it("affiche l'URL de fallback initiale pendant le chargement (avant fetch)", () => {
      // fetch non résolu = wifiConfig null = fallback URL visible immédiatement
      global.fetch = vi.fn().mockReturnValue(new Promise(() => {})) // never resolves
      render(<PlayerDisplay />)
      const qrElements = screen.getAllByTestId('qr-display')
      const wifiUrl = qrElements[0].getAttribute('data-url')
      expect(wifiUrl).toBe('https://buzzcontrol.local/no-wifi-configured')
    })

    it("échappe le ';' dans le SSID (régression escaping WiFi QR)", async () => {
      mockFetchWifi({ ssid: 'My;Wifi', password: 'pass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        // Le SSID 'My;Wifi' doit être échappé en 'My\;Wifi' dans la chaîne WiFi
        expect(wifiUrl).toContain('My\\;Wifi')
        // Vérifier qu'il n'y a pas de ';' brut dans le champ SSID
        expect(wifiUrl).not.toMatch(/S:My;Wifi/)
      })
    })

    it("échappe le '\"' dans le SSID", async () => {
      mockFetchWifi({ ssid: 'My"Wifi', password: 'pass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toContain('My\\"Wifi')
      })
    })

    it("échappe la virgule ',' dans le SSID", async () => {
      mockFetchWifi({ ssid: 'My,Wifi', password: 'pass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toContain('My\\,Wifi')
      })
    })

    it("échappe le '\\' dans le SSID", async () => {
      mockFetchWifi({ ssid: 'My\\Wifi', password: 'pass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        // '\\' doit devenir '\\\\'
        expect(wifiUrl).toContain('My\\\\Wifi')
      })
    })

    it("échappe les caractères spéciaux dans le mot de passe", async () => {
      mockFetchWifi({ ssid: 'TestWifi', password: 'pass;word' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toContain('pass\\;word')
      })
    })

    it("n'altère pas un SSID sans caractères spéciaux", async () => {
      mockFetchWifi({ ssid: 'SimpleWifi123', password: 'securepass' })
      render(<PlayerDisplay />)
      await waitFor(() => {
        const qrElements = screen.getAllByTestId('qr-display')
        const wifiUrl = qrElements[0].getAttribute('data-url')
        expect(wifiUrl).toContain('S:SimpleWifi123')
        expect(wifiUrl).toContain('P:securepass')
      })
    })

  })

})
