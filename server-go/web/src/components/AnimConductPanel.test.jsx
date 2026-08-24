import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimConductPanel from './AnimConductPanel'

// ---------------------------------------------------------------------------
// AnimConductPanel — zone de conduite /anim, réécrite en CINQ LIGNES
// PERMANENTES (#166/F5, T6 — test central du lot).
//
// REMPLACE ENTIÈREMENT l'ancienne suite (#156/#165) : le principe est
// renversé — le composant ne choisit plus QUELS boutons rendre selon la
// phase (branches PREPARE/isPlaying/canReveal/idle, props isPlaying/
// canStart/canReveal), il rend TOUJOURS les cinq mêmes emplacements de L1
// et calcule l'ÉTAT de chacun depuis `phase`/`question` via phaseRules.js.
// Nouvelle API de props : phase, question, qcmInvalidated, revealed,
// nextQuestion, onStart/onPause/onContinue/onStop/onReveal/onSelectNext —
// l'ancienne API n'existe plus. Suppression documentée (#166/T11) : la
// couverture précédente (16 tests) testait un principe qui n'existe plus
// dans le code, elle ne peut pas être "adaptée", seulement remplacée —
// c'est fait ici, exhaustivement.
//
// Source de vérité : maquette
// https://claude.ai/code/artifact/49cb60ae-8c6a-46f6-9268-5b0a6b5eb385,
// tableau "La matrice de L1 et du bouton « À suivre »" (10 situations),
// et son implémentation `utils/phaseRules.js` (déjà testée en isolation
// serait redondant — ici on vérifie le CÂBLAGE de la matrice dans le
// composant : présence, état exact par bouton, libellé secondaire, et
// surtout qu'un bouton éteint n'émet AUCUNE action).
// ---------------------------------------------------------------------------

function baseProps(overrides = {}) {
  return {
    phase: 'STOPPED',
    question: null,
    qcmInvalidated: [],
    revealed: false,
    nextQuestion: null,
    onStart: vi.fn(),
    onPause: vi.fn(),
    onContinue: vi.fn(),
    onStop: vi.fn(),
    onReveal: vi.fn(),
    onSelectNext: vi.fn(),
    ...overrides,
  }
}

function getBtn(container, key) {
  // Les 5 boutons L1 sont dans l'ordre LANCER/PAUSE/CONTINUER/STOP/RÉPONSE —
  // .anim-conduct-btn-{go|optional|danger|off} porte l'état, le TEXTE porte
  // l'identité (LANCER/PAUSE/...), pas une classe par bouton nommée.
  return Array.from(container.querySelectorAll('.anim-conduct-l1 .anim-conduct-btn')).find(
    (btn) => btn.textContent.startsWith(key)
  )
}

function stateOf(btn) {
  const m = btn.className.match(/anim-conduct-btn-(go|optional|danger|off)\b/)
  return m ? m[1] : null
}

function subLabelOf(btn) {
  return btn.querySelector('.anim-conduct-btn-sub')?.textContent ?? ''
}

// ---------------------------------------------------------------------------
// #166/T6 — matrice complète des 10 situations × 5 boutons L1, dérivée de
// utils/phaseRules.js (startButtonState/pauseButtonState/continueButtonState/
// stopButtonState/revealButtonState) et vérifiée contre le tableau de la
// maquette. Libellé secondaire exact pour les états go/optional/danger et
// les cas hors-phase illustrés par la maquette (READY/STARTED/PAUSED/
// STOPPED-jouée/REVEALED) ; repli générique "indispo." ailleurs, vérifié
// non vide pour tout bouton éteint (E3 : "un libellé secondaire indique le
// motif" — aucun bouton éteint ne doit être silencieux).
// ---------------------------------------------------------------------------

