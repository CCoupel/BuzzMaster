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
