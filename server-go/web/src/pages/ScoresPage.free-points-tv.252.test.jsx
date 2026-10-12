import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react'
import ScoresPage from './ScoresPage'
import { useGame } from '../hooks/GameContext'

// #252 (v11.2, lot V2-3) — page « Gestion des Scores » selon la maquette VALIDÉE admin_scores__free_points_tv_toggle :
// champ « autre » (entier non nul de −999 à 999, le signe se choisit avec −/+), interrupteur « 📺 Afficher sur la TV »
// (activé à chaque ouverture), bandeau « correction cachée » + marque 🔇, équipes ET joueurs, « Dernières attributions ».
// Scénario 10 du plan (planner-20261011-213000.md). Boîte noire par libellés / rôles ARIA.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./ScoresPage.css', () => ({}))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, layout, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: new Proxy({}, { get: (_, tag) => makeEl(tag) }), AnimatePresence: ({ children }) => children }
})

const TEAMS = {
  'Les Bleus': { NAME: 'Les Bleus', SCORE: 42, COLOR: [37, 99, 235] },
  'Les Verts': { NAME: 'Les Verts', SCORE: 37, COLOR: [22, 163, 74] },
}
const BUMPERS = {
  'AA:01': { NAME: 'Paul', TEAM: 'Les Bleus', SCORE: 12 },
  'AA:02': { NAME: 'Mamie', TEAM: 'Les Verts', SCORE: 10 },
}
const mk = () => ({ teams: TEAMS, bumpers: BUMPERS, setTeamPoints: vi.fn(), setBumperPoints: vi.fn() })
const history = (n = 7) => Array.from({ length: n }, (_, i) => ({
  EVENT_TYPE: 'POINTS', TIMESTAMP: 1000 + i, TEAM_NAME: `Team${i}`, WINNER_TYPE: 'TEAM', POINTS: i + 1, QUESTION_ID: String(i), DISPLAY: i === 6 ? false : undefined,
}))

let game
beforeEach(() => {
  game = mk()
  useGame.mockReturnValue(game)
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(history()) })))
})
afterEach(() => vi.unstubAllGlobals())

const freeInput = () => screen.getByLabelText('Nombre de points libre (négatif autorisé)')
const tvSwitch = () => screen.getByRole('switch', { name: 'Afficher les attributions sur la TV' })
const give = (n) => screen.getAllByRole('button', { name: new RegExp(`^Donner ${n} point\\(s\\)`) })
const take = (n) => screen.getAllByRole('button', { name: new RegExp(`^Retirer ${n} point\\(s\\)`) })
const type = (v) => fireEvent.change(freeInput(), { target: { value: v } })

describe('ScoresPage — montant libre (#252)', () => {
  it('prédéfinis 1 / 2 / 5 / 10 conservés, champ « autre » présent, boutons −1 / +1 par défaut sur chaque ligne (équipes ET joueurs)', () => {
    render(<ScoresPage />)
    for (const v of ['1', '2', '5', '10']) expect(screen.getByRole('button', { name: v })).toBeTruthy()
    expect(freeInput().getAttribute('placeholder')).toBe('autre')
    expect(give(1).length).toBe(4) // 2 équipes + 2 joueurs
    expect(take(1).length).toBe(4)
  })

  it('Critical — saisie libre 7 : boutons « −7 / +7 » sur toutes les lignes, envoyés tels quels', () => {
    render(<ScoresPage />)
    type('7')
    expect(give(7).length).toBe(4)
    expect(take(7).length).toBe(4)

    fireEvent.click(give(7)[0])
    fireEvent.click(take(7)[0])
    const calls = game.setTeamPoints.mock.calls.concat(game.setBumperPoints.mock.calls)
    expect(calls.map((c) => c[1]).sort((a, b) => a - b)).toEqual([-7, 7])
  })

  it('saisie libre négative −4 : le signe se choisit avec −/+, boutons « −4 / +4 » (un nombre négatif revient au même)', () => {
    render(<ScoresPage />)
    type('-4')
    expect(give(4).length).toBe(4)
    expect(take(4).length).toBe(4)
    fireEvent.click(give(4)[0])
    expect(game.setTeamPoints.mock.calls[0][1]).toBe(4)
    fireEvent.click(take(4)[0])
    expect(game.setTeamPoints.mock.calls[1][1]).toBe(-4)
  })

  it('Critical — le montant s’applique aussi aux JOUEURS (setBumperPoints)', () => {
    render(<ScoresPage />)
    type('3')
    const playerSection = screen.getByRole('heading', { name: 'Joueurs' }).closest('section')
    fireEvent.click(within(playerSection).getAllByRole('button', { name: /^Donner 3 point/ })[0])
    expect(game.setBumperPoints).toHaveBeenCalledTimes(1)
    expect(game.setBumperPoints.mock.calls[0][0]).toMatch(/^AA:/)
    expect(game.setBumperPoints.mock.calls[0][1]).toBe(3)
  })

  it.each(['999', '-999', '1', '-1'])('bornes : %s accepté (aucune erreur, boutons actifs)', (v) => {
    render(<ScoresPage />)
    type(v)
    expect(screen.queryByRole('alert')).toBeNull()
    expect(give(Math.abs(Number(v)))[0].disabled).toBe(false)
  })

  it.each(['2.5', '0', '1000', '-1000'])('Critical — saisie invalide %s : erreur « Entier de −999 à 999 », boutons −/+ désactivés, rien envoyé', (v) => {
    render(<ScoresPage />)
    type(v)
    expect(screen.getByRole('alert').textContent).toBe('Entier de −999 à 999')
    expect(freeInput().getAttribute('aria-invalid')).toBe('true')
    const all = [...screen.getAllByRole('button', { name: /^(Donner|Retirer) \d+ point/ })]
    expect(all.length).toBeGreaterThan(0)
    for (const b of all) expect(b.disabled).toBe(true)
    fireEvent.click(all[0])
    expect(game.setTeamPoints).not.toHaveBeenCalled()
  })

  it('effacer le champ ou choisir un prédéfini revient au montant prédéfini', () => {
    render(<ScoresPage />)
    type('7')
    fireEvent.click(screen.getByRole('button', { name: '5' }))
    expect(freeInput().value).toBe('')
    expect(give(5).length).toBe(4)
    type('7'); type('')
    expect(give(5).length).toBe(4) // dernier prédéfini conservé
  })
})

