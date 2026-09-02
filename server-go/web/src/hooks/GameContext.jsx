import { createContext, useContext } from 'react'
import useWebSocket from './useWebSocket'

const GameContext = createContext(null)

export function GameProvider({ children, endpoint = '/ws/admin' }) {
  const websocket = useWebSocket(endpoint)

  return (
    <GameContext.Provider value={websocket}>
      {children}
    </GameContext.Provider>
  )
}

export function useGame() {
  const context = useContext(GameContext)
  if (!context) {
    throw new Error('useGame must be used within a GameProvider')
  }
  return context
}

// #203 (v8.1.0) — variante non-bloquante de useGame(), pour un composant
// occasionnellement rendu HORS GameProvider dans ses tests unitaires
// existants (RafalePage.jsx, rendue sans wrapper dans
// RafalePage.rafale.test.jsx — 27 tests écrits AVANT l'introduction de
// aiJob/cancelAiGeneration sur cette page, contrat rafale-ai-generation.md
// §6). Retourne `null` au lieu de lever hors contexte, plutôt que de
// modifier le comportement (volontairement strict) de useGame() lui-même,
// utilisé par toutes les autres pages admin.
export function useOptionalGame() {
  return useContext(GameContext)
}
