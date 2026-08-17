import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import Timer from './Timer'

// ---------------------------------------------------------------------------
// Timer — composant partagé /admin, /tv et /anim. Aucun test dédié
// n'existait avant #171 ; ce fichier comble le vide pour garantir la
// non-régression du composant partagé (T2, plan
// _work/reports/plan-20260816-192400.md, risque R1 — "le seul endroit du
// lot qui sort de /anim").
//
// #171/F2 : `/anim` désactive désormais la pastille de phase intégrée
// (`showPhase={false}` sur son Timer de colonne chrono) et la rend
// séparément, sur la ligne réponse, en réutilisant les mêmes classes
// `phase-badge phase-*` — mais le composant Timer.jsx LUI-MÊME n'est pas
// modifié : `showPhase` existait déjà (défaut `true`). `/admin` et `/tv` ne
// passent jamais cette prop et doivent continuer à voir la pastille comme
// avant #171.
// ---------------------------------------------------------------------------

describe('Timer — pastille de phase (showPhase), défaut true (/admin, /tv non régressés)', () => {
  it('affiche la pastille de phase par défaut (showPhase non fourni)', () => {
    render(<Timer currentTime={10} totalTime={30} phase="STARTED" />)
    expect(screen.getByText('EN COURS')).toBeInTheDocument()
  })

  it('showPhase={false} masque la pastille (usage /anim, colonne chrono)', () => {
    render(<Timer currentTime={10} totalTime={30} phase="STARTED" showPhase={false} />)
    expect(screen.queryByText('EN COURS')).not.toBeInTheDocument()
  })

  it('showPhase={true} explicite affiche la pastille (équivalent au défaut, /admin et /tv)', () => {
    render(<Timer currentTime={10} totalTime={30} phase="STARTED" showPhase={true} />)
    expect(screen.getByText('EN COURS')).toBeInTheDocument()
  })

  it.each([
    ['STOPPED', 'ARRET', 'phase-stopped'],
    ['PAUSED', 'PAUSE', 'phase-paused'],
    ['STARTED', 'EN COURS', 'phase-running'],
    ['PREPARE', 'PREPARATION', 'phase-prepare'],
    ['READY', 'PRET', 'phase-ready'],
    ['REVEALED', 'REPONSE', 'phase-revealed'],
  ])('phase %s : libellé "%s" et classe "%s" (contenu inchangé par #171)', (phase, label, cls) => {
    const { container } = render(<Timer currentTime={10} totalTime={30} phase={phase} />)
    expect(screen.getByText(label)).toBeInTheDocument()
    expect(container.querySelector(`.phase-badge.${cls}`)).not.toBeNull()
  })

  it('aucune pastille pour une phase sans badge dédié (ex. NEW_GAME, COUNTDOWN, ENROLL)', () => {
    const { container } = render(<Timer currentTime={10} totalTime={30} phase="NEW_GAME" />)
    expect(container.querySelector('.phase-badge')).toBeNull()
  })
})

describe('Timer — affichage du temps et de la barre, non affectés par showPhase', () => {
  it('affiche le temps formaté MM:SS quel que soit showPhase', () => {
    render(<Timer currentTime={65} totalTime={90} phase="STARTED" showPhase={false} />)
    expect(screen.getByText('01:05')).toBeInTheDocument()
  })

  it('showBar reste indépendant de showPhase (les deux options sont orthogonales)', () => {
    const { container } = render(<Timer currentTime={10} totalTime={30} phase="STARTED" showPhase={false} showBar={true} />)
    expect(container.querySelector('.timer-bar-container')).not.toBeNull()
    expect(container.querySelector('.phase-badge')).toBeNull()
  })
})
