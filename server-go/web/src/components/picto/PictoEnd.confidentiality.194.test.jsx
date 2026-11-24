import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import PictoTv from './PictoTv'
import { AnimPictoView } from './AnimPicto'
import PictoPanel from './PictoPanel'
import { createPictoCanvasStore } from './pictoCanvasStore'

// #194 (v13.0.0, LOT-2.3) — écrans de fin de manche PICTO, d'après contracts/picto.md §6.3 à §6.5, §10 et les maquettes
// M1 (anim), M2 (admin), M3 (TV, zones communes), M4 (tablettes). Confidentialité (critical) : sur la TV et sur les tablettes
// qui ne dessinent pas, le mot n'apparaît JAMAIS avant REVEALED, quelle que soit la phase et même si l'état porte déjà un mot
// résiduel ; en REVEALED seul le mot de la manche s'affiche, dans la zone réponse de la TV ; rien si NO_WORD.
// Révision 8 (2026-11-24, contrat picto.md §6.5 et §15, changement documenté dans contracts/CHANGELOG.md [20261124]) : TV
// et tablettes montrent le MOT SEUL à la révélation — jamais « Trouvé par … » ni « +N pts » propres à Picto ; le crédit se voit
// par l'animation commune (pages/PlayerDisplay.picto-credit.194.test.jsx) ; l'issue « ✅ Trouvé » (sans « par … ») reste sur
// l'admin et l'anim. Fichier à part de PictoEnd.dev194.test.jsx (dev-frontend).

vi.mock('../Timer', () => ({ default: () => <div data-testid="timer" /> }))
beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null })

const WORD = 'Zèbre-secret'
const OTHER_WORDS = ['Cornemuse-autre', 'Pont-levis-autre']
const base = {
  phase: 'STARTED', timer: 40, totalTime: 90,
  question: { TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90, PICTO_WORDS: [WORD, ...OTHER_WORDS] },
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
  PICTO_CANDIDATES: [], PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 3, PICTO_OUTCOME: '', PICTO_REVEALED_WORD: '',
}
const cat = { label: 'Animaux', icon: '🐾' }
const store = createPictoCanvasStore()
const pictoWord = { WORD, EPOCH: 3 }
const NON_REVEALED = ['PREPARE', 'READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED']

describe('TV — le mot n\'apparaît jamais avant REVEALED (§10, M3 zone réponse)', () => {
  it.each(NON_REVEALED)('phase %s : aucun mot, même si PICTO_REVEALED_WORD était résiduel dans l\'état', (phase) => {
    const { container } = render(
      <PictoTv gameState={{ ...base, phase, PICTO_OUTCOME: phase === 'STOPPED' ? 'TIMEOUT' : '', PICTO_REVEALED_WORD: WORD }} catMeta={cat} store={store} pictoWord={pictoWord} />,
    )
    expect(container.textContent).not.toContain(WORD)
    for (const o of OTHER_WORDS) expect(container.textContent).not.toContain(o)
    expect(screen.queryByTestId('picto-revealed-word')).toBeNull()
    expect(container.querySelector('.zone-answers').textContent).toBe('')
  })

  it('le prop pictoWord (réservé au dessinateur/admin/anim) est ignoré par la TV, même transmis par erreur', () => {
    const { container } = render(<PictoTv gameState={{ ...base, phase: 'STARTED' }} catMeta={cat} store={store} pictoWord={pictoWord} />)
    expect(container.textContent).not.toContain(WORD)
  })

  it('REVEALED : le mot de la manche est dans la zone réponse, JAMAIS les autres mots de la question', () => {
    const { container } = render(
      <PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: WORD }} catMeta={cat} store={store} />,
    )
    expect(container.querySelector('.zone-answers').textContent).toContain(WORD)
    expect(screen.getByTestId('picto-revealed-word').textContent).toBe(WORD)
    for (const o of OTHER_WORDS) expect(container.textContent).not.toContain(o)
    // rév. 8 : le mot SEUL dans la zone réponse — ni « Trouvé par … », ni points, ni équipe
    expect(container.querySelector('.zone-answers').textContent).toBe(WORD)
    expect(container.textContent).not.toMatch(/Trouvé par/)
    expect(container.textContent).not.toMatch(/\d+\s*pts?\b/)
    expect(screen.queryByTestId('picto-found-by')).toBeNull()
  })

  it.each(['FOUND', 'TIMEOUT', 'STOPPED'])('REVEALED, issue %s : toujours le mot seul sur la TV (aucune mention de l\'issue ni de l\'équipe)', (outcome) => {
    const { container } = render(
      <PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: outcome, PICTO_REVEALED_WORD: WORD }} catMeta={cat} store={store} />,
    )
    expect(container.querySelector('.zone-answers').textContent).toBe(WORD)
    expect(container.textContent).not.toMatch(/Trouvé|Temps écoulé|arrêtée/)
  })

  it('REVEALED sans mot (NO_WORD) : zone réponse vide, aucune mention de l\'issue', () => {
    const { container } = render(
      <PictoTv gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'NO_WORD', PICTO_REVEALED_WORD: '' }} catMeta={cat} store={store} />,
    )
    expect(container.querySelector('.zone-answers').textContent).toBe('')
    expect(screen.queryByTestId('picto-found-by')).toBeNull()
  })

  it('structure commune de la TV (M3) : chronomètre, bandeau, média, réponse ; écran statique sans défilement', () => {
    const { container } = render(<PictoTv gameState={{ ...base }} catMeta={cat} store={store} />)
    for (const z of ['.zone-timer', '.zone-question', '.zone-media', '.zone-answers']) expect(container.querySelector(z), z).not.toBeNull()
    expect(container.textContent).toMatch(/Alice/) // « ✏️ Alice dessine pour Bleue »
    expect(container.textContent).toMatch(/Animaux/) // indice : la catégorie
    const css = require('node:fs').readFileSync(require('node:path').join(__dirname, 'PictoTv.css'), 'utf8')
    expect(css).toMatch(/overflow:\s*hidden/)
    expect(css).not.toMatch(/overflow(-[xy])?:\s*(auto|scroll)/)
  })
})

