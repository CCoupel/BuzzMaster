import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, waitFor, screen } from '@testing-library/react'

vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const CATALOG = {
  groups: { ambiance: 3, animateur: 2, public: 1 },
  effects: [
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono', 'score'], origin: 'native', min_groups: 0, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: true } },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 5, tick_ms: 400, applicable: { ambiance: false, animateur: false, public: false } },
  ],
}
const GROUPS = { ambiance: 3, animateur: 2, public: 1 }
const FULL = (over = {}) => ({ light: {
  effects: { ambiance: { chrono: 'pulsation', calm: '', score: '' }, animateur: { chrono: '', calm: '', score: '' }, public: { chrono: '', calm: '', score: '' }, ...over.effects },
  speeds: { ambiance: { chrono: 1, calm: 1, score: 1 }, animateur: { chrono: 1, calm: 1, score: 1 }, public: { chrono: 1, calm: 1, score: 1 }, ...over.speeds },
} })
const $ = (id) => document.getElementById(id)
const mount = (props = {}) => render(
  <AmbianceSettingsPanel mode="global" value={FULL()} sections={['lightEffects']} catalog={CATALOG} zoneGroups={GROUPS} onChange={() => {}} {...props} />
)

describe('AmbianceSettingsPanel — un bloc par zone, vitesse, « Tester » (#223, révision 4)', () => {
  it('trois blocs de trois moments ; zone sans groupe = pas de bloc, ligne « en créer un »', () => {
    const onCreateZone = vi.fn()
    mount({ zoneGroups: { ambiance: 3, animateur: 0, public: 1 }, onCreateZone })
    expect(screen.getByTestId('asp-zone-ambiance')).toHaveTextContent('Zone Ambiance')
    expect(screen.queryByTestId('asp-zone-animateur')).toBeNull()
    expect(screen.getByTestId('asp-zone-public')).toHaveTextContent('Zone Public')
    expect($('asp-eff-public-score')).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'en créer un' }))
    expect(onCreateZone).toHaveBeenCalledWith('animateur')
  })

  it('effet : émet light.effects.<zone>.<moment> ; vitesse : light.speeds.<zone>.<moment> en nombre', () => {
    const onChange = vi.fn()
    mount({ onChange })
    fireEvent.change($('asp-eff-animateur-chrono'), { target: { value: 'pulsation' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.animateur.chrono', 'pulsation')
    fireEvent.change($('asp-spd-ambiance-chrono'), { target: { value: '1.5' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.ambiance.chrono', 1.5)
  })

  it('libellés par zone : « Aucun effet (blanc fixe) » (animateur), « (par défaut) » (ambiance, public), raison « demande n groupes, m dans cette zone »', () => {
    mount()
    const texts = (id) => Array.from($(id).options).map(o => o.textContent)
    expect(texts('asp-eff-ambiance-chrono')).toContain('Pulsation (par défaut)')
    expect(texts('asp-eff-animateur-chrono')[0]).toBe('Aucun effet (blanc fixe)')
    expect(texts('asp-eff-public-chrono')[0]).toBe('Aucun effet (par défaut)')
    expect(texts('asp-eff-ambiance-chrono')).toContain('Chenillard — demande 5 groupes, 3 dans cette zone')
    expect(texts('asp-eff-animateur-chrono')).toContain('Chenillard — demande 5 groupes, 2 dans cette zone')
    expect(document.body.textContent).not.toMatch(/anime plusieurs groupes/)
  })

  it('vitesse grisée sans effet ; « plafonné » d\'après tick_ms', () => {
    const { container, rerender } = mount()
    expect($('asp-spd-ambiance-chrono')).not.toBeDisabled()
    expect($('asp-spd-animateur-chrono')).toBeDisabled()
    expect(container.textContent).not.toMatch(/plafonné/)
    rerender(<AmbianceSettingsPanel mode="global" value={FULL({ effects: { ambiance: { chrono: 'chenillard', calm: '', score: '' } }, speeds: { ambiance: { chrono: 2, calm: 1, score: 1 } } })} sections={['lightEffects']} catalog={CATALOG} zoneGroups={GROUPS} onChange={() => {}} />)
    expect(container.textContent).toMatch(/plafonné/)
  })

  it('« Tester » : joue effet, zone, vitesse réglée ; « ■ Arrêter » ; changer la vitesse arrête le test', async () => {
    const onTryEffect = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    const onStopTry = vi.fn().mockResolvedValue({ ok: true })
    mount({ value: FULL({ effects: { animateur: { chrono: 'pulsation', calm: '', score: '' } }, speeds: { animateur: { chrono: 1.5, calm: 1, score: 1 } } }), onTryEffect, onStopTry })
    fireEvent.click($('asp-try-animateur-chrono'))
    await waitFor(() => expect(onTryEffect).toHaveBeenCalledWith({ zone: 'animateur', moment: 'chrono', id: 'pulsation', speed: 1.5 }))
    const stop = await screen.findByRole('button', { name: '■ Arrêter' })
    expect(stop.title).toMatch(/zone Animateur, à la vitesse ×1,5/)
    fireEvent.change($('asp-spd-animateur-chrono'), { target: { value: '2' } })
    await waitFor(() => expect(onStopTry).toHaveBeenCalled())
    await waitFor(() => expect(screen.queryByRole('button', { name: '■ Arrêter' })).toBeNull())
  })

  it('« Tester » grisé avec la raison : aucun effet, effet inapplicable, indisponibilité de la page', () => {
    mount({
      value: FULL({ effects: { ambiance: { chrono: 'chenillard', calm: '', score: '' }, public: { chrono: 'pulsation', calm: '', score: '' } } }),
      onTryEffect: vi.fn(), tryUnavailable: (z) => (z === 'public' ? "Éclairage d'ambiance désactivé" : ''),
    })
    expect($('asp-try-ambiance-calm')).toBeDisabled()
    expect($('asp-try-ambiance-calm').title).toBe('Aucun effet choisi')
    expect($('asp-try-ambiance-chrono')).toBeDisabled()
    expect($('asp-try-ambiance-chrono').title).toBe('Effet inapplicable à cette zone')
    expect($('asp-try-public-chrono')).toBeDisabled()
    expect($('asp-try-public-chrono').title).toBe("Éclairage d'ambiance désactivé")
  })

  it('mode surcharge : « Hérité (…) », jamais de bouton « Tester », null retire la surcharge', () => {
    const onChange = vi.fn()
    mount({ mode: 'override', value: {}, base: FULL({ speeds: { public: { chrono: 1.5, calm: 1, score: 1 } } }), onChange, onTryEffect: vi.fn() })
    expect(document.querySelector('[id^="asp-try-"]')).toBeNull()
    expect($('asp-eff-ambiance-chrono').value).toBe('__inherit__')
    expect($('asp-eff-ambiance-chrono').options[0].textContent).toBe('Hérité (Pulsation)')
    expect($('asp-spd-public-chrono').options[0].textContent).toBe('Hérité (×1,5)')
    fireEvent.change($('asp-spd-public-chrono'), { target: { value: '0.5' } })
    expect(onChange).toHaveBeenCalledWith('light.speeds.public.chrono', 0.5)
    fireEvent.change($('asp-spd-public-chrono'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenLastCalledWith('light.speeds.public.chrono', null)
  })

  it('focusZone met le bloc de la zone en évidence', () => {
    mount({ focusZone: 'public' })
    expect(screen.getByTestId('asp-zone-public').className).toMatch(/is-focus/)
    expect(screen.getByTestId('asp-zone-animateur').className).not.toMatch(/is-focus/)
  })
})
