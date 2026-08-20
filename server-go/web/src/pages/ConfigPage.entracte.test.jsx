import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// ---------------------------------------------------------------------------
// ConfigPage — section Entracte (#119, F8/T6 du plan
// _work/reports/plan-entracte-119-20260820-140825.md).
//
// Contrat : contracts/http-endpoints.md §"Mode ENTRACTE" — section `entracte`
// de POST /game-config.json en snake_case (title/subtitle/panel_size/
// anim_period/anim_intensity) + GET/POST/DELETE /api/config/entracte-image.
// contracts/game-state.md §ENTRACTE_CONFIG — miroir WebSocket en
// UPPER_SNAKE (TITLE/SUBTITLE/IMAGE_IS_CUSTOM/PANEL_SIZE/ANIM_PERIOD/
// ANIM_INTENSITY), reçu via GameState.ENTRACTE_CONFIG (useGame().gameState).
//
// ⚠️ CASSE DIVERGENTE OBSERVÉE (à faire confirmer par code-reviewer) :
// ConfigPage.jsx tient `entracteConfig` en UPPER_SNAKE (mêmes clés que le
// state initial useWebSocket.js) MAIS `fetchGameConfig` (l.258-280) fait
// `setEntracteConfig(data.entracte)` avec le corps BRUT de
// `GET /game-config.json`, qui est en snake_case côté serveur (confirmé en
// lisant internal/config/gameconfig.go — EntracteConfig{Title, Subtitle,
// PanelSize, AnimPeriod, AnimIntensity} avec tags json snake_case). Le
// premier montage écrase donc TITLE/PANEL_SIZE/... par des clés
// title/panel_size/... que le JSX ne lit jamais (entracteConfig.PANEL_SIZE
// devient undefined) — panneau vide après le premier chargement réseau.
// `handleSaveEntracteConfig` a le même problème en sens inverse : il POSTe
// `entracteConfig` (UPPER_SNAKE) tel quel, alors que le contrat documente
// un corps snake_case (encoding/json accepte la casse en lecture côté Go,
// donc ça "marche" au runtime, mais ne correspond pas au corps documenté).
//
// Ce fichier teste donc SÉPARÉMENT :
//   - le chemin normal (WebSocket, GameState.ENTRACTE_CONFIG déjà en
//     UPPER_SNAKE — la vraie source vivante en production, cf. useEffect
//     l.365-374) pour les tests d'édition/sauvegarde/upload d'image ;
//   - le chemin GET /game-config.json isolément, en respectant le format
//     serveur réel (snake_case) — ce test documente le comportement attendu
//     par le contrat et peut légitimement échouer tant que la divergence
//     ci-dessus n'est pas corrigée.
//
// Convention (comme ConfigPage.ai.test.jsx) : ConfigPage rend PLUSIEURS
// boutons "Enregistrer" — chaque test scope ses requêtes avec `within()` à
// partir de la section Entracte, repérée par son titre.
//
// Les 3 curseurs (PANEL_SIZE 20-100, ANIM_PERIOD 2-30, ANIM_INTENSITY 0-100)
// sont repérés par leurs bornes min/max plutôt que par le texte de leur
// label — la forme exacte du label n'est pas fixée par le contrat, ses
// bornes le sont.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/USBConfigModal', () => ({
  default: () => null,
}))

vi.mock('../components/TeamCard', () => ({
  OtaAllModal: () => null,
}))

import { useGame } from '../hooks/GameContext'

const makeConfigPageMock = (overrides = {}) => ({
  teams: {},
  bumpers: {},
  gameState: { backgrounds: [] },
  updateConfig: vi.fn(),
  sendMessage: vi.fn(),
  version: '6.5.2',
  firmwareInfo: { EXISTS: false },
  ...overrides,
})

// Default GET /game-config.json mock: NO "entracte" key at all — the
// `if (data.entracte)` guard (ConfigPage.jsx:270) then skips
// setEntracteConfig entirely, leaving the component's own correctly-shaped
// (UPPER_SNAKE) useState default intact. This deliberately keeps tests that
// aren't about the load path itself decoupled from the casing bug
// documented above.
function mockFetchImplementation({ withEntracteSection = false, postOk = true } = {}) {
  global.fetch = vi.fn((url, options) => {
    if (url === '/config.json' && (!options || options.method === undefined)) {
      return Promise.resolve({ ok: true, json: async () => ({ server: {}, ai: {} }) })
    }
    if (url === '/game-config.json' && (!options || options.method === undefined)) {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          neon_effect: { enabled: false },
          ...(withEntracteSection ? {
            entracte: { title: 'ENTRACTE', subtitle: 'Retour dans 20mn', panel_size: 65, anim_period: 10, anim_intensity: 20 },
          } : {}),
        }),
      })
    }
    if (url === '/game-config.json' && options?.method === 'POST') {
      if (postOk) return Promise.resolve({ ok: true, json: async () => ({}) })
      return Promise.resolve({ ok: false, text: async () => 'Erreur serveur' })
    }
    if (url === '/api/config/entracte-image' && options?.method === 'POST') {
      return Promise.resolve({ ok: true, json: async () => ({ image_is_custom: true }) })
    }
    if (url === '/api/config/entracte-image' && options?.method === 'DELETE') {
      return Promise.resolve({ ok: true, json: async () => ({ image_is_custom: false }) })
    }
    if (url === '/api/wifi/defaults') {
      return Promise.resolve({ ok: true, json: async () => ({ ssid: '', password: '', server_ip: '', server_port: 80 }) })
    }
    if (url === '/api/firmware/buzzclick/version') {
      return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    }
    return Promise.resolve({ ok: false })
  })
}

