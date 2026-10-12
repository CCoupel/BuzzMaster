import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import { OtaModal } from '../components/TeamCard'
import ConnectionBadge from '../components/ConnectionBadge'
import { getRgbColor } from '../utils/colorUtils'
import { TEAM_COLORS, getNextTeamColor } from '../constants/colors'
import './TeamsPage.css'

// Answer colors for QCM mode
const ANSWER_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// #122 (fast-follow cycle 2) — geste unique, confirmation intelligente.
// Le premier cycle conditionnait deux boutons sur RECLAIM_REQUESTED : trop
// rare/imprévisible à déclencher en usage réel (_work/reports/
// plan-analysis-20260801-113000-122-verdict.md). Remplacé par LE bouton "×"
// habituel, toujours visible sur chaque fiche VJoueur, qui ouvre cette
// confirmation : elle nomme le joueur, propose l'action la plus probable en
// premier selon son statut d'équipe (avec équipe → Réinscription conserve
// équipe+score ; sans équipe → Suppression totale), et rend l'AUTRE action
// accessible dans le même dialogue — nécessaire pour pouvoir supprimer
// totalement un joueur assigné (countVirtualPlayersUnsafe compte tous les
// bumpers virtuels, équipe ou non ; sans cette voie, une place ne peut plus
// jamais être rendue à VirtualPlayerLimit une fois la partie pleine).
// L'avertissement "inscriptions fermées" est décrit sous l'option de
// suppression totale, où qu'elle apparaisse dans le dialogue — c'est là,
// pas au survol (ne fonctionne pas au doigt sur tablette), que l'animateur
// doit lire la conséquence avant de cliquer.
function ReclaimConfirmModal({ bumper, enrollmentActive, onRelease, onDelete, onClose }) {
  const name = bumper.NAME || bumper.mac.slice(-6)
  const hasTeam = !!bumper.TEAM

  const handleRelease = () => {
    onRelease(bumper.mac)
    onClose()
  }

  const handleDeleteTotal = () => {
    onDelete(bumper.mac)
    onClose()
  }

  // #134 — avant cette feature, "Réinscription" ne coupait jamais un joueur
  // encore connecté (autorisation différée #122 seulement) : aucun
  // avertissement n'était nécessaire ici. Elle évince désormais immédiatement
  // un joueur CONNECTED (score/équipe conservés côté serveur, re-clé). Deux
  // avertissements distincts, rendus SOUS l'option comme pour deleteOption
  // ci-dessous (pas au survol — ne fonctionne pas au doigt sur tablette) :
  // l'un annonce l'éviction immédiate, l'autre — piège identifié au plan
  // #134, absent du handoff initial — prévient que la reprise elle-même sera
  // bloquée si les inscriptions sont fermées au moment du clic (symétrique à
  // l'avertissement déjà existant sous "Suppression totale").
  const releaseOption = (
    <div key="release" className="reclaim-modal-option">
      <button type="button" className="reclaim-btn reclaim-btn-primary" onClick={handleRelease}>
        Réinscription
      </button>
      <span className="reclaim-sub">Retrouve son score et son équipe</span>
      {bumper.CONNECTED && (
        <div className="reclaim-warning">
          ⚠ {name} est connectée : elle sera renvoyée à l'inscription tout de suite.
        </div>
      )}
      {bumper.CONNECTED && !enrollmentActive && (
        <div className="reclaim-warning">
          ⚠ Inscriptions fermées : {name} ne pourra pas se réinscrire tant qu'elles le sont.
        </div>
      )}
    </div>
  )

  const deleteOption = (
    <div key="delete" className="reclaim-modal-option">
      <button type="button" className="reclaim-btn reclaim-btn-danger" onClick={handleDeleteTotal}>
        Suppression totale
      </button>
      <span className="reclaim-sub">Score et équipe perdus, place libérée</span>
      {!enrollmentActive && (
        <div className="reclaim-warning">
          ⚠ Inscriptions fermées : {name} ne pourra pas revenir après une suppression totale.
        </div>
      )}
    </div>
  )

  // Statut d'équipe = simple présélection, pas une règle cachée : les deux
  // options restent toujours visibles et cliquables, seul l'ordre change.
  const options = hasTeam ? [releaseOption, deleteOption] : [deleteOption, releaseOption]

  return (
    <div className="reclaim-modal-overlay" onClick={onClose}>
      <div className="reclaim-modal" onClick={(e) => e.stopPropagation()}>
        <div className="reclaim-modal-header">
          <h3>Retirer « {name} » ?</h3>
          <button className="reclaim-modal-close" onClick={onClose} aria-label="Fermer">×</button>
        </div>
        <div className="reclaim-modal-body">
          {options[0]}
          <div className="reclaim-modal-divider">ou</div>
          {options[1]}
        </div>
      </div>
    </div>
  )
}

