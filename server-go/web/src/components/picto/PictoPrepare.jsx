/**
 * PictoPrepare — choix de l'équipe et du dessinateur en préparation (PREPARE / READY), partagé par l'admin
 * (`PictoPanel`, maquette M2) et l'animateur (`AnimPicto`, maquette M1). Contrat picto.md §6.1 : « touchez un joueur
 * connecté, ou 🎲 Au hasard » ; seuls les VJoueurs connectés de l'équipe sont sélectionnables, les buzzers
 * physiques n'y figurent pas (il faut un écran pour dessiner).
 */
import { teamCss } from './pictoState'

export default function PictoPrepare({ gs, teamList = [], actions, compact = false }) {
  const team = gs.PICTO_TEAM || ''
  const candidates = gs.PICTO_CANDIDATES || []
  const anyConnected = candidates.some(c => c.CONNECTED)
  return (
    <div className={`picto-prepare${compact ? ' compact' : ''}`}>
      <div className="picto-field">
        <span className="picto-lbl">{compact ? '1 · ' : ''}Équipe qui dessine</span>
        <div className="picto-chips" role="group" aria-label="Équipe qui dessine">
          {teamList.map(t => (
            <button
              key={t.name}
              type="button"
              className={`picto-chip pick${team === t.name ? ' on' : ''}`}
              aria-pressed={team === t.name}
              style={{ '--c': teamCss(t.color) }}
              onClick={() => actions.setTeam(team === t.name ? '' : t.name)}
            >
              <span className="picto-dot" />{t.name}
            </button>
          ))}
          {teamList.length === 0 && <span className="picto-muted">Aucune équipe avec un buzzer.</span>}
        </div>
      </div>
      <div className="picto-field">
        <span className="picto-lbl">{compact ? '2 · ' : ''}Dessinateur{compact ? ' (touchez un joueur connecté, ou « Au hasard »)' : ''}</span>
        {!team ? (
          <p className="picto-muted">Choisissez d'abord l'équipe.</p>
        ) : (
          <>
            <div className="picto-drawers">
              <div className="picto-cands" role="group" aria-label="Dessinateur">
                {candidates.map(c => {
                  const chosen = gs.PICTO_DRAWER === c.ID
                  return (
                    <button
                      key={c.ID}
                      type="button"
                      className={`picto-cand${chosen ? ' on' : ''}${c.CONNECTED ? '' : ' off'}`}
                      aria-pressed={chosen}
                      disabled={!c.CONNECTED}
                      onClick={() => actions.setDrawer(chosen ? '' : c.ID)}
                    >
                      <b>{chosen ? '✏️ ' : ''}{c.NAME}</b>
                      <span className="picto-muted">
                        {c.CONNECTED ? `📱 connecté${chosen ? ' · dessinateur' : ''}` : '📱 hors ligne — non sélectionnable'}
                      </span>
                    </button>
                  )
                })}
              </div>
              <button type="button" className="picto-btn ghost" disabled={!anyConnected} onClick={actions.drawDrawer}>
                <span aria-hidden="true">🎲</span> Au hasard
              </button>
            </div>
            {candidates.length === 0 && <p className="picto-muted">Aucun joueur sur écran dans cette équipe : il faut un écran pour dessiner.</p>}
            <p className="picto-muted">
              « Au hasard » tire parmi les joueurs connectés (rejouable tant que la manche n'a pas démarré). Les buzzers physiques ne sont pas listés : il faut un écran pour dessiner.
            </p>
          </>
        )}
      </div>
    </div>
  )
}
