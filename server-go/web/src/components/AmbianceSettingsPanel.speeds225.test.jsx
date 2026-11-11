import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'

vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const CATALOG = {
  groups: 3,
  effects: [
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono'], origin: 'native', min_groups: 0, applicable: true, host_applicable: true, tick_ms: 1000 },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 5, applicable: false, reason: '', host_applicable: false, tick_ms: 400 },
  ],
}
const GLOBAL = { light: {
  effects: { chrono: 'pulsation', calm: '', score: '' },
  host_effects: { h1: { chrono: '', calm: '', score: '' } },
  speeds: { chrono: 1, calm: 1, score: 1 }, host_speeds: { h1: { chrono: 1, calm: 1, score: 1 } },
} }
const HOSTS = [{ id: 'h1', rank: 1, empty: false }]
const $ = (id) => document.getElementById(id)

describe('AmbianceSettingsPanel — vitesse et libellés révision 3 (#223 R9/R14)', () => {
  it('un sélecteur de vitesse par moment (×0,5 … ×2), émet light.speeds.<moment> en nombre', () => {
    const onChange = vi.fn()
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} hostGroups={HOSTS} onChange={onChange} />)
    const sel = $('asp-spd-room-chrono')
    expect(Array.from(sel.options).map(o => o.textContent)).toEqual(['×0,5', '×0,75', '×1', '×1,5', '×2'])
    expect(sel.value).toBe('1')
    fireEvent.change(sel, { target: { value: '1.5' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.chrono', 1.5)
    fireEvent.change($('asp-spd-host-h1-calm'), { target: { value: '2' } })
    expect(onChange).toHaveBeenCalledWith('light.host_speeds.h1.calm', 2)
  })

  it('vitesse grisée quand aucun effet n\'est joué (Aucun effet / Blanc fixe)', () => {
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} hostGroups={HOSTS} onChange={() => {}} />)
    expect($('asp-spd-room-chrono')).not.toBeDisabled()
    expect($('asp-spd-room-calm')).toBeDisabled()
    expect($('asp-spd-host-h1-chrono')).toBeDisabled()
  })

  it('« plafonné » quand tick ÷ vitesse < 250 ms (tick_ms du catalogue)', () => {
    const value = { light: { ...GLOBAL.light, effects: { ...GLOBAL.light.effects, chrono: 'pulsation' }, speeds: { chrono: 2, calm: 1, score: 1 } } }
    const { container, rerender } = render(<AmbianceSettingsPanel mode="global" value={value} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} />)
    expect(container.textContent).not.toMatch(/plafonné/) // 1000 / 2 = 500 ms
    const fast = { light: { ...value.light, effects: { chrono: 'chenillard', calm: '', score: '' } } }
    rerender(<AmbianceSettingsPanel mode="global" value={fast} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} />)
    expect(container.textContent).toMatch(/plafonné/) // 400 / 2 = 200 ms
  })

  it('mode surcharge : « Hérité (×n) », valeur null retire la surcharge', () => {
    const onChange = vi.fn()
    render(<AmbianceSettingsPanel mode="override" value={{}} base={{ light: { ...GLOBAL.light, speeds: { chrono: 1.5, calm: 1, score: 1 } } }} sections={['lightEffects']} catalog={CATALOG} onChange={onChange} />)
    const sel = $('asp-spd-room-chrono')
    expect(sel.value).toBe('__inherit__')
    expect(sel.options[0].textContent).toBe('Hérité (×1,5)')
    fireEvent.change(sel, { target: { value: '0.5' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.chrono', 0.5)
    fireEvent.change(sel, { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenLastCalledWith('light.speeds.chrono', null)
  })

  it('blocs « Groupe animateur n » sans nom ; raisons « demande n groupes » (salle) et « anime plusieurs groupes » (animateur)', () => {
    const { container } = render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} hostGroups={HOSTS} onChange={() => {}} />)
    expect(container.textContent).toMatch(/🎤 Groupe animateur 1/)
    const room = Array.from($('asp-eff-room-chrono').options).find(o => /Chenillard/.test(o.textContent))
    expect(room.textContent).toBe('Chenillard — demande 5 groupes, 3 configurés')
    const host = Array.from($('asp-eff-host-h1-chrono').options).find(o => /Chenillard/.test(o.textContent))
    expect(host.textContent).toBe('Chenillard — anime plusieurs groupes')
    expect(host).toBeDisabled()
    expect(Array.from($('asp-eff-room-chrono').options).map(o => o.textContent)).toContain('Pulsation (par défaut)')
  })
})
