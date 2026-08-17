import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimNextButton from './AnimNextButton'

// ---------------------------------------------------------------------------
// AnimNextButton — bouton "à suivre" de la conduite /anim (#166/F4, T5).
//
// Extrait d'AnimConductPanel.jsx (#163/#165, où il vivait conditionnellement
// dans le bandeau) puis déplacé en conduite, juste après L1
// (AnimConductPanel.jsx monte <AnimNextButton /> entre `.anim-conduct-l1` et
// `.anim-conduct-l2` — plus aucune trace en zone contexte, cf. AnimPage.jsx
// et le retrait documenté dans AnimPage.test.jsx). Plan de référence :
// _work/reports/plan-20260815-144925.md, tâche F4. Matrice de référence :
// _work/mockups/anim-conduct-permanent-166.html, colonne "À suivre".
//
// Contrat de props : `phase` (gameState.phase), `question` (gameState.
// question — la question COURANTE, nécessaire pour distinguer STOPPED
// "jouée"/"non jouée" via phaseRules.canReveal), `nextQuestion` (dernier
// NEXT_QUESTION reçu), `onSelectNext`. État = `nextButtonState(phase,
// question)` (phaseRules.js) SAUF si `!nextQuestion?.ID` → forcé 'inert'
// (cas limite "fin du quiz", pas une règle de phase).
// ---------------------------------------------------------------------------

const NEXT = { ID: '7', TYPE: 'QCM', QUESTION: 'Qui a peint la nuit étoilée?', POINTS: '20', TIME: '30' }

function stateOf(container) {
  const btn = container.querySelector('.anim-next-btn')
  const m = btn.className.match(/anim-next-btn-(go|optional|inert)\b/)
  return m ? m[1] : null
}

describe('AnimNextButton — matrice des 9 phases (#166/T5, maquette)', () => {
  // phase -> [état attendu, question courante associée]
  const CASES = [
    ['NEW_GAME', 'go', null], // B2 : pointe la 1ʳᵉ question jouable, seule action possible
    ['ENROLL', 'inert', null], // Ready() refusé par le moteur pendant l'enrôlement
    ['PREPARE', 'go', { ID: '1' }],
    ['READY', 'optional', { ID: '1' }], // LANCER dispo à côté -> bypass
    ['COUNTDOWN', 'inert', { ID: '1' }],
    ['STARTED', 'inert', { ID: '1' }],
    ['PAUSED', 'inert', { ID: '1' }],
    ['STOPPED', 'optional', { ID: '1', STATUS: 'STOPPED' }], // "jouée" : RÉPONSE dispo à côté -> bypass
    ['REVEALED', 'go', { ID: '1' }],
  ]

  it.each(CASES)('phase %s -> état %s', (phase, expected, question) => {
    const { container } = render(
      <AnimNextButton phase={phase} question={question} nextQuestion={NEXT} onSelectNext={() => {}} />
    )
    expect(stateOf(container)).toBe(expected)
  })

  // Dixième ligne de la matrice, même phase STOPPED que ci-dessus mais
  // "non jouée" (question.STATUS !== 'STOPPED', ou pas de question courante
  // du tout) : seule action possible -> vert, pas bleu. C'est le cas que
  // phaseRules.nextButtonState(phase) SEUL (sans `question`) ne pouvait pas
  // distinguer — c'est pour ça que la prop `question` existe.
  it('phase STOPPED "non jouée" (question.STATUS ≠ STOPPED) -> go, pas optional', () => {
    const { container } = render(
      <AnimNextButton phase="STOPPED" question={{ ID: '1', STATUS: 'AVAILABLE' }} nextQuestion={NEXT} onSelectNext={() => {}} />
    )
    expect(stateOf(container)).toBe('go')
  })

  it('phase STOPPED sans question courante du tout -> go (pas de canReveal possible)', () => {
    const { container } = render(
      <AnimNextButton phase="STOPPED" question={null} nextQuestion={NEXT} onSelectNext={() => {}} />
    )
    expect(stateOf(container)).toBe('go')
  })
})

describe('AnimNextButton — fin du quiz (nextQuestion sans ID)', () => {
  it('force l\'état "inert" même en phase où nextButtonState renverrait "go" (NEW_GAME)', () => {
    const { container } = render(
      <AnimNextButton phase="NEW_GAME" question={null} nextQuestion={null} onSelectNext={() => {}} />
    )
    expect(stateOf(container)).toBe('inert')
  })

  it('force l\'état "inert" même en phase où nextButtonState renverrait "optional" (READY)', () => {
    const { container } = render(
      <AnimNextButton phase="READY" question={{ ID: '1' }} nextQuestion={{}} onSelectNext={() => {}} />
    )
    expect(stateOf(container)).toBe('inert')
  })

  it('affiche un libellé dédié ("Fin du quiz") au lieu du format habituel', () => {
    render(<AnimNextButton phase="REVEALED" question={null} nextQuestion={null} onSelectNext={() => {}} />)
    expect(screen.getByText('Fin du quiz')).toBeInTheDocument()
    expect(screen.getByText('À suivre')).toBeInTheDocument()
  })

  it('ne rend pas de bloc points/délai en fin de quiz (rien à formater)', () => {
    const { container } = render(
      <AnimNextButton phase="REVEALED" question={null} nextQuestion={null} onSelectNext={() => {}} />
    )
    expect(container.querySelector('.anim-next-btn-meta')).toBeNull()
  })
})

describe('AnimNextButton — contenu (format partagé #163, nextQuestionFormat.js)', () => {
  it('affiche le format exact GATE 2 (énoncé + points/délai) quand une question suivante existe', () => {
    render(<AnimNextButton phase="REVEALED" question={null} nextQuestion={NEXT} onSelectNext={() => {}} />)
    expect(screen.getByText('#7 QCM: Qui a peint la nuit étoilée?')).toBeInTheDocument()
    expect(screen.getByText('20pt 30s')).toBeInTheDocument()
  })
})

describe('AnimNextButton — onSelectNext jamais appelé en état inerte', () => {
  it('inert (phase STARTED) : le clic n\'appelle pas onSelectNext', () => {
    const onSelectNext = vi.fn()
    render(<AnimNextButton phase="STARTED" question={{ ID: '1' }} nextQuestion={NEXT} onSelectNext={onSelectNext} />)
    screen.getByRole('button').click()
    expect(onSelectNext).not.toHaveBeenCalled()
  })

  it('inert (fin du quiz) : le clic n\'appelle pas onSelectNext', () => {
    const onSelectNext = vi.fn()
    render(<AnimNextButton phase="REVEALED" question={null} nextQuestion={null} onSelectNext={onSelectNext} />)
    screen.getByRole('button').click()
    expect(onSelectNext).not.toHaveBeenCalled()
  })

  it('le bouton porte l\'attribut natif disabled en état inerte (double garde, pas seulement le handler)', () => {
    render(<AnimNextButton phase="STARTED" question={{ ID: '1' }} nextQuestion={NEXT} onSelectNext={() => {}} />)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('go/optional : le clic appelle onSelectNext avec l\'ID de la question suivante', () => {
    const onSelectNext = vi.fn()
    render(<AnimNextButton phase="REVEALED" question={null} nextQuestion={NEXT} onSelectNext={onSelectNext} />)
    const btn = screen.getByRole('button')
    expect(btn).not.toBeDisabled()
    btn.click()
    expect(onSelectNext).toHaveBeenCalledWith('7')
  })
})