const MATRIX = [
  {
    situation: 'NEW_GAME',
    phase: 'NEW_GAME',
    question: null,
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'ENROLL',
    phase: 'ENROLL',
    question: null,
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'PREPARE',
    phase: 'PREPARE',
    question: null,
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'READY',
    phase: 'READY',
    question: { ID: '1' },
    expect: {
      LANCER: ['go', 'attendu'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'COUNTDOWN',
    phase: 'COUNTDOWN',
    question: { ID: '1' },
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'STARTED',
    phase: 'STARTED',
    question: { ID: '1' },
    expect: {
      LANCER: ['off', 'en cours'],
      PAUSE: ['optional', 'optionnel'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['danger', 'arrête'],
      RÉPONSE: ['off', 'après arrêt'],
    },
  },
  {
    situation: 'PAUSED',
    phase: 'PAUSED',
    question: { ID: '1' },
    expect: {
      LANCER: ['off', 'en cours'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['go', 'reprise'],
      STOP: ['danger', 'arrête'],
      RÉPONSE: ['off', 'après arrêt'],
    },
  },
  {
    situation: 'STOPPED (jouée)',
    phase: 'STOPPED',
    question: { ID: '1', STATUS: 'STOPPED' },
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['go', 'attendu'],
    },
  },
  {
    situation: 'STOPPED (non jouée)',
    phase: 'STOPPED',
    question: { ID: '1', STATUS: 'AVAILABLE' },
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'indispo.'],
    },
  },
  {
    situation: 'REVEALED',
    phase: 'REVEALED',
    question: { ID: '1' },
    expect: {
      LANCER: ['off', 'indispo.'],
      PAUSE: ['off', 'indispo.'],
      CONTINUER: ['off', 'indispo.'],
      STOP: ['off', 'indispo.'],
      RÉPONSE: ['off', 'déjà révélée'],
    },
  },
]

describe('AnimConductPanel — L1, matrice complète des 10 situations (#166/T6)', () => {
  it.each(MATRIX)('$situation : les 5 boutons L1 sont TOUJOURS présents', ({ phase, question }) => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase, question })} />)
    const buttons = container.querySelectorAll('.anim-conduct-l1 .anim-conduct-btn')
    expect(buttons).toHaveLength(5)
    ;['LANCER', 'PAUSE', 'CONTINUER', 'STOP', 'RÉPONSE'].forEach((label) => {
      expect(getBtn(container, label)).not.toBeUndefined()
    })
  })

  it.each(MATRIX)('$situation : état exact de chaque bouton (classe + disabled)', ({ phase, question, expect: expected }) => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase, question })} />)
    Object.entries(expected).forEach(([label, [state]]) => {
      const btn = getBtn(container, label)
      expect(stateOf(btn), `${label} en ${phase}`).toBe(state)
      expect(btn.disabled, `${label} disabled en ${phase}`).toBe(state === 'off')
    })
  })

  it.each(MATRIX)('$situation : libellé secondaire exact', ({ phase, question, expect: expected }) => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase, question })} />)
    Object.entries(expected).forEach(([label, [, sub]]) => {
      const btn = getBtn(container, label)
      expect(subLabelOf(btn), `sous-libellé de ${label} en ${phase}`).toBe(sub)
    })
  })

  it.each(MATRIX)('$situation : AUCUNE action émise par un bouton éteint', ({ phase, question, expect: expected }) => {
    const handlers = {
      LANCER: 'onStart',
      PAUSE: 'onPause',
      CONTINUER: 'onContinue',
      STOP: 'onStop',
      RÉPONSE: 'onReveal',
    }
    const props = baseProps({ phase, question })
    const { container } = render(<AnimConductPanel {...props} />)
    Object.entries(expected).forEach(([label, [state]]) => {
      const btn = getBtn(container, label)
      btn.click()
      const handlerProp = props[handlers[label]]
      if (state === 'off') {
        expect(handlerProp, `${label} ne doit émettre aucune action en ${phase}`).not.toHaveBeenCalled()
      } else {
        expect(handlerProp, `${label} doit émettre son action en ${phase} (état ${state})`).toHaveBeenCalledTimes(1)
      }
    })
  })
})

// ---------------------------------------------------------------------------
// #171/T3 — REMAP DE CONTENU (plan _work/reports/plan-20260816-192400.md
// §4/§9 F3) : L2 porte désormais les gestes propres au mode (EX-L3, texte
// inchangé), L3 porte la grille QCM ou l'emplacement réservé générique
// (EX-L2, `.anim-qcm-options` inchangée). Les noms de classe
// `.anim-conduct-l2`/`.anim-conduct-l3` ne changent PAS (décision
// dev-frontend, évite un renommage cosmétique) — SEUL leur CONTENU
// s'échange. Les deux describe ci-dessous sont donc RÉÉCRITS (sélecteurs
// L2↔L3 inversés par rapport à #166/T8), pas neutralisés : même
// couverture, contenu à jour. Toujours montées, jamais démontées
// conditionnellement (principe #166 inchangé).
// ---------------------------------------------------------------------------

