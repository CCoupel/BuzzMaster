import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { calcQcmTeamAward, resolvePointsAward } from '../utils/pointsAward'

// ---------------------------------------------------------------------------
// AnimPage — page animateur (#155/#156/#157/#163/#165/#166)
//
// Gabarit #166 (F6) : zone contexte (bandeau : méta, énoncé, AnimAnswerZone,
// chrono) / zone conduite (AnimConductPanel, cinq lignes permanentes) / zone
// équipes (AnimTeamCard enrichie, INCHANGÉE par #166) / bande régie
// (réservée, #167).
//
// AnimConductPanel a désormais sa PROPRE couverture exhaustive de la
// matrice d'état (AnimConductPanel.test.jsx, #166/T6 — 10 phases × 5
// boutons + "à suivre") et AnimAnswerZone la sienne (AnimAnswerZone.test.jsx,
// #166/T7). Depuis #166/F5, ces deux composants ne reçoivent plus de
// booléens précalculés par AnimPage (isPlaying/canStart/canReveal ont
// disparu — AnimConductPanel dérive tout lui-même via phaseRules.js) :
// tester leur RENDU via AnimPage serait une duplication fragile de T6/T7.
// Les deux sont donc MOCKÉS ici (même principe que Timer, déjà mocké avant
// #166) — ce fichier vérifie uniquement le CÂBLAGE (les bonnes props et les
// bons callbacks leur arrivent) et les particularités propres à AnimPage
// (calcul du crédit, TIME/creditPoints de démarrage, dérivation des chips
// de la ligne méta).
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    constructor() { this.isEnabled = false }
    enable() { this.isEnabled = true; return Promise.resolve() }
    disable() { this.isEnabled = false }
  },
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime, phase }) => <div data-testid="timer" data-phase={phase}>{currentTime}</div>,
}))

// #166/F5 — mock de câblage : AnimConductPanel a sa propre couverture
// exhaustive (AnimConductPanel.test.jsx, T6). Ici on capture les props
// reçues (data-attributes) et on expose un bouton par callback pour
// vérifier le déclenchement, sans dépendre du rendu réel (matrice d'état).
vi.mock('../components/AnimConductPanel', () => ({
  default: (props) => (
    <div
      data-testid="conduct-panel"
      data-phase={props.phase}
      data-question-id={props.question?.ID ?? ''}
      data-revealed={String(!!props.revealed)}
      data-next-id={props.nextQuestion?.ID ?? ''}
      data-qcm-invalidated={JSON.stringify(props.qcmInvalidated ?? [])}
      data-memory={JSON.stringify(props.memory ?? null)}
      data-motion={JSON.stringify(props.motion ?? null)}
    >
      <button onClick={props.onStart}>MOCK_START</button>
      <button onClick={props.onPause}>MOCK_PAUSE</button>
      <button onClick={props.onContinue}>MOCK_CONTINUE</button>
      <button onClick={props.onStop}>MOCK_STOP</button>
      <button onClick={props.onReveal}>MOCK_REVEAL</button>
      <button onClick={() => props.onSelectNext('9')}>MOCK_SELECT_NEXT</button>
      {/* #159/T3 — câblage du geste MEMORY vers AnimConductPanel (qui
          délègue lui-même à AnimMemoryGrid, cf. AnimConductPanel.test.jsx
          pour la matrice L3 propre à ce composant). */}
      <button onClick={() => props.onFlipMemoryCard?.('1-1')}>MOCK_FLIP_MEMORY_CARD</button>
      {/* #160/T7 — câblage des 5 gestes MEMOTION vers AnimConductPanel (qui
          délègue à AnimMotionGrid/AnimMotionCard/AnimMotionActions, cf.
          AnimConductPanel.test.jsx pour la matrice L2/L3 propre à ce
          composant). Extension ADDITIVE de ce mock (les boutons/attributs
          MEMORY/QCM ci-dessus sont inchangés) — même principe que
          l'extension MEMORY faite pour #159. */}
      <button onClick={() => props.onSelectMotionCard?.('c1')}>MOCK_SELECT_MOTION_CARD</button>
      <button onClick={() => props.onFlipMotionCard?.()}>MOCK_FLIP_MOTION_CARD</button>
      <button onClick={() => props.onStopMotionTimer?.()}>MOCK_STOP_MOTION_TIMER</button>
      <button onClick={() => props.onRevealMotionCard?.()}>MOCK_REVEAL_MOTION_CARD</button>
      <button onClick={() => props.onDoneMotionCard?.('c1', 'Les Bleus')}>MOCK_DONE_MOTION_CARD</button>
    </div>
  ),
}))

// #166/F10 — même principe : AnimAnswerZone a sa propre couverture
// exhaustive (AnimAnswerZone.test.jsx, T7).
vi.mock('../components/AnimAnswerZone', () => ({
  default: (props) => (
    <div
      data-testid="answer-zone"
      data-question-id={props.question?.ID ?? ''}
      data-revealed={String(!!props.revealed)}
    />
  ),
}))

// #158/F3 — même principe : AnimArdoiseList a sa propre couverture
// exhaustive (AnimArdoiseList.test.jsx, T3).
vi.mock('../components/AnimArdoiseList', () => ({
  default: (props) => (
    <div
      data-testid="ardoise-list"
      data-entries={JSON.stringify((props.entries || []).map(e => e.teamName))}
      data-question-id={props.question?.ID ?? ''}
      data-game-time={String(props.gameTime ?? '')}
      data-credit-points={String(props.creditPoints ?? '')}
      data-revealed={String(!!props.revealed)}
      data-awarded={JSON.stringify(props.awardedTeams || {})}
    >
      <button onClick={() => props.onCredit('MOCK_TEAM', 3)}>MOCK_ARDOISE_CREDIT</button>
    </div>
  ),
}))

function makeGameMock(overrides = {}) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STOPPED',
      timer: 30,
      totalTime: 30,
      gameTime: 0,
      question: null,
      ...overrides.gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    // #166/F1 — progression dans le quiz (question COURANTE) ;
    // { position: 0, total: 0 } = pas encore reçu, même convention que
    // creditPoints ci-dessous. Sans ce défaut, AnimPage.jsx plante dès
    // qu'un test fournit `gameState.question` sans surcharger
    // `questionPosition` (accès direct `questionPosition.total`, sans
    // chaînage optionnel, dans la branche question chargée).
    questionPosition: { position: 0, total: 0 },
    // #170/F1 — équipes déjà créditées pour la question courante, source de
    // vérité du verrouillage AnimCreditControl. Défaut à {} (aucun crédit
    // encore reçu), même convention que questionPosition ci-dessus : sans
    // ce défaut, AnimPage.jsx plante dès qu'un test rend une équipe créditable
    // (accès direct `awardedTeams[team.name]`, sans chaînage optionnel).
    awardedTeams: {},
    // MAJEUR-1 — creditPoints (CREDIT_POINTS) est l'équivalent serveur de
    // pointsInput sur /admin, PAS question.POINTS brut. Défaut à 0 comme
    // l'état initial réel de useWebSocket.js (avant tout CREDIT_POINTS reçu).
    creditPoints: 0,
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setTeamPoints: vi.fn(),
    setBumperPoints: vi.fn(),
    // #159/T3 — flipMemoryCard (flipMemoryCard action), câblé vers
    // AnimConductPanel comme onFlipMemoryCard. vi.fn() par défaut comme les
    // autres callbacks ci-dessus (aucun changement pour les tests
    // pré-existants qui ne l'exercent pas).
    flipMemoryCard: vi.fn(),
    // v6.4.x (#167) — messagerie régie : état par défaut = repos (aucun
    // message actif), même convention que questionPosition/awardedTeams
    // ci-dessus. AnimPage.jsx porte de toute façon ce même défaut en
    // paramètre par défaut de déstructuration ; l'exposer ici aussi permet
    // aux tests de cliquer sur « Vu » sans le répéter à chaque fois.
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    // #160/F2 — les 5 émetteurs MEMOTION de useWebSocket.js (useGame()),
    // vi.fn() par défaut (extension ADDITIVE, comme flipMemoryCard
    // ci-dessus) : aucun changement pour les tests pré-existants qui ne les
    // exercent pas.
    selectMotionCard: vi.fn(),
    flipMotionCard: vi.fn(),
    stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(),
    doneMotionCard: vi.fn(),
    ...overrides,
  }
}

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
  useCategories.mockReturnValue({ categories: [] })
})

afterEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Zone A — contexte : question courante, statut, question suivante
// ---------------------------------------------------------------------------

