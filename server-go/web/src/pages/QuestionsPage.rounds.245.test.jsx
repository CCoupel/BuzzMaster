import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// #245 (v11.2, lot V3-2) — éditeur de questions en manches, maquette VALIDÉE (f″)
// admin_questions_list__rounds_entractes_delete_choice : sans manche = liste identique à v11.1 + « + Nouvelle manche » ;
// création avec la case « Créer les entractes de début et de fin » cochée par défaut ; ▲▼ ; « Déplacer vers… » ; repli
// (localStorage) et « Tout replier » ; filtre « x / y » ; fenêtre de suppression à deux choix (focus « Conserver », Annuler
// n'envoie rien, clavier). Chaque geste envoie un ROUNDS_UPDATE COMPLET (espion roundsUpdate). Scénarios 13 à 17, 21, 31.
// Boîte noire par libellés / rôles ARIA. `critical` : chaque geste envoie la structure complète ; suppression KEEP/DELETE.

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

// ---------------------------------------------------------------------------
// Import useGame après mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helper : mock minimal useGame
// ---------------------------------------------------------------------------

const Q = (id, order, round, over = {}) => ({ ID: String(id), TYPE: 'SPEEDY', QUESTION: `Question ${id}`, ANSWER: 'a', POINTS: '1', TIME: '30', CATEGORY: 'CULTURE', ORDER: order, ...(round ? { ROUND_ID: round } : {}), ...over })
const QUESTIONS = {
  1: Q(1, 0, 'r1'), 2: Q(2, 1, 'r1'), 3: Q(3, 0, 'r2'), 4: Q(4, 0), 5: Q(5, 1),
  6: Q(6, 2, 'r1', { TYPE: 'ENTRACTE', QUESTION: 'Fin de la manche Un' }),
}
const ROUNDS = [{ ID: 'r1', NAME: 'Un' }, { ID: 'r2', NAME: 'Deux' }]

let game
const mount = (over = {}, rounds = ROUNDS, questions = QUESTIONS) => {
  game = {
    questions, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(),
    gameState: { phase: 'STOPPED', question: null, ROUNDS: rounds },
    newGame: vi.fn(), roundsUpdate: vi.fn(() => true), roundsUpdateResult: null, clearRoundsUpdateResult: vi.fn(), ...over,
  }
  useGame.mockReturnValue(game)
  return render(<QuestionsPage />)
}
// le nom accessible d'un groupe peut préfixer le rang (« Manche 1 · Un ») : on retrouve la région par son nom de manche
const group = (name) => {
  const hit = screen.getAllByRole('region').find((r) => { const l = r.getAttribute('aria-label') || ''; return l === name || l.endsWith(` ${name}`) || l.endsWith(`· ${name}`) })
  if (!hit) throw new Error(`groupe « ${name} » introuvable : ${screen.getAllByRole('region').map((r) => r.getAttribute('aria-label')).join(' | ')}`)
  return hit
}
const lastPayload = () => game.roundsUpdate.mock.calls[game.roundsUpdate.mock.calls.length - 1][0]
const cited = (p) => [...p.unassigned, ...p.rounds.flatMap((r) => r.questions)].map(String).sort()
const ALL_IDS = ['1', '2', '3', '4', '5', '6']

// stockage mémoire : le préférence de repli est locale au navigateur (localStorage), absente de cet environnement de test
const memoryStorage = () => {
  const m = new Map()
  return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => { m.set(k, String(v)) }, removeItem: (k) => { m.delete(k) }, clear: () => m.clear() }
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('localStorage', memoryStorage())
})
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

describe('Éditeur en manches — sans manche (#245, scénario 13)', () => {
  it('Critical — aucune manche : liste identique à v11.1 (pas de groupe, pas d’en-tête « Sans manche ») + bouton « + Nouvelle manche »', () => {
    mount({}, [], Object.fromEntries(Object.entries(QUESTIONS).map(([k, q]) => [k, { ...q, ROUND_ID: undefined }])))
    expect(screen.getByRole('button', { name: /\+ Nouvelle manche/ })).toBeTruthy()
    expect(screen.queryByRole('region', { name: 'Sans manche' })).toBeNull()
    expect(screen.queryByRole('button', { name: /Tout replier/ })).toBeNull()
    for (const id of ['1', '3', '5']) expect(screen.getByTestId(`qcard-${id}`)).toBeTruthy()
  })
})