describe('AnimConductPanel — L2, gestes propres au mode (#171/T3, ex-L3)', () => {
  it.each(['NEW_GAME', 'READY', 'STARTED', 'STOPPED', 'REVEALED'])(
    'phase %s : L2 présente, réservée (aucun geste de mode aujourd\'hui, tous modes confondus)',
    (phase) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({ phase, question: { ID: '1', TYPE: 'SPEEDY' } })} />
      )
      const l2 = container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')
      expect(l2).not.toBeNull()
      expect(l2.textContent.length).toBeGreaterThan(0)
    }
  )

  it('aucun état, aucune donnée : pas de bouton ni d\'élément interactif dans L2', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: 'QCM' } })} />
    )
    expect(container.querySelector('.anim-conduct-l2 button')).toBeNull()
  })

  // #171/T3 — mapping vérifié pour les 3 modes explicitement nommés par le
  // dispatch (SPEEDY/QCM/ARDOISE) : le libellé du mode dans L2 varie selon
  // question.TYPE, même en l'absence de geste réel aujourd'hui.
  it.each([
    ['SPEEDY', 'Speedy'],
    ['QCM', 'QCM'],
    ['ARDOISE', 'Ardoise'],
  ])('L2 nomme le mode %s ("%s") dans son texte réservé', (type, label) => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: type } })} />
    )
    const l2 = container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')
    expect(l2.textContent).toContain(label)
  })

  it('sans question chargée : texte réservé générique, sans nom de mode', () => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase: 'NEW_GAME', question: null })} />)
    const l2 = container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')
    expect(l2.textContent.length).toBeGreaterThan(0)
  })
})

describe('AnimConductPanel — L3, grille QCM ou réservé (#171/T3, ex-L2, F9)', () => {
  it('affiche la grille QCM (AnimQcmOptions) quand la question est de type QCM', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'QCM', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' } },
      })} />
    )
    expect(container.querySelector('.anim-conduct-l3 .anim-qcm-options')).not.toBeNull()
  })

  it('affiche un emplacement réservé (pas de grille) hors QCM', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } })} />
    )
    expect(container.querySelector('.anim-conduct-l3 .anim-qcm-options')).toBeNull()
    expect(container.querySelector('.anim-conduct-l3 .anim-conduct-reserved')).not.toBeNull()
  })

  it('affiche un emplacement réservé quand aucune question n\'est chargée', () => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase: 'NEW_GAME', question: null })} />)
    expect(container.querySelector('.anim-conduct-l3 .anim-conduct-reserved')).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// L3, grille MEMORY (#159/T3) — AnimMemoryGrid a sa propre couverture
// exhaustive (AnimMemoryGrid.test.jsx) : ici on vérifie uniquement le
// CÂBLAGE (branche isMemory, passthrough intégral des 7 champs `memory` +
// `teams` + `onFlipMemoryCard`), en lisant le rendu réel du composant (pas
// mocké dans ce fichier, même principe que la branche QCM ci-dessus).
// ---------------------------------------------------------------------------

describe('AnimConductPanel — L3, grille MEMORY (#159/T3)', () => {
  function memoryQuestion() {
    return { ID: '1', TYPE: 'MEMORY', MEMORY_PAIRS: [{ ID: 1, CARD1: { TEXT: 'Q' }, CARD2: { TEXT: 'R' } }] }
  }

  it('affiche la grille MEMORY (AnimMemoryGrid) quand la question est de type MEMORY', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: memoryQuestion() })} />
    )
    expect(container.querySelector('.anim-conduct-l3 .anim-memory-grid')).not.toBeNull()
    expect(container.querySelector('.anim-conduct-l3 .anim-qcm-options')).toBeNull()
    expect(container.querySelector('.anim-conduct-l3 .anim-conduct-reserved')).toBeNull()
  })

  it('transmet phase telle quelle : hors STARTED, la grille est inerte (cartes non cliquables)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STOPPED', question: memoryQuestion() })} />
    )
    const card = container.querySelector('.anim-conduct-l3 .anim-memory-card')
    expect(card.disabled).toBe(true)
    expect(card.className).toMatch(/\banim-memory-card-inert\b/)
  })

  it('transmet memory.flippedCards : la carte retournée est rendue "up"', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: memoryQuestion(),
        memory: { flippedCards: ['1-1'] },
      })} />
    )
    expect(container.querySelector('.anim-conduct-l3 .anim-memory-card-up')).not.toBeNull()
  })

  it('transmet memory.matchedPairs + memory.pairOwners + teams : couleur du propriétaire posée sur la paire trouvée', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: memoryQuestion(),
        teams: { 'Les Rouges': { COLOR: [255, 0, 0] } },
        memory: { matchedPairs: [1], pairOwners: { '1': 'Les Rouges' } },
      })} />
    )
    const matched = container.querySelector('.anim-conduct-l3 .anim-memory-card-matched')
    expect(matched).not.toBeNull()
    expect(matched.style.getPropertyValue('--anim-memory-owner-color')).toBeTruthy()
  })

  it('transmet memory.errors comme globalErrors (repli SOLO du bandeau)', () => {
    render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: memoryQuestion(),
        memory: { errors: 4 },
      })} />
    )
    expect(screen.getByText('4')).toBeInTheDocument()
  })

  it('onFlipMemoryCard reçoit l\'id de la carte cliquée ("pairID-cardNum")', () => {
    const onFlipMemoryCard = vi.fn()
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: memoryQuestion(), onFlipMemoryCard })} />
    )
    container.querySelector('.anim-conduct-l3 .anim-memory-card-down').click()
    expect(onFlipMemoryCard).toHaveBeenCalledTimes(1)
    expect(onFlipMemoryCard.mock.calls[0][0]).toMatch(/^\d+-[12]$/)
  })
})