describe('Tablettes qui ne dessinent pas — jamais le mot avant REVEALED (M4, §10)', () => {
  it.each(NON_REVEALED)('phase %s : équipe qui devine et autres équipes ne voient aucun mot', (phase) => {
    for (const teamName of ['Bleue', 'Rouge']) {
      const { container, unmount } = render(
        <PictoTv isVPlayer playerId="b9" teamName={teamName} store={store} pictoWord={pictoWord}
          gameState={{ ...base, phase, PICTO_REVEALED_WORD: WORD }} />,
      )
      expect(container.textContent, `${teamName}/${phase}`).not.toContain(WORD)
      expect(screen.queryByTestId('picto-canvas')).toBeNull() // jamais la feuille sur une tablette qui ne dessine pas
      unmount()
    }
  })

  it('messages des équipes (M4) : « Devinez ! » pour l\'équipe qui dessine, « … devine le dessin de … » pour les autres', () => {
    const { container, rerender } = render(<PictoTv isVPlayer playerId="b9" teamName="Bleue" store={store} gameState={{ ...base }} />)
    expect(container.textContent).toMatch(/Devinez !/)
    expect(container.textContent).toMatch(/Alice dessine — regardez la TV/)
    rerender(<PictoTv isVPlayer playerId="b9" teamName="Rouge" store={store} gameState={{ ...base }} />)
    expect(container.textContent).toMatch(/Bleue devine le dessin d'Alice/)
  })

  it.each([['Bleue', 'FOUND'], ['Rouge', 'FOUND'], ['Rouge', 'TIMEOUT'], ['Bleue', 'STOPPED']])('REVEALED (%s, issue %s) : « Mot révélé » et le mot, sans « Trouvé par » ni points', (team, outcome) => {
    const { container } = render(
      <PictoTv isVPlayer playerId="b9" teamName={team} store={store}
        gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: outcome, PICTO_REVEALED_WORD: WORD }} />,
    )
    expect(container.textContent).toContain(WORD)
    expect(container.textContent).toMatch(/Mot révélé/)
    expect(container.textContent).not.toMatch(/Trouvé par/)
    expect(container.textContent).not.toMatch(/\d+\s*pts?\b/)
    for (const o of OTHER_WORDS) expect(container.textContent).not.toContain(o)
    expect(screen.queryByTestId('picto-canvas')).toBeNull() // jamais la feuille sur une tablette qui ne dessine pas
  })
})

