import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimAnswerZone from './AnimAnswerZone'

// ---------------------------------------------------------------------------
// AnimAnswerZone — zone réponse permanente de /anim (#166/F10, T7 ; révision
// #169 — révélation par pression tactile).
//
// Plan #166 : _work/reports/plan-20260815-144925.md, tâche F10. Maquette :
// https://claude.ai/code/artifact/49cb60ae-8c6a-46f6-9268-5b0a6b5eb385
// (états "SPEEDY READY", "QCM STARTED", "QCM REVEALED").
//
// Remplace le bloc conditionnel #163/F4 (ANSWER affiché uniquement en
// REVEALED) : la zone est désormais TOUJOURS rendue dès qu'une question est
// chargée, quelle que soit la phase — seul le style change entre flouté et
// net (classe `masked`/`revealed`), jamais la présence ni le contenu. Le
// point R5 du plan ("aucun décalage visuel au reveal") est vérifié ici au
// niveau structure DOM (même contenu, même éléments, seule la classe
// modificatrice change) — la mesure de pixels réelle relève de T12 (QA
// manuelle), jsdom ne fait pas de layout.
//
// #169 (révision, coordination dev-frontend) : le flou permanent est
// remplacé par un geste actif AVANT `revealed` — appui maintenu
// (pointerdown/touch) révèle temporairement, relâcher remasque. Après
// `revealed=true`, comportement #166 inchangé (visible en permanence, sans
// interaction). Implémentation : état interne `peeking`, `visible = revealed
// || peeking`, MÊME classe `revealed`/`masked` que #166 (réutilisée, reflète
// désormais `visible` et non plus la seule prop `revealed`) + nouvelle
// classe `.anim-answer-zone-peekable` uniquement quand `!revealed` (indique
// l'interaction active, absente une fois révélé en permanence). Les tests
// T7 ci-dessus n'ont PAS été modifiés : aucun ne simule d'événement pointer,
// `peeking` y reste toujours `false`, donc `visible === revealed` exactement
// comme avant #169 — comportement légitimement inchangé pour ces scénarios.
// ---------------------------------------------------------------------------

