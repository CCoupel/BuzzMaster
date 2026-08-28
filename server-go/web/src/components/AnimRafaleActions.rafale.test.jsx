import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import AnimRafaleActions from './AnimRafaleActions'

// ---------------------------------------------------------------------------
// AnimRafaleActions — ligne L2 (« gestes propres au mode ») pendant une
// manche RAFALE, `/anim` (milestone v8.0.0 #16/#198, contrat
// contracts/rafale.md §5.1/§8.1, maquette docs/mockups/rafale-v8.html §3).
//
// Deux actions client→serveur SANS payload : RAFALE_VALIDATE / RAFALE_
// INVALIDATE (contrat §5.1) — aucune action d'attribution de points ici
// (§6.2 : TEAM_POINTS existant, réutilisé ailleurs). Cibles tactiles ≥62px
// (min-height, AnimRafaleActions.css) — même seuil que le reste de L1/L2
// (AnimConductPanel), exigence maquette pour un usage tablette en conditions
// de jeu.
//
// Composant déjà livré par dev-frontend au moment où ce fichier est écrit
// (Batch 1) — tests écrits contre son API réelle (props: disabled,
// onValidate, onInvalidate), conforme au contrat.
// ---------------------------------------------------------------------------

const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'AnimRafaleActions.css')

function getButtons(container) {
  return Array.from(container.querySelectorAll('.anim-rafale-action-btn'))
}

describe('AnimRafaleActions — les 2 actions (RAFALE_VALIDATE / RAFALE_INVALIDATE)', () => {
  it('rend exactement 2 boutons : RÉPONSE VALIDE et RÉPONSE INVALIDE', () => {
    const { container } = render(
      <AnimRafaleActions onValidate={vi.fn()} onInvalidate={vi.fn()} />
    )
    const buttons = getButtons(container)
    expect(buttons).toHaveLength(2)
    expect(buttons[0].textContent).toContain('RÉPONSE VALIDE')
    expect(buttons[1].textContent).toContain('RÉPONSE INVALIDE')
  })

  it('clic sur RÉPONSE VALIDE appelle onValidate() exactement une fois', () => {
    const onValidate = vi.fn()
    const onInvalidate = vi.fn()
    const { container } = render(
      <AnimRafaleActions onValidate={onValidate} onInvalidate={onInvalidate} />
    )
    getButtons(container)[0].click()
    expect(onValidate).toHaveBeenCalledTimes(1)
    expect(onInvalidate).not.toHaveBeenCalled()
  })

  it('clic sur RÉPONSE INVALIDE appelle onInvalidate() exactement une fois', () => {
    const onValidate = vi.fn()
    const onInvalidate = vi.fn()
    const { container } = render(
      <AnimRafaleActions onValidate={onValidate} onInvalidate={onInvalidate} />
    )
    getButtons(container)[1].click()
    expect(onInvalidate).toHaveBeenCalledTimes(1)
    expect(onValidate).not.toHaveBeenCalled()
  })

  it('les deux boutons utilisent la palette anim-conduct-btn-{go|danger} existante (aucune couleur nouvelle)', () => {
    const { container } = render(<AnimRafaleActions onValidate={vi.fn()} onInvalidate={vi.fn()} />)
    const buttons = getButtons(container)
    expect(buttons[0].className).toMatch(/\banim-conduct-btn-go\b/)
    expect(buttons[1].className).toMatch(/\banim-conduct-btn-danger\b/)
  })
})

describe('AnimRafaleActions — disabled (hors sous-phase QUESTION, ou question déjà jugée)', () => {
  it('disabled={true} : les 2 boutons portent l\'attribut disabled natif', () => {
    const { container } = render(
      <AnimRafaleActions disabled onValidate={vi.fn()} onInvalidate={vi.fn()} />
    )
    for (const btn of getButtons(container)) {
      expect(btn.disabled).toBe(true)
    }
  })

  it('disabled={true} : un clic n\'émet AUCUNE action (ni onValidate ni onInvalidate)', () => {
    const onValidate = vi.fn()
    const onInvalidate = vi.fn()
    const { container } = render(
      <AnimRafaleActions disabled onValidate={onValidate} onInvalidate={onInvalidate} />
    )
    for (const btn of getButtons(container)) {
      btn.click()
    }
    expect(onValidate).not.toHaveBeenCalled()
    expect(onInvalidate).not.toHaveBeenCalled()
  })

  it('disabled non fourni (défaut false) : les boutons restent cliquables', () => {
    const { container } = render(<AnimRafaleActions onValidate={vi.fn()} onInvalidate={vi.fn()} />)
    for (const btn of getButtons(container)) {
      expect(btn.disabled).toBe(false)
    }
  })
})

describe('AnimRafaleActions — cibles tactiles ≥62px (maquette, usage tablette)', () => {
  it('AnimRafaleActions.css fixe min-height: 62px sur .anim-rafale-action-btn', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    const rule = css.match(/\.anim-rafale-action-btn\s*\{([^}]*)\}/)
    expect(rule).not.toBeNull()
    const minHeightMatch = rule[1].match(/min-height:\s*(\d+)px/)
    expect(minHeightMatch).not.toBeNull()
    expect(Number(minHeightMatch[1])).toBeGreaterThanOrEqual(62)
  })

  it('les 2 boutons rendus portent bien la classe .anim-rafale-action-btn (celle qui porte le seuil)', () => {
    const { container } = render(<AnimRafaleActions onValidate={vi.fn()} onInvalidate={vi.fn()} />)
    expect(getButtons(container)).toHaveLength(2)
  })
})

describe('AnimRafaleActions.css — palette réutilisée telle quelle (aucune redéfinition)', () => {
  it('ne redéfinit PAS .anim-conduct-btn-{go|danger} (réutilisation, pas de duplication)', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    expect(css).not.toMatch(/\.anim-conduct-btn-(go|danger)\s*\{/)
  })
})