describe('Éditeur en manches — groupes, repli, filtre (#245, scénarios 15 et 16)', () => {
  it('« Sans manche » en premier, puis les manches dans l’ordre ; chaque groupe porte ses questions', () => {
    mount()
    const regions = screen.getAllByRole('region').map((r) => r.getAttribute('aria-label'))
    expect(regions[0]).toBe('Sans manche')
    expect(regions[1]).toMatch(/Un$/)
    expect(regions[2]).toMatch(/Deux$/)
    expect(within(group('Un')).getByTestId('qcard-1')).toBeTruthy()
    expect(within(group('Un')).getByTestId('qcard-6')).toBeTruthy() // l'ENTRACTE de la manche
    expect(within(group('Sans manche')).getByTestId('qcard-4')).toBeTruthy()
  })

  it('Critical (15) — replier une manche masque ses questions ; le repli est conservé au rechargement (localStorage)', () => {
    const first = mount()
    fireEvent.click(within(group('Un')).getByRole('button', { name: 'Replier' }))
    expect(screen.queryByTestId('qcard-1')).toBeNull()
    expect(screen.getByTestId('qcard-3')).toBeTruthy() // l'autre manche reste dépliée
    first.unmount()

    mount()
    expect(screen.queryByTestId('qcard-1')).toBeNull()
    expect(within(group('Un')).getByRole('button', { name: 'Déplier' })).toBeTruthy()
  })

  it('« Tout replier » replie tous les groupes, une seconde fois les déplie', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: /Tout replier/ }))
    for (const id of ['1', '3', '4']) expect(screen.queryByTestId(`qcard-${id}`)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Tout replier|Tout déplier/ }))
    expect(screen.getByTestId('qcard-1')).toBeTruthy()
  })

  it('en-tête de groupe : nombre de questions et d’entractes (« 2 questions + 1 entracte »)', () => {
    mount()
    expect(group('Un').textContent).toMatch(/2 questions/)
    expect(group('Un').textContent).toMatch(/1 entracte/)
  })
})

describe('Éditeur en manches — chaque geste envoie la structure COMPLÈTE (#245, scénario 14)', () => {
  it('Critical — ▼ sur la première manche : ROUNDS_UPDATE avec l’ordre inversé et TOUTES les questions citées', () => {
    mount()
    fireEvent.click(within(group('Un')).getByRole('button', { name: 'Descendre la manche' }))
    const p = lastPayload()
    expect(p.rounds.map((r) => r.id)).toEqual(['r2', 'r1'])
    expect(cited(p)).toEqual(ALL_IDS)
  })

  it('▲ est désactivé sur la première manche, ▼ sur la dernière', () => {
    mount()
    expect(within(group('Un')).getByRole('button', { name: 'Monter la manche' }).disabled).toBe(true)
    expect(within(group('Deux')).getByRole('button', { name: 'Descendre la manche' }).disabled).toBe(true)
  })

  it('Critical — « Déplacer vers… » : une question de « Sans manche » passe dans la manche Deux ; structure complète', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Déplacer la question 4 vers une autre manche' }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Deux/ }))
    const p = lastPayload()
    expect(p.rounds.find((r) => r.id === 'r2').questions).toEqual(['3', '4'])
    expect(p.unassigned).toEqual(['5'])
    expect(cited(p)).toEqual(ALL_IDS)
  })

  it('« Déplacer vers… » une question de manche vers « Sans manche »', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Déplacer la question 3 vers une autre manche' }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Sans manche/ }))
    const p = lastPayload()
    expect(p.unassigned).toEqual(['4', '5', '3'])
    expect(p.rounds.find((r) => r.id === 'r2').questions).toEqual([])
    expect(cited(p)).toEqual(ALL_IDS)
  })

  it('une ENTRACTE de manche se déplace comme toute question', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Déplacer la question 6 vers une autre manche' }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Deux/ }))
    expect(lastPayload().rounds.find((r) => r.id === 'r2').questions).toContain('6')
  })
})