describe('AnimAnswerZone — présence et garde de phase', () => {
  it('ne rend rien quand aucune question n\'est chargée (jamais un cadre flouté sans contenu)', () => {
    const { container } = render(<AnimAnswerZone question={null} revealed={false} />)
    expect(container.firstChild).toBeNull()
  })

  it.each([false, true])(
    'est présente dès qu\'une question est chargée, quel que soit revealed=%s',
    (revealed) => {
      const { container } = render(
        <AnimAnswerZone question={{ ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }} revealed={revealed} />
      )
      expect(container.querySelector('.anim-answer-zone')).not.toBeNull()
      expect(screen.getByText('Réponse')).toBeInTheDocument()
    }
  )

  it('classe "masked" hors REVEALED, "revealed" en REVEALED — mêmes éléments dans les deux cas (pas de décalage, R5)', () => {
    const question = { ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }
    const { container, rerender } = render(<AnimAnswerZone question={question} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    expect(zone.className).toMatch(/\bmasked\b/)
    expect(zone.className).not.toMatch(/\brevealed\b/)
    const maskedChildCount = zone.children.length

    rerender(<AnimAnswerZone question={question} revealed={true} />)
    const zoneRevealed = container.querySelector('.anim-answer-zone')
    expect(zoneRevealed.className).toMatch(/\brevealed\b/)
    expect(zoneRevealed.className).not.toMatch(/\bmasked\b/)
    // Même structure (label + value, pas de badge hors QCM) dans les deux états.
    expect(zoneRevealed.children.length).toBe(maskedChildCount)
    expect(screen.getByText('800')).toBeInTheDocument()
  })
})

describe('AnimAnswerZone — hors QCM', () => {
  it('affiche question.ANSWER', () => {
    render(<AnimAnswerZone question={{ ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }} revealed={true} />)
    expect(screen.getByText('800')).toBeInTheDocument()
  })

  it('ANSWER vide affiche un tiret ("—"), jamais un flou sur du vide', () => {
    render(<AnimAnswerZone question={{ ID: '1', TYPE: 'SPEEDY', ANSWER: '' }} revealed={false} />)
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('ANSWER absente (ARDOISE/MEMORY/MEMOTION sans champ) affiche un tiret', () => {
    render(<AnimAnswerZone question={{ ID: '1', TYPE: 'ARDOISE' }} revealed={false} />)
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('ne rend aucune pastille couleur hors QCM', () => {
    const { container } = render(
      <AnimAnswerZone question={{ ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }} revealed={true} />
    )
    expect(container.querySelector('.anim-answer-zone-badge')).toBeNull()
  })
})

describe('AnimAnswerZone — QCM', () => {
  const QCM_QUESTION = {
    ID: '1',
    TYPE: 'QCM',
    QCM_ANSWERS: { RED: 'Sydney', GREEN: 'Canberra', YELLOW: 'Melbourne', BLUE: 'Perth' },
    QCM_CORRECT: 'GREEN',
  }

  it('affiche la pastille couleur (lettre + couleur QCM_COLORS) et le libellé de la bonne proposition', () => {
    const { container } = render(<AnimAnswerZone question={QCM_QUESTION} revealed={true} />)
    const badge = container.querySelector('.anim-answer-zone-badge')
    expect(badge).not.toBeNull()
    expect(badge.textContent).toBe('B') // GREEN -> lettre B, QCM_COLORS
    expect(badge.style.backgroundColor).toBe('rgb(34, 197, 94)') // #22c55e
    expect(screen.getByText('Canberra')).toBeInTheDocument()
  })

  it('QCM_CORRECT absente/inconnue : pas de pastille, valeur en tiret', () => {
    const { container } = render(
      <AnimAnswerZone question={{ ID: '1', TYPE: 'QCM', QCM_ANSWERS: { RED: 'a' }, QCM_CORRECT: null }} revealed={true} />
    )
    expect(container.querySelector('.anim-answer-zone-badge')).toBeNull()
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('reste flouté (masked) avant REVEALED même si la bonne réponse est connue (le payload la porte dès le chargement — R3/#163)', () => {
    const { container } = render(<AnimAnswerZone question={QCM_QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    expect(zone.className).toMatch(/\bmasked\b/)
    // Le contenu est bien présent (pas absent) — c'est le flou CSS qui le
    // cache visuellement, pas une garde de rendu conditionnelle.
    expect(screen.getByText('Canberra')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #169 — révélation par pression tactile, AVANT REVEALED
// ---------------------------------------------------------------------------

describe('AnimAnswerZone — #169, appui maintenu avant REVEALED', () => {
  const QUESTION = { ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }

  it('masquée par défaut (masked), classe "peekable" présente, aucune interaction en cours', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    expect(zone.className).toMatch(/\bmasked\b/)
    expect(zone.className).toMatch(/\banim-answer-zone-peekable\b/)
  })

  it('pointerdown révèle temporairement (classe passe à "revealed")', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
    expect(zone.className).not.toMatch(/\bmasked\b/)
    expect(screen.getByText('800')).toBeInTheDocument()
  })

  it('pointerup remasque après un appui', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
    fireEvent.pointerUp(zone)
    expect(zone.className).toMatch(/\bmasked\b/)
    expect(zone.className).not.toMatch(/\brevealed\b/)
  })

  it('pointerleave (doigt qui glisse hors de la zone) remasque, comme pointerup', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    fireEvent.pointerLeave(zone)
    expect(zone.className).toMatch(/\bmasked\b/)
  })

  it('pointercancel (interruption système du geste) remasque, comme pointerup', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    fireEvent.pointerCancel(zone)
    expect(zone.className).toMatch(/\bmasked\b/)
  })

  it('un nouvel appui après relâchement re-révèle (pas un bascule figé à sens unique)', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    fireEvent.pointerUp(zone)
    fireEvent.pointerDown(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
  })

  it('fonctionne identiquement pour une question QCM (pastille + libellé révélés le temps de l\'appui)', () => {
    const qcm = { ID: '1', TYPE: 'QCM', QCM_ANSWERS: { RED: 'a', GREEN: 'Canberra' }, QCM_CORRECT: 'GREEN' }
    const { container } = render(<AnimAnswerZone question={qcm} revealed={false} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
    expect(screen.getByText('Canberra')).toBeInTheDocument()
  })
})

describe('AnimAnswerZone — #169, aucune régression une fois REVEALED (comportement #166 permanent)', () => {
  const QUESTION = { ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }

  it('visible en permanence dès revealed=true, sans le moindre appui', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={true} />)
    const zone = container.querySelector('.anim-answer-zone')
    expect(zone.className).toMatch(/\brevealed\b/)
  })

  it('pas de classe "peekable" une fois revealed=true (plus d\'interaction active nécessaire)', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={true} />)
    const zone = container.querySelector('.anim-answer-zone')
    expect(zone.className).not.toMatch(/\banim-answer-zone-peekable\b/)
  })

  it('pointerdown puis pointerup n\'ont aucun effet — reste "revealed" tout du long (pas de flicker au clic après le reveal)', () => {
    const { container } = render(<AnimAnswerZone question={QUESTION} revealed={true} />)
    const zone = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
    fireEvent.pointerUp(zone)
    expect(zone.className).toMatch(/\brevealed\b/)
    expect(zone.className).not.toMatch(/\bmasked\b/)
  })
})

describe('AnimAnswerZone — #169, remise à zéro au changement de question', () => {
  it('un appui maintenu ne "fuit" pas sur la question suivante (peeking réinitialisé au changement d\'ID)', () => {
    const q1 = { ID: '1', TYPE: 'SPEEDY', ANSWER: '800' }
    const q2 = { ID: '2', TYPE: 'SPEEDY', ANSWER: '1515' }
    const { container, rerender } = render(<AnimAnswerZone question={q1} revealed={false} />)
    const zone1 = container.querySelector('.anim-answer-zone')
    fireEvent.pointerDown(zone1) // doigt maintenu, pointerup jamais reçu (cas limite : événement manqué)

    rerender(<AnimAnswerZone question={q2} revealed={false} />)
    const zone2 = container.querySelector('.anim-answer-zone')
    expect(zone2.className).toMatch(/\bmasked\b/)
    expect(zone2.className).not.toMatch(/\brevealed\b/)
  })
})
