import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4 Batch 2, révision 3) — colonne « Fichier » de la fenêtre « Ambiance de la manche » :
// trois états (hérité défaut / personnalisé, fichier de manche, fichier manquant), effet IMMÉDIAT des fichiers
// (Remplacer… / Revenir au fichier global / Tester / Écouter ici), indépendant d'Enregistrer / Annuler, compteur « N réglages
// et M fichiers », point « • » d'onglet, confirmation D11 de « Tout remettre à Hérité ». Contrat ambiance-settings.md §9.3,
// §9.5, §9.6 ; maquette ambiance_settings_panel__round_override_files.html. Prop `roundId` (documentée par le composant).
// ---------------------------------------------------------------------------

vi.mock('./RoundGroup.css', () => ({}))
vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import RoundAmbianceDialog from './RoundAmbianceDialog'

const C = (cue, type, label) => ({ cue, type, label })
const CUES = [
  C('depart', '', 'Départ'), C('temps-ecoule', '', 'Temps écoulé'), C('gagne', '', 'Points gagnés'), C('perdu', '', 'Erreur'),
  C('reveal', '', 'Révélation'), C('entracte-debut', '', "Début d'entracte"), C('entracte-fin', '', "Fin d'entracte"),
  C('qcm-indice', 'QCM', 'Indice révélé'),
]
const BASE = {
  sound: { muted: false, countdown: { enabled: true, loop: true }, cues: Object.fromEntries(CUES.map((c) => [c.cue, true])) },
  light: { mode: 'AUTO' },
}
const SOUNDS = {
  cues: CUES.map((c) => ({ cue: c.cue, type: c.type, enabled: true, custom: c.cue === 'temps-ecoule', duration_seconds: c.cue === 'depart' ? 0.45 : c.cue === 'temps-ecoule' ? 1.2 : 0.5, path: `/files/sounds/${c.cue}.wav` })),
  long: [{ name: 'decompte', kind: 'countdown', enabled: true, loop: true, custom: false, duration_seconds: 2.0, path: '/files/sounds/decompte.wav' }],
}
const FILE = (cue, extra = {}) => ({ cue, path: `rounds/r2/${cue}.wav`, present: true, duration_seconds: cue === 'decompte' ? 3.1 : 0.8, ...extra })

let roundFiles
let calls
const ok = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })

beforeEach(() => {
  roundFiles = []
  calls = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    calls.push({ method, url, body: opts.body })
    if (url === '/api/rounds/r2/sounds' && method === 'GET') return ok({ round_id: 'r2', files: roundFiles })
    if (url === '/api/sounds') return ok(SOUNDS)
    if (url === '/api/ambiance/settings') return ok({ global: BASE })
    const m = /^\/api\/rounds\/r2\/sounds\/([^/]+)(\/test)?$/.exec(url)
    if (m && method === 'POST' && m[2]) return ok({ status: 'ok', played: roundFiles.some((f) => f.cue === m[1]) ? 'round' : 'global' })
    if (m && method === 'POST') {
      roundFiles = [...roundFiles.filter((f) => f.cue !== m[1]), FILE(m[1])]
      return ok({ status: 'ok', round_id: 'r2', cue: m[1], path: `rounds/r2/${m[1]}.wav`, duration_seconds: 0.8, converted: false, source_format: null, warning: null })
    }
    if (m && method === 'DELETE') {
      roundFiles = roundFiles.filter((f) => f.cue !== m[1])
      return ok({ status: 'ok' })
    }
    return ok({})
  })
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

const setup = async (props = {}) => {
  const onSave = vi.fn()
  const onCancel = vi.fn()
  render(<RoundAmbianceDialog roundId="r2" roundName="Blind-test" value={{}} base={BASE} cues={CUES} onSave={onSave} onCancel={onCancel} {...props} />)
  await screen.findByText(/Ambiance de la manche « Blind-test »/)
  await waitFor(() => expect(calls.some((c) => c.url === '/api/rounds/r2/sounds')).toBe(true))
  await screen.findByRole('dialog')
  return { onSave, onCancel }
}
const row = (label) => screen.getByText(label).closest('tr')
const btn = (scope, name) => within(scope).queryByRole('button', { name })
const countdownScope = () => screen.getByText(/Fichier du décompte/i).closest('.asp-row') || screen.getByText(/Fichier du décompte/i).parentElement

