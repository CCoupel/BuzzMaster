import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import RafaleTimers from './RafaleTimers'

// ---------------------------------------------------------------------------
// RafaleTimers — double timer du mode RAFALE (milestone v8.0.0 #16/#198,
// contrat contracts/rafale.md §2.2/§4). Deux tickers tournent
// SIMULTANÉMENT et INDÉPENDAMMENT en RAFALE : le timer de MANCHE (réutilise
// le timer global, ~2mn) et le timer de QUESTION (~3s, RAFALE_TICK). Ce
// composant monte Timer.jsx (mono-valeur, non modifié) en DEUX instances —
// framer-motion est auto-mocké via l'alias vite.config.js (Timer.jsx
// l'importe), donc Timer se rend réellement ici sans mock supplémentaire.
//
// Composant déjà livré par dev-frontend au moment où ce fichier est écrit
// (Batch 1) — tests écrits contre son API réelle (props: roundTime/
// roundTotal/questionTime/questionTotal/phase/size/showBar/className).
// ---------------------------------------------------------------------------

function timerDisplays(container) {
  return Array.from(container.querySelectorAll('.timer-display')).map((el) => el.textContent)
}

function mmss(totalSeconds) {
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

describe('RafaleTimers — deux comptes à rebours indépendants', () => {
  it('rend exactement 2 Timer (manche + question)', () => {
    const { container } = render(
      <RafaleTimers roundTime={90} roundTotal={120} questionTime={2} questionTotal={3} phase="STARTED" />
    )
    expect(timerDisplays(container)).toHaveLength(2)
  })

  it('les 2 comptes à rebours affichent des valeurs DIFFÉRENTES et INDÉPENDANTES l\'une de l\'autre', () => {
    const { container } = render(
      <RafaleTimers roundTime={90} roundTotal={120} questionTime={2} questionTotal={3} phase="STARTED" />
    )
    const [roundDisplay, questionDisplay] = timerDisplays(container)
    expect(roundDisplay).toBe(mmss(90))
    expect(questionDisplay).toBe(mmss(2))
    expect(roundDisplay).not.toBe(questionDisplay)
  })

  it('faire varier UNIQUEMENT questionTime laisse le timer de manche inchangé (rendu indépendant)', () => {
    const { container: c1 } = render(
      <RafaleTimers roundTime={90} roundTotal={120} questionTime={3} questionTotal={3} phase="STARTED" />
    )
    const roundBefore = timerDisplays(c1)[0]

    const { container: c2 } = render(
      <RafaleTimers roundTime={90} roundTotal={120} questionTime={0} questionTotal={3} phase="STARTED" />
    )
    const [roundAfter, questionAfter] = timerDisplays(c2)

    expect(roundAfter).toBe(roundBefore) // manche inchangée
    expect(questionAfter).toBe(mmss(0)) // question, elle, a bien changé
  })

  it('faire varier UNIQUEMENT roundTime laisse le timer de question inchangé (rendu indépendant)', () => {
    const { container: c1 } = render(
      <RafaleTimers roundTime={120} roundTotal={120} questionTime={2} questionTotal={3} phase="STARTED" />
    )
    const questionBefore = timerDisplays(c1)[1]

    const { container: c2 } = render(
      <RafaleTimers roundTime={5} roundTotal={120} questionTime={2} questionTotal={3} phase="STARTED" />
    )
    const [roundAfter, questionAfter] = timerDisplays(c2)

    expect(questionAfter).toBe(questionBefore) // question inchangée
    expect(roundAfter).toBe(mmss(5)) // manche, elle, a bien changé
  })

  it('libellés "Manche" et "Question" présents (maquette §3/§4)', () => {
    const { container } = render(
      <RafaleTimers roundTime={90} roundTotal={120} questionTime={2} questionTotal={3} phase="STARTED" />
    )
    const labels = Array.from(container.querySelectorAll('.rafale-timer-label')).map((el) => el.textContent)
    expect(labels).toEqual(['Manche', 'Question'])
  })

  it('valeurs par défaut (aucune prop) : ne plante pas, 2 Timer à 00:00', () => {
    const { container } = render(<RafaleTimers />)
    expect(timerDisplays(container)).toEqual([mmss(0), mmss(0)])
  })
})
