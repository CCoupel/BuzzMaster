import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import AnimSoundActions from './AnimSoundActions'

// ---------------------------------------------------------------------------
// AnimSoundActions — ligne L2, trois gestes de conduite sur le média sonore
// de la question courante (v11.1 #219, contrat websocket-actions.md
// §QUESTION_SOUND, maquette docs/mockups/question-sound-219.html §02).
//
// Dispatché en correctif de revue (finding MAJEUR,
// _work/reports/code-review-frontend-20260922-150000.md) — patron direct :
// AnimRafaleActions.rafale.test.jsx (même composant partagé /anim + /admin,
// même style de test : props onCommand/soundState, pas de useState interne).
//
// Composant déjà livré par dev-frontend (Batch 2) — tests écrits contre son
// API réelle (props: soundState, onCommand).
// ---------------------------------------------------------------------------

const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'AnimSoundActions.css')

function getButtons(container) {
  return Array.from(container.querySelectorAll('.anim-sound-action-btn'))
}

describe('AnimSoundActions — les 3 gestes (Rejouer / Pause↔Reprendre / Stop)', () => {
  it('rend exactement 3 boutons, dans cet ordre : Rejouer, Pause/Reprendre, Stop', () => {
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={vi.fn()} />)
    const buttons = getButtons(container)
    expect(buttons).toHaveLength(3)
    expect(buttons[0].textContent).toContain('Rejouer')
    expect(buttons[2].textContent).toContain('Stop')
  })

  it("soundState='IDLE' : le libellé du 2e bouton est '⏸ Pause' (pas 'Reprendre')", () => {
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={vi.fn()} />)
    expect(getButtons(container)[1].textContent).toContain('Pause')
    expect(getButtons(container)[1].textContent).not.toContain('Reprendre')
  })

  it("soundState='PAUSED' : le libellé du 2e bouton devient '▶ Reprendre' — jamais les deux affichés (maquette §02)", () => {
    const { container } = render(<AnimSoundActions soundState="PAUSED" onCommand={vi.fn()} />)
    expect(getButtons(container)[1].textContent).toContain('Reprendre')
    expect(getButtons(container)[1].textContent).not.toContain('Pause ')
  })
})

describe('AnimSoundActions — dispatch de la commande WS au clic', () => {
  it("clic sur Rejouer → onCommand('PLAY'), quel que soit soundState", () => {
    const onCommand = vi.fn()
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={onCommand} />)
    getButtons(container)[0].click()
    expect(onCommand).toHaveBeenCalledTimes(1)
    expect(onCommand).toHaveBeenCalledWith('PLAY')
  })

  it("soundState='PLAYING', clic sur Pause → onCommand('PAUSE')", () => {
    const onCommand = vi.fn()
    const { container } = render(<AnimSoundActions soundState="PLAYING" onCommand={onCommand} />)
    getButtons(container)[1].click()
    expect(onCommand).toHaveBeenCalledWith('PAUSE')
  })

  it("soundState='PAUSED', clic sur Reprendre → onCommand('RESUME')", () => {
    const onCommand = vi.fn()
    const { container } = render(<AnimSoundActions soundState="PAUSED" onCommand={onCommand} />)
    getButtons(container)[1].click()
    expect(onCommand).toHaveBeenCalledWith('RESUME')
  })

  it("soundState='PLAYING', clic sur Stop → onCommand('STOP')", () => {
    const onCommand = vi.fn()
    const { container } = render(<AnimSoundActions soundState="PLAYING" onCommand={onCommand} />)
    getButtons(container)[2].click()
    expect(onCommand).toHaveBeenCalledWith('STOP')
  })
})

describe('AnimSoundActions — désactivation quand soundState=IDLE (rien à mettre en pause ni à arrêter)', () => {
  it("soundState='IDLE' : Pause et Stop portent l'attribut disabled natif", () => {
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={vi.fn()} />)
    const buttons = getButtons(container)
    expect(buttons[1].disabled).toBe(true)
    expect(buttons[2].disabled).toBe(true)
  })

  it("soundState='IDLE' : Rejouer reste actionnable (jamais disabled, maquette §02)", () => {
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={vi.fn()} />)
    expect(getButtons(container)[0].disabled).toBe(false)
  })

  it("soundState='IDLE' : un clic sur Pause/Stop n'émet AUCUNE commande", () => {
    const onCommand = vi.fn()
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={onCommand} />)
    const buttons = getButtons(container)
    buttons[1].click()
    buttons[2].click()
    expect(onCommand).not.toHaveBeenCalled()
  })

  it.each(['PLAYING', 'PAUSED'])("soundState='%s' : Pause et Stop sont actionnables (non disabled)", (soundState) => {
    const { container } = render(<AnimSoundActions soundState={soundState} onCommand={vi.fn()} />)
    const buttons = getButtons(container)
    expect(buttons[1].disabled).toBe(false)
    expect(buttons[2].disabled).toBe(false)
  })
})

describe('AnimSoundActions — palette anim-conduct-btn-{optional|off} réutilisée telle quelle', () => {
  it("soundState='IDLE' : Rejouer porte 'optional', Pause/Stop portent 'off'", () => {
    const { container } = render(<AnimSoundActions soundState="IDLE" onCommand={vi.fn()} />)
    const buttons = getButtons(container)
    expect(buttons[0].className).toMatch(/\banim-conduct-btn-optional\b/)
    expect(buttons[1].className).toMatch(/\banim-conduct-btn-off\b/)
    expect(buttons[2].className).toMatch(/\banim-conduct-btn-off\b/)
  })

  it("soundState='PLAYING' : les 3 boutons portent 'optional' (aucune couleur nouvelle, même sémantique que PAUSE de L1)", () => {
    const { container } = render(<AnimSoundActions soundState="PLAYING" onCommand={vi.fn()} />)
    for (const btn of getButtons(container)) {
      expect(btn.className).toMatch(/\banim-conduct-btn-optional\b/)
    }
  })

  it('ne redéfinit PAS .anim-conduct-btn-{optional|off} dans son propre CSS (réutilisation, pas de duplication)', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    expect(css).not.toMatch(/\.anim-conduct-btn-(optional|off)\s*\{/)
  })
})

describe('AnimSoundActions.css — cibles tactiles ≥62px (même seuil que le reste de L1/L2)', () => {
  it('AnimSoundActions.css fixe min-height: 62px sur .anim-sound-action-btn', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    const rule = css.match(/\.anim-sound-action-btn\s*\{([^}]*)\}/)
    expect(rule).not.toBeNull()
    const minHeightMatch = rule[1].match(/min-height:\s*(\d+)px/)
    expect(minHeightMatch).not.toBeNull()
    expect(Number(minHeightMatch[1])).toBeGreaterThanOrEqual(62)
  })
})