describe('Colonne « Fichier » — les trois états (#290 r3, §9.6)', () => {
  it('en-tête « Fichier » présent quand la manche est connue (roundId)', async () => {
    await setup()
    expect(screen.getByText('Fichier')).toBeInTheDocument()
  })

  it('hérité (défaut) : nom global, « défaut », durée, Remplacer… et Tester ; ni Revenir, ni Écouter ici, ni badge « manche » (métadonnées lues par GET /api/sounds)', async () => {
    await setup({ cues: null })
    await waitFor(() => expect(row('Départ')).toHaveTextContent('0,45 s')) // métadonnées globales chargées (GET /api/sounds)
    const r = row('Départ')
    expect(r).toHaveTextContent('depart.wav')
    expect(r).toHaveTextContent(/défaut/)
    expect(btn(r, /Remplacer/)).toBeInTheDocument()
    expect(btn(r, 'Tester')).toBeInTheDocument()
    expect(btn(r, /Revenir au fichier global/)).toBeNull()
    expect(btn(r, /Écouter ici/)).toBeNull()
    expect(within(r).queryByText('manche')).toBeNull()
  })

  it('hérité (son global personnalisé) : « personnalisé » et sa durée', async () => {
    await setup({ cues: null })
    await waitFor(() => expect(row('Temps écoulé')).toHaveTextContent('1,20 s'))
    expect(row('Temps écoulé')).toHaveTextContent(/personnalisé/)
  })

  it('fichier de manche : chemin, durée, badge « manche », Remplacer… / Revenir au fichier global / Tester / Écouter ici', async () => {
    roundFiles = [FILE('depart')]
    await setup()
    const r = await waitFor(() => { const x = row('Départ'); expect(x).toHaveTextContent('rounds/r2/depart.wav'); return x })
    expect(r).toHaveTextContent('0,80 s')
    expect(within(r).getByText('manche')).toBeInTheDocument()
    for (const n of [/Remplacer/, /Revenir au fichier global/, 'Tester', /Écouter ici/]) expect(btn(r, n)).toBeInTheDocument()
  })

  it('fichier de manche manquant : « fichier introuvable, le son global est joué », Remplacer… et Revenir, pas de Tester ni d\'Écouter ici', async () => {
    roundFiles = [FILE('depart', { present: false, duration_seconds: null })]
    await setup()
    const r = await waitFor(() => { const x = row('Départ'); expect(x).toHaveTextContent(/fichier introuvable/); return x })
    expect(r).toHaveTextContent(/le son global est joué/)
    expect(btn(r, /Remplacer/)).toBeInTheDocument()
    expect(btn(r, /Revenir au fichier global/)).toBeInTheDocument()
    expect(btn(r, 'Tester')).toBeNull()
    expect(btn(r, /Écouter ici/)).toBeNull()
  })

  it('le décompte a sa ligne Fichier (hérité puis fichier de manche)', async () => {
    roundFiles = [FILE('decompte')]
    await setup()
    await waitFor(() => expect(countdownScope()).toHaveTextContent('rounds/r2/decompte.wav'))
    expect(countdownScope()).toHaveTextContent('3,10 s')
    expect(btn(countdownScope(), /Revenir au fichier global/)).toBeInTheDocument()
  })

  it('sans roundId : aucune colonne Fichier (fenêtre v11.6 LOT-4 inchangée)', async () => {
    render(<RoundAmbianceDialog roundName="Blind-test" value={{}} base={BASE} cues={CUES} onSave={vi.fn()} onCancel={vi.fn()} />)
    await screen.findByRole('dialog')
    expect(screen.queryByText('Fichier')).toBeNull()
    expect(screen.queryByRole('button', { name: /Remplacer/ })).toBeNull()
  })
})

