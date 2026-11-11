import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #223 (v11.7.0, LOT-2) — sélecteurs « effet lumière par moment » du composant mutualisé
// AmbianceSettingsPanel (page Ambiance en mode global, éditeur de manche en mode override).
// Écrit depuis contracts/light-effects.md §6, §7.1, §2.5 et la maquette
// docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects.html.
// Champs émis : light.effects.<moment> et light.host_effects.<id du groupe>.<moment> (jamais lighting.*).
// Révision du GATE LOT-1 : un bloc de trois sélecteurs PAR groupe animateur (prop hostGroups [{id, name, rank, empty}]).
// ---------------------------------------------------------------------------

vi.mock('./AmbianceSettingsPanel.css', () => ({}))

import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const eff = (id, name, moments, extra = {}) => ({
  id, name, description: '', moments, origin: 'native', file: `natifs/${id}.yaml`, min_groups: 1, applicable: true, reason: '', ...extra,
})

const CATALOG = {
  groups: 4,
  effects: [
    eff('pulsation', 'Pulsation', ['chrono']),
    eff('chenillard', 'Chenillard', ['chrono'], { min_groups: 3 }),
    eff('couleur-equipe', 'Couleur de l\'équipe', ['score']),
    eff('mon-effet', 'Mon effet', ['chrono', 'calm'], { origin: 'user', file: 'mon-effet.yaml' }),
    eff('vague-lente', 'Vague lente', ['calm'], { min_groups: 2 }),
  ],
  errors: [],
  unresolved: [],
  dir: 'data/files/effects',
}

const room = (c) => document.getElementById(`asp-eff-room-${c}`)
const host = (id, c) => document.getElementById(`asp-eff-host-${id}-${c}`)
const HOSTS = [{ id: 'ha', name: 'Pupitre', rank: 1, empty: false }, { id: 'hb', name: 'Régie', rank: 2, empty: false }]
const optionTexts = (el) => Array.from(el.querySelectorAll('option')).map(o => o.textContent)

function setup(props = {}) {
  const onChange = vi.fn()
  const utils = render(
    <AmbianceSettingsPanel
      mode="global"
      sections={['lightEffects']}
      value={{}}
      catalog={CATALOG}
      hostGroups={[]}
      onChange={onChange}
      {...props}
    />
  )
  return { onChange, ...utils }
}

