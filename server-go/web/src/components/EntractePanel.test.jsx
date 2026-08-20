import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import EntractePanel from './EntractePanel'

// ---------------------------------------------------------------------------
// EntractePanel — panneau du mode ENTRACTE (v6.5.2, #119).
//
// Plan : _work/reports/plan-entracte-119-20260820-140825.md, tâches F2/F2bis,
// T2/T2bis. Contrat : contracts/game-state.md §ENTRACTE_CONFIG.
//
// Point central du composant (D7/D8) : AUCUNE branche isVPlayer — le rendu
// doit être STRICTEMENT identique quel que soit l'appelant (TV ou VJoueur),
// donc ce fichier ne teste jamais de variante de surface, seulement la prop
// `config`.
// ---------------------------------------------------------------------------

vi.mock('../styles/entracte.css', () => ({}))

function baseConfig(overrides = {}) {
  return {
    TITLE: 'ENTRACTE',
    SUBTITLE: 'Retour dans 20mn',
    IMAGE_IS_CUSTOM: false,
    PANEL_SIZE: 65,
    ANIM_PERIOD: 10,
    ANIM_INTENSITY: 20,
    // C3 (delta #119, plan-entracte-119-fixes-20260820-155123.md) — durée du
    // fondu d'entrée/sortie, ms, défaut 2000, bornée 0-10000.
    TRANSITION_MS: 2000,
    ...overrides,
  }
}

function getPanel(container) {
  return container.querySelector('.entracte-panel')
}

describe('EntractePanel — rendu du titre/sous-titre (T2)', () => {
  it('affiche le titre et le sous-titre issus de la config', () => {
    render(<EntractePanel config={baseConfig({ TITLE: 'Pause déjeuner', SUBTITLE: 'Retour à 13h30' })} />)
    expect(screen.getByText('Pause déjeuner')).toBeInTheDocument()
    expect(screen.getByText('Retour à 13h30')).toBeInTheDocument()
  })

  it("n'affiche pas d'élément sous-titre quand SUBTITLE est vide", () => {
    const { container } = render(<EntractePanel config={baseConfig({ SUBTITLE: '' })} />)
    expect(container.querySelector('.entracte-panel-subtitle')).toBeNull()
  })

  it('retombe sur les défauts contractuels quand config est absent (client défensif)', () => {
    render(<EntractePanel />)
    expect(screen.getByText('ENTRACTE')).toBeInTheDocument()
  })
})

describe('EntractePanel — image de fond présente/absente (T2)', () => {
  it("n'affiche aucun calque de fond quand IMAGE_IS_CUSTOM est false", () => {
    const { container } = render(<EntractePanel config={baseConfig({ IMAGE_IS_CUSTOM: false })} />)
    expect(container.querySelector('.entracte-panel-bg')).toBeNull()
  })

  it('affiche un calque de fond pointant vers /api/game/entracte-image quand IMAGE_IS_CUSTOM est true', () => {
    // Fix (dev-frontend, C1-B6) : endpoint renommé /api/config/entracte-image
    // → /api/game/entracte-image (l'image appartient désormais à la partie,
    // pas à la config serveur — corrections C1, http-endpoints.md).
    const { container } = render(<EntractePanel config={baseConfig({ IMAGE_IS_CUSTOM: true })} />)
    const bg = container.querySelector('.entracte-panel-bg')
    expect(bg).not.toBeNull()
    expect(bg.style.backgroundImage).toContain('/api/game/entracte-image')
  })

  it('aucun chemin de fichier ne transite — seule l\'URL stable /api/game/entracte-image apparaît (contrat http-endpoints.md)', () => {
    const { container } = render(<EntractePanel config={baseConfig({ IMAGE_IS_CUSTOM: true })} />)
    const bg = container.querySelector('.entracte-panel-bg')
    expect(bg.style.backgroundImage).not.toMatch(/data\/files|\.jpg|\.png|\.jpeg|\.webp/i)
  })
})

describe('EntractePanel — PANEL_SIZE appliqué (T2)', () => {
  it('applique PANEL_SIZE à la variable --ep-size du panneau (un seul réglage, largeur ET hauteur — D7)', () => {
    const { container } = render(<EntractePanel config={baseConfig({ PANEL_SIZE: 80 })} />)
    const panel = getPanel(container)
    expect(panel.style.getPropertyValue('--ep-size')).toBe('80%')
  })

  it('reflète le défaut contractuel PANEL_SIZE=65 quand non précisé', () => {
    const { container } = render(<EntractePanel config={baseConfig()} />)
    expect(getPanel(container).style.getPropertyValue('--ep-size')).toBe('65%')
  })
})

