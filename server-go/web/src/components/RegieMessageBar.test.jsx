import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import RegieMessageBar from './RegieMessageBar'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// RegieMessageBar — bandeau d'envoi régie → animateurs (v6.4.x, #167, révisé
// #176).
//
// Plan #167 : _work/reports/plan-20260818-121500.md, tâches F2/F2b/F3b.
// Plan #176 (correctifs UX) : _work/reports/plan-20260818-141638.md, tâches
// F1/F2/F3 — REMPLACE le modèle à 4 états exclusifs (repos/saisie/actif/
// acquitté avec bouton « Nouveau message ») par UNE structure unique où le
// champ de saisie est TOUJOURS visible et éditable, pré-rempli avec le
// message actif, avec un indicateur d'acquittement FUGACE (~4s) au lieu
// d'un état bloquant. Contrat : contracts/websocket-actions.md §"Messagerie
// régie" (sémantique serveur inchangée par #176 — seul le geste/rendu
// côté client change).
//
// T9/T1 (#176) — champ toujours présent, AUCUN bouton « Envoyer », AUCUN
//                bouton « Nouveau message ».
// T2 (#176) — pré-remplissage quand un message devient actif (AC2/AC3).
// T3 (#176) — course écho/saisie : un champ FOCALISÉ n'est jamais écrasé
//             par l'écho serveur (AC4, le point délicat du lot).
// T9b — envoi automatique (Entrée/blur/pause de frappe 2s), cycle de vie du
//       timer, gardes anti-doublon/vide — INCHANGÉ par #176, section non
//       modifiée (garde-fou explicite du plan #176).
// T4 (#176) — vidage du champ à l'effacement (AC5) ; brouillon divergent
//             préservé (AC6) — logique F2b conservée, adaptée au rendu
//             (plus de bouton « Nouveau message », le champ vidé est
//             directement visible).
// T5 (#176) — indicateur fugace « Vu par l'animateur » (AC9) : apparaît sur
//             CLEARED_BY=ANIM, pas sur REGIE, disparaît après un délai,
//             timer nettoyé au démontage — SANS masquer le champ.
// T9c — affichage piloté EXCLUSIVEMENT par l'état WebSocket (regieMessage),
//       jamais par un état local optimiste — INCHANGÉ dans son principe,
//       une assertion réécrite vers l'indicateur fugace (#176).
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

// ResizeObserver n'existe pas nativement en jsdom (#177, F1) — le composant
// en construit un inconditionnellement (mesure de sa hauteur, voir le bloc
// T1 dédié plus bas), donc TOUS les tests de ce fichier ont besoin d'un
// mock global, pas seulement ceux qui vérifient ce mécanisme spécifiquement.
// Capture les instances (et leur callback) pour que le bloc T1 (#177)
// puisse les piloter manuellement.
class ResizeObserverMock {
  constructor(callback) {
    this.callback = callback
    this.observed = []
    ResizeObserverMock.instances.push(this)
  }
  observe(target) { this.observed.push(target) }
  unobserve() {}
  disconnect() { this.disconnected = true }
  fire(height) {
    this.callback([{ contentRect: { height } }], this)
  }
}
ResizeObserverMock.instances = []

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
  ResizeObserverMock.instances = []
  global.ResizeObserver = ResizeObserverMock
  document.documentElement.style.removeProperty('--regie-bar-h')
})

afterEach(() => {
  vi.clearAllMocks()
  vi.useRealTimers()
  document.documentElement.style.removeProperty('--regie-bar-h')
})

// ---------------------------------------------------------------------------
// T1/T9 (#176) — le champ de saisie est TOUJOURS présent et éditable, quel
// que soit l'état du message (repos, actif, juste acquitté) — AC1, AC7.
// Plus d'état bloquant "acquitté", plus de bouton "Nouveau message".
// ---------------------------------------------------------------------------

