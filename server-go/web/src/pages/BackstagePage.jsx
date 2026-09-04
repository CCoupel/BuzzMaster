import { useState } from 'react'
import { useGame } from '../hooks/GameContext'
import QuizMetaForm from '../components/QuizMetaForm'
import EntracteConfigForm from '../components/EntracteConfigForm'
import BackgroundsManager from '../components/BackgroundsManager'
import './BackstagePage.css'
import './ConfigPage.css'
import '../styles/sliders.css'
import '../styles/tabs.css'

const TABS = [
  { key: 'quiz', label: 'Quiz' },
  { key: 'entracte', label: 'Entracte' },
  { key: 'backgrounds', label: "Fonds d'écran" },
]

/**
 * BackstagePage — zone de préparation de la partie (#215, milestone v9.0.0),
 * extraite de QuestionsPage.jsx (qui mélangeait deux métiers sans rapport :
 * la définition du contenu du jeu et le réglage de la soirée). Route
 * `/admin/backstage`, libellé Navbar "Backstage" (groupe Config).
 *
 * Trois onglets, dans l'ordre où on s'en sert : Quiz (métadonnées + NOUVELLE
 * PARTIE), Entracte (pause globale), Fonds d'écran (ambiance + écran
 * d'accueil Nouvelle Partie) — maquette docs/mockups/backstage-215.html §03.
 *
 * Aucun endpoint/action WebSocket/format disque nouveau : les trois briques
 * déplacées avaient déjà leurs points d'accès (UPDATE_QUIZ_META,
 * UPDATE_ENTRACTE_CONFIG + /api/game/entracte-image, /background,
 * /new-game-backgrounds) — critère de propreté du refactor (maquette §04).
 */
export default function BackstagePage() {
  const { gameState, sendMessage, newGame } = useGame()
  const [activeTab, setActiveTab] = useState('quiz')

  return (
    <div className="backstage-page page">
      <header className="page-header">
        <h1 className="page-title">Backstage</h1>
        <p className="page-subtitle">Préparation de la partie — quiz, entracte, fonds d'écran</p>
      </header>

      <div className="page-tabs" role="tablist">
        {TABS.map(t => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={activeTab === t.key}
            className={`page-tab ${activeTab === t.key ? 'active' : ''}`}
            onClick={() => setActiveTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {activeTab === 'quiz' && (
        <QuizMetaForm gameState={gameState} sendMessage={sendMessage} onNewGame={newGame} />
      )}

      {activeTab === 'entracte' && (
        <EntracteConfigForm gameState={gameState} sendMessage={sendMessage} />
      )}

      {activeTab === 'backgrounds' && (
        <div className="backstage-backgrounds-tab">
          <BackgroundsManager
            destination="background"
            title="Ambiance — pendant le jeu"
            hint="Images affichees en boucle sur l'ecran TV pendant le jeu."
            emptyLabel="Aucune image de fond"
            backgrounds={gameState?.backgrounds}
          />
          <BackgroundsManager
            destination="new-game-backgrounds"
            title="Écran d'accueil — Nouvelle Partie"
            hint={'Images affichees en rotation sur l\'ecran TV lors de la phase "Nouvelle Partie". Par defaut, un degrade anime est utilise.'}
            emptyLabel="Aucune image (degrade anime par defaut)"
            backgrounds={gameState?.newGameBackgrounds}
          />
        </div>
      )}
    </div>
  )
}