describe('Éditeur en manches — création et renommage (#245, scénarios 14, 22, 31)', () => {
  const create = (name, { entractes = true } = {}) => {
    fireEvent.click(screen.getByRole('button', { name: /\+ Nouvelle manche/ }))
    const box = screen.getByRole('checkbox', { name: /Créer les entractes de début et de fin/ })
    if (entractes !== box.checked) fireEvent.click(box)
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: name } })
    fireEvent.click(screen.getByRole('button', { name: 'Valider le nom' }))
  }

  it('Critical — la case « Créer les entractes de début et de fin » est cochée par défaut ; la validation envoie la nouvelle manche SANS id avec entractes:true', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: /\+ Nouvelle manche/ }))
    expect(screen.getByRole('checkbox', { name: /Créer les entractes de début et de fin/ }).checked).toBe(true)
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: '  Finale  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Valider le nom' }))
    const p = lastPayload()
    const fresh = p.rounds[p.rounds.length - 1]
    expect(fresh).toEqual({ name: 'Finale', questions: [], entractes: true })
    expect(cited(p)).toEqual(ALL_IDS)
  })

  it('case décochée → entractes:false', () => {
    mount()
    create('Sans pauses', { entractes: false })
    expect(lastPayload().rounds.pop()).toMatchObject({ name: 'Sans pauses', entractes: false })
  })

  it('nom vide : validation désactivée, rien n’est envoyé ; saisie limitée à 60 caractères', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: /\+ Nouvelle manche/ }))
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Valider le nom' }).disabled).toBe(true)
    expect(screen.getByLabelText('Nom de la manche').getAttribute('maxlength')).toBe('60') // 61 caractères impossibles à saisir
    expect(game.roundsUpdate).not.toHaveBeenCalled()
  })

  it('renommer : ✎ puis nom + validation → ROUNDS_UPDATE avec le nouveau nom et l’ID inchangé ; Annuler n’envoie rien', () => {
    mount()
    fireEvent.click(within(group('Un')).getByRole('button', { name: 'Renommer la manche' }))
    fireEvent.change(screen.getByLabelText('Nom de la manche'), { target: { value: 'Échauffement' } })
    fireEvent.click(screen.getByRole('button', { name: 'Valider le nom' }))
    expect(lastPayload().rounds[0]).toMatchObject({ id: 'r1', name: 'Échauffement' })

    game.roundsUpdate.mockClear()
    fireEvent.click(within(group('Deux')).getByRole('button', { name: 'Renommer la manche' }))
    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(game.roundsUpdate).not.toHaveBeenCalled()
  })
})

