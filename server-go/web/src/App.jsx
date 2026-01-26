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
]

function AppContent() {
  const { status, clientCounts, setClientType, version, subscribeLogs, unsubscribeLogs, logs } = useGame()
  const location = useLocation()

  // Show navbar only on admin pages (/admin/* or /anim/*)
  const isAdminRoute = location.pathname.startsWith('/admin') || location.pathname.startsWith('/anim')
  const hideNavbar = !isAdminRoute

  // Set client type based on route (TV display vs admin)
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
          {/* Player enrollment page */}
          <Route path="/" element={<EnrollPage />} />

          {/* Virtual player page */}
          <Route path="/player" element={<VPlayerPage />} />

          {/* TV display */}
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
          <Route
            path="/admin/logs"
            element={<LogsPage />}
          />
          <Route
            path="/anim/logs"
            element={<LogsPage />}
          />
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
