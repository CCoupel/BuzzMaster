import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import RegieMessageBar from './RegieMessageBar'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// RegieMessageBar — bandeau d'envoi régie → animateurs (v6.4.x, #167).
//
// Plan : _work/reports/plan-20260818-121500.md, tâches F2/F2b/F3b. Contrat :
// contracts/websocket-actions.md §"Messagerie régie". Maquette :
// docs/mockups/anim-communication-167-168.html (bandeau /admin, 4 états).
//
// T9 — 4 états, AUCUN bouton « Envoyer ».
// T9b — envoi automatique (Entrée/blur/pause de frappe 2s), cycle de vie du
//       timer, gardes anti-doublon/vide, vidage du champ à l'acquittement.
// T9c — affichage piloté EXCLUSIVEMENT par l'état WebSocket (regieMessage),
//       jamais par un état local optimiste.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('./RegieMessageBar.css', () => ({}))

const IDLE = { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }

function makeGameMock(overrides = {}) {
  return {
    regieMessage: IDLE,
    sendRegieMessage: vi.fn(),
    clearRegieMessage: vi.fn(),
    ...overrides,
  }
}

function getInput() {
  return screen.getByLabelText('Consigne à envoyer aux tablettes animateur')
}

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
})

afterEach(() => {
  vi.clearAllMocks()
  vi.useRealTimers()
})

// ---------------------------------------------------------------------------
// T9 — 4 états de la maquette, aucun bouton « Envoyer »
// ---------------------------------------------------------------------------

describe('RegieMessageBar — état repos', () => {
  it('affiche le champ de saisie et le compteur à 140 (aucun texte tapé)', () => {
    render(<RegieMessageBar />)
    expect(getInput()).toBeInTheDocument()
    expect(getInput()).toHaveValue('')
    expect(screen.getByText('140')).toBeInTheDocument()
  })

  it("n'affiche ni « Effacer » ni « Vu par l'animateur »", () => {
    render(<RegieMessageBar />)
    expect(screen.queryByText('Effacer')).toBeNull()
    expect(screen.queryByText(/vu par l'animateur/i)).toBeNull()
  })
})

describe('RegieMessageBar — état saisie', () => {
  it('le compteur décroît avec la longueur du texte tapé (140 - longueur)', () => {
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Question 12 annulée' } }) // 19 caractères
    expect(screen.getByText('121')).toBeInTheDocument()
  })

  it('maxLength=140 posé sur le champ (commodité — la troncature fait autorité côté serveur)', () => {
    render(<RegieMessageBar />)
    expect(getInput()).toHaveAttribute('maxLength', '140')
  })
})

describe('RegieMessageBar — état message actif', () => {
  it('affiche le texte du message actif et le bouton « Effacer », plus de champ de saisie', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Question 12 annulée', SENT_AT: 1000, CLEARED_BY: '' },
    }))
    render(<RegieMessageBar />)
    expect(screen.getByText('« Question 12 annulée »')).toBeInTheDocument()
    expect(screen.getByText('Effacer')).toBeInTheDocument()
    expect(screen.queryByLabelText('Consigne à envoyer aux tablettes animateur')).toBeNull()
  })

  it('« Effacer » appelle clearRegieMessage (retrait régie, D4)', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1000, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.click(screen.getByText('Effacer'))
    expect(props.clearRegieMessage).toHaveBeenCalledTimes(1)
  })
})

describe('RegieMessageBar — état acquitté', () => {
  it('affiche « Vu par l\'animateur » et le bouton « Nouveau message » (CLEARED_BY=ANIM)', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    render(<RegieMessageBar />)
    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()
    expect(screen.getByText('Nouveau message')).toBeInTheDocument()
  })

  it('CLEARED_BY=REGIE (retrait régie) repasse au repos, PAS à l\'état acquitté', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'REGIE' },
    }))
    render(<RegieMessageBar />)
    expect(screen.queryByText("Vu par l'animateur")).toBeNull()
    expect(getInput()).toBeInTheDocument()
  })

  it('« Nouveau message » bascule vers le champ de saisie (repos)', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    render(<RegieMessageBar />)
    fireEvent.click(screen.getByText('Nouveau message'))
    expect(getInput()).toBeInTheDocument()
    expect(screen.queryByText("Vu par l'animateur")).toBeNull()
  })
})

