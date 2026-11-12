import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #223 (v11.7.0, LOT-2, révision 4) — sélecteurs « effet lumière par zone et par moment » du composant mutualisé
// AmbianceSettingsPanel (page Ambiance en mode global, éditeur de manche en mode override) : un bloc par ZONE
// (Ambiance, Animateur, Public) de trois moments, vitesse par moment, bouton « Tester » (mode global seulement).
// Écrit depuis contracts/light-effects.md §2.5, §6, §7.1, §7.4 et la maquette docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects_zones.html.
// Champs émis : light.effects.<zone>.<moment> et light.speeds.<zone>.<moment> (jamais lighting.*, jamais d'id de groupe).
// Éléments : #asp-eff-<zone>-<moment>, #asp-spd-<zone>-<moment>, #asp-try-<zone>-<moment>, [data-testid=asp-zone-<zone>].
// Complète AmbianceSettingsPanel.zones225.test.jsx.
// ---------------------------------------------------------------------------

vi.mock('./AmbianceSettingsPanel.css', () => ({}))

import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const ALL = { ambiance: true, animateur: true, public: true }
const eff = (id, name, moments, extra = {}) => ({
  id, name, description: '', moments, origin: 'native', file: `natifs/${id}.yaml`, min_groups: 1, tick_ms: 1000, applicable: ALL, ...extra,
})

const GROUPS = { ambiance: 4, animateur: 2, public: 1 }
const CATALOG = {
  groups: GROUPS,
  effects: [
    eff('pulsation', 'Pulsation', ['chrono'], { min_groups: 0, tick_ms: 500 }),
    eff('chenillard', 'Chenillard', ['chrono'], { min_groups: 3, tick_ms: 1000, applicable: { ambiance: true, animateur: false, public: false } }),
    eff('couleur-equipe', 'Couleur de l\'équipe', ['score'], { min_groups: 0 }),
    eff('mon-effet', 'Mon effet', ['chrono', 'calm'], { origin: 'user', file: 'mon-effet.yaml' }),
    eff('degrade', 'Dégradé', ['calm'], { min_groups: 2, tick_ms: 60000, applicable: { ambiance: true, animateur: true, public: false } }),
    eff('rapide', 'Rapide', ['calm'], { min_groups: 0, tick_ms: 400 }),
  ],
  errors: [],
  unresolved: [],
  dir: 'data/files/effects',
}

const ZONES = ['ambiance', 'animateur', 'public']
const MOMENTS = ['chrono', 'calm', 'score']
const sel = (z, c) => document.getElementById(`asp-eff-${z}-${c}`)
const spd = (z, c) => document.getElementById(`asp-spd-${z}-${c}`)
const tryBtn = (z, c) => document.getElementById(`asp-try-${z}-${c}`)
const optionTexts = (el) => Array.from(el.querySelectorAll('option')).map(o => o.textContent)
const row = (el) => el.closest('.asp-row')

const full = (over = {}) => ({
  light: {
    effects: {
      ambiance: { chrono: 'pulsation', calm: '', score: '' }, animateur: { chrono: '', calm: '', score: '' }, public: { chrono: '', calm: '', score: '' },
      ...over.effects,
    },
    speeds: {
      ambiance: { chrono: 1, calm: 1, score: 1 }, animateur: { chrono: 1, calm: 1, score: 1 }, public: { chrono: 1, calm: 1, score: 1 },
      ...over.speeds,
    },
  },
})

function setup(props = {}) {
  const onChange = vi.fn()
  const utils = render(
    <AmbianceSettingsPanel mode="global" sections={['lightEffects']} value={full()} catalog={CATALOG} zoneGroups={GROUPS} onChange={onChange} {...props} />
  )
  return { onChange, ...utils }
}

