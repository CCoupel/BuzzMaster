import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import ArdoiseRevealGate, { ArdoiseReceiptChip, ArdoiseReceiptCounter } from './ArdoiseRevealGate'
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
    fireEvent.click(within(dlg).getByText('Attendre'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onForce).not.toHaveBeenCalled()

    fireEvent.click(screen.getByText('Révéler quand même…'))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Révéler quand même' }))
    expect(onForce).toHaveBeenCalledTimes(1)
  })

  it('pastilles et compteur', () => {
    const { container } = render(
      <>
        <ArdoiseReceiptChip status="ok" />
        <ArdoiseReceiptChip status="wait" />
        <ArdoiseReceiptChip status="off" />
        <ArdoiseReceiptChip status="wait" revealed />
        <ArdoiseReceiptCounter receipt={{ complete: false, receivedCount: 7, total: 10 }} />
        <ArdoiseReceiptCounter receipt={{ complete: true, receivedCount: 10, total: 10 }} />
        <ArdoiseReceiptCounter receipt={{ complete: false, receivedCount: 0, total: 10 }} live />
      </>
    )
    const t = container.textContent
    for (const s of ['✓ reçue', '⏳ en attente', '⚠ hors ligne', 'non confirmée', '7/10 reçues', '10/10 reçues', 'Saisie en cours']) {
      expect(t).toContain(s)
    }
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
    const t = render1('STOPPED').container.textContent
    expect(t).toContain('1/2 reçues')
    expect(t).toContain('✓ reçue')
    expect(t).toContain('⚠ hors ligne')
  })

  it('pendant la question : « Saisie en cours », aucune pastille', () => {
    const t = render1('STARTED').container.textContent
    expect(t).toContain('Saisie en cours')
    expect(t).not.toMatch(/✓ reçue|hors ligne/)
  })

  it('REVEALED : « non confirmée » pour l\'équipe non reçue seulement', () => {
    const t = render1('REVEALED', true).container.textContent
    expect(t.match(/non confirmée/g)).toHaveLength(1)
    expect(t).toContain('✓ reçue')
  })
})
