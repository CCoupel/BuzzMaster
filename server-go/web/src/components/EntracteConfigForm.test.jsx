/**
 * Tests — EntracteConfigForm (#119, delta C1/C4 du plan
 * _work/reports/plan-entracte-119-fixes-20260820-155123.md).
 *
 * RÉ-ADRESSÉ depuis pages/QuestionsPage.entracte.test.jsx (#215, milestone
 * v9.0.0) : la section Entracte a été extraite de QuestionsPage.jsx vers ce
 * composant autonome (props {gameState, sendMessage}, aucun hook interne) —
 * QuestionsPage ne la rend plus du tout, elle vit désormais dans l'onglet
 * "Entracte" de BackstagePage.jsx. Assertions PRÉSERVÉES à l'identique —
 * seul le point de montage change : EntracteConfigForm directement, plus
 * besoin des mocks useCategories/useCategoryFilter/QuestionCard/AIGenerateModal
 * (jamais utilisés par ce composant).
 *
 * C1-F2 : la configuration du panneau ENTRACTE (formulaire propre, propre
 * bouton d'enregistrement, action dédiée `UPDATE_ENTRACTE_CONFIG`, payload
 * UPPER_SNAKE : TITLE, SUBTITLE, PANEL_SIZE, ANIM_PERIOD, ANIM_INTENSITY,
 * TRANSITION_MS — plan C1-B3).
 *
 * ⚠️ C4 (le piège que ce fichier existe pour verrouiller) : le formulaire
 * lit TOUJOURS `gameState.entracteConfigSaved` (← GameState.ENTRACTE_CONFIG_SAVED,
 * admin-only), JAMAIS `gameState.entracteConfig` (le diffusé, gelé pendant
 * une pause) — sans quoi un enregistrement fait pendant l'entracte
 * semblerait perdu au retour sur cette page. Une mention « Prendra effet au
 * prochain entracte » s'affiche quand `gameState.entracte` est actif.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import EntracteConfigForm from './EntracteConfigForm'

vi.mock('./Button', () => ({
  default: ({ children, onClick, disabled, type, loading, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('./Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

const DEFAULT_ENTRACTE_SAVED = {
  TITLE: 'ENTRACTE',
  SUBTITLE: 'Retour dans 20mn',
  IMAGE_IS_CUSTOM: false,
  PANEL_SIZE: 65,
  ANIM_PERIOD: 10,
  ANIM_INTENSITY: 20,
  TRANSITION_MS: 2000,
}

const makeGameState = (overrides = {}) => ({
  entracte: false,
  entracteConfigSaved: DEFAULT_ENTRACTE_SAVED,
  // Le diffusé délibérément DIFFÉRENT du enregistré dans les tests ci-dessous
  // qui l'exercent — s'il fuitait dans le formulaire, ces tests le
  // détecteraient (contrat C4 : le formulaire ne doit JAMAIS s'y alimenter).
  entracteConfig: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'NE DOIT JAMAIS APPARAÎTRE DANS LE FORMULAIRE' },
  ...overrides,
})

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn().mockResolvedValue({ ok: false })
})

describe('EntracteConfigForm — alimentation depuis ENTRACTE_CONFIG_SAVED (C4)', () => {
  it("le formulaire lit gameState.entracteConfigSaved, JAMAIS gameState.entracteConfig (le diffusé)", () => {
    render(<EntracteConfigForm
      gameState={makeGameState({
        entracteConfigSaved: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'Valeur enregistrée' },
        entracteConfig: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'Valeur diffusée (gelée)' },
      })}
      sendMessage={vi.fn()}
    />)

    expect(screen.getByDisplayValue('Valeur enregistrée')).toBeInTheDocument()
    expect(screen.queryByDisplayValue('Valeur diffusée (gelée)')).toBeNull()
  })
})

describe('EntracteConfigForm — sauvegarde émet UPDATE_ENTRACTE_CONFIG (C1-B3)', () => {
  it('Enregistrer envoie UPDATE_ENTRACTE_CONFIG avec les champs UPPER_SNAKE du contrat', () => {
    const sendMessage = vi.fn()
    render(<EntracteConfigForm gameState={makeGameState()} sendMessage={sendMessage} />)

    fireEvent.change(screen.getByDisplayValue('ENTRACTE'), { target: { value: 'Pause déjeuner' } })
    fireEvent.click(screen.getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith('UPDATE_ENTRACTE_CONFIG', expect.objectContaining({
      TITLE: 'Pause déjeuner',
    }))
    const call = sendMessage.mock.calls.find(([action]) => action === 'UPDATE_ENTRACTE_CONFIG')
    const payload = call[1]
    for (const key of ['TITLE', 'SUBTITLE', 'PANEL_SIZE', 'ANIM_PERIOD', 'ANIM_INTENSITY', 'TRANSITION_MS']) {
      expect(payload).toHaveProperty(key)
    }
  })

  it("l'enregistrement réussit et n'est PAS bloqué même si un entracte est actif (accepté à l'enregistrement, C4)", () => {
    const sendMessage = vi.fn()
    render(<EntracteConfigForm gameState={makeGameState({ entracte: true })} sendMessage={sendMessage} />)

    fireEvent.click(screen.getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith('UPDATE_ENTRACTE_CONFIG', expect.anything())
  })
})

describe('EntracteConfigForm — mention "Prendra effet au prochain entracte" (C4-F2)', () => {
  it("n'affiche AUCUNE mention hors entracte", () => {
    render(<EntracteConfigForm gameState={makeGameState({ entracte: false })} sendMessage={vi.fn()} />)
    expect(screen.queryByText(/prendra effet au prochain entracte/i)).toBeNull()
  })

  it('affiche la mention "Prendra effet au prochain entracte" pendant qu\'un entracte est actif', () => {
    render(<EntracteConfigForm gameState={makeGameState({ entracte: true })} sendMessage={vi.fn()} />)
    expect(screen.getByText(/prendra effet au prochain entracte/i)).toBeInTheDocument()
  })
})

describe('EntracteConfigForm — animation ANIM_INTENSITY=0 (non-régression)', () => {
  it('affiche "animation desactivee" à intensité 0 (repris tel quel de la section retirée de ConfigPage)', () => {
    render(<EntracteConfigForm
      gameState={makeGameState({ entracteConfigSaved: { ...DEFAULT_ENTRACTE_SAVED, ANIM_INTENSITY: 0 } })}
      sendMessage={vi.fn()}
    />)
    expect(screen.getByText(/animation d.sactiv.e/i)).toBeInTheDocument()
  })
})

describe('EntracteConfigForm — image de fond (endpoint renommé, C1-B6)', () => {
  it("l'upload envoie un POST multipart vers /api/game/entracte-image (PAS l'ancien /api/config/entracte-image)", () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ image_is_custom: true }) })
    const { container } = render(<EntracteConfigForm gameState={makeGameState()} sendMessage={vi.fn()} />)

    const fileInput = container.querySelector('input[type="file"]')
    expect(fileInput).not.toBeNull()
    const file = new File(['fake-image'], 'fond.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })
    fireEvent.click(screen.getByText(/enregistrer l.image/i))

    const uploadCall = global.fetch.mock.calls.find(([url]) => typeof url === 'string' && url.includes('entracte-image'))
    expect(uploadCall).toBeDefined()
    expect(uploadCall[0]).toBe('/api/game/entracte-image')
    expect(uploadCall[0]).not.toContain('/api/config/')
  })
})