describe('EntractePanel — rendu identique quel que soit l\'appelant (D7, aucune branche isVPlayer)', () => {
  it('la même config produit un DOM strictement identique entre deux montages (pas de variante cachée)', () => {
    const config = baseConfig({ TITLE: 'Identique', SUBTITLE: 'Partout', PANEL_SIZE: 72 })
    const { container: c1 } = render(<EntractePanel config={config} />)
    const { container: c2 } = render(<EntractePanel config={config} />)
    expect(c1.querySelector('.entracte-panel').outerHTML).toBe(c2.querySelector('.entracte-panel').outerHTML)
  })

  it('le composant n\'accepte pas de prop de variante de surface (isVPlayer, variant…) qui changerait le rendu', () => {
    // Passer une prop étrangère ne doit avoir AUCUN effet — le composant ne
    // lit que `config`. Absence de branche isVPlayer garantie par construction.
    const config = baseConfig()
    const { container: c1 } = render(<EntractePanel config={config} />)
    const { container: c2 } = render(<EntractePanel config={config} isVPlayer variant="player" />)
    expect(c1.querySelector('.entracte-panel').outerHTML).toBe(c2.querySelector('.entracte-panel').outerHTML)
  })
})

// ---------------------------------------------------------------------------
// T2 bis — Animation (D8) : ANIM_PERIOD/ANIM_INTENSITY répercutées sur les
// variables CSS ; ANIM_INTENSITY=0 => AUCUNE classe/animation appliquée, pas
// seulement une amplitude nulle qui tournerait quand même (le panneau reste
// affiché 20 minutes, pas de boucle de composition inutile).
// ---------------------------------------------------------------------------

describe('EntractePanel — animation du panneau (T2 bis, D8)', () => {
  it('répercute ANIM_PERIOD et ANIM_INTENSITY sur les variables CSS --ep-anim-duration / --ep-anim-intensity', () => {
    const { container } = render(<EntractePanel config={baseConfig({ ANIM_PERIOD: 6, ANIM_INTENSITY: 40 })} />)
    const panel = getPanel(container)
    expect(panel.style.getPropertyValue('--ep-anim-duration')).toBe('6s')
    expect(panel.style.getPropertyValue('--ep-anim-intensity')).toBe('40')
  })

  it('ANIM_INTENSITY > 0 : la classe d\'animation est appliquée', () => {
    const { container } = render(<EntractePanel config={baseConfig({ ANIM_INTENSITY: 20 })} />)
    expect(getPanel(container).classList.contains('entracte-panel--animated')).toBe(true)
  })

  it('ANIM_INTENSITY = 0 : AUCUNE classe d\'animation n\'est appliquée (désactivation réelle, pas une amplitude nulle)', () => {
    const { container } = render(<EntractePanel config={baseConfig({ ANIM_INTENSITY: 0 })} />)
    expect(getPanel(container).classList.contains('entracte-panel--animated')).toBe(false)
  })

  it('ANIM_INTENSITY = 0 : la variable --ep-anim-intensity reste à 0 (traçable), seule la classe pilote l\'activation', () => {
    const { container } = render(<EntractePanel config={baseConfig({ ANIM_INTENSITY: 0 })} />)
    expect(getPanel(container).style.getPropertyValue('--ep-anim-intensity')).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Transition (C3, delta #119 —
// _work/reports/plan-entracte-119-fixes-20260820-155123.md) : TRANSITION_MS
// pilote la durée du fondu d'entrée/sortie via --ep-transition. Cherché sur
// `.entracte-panel-stage` (le conteneur porteur du fondu, cf. C3 : « le
// fondu se pose sur .entracte-panel-stage ») OU `.entracte-panel` — la
// variable CSS doit être posée sur l'un des deux (une variable héritée
// depuis un ancêtre plus haut ne serait pas visible sur `.style` ici).
// ---------------------------------------------------------------------------

function getTransitionValue(container) {
  const stage = container.querySelector('.entracte-panel-stage')
  const panel = getPanel(container)
  return stage?.style.getPropertyValue('--ep-transition') || panel?.style.getPropertyValue('--ep-transition') || ''
}

describe('EntractePanel — transition d\'entrée/sortie (C3)', () => {
  it('répercute TRANSITION_MS sur la variable CSS --ep-transition', () => {
    const { container } = render(<EntractePanel config={baseConfig({ TRANSITION_MS: 1500 })} />)
    expect(getTransitionValue(container)).toBe('1500ms')
  })

  it('reflète le défaut contractuel TRANSITION_MS=2000 quand non précisé', () => {
    const { container } = render(<EntractePanel config={baseConfig()} />)
    expect(getTransitionValue(container)).toBe('2000ms')
  })

  it('TRANSITION_MS=0 (bascule instantanée) est répercuté tel quel, pas retombé sur le défaut', () => {
    const { container } = render(<EntractePanel config={baseConfig({ TRANSITION_MS: 0 })} />)
    expect(getTransitionValue(container)).toBe('0ms')
  })
})
