import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import AnimConductPanel from './AnimConductPanel'

// #253 — conduite /anim : popup « Lancer la partie » ouverte sur le poste régie → les cinq gestes de L1
// sont grisés (état 'off', libellé « suspendu ») et n'émettent aucune action.

const props = (over = {}) => ({
  phase: 'STARTED', question: { TYPE: 'NORMAL', STATUS: 'STARTED' }, qcmInvalidated: [], revealed: false, nextQuestion: null,
  onStart: vi.fn(), onPause: vi.fn(), onContinue: vi.fn(), onStop: vi.fn(), onReveal: vi.fn(), onSelectNext: vi.fn(),
  ...over,
})
const l1 = (c) => [...c.querySelectorAll('.anim-conduct-l1 .anim-conduct-btn')]

describe('AnimConductPanel — conduite suspendue par le verrou (#253)', () => {
  it('launchLocked : tous les gestes L1 sont éteints, libellé « suspendu », aucune action émise', () => {
    const p = props({ launchLocked: true })
    const { container } = render(<AnimConductPanel {...p} />)
    const btns = l1(container)
    expect(btns).toHaveLength(5)
    btns.forEach((b) => {
      expect(b.className).toMatch(/anim-conduct-btn-off/)
      expect(b.disabled).toBe(true)
      expect(b.querySelector('.anim-conduct-btn-sub').textContent).toBe('suspendu')
      fireEvent.click(b)
    })
    expect(p.onStop).not.toHaveBeenCalled()
    expect(p.onPause).not.toHaveBeenCalled()
  })

  it('sans verrou : STOP reste actif (non-régression)', () => {
    const p = props()
    const { container } = render(<AnimConductPanel {...p} />)
    const stop = l1(container).find((b) => b.textContent.startsWith('STOP'))
    expect(stop.disabled).toBe(false)
    fireEvent.click(stop)
    expect(p.onStop).toHaveBeenCalled()
  })
})