describe('AmbianceSettingsPanel › effets lumière — trois sélecteurs de salle', () => {
  it('trois moments : chronomètre, phases calmes, équipe qui marque', () => {
    setup()
    expect(screen.getByTestId('asp-light-effects')).toBeInTheDocument()
    for (const label of [/Pendant le chronomètre/, /Phases calmes/, /Quand une équipe marque/]) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    expect(room('chrono')).toBeInTheDocument()
    expect(room('calm')).toBeInTheDocument()
    expect(room('score')).toBeInTheDocument()
  })

  it('section masquée tant que le catalogue n\'est pas chargé (catalog indéfini)', () => {
    setup({ catalog: undefined })
    expect(screen.queryByTestId('asp-light-effects')).toBeNull()
  })

  it('défauts globaux = comportement v11.6.0 : pulsation (chrono), aucun effet (calm, score)', () => {
    setup()
    expect(room('chrono')).toHaveValue('pulsation')
    expect(room('calm')).toHaveValue('')
    expect(room('score')).toHaveValue('')
  })

  it('chaque liste ne propose que les effets qui déclarent le moment, « Aucun effet » en tête', () => {
    setup()
    expect(optionTexts(room('chrono')).join('|')).toMatch(/^Aucun effet\|.*Pulsation.*\|.*Chenillard.*\|.*Mon effet/)
    expect(optionTexts(room('chrono')).join('|')).not.toMatch(/Vague lente|Couleur de l'équipe/)
    expect(optionTexts(room('calm')).join('|')).toMatch(/Mon effet/)
    expect(optionTexts(room('calm')).join('|')).not.toMatch(/Pulsation|Chenillard/)
    expect(optionTexts(room('score')).join('|')).toMatch(/Couleur de l'équipe/)
  })

  it('natifs d\'abord, effets de l\'utilisateur ensuite, étiquetés « (personnalisé) »', () => {
    setup()
    const t = optionTexts(room('chrono'))
    expect(t.findIndex(x => /Mon effet/.test(x))).toBeGreaterThan(t.findIndex(x => /Chenillard/.test(x)))
    expect(t.find(x => /Mon effet/.test(x))).toMatch(/personnalisé/)
  })

  it('l\'option par défaut du moment est marquée « (actuel) » en mode global', () => {
    setup()
    expect(optionTexts(room('chrono')).some(x => /Pulsation.*\(actuel\)/.test(x))).toBe(true)
  })

  it('choisir un effet émet light.effects.<moment> avec son id', () => {
    const { onChange } = setup()
    fireEvent.change(room('chrono'), { target: { value: 'chenillard' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.chrono', 'chenillard')
  })

  it('« Aucun effet » émet la chaîne vide (scène fixe), pas null', () => {
    const { onChange } = setup()
    fireEvent.change(room('chrono'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.chrono', '')
  })

  it('une valeur enregistrée est affichée sélectionnée', () => {
    setup({ value: { light: { effects: { chrono: 'chenillard', calm: 'mon-effet', score: 'couleur-equipe' } } } })
    expect(room('chrono')).toHaveValue('chenillard')
    expect(room('calm')).toHaveValue('mon-effet')
    expect(room('score')).toHaveValue('couleur-equipe')
  })
})

describe('AmbianceSettingsPanel › effets lumière — effet inapplicable et introuvable', () => {
  it('un effet qui demande plus de groupes que configurés reste visible, grisé, avec sa raison', () => {
    setup({ catalog: { ...CATALOG, groups: 2, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, applicable: false, reason: '' } : e)) } })
    const opt = Array.from(room('chrono').querySelectorAll('option')).find(o => /Chenillard/.test(o.textContent))
    expect(opt).toBeDisabled()
    expect(opt.textContent).toMatch(/demande 3 groupes, 2 configurés/)
  })

  it('la raison fournie par le serveur est reprise telle quelle', () => {
    setup({ catalog: { ...CATALOG, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, applicable: false, reason: 'demande 3 groupes, 0 configuré' } : e)) } })
    const opt = Array.from(room('chrono').querySelectorAll('option')).find(o => /Chenillard/.test(o.textContent))
    expect(opt.textContent).toMatch(/demande 3 groupes, 0 configuré/)
  })

  it('un effet sélectionné devenu inapplicable avertit du repli (pulsation au chrono) et renvoie vers les groupes', () => {
    const onManageGroups = vi.fn()
    setup({
      value: { light: { effects: { chrono: 'chenillard' } } },
      catalog: { ...CATALOG, groups: 2, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, applicable: false } : e)) },
      onManageGroups,
    })
    const warn = within(screen.getByTestId('asp-light-effects')).getByRole('status')
    expect(warn).toHaveTextContent(/3 groupes/)
    expect(warn).toHaveTextContent(/pulsation est jouée à la place/)
    fireEvent.click(within(warn).getByRole('button', { name: /Gérer les groupes/ }))
    expect(onManageGroups).toHaveBeenCalled()
  })

  it('un effet sélectionné absent du catalogue est conservé « (introuvable) » avec un avertissement', () => {
    const onOpenCatalogue = vi.fn()
    setup({ value: { light: { effects: { calm: 'fichier-supprime' } } }, onOpenCatalogue })
    expect(room('calm')).toHaveValue('fichier-supprime')
    expect(optionTexts(room('calm')).some(x => /fichier-supprime \(introuvable\)/.test(x))).toBe(true)
    const warn = screen.getByRole('status')
    expect(warn).toHaveTextContent(/absent ou invalide/)
    fireEvent.click(within(warn).getByRole('button', { name: /Voir le catalogue/ }))
    expect(onOpenCatalogue).toHaveBeenCalled()
  })

  it('aucun avertissement quand l\'effet est applicable', () => {
    setup({ value: { light: { effects: { chrono: 'pulsation' } } } })
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('désactivé : les trois sélecteurs sont figés', () => {
    setup({ disabled: true })
    expect(room('chrono')).toBeDisabled()
    expect(room('calm')).toBeDisabled()
    expect(room('score')).toBeDisabled()
  })
})