describe('AmbianceSettingsPanel › effets lumière — un bloc par zone, trois moments chacun', () => {
  it('trois blocs de trois moments : chronomètre, phases calmes, équipe qui marque', () => {
    setup()
    expect(screen.getByTestId('asp-light-effects')).toBeInTheDocument()
    for (const z of ZONES) {
      const block = within(screen.getByTestId(`asp-zone-${z}`))
      for (const label of [/Pendant le chronomètre/, /Phases calmes/, /Quand une équipe marque/]) expect(block.getByText(label)).toBeInTheDocument()
      for (const c of MOMENTS) expect(sel(z, c)).toBeInTheDocument()
    }
  })

  it('section masquée tant que le catalogue n\'est pas chargé (catalog indéfini)', () => {
    setup({ catalog: undefined })
    expect(screen.queryByTestId('asp-light-effects')).toBeNull()
  })

  it('défauts globaux = comportement v11.6.0 : pulsation (chrono de l\'ambiance), aucun effet partout ailleurs', () => {
    setup({ value: {} })
    expect(sel('ambiance', 'chrono')).toHaveValue('pulsation')
    for (const [z, c] of [['ambiance', 'calm'], ['ambiance', 'score'], ...['animateur', 'public'].flatMap(z => MOMENTS.map(c => [z, c]))]) {
      expect(sel(z, c)).toHaveValue('')
    }
  })

  it('chaque liste ne propose que les effets qui déclarent le moment, « Aucun effet » en tête', () => {
    setup()
    expect(optionTexts(sel('ambiance', 'chrono')).join('|')).toMatch(/^Aucun effet\|.*Pulsation.*\|.*Chenillard.*\|.*Mon effet/)
    expect(optionTexts(sel('ambiance', 'chrono')).join('|')).not.toMatch(/Dégradé|Couleur de l'équipe/)
    expect(optionTexts(sel('public', 'calm')).join('|')).toMatch(/Mon effet/)
    expect(optionTexts(sel('public', 'calm')).join('|')).not.toMatch(/Pulsation|Chenillard/)
    expect(optionTexts(sel('animateur', 'score')).join('|')).toMatch(/Couleur de l'équipe/)
  })

  it('la première option dépend de la zone : « Aucun effet (blanc fixe) » pour l\'animateur, « Aucun effet » ailleurs', () => {
    setup({ value: {} })
    expect(optionTexts(sel('animateur', 'calm'))[0]).toBe('Aucun effet (blanc fixe)')
    expect(optionTexts(sel('ambiance', 'calm'))[0]).toMatch(/^Aucun effet( \(par défaut\))?$/)
    expect(optionTexts(sel('public', 'calm'))[0]).toMatch(/^Aucun effet( \(par défaut\))?$/)
  })

  it('natifs d\'abord, effets de l\'utilisateur ensuite, étiquetés « (personnalisé) »', () => {
    setup()
    const t = optionTexts(sel('ambiance', 'chrono'))
    expect(t.findIndex(x => /Mon effet/.test(x))).toBeGreaterThan(t.findIndex(x => /Chenillard/.test(x)))
    expect(t.find(x => /Mon effet/.test(x))).toMatch(/personnalisé/)
  })

  it('l\'option par défaut du moment est marquée « (par défaut) » en mode global (pulsation, zone ambiance)', () => {
    setup()
    expect(optionTexts(sel('ambiance', 'chrono')).some(x => /Pulsation.*\(par défaut\)/.test(x))).toBe(true)
    expect(optionTexts(sel('animateur', 'chrono')).some(x => /\(par défaut\)/.test(x))).toBe(false)
  })

  it('choisir un effet émet light.effects.<zone>.<moment> avec son id — par zone', () => {
    const { onChange } = setup()
    fireEvent.change(sel('ambiance', 'chrono'), { target: { value: 'chenillard' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.ambiance.chrono', 'chenillard')
    fireEvent.change(sel('public', 'score'), { target: { value: 'couleur-equipe' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.public.score', 'couleur-equipe')
    fireEvent.change(sel('animateur', 'calm'), { target: { value: 'degrade' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.animateur.calm', 'degrade')
  })

  it('« Aucun effet » émet la chaîne vide, pas null', () => {
    const { onChange } = setup()
    fireEvent.change(sel('ambiance', 'chrono'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.ambiance.chrono', '')
  })

  it('les valeurs enregistrées sont affichées par zone et par moment', () => {
    setup({ value: full({ effects: {
      ambiance: { chrono: 'chenillard', calm: 'mon-effet', score: 'couleur-equipe' },
      animateur: { chrono: 'pulsation', calm: '', score: 'couleur-equipe' },
      public: { chrono: '', calm: 'degrade', score: '' },
    } }) })
    expect(sel('ambiance', 'chrono')).toHaveValue('chenillard')
    expect(sel('ambiance', 'calm')).toHaveValue('mon-effet')
    expect(sel('animateur', 'chrono')).toHaveValue('pulsation')
    expect(sel('animateur', 'score')).toHaveValue('couleur-equipe')
    expect(sel('public', 'calm')).toHaveValue('degrade')
  })

  it('désactivé : les neuf sélecteurs sont figés', () => {
    setup({ disabled: true })
    for (const z of ZONES) for (const c of MOMENTS) expect(sel(z, c)).toBeDisabled()
  })
})

describe('AmbianceSettingsPanel › zone sans groupe non vide : pas de bloc', () => {
  it('seule la zone Ambiance est toujours présente ; Animateur et Public n\'apparaissent qu\'avec un groupe non vide', () => {
    setup({ zoneGroups: { ambiance: 0, animateur: 0, public: 1 }, catalog: { ...CATALOG, groups: { ambiance: 0, animateur: 0, public: 1 } } })
    expect(screen.getByTestId('asp-zone-ambiance')).toBeInTheDocument()
    expect(screen.queryByTestId('asp-zone-animateur')).toBeNull()
    expect(screen.getByTestId('asp-zone-public')).toBeInTheDocument()
    expect(sel('animateur', 'chrono')).toBeNull()
  })

  it('sans groupe animateur ni public (global) : une invitation à en créer un par zone absente', () => {
    const onCreateZone = vi.fn()
    setup({ zoneGroups: { ambiance: 3, animateur: 0, public: 0 }, catalog: { ...CATALOG, groups: { ambiance: 3, animateur: 0, public: 0 } }, onCreateZone })
    const links = screen.getAllByRole('button', { name: /en créer un/ })
    expect(links).toHaveLength(2)
    fireEvent.click(links[1])
    expect(onCreateZone).toHaveBeenLastCalledWith('public')
  })

  it('en surcharge de manche : aucune invitation à créer un groupe', () => {
    setup({ mode: 'override', value: {}, base: full(), zoneGroups: { ambiance: 3, animateur: 0, public: 0 }, onCreateZone: vi.fn() })
    expect(screen.queryByRole('button', { name: /en créer un/ })).toBeNull()
  })

  it('les réglages d\'une zone sans groupe ne sont pas perdus : le bloc réapparaît avec ses valeurs', () => {
    const value = full({ effects: { public: { chrono: '', calm: '', score: 'couleur-equipe' } } })
    const { rerender, onChange } = setup({ value, zoneGroups: { ambiance: 3, animateur: 0, public: 0 } })
    expect(sel('public', 'score')).toBeNull()
    rerender(<AmbianceSettingsPanel mode="global" sections={['lightEffects']} value={value} catalog={CATALOG} zoneGroups={{ ambiance: 3, animateur: 0, public: 1 }} onChange={onChange} />)
    expect(sel('public', 'score')).toHaveValue('couleur-equipe')
  })
})

describe('AmbianceSettingsPanel › effets lumière — effet inapplicable et introuvable, par zone', () => {
  it('un effet qui demande plus de groupes que la zone n\'en a reste visible, grisé, avec sa raison propre à la zone', () => {
    setup()
    const opt = (z) => Array.from(sel(z, 'chrono').querySelectorAll('option')).find(o => /Chenillard/.test(o.textContent))
    expect(opt('ambiance')).not.toBeDisabled()
    expect(opt('animateur')).toBeDisabled()
    expect(opt('animateur').textContent).toMatch(/demande 3 groupes, 2 dans cette zone/)
    expect(opt('public')).toBeDisabled()
    expect(opt('public').textContent).toMatch(/demande 3 groupes, 1 dans cette zone/)
  })

  it('un effet sélectionné devenu inapplicable avertit du repli propre à la zone et renvoie vers les groupes', () => {
    const onManageGroups = vi.fn()
    setup({ value: full({ effects: { animateur: { chrono: 'chenillard', calm: '', score: '' } } }), onManageGroups })
    const warn = within(row(sel('animateur', 'chrono'))).getByRole('status')
    expect(warn).toHaveTextContent(/3 groupes/)
    expect(warn).toHaveTextContent(/blanc fixe/)
    fireEvent.click(within(warn).getByRole('button', { name: /Gérer les groupes/ }))
    expect(onManageGroups).toHaveBeenCalled()
  })

  it('le repli annoncé diffère selon la zone : pulsation (ambiance, chrono), blanc fixe (animateur), rien envoyé (public)', () => {
    const low = { ambiance: 2, animateur: 1, public: 1 }
    const none = { ambiance: false, animateur: false, public: false }
    setup({ catalog: { ...CATALOG, groups: low, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, applicable: none } : e)) }, zoneGroups: low, value: full({ effects: {
      ambiance: { chrono: 'chenillard', calm: '', score: '' }, animateur: { chrono: 'chenillard', calm: '', score: '' }, public: { chrono: 'chenillard', calm: '', score: '' },
    } }) })
    const text = (z) => within(row(sel(z, 'chrono'))).getByRole('status').textContent
    expect(text('ambiance')).toMatch(/pulsation est jouée à la place/)
    expect(text('animateur')).toMatch(/blanc fixe/)
    expect(text('public')).toMatch(/rien n'est envoyé/)
  })

  it('un effet sélectionné absent du catalogue est conservé « (introuvable) » avec un avertissement', () => {
    const onOpenCatalogue = vi.fn()
    setup({ value: full({ effects: { public: { chrono: '', calm: 'fichier-supprime', score: '' } } }), onOpenCatalogue })
    expect(sel('public', 'calm')).toHaveValue('fichier-supprime')
    expect(optionTexts(sel('public', 'calm')).some(x => /fichier-supprime \(introuvable\)/.test(x))).toBe(true)
    const warn = within(row(sel('public', 'calm'))).getByRole('status')
    expect(warn).toHaveTextContent(/absent ou invalide/)
    fireEvent.click(within(warn).getByRole('button', { name: /Voir le catalogue/ }))
    expect(onOpenCatalogue).toHaveBeenCalled()
  })

  it('aucun avertissement quand l\'effet est applicable', () => {
    setup({ value: full({ effects: { animateur: { chrono: 'pulsation', calm: '', score: '' } } }) })
    expect(screen.queryByRole('status')).toBeNull()
  })
})

describe('AmbianceSettingsPanel › effets lumière — mode surcharge de manche (« Hérité »)', () => {
  const base = full({
    effects: {
      ambiance: { chrono: 'pulsation', calm: 'mon-effet', score: '' },
      animateur: { chrono: '', calm: '', score: 'couleur-equipe' },
      public: { chrono: 'pulsation', calm: '', score: '' },
    },
  })

  it('clé absente du Partial ⇒ « Hérité » sélectionné, valeur globale affichée entre parenthèses (par zone)', () => {
    setup({ mode: 'override', value: {}, base })
    expect(sel('ambiance', 'chrono')).toHaveValue('__inherit__')
    expect(optionTexts(sel('ambiance', 'chrono'))[0]).toBe('Hérité (Pulsation)')
    expect(optionTexts(sel('ambiance', 'calm'))[0]).toBe('Hérité (Mon effet (personnalisé))')
    expect(optionTexts(sel('ambiance', 'score'))[0]).toBe('Hérité (Aucun effet)')
    expect(optionTexts(sel('animateur', 'chrono'))[0]).toBe('Hérité (Aucun effet (blanc fixe))')
    expect(optionTexts(sel('animateur', 'score'))[0]).toBe('Hérité (Couleur de l\'équipe)')
    expect(optionTexts(sel('public', 'chrono'))[0]).toBe('Hérité (Pulsation)')
  })

  it('« Hérité » retire la surcharge : onChange(field, null), par zone', () => {
    const { onChange } = setup({ mode: 'override', value: { light: { effects: { ambiance: { chrono: 'chenillard' }, public: { score: 'couleur-equipe' } } } }, base })
    fireEvent.change(sel('ambiance', 'chrono'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.ambiance.chrono', null)
    fireEvent.change(sel('public', 'score'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.public.score', null)
  })

  it('surcharger une seule zone laisse les autres en « Hérité »', () => {
    setup({ mode: 'override', value: { light: { effects: { animateur: { score: 'pulsation' } } } }, base })
    expect(sel('animateur', 'score')).toHaveValue('pulsation')
    expect(within(row(sel('animateur', 'score'))).getByText('manche')).toBeInTheDocument()
    expect(sel('public', 'score')).toHaveValue('__inherit__')
    expect(sel('animateur', 'chrono')).toHaveValue('__inherit__')
    expect(sel('ambiance', 'score')).toHaveValue('__inherit__')
  })

  it('choisir un effet de manche émet la valeur et affiche l\'étiquette « manche »', () => {
    const { onChange, rerender } = setup({ mode: 'override', value: {}, base })
    fireEvent.change(sel('ambiance', 'calm'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.ambiance.calm', '')
    rerender(<AmbianceSettingsPanel mode="override" sections={['lightEffects']} value={{ light: { effects: { ambiance: { calm: '' } } } }} base={base} catalog={CATALOG} zoneGroups={GROUPS} onChange={onChange} />)
    expect(sel('ambiance', 'calm')).toHaveValue('')
    expect(within(row(sel('ambiance', 'calm'))).getByText('manche')).toBeInTheDocument()
  })

  it('« Aucun effet » surchargé (chaîne vide) diffère de « Hérité » : valeur globale non affichée', () => {
    setup({ mode: 'override', value: { light: { effects: { ambiance: { chrono: '' } } } }, base })
    expect(sel('ambiance', 'chrono')).toHaveValue('')
    expect(within(row(sel('ambiance', 'chrono'))).queryByText(/Valeurs? globales? appliquée/)).toBeNull()
  })

  it('en surcharge, l\'état hérité indique que les valeurs globales sont appliquées', () => {
    setup({ mode: 'override', value: {}, base })
    expect(within(row(sel('ambiance', 'chrono'))).getByText(/globales? appliquée/i)).toBeInTheDocument()
  })

  it('en mode global il n\'y a pas d\'état « Hérité »', () => {
    setup({ mode: 'global', value: {} })
    for (const z of ZONES) for (const c of MOMENTS) expect(optionTexts(sel(z, c)).some(x => /Hérité/.test(x))).toBe(false)
  })

  it('la valeur héritée introuvable reste lisible dans « Hérité (…) »', () => {
    setup({ mode: 'override', value: {}, base: full({ effects: { ambiance: { chrono: 'disparu', calm: '', score: '' } } }) })
    expect(optionTexts(sel('ambiance', 'chrono'))[0]).toMatch(/^Hérité \(disparu/)
  })

  it('en surcharge, jamais de bouton « Tester »', () => {
    setup({ mode: 'override', value: {}, base, onTryEffect: vi.fn() })
    expect(document.querySelector('[id^="asp-try-"]')).toBeNull()
  })
})

describe('AmbianceSettingsPanel › effets lumière — vitesse par zone et par moment', () => {
  it('un sélecteur de vitesse par zone et par moment, avec les cinq valeurs ×0,5 · ×0,75 · ×1 · ×1,5 · ×2', () => {
    setup()
    for (const z of ZONES) for (const c of MOMENTS) {
      expect(spd(z, c)).toBeInTheDocument()
      expect(optionTexts(spd(z, c))).toEqual(['×0,5', '×0,75', '×1', '×1,5', '×2'])
    }
  })

  it('défaut ×1 partout (global)', () => {
    setup({ value: {} })
    for (const z of ZONES) for (const c of MOMENTS) expect(spd(z, c)).toHaveValue('1')
  })

  it('une valeur enregistrée est affichée, par zone et par moment', () => {
    setup({ value: full({ effects: { ambiance: { chrono: 'pulsation', calm: '', score: '' } }, speeds: { ambiance: { chrono: 2, calm: 0.5, score: 1 }, public: { chrono: 1, calm: 1, score: 1.5 }, animateur: { chrono: 0.75, calm: 1, score: 1 } } }) })
    expect(spd('ambiance', 'chrono')).toHaveValue('2')
    expect(spd('ambiance', 'calm')).toHaveValue('0.5')
    expect(spd('public', 'score')).toHaveValue('1.5')
    expect(spd('animateur', 'chrono')).toHaveValue('0.75')
    expect(spd('animateur', 'calm')).toHaveValue('1')
  })

  it('choisir une vitesse émet light.speeds.<zone>.<moment> avec un NOMBRE', () => {
    const { onChange } = setup()
    fireEvent.change(spd('ambiance', 'chrono'), { target: { value: '2' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.ambiance.chrono', 2)
    fireEvent.change(spd('ambiance', 'chrono'), { target: { value: '0.75' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.ambiance.chrono', 0.75)
  })

  it('la vitesse d\'une zone n\'émet jamais le chemin d\'une autre zone', () => {
    const { onChange } = setup({ value: full({ effects: { public: { chrono: '', calm: '', score: 'couleur-equipe' } } }) })
    fireEvent.change(spd('public', 'score'), { target: { value: '0.5' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.public.score', 0.5)
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('sans effet joué, le sélecteur de vitesse est grisé (la vitesse vaut pour le moment, elle reste enregistrée)', () => {
    setup({ value: full({ effects: { ambiance: { chrono: 'chenillard', calm: '', score: '' } }, speeds: { ambiance: { chrono: 1, calm: 2, score: 1 } } }) })
    expect(spd('ambiance', 'chrono')).toBeEnabled()
    expect(spd('ambiance', 'calm')).toBeDisabled() // « Aucun effet » en phases calmes
    expect(spd('ambiance', 'calm')).toHaveValue('2') // la valeur n'est pas perdue
    expect(spd('animateur', 'chrono')).toBeDisabled() // blanc fixe
    expect(spd('public', 'chrono')).toBeDisabled() // rien envoyé
  })

  it('la vitesse vaut pour le moment quel que soit l\'effet : changer d\'effet ne la change pas', () => {
    const { onChange } = setup({ value: full({ effects: { ambiance: { chrono: 'pulsation', calm: '', score: '' } }, speeds: { ambiance: { chrono: 2, calm: 1, score: 1 } } }) })
    fireEvent.change(sel('ambiance', 'chrono'), { target: { value: 'chenillard' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.ambiance.chrono', 'chenillard')
    expect(onChange).not.toHaveBeenCalledWith('light.speeds.ambiance.chrono', expect.anything())
    expect(spd('ambiance', 'chrono')).toHaveValue('2')
  })

  it('« plafonné » apparaît quand le tick de l\'effet ÷ la vitesse tombe sous 250 ms (400 ms × 2 = 200 ms), pas à 250 ms pile', () => {
    setup({ value: full({
      effects: { ambiance: { chrono: 'pulsation', calm: 'rapide', score: '' } },
      speeds: { ambiance: { chrono: 2, calm: 2, score: 1 } },
    }) })
    expect(within(row(spd('ambiance', 'calm'))).getByText('plafonné')).toBeInTheDocument()
    // pulsation : 500 ms ÷ 2 = 250 ms : exactement le plancher, pas plafonné
    expect(within(row(spd('ambiance', 'chrono'))).queryByText('plafonné')).toBeNull()
  })

  it('pas de « plafonné » à vitesse ×1', () => {
    setup({ value: full({ effects: { ambiance: { chrono: 'pulsation', calm: 'rapide', score: '' } } }) })
    expect(within(row(spd('ambiance', 'calm'))).queryByText('plafonné')).toBeNull()
  })

  it('mode surcharge : « Hérité (×n) » avec la vitesse globale ; « Hérité » émet null', () => {
    const base = full({ effects: { ambiance: { chrono: 'pulsation', calm: '', score: '' } }, speeds: { ambiance: { chrono: 1.5, calm: 1, score: 1 }, animateur: { chrono: 1, calm: 1, score: 0.5 } } })
    const { onChange } = setup({ mode: 'override', value: { light: { speeds: { ambiance: { chrono: 2 }, animateur: { score: 1 } } } }, base })
    expect(optionTexts(spd('ambiance', 'score'))[0]).toBe('Hérité (×1)')
    expect(optionTexts(spd('ambiance', 'chrono'))[0]).toBe('Hérité (×1,5)')
    expect(optionTexts(spd('animateur', 'score'))[0]).toBe('Hérité (×0,5)')
    expect(spd('ambiance', 'chrono')).toHaveValue('2') // surchargée
    fireEvent.change(spd('ambiance', 'chrono'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.ambiance.chrono', null)
    fireEvent.change(spd('animateur', 'score'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.animateur.score', null)
  })

  it('mode surcharge : seule une vitesse surchargée affiche l\'étiquette « manche »', () => {
    setup({ mode: 'override', value: { light: { speeds: { ambiance: { chrono: 2 } } } }, base: full() })
    expect(within(row(spd('ambiance', 'chrono'))).getByText('manche')).toBeInTheDocument()
    expect(within(row(spd('ambiance', 'calm'))).queryByText('manche')).toBeNull()
  })

  it('en mode global il n\'y a pas d\'état « Hérité » pour la vitesse', () => {
    setup({ mode: 'global', value: {} })
    for (const z of ZONES) for (const c of MOMENTS) expect(optionTexts(spd(z, c)).some(x => /Hérité/.test(x))).toBe(false)
  })

  it('désactivé : tous les sélecteurs de vitesse sont figés', () => {
    setup({ disabled: true })
    for (const z of ZONES) for (const c of MOMENTS) expect(spd(z, c)).toBeDisabled()
  })
})

describe('AmbianceSettingsPanel › « Tester » un effet (§7.4)', () => {
  const withEffects = (z, c, id, speed = 1) => full({ effects: { [z]: { chrono: '', calm: '', score: '', [c]: id } }, speeds: { [z]: { chrono: 1, calm: 1, score: 1, [c]: speed } } })

  it('un bouton « ▶ Tester » par ligne (mode global seulement, quand onTryEffect est fourni)', () => {
    setup({ onTryEffect: vi.fn() })
    for (const z of ZONES) for (const c of MOMENTS) expect(tryBtn(z, c)).toBeInTheDocument()
    expect(tryBtn('ambiance', 'chrono')).toHaveTextContent('▶ Tester')
  })

  it('sans onTryEffect, aucun bouton « Tester »', () => {
    setup()
    expect(document.querySelector('[id^="asp-try-"]')).toBeNull()
  })

  it('cliquer joue l\'effet choisi sur SA zone, à la vitesse réglée', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    setup({ value: withEffects('public', 'score', 'couleur-equipe', 0.5), onTryEffect })
    fireEvent.click(tryBtn('public', 'score'))
    await waitFor(() => expect(onTryEffect).toHaveBeenCalledWith({ zone: 'public', moment: 'score', id: 'couleur-equipe', speed: 0.5 }))
  })

  it('pendant le test : « ■ Arrêter », infobulle avec la zone et la vitesse ; un clic arrête (onStopTry)', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    const onStopTry = vi.fn().mockResolvedValue({ ok: true })
    setup({ value: withEffects('animateur', 'chrono', 'pulsation', 1.5), onTryEffect, onStopTry })
    fireEvent.click(tryBtn('animateur', 'chrono'))
    const stop = await screen.findByRole('button', { name: '■ Arrêter' })
    expect(stop.title).toMatch(/zone Animateur/)
    expect(stop.title).toMatch(/×1,5/)
    fireEvent.click(stop)
    await waitFor(() => expect(onStopTry).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(screen.queryByRole('button', { name: '■ Arrêter' })).toBeNull())
  })

  it('un seul test à la fois : pendant l\'un, il n\'y a qu\'un bouton « ■ Arrêter »', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    setup({ value: full({ effects: { ambiance: { chrono: 'pulsation', calm: 'mon-effet', score: '' } } }), onTryEffect })
    fireEvent.click(tryBtn('ambiance', 'chrono'))
    await screen.findByRole('button', { name: '■ Arrêter' })
    expect(screen.getAllByRole('button', { name: '■ Arrêter' })).toHaveLength(1)
  })

  it('changer un réglage pendant le test l\'arrête (effet ou vitesse)', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    const onStopTry = vi.fn().mockResolvedValue({ ok: true })
    setup({ value: withEffects('ambiance', 'chrono', 'pulsation'), onTryEffect, onStopTry })
    fireEvent.click(tryBtn('ambiance', 'chrono'))
    await screen.findByRole('button', { name: '■ Arrêter' })
    fireEvent.change(sel('ambiance', 'chrono'), { target: { value: 'chenillard' } })
    await waitFor(() => expect(onStopTry).toHaveBeenCalled())
  })

  it('« Tester » grisé avec la raison : aucun effet, effet inapplicable, indisponibilité de la page', () => {
    setup({
      value: full({ effects: { ambiance: { chrono: 'chenillard', calm: '', score: '' }, animateur: { chrono: 'chenillard', calm: '', score: '' }, public: { chrono: 'pulsation', calm: '', score: '' } } }),
      onTryEffect: vi.fn(), tryUnavailable: (z) => (z === 'public' ? 'Une question est en cours' : ''),
    })
    expect(tryBtn('ambiance', 'calm')).toBeDisabled()
    expect(tryBtn('ambiance', 'calm').title).toBe('Aucun effet choisi')
    expect(tryBtn('animateur', 'chrono')).toBeDisabled()
    expect(tryBtn('animateur', 'chrono').title).toBe('Effet inapplicable à cette zone')
    expect(tryBtn('public', 'chrono')).toBeDisabled()
    expect(tryBtn('public', 'chrono').title).toBe('Une question est en cours')
    expect(tryBtn('ambiance', 'chrono')).toBeEnabled()
  })

  it('désactivé : les boutons « Tester » sont figés', () => {
    setup({ disabled: true, onTryEffect: vi.fn(), value: withEffects('ambiance', 'chrono', 'pulsation') })
    expect(tryBtn('ambiance', 'chrono')).toBeDisabled()
  })

  it('un test refusé ne laisse pas le bouton en « Arrêter »', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: false, message: 'refusé' })
    setup({ value: withEffects('ambiance', 'chrono', 'pulsation'), onTryEffect })
    fireEvent.click(tryBtn('ambiance', 'chrono'))
    await waitFor(() => expect(onTryEffect).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: '■ Arrêter' })).toBeNull()
  })
})