export default function TeamsPage() {
  const { teams, bumpers, gameState, updateConfig, showQRCode, hideQRCode, setVirtualPlayerLimit, deleteBumper, releaseBumperName } = useGame()
  const [newTeamName, setNewTeamName] = useState('')
  const [draggedBumper, setDraggedBumper] = useState(null)
  const [dragOverTarget, setDragOverTarget] = useState(null)
  const [maxPlayers, setMaxPlayers] = useState(10)
  const [otaMac, setOtaMac] = useState(null)
  // #122 (fast-follow) — mac du VJoueur pour lequel la confirmation
  // Réinscription/Suppression totale est ouverte. `null` = fermée.
  const [reclaimTargetMac, setReclaimTargetMac] = useState(null)

  // Count physical vs virtual bumpers from server-synchronized data
  const physicalBumperCount = useMemo(() => {
    return Object.values(bumpers).filter(b => !b.IS_VIRTUAL).length
  }, [bumpers])

  // Use server-synchronized virtual player count (source of truth)
  const virtualPlayerCount = gameState?.virtualPlayerCount || 0
  const enrollmentActive = gameState?.enrollmentActive || false

  // Group bumpers by team
  const bumpersByTeam = useMemo(() => {
    const grouped = { unassigned: [] }
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      const teamName = bumper.TEAM || 'unassigned'
      if (!grouped[teamName]) grouped[teamName] = []
      grouped[teamName].push({ mac, ...bumper })
    })
    return grouped
  }, [bumpers])

  // Unassigned bumpers
  const unassignedBumpers = bumpersByTeam.unassigned || []

  // paletteColor : entrée TEAM_COLORS ({ key, label, rgb, deep }) — écrit COLOR
  // ET COLOR_NAME (#113) pour que le backend résolve la LED exacte du buzzer.
  const handleTeamColorChange = (teamName, paletteColor) => {
    updateConfig({
      teams: {
        ...teams,
        [teamName]: { ...teams[teamName], COLOR: paletteColor.rgb, COLOR_NAME: paletteColor.key }
      }
    })
  }

  const handleBumperNameChange = (mac, name) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], NAME: name }
      }
    })
  }

  const handleBumperTeamChange = (mac, teamName) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], TEAM: teamName }
      }
    })
  }

  const handleBumperAnswerColorChange = (mac, answerColor) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], ANSWER_COLOR: answerColor }
      }
    })
  }

  // #123 (F1) — action dédiée DELETE_BUMPER plutôt qu'un UPDATE de
  // configuration amputé : supprimer un joueur n'est pas « mettre à jour la
  // configuration », et seule DELETE_BUMPER déclenche la notification
  // PLAYER_EVICTED côté serveur (#120) pour le VJoueur concerné. B1
  // (dev-backend) couvre désormais aussi ce cas via un diff de roster sur
  // UPDATE, mais ce correctif reste nécessaire : les deux se cumulent plutôt
  // que de se substituer l'un à l'autre.
  const handleDeleteBumper = (mac) => {
    const bumper = bumpers[mac]
    const confirmMsg = bumper?.NAME
      ? `Supprimer le joueur "${bumper.NAME}" ?`
      : `Supprimer le joueur ${mac.slice(-6)} ?`
    if (!window.confirm(confirmMsg)) return
    deleteBumper(mac)
  }

  // #122 (fast-follow) — un VJoueur ouvre toujours la confirmation
  // Réinscription/Suppression totale (ReclaimConfirmModal) ; un buzzer
  // physique garde le comportement historique (confirm() simple), inchangé
  // — cette refonte ne concerne que TeamsPage.jsx et uniquement les VJoueurs.
  const handleBumperDeleteClick = (mac) => {
    const bumper = bumpers[mac]
    if (bumper?.IS_VIRTUAL) {
      setReclaimTargetMac(mac)
      return
    }
    handleDeleteBumper(mac)
  }

  const handleAddTeam = () => {
    if (!newTeamName.trim()) return
    // Attribution déterministe (#113) : première couleur de la palette 16
    // entrées non portée par une équipe existante, recyclage au rang 1 au-delà.
    const nextColor = getNextTeamColor(teams)
    updateConfig({
      teams: {
        ...teams,
        [newTeamName.trim()]: { COLOR: nextColor.rgb, COLOR_NAME: nextColor.key, SCORE: 0 }
      }
    })
    setNewTeamName('')
  }

  const handleDeleteTeam = (teamName) => {
    if (!window.confirm(`Supprimer l'equipe "${teamName}" ?`)) return
    const newTeams = { ...teams }
    delete newTeams[teamName]
    // Unassign bumpers from this team
    const newBumpers = { ...bumpers }
    Object.entries(newBumpers).forEach(([mac, bumper]) => {
      if (bumper.TEAM === teamName) {
        newBumpers[mac] = { ...bumper, TEAM: '' }
      }
    })
    updateConfig({ teams: newTeams, bumpers: newBumpers })
  }

  const handleRenameTeam = (oldName, newName) => {
    if (!newName.trim() || newName === oldName) return
    if (teams[newName]) {
      alert('Une equipe avec ce nom existe deja')
      return
    }
    const newTeams = { ...teams }
    newTeams[newName] = newTeams[oldName]
    delete newTeams[oldName]
    // Update bumpers with new team name
    const newBumpers = { ...bumpers }
    Object.entries(newBumpers).forEach(([mac, bumper]) => {
      if (bumper.TEAM === oldName) {
        newBumpers[mac] = { ...bumper, TEAM: newName }
      }
    })
    updateConfig({ teams: newTeams, bumpers: newBumpers })
  }

  // Drag and Drop handlers
  const handleDragStart = (e, mac) => {
    setDraggedBumper(mac)
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', mac)
    // Add dragging class after a small delay for visual feedback
    setTimeout(() => {
      e.target.classList.add('dragging')
    }, 0)
  }

  const handleDragEnd = (e) => {
    e.target.classList.remove('dragging')
    setDraggedBumper(null)
    setDragOverTarget(null)
  }

  const handleDragOver = (e, targetTeam) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverTarget(targetTeam)
  }

  const handleDragLeave = (e) => {
    // Only clear if leaving the drop zone entirely
    if (!e.currentTarget.contains(e.relatedTarget)) {
      setDragOverTarget(null)
    }
  }

  const handleDrop = (e, targetTeam) => {
    e.preventDefault()
    const mac = e.dataTransfer.getData('text/plain')
    if (mac) {
      handleBumperTeamChange(mac, targetTeam === 'unassigned' ? '' : targetTeam)
    }
    setDragOverTarget(null)
    setDraggedBumper(null)
  }

  const handleStartEnrollment = () => {
    setVirtualPlayerLimit(maxPlayers)
    showQRCode()
  }

  const handleStopEnrollment = () => {
    hideQRCode()
  }

  return (
    <div className="teams-page page">
      <header className="page-header">
        <h1 className="page-title">Equipes & Joueurs</h1>
        <p className="page-subtitle">Glissez-deposez les joueurs pour les assigner aux equipes</p>
      </header>

      <div className="teams-layout">
        {/* Teams Section */}
        <section className="teams-section">
          <div className="section-header">
            <h2 className="section-title">Equipes</h2>
            <form onSubmit={(e) => { e.preventDefault(); handleAddTeam(); }} className="add-team-form">
              <input
                type="text"
                value={newTeamName}
                onChange={(e) => setNewTeamName(e.target.value)}
                placeholder="Nouvelle equipe..."
                className="add-team-input"
              />
              <Button type="submit" variant="primary" size="sm">Ajouter</Button>
            </form>
          </div>

          <div className="teams-grid">
            <AnimatePresence>
              {Object.entries(teams).map(([name, data], index) => {
                const teamBumpers = bumpersByTeam[name] || []
                const rgbColor = getRgbColor(data.COLOR)
                const isDropTarget = dragOverTarget === name
                // Couleurs (#113) portées par une AUTRE équipe — marquées
                // "déjà prise" (hachures) dans le sélecteur, sans être bloquées.
                const otherTeamColorKeys = new Set(
                  Object.entries(teams)
                    .filter(([otherName]) => otherName !== name)
                    .map(([, otherData]) => otherData.COLOR_NAME)
                    .filter(Boolean)
                )

                return (
                  <motion.div
                    key={name}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                    transition={{ delay: index * 0.05 }}
                    className="team-card-wrapper"
                  >
                    <Card
                      padding="lg"
                      className={`team-card ${isDropTarget ? 'drop-target' : ''} ${draggedBumper ? 'can-drop' : ''}`}
                      style={{ '--team-color': rgbColor }}
                      onDragOver={(e) => handleDragOver(e, name)}
                      onDragLeave={handleDragLeave}
                      onDrop={(e) => handleDrop(e, name)}
                    >
                      <div className="team-header">
                        <div
                          className="team-color-badge"
                          style={{ backgroundColor: rgbColor }}
                        >
                          {name.charAt(0).toUpperCase()}
                        </div>
                        <div className="team-info">
                          <input
                            type="text"
                            defaultValue={name}
                            className="team-name-input"
                            onBlur={(e) => handleRenameTeam(name, e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') {
                                e.target.blur()
                              }
                            }}
                          />
                          <span className="team-members-count">
                            {teamBumpers.length} joueur{teamBumpers.length !== 1 ? 's' : ''}
                          </span>
                        </div>
                        <button
                          className="team-delete-btn"
                          onClick={() => handleDeleteTeam(name)}
                          title="Supprimer l'equipe"
                        >
                          ×
                        </button>
                      </div>

                      <div className="team-colors">
                        {TEAM_COLORS.map((paletteColor) => {
                          // Repli RGB pour les équipes créées avant #113 (pas de COLOR_NAME).
                          const isActive = data.COLOR_NAME
                            ? data.COLOR_NAME === paletteColor.key
                            : JSON.stringify(data.COLOR) === JSON.stringify(paletteColor.rgb)
                          const isTaken = otherTeamColorKeys.has(paletteColor.key)
                          const label = isTaken
                            ? `${paletteColor.label} (déjà prise par une autre équipe)`
                            : paletteColor.label
                          return (
                            <button
                              key={paletteColor.key}
                              type="button"
                              className={`color-swatch ${isActive ? 'active' : ''} ${isTaken ? 'taken' : ''}`}
                              style={{ backgroundColor: `rgb(${paletteColor.rgb.join(',')})` }}
                              onClick={() => handleTeamColorChange(name, paletteColor)}
                              title={label}
                              aria-label={label}
                            />
                          )
                        })}
                      </div>

                      {/* Team Members - Draggable */}
                      <div className="team-members-zone">
                        {teamBumpers.length > 0 ? (
                          <div className="team-members-list">
                            {teamBumpers.map(bumper => {
                              // Use answer color if set, otherwise gray (not for VPlayers)
                              const avatarColor = !bumper.IS_VPLAYER && bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]
                                ? ANSWER_COLORS[bumper.ANSWER_COLOR].color
                                : 'var(--gray-400)'
              return (
                                <div key={bumper.mac} className="draggable-member-wrapper">
                                <div
                                  className="draggable-member"
                                  draggable
                                  onDragStart={(e) => handleDragStart(e, bumper.mac)}
                                  onDragEnd={handleDragEnd}
                                >
                                  <div
                                    className={`member-avatar ${bumper.IS_VPLAYER ? 'vplayer-multicolor' : ''}`}
                                    style={!bumper.IS_VPLAYER ? { backgroundColor: avatarColor } : {}}
                                  >
                                    {bumper.IS_VPLAYER ? (
                                      <>
                                        <svg className="vplayer-multicolor-badge" viewBox="0 0 24 24">
                                          <path d="M 12,12 L 12,0 A 12,12 0 0,1 24,12 Z" fill={ANSWER_COLORS.RED.color} />
                                          <path d="M 12,12 L 24,12 A 12,12 0 0,1 12,24 Z" fill={ANSWER_COLORS.GREEN.color} />
                                          <path d="M 12,12 L 12,24 A 12,12 0 0,1 0,12 Z" fill={ANSWER_COLORS.YELLOW.color} />
                                          <path d="M 12,12 L 0,12 A 12,12 0 0,1 12,0 Z" fill={ANSWER_COLORS.BLUE.color} />
                                        </svg>
                                        <span className="vplayer-initial">{(bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()}</span>
                                      </>
                                    ) : (
                                      (bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()
                                    )}
                                  </div>
                                  <div className="member-info">
                                    <input
                                      type="text"
                                      value={bumper.NAME || ''}
                                      placeholder={bumper.mac.slice(-6)}
                                      onChange={(e) => handleBumperNameChange(bumper.mac, e.target.value)}
                                      className="member-name-input"
                                      onClick={(e) => e.stopPropagation()}
                                    />
                                    <span className="member-mac">{bumper.mac}</span>
                                  </div>
                                  {bumper.IS_VPLAYER ? (
                                    <div className="buzzer-vplayer-multicolor">
                                      <svg className="vplayer-multicolor-badge" viewBox="0 0 24 24">
                                        <path d="M 12,12 L 12,0 A 12,12 0 0,1 24,12 Z" fill={ANSWER_COLORS.RED.color} />
                                        <path d="M 12,12 L 24,12 A 12,12 0 0,1 12,24 Z" fill={ANSWER_COLORS.GREEN.color} />
                                        <path d="M 12,12 L 12,24 A 12,12 0 0,1 0,12 Z" fill={ANSWER_COLORS.YELLOW.color} />
                                        <path d="M 12,12 L 0,12 A 12,12 0 0,1 12,0 Z" fill={ANSWER_COLORS.BLUE.color} />
                                      </svg>
                                      <span className="vplayer-initial">{(bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()}</span>
                                    </div>
                                  ) : (
                                    bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR] && (
                                      <span className="answer-color-badge" style={{ backgroundColor: ANSWER_COLORS[bumper.ANSWER_COLOR].color }}>
                                        {ANSWER_COLORS[bumper.ANSWER_COLOR].letter}
                                      </span>
                                    )
                                  )}
                                  <ConnectionBadge state={bumper.CONN_STATE} />
                                  {bumper.ACK_PENDING === true && !bumper.IS_VIRTUAL && !bumper.IS_VPLAYER && (
                                    <span style={{display:'inline-flex',alignItems:'center',justifyContent:'center',width:'18px',height:'18px',borderRadius:'50%',background:'#f59e0b',flexShrink:0,boxShadow:'0 1px 4px rgba(245,158,11,0.5)'}} title="En attente de confirmation"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></span>
                                  )}
                                  {bumper.IS_VIRTUAL && (
                                    <button
                                      className="member-delete-btn"
                                      onClick={(e) => {
                                        e.stopPropagation()
                                        handleBumperDeleteClick(bumper.mac)
                                      }}
                                      title="Supprimer le joueur"
                                    >
                                      ×
                                    </button>
                                  )}
                                  <span className="drag-handle">⋮⋮</span>
                                </div>
                                </div>
                              )
                            })}
                          </div>
                        ) : (
                          <div className="team-drop-hint">
                            Deposez des joueurs ici
                          </div>
                        )}
                      </div>
                    </Card>
                  </motion.div>
                )
              })}
            </AnimatePresence>

            {Object.keys(teams).length === 0 && (
              <Card variant="outlined" padding="lg" className="empty-state">
                <p>Aucune equipe</p>
                <p className="empty-hint">Creez des equipes pour y assigner des joueurs</p>
              </Card>
            )}
          </div>
        </section>

        {/* Unassigned Bumpers Section */}
        <section className="bumpers-section">
          <div className="section-header">
            <h2 className="section-title">Joueurs non assignes</h2>
            <div className="bumper-counts">
              <span className="bumper-count physical">🎮 {physicalBumperCount}</span>
              <span className="bumper-count virtual">📱 {virtualPlayerCount}</span>
            </div>
          </div>

          {/* Enrollment Zone - Compact 2 lines */}
          <div className="enrollment-zone-compact">
            <div className="enrollment-line">
              <span className="enrollment-label">Places max:</span>
              <input
                type="number"
                min="1"
                max="100"
                value={maxPlayers}
                onChange={(e) => setMaxPlayers(parseInt(e.target.value, 10) || 1)}
                className="enrollment-input"
                disabled={enrollmentActive}
              />
              <span className="enrollment-label">Inscrits:</span>
              <span className="enrollment-count">{virtualPlayerCount}/{maxPlayers}</span>
            </div>
            <Button
              variant={enrollmentActive ? 'danger' : 'success'}
              onClick={enrollmentActive ? handleStopEnrollment : handleStartEnrollment}
              fullWidth
              size="sm"
            >
              {enrollmentActive ? '⏹ Fin Inscriptions' : '▶ Lancer Inscriptions'}
            </Button>
          </div>

          <div
            className={`unassigned-zone ${dragOverTarget === 'unassigned' ? 'drop-target' : ''} ${draggedBumper ? 'can-drop' : ''}`}
            onDragOver={(e) => handleDragOver(e, 'unassigned')}
            onDragLeave={handleDragLeave}
            onDrop={(e) => handleDrop(e, 'unassigned')}
          >
            <AnimatePresence>
              {unassignedBumpers.map((bumper, index) => (
                <motion.div
                  key={bumper.mac}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 20 }}
                  transition={{ delay: index * 0.03 }}
                  className="draggable-bumper"
                  draggable
                  onDragStart={(e) => handleDragStart(e, bumper.mac)}
                  onDragEnd={handleDragEnd}
                >
                  <Card
                    padding="md"
                    className={`bumper-card ${bumper.READY === 'TRUE' ? 'ready' : ''}`}
                  >
                    {/* Ligne 1: Nom + badges + bouton suppression */}
                    <div className="bumper-row-name">
                      <input
                        type="text"
                        value={bumper.NAME || ''}
                        placeholder={bumper.mac.slice(-6)}
                        onChange={(e) => handleBumperNameChange(bumper.mac, e.target.value)}
                        className="bumper-name-input"
                      />
                      {bumper.IS_VIRTUAL && (
                        <span className="virtual-badge">📱 VIRTUEL</span>
                      )}
                      {bumper.READY === 'TRUE' && (
                        <span className="ready-badge">PRET</span>
                      )}
                      <ConnectionBadge state={bumper.CONN_STATE} />
                      {bumper.ACK_PENDING === true && !bumper.IS_VIRTUAL && !bumper.IS_VPLAYER && (
                        <span style={{display:'inline-flex',alignItems:'center',justifyContent:'center',width:'18px',height:'18px',borderRadius:'50%',background:'#f59e0b',flexShrink:0,boxShadow:'0 1px 4px rgba(245,158,11,0.5)'}} title="En attente de confirmation"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></span>
                      )}
                      <button
                        className="bumper-delete-btn"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleBumperDeleteClick(bumper.mac)
                        }}
                        title="Supprimer le joueur"
                      >
                        ×
                      </button>
                    </div>

                    {/* Ligne 2: Pastille avatar + 4 couleurs (ou camembert pour VPlayer) */}
                    <div className="bumper-row-colors">
                      <div
                        className={`bumper-avatar ${bumper.IS_VPLAYER ? 'vplayer-multicolor' : bumper.ANSWER_COLOR ? 'has-color' : ''}`}
                        style={!bumper.IS_VPLAYER && bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]
                          ? { backgroundColor: ANSWER_COLORS[bumper.ANSWER_COLOR].color }
                          : {}
                        }
                      >
                        {bumper.IS_VPLAYER ? (
                          <>
                            <svg className="vplayer-multicolor-badge" viewBox="0 0 24 24">
                              <path d="M 12,12 L 12,0 A 12,12 0 0,1 24,12 Z" fill={ANSWER_COLORS.RED.color} />
                              <path d="M 12,12 L 24,12 A 12,12 0 0,1 12,24 Z" fill={ANSWER_COLORS.GREEN.color} />
                              <path d="M 12,12 L 12,24 A 12,12 0 0,1 0,12 Z" fill={ANSWER_COLORS.YELLOW.color} />
                              <path d="M 12,12 L 0,12 A 12,12 0 0,1 12,0 Z" fill={ANSWER_COLORS.BLUE.color} />
                            </svg>
                            <span className="vplayer-initial">{(bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()}</span>
                          </>
                        ) : (
                          (bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()
                        )}
                      </div>
                      <div className="answer-color-selector">
                        {Object.entries(ANSWER_COLORS).map(([key, { color, letter }]) => (
                          <button
                            key={key}
                            className={`answer-color-btn ${bumper.IS_VPLAYER ? 'active' : bumper.ANSWER_COLOR === key ? 'active' : ''} ${bumper.IS_VPLAYER ? 'locked' : ''}`}
                            style={{ backgroundColor: color }}
                            onClick={bumper.IS_VPLAYER ? undefined : (e) => {
                              e.stopPropagation()
                              handleBumperAnswerColorChange(bumper.mac, bumper.ANSWER_COLOR === key ? '' : key)
                            }}
                            title={bumper.IS_VPLAYER ? `VJoueur - ${ANSWER_COLORS[key].label}` : ANSWER_COLORS[key].label}
                            disabled={bumper.IS_VPLAYER}
                          >
                            {letter}
                          </button>
                        ))}
                      </div>
                      <span className="drag-handle">⋮⋮</span>
                    </div>

                    {/* Ligne 3: Infos techniques (MAC + version) */}
                    <div className="bumper-row-tech">
                      <span className="bumper-mac">{bumper.mac}</span>
                      {bumper.FIRMWARE_VERSION && (
                        <span
                          className={`bumper-version${bumper.IS_OUTDATED ? ' outdated' : ''}`}
                          title={bumper.IS_OUTDATED ? 'Cliquer pour mettre à jour' : `v${bumper.FIRMWARE_VERSION} — Ctrl+clic pour forcer la mise à jour`}
                          onClick={(e) => { if (bumper.IS_OUTDATED || e.ctrlKey) { e.stopPropagation(); setOtaMac(bumper.mac) } }}
                          style={(bumper.IS_OUTDATED || true) ? { cursor: 'pointer' } : undefined}
                        >
                          v{bumper.FIRMWARE_VERSION}
                        </span>
                      )}
                    </div>
                  </Card>
                </motion.div>
              ))}
            </AnimatePresence>

            {unassignedBumpers.length === 0 && !draggedBumper && (
              <div className="empty-unassigned">
                <p>Tous les joueurs sont assignes</p>
                <p className="empty-hint">Deposez des joueurs ici pour les retirer de leur equipe</p>
              </div>
            )}

            {unassignedBumpers.length === 0 && draggedBumper && (
              <div className="drop-hint-large">
                Deposez ici pour retirer de l'equipe
              </div>
            )}
          </div>
        </section>
      </div>

      {/* OTA Update Modal - opened when clicking an outdated firmware badge */}
      {otaMac && (() => {
        const b = bumpers[otaMac] || {}
        const otaBuzzer = {
          mac: otaMac,
          name: b.NAME || otaMac,
          firmwareVersion: b.FIRMWARE_VERSION || '',
          otaStatus: b.OTA_STATUS || '',
          otaPercent: b.OTA_PERCENT || 0,
        }
        return <OtaModal buzzer={otaBuzzer} onClose={() => setOtaMac(null)} />
      })()}

      {/* #122 (fast-follow) — confirmation Réinscription/Suppression totale,
          ouverte par le bouton "×" de n'importe quelle fiche VJoueur. */}
      {reclaimTargetMac && bumpers[reclaimTargetMac] && (
        <ReclaimConfirmModal
          bumper={{ mac: reclaimTargetMac, ...bumpers[reclaimTargetMac] }}
          enrollmentActive={enrollmentActive}
          onRelease={releaseBumperName}
          onDelete={deleteBumper}
          onClose={() => setReclaimTargetMac(null)}
        />
      )}
    </div>
  )
}