describe('RegieMessageBar — le champ est toujours présent (AC1, AC7)', () => {
  it('état repos : champ vide, compteur à 140', () => {
    render(<RegieMessageBar />)
    expect(getInput()).toBeInTheDocument()
    expect(getInput()).toHaveValue('')
    expect(screen.getByText('140')).toBeInTheDocument()
  })

  it("état repos : n'affiche ni « Effacer » ni indicateur d'acquittement", () => {
    render(<RegieMessageBar />)
    expect(screen.queryByText('Effacer')).toBeNull()
    expect(screen.queryByText(/vu par l'animateur/i)).toBeNull()
  })

  it('le compteur décroît avec la longueur du texte tapé (140 - longueur)', () => {
    render(<RegieMessageBar />)
    fireEvent.change(getInput(), { target: { value: 'Question 12 annulée' } }) // 19 caractères
    expect(screen.getByText('121')).toBeInTheDocument()
  })

  it('maxLength=140 posé sur le champ (commodité — la troncature fait autorité côté serveur)', () => {
    render(<RegieMessageBar />)
    expect(getInput()).toHaveAttribute('maxLength', '140')
  })

  it('message actif : le CHAMP reste présent (pas remplacé par un texte statique), plus « Effacer »', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Question 12 annulée', SENT_AT: 1000, CLEARED_BY: '' },
    }))
    render(<RegieMessageBar />)
    expect(getInput()).toBeInTheDocument() // #176 AC1 : plus de branche exclusive sans champ
    expect(screen.getByText('Effacer')).toBeInTheDocument()
  })

  it('« Effacer » appelle clearRegieMessage (retrait régie, D4) — disponible tant qu\'ACTIVE (AC8)', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1000, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    fireEvent.click(screen.getByText('Effacer'))
    expect(props.clearRegieMessage).toHaveBeenCalledTimes(1)
  })

  it('« Effacer » absent quand aucun message n\'est actif', () => {
    render(<RegieMessageBar />)
    expect(screen.queryByText('Effacer')).toBeNull()
  })

  it('un message juste acquitté (CLEARED_BY=ANIM, ACTIVE=false) : le champ reste présent, aucun bouton « Nouveau message » (AC7)', () => {
    useGame.mockReturnValue(makeGameMock({
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    render(<RegieMessageBar />)
    expect(getInput()).toBeInTheDocument()
    expect(screen.queryByText('Nouveau message')).toBeNull()
  })
})

describe('RegieMessageBar — aucun bouton « Envoyer », dans aucun état (AC1c)', () => {
  it.each([
    ['repos', IDLE],
    ['actif', { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' }],
    ['juste acquitté', { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' }],
  ])('état %s : pas de bouton "Envoyer"', (_, regieMessage) => {
    useGame.mockReturnValue(makeGameMock({ regieMessage }))
    render(<RegieMessageBar />)
    const sendButtons = screen.queryAllByRole('button', { name: /envoyer/i })
    expect(sendButtons).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// T2 (#176) — pré-remplissage : quand un message devient actif, le champ
// affiche son texte — y compris pour un second poste régie qui n'a rien
// tapé lui-même (AC2, AC3).
// ---------------------------------------------------------------------------

describe('RegieMessageBar — pré-remplissage du champ (T2, AC2/AC3)', () => {
  it('un message qui devient actif pré-remplit le champ avec son texte', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    expect(getInput()).toHaveValue('')

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Question 12 annulée', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    expect(getInput()).toHaveValue('Question 12 annulée')
  })

  it("un second poste régie (qui n'a rien tapé) voit le texte envoyé par l'autre poste (AC3)", () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    // Aucune frappe locale ici — seul l'état serveur change, comme si un
    // AUTRE poste /admin venait d'envoyer.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Envoyé par un autre poste', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    expect(getInput()).toHaveValue('Envoyé par un autre poste')
    expect(props.sendRegieMessage).not.toHaveBeenCalled() // pure réception, aucun envoi de ce poste
  })

  it('le compteur reflète la longueur du texte pré-rempli', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Douze caractères', SENT_AT: 1, CLEARED_BY: '' }, // 16 caractères
    })
    useGame.mockReturnValue(props)
    render(<RegieMessageBar />)
    expect(screen.getByText('124')).toBeInTheDocument() // 140 - 16
  })
})

// ---------------------------------------------------------------------------
// T3 (#176) — course écho/saisie, LE point délicat du lot : un champ
// FOCALISÉ ne doit JAMAIS être écrasé par l'écho serveur (AC4). Sans cette
// garde, la régie tape "abcd", la pause de frappe envoie "abc" (frappe
// antérieure), l'écho serveur revient et réécrirait le champ à "abc" en
// pleine saisie — une course classique entre un champ contrôlé et son écho.
// ---------------------------------------------------------------------------

describe('RegieMessageBar — course écho/saisie (T3, AC4)', () => {
  it('un REGIE_MESSAGE entrant ne réécrit PAS un champ FOCALISÉ, même avec un texte différent', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    const input = getInput()

    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'abcd' } }) // la régie continue de taper

    // L'écho serveur revient pour une frappe antérieure ("abc"), PENDANT
    // que le champ est toujours focalisé.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'abc', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    expect(getInput()).toHaveValue('abcd') // pas écrasé par "abc"
  })

  it('une fois le champ défocalisé (blur), un REGIE_MESSAGE entrant LE synchronise de nouveau normalement', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    const input = getInput()

    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'Brouillon local' } })
    fireEvent.blur(input) // la régie quitte le champ (déclenche aussi un envoi, F2b — sans incidence ici)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Message venu d\'un autre poste', SENT_AT: 2, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)

    expect(getInput()).toHaveValue("Message venu d'un autre poste")
  })

  it('un champ NON focalisé (jamais touché) se synchronise sans condition — cas nominal AC2/AC3', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Première synchro', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    expect(getInput()).toHaveValue('Première synchro')

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Remplacement par un autre poste', SENT_AT: 2, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    expect(getInput()).toHaveValue('Remplacement par un autre poste')
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
// T4 (#176) — vidage du champ à l'effacement (AC5), seulement si le contenu
// n'a pas divergé entre-temps (AC6, décision ② du plan #176 : la garde F2b
// est CONSERVÉE telle quelle, seul le rendu change — plus besoin de cliquer
// « Nouveau message », le champ vidé est directement visible).
// ---------------------------------------------------------------------------

describe('RegieMessageBar — vidage automatique du champ à l\'effacement (T4, AC5)', () => {
  it('le texte qu\'on vient de faire acquitter (encore présent en local, non focalisé) est vidé AUTOMATIQUEMENT', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    // La régie tape et envoie (Entrée) — champ non focalisé pour ce test
    // (pas de fireEvent.focus), donc l'écho serveur peut ensuite le
    // synchroniser sans être bloqué par la garde de course (T3).
    fireEvent.change(getInput(), { target: { value: 'Consigne' } })
    fireEvent.keyDown(getInput(), { key: 'Enter' })

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    expect(getInput()).toHaveValue('Consigne')

    // L'animateur acquitte.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    // #176 AC5 : vidé automatiquement, SANS action de l'admin (pas de bouton
    // à cliquer — le champ vidé est déjà l'état affiché).
    expect(getInput()).toHaveValue('')
  })

  it('un brouillon qui DIVERGE du message acquitté (composé au clavier PENDANT qu\'un autre poste avait un message actif) n\'est PAS détruit (AC6)', () => {
    const props = makeGameMock({ regieMessage: IDLE })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    const input = getInput()

    // Un AUTRE poste /admin a déjà un message actif ("X").
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'X', SENT_AT: 1, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    expect(input).toHaveValue('X') // pré-rempli (T2)

    // La régie de CE poste se met à composer "Y" à la place — champ
    // focalisé, donc protégé de tout futur écho (T3) tant qu'elle y reste.
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'Y' } })

    // "X" est acquitté pendant que "Y" est en cours de composition.
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    // Le brouillon "Y" de cette session doit survivre — il ne correspond pas
    // au texte "X" qui vient d'être acquitté (comparaison sur le CONTENU,
    // pas seulement sur le focus).
    expect(getInput()).toHaveValue('Y')
  })
})

