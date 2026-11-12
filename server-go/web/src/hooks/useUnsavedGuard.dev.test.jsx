import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { useUnsavedGuard } from './useUnsavedGuard'

function Probe({ dirty }) {
  useUnsavedGuard(dirty)
  return (
    <div>
      <a href="/admin/quiz">Quiz</a>
      <a href={window.location.pathname}>Ici</a>
      <a href="https://exemple.org/x">Dehors</a>
      <a href="/admin/x" target="_blank">Onglet</a>
      <a href="/api/lighting/effects/a/download" download="a.yaml">Télécharger</a>
    </div>
  )
}
afterEach(() => { vi.restoreAllMocks() })

describe('useUnsavedGuard (#210)', () => {
  it('modifié : un lien interne vers une autre page demande confirmation ; refus = navigation annulée', () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<Probe dirty />)
    const ev = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 })
    screen.getByText('Quiz').dispatchEvent(ev)
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(ev.defaultPrevented).toBe(true)
  })
  it('confirmé : la navigation continue', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<Probe dirty />)
    const ev = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 })
    screen.getByText('Quiz').dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(false)
  })
  it('pas de question : page courante, lien externe, nouvel onglet, téléchargement, ou rien de modifié', () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { rerender } = render(<Probe dirty />)
    for (const t of ['Ici', 'Dehors', 'Onglet', 'Télécharger']) fireEvent.click(screen.getByText(t))
    expect(confirm).not.toHaveBeenCalled()
    rerender(<Probe dirty={false} />)
    fireEvent.click(screen.getByText('Quiz'))
    expect(confirm).not.toHaveBeenCalled()
  })
  it('fermeture / rechargement : beforeunload demande la confirmation du navigateur seulement si modifié', () => {
    const { rerender } = render(<Probe dirty />)
    const ev = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
    rerender(<Probe dirty={false} />)
    const ev2 = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(ev2)
    expect(ev2.defaultPrevented).toBe(false)
  })
  it('Précédent : confirmation ; refus = la garde est remise en place', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const push = vi.spyOn(window.history, 'pushState')
    render(<Probe dirty />)
    expect(push).toHaveBeenCalledTimes(1) // entrée « garde »
    window.dispatchEvent(new PopStateEvent('popstate'))
    expect(window.confirm).toHaveBeenCalled()
    expect(push).toHaveBeenCalledTimes(2)
  })
  it('Précédent confirmé : la page est quittée (history.back)', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const back = vi.spyOn(window.history, 'back').mockImplementation(() => {})
    render(<Probe dirty />)
    window.dispatchEvent(new PopStateEvent('popstate'))
    expect(back).toHaveBeenCalledTimes(1)
  })
})