function getEntracteSection() {
  const heading = screen.getByText(/entracte/i, { selector: '.config-section-title, h3' })
  return heading.closest('.config-section')
}

function getRangeByBounds(section, min, max) {
  return Array.from(section.querySelectorAll('input[type="range"]'))
    .find(el => el.getAttribute('min') === String(min) && el.getAttribute('max') === String(max))
}

async function renderAndGetSection() {
  render(<ConfigPage />)
  await waitFor(() => expect(screen.getByText(/entracte/i, { selector: '.config-section-title, h3' })).toBeInTheDocument())
  return getEntracteSection()
}

beforeEach(() => {
  useGame.mockReturnValue(makeConfigPageMock())
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('ConfigPage — section Entracte : défauts contractuels (T6)', () => {
  it('affiche le titre de section "Entracte" et les défauts (TITLE/PANEL_SIZE) sans aucune source externe', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()

    expect(within(section).getByDisplayValue('ENTRACTE')).toBeInTheDocument()
    expect(within(section).getByDisplayValue('Retour dans 20mn')).toBeInTheDocument()
    expect(getRangeByBounds(section, 20, 100).value).toBe('65')
    expect(getRangeByBounds(section, 2, 30).value).toBe('10')
    expect(getRangeByBounds(section, 0, 100).value).toBe('20')
  })
})

describe('ConfigPage — section Entracte : chargement GET /game-config.json (T6)', () => {
  // Ce test respecte le format contractuel réel (snake_case,
  // contracts/http-endpoints.md) tel qu'émis par le serveur Go
  // (internal/config/gameconfig.go, EntracteConfig). S'il échoue, c'est
  // très probablement la divergence de casse documentée en tête de fichier
  // (ConfigPage.jsx:270-272 assigne data.entracte tel quel à un state que
  // le JSX lit en UPPER_SNAKE) — à signaler à code-reviewer/dev-frontend,
  // pas un test à "corriger" pour le faire passer artificiellement.
  it('charge title/panel_size/... depuis la réponse snake_case réelle du serveur', async () => {
    mockFetchImplementation({ withEntracteSection: true })
    const section = await renderAndGetSection()

    await waitFor(() => {
      expect(within(section).getByDisplayValue('ENTRACTE')).toBeInTheDocument()
    })
    expect(getRangeByBounds(section, 20, 100).value).toBe('65')
  })
})

describe('ConfigPage — section Entracte : synchronisation depuis GameState.ENTRACTE_CONFIG (WebSocket, T6)', () => {
  it('reflète un ENTRACTE_CONFIG déjà en UPPER_SNAKE reçu via useGame() (chemin réel de production, cf. GameProvider)', async () => {
    mockFetchImplementation()
    useGame.mockReturnValue(makeConfigPageMock({
      gameState: {
        backgrounds: [],
        entracteConfig: { TITLE: 'Pause déjeuner', SUBTITLE: 'Retour à 13h30', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 80, ANIM_PERIOD: 6, ANIM_INTENSITY: 40 },
      },
    }))
    const section = await renderAndGetSection()

    await waitFor(() => {
      expect(within(section).getByDisplayValue('Pause déjeuner')).toBeInTheDocument()
    })
    expect(within(section).getByDisplayValue('Retour à 13h30')).toBeInTheDocument()
    expect(getRangeByBounds(section, 20, 100).value).toBe('80')
    expect(getRangeByBounds(section, 2, 30).value).toBe('6')
    expect(getRangeByBounds(section, 0, 100).value).toBe('40')
  })
})

describe('ConfigPage — section Entracte : saisie et sauvegarde (T6)', () => {
  it('la saisie du titre met à jour le champ localement', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()

    const titleInput = within(section).getByDisplayValue('ENTRACTE')
    fireEvent.change(titleInput, { target: { value: 'Pause café' } })
    expect(within(section).getByDisplayValue('Pause café')).toBeInTheDocument()
  })

  it('déplacer le curseur "Taille du panneau" (PANEL_SIZE, 20-100) met à jour la valeur affichée', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()

    const sizeSlider = getRangeByBounds(section, 20, 100)
    fireEvent.change(sizeSlider, { target: { value: '90' } })
    expect(sizeSlider.value).toBe('90')
  })

  it('Enregistrer envoie POST /game-config.json avec une section entracte reflétant les valeurs à l\'écran', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()

    fireEvent.change(within(section).getByDisplayValue('ENTRACTE'), { target: { value: 'Pause déjeuner' } })
    fireEvent.change(getRangeByBounds(section, 20, 100), { target: { value: '80' } })

    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      const postCall = global.fetch.mock.calls.find(([url, opts]) => url === '/game-config.json' && opts?.method === 'POST')
      expect(postCall).toBeDefined()
    })
    const postCall = global.fetch.mock.calls.find(([url, opts]) => url === '/game-config.json' && opts?.method === 'POST')
    const body = JSON.parse(postCall[1].body)
    expect(body.entracte).toBeDefined()
    // Valeurs à jour envoyées, quelle que soit la casse des clés (voir note
    // de casse en tête de fichier — le serveur Go accepte les deux via le
    // fallback insensible à la casse d'encoding/json, mais le corps
    // documenté par le contrat est en snake_case).
    const values = Object.values(body.entracte)
    expect(values).toContain('Pause déjeuner')
    expect(values).toContain(80)
  })

  it('ANIM_INTENSITY = 0 affiche explicitement "animation desactivee" (sans quoi un curseur à zéro se lit comme un bug)', async () => {
    mockFetchImplementation()
    useGame.mockReturnValue(makeConfigPageMock({
      gameState: {
        backgrounds: [],
        entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: 'Retour dans 20mn', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 0 },
      },
    }))
    const section = await renderAndGetSection()

    await waitFor(() => {
      expect(within(section).getByText(/animation d.sactiv.e/i)).toBeInTheDocument()
    })
  })
})