describe('Tablette du dessinateur — feuille et mot (M4)', () => {
  const props = { isVPlayer: true, playerId: 'b1', store, sendMessage: vi.fn() }

  it('préparation et retour de relance : « C\'est toi qui dessines ! » SANS le mot', () => {
    for (const phase of ['PREPARE', 'READY', 'COUNTDOWN']) {
      const { container, unmount } = render(<PictoTv {...props} gameState={{ ...base, phase }} pictoWord={pictoWord} />)
      expect(container.textContent, phase).toMatch(/C'est toi qui dessines/)
      expect(container.textContent, phase).not.toContain(WORD)
      unmount()
    }
  })

  it('le mot d\'une AUTRE époque (manche précédente) n\'est jamais affiché en manche', () => {
    const { container } = render(<PictoTv {...props} gameState={{ ...base, phase: 'STARTED' }} pictoWord={{ WORD: 'Ancien-mot', EPOCH: 2 }} />)
    expect(container.textContent).not.toContain('Ancien-mot')
  })

  it('mot en grand en manche, feuille sans aucun bouton (aucun outil, jamais d\'effacement)', () => {
    render(<PictoTv {...props} gameState={{ ...base, phase: 'STARTED' }} pictoWord={pictoWord} />)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe(WORD)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('pause : « Jeu en pause », feuille verrouillée ; révélation : feuille finale + mot, toujours sans bouton', () => {
    const { rerender } = render(<PictoTv {...props} gameState={{ ...base, phase: 'PAUSED' }} pictoWord={pictoWord} />)
    expect(screen.getByText('⏸ Jeu en pause')).toBeTruthy()
    rerender(<PictoTv {...props} gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: WORD }} />)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe(WORD)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('REVEALED (rév. 8) : « Mot révélé », le mot et la feuille finale, sans « Trouvé par » ni points', () => {
    const { container } = render(<PictoTv {...props} gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: WORD }} />)
    expect(container.textContent).toMatch(/Mot révélé/)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe(WORD)
    expect(container.querySelector('[data-testid="picto-input"], [data-testid="picto-canvas"]')).not.toBeNull() // la feuille finale reste
    expect(container.textContent).not.toMatch(/Trouvé par/)
    expect(container.textContent).not.toMatch(/\d+\s*pts?\b/)
  })
})

describe('Alerte « dessinateur déconnecté » (§6.7, admin et anim)', () => {
  const acts = { setTeam: vi.fn(), setDrawer: vi.fn(), drawDrawer: vi.fn(), found: vi.fn() }
  const teams = { Bleue: {} }
  const bumpers = { x: { TEAM: 'Bleue' } }
  const disconnected = { ...base, PICTO_DRAWER_CONNECTED: false }

  it.each(['STARTED', 'PAUSED'])('anim, manche en cours (%s) : alerte nommant le joueur, sans bouton de remplacement', (phase) => {
    const { container } = render(<AnimPictoView gameState={{ ...disconnected, phase }} teams={teams} bumpers={bumpers} actions={acts} store={store} pictoWord={pictoWord} />)
    expect(screen.getByTestId('picto-disconnected').textContent).toMatch(/Alice/)
    expect(screen.getByTestId('picto-disconnected').textContent).toMatch(/relancez la question/)
    expect(container.textContent).not.toMatch(/remplacer/i) // jamais de remplacement du dessinateur en manche (I6)
  })

  it('admin : même alerte', () => {
    render(<PictoPanel gameState={disconnected} actions={acts} pictoWord={pictoWord} />)
    expect(screen.getByTestId('picto-disconnected').textContent).toMatch(/Alice/)
  })

  it('aucune alerte tant que le dessinateur est connecté', () => {
    render(<AnimPictoView gameState={base} teams={teams} bumpers={bumpers} actions={acts} store={store} pictoWord={pictoWord} />)
    expect(screen.queryByTestId('picto-disconnected')).toBeNull()
  })

  it('admin et anim voient le mot de la manche (conduite), et le mot révélé à la révélation', () => {
    const { rerender } = render(<AnimPictoView gameState={base} teams={teams} bumpers={bumpers} actions={acts} store={store} pictoWord={pictoWord} />)
    expect(screen.getByTestId('anim-picto-word').textContent).toBe(WORD)
    rerender(<AnimPictoView gameState={{ ...base, phase: 'REVEALED', PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: WORD }} teams={teams} bumpers={bumpers} actions={acts} store={store} />)
    expect(screen.getByTestId('anim-picto-word').textContent).toBe(WORD)
  })
})

describe('Issue de la manche sur l\'admin et l\'anim : « ✅ Trouvé » sans « par … » (rév. 8)', () => {
  const acts = { setTeam: vi.fn(), setDrawer: vi.fn(), drawDrawer: vi.fn(), found: vi.fn() }
  const teams = { Bleue: {} }
  const bumpers = { x: { TEAM: 'Bleue' } }
  const revealed = (outcome) => ({ ...base, phase: 'REVEALED', PICTO_OUTCOME: outcome, PICTO_REVEALED_WORD: WORD })

  it('anim, FOUND : « ✅ Trouvé », sans le nom de l\'équipe ni « par »', () => {
    render(<AnimPictoView gameState={revealed('FOUND')} teams={teams} bumpers={bumpers} actions={acts} store={store} />)
    const el = screen.getByTestId('anim-picto-outcome')
    expect(el.textContent).toMatch(/✅ Trouvé/)
    expect(el.textContent).not.toMatch(/Trouvé par/)
    expect(el.querySelector('b').textContent).toBe('✅ Trouvé')
  })

  it('admin, FOUND : « ✅ Trouvé », sans « par … »', () => {
    render(<PictoPanel gameState={revealed('FOUND')} actions={acts} />)
    const el = screen.getByTestId('picto-outcome')
    expect(el.textContent).toMatch(/✅ Trouvé/)
    expect(el.textContent).not.toMatch(/Trouvé par/)
  })

  it.each([['TIMEOUT', /Temps écoulé/], ['STOPPED', /arrêtée/]])('anim et admin, issue %s : libellé propre, sans « Trouvé »', (outcome, label) => {
    const { unmount } = render(<AnimPictoView gameState={revealed(outcome)} teams={teams} bumpers={bumpers} actions={acts} store={store} />)
    expect(screen.getByTestId('anim-picto-outcome').textContent).toMatch(label)
    expect(screen.getByTestId('anim-picto-outcome').textContent).not.toMatch(/Trouvé/)
    unmount()
    render(<PictoPanel gameState={revealed(outcome)} actions={acts} />)
    expect(screen.getByTestId('picto-outcome').textContent).toMatch(label)
  })
})