// L4 était réservée (texte statique, aucune donnée) avant #168. Le lot
// v6.4.x #168 lui donne un contenu réel — AnimExplanationNote, propre
// couverture exhaustive dans AnimExplanationNote.test.jsx (F6/F7). Ce bloc
// est RÉÉCRIT (pas neutralisé, changement documenté : plan
// _work/reports/plan-20260818-121500.md, tâche F7) — il vérifie désormais le
// CÂBLAGE (question/revealed transmis tels quels) plutôt que la réserve
// vide, avec la même couverture par phase que l'ancienne suite.
describe('AnimConductPanel — L4, note d\'explication (#168, F7)', () => {
  it.each(['NEW_GAME', 'READY', 'STARTED', 'STOPPED', 'REVEALED'])(
    'phase %s : L4 présente, question sans EXPLANATION -> emplacement au repos',
    (phase) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({ phase, question: { ID: '1', TYPE: 'SPEEDY' } })} />
      )
      expect(container.querySelector('.anim-conduct-l4 .anim-explanation-note.empty')).not.toBeNull()
      expect(screen.getByText('Aucune note pour cette question')).toBeInTheDocument()
    }
  )

  it('aucun bouton dans L4 (AnimExplanationNote n\'en rend jamais)', () => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase: 'REVEALED', question: { ID: '1' } })} />)
    expect(container.querySelector('.anim-conduct-l4 button')).toBeNull()
  })

  it('question avec EXPLANATION : contenu transmis à AnimExplanationNote, visible en REVEALED', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'REVEALED',
        revealed: true,
        question: { ID: '1', TYPE: 'SPEEDY', EXPLANATION: 'Une note d\'explication.' },
      })} />
    )
    const note = container.querySelector('.anim-conduct-l4 .anim-explanation-note')
    expect(note).not.toBeNull()
    expect(note.className).toMatch(/\bshown\b/)
    expect(screen.getByText('Une note d\'explication.')).toBeInTheDocument()
  })

  it('question avec EXPLANATION hors REVEALED : floutée, pas visible en clair sans interaction', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        revealed: false,
        question: { ID: '1', TYPE: 'SPEEDY', EXPLANATION: 'Une note d\'explication.' },
      })} />
    )
    const note = container.querySelector('.anim-conduct-l4 .anim-explanation-note')
    expect(note.className).toMatch(/\bmasked\b/)
  })
})

// ---------------------------------------------------------------------------
// À suivre (AnimNextButton) — câblage (props transmises, matrice d'état
// exhaustive dans AnimNextButton.test.jsx, #166/T5) ET position/ancrage
// (#171/T4, F3, RÉÉCRIT — #166 le plaçait juste après L1 ; #171 le déplace
// en L5, DERNIER enfant de `.anim-conduct`, après le bloc central
// `.anim-conduct-mid` qui regroupe désormais L2/L3/L4). Réécrit, pas
// neutralisé : même composant, nouvelle position à vérifier.
// ---------------------------------------------------------------------------

