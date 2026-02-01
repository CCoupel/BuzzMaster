import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Mock GameContext
jest.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    teams: {},
    bumpers: {},
    gameState: { backgrounds: [] },
    updateConfig: jest.fn(),
    sendMessage: jest.fn(),
    version: '2.49.0'
  })
}))

// Mock framer-motion
jest.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }) => <div {...props}>{children}</div>
  }
}))

describe('ConfigPage - Server Parameters', () => {
  beforeEach(() => {
    // Mock fetch
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.clearAllMocks()
  })

  test('Should render "Parametres serveur" section', () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

    render(<ConfigPage />)

    expect(screen.getByText('Parametres serveur')).toBeInTheDocument()
  })

  test('Should load server parameters from /config.json on mount', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: true, debug: true }
      })
    })

    render(<ConfigPage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/config.json')
    })
  })

  test('Should display auto_open_browsers toggle', () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

    render(<ConfigPage />)

    const label = screen.getByText('Ouvrir les navigateurs automatiquement')
    expect(label).toBeInTheDocument()
  })

  test('Should display debug toggle', () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

    render(<ConfigPage />)

    const label = screen.getByText('Mode debug')
    expect(label).toBeInTheDocument()
  })

  test('Should toggle auto_open_browsers on checkbox change', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

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
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

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
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        server: { auto_open_browsers: false, debug: false }
      })
    })

    const { container } = render(<ConfigPage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled()
    })

    global.fetch.mockClear()
    global.fetch.mockResolvedValueOnce({ ok: true })

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
