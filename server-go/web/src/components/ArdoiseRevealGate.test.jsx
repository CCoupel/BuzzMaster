import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import ArdoiseRevealGate, { ArdoiseReceiptIcon, ArdoiseReceiptCounter } from './ArdoiseRevealGate'
import AnimArdoiseList from './AnimArdoiseList'
import { getArdoiseReceipt } from '../utils/ardoiseReceipt'

// #246 (F-STOP) — garde de REVEAL ARDOISE : motif, forçage confirmé, pastilles, compteur.

const pending = [{ team: 'Les Verts', offline: true }, { team: 'Les Oranges', offline: false }]

describe('ArdoiseRevealGate', () => {
  it('rien à afficher sans équipe en attente', () => {
    const { container } = render(<ArdoiseRevealGate pending={[]} onForce={vi.fn()} />)
    expect(container.textContent).toBe('')
  })

  it.each(['admin', 'anim'])('variante %s : motif listant les équipes, hors ligne signalée', (variant) => {
    const { container } = render(<ArdoiseRevealGate pending={pending} variant={variant} onForce={vi.fn()} />)
    const t = container.textContent.replace(/\s+/g, ' ')
    expect(t).toMatch(/En attente (des réponses )?de 2 tablettes/)
    expect(t).toMatch(/Les Verts[^]*hors ligne/)
    expect(t).toMatch(/Les Oranges/)
  })

  it('« Révéler quand même… » ne force jamais directement : confirmation, « Attendre » annule, confirmer force', () => {
    const onForce = vi.fn()
    render(<ArdoiseRevealGate pending={pending} onForce={onForce} />)
    fireEvent.click(screen.getByText('Révéler quand même…'))
    expect(onForce).not.toHaveBeenCalled()
    const dlg = screen.getByRole('dialog')
    expect(dlg.textContent).toMatch(/Révéler sans toutes les réponses \?/)
    // #246 : plus de « marquées non confirmée » — le texte grisé et l'icône d'origine le disent
    expect(dlg.textContent).toMatch(/garderont leur dernier texte reçu/)
    expect(dlg.textContent).toMatch(/grisé avec son icône d’origine/)
    expect(dlg.textContent).not.toMatch(/non confirmée/)
    fireEvent.click(within(dlg).getByText('Attendre'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onForce).not.toHaveBeenCalled()

    fireEvent.click(screen.getByText('Révéler quand même…'))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Révéler quand même' }))
    expect(onForce).toHaveBeenCalledTimes(1)
  })

  it('icônes de réception (title + aria-label) et compteur', () => {
    const { container } = render(
      <>
        <ArdoiseReceiptIcon status="ok" />
        <ArdoiseReceiptIcon status="wait" />
        <ArdoiseReceiptIcon status="off" />
        <ArdoiseReceiptIcon status="wait" revealed />
        <ArdoiseReceiptIcon />
        <ArdoiseReceiptCounter receipt={{ complete: false, receivedCount: 7, total: 10 }} />
        <ArdoiseReceiptCounter receipt={{ complete: true, receivedCount: 10, total: 10 }} />
        <ArdoiseReceiptCounter receipt={{ complete: false, receivedCount: 0, total: 10 }} live />
      </>
    )
    const icons = [...container.querySelectorAll('.ardoise-receipt-icon')]
    expect(icons).toHaveLength(5)
    expect(icons.map((i) => i.textContent)).toEqual(['✓', '⏳', '⚠', '⏳', ''])
    expect(icons[0].getAttribute('aria-label')).toBe('Réponse reçue')
    expect(icons[0].getAttribute('title')).toBe('Réponse reçue')
    expect(icons[2].getAttribute('aria-label')).toMatch(/hors ligne/)
    expect(icons[3].className).toMatch(/unconfirmed/)
    expect(icons[3].getAttribute('aria-label')).toMatch(/Non confirmée/)
    expect(icons[4].getAttribute('aria-hidden')).toBe('true') // colonne réservée, vide
    const t = container.textContent
    for (const s of ['7/10 reçues', '10/10 reçues', 'Saisie en cours']) expect(t).toContain(s)
  })
})

describe('AnimArdoiseList — réception (#246)', () => {
  const team = { COLOR: [0, 0, 255] }
  const entries = [
    { team, teamName: 'Les Bleus', answer: { TEXT: 'PARIS', STARTED_AT: 1, SUBMITTED_AT: 2 } },
    { team, teamName: 'Les Verts', answer: { TEXT: 'LYON', STARTED_AT: 1, SUBMITTED_AT: 3 } },
  ]
  const teams = { 'Les Bleus': {}, 'Les Verts': {} }
  const bumpers = { a: { IS_VPLAYER: true, TEAM: 'Les Bleus', CONNECTED: true }, b: { IS_VPLAYER: true, TEAM: 'Les Verts', CONNECTED: false } }
  const render1 = (phase, revealed = false) => render(
    <AnimArdoiseList
      entries={entries} question={{ TYPE: 'ARDOISE', POINTS: '2' }} gameTime={0} creditPoints={2}
      revealed={revealed} awardedTeams={{}} onCredit={vi.fn()} phase={phase}
      receipt={getArdoiseReceipt(teams, bumpers, { 'Les Bleus': { RECEIVED: true } })}
    />
  )

  it('après STOP : 1/2 reçues, pastilles reçue / hors ligne', () => {
    const { container } = render1('STOPPED')
    expect(container.textContent).toContain('1/2 reçues')
    const labels = [...container.querySelectorAll('.ardoise-receipt-icon')].map((i) => i.getAttribute('aria-label'))
    expect(labels[0]).toBe('Réponse reçue')
    expect(labels[1]).toMatch(/hors ligne/)
  })

  it('pendant la question : « Saisie en cours », aucune pastille', () => {
    const t = render1('STARTED').container.textContent
    expect(t).toContain('Saisie en cours')
    expect(t).not.toMatch(/hors ligne/) // légende masquée
    expect([...render1('STARTED').container.querySelectorAll('.ardoise-receipt-icon[role=img]')]).toHaveLength(0)
  })

  it('REVEALED : « non confirmée » pour l\'équipe non reçue seulement', () => {
    const { container } = render1('REVEALED', true)
    const labels = [...container.querySelectorAll('.ardoise-receipt-icon[role=img]')].map((i) => i.getAttribute('aria-label'))
    expect(labels.filter((l) => /Non confirmée/.test(l))).toHaveLength(1)
    expect(labels).toContain('Réponse reçue')
  })
})