describe('Effet immédiat des fichiers (D10, §9.3)', () => {
  it('Remplacer… : POST multipart /api/rounds/r2/sounds/depart avec le champ `file`, sans toucher à Enregistrer', async () => {
    const { onSave } = await setup()
    const r = row('Départ')
    const file = new File([new Uint8Array(8)], 'mon.wav', { type: 'audio/wav' })
    fireEvent.change(r.querySelector('input[type="file"]'), { target: { files: [file] } })
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.url === '/api/rounds/r2/sounds/depart')).toBe(true))
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/rounds/r2/sounds/depart')
    expect(post.body).toBeInstanceOf(FormData)
    expect(post.body.get('file')).toBe(file)
    expect(onSave).not.toHaveBeenCalled()
    await waitFor(() => expect(row('Départ')).toHaveTextContent('rounds/r2/depart.wav'))
  })

  it('Revenir au fichier global : DELETE immédiat, la ligne redevient héritée', async () => {
    roundFiles = [FILE('depart')]
    const { onSave } = await setup()
    await waitFor(() => expect(row('Départ')).toHaveTextContent('rounds/r2/depart.wav'))
    fireEvent.click(btn(row('Départ'), /Revenir au fichier global/))
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/rounds/r2/sounds/depart')).toBe(true))
    await waitFor(() => expect(row('Départ')).toHaveTextContent('depart.wav'))
    await waitFor(() => expect(within(row('Départ')).queryByText('manche')).toBeNull())
    expect(onSave).not.toHaveBeenCalled()
  })

  it('Tester : POST /api/rounds/r2/sounds/depart/test (fichier effectif) et retour utilisateur', async () => {
    roundFiles = [FILE('depart')]
    await setup()
    await waitFor(() => expect(row('Départ')).toHaveTextContent('rounds/r2/depart.wav'))
    fireEvent.click(btn(row('Départ'), 'Tester'))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.url === '/api/rounds/r2/sounds/depart/test')).toBe(true))
    expect(await screen.findByText(/joué sur l.enceinte/i)).toBeInTheDocument()
  })

  it('Écouter ici : lecture locale de /files/sounds/rounds/r2/depart.wav?v=<jeton> (aucun appel serveur de test)', async () => {
    roundFiles = [FILE('depart')]
    const urls = []
    vi.stubGlobal('Audio', class { constructor(u) { urls.push(u) } play() { return Promise.resolve() } })
    await setup()
    await waitFor(() => expect(row('Départ')).toHaveTextContent('rounds/r2/depart.wav'))
    fireEvent.click(btn(row('Départ'), /Écouter ici/))
    expect(urls).toHaveLength(1)
    expect(urls[0]).toMatch(/^\/files\/sounds\/rounds\/r2\/depart\.wav\?v=\d+$/)
    expect(calls.filter((c) => /\/test$/.test(c.url))).toHaveLength(0)
  })

  it('un envoi refusé (400) affiche la cause et laisse la ligne héritée', async () => {
    global.fetch = vi.fn(async (url, opts = {}) => {
      calls.push({ method: opts.method || 'GET', url, body: opts.body })
      if (opts.method === 'POST' && /\/sounds\/depart$/.test(url)) return ok({ status: 'error', message: 'ce son dure 6,0 s' }, 400)
      if (url === '/api/rounds/r2/sounds') return ok({ round_id: 'r2', files: [] })
      if (url === '/api/sounds') return ok(SOUNDS)
      return ok({})
    })
    await setup()
    fireEvent.change(row('Départ').querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'long.wav')] } })
    expect(await screen.findByText(/ce son dure 6,0 s/)).toBeInTheDocument()
    expect(row('Départ')).not.toHaveTextContent('rounds/r2/depart.wav')
  })
})

describe('Indépendance Enregistrer / Annuler et fichiers (D10)', () => {
  it('Enregistrer ne renvoie JAMAIS `sound.files` (ROUNDS_UPDATE l\'ignore) : seulement les tri-états et le mode', async () => {
    roundFiles = [FILE('depart')]
    const { onSave } = await setup({ value: { sound: { muted: true, files: { depart: 'rounds/r2/depart.wav' } } } })
    await waitFor(() => expect(row('Départ')).toHaveTextContent('rounds/r2/depart.wav'))
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave).toHaveBeenCalledTimes(1)
    const sent = onSave.mock.calls[0][0]
    expect(sent.sound.muted).toBe(true)
    expect('files' in (sent.sound || {})).toBe(false)
  })

  it('Annuler après un fichier remplacé : rien n\'est annulé côté fichier (déjà appliqué), aucune requête de suppression', async () => {
    const { onCancel } = await setup()
    fireEvent.change(row('Départ').querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'a.wav')] } })
    await waitFor(() => expect(roundFiles).toHaveLength(1))
    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(onCancel).toHaveBeenCalled()
    expect(calls.filter((c) => c.method === 'DELETE')).toHaveLength(0)
    expect(roundFiles).toHaveLength(1)
  })

  it('la fenêtre indique l\'effet immédiat des fichiers', async () => {
    await setup()
    expect(screen.getByRole('dialog')).toHaveTextContent(/immédiatement/i)
  })
})

