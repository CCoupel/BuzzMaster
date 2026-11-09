import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — composant mutualisé AmbianceSettingsPanel, MODE OVERRIDE + parité global/override.
// Contrat : contracts/ambiance-settings.md §6 (tri-états « Hérité / … », `base` = valeurs globales en italique sous
// « Hérité », `onChange(path, value | null)` — null = retour à « Hérité » —, point « • » sur l'onglet surchargé,
// `renderCatalogue` global seulement, `sections`) ; test de PARITÉ exigé (« liste des chemins éditables = liste des
// clés de Settings §2 ») ; maquette ambiance_settings_panel__round_override.html. Écrit depuis le contrat/la maquette.
// ---------------------------------------------------------------------------

vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const C = (cue, type, label, sub = '') => ({ cue, type, label, sub })
const CUES = [
  C('depart', '', 'Départ'), C('temps-ecoule', '', 'Temps écoulé'), C('gagne', '', 'Points gagnés'), C('perdu', '', 'Erreur'),
  C('reveal', '', 'Révélation'), C('entracte-debut', '', "Début d'entracte"), C('entracte-fin', '', "Fin d'entracte"),
  C('qcm-indice', 'QCM', 'Indice révélé'),
  C('memory-carte', 'MEMORY', 'Carte retournée'), C('memory-paire', 'MEMORY', 'Paire trouvée'),
  C('memotion-selection', 'MEMOTION', 'Carte choisie'), C('memotion-fin-carte', 'MEMOTION', 'Fin de carte'),
  C('rafale-bonne', 'RAFALE', 'Bonne réponse'), C('rafale-mauvaise', 'RAFALE', 'Mauvaise réponse'), C('rafale-fin-lot', 'RAFALE', 'Fin de lot'),
]
const ALL_CUE_PATHS = CUES.map((c) => `sound.cues.${c.cue}`)
// Settings (§2) : les 4 champs hors cues + une entrée par cue du registre.
const SETTINGS_PATHS = ['sound.muted', 'sound.countdown.enabled', 'sound.countdown.loop', 'light.mode', ...ALL_CUE_PATHS].sort()

const BASE = {
  sound: { muted: false, countdown: { enabled: true, loop: true }, cues: Object.fromEntries(CUES.map((c) => [c.cue, c.cue !== 'reveal'])) },
  light: { mode: 'AUTO' },
}

const setupOverride = (value = {}, props = {}) => {
  const onChange = vi.fn()
  const utils = render(<AmbianceSettingsPanel mode="override" value={value} base={BASE} cues={CUES} onChange={onChange} {...props} />)
  return { onChange, ...utils }
}
const group = (name) => screen.getByRole('radiogroup', { name })
const tab = (name) => screen.getByRole('tab', { name: new RegExp(`^${name}`) })
const radio = (grp, name) => within(grp).getByRole('radio', { name })