describe('RegieMessageBar — aucun bouton « Envoyer », dans aucun état (AC1c)', () => {
  it.each([
    ['repos', IDLE],
    ['actif', { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' }],
    ['acquitté', { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' }],
  ])('état %s : pas de bouton "Envoyer"', (_, regieMessage) => {
    useGame.mockReturnValue(makeGameMock({ regieMessage }))
    render(<RegieMessageBar />)
    const sendButtons = screen.queryAllByRole('button', { name: /envoyer/i })
    expect(sendButtons).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// T9b — envoi automatique, trois déclencheurs simultanés (F2b)
// ---------------------------------------------------------------------------

describe('RegieMessageBar — envoi automatique (F2b)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('touche Entrée envoie immédiatement le texte trimmé', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: '  Pause technique  ' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })
    expect(props.sendRegieMessage).toHaveBeenCalledTimes(1)
    expect(props.sendRegieMessage).toHaveBeenCalledWith('Pause technique')
  })

  it('perte de focus (blur) envoie le texte', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne au blur' } })
    fireEvent.blur(getInput())
    expect(props.sendRegieMessage).toHaveBeenCalledWith('Consigne au blur')
  })

  it('pause de frappe de 2000ms envoie le texte (sans Entrée ni blur)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne par pause' } })
    expect(props.sendRegieMessage).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(1999) })
    expect(props.sendRegieMessage).not.toHaveBeenCalled()
    act(() => { vi.advanceTimersByTime(1) })
    expect(props.sendRegieMessage).toHaveBeenCalledWith('Consigne par pause')
  })

  it('une nouvelle frappe réarme le délai de 2000ms (pas d\'envoi tant que la frappe continue)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Cons' } })
    act(() => { vi.advanceTimersByTime(1500) })
    fireEvent.change(getInput(), { target: { value: 'Consigne' } }) // réarme le timer
    act(() => { vi.advanceTimersByTime(1500) })
    expect(props.sendRegieMessage).not.toHaveBeenCalled() // 1500 depuis la dernière frappe seulement
    act(() => { vi.advanceTimersByTime(500) })
    expect(props.sendRegieMessage).toHaveBeenCalledWith('Consigne')
  })

  it('Entrée annule le timer de pause en cours (un seul envoi, pas deux)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })
    act(() => { vi.advanceTimersByTime(2000) }) // le timer, s'il n'était pas annulé, enverrait un doublon
    expect(props.sendRegieMessage).toHaveBeenCalledTimes(1)
  })

  it('blur annule le timer de pause en cours (un seul envoi, pas deux)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne' } })
    fireEvent.blur(getInput())
    act(() => { vi.advanceTimersByTime(2000) })
    expect(props.sendRegieMessage).toHaveBeenCalledTimes(1)
  })

  it('texte vide ou uniquement des espaces : Entrée/blur/pause n\'envoient rien', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: '   ' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })
    fireEvent.blur(getInput())
    act(() => { vi.advanceTimersByTime(2000) })
    expect(props.sendRegieMessage).not.toHaveBeenCalled()
  })

  it('un blur juste après l\'Entrée qui a envoyé le MÊME texte ne renvoie pas (garde anti-doublon cliente)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne stable' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })
    expect(props.sendRegieMessage).toHaveBeenCalledTimes(1)
    // Le champ garde le même contenu (pas remis à zéro : le message est
    // toujours actif tant que non acquitté) ; un clic ailleurs déclenche blur.
    fireEvent.blur(getInput())
    expect(props.sendRegieMessage).toHaveBeenCalledTimes(1) // toujours 1, pas 2
  })

  it('timer nettoyé au démontage (aucun envoi après unmount)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    const { unmount } = render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Consigne' } })
    unmount()
    act(() => { vi.advanceTimersByTime(5000) })
    expect(props.sendRegieMessage).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// F2b — vidage du champ à l'acquittement, seulement si le contenu n'a pas
// divergé entre-temps
// ---------------------------------------------------------------------------

describe('RegieMessageBar — vidage du champ à l\'acquittement (F2b)', () => {
  it('le texte qu\'on vient de faire acquitter (encore présent en local) est vidé', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    // La régie tape et envoie (Entrée) — localText contient toujours
    // 'Consigne' même une fois la vue basculée sur l'état "actif" (l'input
    // disparaît du DOM mais l'état React persiste).
    fireEvent.change(getInput(), { target: { value: 'Consigne' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    expect(screen.getByText('« Consigne »')).toBeInTheDocument()

    // L'animateur acquitte.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    fireEvent.click(screen.getByText('Nouveau message'))
    expect(getInput()).toHaveValue('') // le texte acquitté a été vidé
  })

  it('un brouillon qui DIVERGE du message acquitté (envoyé entre-temps par un autre poste) n\'est PAS détruit', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    // La régie commence à composer "Y" — sans l'envoyer (aucun déclencheur).
    fireEvent.change(getInput(), { target: { value: 'Y' } })

    // Pendant ce temps, un AUTRE poste /admin envoie "X" — le message actif
    // partagé n'est PAS le brouillon de cette session.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'X', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    // Puis acquitté.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    fireEvent.click(screen.getByText('Nouveau message'))
    // Le brouillon "Y" de cette session doit survivre — il ne correspond pas
    // au texte "X" qui vient d'être acquitté.
    expect(getInput()).toHaveValue('Y')
  })
})

// ---------------------------------------------------------------------------
// T9c — affichage piloté EXCLUSIVEMENT par l'état WebSocket (F3b) : une mise
// à jour REGIE_MESSAGE venue d'une AUTRE session doit changer l'affichage
// SANS que ce composant ait rien envoyé lui-même.
// ---------------------------------------------------------------------------

describe('RegieMessageBar — affichage piloté par regieMessage, jamais un état local (T9c, F3b)', () => {
  it('un second poste régie qui tape voit son propre message actif refléter regieMessage — sans appeler sendRegieMessage', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    expect(getInput()).toBeInTheDocument() // repos

    // Un AUTRE poste /admin envoie un message : le hook WS de CE poste reçoit
    // la diffusion REGIE_MESSAGE et regieMessage change — sans qu'aucune
    // action locale (frappe, Entrée, blur) n'ait eu lieu ici.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Envoyé par l\'autre poste', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    expect(screen.getByText("« Envoyé par l'autre poste »")).toBeInTheDocument()
    expect(props.sendRegieMessage).not.toHaveBeenCalled()
  })

  it('un animateur qui acquitte depuis sa tablette fait passer CE poste régie à "Vu par l\'animateur" sans action locale', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    expect(screen.getByText('« Consigne »')).toBeInTheDocument()

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()
    expect(props.clearRegieMessage).not.toHaveBeenCalled()
  })
})