describe('ScoresPage — « Afficher sur la TV » (#252)', () => {
  it('Critical — interrupteur ACTIVÉ par défaut ; aucun bandeau ni 🔇 ; l’envoi ne porte pas DISPLAY:false (comportement v11.1)', () => {
    render(<ScoresPage />)
    expect(tvSwitch().getAttribute('aria-checked')).toBe('true')
    expect(screen.queryByText(/Correction cachée/)).toBeNull()
    expect(document.body.textContent).not.toMatch(/🔇/)
    fireEvent.click(give(1)[0])
    const opts = game.setTeamPoints.mock.calls[0][2]
    expect(opts?.display).not.toBe(false)
  })

  it('Critical — désactivé : bandeau « correction cachée », marque 🔇 et « sans affichage TV » sur tous les boutons, envoi display:false (équipes ET joueurs)', () => {
    render(<ScoresPage />)
    fireEvent.click(tvSwitch())

    expect(tvSwitch().getAttribute('aria-checked')).toBe('false')
    expect(screen.getByText(/Correction cachée : les points sont enregistrés sans animation, sans son ni LED/)).toBeTruthy()
    const buttons = screen.getAllByRole('button', { name: /point\(s\) sans affichage TV$/ })
    expect(buttons.length).toBe(8) // 4 lignes × (−, +)
    for (const b of buttons) expect(b.textContent).toMatch(/🔇/)

    fireEvent.click(screen.getAllByRole('button', { name: /^Donner 1 point\(s\) sans affichage TV$/ })[0])
    fireEvent.click(screen.getAllByRole('button', { name: /^Retirer 1 point\(s\) sans affichage TV$/ })[3])
    const all = game.setTeamPoints.mock.calls.concat(game.setBumperPoints.mock.calls)
    expect(all.length).toBe(2)
    for (const c of all) expect(c[2]).toEqual(expect.objectContaining({ display: false }))
  })

  it('réactiver l’interrupteur retire bandeau et marques', () => {
    render(<ScoresPage />)
    fireEvent.click(tvSwitch())
    fireEvent.click(tvSwitch())
    expect(screen.queryByText(/Correction cachée/)).toBeNull()
    expect(document.body.textContent).not.toMatch(/🔇/)
  })

  it('Critical — réactivé à CHAQUE ouverture de la page (état de page, jamais sauvegardé : pas de correction cachée oubliée)', () => {
    const first = render(<ScoresPage />)
    fireEvent.click(tvSwitch())
    expect(tvSwitch().getAttribute('aria-checked')).toBe('false')
    first.unmount()

    render(<ScoresPage />)
    expect(tvSwitch().getAttribute('aria-checked')).toBe('true')
  })

  it('libellé d’état : « animation + son + LED » / « correction cachée »', () => {
    render(<ScoresPage />)
    expect(screen.getByText('animation + son + LED')).toBeTruthy()
    fireEvent.click(tvSwitch())
    expect(screen.getAllByText('correction cachée').length).toBeGreaterThan(0)
  })

  it('le montant libre et l’interrupteur se combinent : −4 caché', () => {
    render(<ScoresPage />)
    type('-4')
    fireEvent.click(tvSwitch())
    fireEvent.click(take(4)[0])
    const c = game.setTeamPoints.mock.calls[0]
    expect(c[1]).toBe(-4)
    expect(c[2]).toEqual(expect.objectContaining({ display: false }))
  })
})