// ---------------------------------------------------------------------------
// T9c — affichage piloté EXCLUSIVEMENT par l'état WebSocket (F3b) : une mise
// à jour REGIE_MESSAGE venue d'une AUTRE session doit changer l'affichage
// SANS que ce composant ait rien envoyé lui-même. Sous #176, "l'affichage"
// se lit désormais dans la VALEUR du champ (T2 couvre déjà le cas simple
// "second poste qui n'a rien tapé") — ce bloc se concentre sur l'assertion
// réécrite vers l'indicateur fugace (plan #176 : ":347 à réécrire").
// ---------------------------------------------------------------------------

describe('RegieMessageBar — affichage piloté par regieMessage, jamais un état local (T9c, F3b)', () => {
  it('un animateur qui acquitte depuis sa tablette fait apparaître l\'indicateur fugace SANS action locale ni masquer le champ', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)
    expect(getInput()).toHaveValue('Consigne')

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    // #176 (décision ①) : indicateur fugace, PAS un état bloquant — le champ
    // (vidé, T4) reste présent et éditable en même temps.
    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()
    expect(getInput()).toBeInTheDocument()
    expect(props.clearRegieMessage).not.toHaveBeenCalled() // aucune action locale n'a eu lieu
  })
})

// ---------------------------------------------------------------------------
// T5 (#176) — indicateur fugace « Vu par l'animateur » (AC9, décision ①) :
// apparaît sur CLEARED_BY=ANIM, PAS sur CLEARED_BY=REGIE (la régie sait ce
// qu'elle vient de faire), disparaît après un court délai, sans jamais
// masquer ni désactiver le champ. Timer nettoyé au démontage, comme celui
// du debounce (F3).
// ---------------------------------------------------------------------------

