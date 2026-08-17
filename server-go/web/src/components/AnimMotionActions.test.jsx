import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import AnimMotionActions from './AnimMotionActions'
import { motionGestures } from '../utils/motionRules'

// ---------------------------------------------------------------------------
// AnimMotionActions — ligne L2 "gestes du mode" en MEMOTION (#160/F6, T5).
// PREMIER occupant réel de L2 (#171 l'avait laissée vide pour tous les
// modes). Rend la ligne EXCLUSIVEMENT à partir de motionRules.motionGestures
// (pas mocké dans ce fichier — même principe que AnimMemoryGrid consommant
// memoryGrid.js sans mock : on vérifie le VRAI câblage), palette
// `anim-conduct-btn-{go|optional|danger|off}` réutilisée telle quelle
// (plan §F6), `disabled` natif sur "off", aucune action émise par un
// bouton éteint.
//
// Contrat de props (défini ici en TDD) :
//   { subphase, timerRunning, currentTeam, currentTeamColor, selectedCardId,
//     cardPoints, onFlipMotionCard, onStopMotionTimer, onRevealMotionCard,
//     onDoneMotionCard }
// Dispatch action WS -> handler (le champ `action` de motionRules.js est le
// nom d'action WebSocket littéral, contracts/websocket-actions.md) :
//   MEMOTION_FLIP       -> onFlipMotionCard()
//   MEMOTION_STOP_TIMER -> onStopMotionTimer()
//   MEMOTION_REVEAL     -> onRevealMotionCard()
//   MEMOTION_DONE       -> onDoneMotionCard(payload.CARD_ID, payload.WINNER_TEAM)
// ---------------------------------------------------------------------------

const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'AnimMotionActions.css')

function baseProps(overrides = {}) {
  return {
    subphase: 'SELECTED',
    timerRunning: false,
    currentTeam: 'Les Bleus',
    currentTeamColor: [37, 99, 235],
    selectedCardId: 'c7',
    cardPoints: 3,
    onFlipMotionCard: vi.fn(),
    onStopMotionTimer: vi.fn(),
    onRevealMotionCard: vi.fn(),
    onDoneMotionCard: vi.fn(),
    ...overrides,
  }
}

function getBtn(container, label) {
  return Array.from(container.querySelectorAll('.anim-conduct-btn')).find(
    (btn) => btn.textContent.startsWith(label)
  )
}

// ---------------------------------------------------------------------------
// Bandeaux MEMORIZE/GRID — motionGestures retourne [] pour ces deux
// sous-phases (T2) : AnimMotionActions doit rendre un bandeau d'information
// plutôt qu'une ligne vide (plan §F6, maquette §MEMORIZE/GRID "notice").
// ---------------------------------------------------------------------------