describe('AmbianceSettingsPanel (mode override) — tri-états « Hérité » (#290)', () => {
  it('value vide ⇒ tout est « Hérité » (sélectionné) : Son, Décompte (2 lignes), Lumière', () => {
    setupOverride({})
    for (const name of [/^Son$/i, /décompte actif/i, /lecture du décompte/i, /mode d'éclairage/i]) {
      expect(radio(group(name), 'Hérité')).toHaveAttribute('aria-checked', 'true')
    }
  })

  it("choisir une valeur émet onChange(chemin, valeur) ; revenir à « Hérité » émet onChange(chemin, null)", () => {
    const { onChange } = setupOverride({ sound: { muted: true } })
    expect(radio(group(/^Son$/i), 'Coupé')).toHaveAttribute('aria-checked', 'true')
    expect(radio(group(/^Son$/i), 'Hérité')).toHaveAttribute('aria-checked', 'false')
    fireEvent.click(radio(group(/^Son$/i), 'Hérité'))
    expect(onChange).toHaveBeenCalledWith('sound.muted', null)
    fireEvent.click(radio(group(/^Son$/i), 'ON'))
    expect(onChange).toHaveBeenCalledWith('sound.muted', false)
  })

  it('Décompte : Inactif / Une fois émettent leurs chemins ; l\'état chargé est reflété', () => {
    const { onChange } = setupOverride({ sound: { countdown: { enabled: false } } })
    expect(radio(group(/décompte actif/i), 'Inactif')).toHaveAttribute('aria-checked', 'true')
    expect(radio(group(/lecture du décompte/i), 'Hérité')).toHaveAttribute('aria-checked', 'true') // loop non surchargé
    fireEvent.click(radio(group(/lecture du décompte/i), 'Une fois'))
    expect(onChange).toHaveBeenCalledWith('sound.countdown.loop', false)
    fireEvent.click(radio(group(/décompte actif/i), 'Hérité'))
    expect(onChange).toHaveBeenCalledWith('sound.countdown.enabled', null)
  })

  it("Lumière : Hérité / ON / AUTO / OFF ; ON émet ('light.mode', 'ON')", () => {
    const { onChange } = setupOverride({})
    const g = group(/mode d'éclairage/i)
    for (const n of ['Hérité', 'ON', 'AUTO', 'OFF']) expect(radio(g, n)).toBeInTheDocument()
    fireEvent.click(radio(g, 'ON'))
    expect(onChange).toHaveBeenCalledWith('light.mode', 'ON')
  })

  it('une cue : Hérité / Actif / Inactif ; « Inactif » émet (sound.cues.<cue>, false) — nouvelles cues comprises', () => {
    const { onChange } = setupOverride({})
    fireEvent.click(radio(group('Départ'), 'Inactif'))
    expect(onChange).toHaveBeenCalledWith('sound.cues.depart', false)
    fireEvent.click(tab('QCM'))
    fireEvent.click(radio(group('Indice révélé'), 'Inactif'))
    expect(onChange).toHaveBeenCalledWith('sound.cues.qcm-indice', false)
    fireEvent.click(tab('RAFALE'))
    fireEvent.click(radio(group('Fin de lot'), 'Actif'))
    expect(onChange).toHaveBeenCalledWith('sound.cues.rafale-fin-lot', true)
  })
})

describe('AmbianceSettingsPanel (mode override) — valeurs globales héritées (#290)', () => {
  it('sous « Hérité », la valeur GLOBALE courante est affichée (Son, Décompte, Lumière)', () => {
    setupOverride({})
    expect(screen.getByText(/global\s*:\s*ON/i)).toBeInTheDocument()           // son non coupé
    expect(screen.getByText(/global\s*:\s*actif,\s*en boucle/i)).toBeInTheDocument()
    expect(screen.getByText(/global\s*:\s*AUTO/i)).toBeInTheDocument()
  })

  it('colonne « Valeur globale » : chaque cue montre sa valeur globale (Révélation inactive au global)', () => {
    setupOverride({})
    expect(screen.getByText('Valeur globale')).toBeInTheDocument()
    const row = screen.getByText('Révélation').closest('tr')
    expect(row).toHaveTextContent(/inactif/i)
    expect(screen.getByText('Départ').closest('tr')).toHaveTextContent(/actif/i)
  })

  it('le mode override n\'affiche JAMAIS la colonne catalogue (fichier / Remplacer / Tester) même si renderCatalogue est fourni', () => {
    const renderCatalogue = vi.fn(() => <td>fichier</td>)
    setupOverride({}, { renderCatalogue, catalogueColumns: [<th key="f">Fichier</th>] })
    expect(renderCatalogue).not.toHaveBeenCalled()
    expect(screen.queryByText('Fichier')).not.toBeInTheDocument()
  })
})

describe('AmbianceSettingsPanel (mode override) — onglets et point « surchargé » (#290)', () => {
  it('un onglet dont au moins une cue est surchargée porte un point « • » ; les autres non', () => {
    setupOverride({ sound: { cues: { 'qcm-indice': false } } })
    expect(within(tab('QCM')).getByLabelText(/surchargé/i)).toBeInTheDocument()
    expect(within(tab('Global')).queryByLabelText(/surchargé/i)).toBeNull()
    expect(within(tab('RAFALE')).queryByLabelText(/surchargé/i)).toBeNull()
  })

  it('surcharger une cue Global ne marque que l\'onglet Global', () => {
    setupOverride({ sound: { cues: { depart: false } } })
    expect(within(tab('Global')).getByLabelText(/surchargé/i)).toBeInTheDocument()
    expect(within(tab('QCM')).queryByLabelText(/surchargé/i)).toBeNull()
  })

  it('les cinq onglets et leurs compteurs sont identiques au mode global (7 / 1 / 2 / 2 / 3)', () => {
    setupOverride({})
    expect(tab('Global')).toHaveTextContent('7')
    expect(tab('QCM')).toHaveTextContent('1')
    expect(tab('MEMORY')).toHaveTextContent('2')
    expect(tab('MEMOTION')).toHaveTextContent('2')
    expect(tab('RAFALE')).toHaveTextContent('3')
  })
})

describe('AmbianceSettingsPanel — sections et état (#290)', () => {
  it("sections=['light'] en override : seule la ligne Lumière", () => {
    setupOverride({}, { sections: ['light'] })
    expect(group(/mode d'éclairage/i)).toBeInTheDocument()
    expect(screen.queryByRole('radiogroup', { name: /^Son$/i })).toBeNull()
    expect(screen.queryByText('Départ')).toBeNull()
  })

  it('disabled fige les contrôles en override', () => {
    setupOverride({}, { disabled: true })
    expect(radio(group(/^Son$/i), 'Coupé')).toBeDisabled()
  })
})

// ---------------------------------------------------------------------------
// PARITÉ (contrat §6) : les chemins éditables du composant = les clés de Settings (§2), dans les DEUX modes.
// Un champ ajouté d'un côté sans l'autre fait échouer ce test.
// ---------------------------------------------------------------------------

describe('AmbianceSettingsPanel — parité des champs global / override (#290)', () => {
  const TABS = ['Global', 'QCM', 'MEMORY', 'MEMOTION', 'RAFALE']

  it('mode OVERRIDE : les chemins émis couvrent EXACTEMENT les clés de Settings', () => {
    const { onChange } = setupOverride({})
    fireEvent.click(radio(group(/^Son$/i), 'Coupé'))
    fireEvent.click(radio(group(/décompte actif/i), 'Inactif'))
    fireEvent.click(radio(group(/lecture du décompte/i), 'Une fois'))
    fireEvent.click(radio(group(/mode d'éclairage/i), 'OFF'))
    for (const t of TABS) {
      fireEvent.click(tab(t))
      CUES.filter((c) => (c.type || 'Global').toUpperCase() === t.toUpperCase() || (t === 'Global' && !c.type)).forEach((c) => {
        fireEvent.click(radio(group(c.label), 'Inactif'))
      })
    }
    const paths = [...new Set(onChange.mock.calls.map((c) => c[0]))].sort()
    expect(paths).toEqual(SETTINGS_PATHS)
  })

  it('mode GLOBAL : les chemins émis couvrent EXACTEMENT les mêmes clés', () => {
    const onChange = vi.fn()
    render(<AmbianceSettingsPanel mode="global" value={{ sound: { muted: false, countdown: { enabled: true, loop: true }, cues: {} }, light: { mode: 'AUTO' } }} cues={CUES} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Coupé' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Inactif' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Une fois' }))
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    for (const t of TABS) {
      fireEvent.click(tab(t))
      CUES.filter((c) => (t === 'Global' ? !c.type : c.type === t)).forEach((c) => {
        fireEvent.click(screen.getByRole('checkbox', { name: new RegExp(c.label) }))
      })
    }
    const paths = [...new Set(onChange.mock.calls.map((c) => c[0]))].sort()
    expect(paths).toEqual(SETTINGS_PATHS)
  })
})