describe('Compteur, onglet « • » et confirmation « Tout remettre à Hérité » (D11)', () => {
  it('compteur : « 2 réglages et 2 fichiers surchargés » (maquette)', async () => {
    roundFiles = [FILE('depart'), FILE('decompte')]
    await setup({ value: { sound: { cues: { reveal: false } }, light: { mode: 'ON' } } })
    expect(await screen.findByText(/2 réglages et 2 fichiers surchargés/)).toBeInTheDocument()
  })

  it('compteur sans fichier : formulation v11.6 inchangée ; rien du tout : « Hérite tout »', async () => {
    await setup({ value: { light: { mode: 'ON' } } })
    expect(screen.getByText(/1 réglage surchargé/)).toBeInTheDocument()
    expect(screen.queryByText(/\d+ fichiers?/)).toBeNull()
  })

  it('un onglet dont SEUL un fichier est surchargé porte le point « • »', async () => {
    roundFiles = [FILE('qcm-indice')]
    await setup()
    const tab = screen.getByRole('tab', { name: /^QCM/ })
    await waitFor(() => expect(within(tab).getByLabelText(/surchargé/i)).toBeInTheDocument())
    expect(within(screen.getByRole('tab', { name: /^Global/ })).queryByLabelText(/surchargé/i)).toBeNull()
  })

  it('confirmation nommant le nombre de fichiers ; refus ⇒ rien n\'est supprimé ni remis à Hérité', async () => {
    roundFiles = [FILE('depart'), FILE('gagne')]
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { onSave } = await setup({ value: { sound: { muted: true } } })
    await waitFor(() => expect(screen.getByText(/2 fichiers/)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Tout remettre à/ }))
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm.mock.calls[0][0]).toMatch(/2 fichiers/)
    expect(calls.filter((c) => c.method === 'DELETE')).toHaveLength(0)
    expect(roundFiles).toHaveLength(2)
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave.mock.calls[0][0].sound.muted).toBe(true) // le brouillon n'a pas été vidé
  })

  it('confirmation acceptée ⇒ DELETE de CHAQUE fichier de la manche puis remise à Hérité des réglages', async () => {
    roundFiles = [FILE('depart'), FILE('gagne')]
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { onSave } = await setup({ value: { sound: { muted: true } } })
    await waitFor(() => expect(screen.getByText(/2 fichiers/)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Tout remettre à/ }))
    await waitFor(() => expect(calls.filter((c) => c.method === 'DELETE').map((c) => c.url).sort()).toEqual(['/api/rounds/r2/sounds/depart', '/api/rounds/r2/sounds/gagne']))
    await waitFor(() => expect(screen.getByText(/Hérite tout du réglage global/)).toBeInTheDocument())
    expect(roundFiles).toHaveLength(0)
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave).toHaveBeenCalledWith({})
  })

  it('un DELETE en échec ⇒ erreur nommant le son non supprimé, liste rechargée, réglages PAS remis à Hérité', async () => {
    roundFiles = [FILE('depart'), FILE('gagne')]
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const base = global.fetch
    global.fetch = vi.fn(async (url, opts = {}) => {
      if ((opts.method || 'GET') === 'DELETE' && url === '/api/rounds/r2/sounds/gagne') {
        calls.push({ method: 'DELETE', url })
        return ok({ status: 'error', message: 'disque plein' }, 500)
      }
      return base(url, opts)
    })
    const { onSave } = await setup({ value: { sound: { muted: true } } })
    await waitFor(() => expect(screen.getByText(/2 fichiers/)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Tout remettre à/ }))
    const err = await screen.findByText(/non supprimé/)
    expect(err.textContent).toMatch(/Points gagnés/)
    expect(err.textContent).not.toMatch(/Départ/)
    // état réel rechargé : depart supprimé, gagne toujours là
    await waitFor(() => expect(screen.getByText(/1 fichier surchargé|1 réglage et 1 fichier surchargé/)).toBeInTheDocument())
    expect(calls.filter((c) => c.method === 'GET' && c.url === '/api/rounds/r2/sounds').length).toBeGreaterThanOrEqual(2)
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave.mock.calls[0][0].sound.muted).toBe(true) // brouillon conservé
  })

  it('aucun fichier de manche ⇒ aucune confirmation (comportement LOT-4 inchangé)', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    await setup({ value: { sound: { muted: true } } })
    fireEvent.click(screen.getByRole('button', { name: /Tout remettre à/ }))
    expect(confirm).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.getByText(/Hérite tout du réglage global/)).toBeInTheDocument())
  })
})