describe('Éditeur en manches — suppression avec choix (#245, scénarios 26, 27, 31)', () => {
  const openDelete = (name = 'Un') => {
    fireEvent.click(within(group(name)).getByRole('button', { name: 'Supprimer la manche' }))
    return screen.getByRole('dialog')
  }

  it('Critical — la fenêtre dit combien d’entrées et d’entractes sont concernées, avec deux choix et Annuler ; focus sur « Conserver »', () => {
    mount()
    const dlg = openDelete('Un')
    expect(dlg.getAttribute('aria-modal')).toBe('true')
    expect(within(dlg).getByRole('button', { name: /Conserver les 3 entrées/ })).toBeTruthy()
    expect(within(dlg).getByRole('button', { name: /Supprimer aussi les 3 entrées/ })).toBeTruthy()
    expect(dlg.textContent).toMatch(/entracte/i)
    expect(document.activeElement).toBe(within(dlg).getByRole('button', { name: /Conserver les 3 entrées/ }))
  })

  it('Critical — Conserver : DELETED_ROUNDS{KEEP}, ses entrées passent à la fin de « Sans manche », toutes les questions restent citées', () => {
    mount()
    fireEvent.click(within(openDelete('Un')).getByRole('button', { name: /Conserver/ }))
    const p = lastPayload()
    expect(p.deletedRounds).toEqual([{ id: 'r1', questions: 'KEEP' }])
    expect(p.rounds.map((r) => r.id)).toEqual(['r2'])
    expect(p.unassigned).toEqual(['4', '5', '1', '2', '6'])
    expect(cited(p)).toEqual(ALL_IDS)
  })

  it('Critical — Supprimer aussi : DELETED_ROUNDS{DELETE}, ses entrées ne sont plus citées', () => {
    mount()
    fireEvent.click(within(openDelete('Un')).getByRole('button', { name: /Supprimer aussi/ }))
    const p = lastPayload()
    expect(p.deletedRounds).toEqual([{ id: 'r1', questions: 'DELETE' }])
    expect(cited(p)).toEqual(['3', '4', '5'])
  })

  it('Annuler ferme la fenêtre et n’envoie rien ; Échap aussi', () => {
    mount()
    fireEvent.click(within(openDelete('Un')).getByRole('button', { name: 'Annuler' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(game.roundsUpdate).not.toHaveBeenCalled()
    fireEvent.keyDown(openDelete('Deux'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(game.roundsUpdate).not.toHaveBeenCalled()
  })

  it('le focus reste dans la fenêtre (Tab boucle)', () => {
    mount()
    const dlg = openDelete('Un')
    const buttons = within(dlg).getAllByRole('button')
    buttons[buttons.length - 1].focus()
    fireEvent.keyDown(dlg, { key: 'Tab' })
    expect(dlg.contains(document.activeElement)).toBe(true)
  })
})

describe('Éditeur en manches — résultat du serveur (#245)', () => {
  it('REFUSED (INCOMPLETE / MISSING_ROUND_DECISION) : message à l’admin, résultat consommé', () => {
    mount({ roundsUpdateResult: { status: 'REFUSED', reason: 'INCOMPLETE', questions: ['4'] } })
    expect(screen.getByRole('status').textContent).toMatch(/manche|identifiant/i)
    expect(game.clearRoundsUpdateResult).toHaveBeenCalled()
  })
})

describe('Éditeur en manches — filtre de catégorie et « + Question » (#245, scénarios 16 et 17)', () => {
  it('Critical — filtre actif : chaque groupe affiche « x / y » (affichées / total), les groupes vides restent affichés', () => {
    const qs = {
      1: Q(1, 0, 'r1', { CATEGORY: 'HISTORY' }), 2: Q(2, 1, 'r1', { CATEGORY: 'CULTURE' }), 3: Q(3, 0, 'r2', { CATEGORY: 'CULTURE' }),
      4: Q(4, 0, '', { CATEGORY: 'HISTORY' }),
    }
    mount({}, ROUNDS, qs)
    const chip = screen.getAllByRole('button').find((b) => b.textContent.includes('Histoire') || b.getAttribute('title') === 'Histoire')
    fireEvent.click(chip) // filtre « Histoire »
    expect(group('Un').textContent).toMatch(/1 \/ 2 questions/)
    expect(group('Deux').textContent).toMatch(/0 \/ 1 question/) // groupe sans résultat : toujours affiché
    expect(screen.queryByTestId('qcard-2')).toBeNull()
    expect(screen.getByTestId('qcard-1')).toBeTruthy()
  })

  it('une manche vide est affichée avec son invitation', () => {
    mount({}, [...ROUNDS, { ID: 'r3', NAME: 'Vide' }])
    expect(group('Vide').textContent).toMatch(/Manche vide/)
  })

  it('Critical (17) — « + Question » dans une manche ouvre le formulaire avec cette manche choisie et l’envoie en round_id', async () => {
    const posted = []
    vi.stubGlobal('fetch', vi.fn((url, opts) => {
      if (String(url).includes('/questions') && opts?.method === 'POST') posted.push(opts.body)
      return Promise.resolve({ ok: opts?.method === 'POST', json: () => Promise.resolve({}) })
    }))
    mount()
    fireEvent.click(screen.getByRole('button', { name: /Ajouter une question à .*Deux/ }))
    expect(screen.getByRole('combobox', { name: /Manche/ }).value).toBe('r2')
    fireEvent.change(screen.getByLabelText(/Question \*|Question$/), { target: { value: 'Nouvelle ?' } })
    fireEvent.change(document.getElementById('answer-input'), { target: { value: 'Oui' } })
    fireEvent.click(screen.getByRole('button', { name: /^(Ajouter|Modifier)$/ }))
    await vi.waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('round_id')).toBe('r2')
  })

  it('le champ Manche du formulaire liste « Sans manche » et les manches', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: /Ajouter une question à .*Un/ }))
    const sel = screen.getByRole('combobox', { name: /Manche/ })
    const labels = [...sel.querySelectorAll('option')].map((o) => o.textContent)
    expect(labels[0]).toMatch(/Sans manche/)
    expect(labels.some((l) => /Un/.test(l))).toBe(true)
    expect(labels.some((l) => /Deux/.test(l))).toBe(true)
  })
})