describe('AnimPage — Zone A (contexte), ligne méta (#166/F3)', () => {
  it('affiche "Aucune question en cours" quand aucune question n\'est chargée', () => {
    render(<AnimPage />)
    expect(screen.getByText('Aucune question en cours')).toBeInTheDocument()
  })

  it('affiche ID et TYPE (libellé questionTypeMeta) de la question courante', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', timer: 20, totalTime: 30, question: { ID: '42', TYPE: 'QCM', CATEGORY: 'SCIENCE' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('#42')).toBeInTheDocument()
    expect(screen.getByText('QCM')).toBeInTheDocument()
  })

  // #166/F3 — questionTypeMeta.js remplace le repli textuel brut
  // `question.TYPE || 'SPEEDY'` (#163) par getQuestionTypeMeta, dont le
  // repli est { icon: '⚡', label: 'Speedy' } — casse en title case, PAS
  // "SPEEDY" tout capitales comme avant #166.
  it('replie sur le libellé "Speedy" (questionTypeMeta) quand TYPE est absent', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', timer: 20, totalTime: 30, question: { ID: '1' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Speedy')).toBeInTheDocument()
    expect(screen.queryByText('SPEEDY')).not.toBeInTheDocument()
  })

  it('affiche le statut de connexion', () => {
    useGame.mockReturnValue(makeGameMock({ status: 'disconnected' }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-connection-status.disconnected')).not.toBeNull()
    expect(screen.getByText('Déconnecté')).toBeInTheDocument()
  })

  it('affiche le chip catégorie (icône + libellé) quand la question en a une', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', CATEGORY: 'HISTORY' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Histoire')).toBeInTheDocument()
  })

  // #166/F1 — progression n/total (CURRENT_POSITION/TOTAL_QUESTIONS),
  // contracts/CHANGELOG.md [20260815-1]. N'affiche rien tant que le serveur
  // n'a pas renseigné TOTAL_QUESTIONS (0 = jamais reçu) — pas de "0/0"
  // trompeur avant le premier NEXT_QUESTION.
  it('affiche la progression n/total quand questionPosition.total > 0', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } },
      questionPosition: { position: 7, total: 12 },
    }))
    render(<AnimPage />)
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByText('/12')).toBeInTheDocument()
  })

  // #171/F1 — sélecteur corrigé : le chip de progression est passé de
  // `.anim-question-counter` (classe seule) à `.anim-chip.anim-chip-count`
  // (promu à la même famille que les autres chips de la ligne méta).
  // L'ancien sélecteur ne matchait plus RIEN sur la page après #171,
  // rendant ce test vacuously vrai (il aurait "passé" même si la
  // progression restait affichée par erreur) — corrigé pour rester un test
  // réel, pas neutralisé.
  it("n'affiche pas de progression tant que total vaut 0 (jamais reçu)", () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } },
      questionPosition: { position: 0, total: 0 },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-chip-count')).toBeNull()
  })

  it('affiche les chips d\'options conditionnelles : cible des points, indices QCM, mode de tour', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'QCM', POINTS_TARGET: 'TEAM', QCM_HINTS_ENABLED: true },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Équipe')).toBeInTheDocument()
    expect(screen.getByText('Indices')).toBeInTheDocument()
  })

  it('chip "Indices" absente si QCM_HINTS_ENABLED est faux, même en QCM', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: false } },
    }))
    render(<AnimPage />)
    expect(screen.queryByText('Indices')).not.toBeInTheDocument()
  })

  it('chip mode de tour affichée pour MEMORY/MEMOTION avec MEMORY_MODE renseigné (modes pas encore conduits depuis /anim, mais la donnée existe déjà côté modèle)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'MEMORY', MEMORY_MODE: 'CHACUN_SON_TOUR' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Chacun son tour')).toBeInTheDocument()
  })

  it('passe currentTime/totalTime/phase au Timer', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'PAUSED', timer: 12, totalTime: 30, question: null },
    }))
    render(<AnimPage />)
    const timer = screen.getByTestId('timer')
    expect(timer).toHaveAttribute('data-phase', 'PAUSED')
    expect(timer.textContent).toBe('12')
  })
})

// ---------------------------------------------------------------------------
// #166/T9, RÉVISÉ #171/T1 — ordre des chips de la ligne méta + placement du
// chronomètre (E6 = option i, colonne du bandeau). #171 change l'ordre
// (n/total passe EN TÊTE des chips, avant catégorie) et promeut #ID au rang
// de "titre" (même taille que les autres chips, classe `.anim-chip-title`)
// — l'ancien #ID "en retrait" (`.anim-question-id`, texte discret) est
// SUPPRIMÉ, pas conservé en double (plan §2 : "un seul #ID, pas deux").
// Réécrit, pas neutralisé : même couverture (ordre + unicité), contrat à
// jour.
// ---------------------------------------------------------------------------

describe('AnimPage — Zone A, ordre des chips et chronomètre en colonne (#166/T9, #171/T1)', () => {
  it('ordonne les chips : statut · n/total · catégorie · type · #ID · options · points', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '12', TYPE: 'QCM', CATEGORY: 'HISTORY', POINTS_TARGET: 'TEAM', QCM_HINTS_ENABLED: true, POINTS: '10' },
      },
      questionPosition: { position: 7, total: 12 },
    }))
    const { container } = render(<AnimPage />)
    const text = container.querySelector('.anim-meta-row').textContent
    const posConnecte = text.indexOf('Connecté')
    const posCounter = text.indexOf('7/12') !== -1 ? text.indexOf('7/12') : text.indexOf('7')
    const posHistoire = text.indexOf('Histoire')
    const posQcm = text.indexOf('QCM')
    const posId = text.indexOf('#12')
    const posOption = text.indexOf('Équipe')
    const posPoints = text.indexOf('10pt')

    expect(posConnecte).toBeGreaterThanOrEqual(0)
    expect(posCounter).toBeGreaterThan(posConnecte)
    expect(posHistoire).toBeGreaterThan(posCounter)
    expect(posQcm).toBeGreaterThan(posHistoire)
    expect(posId).toBeGreaterThan(posQcm)
    expect(posOption).toBeGreaterThan(posId)
    expect(posPoints).toBeGreaterThan(posOption)
  })

  // #171/T1 — #ID promu "titre" : même famille de chip que les autres
  // (`.anim-chip.anim-chip-title`), plus de texte "en retrait" séparé.
  it('#ID est rendu comme un chip à part entière (`.anim-chip-title`), même famille que les autres chips', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '99', TYPE: 'SPEEDY' } },
    }))
    const { container } = render(<AnimPage />)
    const idEl = screen.getByText('#99')
    expect(idEl.className).toMatch(/\banim-chip\b/)
    expect(idEl.className).toMatch(/\banim-chip-title\b/)
  })

  // #171/T1, R7 — un seul #ID sur la page (l'ancien exemplaire "en retrait"
  // est supprimé, pas dupliqué à côté du nouveau chip).
  it('#ID apparaît UNE SEULE FOIS sur la page (pas de doublon)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '42', TYPE: 'SPEEDY' } },
    }))
    render(<AnimPage />)
    expect(screen.getAllByText('#42')).toHaveLength(1)
  })

  it('le statut de connexion garde sa taille actuelle (pas la classe des chips agrandis)', () => {
    useGame.mockReturnValue(makeGameMock({ status: 'connected' }))
    const { container } = render(<AnimPage />)
    const statusEl = container.querySelector('.anim-connection-status')
    expect(statusEl).not.toBeNull()
    expect(statusEl.className).not.toMatch(/anim-chip/)
  })

  it('le chronomètre est monté dans sa colonne dédiée (E6 = option i), pas dans la ligne méta', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', timer: 20, totalTime: 30, question: { ID: '1' } },
    }))
    const { container } = render(<AnimPage />)
    const chronoCol = container.querySelector('.anim-chrono-col')
    expect(chronoCol).not.toBeNull()
    expect(chronoCol.querySelector('[data-testid="timer"]')).not.toBeNull()
    // Absent de la ligne méta elle-même.
    expect(container.querySelector('.anim-meta-row [data-testid="timer"]')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Zone conduite (#166/F5) : câblage vers AnimConductPanel (mocké — sa
// matrice d'état est couverte exhaustivement par AnimConductPanel.test.jsx,
// T6). Ici : les bonnes props lui arrivent, et ses callbacks déclenchent
// les bonnes actions useGame() avec les bons arguments.
// ---------------------------------------------------------------------------

describe('AnimPage — zone conduite, câblage vers AnimConductPanel (#166/F5)', () => {
  it('transmet phase/question/qcmInvalidated/revealed/nextQuestion tels quels', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'REVEALED', question: { ID: '1', TYPE: 'QCM' }, qcmInvalidated: ['YELLOW'] },
      nextQuestion: { ID: '9' },
    }))
    render(<AnimPage />)
    const panel = screen.getByTestId('conduct-panel')
    expect(panel).toHaveAttribute('data-phase', 'REVEALED')
    expect(panel).toHaveAttribute('data-question-id', '1')
    expect(panel).toHaveAttribute('data-revealed', 'true')
    expect(panel).toHaveAttribute('data-next-id', '9')
    expect(panel).toHaveAttribute('data-qcm-invalidated', JSON.stringify(['YELLOW']))
  })

  it('revealed=false hors phase REVEALED', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STARTED', question: { ID: '1' } } }))
    render(<AnimPage />)
    expect(screen.getByTestId('conduct-panel')).toHaveAttribute('data-revealed', 'false')
  })

  it('LANCER (onStart) envoie startGame avec TIME de la question et creditPoints (MAJEUR-1), pas question.POINTS', () => {
    const props = makeGameMock({
      gameState: { phase: 'READY', question: { ID: '1', TIME: '45', POINTS: '3' } },
      creditPoints: 7, // ajusté côté admin, diverge délibérément de question.POINTS (3)
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_START').click()
    expect(props.startGame).toHaveBeenCalledWith(45, 7)
  })

  it('LANCER replie sur 30s/1pt si TIME est absent et creditPoints vaut 0', () => {
    const props = makeGameMock({ gameState: { phase: 'READY', question: { ID: '1' } }, creditPoints: 0 })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_START').click()
    expect(props.startGame).toHaveBeenCalledWith(30, 1)
  })

  it('onSelectNext (À suivre) envoie selectQuestion avec l\'ID transmis', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_SELECT_NEXT').click() // mock appelle onSelectNext('9')
    expect(props.selectQuestion).toHaveBeenCalledWith('9')
  })

  it('onStop/onPause/onContinue/onReveal appellent les actions useGame() correspondantes', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_STOP').click()
    expect(props.stopGame).toHaveBeenCalledTimes(1)
    screen.getByText('MOCK_PAUSE').click()
    expect(props.pauseGame).toHaveBeenCalledTimes(1)
    screen.getByText('MOCK_CONTINUE').click()
    expect(props.continueGame).toHaveBeenCalledTimes(1)
    screen.getByText('MOCK_REVEAL').click()
    expect(props.revealAnswer).toHaveBeenCalledTimes(1)
  })
})

