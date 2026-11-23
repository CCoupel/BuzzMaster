import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import PictoPanel from './PictoPanel'
import { AnimPictoView } from './AnimPicto'
import PictoTv from './PictoTv'
import { createPictoCanvasStore } from './pictoCanvasStore'
import { pictoLaunchBlocked, pictoLaunchReason, pictoShownWord } from './pictoState'

vi.mock('../Timer', () => ({ default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div> }))
beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null })

const base = {
  phase: 'READY', timer: 72, totalTime: 90,
  question: { TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90, PICTO_WORDS: ['Maison', 'Chien'] },
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
  PICTO_CANDIDATES: [{ ID: 'b1', NAME: 'Alice', CONNECTED: true }, { ID: 'b2', NAME: 'Bruno', CONNECTED: true }, { ID: 'b3', NAME: 'Chloé', CONNECTED: false }],
  PICTO_WORDS_COUNT: 2, PICTO_CANVAS_EPOCH: 3, PICTO_REVEALED_WORD: '',
}
const teamList = [{ name: 'Bleue', color: [0, 0, 255] }, { name: 'Rouge', color: [255, 0, 0] }]
const actions = () => ({ setTeam: vi.fn(), setDrawer: vi.fn(), drawDrawer: vi.fn() })

describe('pictoState', () => {
  it('Lancer est bloqué sans dessinateur connecté, avec la raison', () => {
    expect(pictoLaunchBlocked({ ...base, PICTO_DRAWER: '' })).toBe(true)
    expect(pictoLaunchReason({ ...base, PICTO_DRAWER: '' })).toBe('choisir le dessinateur')
    expect(pictoLaunchBlocked({ ...base, PICTO_DRAWER_CONNECTED: false })).toBe(true)
    expect(pictoLaunchBlocked(base)).toBe(false)
    expect(pictoLaunchBlocked({ question: { TYPE: 'SPEEDY' } })).toBe(false)
  })
  it('le mot n\'est montré qu\'en manche, de l\'époque courante ; révélé seulement en REVEALED', () => {
    const w = { WORD: 'Maison', EPOCH: 3 }
    expect(pictoShownWord({ ...base, phase: 'STARTED' }, w)).toBe('Maison')
    expect(pictoShownWord({ ...base, phase: 'STARTED' }, { ...w, EPOCH: 2 })).toBe('')
    expect(pictoShownWord({ ...base, phase: 'READY' }, w)).toBe('')
    expect(pictoShownWord({ ...base, phase: 'REVEALED', PICTO_REVEALED_WORD: 'Chien' }, w)).toBe('Chien')
  })
})

describe('PictoPanel (admin, M2) et préparation', () => {
  it('choisit l\'équipe, un joueur connecté, ou tire au hasard ; un joueur hors ligne n\'est pas sélectionnable', () => {
    const a = actions()
    render(<PictoPanel gameState={base} actions={a} teamList={teamList} />)
    fireEvent.click(screen.getByRole('button', { name: /Rouge/ }))
    expect(a.setTeam).toHaveBeenCalledWith('Rouge')
    fireEvent.click(screen.getByRole('button', { name: /Bruno/ }))
    expect(a.setDrawer).toHaveBeenCalledWith('b2')
    expect(screen.getByRole('button', { name: /Chloé/ }).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: /Au hasard/ }))
    expect(a.drawDrawer).toHaveBeenCalled()
  })
  it('affiche le nombre de mots et la catégorie ; pas de mot de la manche en préparation', () => {
    render(<PictoPanel gameState={base} actions={actions()} teamList={teamList} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} />)
    expect(screen.getByText(/2 mots/)).toBeTruthy()
    expect(screen.queryByTestId('picto-word')).toBeNull()
  })
  it('en manche : mot secret affiché ; alerte si le dessinateur est déconnecté', () => {
    const gs = { ...base, phase: 'STARTED', PICTO_DRAWER_CONNECTED: false }
    render(<PictoPanel gameState={gs} actions={actions()} teamList={teamList} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} />)
    expect(screen.getByTestId('picto-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-disconnected').textContent).toMatch(/Alice/)
  })
})

