import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'


vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

// #215 — QuestionsPage appelle désormais useNavigate()/useSearchParams()
// (onglets Questions/Rafale, lien "configurer le quiz" → navigation vers
// Backstage) : ces tests ne montent aucun Router réel, donc mock plutôt que
// de faire porter <MemoryRouter> à chacun des (nombreux) render() ci-dessous.
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions || [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

vi.mock('../components/CategoryBalance', () => ({
  default: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`qcard-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {
    CULTURE: { label: 'Culture', icon: '🎭', color: '#8b5cf6' },
    SPORT:   { label: 'Sport',   icon: '⚽', color: '#22c55e' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css',    () => ({}))


import { useGame } from '../hooks/GameContext'

// #245 (v11.2, lot V3-2) — éditeur de manches, maquette VALIDÉE (f″)
// admin_questions_list__rounds_entractes_delete_choice : groupes repliables, « Sans manche » en premier, ▲▼,
// « Déplacer vers… », création avec entractes, suppression à deux choix, « + Question », champ « Manche ».

const Q = (id, type = 'QCM', over = {}) => ({ ID: String(id), TYPE: type, QUESTION: `Q${id}`, ANSWER: 'A', POINTS: '1', TIME: '30', CATEGORY: 'CULTURE', ORDER: Number(id), ...over })
const QUESTIONS = {
  1: Q(1, 'QCM', { ROUND_ID: 'r1' }), 2: Q(2, 'SPEEDY', { ROUND_ID: 'r1' }), 3: Q(3, 'ENTRACTE', { ROUND_ID: 'r1' }),
  4: Q(4, 'QCM', { ROUND_ID: 'r2' }), 5: Q(5, 'SPEEDY'), 6: Q(6, 'SPEEDY'),
}
const ROUNDS = [{ ID: 'r1', NAME: 'Échauffement' }, { ID: 'r2', NAME: 'Finale' }]

const mk = (over = {}) => ({
  questions: QUESTIONS, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(() => true),
  gameState: { phase: 'STOPPED', question: null, ROUNDS }, newGame: vi.fn(),
  roundsUpdate: vi.fn(() => true), roundsUpdateResult: null, clearRoundsUpdateResult: vi.fn(),
  ...over,
})
const show = (over = {}) => { const g = mk(over); useGame.mockReturnValue(g); return { g, ...render(<QuestionsPage />) } }
const lastPayload = (g) => g.roundsUpdate.mock.calls.at(-1)[0]
const group = (name) => screen.getByRole('region', { name: name.replace(/^Manche \d+ · /, '') })
const titles = () => screen.getAllByRole('region').map((r) => r.getAttribute('aria-label'))

const createStorageMock = () => {
  const m = new Map()
  return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => { m.set(k, String(v)) }, removeItem: (k) => { m.delete(k) }, clear: () => m.clear() }
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('localStorage', createStorageMock())
  global.fetch = vi.fn((url, opts) => {
    const isSave = String(url).includes('/questions') && opts?.method === 'POST'
    return Promise.resolve({ ok: isSave, json: () => Promise.resolve({}), text: () => Promise.resolve('') })
  })
})
afterEach(() => { vi.unstubAllGlobals() })

describe('Éditeur — sans manche (comme v11.1)', () => {
  it('aucun groupe, liste inchangée, bouton « + Nouvelle manche », pas de champ Manche', () => {
    show({ gameState: { phase: 'STOPPED', question: null, ROUNDS: [] } })
    expect(screen.queryAllByRole('region')).toHaveLength(0)
    expect(screen.getByRole('button', { name: '+ Nouvelle manche' })).toBeTruthy()
    expect(screen.queryByLabelText('Manche')).toBeNull()
    expect(screen.getByTestId('qcard-1')).toBeTruthy()
  })
})

describe('Éditeur — groupes', () => {
  it('Critical — « Sans manche » en premier, puis les manches dans l’ordre ; compteurs « questions + entractes »', () => {
    show()
    expect(titles()).toEqual(['Sans manche', 'Échauffement', 'Finale'])
    expect(group('Manche 1 · Échauffement').textContent).toMatch(/2 questions \+ 1 entracte/)
    expect(group('Sans manche').textContent).toMatch(/2 questions/)
    expect(within(group('Sans manche')).getByTestId('qcard-5')).toBeTruthy()
    expect(within(group('Manche 2 · Finale')).getByTestId('qcard-4')).toBeTruthy()
  })

  it('repli : mémorisé dans localStorage (par navigateur), « Tout replier » replie puis déplie tout', () => {
    show()
    fireEvent.click(within(group('Manche 1 · Échauffement')).getByRole('button', { name: 'Replier' }))
    expect(screen.queryByTestId('qcard-1')).toBeNull()
    expect(JSON.parse(localStorage.getItem('buzz.rounds.collapsed'))).toMatchObject({ r1: true })
    fireEvent.click(screen.getByRole('button', { name: /Tout replier/ }))
    expect(screen.queryByTestId('qcard-4')).toBeNull()
    expect(screen.queryByTestId('qcard-5')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Tout replier/ }))
    expect(screen.getByTestId('qcard-1')).toBeTruthy()
  })

  it('repli restauré au rechargement', () => {
    localStorage.setItem('buzz.rounds.collapsed', JSON.stringify({ r2: true }))
    show()
    expect(screen.queryByTestId('qcard-4')).toBeNull()
    expect(screen.getByTestId('qcard-1')).toBeTruthy()
  })

  it('manche vide : message dédié', () => {
    show({ gameState: { phase: 'STOPPED', question: null, ROUNDS: [...ROUNDS, { ID: 'r3', NAME: 'Rapidité' }] } })
    expect(group('Manche 3 · Rapidité').textContent).toMatch(/Manche vide/)
  })
})

describe('Éditeur — gestes → ROUNDS_UPDATE complet (scénario 14)', () => {
  it('Critical — création : case « entractes » cochée par défaut, envoi ENTRACTES et toutes les questions citées', () => {
    const { g } = show()
    fireEvent.click(screen.getByRole('button', { name: '+ Nouvelle manche' }))
    expect(screen.getByLabelText(/Créer les entractes de début et de fin/).checked).toBe(true)
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: '  Duel  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Valider le nom' }))
    expect(lastPayload(g)).toEqual({
      rounds: [
        { id: 'r1', name: 'Échauffement', questions: ['1', '2', '3'] },
        { id: 'r2', name: 'Finale', questions: ['4'] },
        { name: 'Duel', questions: [], entractes: true },
      ],
      unassigned: ['5', '6'],
      deletedRounds: [],
    })
  })

  it('case décochée : ENTRACTES false', () => {
    const { g } = show()
    fireEvent.click(screen.getByRole('button', { name: '+ Nouvelle manche' }))
    fireEvent.click(screen.getByLabelText(/Créer les entractes/))
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: 'Duel' } })
    fireEvent.keyDown(screen.getByLabelText('Nom de la manche'), { key: 'Enter' })
    expect(lastPayload(g).rounds.at(-1)).toEqual({ name: 'Duel', questions: [], entractes: false })
  })

  it('nom vide : validation impossible, rien n’est envoyé ; Échap annule', () => {
    const { g } = show()
    fireEvent.click(screen.getByRole('button', { name: '+ Nouvelle manche' }))
    expect(screen.getByRole('button', { name: 'Valider le nom' }).disabled).toBe(true)
    fireEvent.keyDown(screen.getByLabelText('Nom de la manche'), { key: 'Enter' })
    expect(g.roundsUpdate).not.toHaveBeenCalled()
    fireEvent.keyDown(screen.getByLabelText('Nom de la manche'), { key: 'Escape' })
    expect(screen.queryByLabelText('Nom de la manche')).toBeNull()
  })

  it('renommage : structure complète, entractes inchangées (pas de ENTRACTES)', () => {
    const { g } = show()
    fireEvent.click(within(group('Manche 1 · Échauffement')).getByRole('button', { name: 'Renommer la manche' }))
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: 'Mise en route' } })
    fireEvent.click(screen.getByRole('button', { name: 'Valider le nom' }))
    const p = lastPayload(g)
    expect(p.rounds[0]).toEqual({ id: 'r1', name: 'Mise en route', questions: ['1', '2', '3'] })
    expect(p.rounds).toHaveLength(2)
  })

  it('▲▼ : réordonne les manches (désactivés aux extrémités)', () => {
    const { g } = show()
    expect(within(group('Manche 1 · Échauffement')).getByRole('button', { name: 'Monter la manche' }).disabled).toBe(true)
    expect(within(group('Manche 2 · Finale')).getByRole('button', { name: 'Descendre la manche' }).disabled).toBe(true)
    fireEvent.click(within(group('Manche 2 · Finale')).getByRole('button', { name: 'Monter la manche' }))
    expect(lastPayload(g).rounds.map((r) => r.id)).toEqual(['r2', 'r1'])
  })

  it('« Déplacer vers… » (tactile) : question vers une autre manche, à la fin ; groupe actuel désactivé', () => {
    const { g } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Déplacer la question 5 vers une autre manche' }))
    expect(screen.getByRole('menuitem', { name: 'Sans manche (actuelle)' }).disabled).toBe(true)
    fireEvent.click(screen.getByRole('menuitem', { name: 'Manche 2 · Finale' }))
    const p = lastPayload(g)
    expect(p.rounds[1].questions).toEqual(['4', '5'])
    expect(p.unassigned).toEqual(['6'])
  })
})

describe('Éditeur — suppression avec choix (scénario 31)', () => {
  const open = () => {
    const r = show()
    fireEvent.click(within(group('Manche 1 · Échauffement')).getByRole('button', { name: 'Supprimer la manche' }))
    return r
  }

  it('Critical — fenêtre : comptes justes, focus sur « Conserver », rôle dialog modal', () => {
    open()
    const d = screen.getByRole('dialog')
    expect(d.getAttribute('aria-modal')).toBe('true')
    expect(d.textContent).toMatch(/Elle contient 2 questions et 1 entracte\. Que faire de ces 3 entrées \?/)
    const keep = screen.getByRole('button', { name: /Conserver les 3 entrées/ })
    expect(document.activeElement).toBe(keep)
    expect(screen.getByRole('button', { name: /Supprimer aussi les 3 entrées/ })).toBeTruthy()
  })

  it('Conserver : DELETED_ROUNDS KEEP, entrées à la fin de « Sans manche »', () => {
    const { g } = open()
    fireEvent.click(screen.getByRole('button', { name: /Conserver les 3 entrées/ }))
    const p = lastPayload(g)
    expect(p.deletedRounds).toEqual([{ id: 'r1', questions: 'KEEP' }])
    expect(p.rounds.map((r) => r.id)).toEqual(['r2'])
    expect(p.unassigned).toEqual(['5', '6', '1', '2', '3'])
  })

  it('Supprimer aussi : DELETED_ROUNDS DELETE, entrées non citées', () => {
    const { g } = open()
    fireEvent.click(screen.getByRole('button', { name: /Supprimer aussi les 3 entrées/ }))
    const p = lastPayload(g)
    expect(p.deletedRounds).toEqual([{ id: 'r1', questions: 'DELETE' }])
    expect(p.unassigned).toEqual(['5', '6'])
  })

  it('Annuler et Échap : rien n’est envoyé', () => {
    const { g } = open()
    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(within(group('Manche 1 · Échauffement')).getByRole('button', { name: 'Supprimer la manche' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(g.roundsUpdate).not.toHaveBeenCalled()
  })
})

describe('Éditeur — « + Question » et champ « Manche »', () => {
  it('Critical — « + Question » d’une manche : le formulaire porte la manche, l’enregistrement envoie round_id', async () => {
    show()
    fireEvent.click(within(group('Manche 2 · Finale')).getByRole('button', { name: /Ajouter une question à/ }))
    expect(screen.getByLabelText('Manche').value).toBe('r2')
    fireEvent.change(screen.getByLabelText(/^Question \*/), { target: { value: 'Nouvelle ?' } })
    fireEvent.change(document.getElementById('answer-input'), { target: { value: 'Oui' } })
    fireEvent.click(screen.getByRole('button', { name: /^(Ajouter)$/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const body = global.fetch.mock.calls.find((c) => c[0] === '/questions')[1].body
    expect(body.get('round_id')).toBe('r2')
  })

  it('changer la manche d’une question existante : ROUNDS_UPDATE après l’enregistrement (pas de round_id)', async () => {
    const { g } = show()
    fireEvent.click(screen.getByTestId('qcard-5'))
    expect(screen.getByLabelText('Manche').value).toBe('')
    fireEvent.change(screen.getByLabelText('Manche'), { target: { value: 'r1' } })
    fireEvent.click(screen.getByRole('button', { name: /^Modifier$/ }))
    await waitFor(() => expect(g.roundsUpdate).toHaveBeenCalled())
    const body = global.fetch.mock.calls.find((c) => c[0] === '/questions')[1].body
    expect(body.has('round_id')).toBe(false)
    expect(lastPayload(g).rounds[0].questions).toEqual(['1', '2', '3', '5'])
  })
})

describe('Éditeur — refus serveur', () => {
  it('ROUNDS_UPDATE_RESULT REFUSED : message d’erreur et résultat consommé', () => {
    const g = mk({ roundsUpdateResult: { status: 'REFUSED', reason: 'INCOMPLETE' } })
    useGame.mockReturnValue(g)
    render(<QuestionsPage />)
    expect(screen.getByText(/ont changé sur un autre poste/)).toBeTruthy()
    expect(g.clearRoundsUpdateResult).toHaveBeenCalled()
  })
})