describe('AnimPage — zone contexte, câblage vers AnimAnswerZone (#166/F10)', () => {
  it('transmet question et revealed tels quels', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'REVEALED', question: { ID: '5' } },
    }))
    render(<AnimPage />)
    const zone = screen.getByTestId('answer-zone')
    expect(zone).toHaveAttribute('data-question-id', '5')
    expect(zone).toHaveAttribute('data-revealed', 'true')
  })

  it('question absente : data-question-id vide, revealed=false hors REVEALED', () => {
    render(<AnimPage />)
    const zone = screen.getByTestId('answer-zone')
    expect(zone).toHaveAttribute('data-question-id', '')
    expect(zone).toHaveAttribute('data-revealed', 'false')
  })
})

// ---------------------------------------------------------------------------
// #171/T2 — statut de partie : la pastille de phase déménage de la colonne
// chrono (Timer, `showPhase={false}` désormais) vers la ligne réponse
// (`.anim-answer-row`, juste avant AnimAnswerZone, mêmes classes/libellés
// que Timer.jsx via utils/phaseBadge.js). Timer lui-même n'est pas mocké
// dans ce fichier (voir mock plus haut : seuls currentTime/phase sont
// exposés via data-attributes) — on vérifie ici que la ligne réponse porte
// la pastille et que le mock Timer ne la porte plus (elle ne pourrait de
// toute façon pas la rendre, le mock ne rend que currentTime).
// ---------------------------------------------------------------------------

describe('AnimPage — statut de partie sur la ligne réponse (#171/T2)', () => {
  it.each([
    ['STOPPED', 'ARRET'],
    ['PAUSED', 'PAUSE'],
    ['STARTED', 'EN COURS'],
    ['PREPARE', 'PREPARATION'],
    ['READY', 'PRET'],
    ['REVEALED', 'REPONSE'],
  ])('phase %s : pastille "%s" rendue dans .anim-answer-row', (phase, label) => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase, question: { ID: '1' } },
    }))
    const { container } = render(<AnimPage />)
    const row = container.querySelector('.anim-answer-row')
    expect(row).not.toBeNull()
    expect(row.querySelector('.phase-badge')).not.toBeNull()
    expect(row.textContent).toContain(label)
  })

  it('aucune pastille pour une phase sans badge dédié (NEW_GAME)', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'NEW_GAME', question: { ID: '1' } } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-answer-row .phase-badge')).toBeNull()
  })

  it('la pastille précède AnimAnswerZone dans .anim-answer-row (juste avant la zone réponse)', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STARTED', question: { ID: '1' } } }))
    const { container } = render(<AnimPage />)
    const row = container.querySelector('.anim-answer-row')
    const badge = row.querySelector('.phase-badge')
    const zone = screen.getByTestId('answer-zone')
    // eslint-disable-next-line no-bitwise
    expect(badge.compareDocumentPosition(zone) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})

// ---------------------------------------------------------------------------
// #158/T4 — mode ARDOISE : AnimArdoiseList remplace les cartes équipe
// (mockée — sa propre couverture exhaustive vit dans
// AnimArdoiseList.test.jsx, T3). Phases STARTED/PAUSED/STOPPED/REVEALED
// uniquement ; filtre équipes à joueur virtuel (parité #93) ; le crédit
// cible TOUJOURS l'équipe (setTeamPoints direct, pas de résolution
// PLAYER/bumper comme SPEEDY).
// ---------------------------------------------------------------------------

