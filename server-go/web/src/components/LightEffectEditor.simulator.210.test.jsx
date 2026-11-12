import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

// #210 (v11.8.0, LOT-2, F2.3) — simulateur multi-salles, pas à pas et « Essayer sur la salle » de l'éditeur.
// Source : contracts/light-effects.md §11.8 (pas à pas, aperçu multi-tailles, essai), §11.10 (draft/try), §7.3 (`vars`, trames) ;
// maquette docs/mockup/v11.8.0/ui/ambiance_page__effect_editor.html (§01 « Simulateur » et « Essayer sur la salle »).
//
// SURFACE SUPPOSÉE (écrite avant le code de dev-frontend ; à rapprocher de son handoff — seuls `mount()` et les libellés ci-dessous
// changent) : tout passe par <LightEffectEditor id zoneGroups onClose onSaved /> et des appels `fetch` simulés ; les libellés sont ceux de la
// maquette : boutons « ⏮ » (title « Revenir au début »), « ◀ tick » (« Tick précédent »), « ▶ Lecture » / « ⏸ Pause » (« Lecture / pause »),
// « tick ▶ » (« Tick suivant »), réglette (title « Position dans l'aperçu (tick) »), rangées « 1 groupe », « 3 groupes », « 6 groupes »,
// « 12 groupes », « Zone Ambiance — 4 groupes » avec case à cocher, ampoules « g1 »…, variables « gA = g3 », horloge
// « tick N · pas k · chronomètre simulé : S s », « sous le minimum : non joué en salle », zones « Ambiance · 4 groupes »…, « Essayer 10 s »,
// « ■ Arrêter l'essai ». Le simulateur ne rejoue QUE les trames du serveur (aucune requête de plus au pas à pas).

vi.mock('./LightEffectEditor.css', () => ({}))
import LightEffectEditor from './LightEffectEditor'

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const DOC = {
  id: 'mon-effet', name: 'Mon effet', description: '', moments: ['chrono'], min_groups: 3, tick_ms: 1000, loop: true,
  variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }], pre: [A('g0', 'scene')], steps: [[A('g0'), A('gA', 'urgence')], [A('g*')]], last: [A('g0', 'scene', 50)], post: null,
}
const SOURCE = { id: 'mon-effet', origin: 'user', file: 'mon-effet.yaml', read_only: false, yaml: 'id: mon-effet\n', effect: DOC, has_comments: false, custom_order: false, grid: { editable: true, effect: DOC, converted: [], blocked: [] } }

// Séquence du serveur : tick 0 = PRE, 1 = pas:0, 2 = pas:1, 3 = DERNIER, 4 = pas:0, 5 = pas:1, 6 = DERNIER
const SEQ = ['pre', 'pas:0', 'pas:1', 'dernier', 'pas:0', 'pas:1', 'dernier']
const preview = (n) => ({
  id: 'mon-effet', moment: 'chrono', groups: n, tick_ms: 1000, duration_ms: 20000, chrono_s: 20, loop: true,
  frames: SEQ.map((step, tick) => ({
    at_ms: tick * 1000, tick, step,
    groups: Array.from({ length: n }, (_, i) => ({ color: i === tick % n ? [255, 255, 255] : [79, 70, 229], intensity: 255 })),
    vars: { gA: step === 'pre' ? 0 : ((tick % n) + 1) },
  })),
})
const json = (status, data) => ({ ok: status < 400, status, json: async () => data })