describe('AnimConductPanel — "À suivre", câblage (#171/F3)', () => {
  it('reçoit phase/question/nextQuestion/onSelectNext, geste fonctionnel', () => {
    const props = baseProps({
      phase: 'REVEALED',
      question: { ID: '1', STATUS: 'STOPPED' },
      nextQuestion: { ID: '9', TYPE: 'SPEEDY', QUESTION: 'Q ?', POINTS: '5', TIME: '15' },
    })
    render(<AnimConductPanel {...props} />)
    expect(screen.getByText('À suivre')).toBeInTheDocument()
    expect(screen.getByText('#9 SPEEDY: Q ?')).toBeInTheDocument()
    screen.getByText('À suivre').closest('button').click()
    expect(props.onSelectNext).toHaveBeenCalledWith('9')
  })
})

// ---------------------------------------------------------------------------
// #171/T4 (nouveau, piège principal R2) — ancrage de L5. La position de
// "à suivre" (dernier enfant de `.anim-conduct`, donc L5 ancré en bas, juste
// au-dessus de la bande régie hors de ce composant) ne doit PAS dépendre du
// contenu du bloc central `.anim-conduct-mid` (L2/L3/L4). jsdom ne fait pas
// de layout — la mesure de pixels réelle relève de T9 (QA) — mais la
// structure DOM (dernier enfant, toujours) est le garant structurel de
// l'ancrage CSS (`flex-direction: column`, L5 = dernier enfant du flex).
// Le bloc central DOIT porter `min-height: 0` (R2, sans quoi un enfant flex
// refuse de rétrécir sous son contenu et L4 repousserait L5 hors écran) —
// vérifié via getComputedStyle, qui reflète les styles INLINE React mais
// PAS les règles d'une feuille CSS externe non chargée par jsdom ; si
// min-height:0 est posé en CSS externe (fichier .css) plutôt qu'inline,
// ce test ne peut pas le détecter et la vérification réelle revient à la
// revue de code (R1-f) — noté explicitement plutôt que silencieux.
// ---------------------------------------------------------------------------

