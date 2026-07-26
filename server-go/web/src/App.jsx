import React from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import { GameProvider, useGame } from './hooks/GameContext'
import Navbar from './components/Navbar'
import GamePage from './pages/GamePage'
import ScoresPage from './pages/ScoresPage'
import TeamsPage from './pages/TeamsPage'
import QuestionsPage from './pages/QuestionsPage'
import HistoryPage from './pages/HistoryPage'
import CategoryPalmaresPage from './pages/CategoryPalmaresPage'
import LogsPage from './pages/LogsPage'
import ConfigPage from './pages/ConfigPage'
import BackupPage from './pages/BackupPage'
import UpdatePage from './pages/UpdatePage'
import PlayerDisplay from './pages/PlayerDisplay'
import EnrollPage from './pages/EnrollPage'
import VPlayerPage from './pages/VPlayerPage'
import './App.css'

// Admin routes - duplicated for both /admin and /anim prefixes
const adminRoutes = [
  { path: '', element: <GamePage /> },
  { path: 'scoreboard', element: <ScoresPage /> },
  { path: 'teams', element: <TeamsPage /> },
  { path: 'quiz', element: <QuestionsPage /> },
  { path: 'history', element: <HistoryPage /> },
  { path: 'palmares', element: <CategoryPalmaresPage /> },
  { path: 'settings', element: <ConfigPage /> },
  { path: 'backup', element: <BackupPage /> },
  { path: 'updates', element: <UpdatePage /> },
]

function AppContent() {
  const { status, clientCounts, version, bumpers } = useGame()
  const location = useLocation()

  // Show navbar only on admin pages (/admin/* or /anim/*)
  const isAdminRoute = location.pathname.startsWith('/admin') || location.pathname.startsWith('/anim')
  const hideNavbar = !isAdminRoute

  return (
    <div className="app">
      {!hideNavbar && <Navbar connectionStatus={status} clientCounts={clientCounts} serverVersion={version} bumpers={bumpers} />}
      <main className={`main-content ${hideNavbar ? 'fullscreen' : ''}`}>
        <Routes>
          {/* Player enrollment page — connected via GameProvider endpoint="/ws/player" */}
          <Route path="/" element={<EnrollPage />} />

          {/* Virtual player page — connected via GameProvider endpoint="/ws/player" */}
          <Route path="/player" element={<VPlayerPage />} />

          {/* TV display — connected via GameProvider endpoint="/ws/tv" */}
          <Route path="/tv" element={<PlayerDisplay />} />

          {/* Admin root routes (without trailing slash) */}
          <Route path="/admin" element={<GamePage />} />
          <Route path="/anim" element={<GamePage />} />

          {/* Admin sub-routes with /admin prefix */}
          {adminRoutes.filter(r => r.path !== '').map(route => (
            <Route
              key={`admin-${route.path}`}
              path={`/admin/${route.path}`}
              element={route.element}
            />
          ))}

          {/* Admin sub-routes with /anim prefix (alias) */}
          {adminRoutes.filter(r => r.path !== '').map(route => (
            <Route
              key={`anim-${route.path}`}
              path={`/anim/${route.path}`}
              element={route.element}
            />
          ))}

          {/* Logs page (dedicated WebSocket) */}
          <Route path="/admin/logs" element={<LogsPage />} />
          <Route path="/anim/logs" element={<LogsPage />} />
        </Routes>
      </main>
    </div>
  )
}

export default function App() {
  const location = useLocation()
  const isAdminRoute = location.pathname.startsWith('/admin') || location.pathname.startsWith('/anim')
  const isTvRoute = location.pathname === '/tv'
  // All other routes (/, /player, /enroll) use /ws/player
  const endpoint = isAdminRoute ? '/ws/admin' : isTvRoute ? '/ws/tv' : '/ws/player'

  return (
    <GameProvider endpoint={endpoint}>
      <AppContent />
    </GameProvider>
  )
}