let drafts, tries, stops, tryAnswer
const install = ({ draftOk = true } = {}) => {
  drafts = []; tries = []; stops = 0
  tryAnswer = () => json(200, { ok: true, duration_ms: 10000 })
  global.fetch = vi.fn(async (url, opts = {}) => {
    const u = String(url)
    if (/\/source$/.test(u)) return json(200, SOURCE)
    if (/\/draft\/try$/.test(u)) { tries.push(JSON.parse(opts.body)); return tryAnswer() }
    if (/\/try\/stop$/.test(u)) { stops += 1; return json(200, { ok: true }) }
    if (/\/draft$/.test(u)) {
      const b = JSON.parse(opts.body); drafts.push(b)
      return draftOk ? json(200, { ok: true, errors: [], effect: DOC, yaml: 'id: mon-effet\n', has_comments: false, custom_order: false, grid: { editable: true, effect: DOC, converted: [], blocked: [] }, moment: 'chrono', previews: (b.groups || []).map(preview) })
        : json(200, { ok: false, errors: [{ line: 0, path: 'steps[0]', message: 'cassé' }], effect: null, yaml: '', grid: null, previews: [] })
    }
    return json(404, {})
  })
}
const ZONES = { ambiance: 4, animateur: 3, public: 0 }
const mount = async (props = {}) => {
  const utils = render(<LightEffectEditor id="mon-effet" zoneGroups={ZONES} onClose={vi.fn()} onSaved={vi.fn()} {...props} />)
  await screen.findByTestId('effect-editor')
  await screen.findByText(/^tick 0/, {}, { timeout: 3000 }) // état final : première trame du serveur affichée
  await waitFor(() => expect(screen.queryByText('Vérification par le serveur…')).toBeNull())
  return utils
}
const row = (label) => screen.getByText(label, { selector: 'label, label *' }).closest('.row, [data-testid^="sim-row"], label')?.parentElement
const clock = () => screen.getByText(/^tick \d+/)
const goto = (btnTitle) => fireEvent.click(screen.getByTitle(btnTitle))
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('rangées du simulateur (§11.8 « Aperçu »)', () => {
  it('quatre salles types (1, 3, 6, 12 groupes) plus une rangée par zone réelle de taille différente', async () => {
    install()
    await mount()
    for (const l of ['1 groupe', '3 groupes', '6 groupes', '12 groupes']) expect(screen.getByText(l)).toBeInTheDocument()
    expect(screen.getByText('Zone Ambiance — 4 groupes')).toBeInTheDocument()
    expect(screen.queryByText(/Zone Animateur/)).toBeNull() // 3 groupes : déjà représenté
    expect(screen.queryByText(/Zone Public/)).toBeNull() // aucun groupe : zone absente
  })

  it('les tailles demandées au serveur : 1, 3, 6, 12 puis les zones réelles distinctes', async () => {
    install()
    await mount()
    await waitFor(() => expect(drafts.some(d => JSON.stringify(d.groups) === JSON.stringify([1, 3, 6, 12, 4]))).toBe(true))
  })

  it('une ampoule par groupe, étiquetée g1, g2…', async () => {
    install()
    await mount()
    const r = within(screen.getByText('3 groupes').closest('.row') || screen.getByText('3 groupes').parentElement.parentElement)
    for (const g of ['g1', 'g2', 'g3']) expect(r.getByText(g)).toBeInTheDocument()
    expect(r.queryByText('g4')).toBeNull()
  })

  it('une case à cocher masque / rétablit une rangée', async () => {
    install()
    await mount()
    const label = screen.getByText('6 groupes').closest('label')
    const box = within(label).getByRole('checkbox')
    expect(box).toBeChecked()
    fireEvent.click(box)
    expect(box).not.toBeChecked()
    expect(label.closest('.row')).toHaveClass('off')
    fireEvent.click(box)
    expect(label.closest('.row')).not.toHaveClass('off')
  })

  it('« sous le minimum : non joué en salle » sur les rangées plus petites que groupes_min (3), pas sur les autres', async () => {
    install()
    await mount()
    expect(within(screen.getByText('1 groupe').closest('label')).getByText(/sous le minimum : non joué en salle/)).toBeInTheDocument()
    expect(within(screen.getByText('3 groupes').closest('label')).queryByText(/sous le minimum/)).toBeNull()
    expect(within(screen.getByText('12 groupes').closest('label')).queryByText(/sous le minimum/)).toBeNull()
  })
})