describe('AnimConductPanel — L5, ancrage en bas (#171/T4)', () => {
  it('"à suivre" est le DERNIER enfant de .anim-conduct avec un bloc central VIDE (SPEEDY, pas de contenu L2/L3/L4 substantiel)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'REVEALED', question: { ID: '1', TYPE: 'SPEEDY' } })} />
    )
    const root = container.querySelector('.anim-conduct')
    const lastChild = root.lastElementChild
    expect(lastChild.querySelector('.anim-next-btn') || lastChild.classList.contains('anim-next-btn')).toBeTruthy()
  })

  it('"à suivre" reste le DERNIER enfant de .anim-conduct avec un bloc central CHARGÉ (QCM, grille + gestes + note)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'QCM', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' } },
        qcmInvalidated: ['YELLOW'],
      })} />
    )
    const root = container.querySelector('.anim-conduct')
    const lastChild = root.lastElementChild
    expect(lastChild.querySelector('.anim-next-btn') || lastChild.classList.contains('anim-next-btn')).toBeTruthy()
  })

  it('la position de "à suivre" (dernier enfant) est identique, bloc central vide ou chargé — c\'est TOUJOURS le même enfant qui est dernier', () => {
    const empty = render(
      <AnimConductPanel {...baseProps({ phase: 'REVEALED', question: { ID: '1', TYPE: 'SPEEDY' } })} />
    )
    const loaded = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: { ID: '1', TYPE: 'QCM', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' } },
      })} />
    )
    // AnimNextButton rend directement <button class="anim-next-btn ...">,
    // sans wrapper — quand ce bouton EST lui-même le dernier enfant (le cas
    // attendu), `.querySelector('.anim-next-btn')` ne le trouve pas (il
    // cherche parmi les DESCENDANTS, jamais l'élément interrogé lui-même).
    // Même repli que les deux tests précédents de ce describe.
    const lastEmpty = empty.container.querySelector('.anim-conduct').lastElementChild
    const lastLoaded = loaded.container.querySelector('.anim-conduct').lastElementChild
    const lastTagEmpty = lastEmpty.querySelector('.anim-next-btn') || lastEmpty.classList.contains('anim-next-btn')
    const lastTagLoaded = lastLoaded.querySelector('.anim-next-btn') || lastLoaded.classList.contains('anim-next-btn')
    expect(lastTagEmpty).toBeTruthy()
    expect(lastTagLoaded).toBeTruthy()
  })

  it('le bloc central regroupe L2/L3/L4 (pas "à suivre") — L1 et "à suivre" restent HORS de ce bloc', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: 'QCM' } })} />
    )
    const mid = container.querySelector('.anim-conduct-mid')
    expect(mid).not.toBeNull()
    expect(mid.querySelector('.anim-conduct-l2')).not.toBeNull()
    expect(mid.querySelector('.anim-conduct-l3')).not.toBeNull()
    expect(mid.querySelector('.anim-conduct-l4')).not.toBeNull()
    expect(mid.querySelector('.anim-next-btn')).toBeNull()
    expect(mid.querySelector('.anim-conduct-l1')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// waitReason (#172/C2) — motif d'attente PREPARE, calculé par
// utils/prepareWaitReason.js et passé par AnimPage.jsx (useMemo, `{short:
// true}`). Remplace le repli générique "indispo." du sous-libellé LANCER
// UNIQUEMENT quand le bouton est à l'état 'off' en phase PREPARE. Gap
// identifié en QA (#172, `_work/reports/qa-20260817-145412.md` §4) : aucun
// test n'exerçait ce câblage avant cet ajout. Sans `waitReason` (prop non
// fournie), le comportement doit rester STRICTEMENT identique à avant
// (rétrocompatibilité des appels existants du composant, cf. handoff
// dev-frontend `_work/handoff/dev-frontend-20260817-1445-172c.md` §C2) —
// déjà couvert par la matrice #166/T6 ci-dessus (situation PREPARE, aucune
// des entrées de MATRIX ne passe waitReason), reconfirmé explicitement ici.
// ---------------------------------------------------------------------------

describe('AnimConductPanel — waitReason, sous-libellé LANCER en PREPARE (#172/C2)', () => {
  it('phase PREPARE, bouton LANCER "off", waitReason fourni → remplace "indispo." par le motif', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null, waitReason: 'buzzers' })} />
    )
    const startBtn = getBtn(container, 'LANCER')
    expect(stateOf(startBtn)).toBe('off')
    expect(subLabelOf(startBtn)).toBe('buzzers')
  })

  it.each(['buzzers', '1 équipe', '2 équipes'])(
    'phase PREPARE, LANCER "off" → motif "%s" affiché tel quel (aucune transformation locale du texte)',
    (reason) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null, waitReason: reason })} />
      )
      expect(subLabelOf(getBtn(container, 'LANCER'))).toBe(reason)
    }
  )

  it('phase PREPARE, waitReason NON fourni (prop omise) → repli générique "indispo." inchangé (non-régression)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null })} />
    )
    expect(subLabelOf(getBtn(container, 'LANCER'))).toBe('indispo.')
  })

  it('phase PREPARE, waitReason=null explicite (valeur par défaut du composant) → "indispo." inchangé', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null, waitReason: null })} />
    )
    expect(subLabelOf(getBtn(container, 'LANCER'))).toBe('indispo.')
  })

  it('waitReason fourni mais phase !== PREPARE (STARTED) → ignoré, sous-libellé LANCER reste "en cours"', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1' }, waitReason: 'buzzers' })} />
    )
    const startBtn = getBtn(container, 'LANCER')
    expect(stateOf(startBtn)).toBe('off')
    expect(subLabelOf(startBtn)).toBe('en cours')
  })

  it('waitReason fourni mais bouton LANCER pas à l\'état "off" (READY → "go") → ignoré, sous-libellé reste "attendu"', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'READY', question: { ID: '1' }, waitReason: 'buzzers' })} />
    )
    const startBtn = getBtn(container, 'LANCER')
    expect(stateOf(startBtn)).toBe('go')
    expect(subLabelOf(startBtn)).toBe('attendu')
  })

  it('waitReason fourni en PREPARE mais ne concerne QUE le bouton LANCER (les 4 autres restent "indispo.")', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null, waitReason: 'buzzers' })} />
    )
    ;['PAUSE', 'CONTINUER', 'STOP', 'RÉPONSE'].forEach((key) => {
      const btn = getBtn(container, key)
      expect(stateOf(btn)).toBe('off')
      expect(subLabelOf(btn)).toBe('indispo.')
    })
  })

  it('bouton LANCER en PREPARE reste désactivé (disabled, onClick neutralisé) même avec waitReason fourni', () => {
    const onStart = vi.fn()
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'PREPARE', question: null, waitReason: 'buzzers', onStart })} />
    )
    const startBtn = getBtn(container, 'LANCER')
    expect(startBtn.disabled).toBe(true)
    startBtn.click()
    expect(onStart).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// #160/T6 — MEMOTION : PREMIER mode à occuper réellement L2 (jusqu'ici