describe('ConfigPage — section Entracte : image de fond (T6)', () => {
  it('upload une image envoie un POST multipart vers /api/config/entracte-image', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()

    const fileInput = section.querySelector('input[type="file"]')
    expect(fileInput).not.toBeNull()
    const file = new File(['fake-image'], 'fond.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    fireEvent.click(within(section).getByText("Enregistrer l'image"))

    await waitFor(() => {
      const uploadCall = global.fetch.mock.calls.find(([url, opts]) => url === '/api/config/entracte-image' && opts?.method === 'POST')
      expect(uploadCall).toBeDefined()
      expect(uploadCall[1].body).toBeInstanceOf(FormData)
    })
  })

  it('le bouton "Retirer l\'image" n\'apparaît que si une image personnalisée existe', async () => {
    mockFetchImplementation()
    const section = await renderAndGetSection()
    expect(within(section).queryByText("Retirer l'image")).toBeNull()
  })

  it('supprimer une image personnalisée envoie DELETE /api/config/entracte-image', async () => {
    mockFetchImplementation()
    useGame.mockReturnValue(makeConfigPageMock({
      gameState: {
        backgrounds: [],
        entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: 'Retour dans 20mn', IMAGE_IS_CUSTOM: true, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      },
    }))
    const section = await renderAndGetSection()

    // handleEntracteImageDelete demande confirmation (window.confirm) avant
    // d'appeler DELETE (ConfigPage.jsx:921) — accepté ici pour exercer le
    // chemin complet.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const deleteBtn = await waitFor(() => within(section).getByText("Retirer l'image"))
    fireEvent.click(deleteBtn)

    await waitFor(() => {
      const deleteCall = global.fetch.mock.calls.find(([url, opts]) => url === '/api/config/entracte-image' && opts?.method === 'DELETE')
      expect(deleteCall).toBeDefined()
    })
    confirmSpy.mockRestore()
  })

  it('une suppression annulée (confirm=false) n\'envoie AUCUNE requête DELETE', async () => {
    mockFetchImplementation()
    useGame.mockReturnValue(makeConfigPageMock({
      gameState: {
        backgrounds: [],
        entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: 'Retour dans 20mn', IMAGE_IS_CUSTOM: true, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      },
    }))
    const section = await renderAndGetSection()

    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const deleteBtn = await waitFor(() => within(section).getByText("Retirer l'image"))
    fireEvent.click(deleteBtn)

    const deleteCall = global.fetch.mock.calls.find(([url, opts]) => url === '/api/config/entracte-image' && opts?.method === 'DELETE')
    expect(deleteCall).toBeUndefined()
    confirmSpy.mockRestore()
  })
})
