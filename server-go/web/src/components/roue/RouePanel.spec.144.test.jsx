/**
 * #144 (v12.0.0, LOT-1.2) — commandes admin ROUE : spécification (contracts/roue.md §5, §5.2, §8 ; maquette M2).
 * Complète RouePanel.144.test.jsx (développeur). L'interface ne décide rien : elle affiche ce que dit le serveur
 * (ROUE_SUBPHASE / ROUE_STEP) et envoie les actions ; le serveur reste seul juge (ROUE_ERROR).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, act } from '@testing-library/react'
import RouePanel from './RouePanel'
import RoueSteps from './RoueSteps'
import { RoueErrorLine, RoueKeyboard, RoueNextButtons } from './RoueControls'
import { roueGates } from './roueState'

afterEach(() => cleanup())

const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_GAINS_RULE: 'WINNER_ONLY', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: '', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 300, Bleus: 0 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null,
    SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }] },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})

function actionsMock() {
  return { setTeams: vi.fn(), spin: vi.fn(), stop: vi.fn(), letter: vi.fn(), flip: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn() }
}
const btn = (re) => screen.getByRole('button', { name: re })
const queryBtn = (re) => screen.queryByRole('button', { name: re })

describe('RouePanel — une action principale par étape', () => {
  it('étape 1 : « Lancer la roue » envoie ROUE_SPIN', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    fireEvent.click(btn(/Lancer la roue/))
    expect(a.spin).toHaveBeenCalledTimes(1)
  })

  it('étape 2 : « Forcer l\'arrêt » envoie ROUE_STOP ; décompte de la durée maximale affiché', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 })} actions={a} />)
    expect(screen.getByText(/arrêt automatique dans/)).toBeTruthy()
    fireEvent.click(btn(/Forcer l'arrêt/))
    expect(a.stop).toHaveBeenCalledTimes(1)
  })

  it('étape 3 : aucun geste, la case d\'arrivée est annoncée', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'STOPPING', ROUE_STEP: 3, ROUE_WHEEL: { ...gs().ROUE_WHEEL, STOP: { TO: 1, DURATION_MS: 4000 } } })} actions={actionsMock()} />)
    expect(screen.getByText(/BANQUEROUTE/)).toBeTruthy()
    expect(queryBtn(/Lancer|Forcer|Retourner|Réponse validée/)).toBeNull()
  })

  it('étape 4 : valider une consonne envoie ROUE_LETTER avec la lettre choisie', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_WHEEL: { ...gs().ROUE_WHEEL, CURRENT: 0 } })} actions={a} />)
    expect(btn(/Choisissez une consonne/).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'N' }))
    fireEvent.click(btn(/Valider la lettre N/))
    expect(a.letter).toHaveBeenCalledWith('N')
  })

  it('étape 4 : une consonne déjà proposée n\'est plus sélectionnable', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_LETTERS: [{ LETTER: 'T', TEAM: 'Rouges', HIT: true }] })} actions={actionsMock()} />)
    expect(screen.getByRole('button', { name: 'T' }).disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'N' }).disabled).toBe(false)
  })

  it('étape 4 : le clavier physique + Entrée valide la lettre', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })} actions={a} />)
    fireEvent.keyDown(window, { key: 'n' })
    fireEvent.keyDown(window, { key: 'Enter' })
    expect(a.letter).toHaveBeenCalledWith('N')
  })

  it('étape 4 : la sélection est effacée en quittant LETTER', () => {
    const { rerender } = render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })} actions={actionsMock()} />)
    fireEvent.click(screen.getByRole('button', { name: 'N' }))
    rerender(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1 })} actions={actionsMock()} />)
    rerender(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })} actions={actionsMock()} />)
    expect(btn(/Choisissez une consonne/)).toBeTruthy()
  })

  it('étape 5 (LIT) : « Retourner les cases » envoie ROUE_FLIP', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LIT', ROUE_STEP: 5, ROUE_LAST_EVENT: { SEQ: 3, KIND: 'LETTER_HIT', TEAM: 'Rouges', LETTER: 'N', COUNT: 2, VALUE: 100, GAIN: 200 } })} actions={a} />)
    fireEvent.click(btn(/Retourner les cases/))
    expect(a.flip).toHaveBeenCalledTimes(1)
  })

  it('étape 5 (REVEALING) : aucun geste, gain du tour affiché', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'REVEALING', ROUE_STEP: 5, ROUE_TURN_GAIN: 200, ROUE_LAST_EVENT: { SEQ: 4, KIND: 'FLIP', TEAM: 'Rouges', LETTER: 'N', COUNT: 2, VALUE: 0, GAIN: 0 } })} actions={actionsMock()} />)
    expect(queryBtn(/Retourner/)).toBeNull()
    expect(document.body.textContent).toMatch(/\+200/)
  })

  it('étape 6 : « Réponse validée » envoie ROUE_ANSWER', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: 'SAME' })} actions={a} />)
    fireEvent.click(btn(/Réponse validée/))
    expect(a.answer).toHaveBeenCalledTimes(1)
  })
})

describe('RouePanel — étape 7 : deux boutons, un conseillé', () => {
  const end = (over) => gs({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_NEXT_TEAM: 'Bleus', ...over })

  it('« Tour suivant » envoie HAND SAME, « Équipe suivante » HAND NEXT', () => {
    const a = actionsMock()
    render(<RouePanel gameState={end({ ROUE_NEXT_DEFAULT: 'NEXT' })} actions={a} />)
    fireEvent.click(btn(/Tour suivant/))
    expect(a.next).toHaveBeenLastCalledWith('SAME')
    fireEvent.click(btn(/Équipe suivante/))
    expect(a.next).toHaveBeenLastCalledWith('NEXT')
  })

  it.each([['SAME', /Tour suivant/], ['NEXT', /Équipe suivante/]])('conseillé %s : ce bouton est mis en avant, l\'autre reste utilisable', (rec, re) => {
    render(<RouePanel gameState={end({ ROUE_NEXT_DEFAULT: rec })} actions={actionsMock()} />)
    expect(btn(re).className).toMatch(/primary/)
    const other = rec === 'SAME' ? btn(/Équipe suivante/) : btn(/Tour suivant/)
    expect(other.className).not.toMatch(/primary/)
    expect(other.disabled).toBe(false)
  })

  it('les noms d\'équipes figurent sur les boutons', () => {
    render(<RouePanel gameState={end({ ROUE_NEXT_DEFAULT: 'NEXT' })} actions={actionsMock()} />)
    expect(btn(/Tour suivant/).textContent).toMatch(/Rouges/)
    expect(btn(/Équipe suivante/).textContent).toMatch(/Bleus/)
  })

  it('SOLO : « Équipe suivante » absent, « Tour suivant » seul', () => {
    render(<RouePanel gameState={end({ question: { ...gs().question, ROUE_MODE: 'SOLO' }, ROUE_TEAMS: ['Rouges'], ROUE_NEXT_TEAM: '' })} actions={actionsMock()} />)
    expect(queryBtn(/Équipe suivante/)).toBeNull()
    expect(btn(/Tour suivant/).disabled).toBe(false)
  })

  it('les deux boutons sont aussi actifs depuis l\'étape 6 (réponse fausse ou absente)', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: 'SAME' })} actions={actionsMock()} />)
    expect(btn(/Tour suivant/).disabled).toBe(false)
    expect(btn(/Équipe suivante/).disabled).toBe(false)
  })

  it('banqueroute / tour perdu / lettre absente : le motif est affiché', () => {
    const { rerender } = render(<RouePanel gameState={end({ ROUE_WHEEL: { ...gs().ROUE_WHEEL, CURRENT: 1 } })} actions={actionsMock()} />)
    expect(document.body.textContent).toMatch(/Banqueroute/)
    rerender(<RouePanel gameState={end({ ROUE_LETTERS: [{ LETTER: 'Z', TEAM: 'Rouges', HIT: false }] })} actions={actionsMock()} />)
    expect(document.body.textContent).toMatch(/Lettre Z absente/)
  })
})

describe('RouePanel — plus de consonne', () => {
  it('pas de lancer : « Réponse validée » et fin de tour proposés depuis TURN', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_CONSONANTS_LEFT: false })} actions={a} />)
    expect(queryBtn(/Lancer la roue/)).toBeNull()
    fireEvent.click(btn(/Réponse validée/))
    expect(a.answer).toHaveBeenCalledTimes(1)
    expect(btn(/Tour suivant/)).toBeTruthy()
  })
})

describe('RouePanel — passer la main et dévoiler (admin seulement)', () => {
  it('« Passer la main » envoie ROUE_PASS en début de tour', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    fireEvent.click(btn(/Passer la main/))
    expect(a.pass).toHaveBeenCalledTimes(1)
  })

  it('« Passer la main » est inactif hors TURN et en SOLO', () => {
    const { rerender } = render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 })} actions={actionsMock()} />)
    expect(btn(/Passer la main/).disabled).toBe(true)
    rerender(<RouePanel gameState={gs({ question: { ...gs().question, ROUE_MODE: 'SOLO' }, ROUE_TEAMS: ['Rouges'] })} actions={actionsMock()} />)
    expect(btn(/Passer la main/).disabled).toBe(true)
  })

  it('« Dévoiler » demande une confirmation avant d\'envoyer ROUE_REVEAL ; Annuler n\'envoie rien', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    fireEvent.click(btn(/Dévoiler la phrase/))
    expect(a.reveal).not.toHaveBeenCalled()
    fireEvent.click(btn(/Annuler/))
    expect(a.reveal).not.toHaveBeenCalled()
    fireEvent.click(btn(/Dévoiler la phrase/))
    fireEvent.click(btn(/Confirmer/))
    expect(a.reveal).toHaveBeenCalledTimes(1)
  })

  it('« Dévoiler » est inactif une fois la phrase résolue', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_STEP: 0, ROUE_WINNER: 'Rouges', ROUE_AWARDS: { Rouges: 300 } })} actions={actionsMock()} />)
    expect(btn(/Dévoiler la phrase/).disabled).toBe(true)
  })
})

describe('RouePanel — phrase solution et cagnottes', () => {
  it('l\'admin voit la phrase solution ; une lettre trouvée est marquée affichée', () => {
    const { container } = render(<RouePanel gameState={gs({ ROUE_LETTERS: [{ LETTER: 'N', TEAM: 'Rouges', HIT: true }] })} actions={actionsMock()} />)
    expect(container.querySelector('.roue-sol').textContent).toContain('BONJOUR')
    const n = Array.from(container.querySelectorAll('.roue-sol span')).filter(e => e.textContent === 'N')
    expect(n.length).toBeGreaterThan(0)
    expect(n.every(e => e.className.includes('rv'))).toBe(true)
  })

  it('sans phrase dans le payload (accès restreint) : pas de bloc solution, pas d\'exception', () => {
    const { container } = render(<RouePanel gameState={gs({ question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO' } })} actions={actionsMock()} />)
    expect(container.querySelector('.roue-sol')).toBeNull()
  })

  it('une carte de cagnotte par équipe ; l\'équipe qui a la main est signalée', () => {
    const { container } = render(<RouePanel gameState={gs()} actions={actionsMock()} />)
    const chips = container.querySelectorAll('.roue-bank-chip')
    expect(chips).toHaveLength(2)
    expect(container.querySelector('.roue-bank-chip.active').textContent).toMatch(/Rouges.*a la main/)
  })

  it('résolution : le message annonce le gagnant et le crédit', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_STEP: 0, ROUE_WINNER: 'Bleus', ROUE_AWARDS: { Bleus: 1500 } })} actions={actionsMock()} />)
    expect(document.body.textContent).toMatch(/Les Bleus ont trouvé la phrase.*1\s?500/)
  })

  it('dévoilée sans gagnant : message dédié', () => {
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_STEP: 0 })} actions={actionsMock()} />)
    expect(document.body.textContent).toMatch(/dévoilée sans gagnant/)
  })
})

describe('RouePanel — avant le départ et en pause', () => {
  const teamList = [{ name: 'Rouges', color: '#ef4444', ready: true }, { name: 'Bleus', color: '#3b82f6', ready: true }, { name: 'Verts', color: '#22c55e', ready: false }]

  it('PREPARE : cocher une équipe envoie ROUE_SET_TEAMS avec l\'ordre de passage', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ phase: 'PREPARE', ROUE_TEAMS: ['Bleus'] })} actions={a} teamList={teamList} />)
    fireEvent.click(screen.getByLabelText(/Rouges/))
    expect(a.setTeams).toHaveBeenLastCalledWith(['Bleus', 'Rouges'])
    fireEvent.click(screen.getByLabelText(/Bleus/))
    expect(a.setTeams).toHaveBeenLastCalledWith([])
  })

  it('SOLO : une seule équipe, cocher une autre remplace la sélection', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ phase: 'PREPARE', question: { ...gs().question, ROUE_MODE: 'SOLO' }, ROUE_TEAMS: ['Bleus'] })} actions={a} teamList={teamList} />)
    fireEvent.click(screen.getByLabelText(/Rouges/))
    expect(a.setTeams).toHaveBeenLastCalledWith(['Rouges'])
  })

  it('une équipe non prête n\'est pas sélectionnable', () => {
    render(<RouePanel gameState={gs({ phase: 'PREPARE', ROUE_TEAMS: [] })} actions={actionsMock()} teamList={teamList} />)
    expect(screen.getByLabelText(/Verts/).disabled).toBe(true)
  })

  it('les motifs d\'attente sont affichés', () => {
    render(<RouePanel gameState={gs({ phase: 'PREPARE', ROUE_TEAMS: [] })} actions={actionsMock()} teamList={teamList} waitReasons={['sélectionnez au moins deux équipes participantes']} />)
    expect(screen.getByText(/sélectionnez au moins deux équipes/)).toBeTruthy()
  })

  it('PAUSED : toutes les commandes sont désactivées', () => {
    render(<RouePanel gameState={gs({ phase: 'PAUSED' })} actions={actionsMock()} />)
    // le bouton d'étape est dans un <fieldset disabled> : :disabled (pas la propriété du bouton)
    expect(btn(/Lancer la roue/).matches(':disabled')).toBe(true)
    expect(btn(/Passer la main/).disabled).toBe(true)
    expect(btn(/Dévoiler la phrase/).disabled).toBe(true)
    expect(document.body.textContent).toMatch(/en pause/)
  })

  it('hors manche (STOPPED) : rien n\'est rendu', () => {
    const { container } = render(<RouePanel gameState={gs({ phase: 'STOPPED' })} actions={actionsMock()} />)
    expect(container.querySelector('[data-testid="roue-panel"]')).toBeNull()
  })
})

describe('RoueSteps — bandeau des 7 étapes', () => {
  it('7 étapes ; les précédentes cochées, la courante marquée', () => {
    const { container } = render(<RoueSteps step={4} />)
    const items = container.querySelectorAll('li')
    expect(items).toHaveLength(7)
    expect(Array.from(items).map(i => i.className.includes('done'))).toEqual([true, true, true, false, false, false, false])
    expect(items[3].getAttribute('aria-current')).toBe('step')
    expect(container.querySelectorAll('[aria-current="step"]')).toHaveLength(1)
  })
  it('étape 0 (hors tour) : aucune étape courante', () => {
    const { container } = render(<RoueSteps step={0} />)
    expect(container.querySelector('[aria-current="step"]')).toBeNull()
  })
  it('étape 7 : les six premières cochées', () => {
    const { container } = render(<RoueSteps step={7} />)
    expect(container.querySelectorAll('li.done')).toHaveLength(6)
  })
})

describe('RoueErrorLine — refus du serveur', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it.each([
    ['VOWEL_NOT_AVAILABLE', /Voyelle non disponible/], ['NOT_A_LETTER', /pas une lettre/], ['WRONG_SUBPHASE', /déjà prise en compte/],
    ['NOT_ALLOWED', /non autorisée/], ['INVALID_TEAMS', /non conforme/],
  ])('%s : message lisible', (code, re) => {
    render(<RoueErrorLine error={{ CODE: code, MESSAGE: 'x', SEQ: 1 }} />)
    expect(screen.getByRole('alert').textContent).toMatch(re)
  })

  it('code inconnu : repli sur le message serveur', () => {
    render(<RoueErrorLine error={{ CODE: 'ZZZ', MESSAGE: 'message du serveur', SEQ: 1 }} />)
    expect(screen.getByRole('alert').textContent).toBe('message du serveur')
  })

  it('disparaît après 4 s ; un nouveau refus identique (SEQ+1) réaffiche', () => {
    const { rerender } = render(<RoueErrorLine error={{ CODE: 'WRONG_SUBPHASE', MESSAGE: '', SEQ: 1 }} />)
    expect(screen.queryByRole('alert')).not.toBeNull()
    act(() => { vi.advanceTimersByTime(4100) })
    expect(screen.queryByRole('alert')).toBeNull()
    rerender(<RoueErrorLine error={{ CODE: 'WRONG_SUBPHASE', MESSAGE: '', SEQ: 2 }} />)
    expect(screen.queryByRole('alert')).not.toBeNull()
  })

  it('aucune erreur : rien', () => {
    const { container } = render(<RoueErrorLine error={null} />)
    expect(container.firstChild).toBeNull()
  })
})

describe('RoueKeyboard — consonnes', () => {
  it('20 consonnes proposées ; les 6 voyelles désactivées et jamais sélectionnables', () => {
    const onSelect = vi.fn()
    const { container } = render(<RoueKeyboard enabled onSelect={onSelect} />)
    expect(container.querySelectorAll('.roue-kbd .roue-key')).toHaveLength(20)
    const vowels = Array.from(container.querySelectorAll('.roue-vow .roue-key'))
    expect(vowels.map(v => v.textContent).sort()).toEqual(['A', 'E', 'I', 'O', 'U', 'Y'])
    expect(vowels.every(v => v.disabled)).toBe(true)
  })

  it('désactivé (hors étape 4) : aucune touche cliquable, le clavier physique est ignoré', () => {
    const onSelect = vi.fn()
    const { container } = render(<RoueKeyboard enabled={false} onSelect={onSelect} />)
    expect(Array.from(container.querySelectorAll('.roue-kbd .roue-key')).every(k => k.disabled)).toBe(true)
    fireEvent.keyDown(window, { key: 'n' })
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('la frappe dans un champ de saisie ne sélectionne pas de lettre', () => {
    const onSelect = vi.fn()
    render(<div><input data-testid="i" /><RoueKeyboard enabled onSelect={onSelect} /></div>)
    fireEvent.keyDown(screen.getByTestId('i'), { key: 'n' })
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('consonnes jouées : trouvée / absente distinguées, colorées par équipe si demandé', () => {
    const { container } = render(<RoueKeyboard enabled letters={[{ LETTER: 'T', TEAM: 'Rouges', HIT: true }, { LETTER: 'Z', TEAM: 'Bleus', HIT: false }]}
      teamColor={(t) => (t === 'Rouges' ? [239, 68, 68] : [59, 130, 246])} />)
    const t = screen.getByRole('button', { name: /^T/ })
    const z = screen.getByRole('button', { name: /^Z/ })
    expect(t.className).toMatch(/hit/); expect(z.className).toMatch(/miss/)
    expect(t.disabled && z.disabled).toBe(true)
    expect(t.style.getPropertyValue('--c')).toMatch(/239/)
    expect(container.textContent).toMatch(/Rouges/)
  })
})

describe('RoueNextButtons — portes', () => {
  it('sans sous-phase de fin de tour : les deux boutons sont désactivés', () => {
    const g = gs({ ROUE_SUBPHASE: 'SPINNING' })
    render(<RoueNextButtons gs={g} gates={roueGates(g)} onNext={() => {}} />)
    expect(btn(/Tour suivant/).disabled).toBe(true)
    expect(btn(/Équipe suivante/).disabled).toBe(true)
  })
})