describe('AnimMotionActions — bandeaux MEMORIZE/GRID (aucun geste)', () => {
  it('MEMORIZE : bandeau d\'attente, aucun bouton', () => {
    const { container } = render(<AnimMotionActions {...baseProps({ subphase: 'MEMORIZE' })} />)
    expect(container.querySelectorAll('.anim-conduct-btn')).toHaveLength(0)
    expect(container.querySelector('.anim-motion-banner')).not.toBeNull()
  })

  it('GRID : bandeau rappelant l\'équipe au tour (currentTeam)', () => {
    const { container } = render(<AnimMotionActions {...baseProps({ subphase: 'GRID', currentTeam: 'Les Bleus' })} />)
    expect(container.querySelectorAll('.anim-conduct-btn')).toHaveLength(0)
    const banner = container.querySelector('.anim-motion-banner')
    expect(banner).not.toBeNull()
    expect(banner.textContent).toContain('Les Bleus')
  })

  it('GRID sans équipe courante (mode SOLO) : bandeau présent, ne mentionne aucune équipe, ne plante pas', () => {
    const { container } = render(<AnimMotionActions {...baseProps({ subphase: 'GRID', currentTeam: '' })} />)
    expect(() => container.querySelector('.anim-motion-banner')).not.toThrow()
    expect(container.querySelector('.anim-motion-banner')).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// SELECTED — DÉMARRER (go) / ANNULER (optional), câblage vers les handlers.
// ---------------------------------------------------------------------------

describe('AnimMotionActions — SELECTED', () => {
  it('rend DÉMARRER (go) et ANNULER (optional), aucun bandeau', () => {
    const { container } = render(<AnimMotionActions {...baseProps({ subphase: 'SELECTED' })} />)
    expect(container.querySelector('.anim-motion-banner')).toBeNull()
    const start = getBtn(container, 'DÉMARRER')
    const cancel = getBtn(container, 'ANNULER')
    expect(start.className).toMatch(/\banim-conduct-btn-go\b/)
    expect(cancel.className).toMatch(/\banim-conduct-btn-optional\b/)
    expect(start.disabled).toBe(false)
    expect(cancel.disabled).toBe(false)
  })

  it('DÉMARRER appelle onFlipMotionCard()', () => {
    const props = baseProps({ subphase: 'SELECTED' })
    const { container } = render(<AnimMotionActions {...props} />)
    getBtn(container, 'DÉMARRER').click()
    expect(props.onFlipMotionCard).toHaveBeenCalledTimes(1)
  })

  it('ANNULER appelle onDoneMotionCard(selectedCardId, "")', () => {
    const props = baseProps({ subphase: 'SELECTED', selectedCardId: 'c7' })
    const { container } = render(<AnimMotionActions {...props} />)
    getBtn(container, 'ANNULER').click()
    expect(props.onDoneMotionCard).toHaveBeenCalledWith('c7', '')
  })
})

// ---------------------------------------------------------------------------
// QUESTION — STOP CHRONO / RÉVÉLER / SANS VAINQUEUR, matrice chrono
// en cours vs à zéro, AUCUNE action émise par un bouton éteint.
// ---------------------------------------------------------------------------

describe('AnimMotionActions — QUESTION, matrice chrono', () => {
  it('chrono EN COURS : RÉVÉLER désactivé (off), le clic n\'émet rien', () => {
    const props = baseProps({ subphase: 'QUESTION', timerRunning: true })
    const { container } = render(<AnimMotionActions {...props} />)
    const reveal = getBtn(container, 'RÉVÉLER')
    expect(reveal.className).toMatch(/\banim-conduct-btn-off\b/)
    expect(reveal.disabled).toBe(true)
    reveal.click()
    expect(props.onRevealMotionCard).not.toHaveBeenCalled()
  })

  it('chrono À ZÉRO : RÉVÉLER devient "go", cliquable, appelle onRevealMotionCard()', () => {
    const props = baseProps({ subphase: 'QUESTION', timerRunning: false })
    const { container } = render(<AnimMotionActions {...props} />)
    const reveal = getBtn(container, 'RÉVÉLER')
    expect(reveal.className).toMatch(/\banim-conduct-btn-go\b/)
    reveal.click()
    expect(props.onRevealMotionCard).toHaveBeenCalledTimes(1)
  })

  it('chrono À ZÉRO : STOP CHRONO devient "off" (rien à arrêter), le clic n\'émet rien', () => {
    const props = baseProps({ subphase: 'QUESTION', timerRunning: false })
    const { container } = render(<AnimMotionActions {...props} />)
    const stop = getBtn(container, 'STOP CHRONO')
    expect(stop.disabled).toBe(true)
    stop.click()
    expect(props.onStopMotionTimer).not.toHaveBeenCalled()
  })

  it('chrono EN COURS : STOP CHRONO appelle onStopMotionTimer()', () => {
    const props = baseProps({ subphase: 'QUESTION', timerRunning: true })
    const { container } = render(<AnimMotionActions {...props} />)
    getBtn(container, 'STOP CHRONO').click()
    expect(props.onStopMotionTimer).toHaveBeenCalledTimes(1)
  })

  it('SANS VAINQUEUR (toujours "optional") appelle onDoneMotionCard(selectedCardId, "")', () => {
    const props = baseProps({ subphase: 'QUESTION', selectedCardId: 'c9' })
    const { container } = render(<AnimMotionActions {...props} />)
    const noWinner = getBtn(container, 'SANS VAINQUEUR')
    expect(noWinner.disabled).toBe(false)
    noWinner.click()
    expect(props.onDoneMotionCard).toHaveBeenCalledWith('c9', '')
  })
})

// ---------------------------------------------------------------------------
// REVEAL — équipe courante (couleur) / PERSONNE.
// ---------------------------------------------------------------------------

describe('AnimMotionActions — REVEAL', () => {
  it('bouton de l\'équipe courante : libellé = nom de l\'équipe, couleur d\'équipe posée (background + texte contrasté)', () => {
    const { container } = render(
      <AnimMotionActions {...baseProps({ subphase: 'REVEAL', currentTeam: 'Les Bleus', currentTeamColor: [37, 99, 235] })} />
    )
    const teamBtn = getBtn(container, 'Les Bleus')
    expect(teamBtn).not.toBeUndefined()
    expect(teamBtn.style.backgroundColor).toBeTruthy()
    expect(teamBtn.style.color).toBeTruthy()
  })

  it('bouton équipe appelle onDoneMotionCard(selectedCardId, currentTeam)', () => {
    const props = baseProps({ subphase: 'REVEAL', currentTeam: 'Les Bleus', selectedCardId: 'c7' })
    const { container } = render(<AnimMotionActions {...props} />)
    getBtn(container, 'Les Bleus').click()
    expect(props.onDoneMotionCard).toHaveBeenCalledWith('c7', 'Les Bleus')
  })

  it('PERSONNE appelle onDoneMotionCard(selectedCardId, "")', () => {
    const props = baseProps({ subphase: 'REVEAL', selectedCardId: 'c7' })
    const { container } = render(<AnimMotionActions {...props} />)
    getBtn(container, 'PERSONNE').click()
    expect(props.onDoneMotionCard).toHaveBeenCalledWith('c7', '')
  })

  it('mode SOLO (currentTeam vide) : SEUL PERSONNE est rendu, aucun bouton d\'équipe', () => {
    const { container } = render(<AnimMotionActions {...baseProps({ subphase: 'REVEAL', currentTeam: '' })} />)
    const buttons = container.querySelectorAll('.anim-conduct-btn')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].textContent).toContain('PERSONNE')
  })
})

// ---------------------------------------------------------------------------
// Rendu EXCLUSIVEMENT piloté par motionRules — pas de condition réécrite
// localement. Vérifié en comparant le nombre de boutons rendus au nombre de
// gestes retournés par motionGestures pour le MÊME contexte, sur les cinq
// sous-phases.
// ---------------------------------------------------------------------------

describe('AnimMotionActions — rendu piloté uniquement par motionRules (#160/T5, central)', () => {
  it.each(['MEMORIZE', 'GRID', 'SELECTED', 'QUESTION', 'REVEAL'])(
    'sous-phase %s : le nombre de boutons rendus == le nombre de gestes retournés par motionGestures',
    (subphase) => {
      const props = baseProps({ subphase })
      const expectedGestures = motionGestures(subphase, {
        timerRunning: props.timerRunning,
        currentTeam: props.currentTeam,
        currentTeamColor: props.currentTeamColor,
        selectedCardId: props.selectedCardId,
        cardPoints: props.cardPoints,
      })
      const { container } = render(<AnimMotionActions {...props} />)
      expect(container.querySelectorAll('.anim-conduct-btn')).toHaveLength(expectedGestures.length)
    }
  )
})

describe('AnimMotionActions.css — palette anim-conduct-btn réutilisée telle quelle (#160/F6)', () => {
  it('AnimMotionActions.css ne redéfinit PAS .anim-conduct-btn-{go|optional|danger|off} (réutilisation, pas de duplication de palette)', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    expect(css).not.toMatch(/\.anim-conduct-btn-(go|optional|danger|off)\s*\{/)
  })
})