describe('AmbianceSettingsPanel › effets lumière — un bloc par groupe animateur (§2.5, §6)', () => {
  it('sans groupe animateur : pas de bloc, invitation à en créer un (global)', () => {
    const onCreateHost = vi.fn()
    setup({ onCreateHost })
    expect(host('ha', 'chrono')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /en créer un/ }))
    expect(onCreateHost).toHaveBeenCalled()
  })

  it('un bloc de trois sélecteurs par groupe animateur, titré 🎤k · nom, « Blanc fixe » par défaut', () => {
    setup({ hostGroups: HOSTS })
    expect(within(screen.getByTestId('asp-host-ha')).getByText(/🎤1 · Pupitre/)).toBeInTheDocument()
    expect(within(screen.getByTestId('asp-host-hb')).getByText(/🎤2 · Régie/)).toBeInTheDocument()
    for (const id of ['ha', 'hb']) {
      for (const c of ['chrono', 'calm', 'score']) {
        expect(host(id, c)).toHaveValue('')
        expect(optionTexts(host(id, c))[0]).toMatch(/^Blanc fixe/)
      }
    }
  })

  it('un groupe vide est signalé dans son titre', () => {
    setup({ hostGroups: [{ id: 'ha', name: 'Pupitre', rank: 1, empty: true }] })
    expect(within(screen.getByTestId('asp-host-ha')).getByText(/groupe vide/)).toBeInTheDocument()
  })

  it('les effets à plusieurs groupes sont grisés pour chaque animateur, pas les effets à un groupe', () => {
    setup({ hostGroups: HOSTS })
    for (const id of ['ha', 'hb']) {
      const opts = (c) => Array.from(host(id, c).querySelectorAll('option'))
      const chenillard = opts('chrono').find(o => /Chenillard/.test(o.textContent))
      expect(chenillard).toBeDisabled()
      expect(chenillard.textContent).toMatch(/plusieurs groupes/)
      expect(opts('chrono').find(o => /Pulsation/.test(o.textContent))).not.toBeDisabled()
      expect(opts('score').find(o => /Couleur de l'équipe/.test(o.textContent))).not.toBeDisabled()
      expect(opts('calm').find(o => /Vague lente/.test(o.textContent))).toBeDisabled()
    }
  })

  it('choisir un effet émet light.host_effects.<id>.<moment> — chaque groupe a son propre chemin', () => {
    const { onChange } = setup({ hostGroups: HOSTS })
    fireEvent.change(host('hb', 'score'), { target: { value: 'couleur-equipe' } })
    expect(onChange).toHaveBeenCalledWith('light.host_effects.hb.score', 'couleur-equipe')
    fireEvent.change(host('ha', 'chrono'), { target: { value: 'pulsation' } })
    expect(onChange).toHaveBeenCalledWith('light.host_effects.ha.chrono', 'pulsation')
    expect(onChange).not.toHaveBeenCalledWith('light.effects.score', expect.anything())
  })

  it('les valeurs enregistrées sont affichées par groupe', () => {
    setup({ hostGroups: HOSTS, value: { light: { host_effects: { ha: { chrono: 'pulsation' }, hb: { score: 'couleur-equipe' } } } } })
    expect(host('ha', 'chrono')).toHaveValue('pulsation')
    expect(host('ha', 'score')).toHaveValue('')
    expect(host('hb', 'score')).toHaveValue('couleur-equipe')
    expect(host('hb', 'chrono')).toHaveValue('')
  })

  it('rappelle que les animateurs ignorent ON / OFF et le Flash', () => {
    setup({ hostGroups: HOSTS })
    expect(screen.getByText(/ni par le Flash/)).toBeInTheDocument()
  })

  it('un effet d\'animateur inapplicable avertit, sous le sélecteur du groupe concerné seulement', () => {
    setup({ hostGroups: HOSTS, value: { light: { host_effects: { ha: { chrono: 'chenillard' } } } } })
    expect(within(screen.getByTestId('asp-host-ha')).getByRole('status')).toHaveTextContent(/blanc fixe/)
    expect(within(screen.getByTestId('asp-host-hb')).queryByRole('status')).toBeNull()
  })

  it('le groupe ciblé par « Choisir ses effets » (focusHostId) est mis en évidence', () => {
    setup({ hostGroups: HOSTS, focusHostId: 'hb' })
    expect(screen.getByTestId('asp-host-hb').className).toMatch(/is-focus/)
    expect(screen.getByTestId('asp-host-ha').className).not.toMatch(/is-focus/)
  })
})

