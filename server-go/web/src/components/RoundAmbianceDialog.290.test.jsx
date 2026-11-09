import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — fenêtre « Ambiance de la manche » : héberge AmbianceSettingsPanel en mode override.
// Maquette ambiance_settings_panel__round_override.html : titre « Ambiance de la manche « X » », badge « Hérite tout
// du réglage global » / « N réglages surchargés » (cues comptées une à une), « Tout remettre à « Hérité » »,
// Annuler / Enregistrer. Rien n'est écrit dans le global : l'enregistrement renvoie le Partial à l'appelant
// (transporté ensuite par ROUNDS_UPDATE). Props supposées (documentées par le composant) : roundName, value, base,
// cues, onSave(partial), onCancel(). Écrit depuis la maquette/le contrat.
// ---------------------------------------------------------------------------

vi.mock('./RoundGroup.css', () => ({}))
vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import RoundAmbianceDialog from './RoundAmbianceDialog'

const C = (cue, type, label) => ({ cue, type, label })
const CUES = [
  C('depart', '', 'Départ'), C('temps-ecoule', '', 'Temps écoulé'), C('gagne', '', 'Points gagnés'), C('perdu', '', 'Erreur'),
  C('reveal', '', 'Révélation'), C('entracte-debut', '', "Début d'entracte"), C('entracte-fin', '', "Fin d'entracte"),
  C('qcm-indice', 'QCM', 'Indice révélé'),
  C('memory-carte', 'MEMORY', 'Carte retournée'), C('memory-paire', 'MEMORY', 'Paire trouvée'),
  C('memotion-selection', 'MEMOTION', 'Carte choisie'), C('memotion-fin-carte', 'MEMOTION', 'Fin de carte'),
  C('rafale-bonne', 'RAFALE', 'Bonne réponse'), C('rafale-mauvaise', 'RAFALE', 'Mauvaise réponse'), C('rafale-fin-lot', 'RAFALE', 'Fin de lot'),
]
const BASE = {
  sound: { muted: false, countdown: { enabled: true, loop: true }, cues: Object.fromEntries(CUES.map((c) => [c.cue, true])) },
  light: { mode: 'AUTO' },
}

const setup = (value = {}) => {
  const onSave = vi.fn()
  const onCancel = vi.fn()
  render(<RoundAmbianceDialog roundName="Blind-test" value={value} base={BASE} cues={CUES} onSave={onSave} onCancel={onCancel} />)
  return { onSave, onCancel }
}
const radioIn = (groupName, name) => within(screen.getByRole('radiogroup', { name: groupName })).getByRole('radio', { name })

describe('RoundAmbianceDialog (#290)', () => {
  it('titre « Ambiance de la manche « Blind-test » » et message « Hérite tout du réglage global » quand rien n\'est surchargé', () => {
    setup({})
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText(/Ambiance de la manche « Blind-test »/)).toBeInTheDocument()
    expect(screen.getByText(/Hérite tout du réglage global/)).toBeInTheDocument()
  })

  it('le compteur du badge = nombre de champs définis, cues comptées une à une (maquette : 3 réglages surchargés)', () => {
    setup({ sound: { countdown: { enabled: false }, cues: { 'qcm-indice': false } }, light: { mode: 'ON' } })
    expect(screen.getByText(/3 réglages surchargés/)).toBeInTheDocument()
  })

  it('un seul réglage : accord au singulier ; deux cues = deux réglages', () => {
    setup({ sound: { muted: true } })
    expect(screen.getByText(/1 réglage surchargé/)).toBeInTheDocument()
  })

  it('modifier un réglage met à jour le compteur ; revenir à « Hérité » le retire', () => {
    setup({})
    fireEvent.click(radioIn(/^Son$/i, 'Coupé'))
    expect(screen.getByText(/1 réglage surchargé/)).toBeInTheDocument()
    fireEvent.click(radioIn(/^Son$/i, 'Hérité'))
    expect(screen.getByText(/Hérite tout du réglage global/)).toBeInTheDocument()
  })

  it('Enregistrer renvoie le Partial (forme du contrat §3.2), uniquement les champs surchargés', () => {
    const { onSave } = setup({})
    fireEvent.click(radioIn(/décompte actif/i, 'Inactif'))
    fireEvent.click(radioIn(/mode d'éclairage/i, 'ON'))
    fireEvent.click(screen.getByRole('tab', { name: /^QCM/ }))
    fireEvent.click(radioIn('Indice révélé', 'Inactif'))
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave).toHaveBeenCalledTimes(1)
    expect(onSave).toHaveBeenCalledWith({
      sound: { countdown: { enabled: false }, cues: { 'qcm-indice': false } },
      light: { mode: 'ON' },
    })
  })

  it('Enregistrer sans rien changer renvoie la valeur d\'origine (aucun champ ajouté, aucun retiré)', () => {
    const initial = { sound: { muted: true } }
    const { onSave } = setup(initial)
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave).toHaveBeenCalledWith(initial)
  })

  it('« Tout remettre à « Hérité » » vide la surcharge (Partial {}), désactivé quand il n\'y a rien à remettre', () => {
    const { onSave } = setup({ sound: { muted: true }, light: { mode: 'OFF' } })
    const reset = screen.getByRole('button', { name: /Tout remettre à/ })
    expect(reset).toBeEnabled()
    fireEvent.click(reset)
    expect(screen.getByText(/Hérite tout du réglage global/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tout remettre à/ })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(onSave).toHaveBeenCalledWith({})
  })

  it('Annuler ferme sans rien enregistrer (même après modification)', () => {
    const { onSave, onCancel } = setup({})
    fireEvent.click(radioIn(/^Son$/i, 'Coupé'))
    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onSave).not.toHaveBeenCalled()
  })

  it('Échap ferme la fenêtre (accessibilité)', () => {
    const { onCancel } = setup({})
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onCancel).toHaveBeenCalled()
  })

  it('aucune colonne catalogue : une manche active ou coupe un son, elle n\'en change pas le fichier (D7)', () => {
    setup({})
    expect(screen.queryByText(/Remplacer/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Tester/ })).not.toBeInTheDocument()
    expect(screen.getByText('Valeur globale')).toBeInTheDocument()
  })
})