describe('AnimPictoView (anim, M1)', () => {
  const teams = { Bleue: { COLOR: [0, 0, 255] }, Rouge: { COLOR: [255, 0, 0] }, Vide: {} }
  const bumpers = { x: { TEAM: 'Bleue' }, y: { TEAM: 'Rouge' } }
  it('préparation : seules les équipes avec des joueurs sont proposées', () => {
    render(<AnimPictoView gameState={base} teams={teams} bumpers={bumpers} actions={actions()} />)
    expect(screen.getByRole('button', { name: /Bleue/ })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Vide/ })).toBeNull()
    expect(screen.getByText(/mots dans la question/)).toBeTruthy()
  })
  it('manche : mot et gain, pas de bouton de remplacement du dessinateur', () => {
    render(<AnimPictoView gameState={{ ...base, phase: 'STARTED' }} teams={teams} bumpers={bumpers} actions={actions()} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} store={createPictoCanvasStore()} />)
    expect(screen.getByTestId('anim-picto-word').textContent).toBe('Maison')
    expect(screen.getByText(/\+10 pts/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Changer/ })).toBeNull()
  })
})

describe('PictoTv — TV (M3) et tablettes (M4)', () => {
  const cat = { label: 'Animaux', icon: '🐾' }
  const store = createPictoCanvasStore()
  it('TV : indice, chronomètre commun, feuille ; aucun mot avant la révélation (même si pictoWord est fourni)', () => {
    const { container } = render(<PictoTv gameState={{ ...base, phase: 'STARTED' }} catMeta={cat} store={store} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} />)
    expect(screen.getByTestId('timer')).toBeTruthy()
    expect(screen.getByTestId('picto-hint').textContent).toMatch(/Animaux/)
    expect(screen.getByTestId('picto-hint').textContent).toMatch(/Alice/)
    expect(screen.getByTestId('picto-canvas')).toBeTruthy()
    expect(container.textContent).not.toMatch(/Maison/)
    expect(container.textContent).not.toMatch(/Chien/)
  })
  it('TV : le mot révélé apparaît dans la zone réponse en REVEALED seulement', () => {
    render(<PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison' }} catMeta={cat} store={store} />)
    expect(screen.getByTestId('picto-revealed-word').textContent).toBe('Maison')
  })
  it('dessinateur : « C\'est toi qui dessines ! » en préparation, puis mot et feuille sans aucun bouton', () => {
    const send = vi.fn()
    const { rerender } = render(<PictoTv isVPlayer playerId="b1" gameState={base} store={store} sendMessage={send} />)
    expect(screen.getByText("C'est toi qui dessines !")).toBeTruthy()
    rerender(<PictoTv isVPlayer playerId="b1" gameState={{ ...base, phase: 'STARTED' }} store={store} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} sendMessage={send} />)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-input')).toBeTruthy()
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })
  it('dessinateur qui recharge en manche : redemande la feuille', () => {
    const send = vi.fn()
    render(<PictoTv isVPlayer playerId="b1" gameState={{ ...base, phase: 'STARTED' }} store={store} sendMessage={send} />)
    expect(send).toHaveBeenCalledWith('PICTO_CANVAS_SYNC', {})
  })
  it('équipe qui dessine : « Devinez ! » ; autres équipes : attente ; aucune ne reçoit le mot ni la feuille', () => {
    const gs = { ...base, phase: 'STARTED' }
    const { container, rerender } = render(<PictoTv isVPlayer playerId="b2" teamName="Bleue" gameState={gs} store={store} pictoWord={{ WORD: 'Maison', EPOCH: 3 }} />)
    expect(screen.getByText('Devinez !')).toBeTruthy()
    expect(container.textContent).not.toMatch(/Maison/)
    expect(screen.queryByTestId('picto-canvas')).toBeNull()
    rerender(<PictoTv isVPlayer playerId="b9" teamName="Rouge" gameState={gs} store={store} />)
    expect(screen.getByText(/devine le dessin d'Alice/)).toBeTruthy()
  })
})