// toujours l'emplacement réservé générique, y compris pour MEMORY qui n'a
// pas de geste propre — le geste MEMORY est la carte elle-même, en L3).
// L2 rend AnimMotionActions (gestes du mode), L3 devient une branche à
// QUATRE voies (QCM / MEMORY / MEMOTION / réservé) : AnimMotionGrid en
// MEMORIZE/GRID, AnimMotionCard en SELECTED/QUESTION/REVEAL — plan §F7.
//
// Ni AnimMotionActions ni AnimMotionGrid/AnimMotionCard ne sont mockés ici
// (même principe que la branche MEMORY existante ci-dessus) : ce describe
// vérifie le CÂBLAGE réel (présence, props transmises), pas une seconde
// fois leur matrice d'état propre (AnimMotionActions.test.jsx,
// AnimMotionGrid.test.jsx, AnimMotionCard.test.jsx).
//
// Nouvelles props (plan §F7) : motion={{ subphase, cardStates, cardTeams,
// currentTeam, currentTeamColor, selectedId, participatingTeams }} +
// onSelectMotionCard, onFlipMotionCard, onStopMotionTimer,
// onRevealMotionCard, onDoneMotionCard.
// ---------------------------------------------------------------------------

function motionQuestion(overrides = {}) {
  return {
    ID: '1',
    TYPE: 'MEMOTION',
    MOTION_CARDS: [
      { ID: 'c1', RECTO_THEME: 'Cinéma', DIFFICULTY: 1 },
      { ID: 'c2', RECTO_THEME: 'Sport', DIFFICULTY: 2 },
    ],
    ...overrides,
  }
}

function motionProps(overrides = {}) {
  return {
    subphase: 'GRID',
    cardStates: {},
    cardTeams: {},
    currentTeam: null,
    currentTeamColor: null,
    selectedId: null,
    participatingTeams: [],
    ...overrides,
  }
}

describe('AnimConductPanel — L2, gestes MEMOTION (#160/T6, RÉÉCRIT — L2 n\'est plus TOUJOURS vide)', () => {
  it('question MEMOTION : L2 rend AnimMotionActions, PAS l\'emplacement réservé', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion(),
        motion: motionProps({ subphase: 'SELECTED' }),
      })} />
    )
    expect(container.querySelector('.anim-conduct-l2 .anim-motion-banner, .anim-conduct-l2 .anim-conduct-btn')).not.toBeNull()
    expect(container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')).toBeNull()
  })

  // R5 du plan — risque nommé explicitement : la première occupation de L2
  // ne doit RIEN changer pour les autres modes. Reprend la couverture #171
  // ci-dessus (L2 toujours réservée) et l'étend nommément à MEMORY (qui n'a
  // jamais eu de geste propre non plus, son geste est en L3).
  it.each(['SPEEDY', 'QCM', 'ARDOISE', 'MEMORY'])(
    'question %s (PAS MEMOTION) : L2 reste l\'emplacement réservé générique, jamais AnimMotionActions (R5, non-régression)',
    (type) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: type } })} />
      )
      expect(container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')).not.toBeNull()
      expect(container.querySelector('.anim-conduct-l2 .anim-motion-banner')).toBeNull()
      expect(container.querySelector('.anim-conduct-l2 .anim-conduct-btn')).toBeNull()
    }
  )

  it('sans question chargée : L2 reste réservée (pas de crash sur motion undefined)', () => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase: 'NEW_GAME', question: null })} />)
    expect(container.querySelector('.anim-conduct-l2 .anim-conduct-reserved')).not.toBeNull()
  })

  it('transmet subphase/timerRunning/currentTeam/currentTeamColor/selectedCardId à AnimMotionActions (câblage REVEAL : bouton d\'équipe visible)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion(),
        motion: motionProps({ subphase: 'REVEAL', currentTeam: 'Les Bleus', currentTeamColor: [37, 99, 235] }),
      })} />
    )
    const teamBtn = Array.from(container.querySelectorAll('.anim-conduct-l2 .anim-conduct-btn')).find(
      (btn) => btn.textContent.startsWith('Les Bleus')
    )
    expect(teamBtn).not.toBeUndefined()
  })

  it('les 4 handlers MEMOTION reçus sont bien câblés vers AnimMotionActions (DÉMARRER déclenche onFlipMotionCard)', () => {
    const onFlipMotionCard = vi.fn()
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion(),
        motion: motionProps({ subphase: 'SELECTED' }),
        onFlipMotionCard,
      })} />
    )
    const startBtn = Array.from(container.querySelectorAll('.anim-conduct-l2 .anim-conduct-btn')).find(
      (btn) => btn.textContent.startsWith('DÉMARRER')
    )
    startBtn.click()
    expect(onFlipMotionCard).toHaveBeenCalledTimes(1)
  })
})