describe('pas à pas (§11.8) — aucune requête de plus', () => {
  it('l\'horloge affiche le tick, le pas joué et le chronomètre simulé ; PRE au tick 0', async () => {
    install()
    await mount()
    expect(clock()).toHaveTextContent('tick 0 · PRE · chronomètre simulé : 20 s')
  })

  it('« tick ▶ » avance d\'un tick : pas 0, pas 1, DERNIER ; « ◀ tick » recule ; « ⏮ » revient au début', async () => {
    install()
    await mount()
    const n = global.fetch.mock.calls.length
    goto('Tick suivant'); expect(clock()).toHaveTextContent(/^tick 1 · pas 0/)
    goto('Tick suivant'); expect(clock()).toHaveTextContent(/^tick 2 · pas 1/)
    goto('Tick suivant'); expect(clock()).toHaveTextContent(/^tick 3 · DERNIER/)
    goto('Tick précédent'); expect(clock()).toHaveTextContent(/^tick 2 · pas 1/)
    goto('Revenir au début'); expect(clock()).toHaveTextContent(/^tick 0 · PRE/)
    expect(global.fetch.mock.calls.length).toBe(n) // le pas à pas se déplace dans les trames déjà reçues
  })

  it('« tick précédent » au tick 0 reste au début', async () => {
    install()
    await mount()
    goto('Tick précédent')
    expect(clock()).toHaveTextContent(/^tick 0/)
  })

  it('la réglette se déplace sur un tick', async () => {
    install()
    await mount()
    fireEvent.change(screen.getByTitle("Position dans l'aperçu (tick)"), { target: { value: '4' } })
    expect(clock()).toHaveTextContent(/^tick 4 · pas 0/)
  })

  it('variables : le groupe réel désigné par chaque variable est LU dans vars, par rangée (— = aucun)', async () => {
    install()
    await mount()
    expect(within(screen.getByText('6 groupes').closest('.row')).getByText(/gA = —/)).toBeInTheDocument() // tick 0 : PRE, vars.gA = 0
    goto('Tick suivant')
    expect(within(screen.getByText('6 groupes').closest('.row')).getByText(/gA = g2/)).toBeInTheDocument()
    expect(within(screen.getByText('3 groupes').closest('.row')).getByText(/gA = g2/)).toBeInTheDocument()
    goto('Tick suivant')
    expect(within(screen.getByText('3 groupes').closest('.row')).getByText(/gA = g3/)).toBeInTheDocument()
  })

  it('lecture / pause : une seule horloge pour toutes les rangées ; Pause fige le tick', async () => {
    install()
    await mount()
    vi.useFakeTimers()
    fireEvent.click(screen.getByRole('button', { name: /Lecture/ }))
    expect(screen.getByRole('button', { name: /Pause/ })).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    const t = Number(/^tick (\d+)/.exec(clock().textContent)[1])
    expect(t).toBeGreaterThanOrEqual(2)
    fireEvent.click(screen.getByRole('button', { name: /Pause/ }))
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(Number(/^tick (\d+)/.exec(clock().textContent)[1])).toBe(t)
    expect(screen.getByRole('button', { name: /Lecture/ })).toBeInTheDocument()
  })

  it('un pas à pas pendant la lecture met en pause', async () => {
    install()
    await mount()
    vi.useFakeTimers()
    fireEvent.click(screen.getByRole('button', { name: /Lecture/ }))
    fireEvent.click(screen.getByTitle('Tick suivant'))
    expect(screen.getByRole('button', { name: /Lecture/ })).toBeInTheDocument()
  })

  it('le pas joué est surligné dans la grille ; le clic sur l\'en-tête d\'un pas place l\'aperçu sur son premier tick', async () => {
    install()
    await mount()
    goto('Tick suivant') // pas 0
    expect(screen.getByTestId('cell-0-g1').closest('td')).toHaveClass('is-playing')
    goto('Tick suivant') // pas 1
    expect(screen.getByTestId('cell-1-g1').closest('td')).toHaveClass('is-playing')
    expect(screen.getByTestId('cell-0-g1').closest('td')).not.toHaveClass('is-playing')
    fireEvent.click(screen.getAllByTitle("Placer l'aperçu sur ce pas")[2]) // PRE, pas 0, pas 1 : « pas 1 »
    expect(clock()).toHaveTextContent(/^tick 2 · pas 1/)
  })

  it('un clic sur PRE ou DERNIER place l\'aperçu sur leur tick', async () => {
    install()
    await mount()
    fireEvent.click(screen.getAllByTitle("Placer l'aperçu sur ce pas")[0])
    expect(clock()).toHaveTextContent(/^tick 0 · PRE/)
    const last = screen.getAllByTitle("Placer l'aperçu sur ce pas").find(b => b.textContent === 'DERNIER')
    fireEvent.click(last)
    expect(clock()).toHaveTextContent(/^tick 3 · DERNIER/)
  })

  it('brouillon modifié : le simulateur reprend les nouvelles trames du serveur sans plantage', async () => {
    install()
    await mount()
    goto('Tick suivant')
    fireEvent.click(screen.getByTestId('cell-0-g4'))
    await waitFor(() => expect(drafts.length).toBeGreaterThan(1), { timeout: 2000 })
    await waitFor(() => expect(screen.queryByText('Vérification par le serveur…')).toBeNull())
    expect(clock()).toHaveTextContent(/^tick \d+/)
  })

  it('brouillon invalide : pas de trames, message à la place des rangées (pas de plantage)', async () => {
    install({ draftOk: false })
    render(<LightEffectEditor id="mon-effet" zoneGroups={ZONES} onClose={vi.fn()} />)
    await screen.findByTestId('effect-editor')
    await waitFor(() => expect(drafts.length).toBeGreaterThan(0))
    await waitFor(() => expect(screen.queryByText('Vérification par le serveur…')).toBeNull())
    expect(screen.queryByText(/^tick \d+/)).toBeNull()
  })
})