describe('ScoresPage — « Dernières attributions » (#252, maquette)', () => {
  it('Critical — section présente, 5 entrées les plus récentes (même source que l’Historique), la correction cachée marquée 🔇', async () => {
    render(<ScoresPage />)
    const section = (await screen.findByRole('heading', { name: 'Dernières attributions' })).closest('section')
    await waitFor(() => expect(within(section).getAllByRole('listitem').length).toBe(5))
    const items = within(section).getAllByRole('listitem').map((li) => li.textContent)
    expect(items.some((t) => /Team6/.test(t))).toBe(true) // la plus récente
    expect(items.some((t) => /Team0|Team1\b/.test(t))).toBe(false) // les plus anciennes sont écartées
    const hidden = within(section).getAllByRole('listitem').filter((li) => /🔇/.test(li.textContent))
    expect(hidden.length).toBe(1)
    expect(hidden[0].textContent).toMatch(/Team6/)
  })

  it('un historique indisponible laisse la section vide sans erreur', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('réseau'))))
    render(<ScoresPage />)
    const section = (await screen.findByRole('heading', { name: 'Dernières attributions' })).closest('section')
    expect(within(section).queryAllByRole('listitem').length).toBe(0)
  })

  it('TEAM_DELETED (ligne d’audit #249) n’apparaît jamais dans les dernières attributions', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([...history(2), { EVENT_TYPE: 'TEAM_DELETED', TEAM_NAME: 'Fantôme', TIMESTAMP: 9999, POINTS: 0 }]) })))
    render(<ScoresPage />)
    const section = (await screen.findByRole('heading', { name: 'Dernières attributions' })).closest('section')
    await waitFor(() => expect(within(section).getAllByRole('listitem').length).toBe(2))
    expect(section.textContent).not.toMatch(/Fantôme/)
  })
})

describe('ScoresPage — « Dernières attributions » : rechargement et crédit nul (#252, revue)', () => {
  const historyCalls = () => global.fetch.mock.calls.filter((c) => String(c[0]).includes('/history')).length

  it('ne relance pas /history à chaque UPDATE (équipes, joueurs, état) : seulement quand SCORE_CHANGE.SEQ change', async () => {
    const base = { ...mk(), gameState: { SCORE_CHANGE: { SEQ: 3, DISPLAY: true, TEAMS: [], PLAYERS: [] } } }
    useGame.mockReturnValue(base)
    const { rerender } = render(<ScoresPage />)
    await screen.findByRole('heading', { name: 'Dernières attributions' })
    await waitFor(() => expect(historyCalls()).toBeGreaterThan(0))
    const afterMount = historyCalls()

    // UPDATE sans nouvelle attribution : nouveaux objets équipes / joueurs / gameState, même SEQ.
    for (let i = 0; i < 3; i++) {
      useGame.mockReturnValue({
        ...base,
        teams: { ...TEAMS },
        bumpers: { ...BUMPERS },
        gameState: { SCORE_CHANGE: { SEQ: 3, DISPLAY: true, TEAMS: [], PLAYERS: [] }, PHASE: 'STOPPED' },
      })
      rerender(<ScoresPage />)
    }
    await new Promise((r) => setTimeout(r, 50))
    expect(historyCalls()).toBe(afterMount)

    // Nouvelle attribution : SEQ change → un rechargement.
    useGame.mockReturnValue({ ...base, gameState: { SCORE_CHANGE: { SEQ: 4, DISPLAY: true, TEAMS: ['Les Bleus'], PLAYERS: [] } } })
    rerender(<ScoresPage />)
    await waitFor(() => expect(historyCalls()).toBe(afterMount + 1))
  })

  it('un crédit à 0 s’affiche « 0 pt » (jamais « −0 » ni « +0 »)', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({
      ok: true,
      json: () => Promise.resolve([{ EVENT_TYPE: 'POINTS', TIMESTAMP: 5000, TEAM_NAME: 'Les Bleus', WINNER_TYPE: 'TEAM', POINTS: 0, QUESTION_ID: '' }]),
    })))
    render(<ScoresPage />)
    const section = (await screen.findByRole('heading', { name: 'Dernières attributions' })).closest('section')
    await waitFor(() => expect(within(section).getAllByRole('listitem').length).toBe(1))
    const text = within(section).getAllByRole('listitem')[0].textContent
    expect(text).toMatch(/0 pt/)
    expect(text).not.toMatch(/[−-]0/)
    expect(text).not.toMatch(/\+0/)
  })
})
