import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import PictoPanel from './PictoPanel'
import { AnimPictoView } from './AnimPicto'
import PictoTv from './PictoTv'
import { createPictoCanvasStore } from './pictoCanvasStore'
import { canPictoFound, outcomeLabel, makePictoActions } from './pictoState'
import { resolveAwardLevel } from '../../utils/pointsAward'

// #194 (LOT-2.3, tâche 12) — fin de manche : « Trouvé », temps écoulé, révélation, crédit, alerte de déconnexion.
vi.mock('../Timer', () => ({ default: () => <div data-testid="timer" /> }))
beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null })

const base = {
  phase: 'STARTED', timer: 40, totalTime: 90,
  question: { TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
  PICTO_CANDIDATES: [], PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 3, PICTO_OUTCOME: '', PICTO_REVEALED_WORD: '',
}
const w = { WORD: 'Maison', EPOCH: 3 }
const acts = () => ({ setTeam: vi.fn(), setDrawer: vi.fn(), drawDrawer: vi.fn(), found: vi.fn() })

describe('pictoState — issues', () => {
  it('Trouvé proposé en manche, et après le gong tant que l\'issue est TIMEOUT (I5) ; jamais ailleurs', () => {
    expect(canPictoFound({ ...base })).toBe(true)
    expect(canPictoFound({ ...base, phase: 'PAUSED' })).toBe(true)
    expect(canPictoFound({ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' })).toBe(true)
    expect(canPictoFound({ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'STOPPED' })).toBe(false)
    expect(canPictoFound({ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'NO_WORD' })).toBe(false)
    expect(canPictoFound({ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND' })).toBe(false)
    expect(canPictoFound({ ...base, phase: 'READY' })).toBe(false)
  })
  it('libellés des 4 issues ; « Trouvé » sans nom d\'équipe', () => {
    expect(outcomeLabel({ ...base, PICTO_OUTCOME: 'FOUND' })).toBe('✅ Trouvé')
    expect(outcomeLabel({ ...base, PICTO_OUTCOME: 'TIMEOUT' })).toMatch(/Temps écoulé/)
    expect(outcomeLabel({ ...base, PICTO_OUTCOME: 'STOPPED' })).toMatch(/arrêtée/)
    expect(outcomeLabel({ ...base, PICTO_OUTCOME: 'NO_WORD' })).toMatch(/Aucun mot/)
  })
  it('PICTO_FOUND est envoyé sans charge', () => {
    const send = vi.fn(); makePictoActions(send).found()
    expect(send).toHaveBeenCalledWith('PICTO_FOUND', {})
  })
  it('crédit : PICTO est toujours attribué à l\'équipe (pas de score individuel du dessinateur)', () => {
    expect(resolveAwardLevel({ TYPE: 'PICTO', POINTS_TARGET: 'PLAYER' })).toBe('TEAM')
    expect(resolveAwardLevel({ TYPE: 'PICTO' })).toBe('TEAM')
  })
})

describe('PictoPanel (admin) — fin de manche', () => {
  it('manche en cours : « Trouvé ! » envoie PICTO_FOUND', () => {
    const a = acts()
    render(<PictoPanel gameState={base} actions={a} pictoWord={w} />)
    fireEvent.click(screen.getByTestId('picto-found'))
    expect(a.found).toHaveBeenCalled()
  })
  it('temps écoulé : « Trouvé (sur le gong) » proposé, mot secret toujours affiché à l\'admin, explication de Reveal', () => {
    render(<PictoPanel gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' }} actions={acts()} pictoWord={w} />)
    expect(screen.getByTestId('picto-found').textContent).toMatch(/sur le gong/)
    expect(screen.getByTestId('picto-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-outcome').textContent).toMatch(/Reveal/)
  })
  it('arrêt : pas de « Trouvé »', () => {
    render(<PictoPanel gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'STOPPED' }} actions={acts()} pictoWord={w} />)
    expect(screen.queryByTestId('picto-found')).toBeNull()
  })
  it('révélé : mot révélé et crédit par la colonne Équipes', () => {
    render(<PictoPanel gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison' }} actions={acts()} />)
    expect(screen.getByTestId('picto-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-outcome').textContent).toMatch(/Trouvé.*colonne Équipes/)
  })
})

describe('AnimPictoView — fin de manche', () => {
  const teams = { Bleue: {} }; const bumpers = { x: { TEAM: 'Bleue' } }
  it('Trouvé en manche ; Trouvé + Révéler après le gong', () => {
    const a = acts(); const onReveal = vi.fn()
    const { rerender } = render(<AnimPictoView gameState={base} teams={teams} bumpers={bumpers} actions={a} pictoWord={w} store={createPictoCanvasStore()} onReveal={onReveal} />)
    fireEvent.click(screen.getByTestId('anim-picto-found')); expect(a.found).toHaveBeenCalled()
    expect(screen.queryByTestId('anim-picto-reveal')).toBeNull()
    rerender(<AnimPictoView gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' }} teams={teams} bumpers={bumpers} actions={a} pictoWord={w} store={createPictoCanvasStore()} onReveal={onReveal} />)
    expect(screen.getByTestId('anim-picto-found').textContent).toMatch(/gong/)
    fireEvent.click(screen.getByTestId('anim-picto-reveal')); expect(onReveal).toHaveBeenCalled()
  })
  it('arrêt / sans mot : Révéler seulement ; révélé : issue et mot', () => {
    const { rerender } = render(<AnimPictoView gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'NO_WORD' }} teams={teams} bumpers={bumpers} actions={acts()} store={createPictoCanvasStore()} onReveal={() => {}} />)
    expect(screen.queryByTestId('anim-picto-found')).toBeNull()
    expect(screen.getByTestId('anim-picto-reveal')).toBeTruthy()
    rerender(<AnimPictoView gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison' }} teams={teams} bumpers={bumpers} actions={acts()} store={createPictoCanvasStore()} />)
    expect(screen.getByTestId('anim-picto-word').textContent).toBe('Maison')
    expect(screen.getByTestId('anim-picto-outcome').textContent).toMatch(/✅ Trouvé/)
    expect(screen.getByTestId('anim-picto-outcome').textContent).not.toMatch(/par Bleue/)
  })
  it('alerte dessinateur déconnecté en manche', () => {
    render(<AnimPictoView gameState={{ ...base, PICTO_DRAWER_CONNECTED: false }} teams={teams} bumpers={bumpers} actions={acts()} store={createPictoCanvasStore()} />)
    expect(screen.getByTestId('picto-disconnected').textContent).toMatch(/Alice/)
  })
})

describe('PictoTv — révélation et tablettes', () => {
  const cat = { label: 'Animaux', icon: '🐾' }; const store = createPictoCanvasStore()
  it('TV : le mot révélé seul — ni « Trouvé par … » ni « +N pts » (l\'animation de gain commune s\'en charge)', () => {
    render(<PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison' }} catMeta={cat} store={store} />)
    expect(screen.getByTestId('picto-revealed-word').textContent).toBe('Maison')
    expect(screen.queryByTestId('picto-found-by')).toBeNull()
    expect(document.body.textContent).not.toMatch(/Trouvé par|\+10 pts/)
  })
  it('TV : même mot seul pour les autres issues ; rien du tout si aucun mot (NO_WORD)', () => {
    const { rerender, container } = render(<PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'TIMEOUT', PICTO_REVEALED_WORD: 'Maison' }} catMeta={cat} store={store} />)
    expect(screen.getByTestId('picto-revealed-word')).toBeTruthy()
    expect(screen.queryByTestId('picto-found-by')).toBeNull()
    rerender(<PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'NO_WORD', PICTO_REVEALED_WORD: '' }} catMeta={cat} store={store} />)
    expect(screen.queryByTestId('picto-revealed-word')).toBeNull()
    expect(container.querySelector('.zone-answers').textContent).toBe('')
  })
  it('TV : temps écoulé, la zone réponse reste vide jusqu\'à la révélation', () => {
    const { container } = render(<PictoTv gameState={{ ...base, phase: 'STOPPED', timer: 0, PICTO_OUTCOME: 'TIMEOUT' }} catMeta={cat} store={store} pictoWord={w} />)
    expect(container.querySelector('.zone-answers').textContent).toBe('')
    expect(container.textContent).not.toMatch(/Maison/)
  })
  it('dessinateur : feuille verrouillée avec le motif (pause, temps écoulé, arrêt), puis « Mot révélé » et le mot en révélation', () => {
    const props = { isVPlayer: true, playerId: 'b1', store, sendMessage: vi.fn() }
    const { rerender } = render(<PictoTv {...props} gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' }} pictoWord={w} />)
    expect(screen.getByText('⏰ Temps écoulé')).toBeTruthy()
    rerender(<PictoTv {...props} gameState={{ ...base, phase: 'STOPPED', PICTO_OUTCOME: 'STOPPED' }} pictoWord={w} />)
    expect(screen.getByText('⏹ Manche arrêtée')).toBeTruthy()
    rerender(<PictoTv {...props} gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison' }} />)
    expect(screen.getByText('Mot révélé')).toBeTruthy()
    expect(document.body.textContent).not.toMatch(/Trouvé par/)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe('Maison')
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })
  it('autres tablettes en révélation : « Mot révélé » puis le mot seul, sans feuille', () => {
    render(<PictoTv isVPlayer playerId="b9" teamName="Rouge" store={store} gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison' }} />)
    expect(screen.getByTestId('picto-guesser-revealed').textContent).toMatch(/Mot révélé.*Maison/)
    expect(screen.getByTestId('picto-guesser-revealed').textContent).not.toMatch(/Trouvé par/)
    expect(screen.queryByTestId('picto-canvas')).toBeNull()
  })
})
