import React from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import { GameProvider, useGame } from './hooks/GameContext'
import Navbar from './components/Navbar'
import GamePage from './pages/GamePage'
import ScoresPage from './pages/ScoresPage'
import TeamsPage from './pages/TeamsPage'
import QuestionsPage from './pages/QuestionsPage'
import ConfigPage from './pages/ConfigPage'
import PlayerDisplay from './pages/PlayerDisplay'
import './App.css'

function AppContent() {
  const { status, clientCounts, setClientType, version } = useGame()
  const location = useLocation()

  // Hide navbar on TV display
  const hideNavbar = location.pathname === '/tv'

  // Set client type based on route (TV display vs admin)
  // Only send when connected to ensure WebSocket is ready
  React.useEffect(() => {
    if (status === 'connected') {
      if (location.pathname === '/tv') {
        setClientType('tv')
      } else {
        setClientType('admin')
      }
    }
  }, [location.pathname, setClientType, status])

  return (
    <div className="app">
      {!hideNavbar && <Navbar connectionStatus={status} clientCounts={clientCounts} serverVersion={version} />}
      <main className={`main-content ${hideNavbar ? 'fullscreen' : ''}`}>
        <Routes>
          <Route path="/" element={<GamePage />} />
          <Route path="/scoreboard" element={<ScoresPage />} />
          <Route path="/teams" element={<TeamsPage />} />
          <Route path="/quiz" element={<QuestionsPage />} />
          <Route path="/settings" element={<ConfigPage />} />
          <Route path="/tv" element={<PlayerDisplay />} />
        </Routes>
      </main>
    </div>
  )
}

export default function App() {
  return (
    <GameProvider>
      <AppContent />
    </GameProvider>
  )
}
