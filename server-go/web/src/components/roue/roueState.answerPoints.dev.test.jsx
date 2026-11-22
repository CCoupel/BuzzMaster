// QUALIF 12.1.0.2 — « Une réponse ? À voix haute » s'AJOUTE aux points gagnés (FLIP de consonne), il ne les remplace pas.
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import RoueTv from './RoueTv'

const base = {
  phase: 'STARTED', question: { TYPE: 'ROUE' }, ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_TEAMS: ['Rouges'], ROUE_CURRENT_TEAM: 'Rouges',
  ROUE_TEAM_BANK: { Rouges: 0 }, ROUE_BOARD: { COLS: 14, ROWS: ['RR'] }, ROUE_LETTERS: [{ LETTER: 'R', TEAM: 'Rouges', HIT: true }],
  ROUE_SHOW_LETTERS: true, ROUE_TURN_GAIN: 1500, ROUE_WHEEL: { SEGMENTS: [{ KIND: 'value', VALUE: 500 }], CURRENT: 0, LAYOUT: 'strip' },
}
const flip = (o) => ({ ...base, ROUE_LAST_EVENT: { SEQ: 5, KIND: 'FLIP', TEAM: 'Rouges', LETTER: 'R', COUNT: 3, VALUE: 0, GAIN: 0, ...o } })
const eventText = (gs, props = {}) => render(<RoueTv gameState={gs} {...props} />).container.querySelector('.roue-event, .roue-vp-msg')

describe('étape 6 après une consonne : points ET « Une réponse »', () => {
  it('TV : « R : 3 × 500 = +1 500 » puis, en dessous, « Une réponse ? À voix haute »', () => {
    const el = eventText(flip())
    expect(el.textContent).toContain('R : 3 × 500 = ')
    expect(el.textContent).toContain('1 500')
    expect(el.textContent).toContain('Une réponse ? À voix haute')
    expect(el.querySelector('br')).toBeTruthy()
    expect(el.textContent.indexOf('+1')).toBeLessThan(el.textContent.indexOf('Une réponse'))
  })
  it('tablette joueur : les deux aussi', () => {
    const el = eventText(flip(), { isVPlayer: true, teamName: 'Rouges' })
    expect(el.textContent).toContain('3 × 500')
    expect(el.textContent).toContain('Une réponse ? À voix haute')
  })
  it('voyelle (étape 6 après achat) : inchangé, texte seul', () => {
    const el = eventText(flip({ LETTER: 'E', GAIN: 750 }))
    expect(el.textContent).toBe('Une réponse ? À voix haute')
  })
})