describe('AmbianceSettingsPanel › effets lumière — mode surcharge de manche (« Hérité »)', () => {
  const base = {
    light: {
      effects: { chrono: 'pulsation', calm: 'mon-effet', score: '' },
      host_effects: { ha: { chrono: '', calm: '', score: 'couleur-equipe' }, hb: { chrono: 'pulsation', calm: '', score: '' } },
    },
  }

  it('clé absente du Partial ⇒ « Hérité » sélectionné, valeur globale affichée entre parenthèses', () => {
    setup({ mode: 'override', value: {}, base, hostGroups: HOSTS })
    expect(room('chrono')).toHaveValue('__inherit__')
    expect(optionTexts(room('chrono'))[0]).toBe('Hérité (Pulsation)')
    expect(optionTexts(room('calm'))[0]).toBe('Hérité (Mon effet (personnalisé))')
    expect(optionTexts(room('score'))[0]).toBe('Hérité (Aucun effet)')
    expect(optionTexts(host('ha', 'chrono'))[0]).toBe('Hérité (Blanc fixe)')
    expect(optionTexts(host('ha', 'score'))[0]).toBe('Hérité (Couleur de l\'équipe)')
    expect(optionTexts(host('hb', 'chrono'))[0]).toBe('Hérité (Pulsation)')
  })

  it('« Hérité » retire la surcharge : onChange(field, null), par groupe', () => {
    const { onChange } = setup({ mode: 'override', value: { light: { effects: { chrono: 'chenillard' }, host_effects: { hb: { score: 'couleur-equipe' } } } }, base, hostGroups: HOSTS })
    fireEvent.change(room('chrono'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.chrono', null)
    fireEvent.change(host('hb', 'score'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.host_effects.hb.score', null)
  })

  it('surcharger un seul groupe animateur laisse l\'autre en « Hérité »', () => {
    setup({ mode: 'override', value: { light: { host_effects: { ha: { score: 'pulsation' } } } }, base, hostGroups: HOSTS })
    expect(host('ha', 'score')).toHaveValue('pulsation')
    expect(within(host('ha', 'score').closest('.asp-row')).getByText('manche')).toBeInTheDocument()
    expect(host('hb', 'score')).toHaveValue('__inherit__')
    expect(host('ha', 'chrono')).toHaveValue('__inherit__')
  })

  it('choisir un effet de manche émet la valeur et affiche l\'étiquette « manche »', () => {
    const { onChange, rerender } = setup({ mode: 'override', value: {}, base })
    fireEvent.change(room('calm'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.calm', '')
    rerender(
      <AmbianceSettingsPanel mode="override" sections={['lightEffects']} value={{ light: { effects: { calm: '' } } }} base={base} catalog={CATALOG} hostGroups={[]} onChange={onChange} />
    )
    expect(room('calm')).toHaveValue('')
    expect(within(room('calm').closest('.asp-row')).getByText('manche')).toBeInTheDocument()
  })

  it('« Aucun effet » surchargé (chaîne vide) diffère de « Hérité » : valeur globale non affichée', () => {
    setup({ mode: 'override', value: { light: { effects: { chrono: '' } } }, base })
    expect(room('chrono')).toHaveValue('')
    expect(within(room('chrono').closest('.asp-row')).queryByText(/Valeur globale appliquée/)).toBeNull()
  })

  it('en surcharge, l\'état hérité indique « Valeur globale appliquée. »', () => {
    setup({ mode: 'override', value: {}, base })
    expect(within(room('chrono').closest('.asp-row')).getByText(/Valeur globale appliquée/)).toBeInTheDocument()
  })

  it('en surcharge sans groupe animateur : aucun bloc ni invitation à en créer', () => {
    setup({ mode: 'override', value: {}, base, hostGroups: [], onCreateHost: vi.fn() })
    expect(host('ha', 'chrono')).toBeNull()
    expect(screen.queryByRole('button', { name: /en créer un/ })).toBeNull()
  })

  it('en mode global il n\'y a pas d\'état « Hérité »', () => {
    setup({ mode: 'global', value: {}, hostGroups: HOSTS })
    for (const c of ['chrono', 'calm', 'score']) {
      expect(optionTexts(room(c)).some(x => /Hérité/.test(x))).toBe(false)
      expect(optionTexts(host('ha', c)).some(x => /Hérité/.test(x))).toBe(false)
    }
  })

  it('la valeur héritée introuvable reste lisible dans « Hérité (…) »', () => {
    setup({ mode: 'override', value: {}, base: { light: { effects: { chrono: 'disparu' } } } })
    expect(optionTexts(room('chrono'))[0]).toMatch(/^Hérité \(disparu/)
  })
})