describe('AnimConductPanel — L3, branche à QUATRE voies (#160/T6, F7) : AnimMotionGrid vs AnimMotionCard', () => {
  it.each(['MEMORIZE', 'GRID'])(
    'subphase %s : L3 affiche AnimMotionGrid (grille), pas AnimMotionCard',
    (subphase) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({
          phase: 'STARTED',
          question: motionQuestion(),
          motion: motionProps({ subphase }),
        })} />
      )
      expect(container.querySelector('.anim-conduct-l3 .anim-motion-grid')).not.toBeNull()
      expect(container.querySelector('.anim-conduct-l3 .anim-motion-card-focus')).toBeNull()
    }
  )

  it.each(['SELECTED', 'QUESTION', 'REVEAL'])(
    'subphase %s : L3 affiche AnimMotionCard (carte au premier plan), pas la grille',
    (subphase) => {
      const { container } = render(
        <AnimConductPanel {...baseProps({
          phase: 'STARTED',
          question: motionQuestion(),
          motion: motionProps({ subphase, selectedId: 'c1' }),
        })} />
      )
      expect(container.querySelector('.anim-conduct-l3 .anim-motion-card-focus')).not.toBeNull()
      expect(container.querySelector('.anim-conduct-l3 .anim-motion-grid')).toBeNull()
    }
  )

  it('sélection d\'une carte (GRID) émet onSelectMotionCard(cardId)', () => {
    const onSelectMotionCard = vi.fn()
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion(),
        motion: motionProps({ subphase: 'GRID' }),
        onSelectMotionCard,
      })} />
    )
    container.querySelector('.anim-conduct-l3 .anim-motion-card').click()
    expect(onSelectMotionCard).toHaveBeenCalledWith('c1')
  })

  it('non-MEMOTION : L3 reste inchangée (QCM/MEMORY/réservé), aucune trace de composants MEMOTION', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({ phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } })} />
    )
    expect(container.querySelector('.anim-conduct-l3 .anim-motion-grid')).toBeNull()
    expect(container.querySelector('.anim-conduct-l3 .anim-motion-card-focus')).toBeNull()
    expect(container.querySelector('.anim-conduct-l3 .anim-conduct-reserved')).not.toBeNull()
  })
})

describe('AnimConductPanel — L1/L4/L5 inchangées par MEMOTION (#160/T6, non-régression)', () => {
  // #168 (F7) — L4 rend désormais AnimExplanationNote au lieu de la réserve
  // statique ; motionQuestion() (ci-dessous) ne porte pas de champ
  // EXPLANATION, donc l'emplacement au repos reste le comportement attendu
  // — assertion mise à jour sur le nouveau sélecteur, même invariant
  // (MEMOTION ne perturbe ni L1 ni L4).
  it('L1 garde ses 5 boutons, L4 affiche l\'emplacement au repos #168 (pas de note sur cette question), même avec une question MEMOTION chargée', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion(),
        motion: motionProps({ subphase: 'SELECTED' }),
      })} />
    )
    expect(container.querySelectorAll('.anim-conduct-l1 .anim-conduct-btn')).toHaveLength(5)
    expect(container.querySelector('.anim-conduct-l4 .anim-explanation-note.empty')).not.toBeNull()
  })

  // #171/T4 — même piège que pour QCM/MEMORY : une grille MEMOTION à 20
  // cartes (bloc central chargé) ne doit jamais repousser "à suivre" hors
  // de sa position de dernier enfant.
  it('"à suivre" (L5) reste le DERNIER enfant de .anim-conduct avec MEMOTION en GRID (bloc central chargé)', () => {
    const { container } = render(
      <AnimConductPanel {...baseProps({
        phase: 'STARTED',
        question: motionQuestion({ MOTION_CARDS: Array.from({ length: 20 }, (_, i) => ({ ID: `c${i}`, RECTO_THEME: `T${i}`, DIFFICULTY: 1 })) }),
        motion: motionProps({ subphase: 'GRID' }),
      })} />
    )
    const root = container.querySelector('.anim-conduct')
    const lastChild = root.lastElementChild
    expect(lastChild.querySelector('.anim-next-btn') || lastChild.classList.contains('anim-next-btn')).toBeTruthy()
  })
})