describe('Essayer sur la salle (E7, §11.10)', () => {
  it('zones existantes seulement ; Ambiance choisie par défaut ; Public « aucun groupe » inactive', async () => {
    install()
    await mount()
    expect(screen.getByText('Ambiance · 4 groupes')).toBeInTheDocument()
    expect(screen.getByText('Animateur · 3 groupes')).toBeInTheDocument()
    const pub = screen.getByText('Public · aucun groupe')
    expect(pub.closest('button, [role="radio"], .chip')).toHaveAttribute('title', 'Aucun groupe dans la zone Public')
    fireEvent.click(pub)
    expect(screen.getByRole('button', { name: /Essayer 10 s/ })).toBeEnabled()
  })

  it('« Essayer 10 s » envoie le BROUILLON à POST …/draft/try avec la zone, sans rien enregistrer', async () => {
    install()
    await mount()
    fireEvent.click(screen.getByText('Animateur · 3 groupes'))
    fireEvent.click(screen.getByRole('button', { name: /Essayer 10 s/ }))
    await waitFor(() => expect(tries).toHaveLength(1))
    expect(tries[0].zone).toBe('animateur')
    expect(tries[0]).toHaveProperty('effect')
    expect(tries[0]).not.toHaveProperty('groups')
    expect(global.fetch.mock.calls.some(([, o]) => o?.method === 'PUT')).toBe(false)
  })

  it('pendant l\'essai le bouton devient « ■ Arrêter l\'essai » ; l\'arrêt appelle POST …/try/stop', async () => {
    install()
    await mount()
    fireEvent.click(screen.getByRole('button', { name: /Essayer 10 s/ }))
    const stop = await screen.findByRole('button', { name: /Arrêter l'essai/ })
    fireEvent.click(stop)
    await waitFor(() => expect(stops).toBe(1))
    expect(await screen.findByRole('button', { name: /Essayer 10 s/ })).toBeInTheDocument()
  })

  it('refus du serveur : la raison est affichée (éclairage désactivé, question en cours…), rien ne tourne', async () => {
    install()
    await mount()
    tryAnswer = () => json(409, { result: 'refused', reason: 'game' })
    fireEvent.click(screen.getByRole('button', { name: /Essayer 10 s/ }))
    expect(await screen.findByText(/question est en cours/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Arrêter l'essai/ })).toBeNull()
    tryAnswer = () => json(409, { result: 'refused', reason: 'disabled' })
    fireEvent.click(screen.getByRole('button', { name: /Essayer 10 s/ }))
    expect(await screen.findByText(/désactivé/i)).toBeInTheDocument()
  })

  it('brouillon en erreur : bouton grisé avec la raison « Corrigez les erreurs de l\'effet »', async () => {
    install({ draftOk: false })
    render(<LightEffectEditor id="mon-effet" zoneGroups={ZONES} onClose={vi.fn()} />)
    await screen.findByTestId('effect-editor')
    await waitFor(() => expect(screen.getByRole('button', { name: /Essayer/ })).toBeDisabled())
    expect(screen.getByText(/Corrigez les erreurs de l'effet/)).toBeInTheDocument()
  })

  it('effet qui demande plus de groupes que la zone : bouton grisé « Cet effet demande 5 groupes ; la zone Animateur en a 3 »', async () => {
    install()
    const big = { ...SOURCE, effect: { ...DOC, min_groups: 5 }, grid: { editable: true, effect: { ...DOC, min_groups: 5 }, converted: [], blocked: [] } }
    global.fetch = vi.fn(async (url, opts = {}) => (/\/source$/.test(String(url)) ? json(200, big) : (/\/draft$/.test(String(url)) ? json(200, { ok: true, errors: [], effect: big.effect, yaml: '', has_comments: false, custom_order: false, grid: big.grid, moment: 'chrono', previews: JSON.parse(opts.body).groups.map(preview) }) : json(404, {}))))
    await mount()
    fireEvent.click(screen.getByText('Animateur · 3 groupes'))
    expect(screen.getByRole('button', { name: /Essayer/ })).toBeDisabled()
    expect(screen.getByText(/demande 5 groupes ; la zone Animateur en a 3/i)).toBeInTheDocument()
  })

  it('la dernière zone choisie est retenue par navigateur', async () => {
    install(); localStorage.clear()
    const first = await mount()
    fireEvent.click(screen.getByText('Animateur · 3 groupes'))
    first.unmount()
    install()
    await mount()
    await waitFor(() => expect(JSON.stringify(Object.entries(localStorage))).toMatch(/animateur/))
  })

  it('un effet livré (lecture seule) : simulateur et essai restent disponibles', async () => {
    install()
    global.fetch = vi.fn(async (url, opts = {}) => (/\/source$/.test(String(url)) ? json(200, { ...SOURCE, origin: 'native', read_only: true }) : (/\/draft$/.test(String(url)) ? json(200, { ok: true, errors: [], effect: DOC, yaml: '', has_comments: false, custom_order: false, grid: SOURCE.grid, moment: 'chrono', previews: JSON.parse(opts.body).groups.map(preview) }) : json(200, { ok: true, duration_ms: 10000 }))))
    await mount()
    expect(screen.getByRole('button', { name: /Essayer 10 s/ })).toBeEnabled()
    expect(screen.getByTitle('Tick suivant')).toBeEnabled()
  })
})
