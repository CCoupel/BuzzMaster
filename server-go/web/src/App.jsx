import React from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { GameProvider, useGame } from './hooks/GameContext'
import Navbar from './components/Navbar'
import RegieMessageBar from './components/RegieMessageBar'
import GamePage from './pages/GamePage'
import ScoresPage from './pages/ScoresPage'
import TeamsPage from './pages/TeamsPage'
import QuestionsPage from './pages/QuestionsPage'
import BackstagePage from './pages/BackstagePage'
import HistoryPage from './pages/HistoryPage'
import CategoryPalmaresPage from './pages/CategoryPalmaresPage'
import LogsPage from './pages/LogsPage'
import ConfigPage from './pages/ConfigPage'
import BackupPage from './pages/BackupPage'
import UpdatePage from './pages/UpdatePage'
import PlayerDisplay from './pages/PlayerDisplay'
import EnrollPage from './pages/EnrollPage'
import VPlayerPage from './pages/VPlayerPage'
import AnimPage from './pages/AnimPage'
import './App.css'

// Admin routes — /admin prefix only. /anim used to be an alias serving these
// same routes (BREAKING change, #155) — it now serves its own page (AnimPage,
// below), connected on /ws/anim with the reduced ClientTypeAnim. See
// contracts/websocket-endpoints.md and contracts/CHANGELOG.md [20260813-2].
const adminRoutes = [
  { path: '', element: <GamePage /> },
  { path: 'scoreboard', element: <ScoresPage /> },
  { path: 'teams', element: <TeamsPage /> },
  { path: 'quiz', element: <QuestionsPage /> },
  // Backstage (#215, milestone v9.0.0) — préparation de la partie (Quiz
  // méta/Entracte/Fonds d'écran), extraite de QuestionsPage qui mélangeait
  // deux métiers sans rapport. RAFALE (milestone v8.0.0, #16/#197/#198)
  // n'est plus une route dédiée : son réservoir est devenu un onglet de
  // QuestionsPage — /admin/rafale est conservée en redirection ci-dessous
  // pour ne pas casser les favoris existants.
  { path: 'backstage', element: <BackstagePage /> },
  { path: 'history', element: <HistoryPage /> },
  { path: 'palmares', element: <CategoryPalmaresPage /> },
  { path: 'settings', element: <ConfigPage /> },
  { path: 'backup', element: <BackupPage /> },
  { path: 'updates', element: <UpdatePage /> },
]

function AppContent() {
  const { status, clientCounts, version, bumpers } = useGame()
  const location = useLocation()

  // Show navbar only on admin pages (/admin/*) — /anim is its own page
  // (AnimPage) and never shows the régie navbar, same as /tv and /player (D2/F2).
  const isAdminRoute = location.pathname.startsWith('/admin')
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

          {/* Admin root route (without trailing slash) */}
          <Route path="/admin" element={<GamePage />} />

          {/* Admin sub-routes with /admin prefix */}
          {adminRoutes.filter(r => r.path !== '').map(route => (
            <Route
              key={`admin-${route.path}`}
              path={`/admin/${route.path}`}
              element={route.element}
            />
          ))}

          {/* Logs page (dedicated WebSocket) */}
          <Route path="/admin/logs" element={<LogsPage />} />

          {/* #215 — /admin/rafale (route dédiée jusqu'à v9.0.0) redirige vers
              l'onglet Rafale de /admin/quiz, pour ne pas casser les favoris
              existants (maquette backstage-215.html §02). */}
          <Route path="/admin/rafale" element={<Navigate to="/admin/quiz?tab=rafale" replace />} />

          {/* Animateur page — its own single route, connected via /ws/anim (#155) */}
          <Route path="/anim" element={<AnimPage />} />
        </Routes>
      </main>
      {/* #167 (F3) — bandeau régie, monté APRÈS <main> sous la même condition
          isAdminRoute que Navbar. Pleine largeur du bas de l'écran (position:
          fixed dans RegieMessageBar.css) — absent de /anim, /tv, /player
          (AC1). */}
      {isAdminRoute && <RegieMessageBar />}
    </div>
  )
}

export default function App() {
  const location = useLocation()
  const isAdminRoute = location.pathname.startsWith('/admin')
  const isAnimRoute = location.pathname.startsWith('/anim')
  const isTvRoute = location.pathname === '/tv'
  // All other routes (/, /player, /enroll) use /ws/player
  const endpoint = isAdminRoute ? '/ws/admin' : isAnimRoute ? '/ws/anim' : isTvRoute ? '/ws/tv' : '/ws/player'

  return (
    <GameProvider endpoint={endpoint}>
      <AppContent />
    </GameProvider>
  )
}