describe('RegieMessageBar — indicateur fugace "Vu par l\'animateur" (T5, AC9)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('apparaît après une transition ACTIVE true -> false avec CLEARED_BY=ANIM', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()
  })

  it("n'apparaît PAS quand CLEARED_BY=REGIE (retrait régie, pas un acquittement)", () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'REGIE' },
    }))
    rerender(<RegieMessageBar />)

    expect(screen.queryByText("Vu par l'animateur")).toBeNull()
  })

  it('disparaît après un court délai (~4s d\'après le plan) — fugace, pas permanent', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)
    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()

    // Borne basse généreuse : encore là peu après l'apparition.
    act(() => { vi.advanceTimersByTime(1000) })
    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()

    // Borne haute généreuse (le plan dit "~4s", pas une valeur exacte
    // contractuelle) : parti au-delà d'une seconde marge raisonnable.
    act(() => { vi.advanceTimersByTime(6000) })
    expect(screen.queryByText("Vu par l'animateur")).toBeNull()
  })

  it('un nouvel acquittement réarme l\'indicateur (ne reste pas figé "disparu" après un premier cycle)', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Première', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)
    act(() => { vi.advanceTimersByTime(7000) }) // laisse le premier indicateur disparaître
    expect(screen.queryByText("Vu par l'animateur")).toBeNull()

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: true, TEXT: 'Seconde', SENT_AT: 2, CLEARED_BY: '' },
    }))
    rerender(<RegieMessageBar />)
    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)

    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()
  })

  it('timer nettoyé au démontage — aucune erreur si le composant disparaît pendant que l\'indicateur est affiché', () => {
    const props = makeGameMock({
      regieMessage: { ACTIVE: true, TEXT: 'Consigne', SENT_AT: 1, CLEARED_BY: '' },
    })
    useGame.mockReturnValue(props)
    const { rerender, unmount } = render(<RegieMessageBar />)

    useGame.mockReturnValue(makeGameMock({
      ...props,
      regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: 'ANIM' },
    }))
    rerender(<RegieMessageBar />)
    expect(screen.getByText("Vu par l'animateur")).toBeInTheDocument()

    expect(() => unmount()).not.toThrow()
    expect(() => { act(() => { vi.advanceTimersByTime(10000) }) }).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// T1 (#177) — mesure réelle de la hauteur du bandeau, propagée via la
// variable CSS `--regie-bar-h` sur `document.documentElement` (plan
// _work/reports/plan-20260818-174801.md, tâche F1).
//
// ⚠️ Ce bloc vérifie UNIQUEMENT le MÉCANISME (la variable posée/mise à
// jour/nettoyée) — jamais le RÉSULTAT visuel (absence de scrollbar) :
// jsdom ne fait aucun calcul de mise en page (`offsetHeight` y vaut
// toujours 0, `100vh` n'est jamais résolu). Le résultat relève de la
// recette visuelle (tests/procedures/), exécutée par l'utilisateur.
//
// ResizeObserver est mocké globalement en tête de fichier (toutes les
// suites de ce fichier en ont besoin, le composant en construit un
// inconditionnellement) — capturant le callback passé au constructeur pour
// le déclencher manuellement avec un événement `{ contentRect: { height } }`
// (forme standard de l'API, hypothèse documentée : le plan ne fixe pas la
// forme exacte de l'entrée lue par l'implémentation). `ResizeObserverMock`
// et la remise à zéro de `--regie-bar-h` sont pris en charge par le
// `beforeEach`/`afterEach` globaux ; ce bloc pilote juste les instances.
// ---------------------------------------------------------------------------

describe('RegieMessageBar — mesure de hauteur via --regie-bar-h (#177, T1)', () => {

  it('observe son élément racine au montage (un ResizeObserver est bien créé et activé)', () => {
    render(<RegieMessageBar />)
    expect(ResizeObserverMock.instances).toHaveLength(1)
    expect(ResizeObserverMock.instances[0].observed).toHaveLength(1)
    expect(ResizeObserverMock.instances[0].observed[0]).toBeInstanceOf(HTMLElement)
  })

  it('pose --regie-bar-h sur document.documentElement quand une mesure arrive', () => {
    render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(62) })

    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('62px')
  })

  it('met à jour --regie-bar-h quand ResizeObserver rapporte une nouvelle hauteur', () => {
    render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(44) })
    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('44px')

    act(() => { observer.fire(88) }) // ex. retour à la ligne sur fenêtre étroite (#176)
    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('88px')
  })

  it('arrondit la hauteur mesurée', () => {
    render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(62.7) })

    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('63px')
  })

  it("n'écrit PAS quand la valeur arrondie est inchangée (évite des invalidations de layout inutiles)", () => {
    render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]
    const setPropertySpy = vi.spyOn(document.documentElement.style, 'setProperty')

    act(() => { observer.fire(44.4) }) // #180 — Math.ceil : arrondit à 45
    act(() => { observer.fire(44.2) }) // arrondit aussi à 45 — pas de nouvelle écriture attendue

    const regieBarHCalls = setPropertySpy.mock.calls.filter(call => call[0] === '--regie-bar-h')
    expect(regieBarHCalls).toHaveLength(1)
    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('45px')

    setPropertySpy.mockRestore()
  })

  it('remet --regie-bar-h à 0px au démontage (AC8 — aucune réservation résiduelle)', () => {
    const { unmount } = render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]
    act(() => { observer.fire(62) })
    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('62px')

    unmount()

    expect(document.documentElement.style.getPropertyValue('--regie-bar-h')).toBe('0px')
  })

  it('déconnecte le ResizeObserver au démontage (pas de fuite de callback)', () => {
    const { unmount } = render(<RegieMessageBar />)
    const observer = ResizeObserverMock.instances[0]

    unmount()

    expect(observer.disconnected).toBe(true)
  })
})