describe('AnimPage — mode ARDOISE, câblage vers AnimArdoiseList (#158/T4)', () => {
  function ardoiseMock(phase, overrides = {}) {
    return makeGameMock({
      gameState: {
        phase,
        question: { ID: '1', TYPE: 'ARDOISE' },
        gameTime: 10_000_000,
        ARDOISE_ANSWERS: { 'Les Rouges': { TEXT: 'Réponse', STARTED_AT: 12_000_000 } },
        ...overrides.gameState,
      },
      creditPoints: 3,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 }, 'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 } },
      bumpers: {
        r1: { TEAM: 'Les Rouges', IS_VPLAYER: true },
        b1: { TEAM: 'Les Bleus' }, // pas VJoueur — exclue de la liste ARDOISE
      },
      ...overrides,
    })
  }

  it.each(['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'])(
    'phase %s : AnimArdoiseList remplace les cartes équipe',
    (phase) => {
      useGame.mockReturnValue(ardoiseMock(phase))
      render(<AnimPage />)
      expect(screen.getByTestId('ardoise-list')).toBeInTheDocument()
      expect(screen.queryByText('Les Rouges')).not.toBeInTheDocument() // pas de carte équipe classique
    }
  )

  it.each(['READY', 'PREPARE', 'NEW_GAME'])(
    'phase %s (hors STARTED/PAUSED/STOPPED/REVEALED) : cartes équipe normales, pas de liste ARDOISE',
    (phase) => {
      useGame.mockReturnValue(ardoiseMock(phase))
      render(<AnimPage />)
      expect(screen.queryByTestId('ardoise-list')).not.toBeInTheDocument()
    }
  )

  it('hors ARDOISE (SPEEDY/QCM) : cartes équipe normales même dans les phases STARTED/STOPPED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'SPEEDY' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', IS_VPLAYER: true } },
    }))
    render(<AnimPage />)
    expect(screen.queryByTestId('ardoise-list')).not.toBeInTheDocument()
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
  })

  it('ne transmet que les équipes à joueur virtuel (parité #93) — Les Bleus (sans VJoueur) exclue', () => {
    useGame.mockReturnValue(ardoiseMock('STOPPED'))
    render(<AnimPage />)
    const entries = JSON.parse(screen.getByTestId('ardoise-list').dataset.entries)
    expect(entries).toEqual(['Les Rouges'])
  })

  it('transmet question/gameTime/creditPoints/revealed/awardedTeams tels quels', () => {
    useGame.mockReturnValue(ardoiseMock('REVEALED', {
      awardedTeams: { 'Les Rouges': { POINTS: 3, TIMESTAMP: 1000 } },
    }))
    render(<AnimPage />)
    const el = screen.getByTestId('ardoise-list')
    expect(el).toHaveAttribute('data-question-id', '1')
    expect(el).toHaveAttribute('data-game-time', '10000000')
    expect(el).toHaveAttribute('data-credit-points', '3')
    expect(el).toHaveAttribute('data-revealed', 'true')
    expect(JSON.parse(el.dataset.awarded)).toEqual({ 'Les Rouges': { POINTS: 3, TIMESTAMP: 1000 } })
  })

  it('onCredit cible TOUJOURS l\'équipe via setTeamPoints (mirror exact de /admin, pas de résolution PLAYER/bumper)', () => {
    const props = ardoiseMock('REVEALED')
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_ARDOISE_CREDIT').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('MOCK_TEAM', 3)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// #166/T10 — disposition : bande régie réservée (#166/F8, #167), présente,
// vide, sans interaction, dans toutes les phases. Colonne équipes pleine
// hauteur : la mesure de hauteur réelle relève de T12 (QA, jsdom ne fait
// pas de layout) — ici on vérifie que la zone équipes existe bien comme
// zone de grille dédiée (`.anim-zone-teams`), inchangée par #166 (seul son
// `grid-area` change, AnimPage.css — non testable en jsdom, T12).
// ---------------------------------------------------------------------------

// La bande régie était réservée (texte statique "Messagerie régie", aucun
// élément interactif) avant #167. Le lot v6.4.x #167 lui donne un contenu
// réel — réception du message actif + bouton « Vu » (F4, câblé sur
// clearRegieMessage). RÉÉCRIT (pas neutralisé, changement documenté : plan
// _work/reports/plan-20260818-121500.md, tâche F4) : mêmes phases
// couvertes, mais sur le comportement réel plutôt que la réserve vide.
describe('AnimPage — bande régie, réception du message (#167, F4)', () => {
  it.each(['NEW_GAME', 'STARTED', 'REVEALED'])(
    'phase %s : aucun message actif -> état repos, sans élément interactif',
    (phase) => {
      useGame.mockReturnValue(makeGameMock({ gameState: { phase, question: null } }))
      const { container } = render(<AnimPage />)
      const bar = container.querySelector('.anim-zone-regie .anim-regie-bar')
      expect(bar).not.toBeNull()
      expect(bar.textContent).toContain('Aucun message de la régie')
      expect(container.querySelector('.anim-zone-regie button')).toBeNull()
      expect(container.querySelector('.anim-zone-regie input')).toBeNull()
    }
  )

  it('message actif : texte affiché et bouton « Vu », quelle que soit la phase de jeu', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: null },
      regieMessage: { ACTIVE: true, TEXT: 'Question 12 annulée', SENT_AT: 1, CLEARED_BY: '' },
    }))
    const { container } = render(<AnimPage />)
    const bar = container.querySelector('.anim-zone-regie .anim-regie-bar')
    expect(bar.textContent).toContain('Question 12 annulée')
    expect(screen.getByText('Vu')).toBeInTheDocument()
  })

  it('cliquer sur « Vu » appelle clearRegieMessage (acquittement, AC2/AC3)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: null },
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('Vu').click()
    expect(props.clearRegieMessage).toHaveBeenCalledTimes(1)
  })

  it('AUCUNE transition de jeu (NEW_GAME) n\'efface le message affiché — l\'état vient exclusivement de regieMessage (AC12)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'NEW_GAME', question: null },
      regieMessage: { ACTIVE: true, TEXT: 'Consigne persistante', SENT_AT: 1, CLEARED_BY: '' },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Vu')).toBeInTheDocument()
    expect(screen.getByText('Consigne persistante')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — équipes : carte de base (nom, couleur, score), équipes sans
// joueur masquées (même règle de base que /admin, #45). INCHANGÉE par
// #166/T10 — AnimTeamCard.test.jsx reste vert SANS LA MOINDRE MODIFICATION
// (fichier non touché depuis #155, vérifié : seule la grid-area de
// `.anim-zone-teams` change, AnimPage.css).
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C (équipes)', () => {
  it('affiche les équipes ayant au moins un joueur, avec leur score', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 15 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 8 },
      },
      bumpers: {
        mac1: { TEAM: 'Les Rouges' },
        mac2: { TEAM: 'Les Bleus' },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText('Les Bleus')).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
  })

  it('masque une équipe sans joueur assigné', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 15 },
        'Equipe Vide': { COLOR: [0, 255, 0], SCORE: 0 },
      },
      bumpers: {
        mac1: { TEAM: 'Les Rouges' },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.queryByText('Equipe Vide')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — ordre de buzz, rang, temps de réaction (#156/F6)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, ordre de buzz (#156/F6)', () => {
  function buzzGameMock(phase) {
    return makeGameMock({
      gameState: { phase, gameTime: 1_000_000, question: { ID: '1', TYPE: 'SPEEDY' } },
      teams: {
        'Lente': { COLOR: [0, 0, 255], SCORE: 0, TIME: 3_000_000 },
        'Rapide': { COLOR: [255, 0, 0], SCORE: 0, TIME: 1_500_000 },
        'PasBuzze': { COLOR: [0, 255, 0], SCORE: 0, TIME: 0 },
      },
      bumpers: {
        m1: { TEAM: 'Lente' },
        m2: { TEAM: 'Rapide' },
        m3: { TEAM: 'PasBuzze' },
      },
    })
  }

  it('réordonne les équipes par TIME croissant pendant STARTED', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    expect(names).toEqual(['Rapide', 'Lente', 'PasBuzze'])
  })

  it('affiche le badge de rang (🏆) sur la première équipe en STARTED', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    render(<AnimPage />)
    expect(screen.getByText('🏆')).toBeInTheDocument()
  })

  it('masque le badge de rang en STOPPED (mais garde le tri)', () => {
    useGame.mockReturnValue(buzzGameMock('STOPPED'))
    const { container } = render(<AnimPage />)
    expect(screen.queryByText('🏆')).not.toBeInTheDocument()
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    expect(names).toEqual(['Rapide', 'Lente', 'PasBuzze'])
  })

  it('affiche le temps de réaction formaté pour une équipe ayant buzzé', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    render(<AnimPage />)
    // Rapide : TIME 1_500_000, gameTime 1_000_000 → 0.500s
    expect(screen.getByText('0.500s')).toBeInTheDocument()
  })

  it('hors des phases actives (READY), pas de tri ni de temps de réaction', () => {
    useGame.mockReturnValue(buzzGameMock('READY'))
    const { container } = render(<AnimPage />)
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    // Ordre d'objet d'origine (Lente, Rapide, PasBuzze), pas de tri
    expect(names).toEqual(['Lente', 'Rapide', 'PasBuzze'])
    expect(screen.queryByText('0.500s')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — crédit (#156/F6)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, crédit (#156/F6)', () => {
  it('aucun bouton de crédit avant l\'arrêt de la question (ex: STARTED)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
    }))
    render(<AnimPage />)
    expect(screen.queryByText(/pts/)).not.toBeInTheDocument()
  })

  it('STOPPED et REVEALED : bouton de crédit visible avec le montant de creditPoints', () => {
    ;['STOPPED', 'REVEALED'].forEach(phase => {
      useGame.mockReturnValue(makeGameMock({
        gameState: { phase, question: { ID: '1', POINTS: '5' } },
        creditPoints: 5,
        teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
        bumpers: { m1: { TEAM: 'Les Rouges' } },
      }))
      const { unmount } = render(<AnimPage />)
      expect(screen.getByText('+5 pts')).toBeInTheDocument()
      unmount()
    })
  })

  // MAJEUR-1 — scénario exact de la revue de code : question sélectionnée à
  // 10 points, l'admin ajuste pointsInput à 20 sans resélectionner (donc
  // question.POINTS reste 10, seul creditPoints — rediffusé par
  // CREDIT_POINTS — reflète l'ajustement). /anim doit créditer 20, pas 10.
  it('crédite creditPoints (ajusté par l\'admin), jamais question.POINTS brut', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '10' } },
      creditPoints: 20,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    expect(screen.getByText('+20 pts')).toBeInTheDocument()
    expect(screen.queryByText('+10 pts')).not.toBeInTheDocument()

    screen.getByText('+20 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('m1', 20)
  })

  it('POINTS_TARGET absent (PLAYER) : crédite le bumper le plus rapide de l\'équipe', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0, TIME: 2000 } },
      bumpers: {
        slow: { TEAM: 'Les Rouges', TIME: 5000 },
        fast: { TEAM: 'Les Rouges', TIME: 2000 },
      },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('+5 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('fast', 5)
    expect(props.setTeamPoints).not.toHaveBeenCalled()
  })

  it('POINTS_TARGET=TEAM : crédite l\'équipe directement', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5', POINTS_TARGET: 'TEAM' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('+5 pts').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 5)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it('replie sur 1 point si creditPoints vaut 0 (aucun CREDIT_POINTS encore reçu)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 0,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+1 pts')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #171/F4/F6, T7 — crédit universel : AnimCreditControl monté pour TOUTE
// équipe dès `creditEnabled` (plus de gate sur rang/temps/réponse QCM).
// `canAwardPoints` (T5) décide seulement si "+N pts" s'ajoute à "0 pt"
// (toujours proposé, #170) — jamais si le contrôle est monté. R3 : le
// verrouillage #170 (awardedTeams) reste la SEULE source de vérité pour
// l'état verrouillé, orthogonal à "a tenté" — une équipe créditée par la
// régie sans avoir buzzé doit rester verrouillée.
// ---------------------------------------------------------------------------

describe('AnimPage — crédit universel et verrouillage (#171/F4/F6, T7)', () => {
  it('SPEEDY : "0 pt" proposé même à une équipe qui n\'a pas buzzé, "+N pts" absent, motif "pas de buzz"', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'SPEEDY', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 0 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.queryByText('+5 pts')).not.toBeInTheDocument()
    expect(screen.getByText('pas de buzz')).toBeInTheDocument()
  })

  it('QCM : "0 pt" proposé même à une équipe qui n\'a pas répondu, motif "pas de réponse"', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'QCM', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 0 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.getByText('pas de réponse')).toBeInTheDocument()
  })

  // #159 — MEMORY est désormais câblé explicitement (isMemoryQuestion,
  // voir describe "MEMORY, crédit" plus bas) : son montant vient de
  // calcMemoryScore (0 pt sans paire trouvée), donc "+5 pts" ne s'affiche
  // plus pour ce cas — MEMORY ne peut plus servir d'exemple de "type
  // inconnu/futur". MEMOTION reste le seul type non câblé sur ce chemin de
  // crédit (chip de mode mis à part, ligne 154) — repli permissif inchangé.
  it('type inconnu/futur (ex. sans TYPE encore câblé) : "+N pts" ET "0 pt" proposés (défaut permissif, R4)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'MEMOTION', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 0 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+5 pts')).toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.queryByText('pas de buzz')).not.toBeInTheDocument()
    expect(screen.queryByText('pas de réponse')).not.toBeInTheDocument()
  })

  // R3 — le piège explicite du plan : ne pas confondre "a tenté"
  // (canAwardPoints) et "est verrouillée" (awardedTeams, #170). La régie
  // n'a aucune garde et peut créditer une équipe qui n'a jamais buzzé.
  it('R3 — équipe SANS tentative mais DÉJÀ créditée par la régie reste verrouillée avec son montant', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'SPEEDY', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 0 } }, // jamais buzzé
      awardedTeams: { 'Les Rouges': { POINTS: 5, TIMESTAMP: 1000 } }, // créditée par /admin
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('+5 pts')).toBeInTheDocument() // montant du VERROU (serveur), pas de "+N pts" cliquable
    expect(container.querySelectorAll('.anim-credit-control-btn')).toHaveLength(0)
  })

  it('ARDOISE : inchangée (#158) — ne passe pas par ce chemin de crédit universel (AnimArdoiseList mockée, propre couverture)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'ARDOISE' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', IS_VPLAYER: true } },
    }))
    const { container } = render(<AnimPage />)
    expect(screen.getByTestId('ardoise-list')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-card')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Zone C — câblage du crédit synchronisé (#170/T6)
//
// La carte d'équipe utilise désormais AnimCreditControl (composant testé
// exhaustivement dans AnimCreditControl.test.jsx, T5) au lieu du bouton
// `.anim-team-credit-btn` — ici on vérifie uniquement le CÂBLAGE :
// awardedTeams[team.name] arrive bien en prop `awarded`, et la cible/le
// montant du crédit restent ceux de #156/#157 (getTeamAward inchangé),
// désormais transmis comme argument de handleCredit au lieu d'être
// recalculés dans le composant.
// ---------------------------------------------------------------------------

describe('AnimPage — câblage du crédit synchronisé vers AnimCreditControl (#170/T6)', () => {
  it('équipe non présente dans awardedTeams : état libre (deux gestes disponibles)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
      awardedTeams: {},
    }))
    const { container } = render(<AnimPage />)
    expect(screen.getByText('+5 pts')).toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(container.querySelector('.anim-credit-control-locked')).toBeNull()
  })

  it('équipe présente dans awardedTeams : état verrouillé, aucun geste, aucune action au clic', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
      awardedTeams: { 'Les Rouges': { POINTS: 5, TIMESTAMP: 1000 } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(container.querySelectorAll('.anim-credit-control-btn')).toHaveLength(0)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
    expect(props.setTeamPoints).not.toHaveBeenCalled()
  })

  // R1 — même piège qu'au niveau composant (AnimCreditControl.test.jsx),
  // vérifié ici au niveau intégration AnimPage : une équipe refusée
  // (0 pt) doit rester verrouillée, pas redevenir créditable.
  it('équipe refusée (awardedTeams[team].POINTS === 0) : verrouillée comme un crédit positif', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
      awardedTeams: { 'Les Rouges': { POINTS: 0, TIMESTAMP: 1000 } },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.queryByText('+5 pts')).not.toBeInTheDocument()
  })

  it('"0 pt" (refus) suit le même chemin de crédit que "+N pts" : setBumperPoints(mac, 0)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('0 pt').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('m1', 0)
  })

  it('"0 pt" (refus) cible l\'équipe directement quand POINTS_TARGET=TEAM, comme un crédit ordinaire', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5', POINTS_TARGET: 'TEAM' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('0 pt').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 0)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it('équipe verrouillée en QCM : le montant affiché est celui du serveur (awardedTeams), pas recalculé par calcQcmTeamAward', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED' },
      },
      creditPoints: 10,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 3 } },
      // Une autre tablette a crédité 7 pts avant que HINTS_AT_BUZZ=3 n'ait
      // localement calculé un montant différent (ex. indice tombé entre
      // temps) — le verrouillage affiche TOUJOURS ce que le serveur
      // confirme, jamais un recalcul local.
      awardedTeams: { 'Les Rouges': { POINTS: 7, TIMESTAMP: 1000 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — crédit QCM par équipe (#157/T3)
// ---------------------------------------------------------------------------

describe('AnimPage — crédit QCM par équipe (#157/T3)', () => {
  function qcmGameMock(phase, overrides = {}) {
    // Déstructure `gameState` séparément — un spread `...overrides` en
    // dernier écraserait entièrement le `gameState` fusionné ci-dessous
    // (phase/question perdus) si `overrides.gameState` ne contient qu'un
    // sous-ensemble de champs (ex: juste `qcmInvalidated`).
    const { gameState: gameStateOverrides, ...restOverrides } = overrides
    return makeGameMock({
      gameState: {
        phase,
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS: '10' },
        ...gameStateOverrides,
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 0 },
      },
      ...restOverrides,
    })
  }

  it('chaque équipe affiche et crédite SON montant (pénalité de son buzzer)', () => {
    const props = qcmGameMock('STOPPED')
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    // Les Rouges : 1 indice au buzz -> 10*0.67 = 7 pts
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
    // Les Bleus : 0 indice au buzz -> 10 pts pleins
    expect(screen.getByText('+10 pts')).toBeInTheDocument()

    screen.getByText('+7 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('r1', 7)
    screen.getByText('+10 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('b1', 10)
  })

  it('équipe sans buzzer correct : replie sur la pénalité des indices courants (pas 0)', () => {
    const props = qcmGameMock('STOPPED', {
      gameState: { qcmInvalidated: ['GREEN'] }, // 1 indice courant invalidé -> pénalité 0.67
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'BLUE', HINTS_AT_BUZZ: 2 }, // mauvaise couleur
      },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    // Repli : pénalité des indices COURANTS (1 -> 0.67 -> 7), pas le
    // HINTS_AT_BUZZ du buzz (2 -> 0.33 -> 3)
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
    expect(screen.queryByText('+3 pts')).not.toBeInTheDocument()
  })

  it('le montant est identique à celui que calcQcmTeamAward (la même règle que /admin) calculerait', () => {
    // 2 indices au buzz -> pénalité 0.33 -> 10*0.33 = 3.3 -> round 3
    const props = qcmGameMock('REVEALED', {
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    expect(screen.getByText('+3 pts')).toBeInTheDocument()
  })

  it('non-régression SPEEDY : montant unique identique pour toutes les équipes', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'SPEEDY', POINTS: '10' } },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000 },
        b1: { TEAM: 'Les Bleus', TIME: 1200 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const creditButtons = Array.from(container.querySelectorAll('.anim-credit-control-btn-award')).map(b => b.textContent)
    expect(creditButtons).toEqual(['+10 pts', '+10 pts'])
  })

  it('le crédit reste indisponible avant STOPPED/REVEALED en QCM aussi', () => {
    useGame.mockReturnValue(qcmGameMock('STARTED'))
    render(<AnimPage />)
    expect(screen.queryByText(/\+\d+ pts/)).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — réponse QCM (couleur, joueur, justesse) (#157/T4)
// ---------------------------------------------------------------------------

describe('AnimPage — réponse QCM en zone C (#157/T4)', () => {
  function qcmAnswerGameMock(phase) {
    return makeGameMock({
      gameState: {
        phase,
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: false, QCM_CORRECT: 'RED' },
      },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', NAME: 'Alice' } },
    })
  }

  it('affiche la couleur choisie et le nom du joueur dès le buzz (avant STOPPED/REVEALED)', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-qcm-color')).not.toBeNull()
  })

  it('n\'affiche PAS la justesse (✓/✗) avant REVEALED (décision D1)', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-correct')).toBeNull()

    useGame.mockReturnValue(qcmAnswerGameMock('STOPPED'))
    const { container: container2 } = render(<AnimPage />)
    expect(container2.querySelector('.anim-team-qcm-correct')).toBeNull()
  })

  it('affiche ✓ en REVEALED quand la couleur choisie est correcte', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('REVEALED'))
    const { container } = render(<AnimPage />)
    const marker = container.querySelector('.anim-team-qcm-correct')
    expect(marker).not.toBeNull()
    expect(marker.textContent).toBe('✓')
    expect(marker.className).toContain('correct')
  })

  it('affiche ✗ en REVEALED quand la couleur choisie est incorrecte', () => {
    const props = makeGameMock({
      gameState: { phase: 'REVEALED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'BLUE', NAME: 'Bob' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const marker = container.querySelector('.anim-team-qcm-correct')
    expect(marker.textContent).toBe('✗')
    expect(marker.className).toContain('incorrect')
  })

  it('n\'affiche rien tant que l\'équipe n\'a pas buzzé', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 0 } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-answer')).toBeNull()
  })

  it('reste visuellement inchangée hors QCM (pas de badge de couleur en SPEEDY)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000 } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-answer')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Veille écran — reprend le motif PlayerDisplay.jsx:912-921
// ---------------------------------------------------------------------------

describe('AnimPage — veille écran', () => {
  it('utilise le wake lock natif quand disponible', async () => {
    const request = vi.fn().mockResolvedValue({ release: vi.fn() })
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    render(<AnimPage />)
    await vi.waitFor(() => expect(request).toHaveBeenCalledWith('screen'))

    delete navigator.wakeLock
  })

  it('replie sur NoSleep.js quand le wake lock natif est indisponible', () => {
    expect('wakeLock' in navigator).toBe(false)
    // Pas d'assertion sur l'instance NoSleep (mockée) au-delà du rendu sans
    // erreur — le chemin nominal HTTP est déjà couvert par la même logique
    // testée sur PlayerDisplay (PlayerDisplay.*.test.jsx).
    expect(() => render(<AnimPage />)).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// #157 T5 (test-writer) — compléments aux 16 tests T3/T4 de dev-frontend
// (rapport _work/reports/dev-frontend-20260813-153639.md, commit 8ebffd1).
// Angles non couverts par cette suite, vérifiés en la relisant d'abord pour
// ne rien dupliquer :
//   - un rendu MIXTE (3 équipes, les 3 branches de calcQcmTeamAward à la
//     fois — correct/repli-couleur-fausse/repli-sans-buzz) au lieu d'un
//     scénario par test
//   - le "no-op" de handleCredit quand aucun bumper n'a buzzé pour une
//     équipe cible=PLAYER (fastestBumper undefined) — non exercé
//   - la garde isQcmWithHints (QCM_HINTS_ENABLED=false) : seule la
//     régression SPEEDY était testée, pas la régression QCM-sans-indices,
//     un code path distinct (`gameState.question?.TYPE === 'QCM' &&
//     gameState.question?.QCM_HINTS_ENABLED`, AnimPage.jsx:146)
//   - QCM + POINTS_TARGET=TEAM combinés (testés séparément jusqu'ici :
//     QCM+PLAYER d'un côté, SPEEDY+TEAM de l'autre)
//   - un balayage de parité sur plusieurs valeurs de HINTS_AT_BUZZ (pas
//     seulement le cas 2 indices déjà couvert)
//   - la table QCM_COLORS réellement consommée (lettre/couleur/label exacts
//     pour une couleur autre que RED) et ses deux cas limites (couleur
//     absente de la table, buzzer correct sans NAME)
// ---------------------------------------------------------------------------

describe('AnimPage — crédit QCM par équipe, compléments (#157/T3)', () => {
  // #171/F4/F6, RÉÉCRIT (pas neutralisé) — avant #171, "Les Verts" (n'a pas
  // buzzé) affichait quand même "+3 pts" (repli neutre de calcQcmTeamAward)
  // et le test vérifiait juste que le clic ne créditait personne. Depuis
  // #171 (D inversée), canAwardPoints retire "+N pts" de l'offre pour une
  // équipe qui n'a pas tenté : "Les Verts" ne voit plus que "0 pt". Même
  // scénario de départ (3 équipes, 3 situations), assertions mises à jour.
  it('rendu mixte : 3 équipes, dont une sans tentative (0 pt seul)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS: '10' },
        qcmInvalidated: ['GREEN', 'YELLOW'], // 2 indices courants -> pénalité 0.33 pour le repli
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 }, // buzzer correct, 1 indice -> 7 pts
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 }, // buzzer incorrect -> repli 0.33 -> 3 pts
        'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 }, // n'a pas buzzé (TIME=0) -> pas de tentative
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'GREEN', HINTS_AT_BUZZ: 0 },
        v1: { TEAM: 'Les Verts', TIME: 0 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)

    // Rouges (buzzer correct, 1 indice) -> 7 pts ; Bleus (buzzer incorrect)
    // -> repli 0.33 -> 3 pts ; Verts (pas de tentative) -> "+N pts" absent,
    // seul "0 pt" proposé (D inversée, #171).
    const amounts = Array.from(container.querySelectorAll('.anim-credit-control-btn-award')).map(b => b.textContent)
    expect(amounts).toEqual(['+7 pts', '+3 pts']) // Les Verts n'y figure PAS

    const cardFor = (teamName) =>
      Array.from(container.querySelectorAll('.anim-team-card'))
        .find(card => card.querySelector('.anim-team-card-name')?.textContent === teamName)

    cardFor('Les Rouges').querySelector('.anim-credit-control-btn-award').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('r1', 7)

    // Les Bleus ont buzzé (mauvaise couleur) — le crédit va quand même à ce
    // bumper, au montant réduit.
    cardFor('Les Bleus').querySelector('.anim-credit-control-btn-award').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('b1', 3)

    // Les Verts n'ont pas buzzé du tout : pas de bouton "+N pts" (motif
    // "pas de réponse" affiché à côté), et le clic sur "0 pt" ne crédite
    // personne (aucun bumper éligible — couvert plus précisément par le
    // test suivant).
    const vertsCard = cardFor('Les Verts')
    expect(vertsCard.querySelector('.anim-credit-control-btn-award')).toBeNull()
    expect(vertsCard.textContent).toContain('pas de réponse')
    props.setBumperPoints.mockClear()
    vertsCard.querySelector('.anim-credit-control-btn-zero').click()
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  // #171/F4/F6/T7, RÉÉCRIT (pas neutralisé) — avant #171, une équipe sans
  // buzz voyait quand même "+N pts" (repli neutre de calcQcmTeamAward) et
  // le test vérifiait que le CLIC ne créditait personne (défense après
  // coup). Depuis #171, `canAwardPoints` retire "+N pts" de l'offre AVANT
  // même le clic — l'équipe sans tentative ne voit plus que "0 pt" (D
  // inversée, plan §5) : le test change de nature (absence du geste, pas
  // clic sans effet), même scénario de départ (équipe QCM, TIME=0).
  it('équipe sans aucun buzz (QCM) : "+N pts" absent, seul "0 pt" est proposé, motif "pas de réponse" affiché', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED' },
        qcmInvalidated: [],
      },
      creditPoints: 10,
      teams: { 'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 } },
      bumpers: { v1: { TEAM: 'Les Verts', TIME: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    expect(screen.queryByText(/^\+\d+ pts$/)).not.toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.getByText('pas de réponse')).toBeInTheDocument()

    // Cible PLAYER : aucun bumper de l'équipe n'a TIME > 0 (v1 a TIME: 0),
    // donc handleCredit ne trouve pas de "plus rapide" éligible — même
    // comportement défensif qu'avant #171, atteint différemment (le geste
    // proposé est désormais "0 pt", pas "+N pts", mais reste sans effet
    // faute de bumper cible).
    screen.getByText('0 pt').click()
    expect(props.setBumperPoints).not.toHaveBeenCalled()
    expect(props.setTeamPoints).not.toHaveBeenCalled()
  })

  it('QCM_HINTS_ENABLED=false : montant unique pour toutes les équipes malgré des indices différents au buzz (garde isQcmWithHints)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: false, QCM_CORRECT: 'RED' },
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        // Indices très différents (2 vs 0) — ne doivent avoir AUCUN effet
        // puisque QCM_HINTS_ENABLED est false : c'est un code path distinct
        // de la non-régression SPEEDY (TYPE différent), à garder testé
        // séparément.
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'GREEN', HINTS_AT_BUZZ: 0 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const amounts = Array.from(container.querySelectorAll('.anim-credit-control-btn-award')).map(b => b.textContent)
    expect(amounts).toEqual(['+10 pts', '+10 pts'])
  })

  it('QCM + POINTS_TARGET=TEAM : crédite l\'équipe avec SON montant (pas un montant global)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: {
          ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS_TARGET: 'TEAM',
        },
      },
      creditPoints: 10,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    screen.getByText('+7 pts').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 7)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it.each([0, 1, 2, 3])(
    'parité avec calcQcmTeamAward pour %i indice(s) au buzz',
    (hints) => {
      const question = { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED' }
      const bumper = { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: hints, mac: 'r1' }
      const expected = calcQcmTeamAward(question, 10, [bumper], 0).amount

      const props = makeGameMock({
        gameState: { phase: 'STOPPED', question },
        creditPoints: 10,
        teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
        bumpers: { r1: bumper },
      })
      useGame.mockReturnValue(props)
      const { container } = render(<AnimPage />)
      expect(container.querySelector('.anim-credit-control-btn-award').textContent).toBe(`+${expected} pts`)
    }
  )
})

describe('AnimPage — réponse QCM en zone C, compléments (#157/T4)', () => {
  it('couleur autre que RED : lettre, teinte et libellé exacts de la table QCM_COLORS (GREEN)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 } },
      bumpers: { v1: { TEAM: 'Les Verts', TIME: 1000, ANSWER_COLOR: 'GREEN', NAME: 'Chloé' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const swatch = container.querySelector('.anim-team-qcm-color')
    expect(swatch.textContent).toBe('B')
    expect(swatch.style.backgroundColor).toBe('rgb(34, 197, 94)') // #22c55e
    expect(swatch.title).toBe('Vert')
  })

  it('ANSWER_COLOR absente de QCM_COLORS : pas de pastille, mais le nom du joueur reste affiché', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'PURPLE', NAME: 'Zoé' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-color')).toBeNull()
    expect(screen.getByText('Zoé')).toBeInTheDocument()
  })

  it('buzzer correct sans NAME : pastille affichée, aucun nom rendu', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED' } }, // pas de NAME
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-color')).not.toBeNull()
    expect(container.querySelector('.anim-team-qcm-player')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Zone A — énoncé de la question en cours (#163/F1, T2)
//
// Plan _work/reports/plan-20260814-101626.md, maquette
// https://claude.ai/code/artifact/76c34d5c-74ce-4dcd-ad0d-3b102988b7af, état 1
// « phase READY ». Condition d'affichage : présence de gameState.question
// SEULE — aucune garde de phase (l'animateur doit pouvoir lire la question à
// voix haute avant de presser LANCER, écart assumé avec la TV qui n'affiche
// le contenu qu'à partir de STARTED, PlayerDisplay.jsx:627). Tableau
// « Règles d'affichage par phase » de la maquette : "Texte de la question"
// -> ✓ à PREPARE/READY/STARTED-PAUSED/STOPPED/REVEALED.
// ---------------------------------------------------------------------------

describe('AnimPage — Zone A, énoncé de la question en cours (#163/F1, T2)', () => {
  const QUESTION_TEXT = "En quelle année Charlemagne a-t-il été couronné empereur d'Occident ?"

  it.each(['READY', 'STARTED', 'STOPPED', 'REVEALED'])(
    "affiche l'énoncé de la question en phase %s (sans garde de phase)",
    (phase) => {
      useGame.mockReturnValue(makeGameMock({
        gameState: { phase, question: { ID: '12', TYPE: 'SPEEDY', QUESTION: QUESTION_TEXT } },
      }))
      render(<AnimPage />)
      expect(screen.getByText(QUESTION_TEXT)).toBeInTheDocument()
    }
  )

  it('n\'affiche aucun énoncé quand gameState.question est nul — seul le repli "Aucune question en cours" reste rendu', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', question: null } }))
    render(<AnimPage />)
    expect(screen.getByText('Aucune question en cours')).toBeInTheDocument()
    expect(screen.queryByText(QUESTION_TEXT)).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #166/T11 — SUPPRIMÉ (pas neutralisé) : describe('AnimPage — Zone A, titre
// de la question suivante (#163/F2, T4, GATE 2 D1)') testait la puce
// "Suivante" du bandeau zone contexte (`.anim-next-question-statement` /
// `.anim-next-question-meta`). #166/F4 déplace ce contenu — même format,
// via le même `nextQuestionFormat.js` — dans le bouton "à suivre" de la
// zone conduite (`AnimNextButton`) ; la puce elle-même n'existe plus dans
// AnimPage.jsx (plan, critère d'acceptation : "la puce «Suivante» (#163/F2)
// ... n'existe plus sous sa forme précédente"). Couverture équivalente et
// exhaustive désormais dans AnimNextButton.test.jsx (#166/T5) et la matrice
// de AnimConductPanel.test.jsx (#166/T6, colonne "À suivre").
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Zone A — points de la question en cours, ligne méta (#163, retouche
// alignement à droite, coordination dev-frontend)
//
// Nouveau span dédié `.anim-question-points` ("<POINTS>pt"), aligné à droite
// de `.anim-meta-row` via margin-left:auto (n'affecte pas la position du
// Timer, contrainte explicite de dev-frontend). Condition d'affichage :
// `question?.POINTS != null` (0 est une valeur légitime, pas un "absent").
// ---------------------------------------------------------------------------

describe('AnimPage — Zone A, points de la question en cours (#163, retouche alignement)', () => {
  it('affiche les points de la question courante ("<POINTS>pt")', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', QUESTION: 'Q ?', POINTS: '15' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('15pt')).toBeInTheDocument()
  })

  it('affiche "0pt" quand POINTS vaut explicitement 0 (valeur légitime, pas absente)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', QUESTION: 'Q ?', POINTS: '0' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('0pt')).toBeInTheDocument()
  })

  it("n'affiche aucun point quand POINTS est absent (undefined)", () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', QUESTION: 'Q ?' } },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-question-points')).toBeNull()
  })

  it("n'affiche aucun point quand aucune question n'est en cours", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', question: null } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-question-points')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// #166/T11 — SUPPRIMÉ (pas neutralisé) : describe('AnimPage — Zone A,
// réponse attendue hors QCM (#163/F4, T5, GATE 2 D2)') testait le bloc
// `.anim-answer-box` conditionnel (affiché uniquement en REVEALED). #166/F10
// le remplace par la zone réponse PERMANENTE (`AnimAnswerZone`, montée à la
// place, `.anim-answer-box` n'existe plus dans AnimPage.jsx) — toujours
// rendue dès qu'une question est chargée, floutée hors REVEALED plutôt
// qu'absente. Couverture équivalente et exhaustive désormais dans
// AnimAnswerZone.test.jsx (#166/T7, déjà committé, 11/11 PASS) ; le câblage
// AnimPage → AnimAnswerZone (props question/revealed) est vérifié plus haut
// (describe "zone contexte, câblage vers AnimAnswerZone").
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// #159/T3 — zone conduite, câblage MEMORY vers AnimConductPanel. Le rendu
// réel de la grille (AnimMemoryGrid) a sa propre couverture exhaustive
// (AnimMemoryGrid.test.jsx) et AnimConductPanel sa propre matrice L3
// (AnimConductPanel.test.jsx) — ici on vérifie seulement que les 7 champs
// MEMORY_* de gameState arrivent bien groupés dans la prop `memory`, et que
// flipMemoryCard (useGame()) est bien transmis comme onFlipMemoryCard.
// ---------------------------------------------------------------------------

describe('AnimPage — zone conduite, câblage MEMORY vers AnimConductPanel (#159/T3)', () => {
  it('regroupe les 7 champs MEMORY_* de gameState dans la prop `memory`', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'MEMORY' },
        memoryFlippedCards: ['1-1'],
        memoryMatchedPairs: [2],
        MEMORY_PAIR_OWNERS: { '2': 'Les Rouges' },
        MEMORY_CURRENT_TEAM: 'Les Rouges',
        MEMORY_TEAM_PAIRS: { 'Les Rouges': 1 },
        MEMORY_TEAM_ERRORS: { 'Les Rouges': 0 },
        memoryErrors: 3,
      },
    }))
    const { getByTestId } = render(<AnimPage />)
    const memory = JSON.parse(getByTestId('conduct-panel').getAttribute('data-memory'))
    expect(memory).toEqual({
      flippedCards: ['1-1'],
      matchedPairs: [2],
      pairOwners: { '2': 'Les Rouges' },
      currentTeam: 'Les Rouges',
      teamPairs: { 'Les Rouges': 1 },
      teamErrors: { 'Les Rouges': 0 },
      errors: 3,
    })
  })

  it('transmet flipMemoryCard comme onFlipMemoryCard : le geste MOCK_FLIP_MEMORY_CARD déclenche flipMemoryCard("1-1")', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'MEMORY' } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('MOCK_FLIP_MEMORY_CARD').click()
    expect(props.flipMemoryCard).toHaveBeenCalledWith('1-1')
  })
})

// ---------------------------------------------------------------------------
// #159/T4 — zone équipes, participation/activité MEMORY. AnimTeamCard n'est
// pas mockée ici (comme pour le reste de Zone C) : on vérifie le rendu réel
// des classes `active`/`dimmed` (AnimTeamCard.jsx, F4) et du libellé
// memoryStat, tous deux dérivés par AnimPage depuis MEMORY_CURRENT_TEAM/
// MEMORY_PARTICIPATING_TEAMS/MEMORY_TEAM_PAIRS/MEMORY_TEAM_ERRORS.
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, participation et équipe active MEMORY (#159/T4)', () => {
  // Bug corrigé (signalé par dev-frontend) : `...overrides` en dernier
  // écrasait ENTIÈREMENT la clé `gameState` construite juste au-dessus avec
  // la valeur brute passée par l'appelant (`overrides.gameState` seul, sans
  // fusion) — `phase`/`question`/`MEMORY_PAIRS` disparaissaient dès qu'un
  // test passait `{gameState: {...}}`, donc `question` redevenait `null`
  // et `isMemoryQuestion` `false` chez AnimPage. `...overrides` doit être
  // spreadé EN PREMIER, `gameState` reconstruit PAR-DESSUS pour fusionner
  // avec `overrides.gameState` comme prévu.
  function memoryGameMock(overrides = {}) {
    return makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: { m1: { TEAM: 'Les Rouges' }, m2: { TEAM: 'Les Bleus' } },
      ...overrides,
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'MEMORY', MEMORY_PAIRS: [{ ID: 1 }, { ID: 2 }] },
        ...overrides.gameState,
      },
    })
  }

  it('MEMORY_CURRENT_TEAM identifie l\'équipe active (classe anim-team-card-active), la seule', () => {
    useGame.mockReturnValue(memoryGameMock({
      gameState: { MEMORY_CURRENT_TEAM: 'Les Rouges' },
    }))
    const { container } = render(<AnimPage />)
    const active = container.querySelectorAll('.anim-team-card-active')
    expect(active).toHaveLength(1)
    expect(active[0].querySelector('.anim-team-card-name').textContent).toBe('Les Rouges')
  })

  it('MEMORY_PARTICIPATING_TEAMS restreint : équipe absente de la liste -> en retrait (dimmed) + "ne participe pas"', () => {
    useGame.mockReturnValue(memoryGameMock({
      gameState: { MEMORY_PARTICIPATING_TEAMS: ['Les Rouges'] },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-card-dimmed')).not.toBeNull()
    expect(screen.getByText('ne participe pas')).toBeInTheDocument()
  })

  it('MEMORY_PARTICIPATING_TEAMS vide/absent : aucune restriction, aucune équipe en retrait', () => {
    useGame.mockReturnValue(memoryGameMock())
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.anim-team-card-dimmed')).toHaveLength(0)
    expect(screen.queryByText('ne participe pas')).not.toBeInTheDocument()
  })

  it('compteur par équipe (MEMORY_TEAM_PAIRS/TEAM_ERRORS) : libellé "N paire(s) · M erreur(s)", singulier/pluriel', () => {
    useGame.mockReturnValue(memoryGameMock({
      gameState: {
        MEMORY_TEAM_PAIRS: { 'Les Rouges': 1, 'Les Bleus': 2 },
        MEMORY_TEAM_ERRORS: { 'Les Rouges': 1, 'Les Bleus': 3 },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('1 paire · 1 erreur')).toBeInTheDocument()
    expect(screen.getByText('2 paires · 3 erreurs')).toBeInTheDocument()
  })

  it('SOLO (pas de MEMORY_TEAM_PAIRS) : repli sur les compteurs globaux memoryMatchedPairs/memoryErrors pour chaque équipe', () => {
    useGame.mockReturnValue(memoryGameMock({
      gameState: { memoryMatchedPairs: [1, 2], memoryErrors: 4 },
    }))
    render(<AnimPage />)
    const stats = screen.getAllByText('2 paires · 4 erreurs')
    expect(stats).toHaveLength(2) // les deux équipes affichent le même compteur global
  })

  it('hors question MEMORY : ni active ni dimmed ni memoryStat, même avec MEMORY_CURRENT_TEAM résiduel', () => {
    useGame.mockReturnValue(memoryGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'SPEEDY' },
        MEMORY_CURRENT_TEAM: 'Les Rouges', // résidu d'une question MEMORY précédente
      },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.anim-team-card-active')).toHaveLength(0)
    expect(container.querySelectorAll('.anim-team-card-dimmed')).toHaveLength(0)
    expect(container.querySelector('.anim-team-memory-stat')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// #159/T5 — zone équipes, crédit MEMORY. Le calcul lui-même
// (calcMemoryScore/resolvePointsAward) a sa couverture exhaustive dans
// pointsAward.test.js (pré-existante, #156/#157) — ici on vérifie
// uniquement que le montant affiché sur /anim EST celui que ce calcul
// mutualisé produit pour les compteurs de l'équipe (jamais recalculé
// localement), que le repli SOLO/multi-équipes est respecté, et que la
// non-participation retire "+N pts" sans jamais afficher de motif
// "pas de buzz/réponse" (F6, spécifique à MEMORY).
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, crédit MEMORY (#159/T5)', () => {
  it('crédite exactement le montant de calcMemoryScore pour les compteurs de l\'équipe (multi-équipes)', () => {
    const question = {
      ID: '1',
      TYPE: 'MEMORY',
      MEMORY_PAIRS: [{ ID: 1 }, { ID: 2 }, { ID: 3 }],
      MEMORY_CONFIG: { POINTS_PER_PAIR: 10, ERROR_PENALTY: 2, COMPLETION_BONUS: 5 },
    }
    // Les Rouges : 3/3 paires (complet, bonus), 1 erreur -> 3*10 + 5 - 1*2 = 33
    const expected = resolvePointsAward(question, 999, { memory: { matchedPairs: 3, errors: 1 } }).amount
    expect(expected).toBe(33)

    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question, MEMORY_TEAM_PAIRS: { 'Les Rouges': 3 }, MEMORY_TEAM_ERRORS: { 'Les Rouges': 1 } },
      creditPoints: 999, // basePoints n'a AUCUN effet sur MEMORY (calcMemoryScore l'ignore)
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+33 pts')).toBeInTheDocument()
  })

  it('SOLO (pas de MEMORY_TEAM_PAIRS) : le crédit utilise le repli global memoryMatchedPairs/memoryErrors', () => {
    const question = {
      ID: '1',
      TYPE: 'MEMORY',
      MEMORY_PAIRS: [{ ID: 1 }, { ID: 2 }],
      MEMORY_CONFIG: { POINTS_PER_PAIR: 10, ERROR_PENALTY: 0, COMPLETION_BONUS: 0 },
    }
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question, memoryMatchedPairs: [1], memoryErrors: 0 },
      creditPoints: 5,
      teams: { 'Solo': { COLOR: [0, 255, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Solo' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+10 pts')).toBeInTheDocument() // 1 paire * 10
  })

  it('équipe non participante (MEMORY_PARTICIPATING_TEAMS) : "0 pt" seul, jamais de motif "pas de buzz/réponse"', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'MEMORY', MEMORY_PAIRS: [{ ID: 1 }] },
        MEMORY_PARTICIPATING_TEAMS: ['Les Rouges'],
      },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 }, 'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' }, m2: { TEAM: 'Les Bleus' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('ne participe pas')).toBeInTheDocument()
    expect(screen.queryByText('pas de buzz')).not.toBeInTheDocument()
    expect(screen.queryByText('pas de réponse')).not.toBeInTheDocument()
    // Les Bleus (non participante) ne voit que "0 pt" ; Les Rouges (participante,
    // 0 paire trouvée) voit aussi "+0 pts" (amount=0, distinct de "0 pt" le refus).
    expect(screen.getAllByText('0 pt')).toHaveLength(2) // le refus, proposé aux deux
  })

  // R3 (#170), transposé à MEMORY : le verrouillage reste la SEULE source de
  // vérité, orthogonal à la participation — une équipe non participante mais
  // déjà créditée par la régie doit rester verrouillée avec son montant.
  it('équipe non participante mais DÉJÀ créditée (awardedTeams) reste verrouillée avec son montant', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'MEMORY', MEMORY_PAIRS: [{ ID: 1 }] },
        MEMORY_PARTICIPATING_TEAMS: ['Les Bleus'],
      },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
      awardedTeams: { 'Les Rouges': { POINTS: 5, TIMESTAMP: 1000 } },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('+5 pts')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #160/T7 — zone conduite, câblage MEMOTION vers AnimConductPanel. Même
// principe que le describe MEMORY équivalent ci-dessus (#159/T3) : le rendu
// réel des composants MEMOTION (AnimMotionGrid/Card/Actions) a sa propre
// couverture exhaustive (AnimMotionGrid.test.jsx, AnimMotionCard.test.jsx,
// AnimMotionActions.test.jsx) et le câblage L2/L3 sa propre couverture
// (AnimConductPanel.test.jsx) — ici on vérifie SEULEMENT que les 7 champs
// MEMOTION_* de gameState arrivent bien groupés dans la prop `motion`, et
// que les 5 émetteurs (useGame()) sont bien transmis comme
// onSelectMotionCard/onFlipMotionCard/onStopMotionTimer/onRevealMotionCard/
// onDoneMotionCard.
// ---------------------------------------------------------------------------

describe('AnimPage — zone conduite, câblage MEMOTION vers AnimConductPanel (#160/T7)', () => {
  it('regroupe les champs MEMOTION_* de gameState dans la prop `motion`', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'MEMOTION' },
        MEMOTION_SUBPHASE: 'SELECTED',
        MEMOTION_CARD_STATES: { c1: 'DONE' },
        MEMOTION_CARD_TEAMS: { c1: 'Les Rouges' },
        MEMOTION_CURRENT_TEAM: 'Les Bleus',
        MEMOTION_CURRENT_TEAM_COLOR: [37, 99, 235],
        MEMOTION_SELECTED: 'c1',
        MEMOTION_PARTICIPATING_TEAMS: ['Les Bleus', 'Les Rouges'],
      },
    }))
    const { getByTestId } = render(<AnimPage />)
    const motion = JSON.parse(getByTestId('conduct-panel').getAttribute('data-motion'))
    expect(motion).toMatchObject({
      subphase: 'SELECTED',
      cardStates: { c1: 'DONE' },
      cardTeams: { c1: 'Les Rouges' },
      currentTeam: 'Les Bleus',
      currentTeamColor: [37, 99, 235],
      selectedId: 'c1',
      participatingTeams: ['Les Bleus', 'Les Rouges'],
    })
  })

  it('transmet selectMotionCard/flipMotionCard/stopMotionTimer/revealMotionCard/doneMotionCard aux 5 callbacks onXxx', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'MEMOTION' } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    screen.getByText('MOCK_SELECT_MOTION_CARD').click()
    expect(props.selectMotionCard).toHaveBeenCalledWith('c1')

    screen.getByText('MOCK_FLIP_MOTION_CARD').click()
    expect(props.flipMotionCard).toHaveBeenCalledTimes(1)

    screen.getByText('MOCK_STOP_MOTION_TIMER').click()
    expect(props.stopMotionTimer).toHaveBeenCalledTimes(1)

    screen.getByText('MOCK_REVEAL_MOTION_CARD').click()
    expect(props.revealMotionCard).toHaveBeenCalledTimes(1)

    screen.getByText('MOCK_DONE_MOTION_CARD').click()
    expect(props.doneMotionCard).toHaveBeenCalledWith('c1', 'Les Bleus')
  })

  it('hors MEMOTION : motion vaut null/vide, aucun des 5 callbacks n\'est court-circuité pour autant (toujours transmis, comportement stable)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } },
    }))
    const { getByTestId } = render(<AnimPage />)
    const motion = JSON.parse(getByTestId('conduct-panel').getAttribute('data-motion'))
    // Les 7 champs MEMOTION_* de gameState sont à leurs valeurs d'ORIGINE
    // (chaînes/objets vides, cf. useWebSocket.js:69-75) même hors MEMOTION —
    // AnimPage ne les filtre pas par TYPE, seul AnimConductPanel décide quoi
    // en faire (branche L2/L3). Vérifié structurellement présent, pas absent.
    expect(motion).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// #160/T7 — zone équipes, participation/activité MEMOTION. Même principe et
// mêmes assertions que le describe MEMORY équivalent (#159/T4) : "liseré"
// (classe active) sur l'équipe correspondant à MEMOTION_CURRENT_TEAM,
// atténuation (dimmed) + "ne participe pas" pour les équipes hors
// MEMOTION_PARTICIPATING_TEAMS (plan §F8, dernier paragraphe).
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, participation et équipe active MEMOTION (#160/T7, F8)', () => {
  function motionGameMock(overrides = {}) {
    return makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: { m1: { TEAM: 'Les Rouges' }, m2: { TEAM: 'Les Bleus' } },
      ...overrides,
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'MEMOTION', MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'T', DIFFICULTY: 1 }] },
        ...overrides.gameState,
      },
    })
  }

  it('MEMOTION_CURRENT_TEAM identifie l\'équipe active (classe anim-team-card-active), la seule', () => {
    useGame.mockReturnValue(motionGameMock({
      gameState: { MEMOTION_CURRENT_TEAM: 'Les Rouges' },
    }))
    const { container } = render(<AnimPage />)
    const active = container.querySelectorAll('.anim-team-card-active')
    expect(active).toHaveLength(1)
    expect(active[0].querySelector('.anim-team-card-name').textContent).toBe('Les Rouges')
  })

  it('MEMOTION_PARTICIPATING_TEAMS restreint : équipe absente de la liste -> en retrait (dimmed) + "ne participe pas"', () => {
    useGame.mockReturnValue(motionGameMock({
      gameState: { MEMOTION_PARTICIPATING_TEAMS: ['Les Rouges'] },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-card-dimmed')).not.toBeNull()
    expect(screen.getByText('ne participe pas')).toBeInTheDocument()
  })

  it('MEMOTION_PARTICIPATING_TEAMS vide/absent : aucune restriction, aucune équipe en retrait', () => {
    useGame.mockReturnValue(motionGameMock())
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.anim-team-card-dimmed')).toHaveLength(0)
    expect(screen.queryByText('ne participe pas')).not.toBeInTheDocument()
  })

  it('hors question MEMOTION : ni active ni dimmed, même avec MEMOTION_CURRENT_TEAM résiduel (résidu d\'une manche précédente)', () => {
    useGame.mockReturnValue(motionGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'SPEEDY' },
        MEMOTION_CURRENT_TEAM: 'Les Rouges',
      },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.anim-team-card-active')).toHaveLength(0)
    expect(container.querySelectorAll('.anim-team-card-dimmed')).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// #160/T7 — AC7, CRITIQUE (risque R3 du plan) : aucun contrôle de crédit
// (AnimCreditControl) ne doit jamais être rendu pendant une manche MEMOTION
// — c'est le moteur SEUL qui crédite via MEMOTION_DONE. Toute la manche vit
// en phase STARTED (les 5 sous-phases sont un état INTERNE à STARTED,
// MEMOTION_SUBPHASE — pas des valeurs de gameState.phase), et
// CREDIT_PHASES = ['STOPPED', 'REVEALED'] (AnimPage.jsx:36) exclut déjà
// STARTED : ce test verrouille structurellement cette garantie contre toute
// régression future qui ajouterait par erreur un cas spécial MEMOTION au
// chemin de crédit universel pendant STARTED.
//
// Ne contredit PAS le test pré-existant "type inconnu/futur (ex. MEMOTION)"
// de la section crédit universel ci-dessus (ligne ~850) : celui-ci couvre
// un état de PHASE différent (STOPPED, cas de repli permissif générique,
// hors périmètre de #160/F8) — ce describe-ci couvre spécifiquement STARTED,
// où vit la manche MEMOTION réelle.
// ---------------------------------------------------------------------------

describe('AnimPage — AC7 CRITIQUE : aucun contrôle de crédit pendant une manche MEMOTION (#160/T7)', () => {
  function motionRoundMock(subphase, overrides = {}) {
    return makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'MEMOTION', MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'T', DIFFICULTY: 1 }] },
        MEMOTION_SUBPHASE: subphase,
        MEMOTION_CURRENT_TEAM: 'Les Rouges',
        ...overrides.gameState,
      },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
      ...overrides,
    })
  }

  it.each(['MEMORIZE', 'GRID', 'SELECTED', 'QUESTION', 'REVEAL'])(
    'sous-phase %s (phase STARTED) : AUCUN élément de crédit rendu (ni "+N pts" ni "0 pt")',
    (subphase) => {
      useGame.mockReturnValue(motionRoundMock(subphase))
      const { container } = render(<AnimPage />)
      expect(container.querySelector('.anim-team-credit-group')).toBeNull()
      expect(screen.queryByText(/^\+\d+ pts$/)).not.toBeInTheDocument()
      expect(screen.queryByText('0 pt')).not.toBeInTheDocument()
    }
  )

  it('même avec une équipe DÉJÀ créditée par la régie (awardedTeams non vide) pendant STARTED : toujours aucun contrôle de crédit rendu', () => {
    useGame.mockReturnValue(motionRoundMock('REVEAL', {
      awardedTeams: { 'Les Rouges': { POINTS: 3, TIMESTAMP: 1000 } },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-credit-group')).toBeNull()
    expect(container.querySelector('.anim-credit-control-locked')).toBeNull()
  })

  it('non-régression : la garde générale CREDIT_PHASES reste STOPPED/REVEALED (STARTED exclu) — vérifiée aussi hors MEMOTION pour ancrer le contraste', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', POINTS: '5' } },
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
    }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-credit-group')).toBeNull()
  })
})
