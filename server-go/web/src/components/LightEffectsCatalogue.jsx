import { useEffect, useMemo, useRef, useState } from 'react'
import Button from './Button'
import LightEffectPreview from './LightEffectPreview'
import {
  MOMENTS, ZONE_LABEL, TRY_SECONDS, TRY_EVENT, formatSpeed, globalEffect, globalSpeed, visibleZones,
} from '../utils/lightEffects'
import './LightEffectsCatalogue.css'

// #299 (v11.7.0, LOT-3) — catalogue des effets (sous-onglet Effets de Ambiance › Lumière).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__effects_catalogue_zones.html. Contrat : contracts/light-effects.md
// §7.1 (catalogue, `steps`, `tick_ms`), §7.2 (rechargement), §7.3 (aperçu : `tick`, `step`, `duration_ms`, `chrono_s`), §7.4 (essai).
//
// Props :
//   catalog        GET /api/lighting/effects normalisé (useLightEffects)
//   lighting       section `lighting` de config.json (« utilisé : zone, moment » d'après les réglages GLOBAUX)
//   zoneGroups     { ambiance, animateur, public } groupes non vides par zone
//   onReload()     POST /api/lighting/effects/reload ; onTry(id, seconds?) -> {ok, duration_ms?} ; onStopTry()
//   tryUnavailable() -> '' | raison (éclairage désactivé, pont, forçage ON/OFF/Flash, question en cours)
//   onManageGroups() ; focus : {id, n} carte à mettre en vue (lien « Voir l'aperçu » des réglages)
//
// Un seul aperçu à la fois ; un seul essai à la fois ; un essai et un test (bouton « Tester » des réglages) ne tournent
// jamais ensemble : le dernier lancé remplace l'autre (événement fenêtre TRY_EVENT).

const MOMENT_LABEL = { chrono: 'Chronomètre', calm: 'Phases calmes', score: 'Équipe qui marque' }
const MOMENT_SHORT = { chrono: 'chronomètre', calm: 'phases calmes', score: 'équipe qui marque' }
const ORIGIN_BADGE = { native: 'natif', user: 'personnalisé', override: 'natif modifié' }

// « utilisé : Animateur, chronomètre (×1,5) » d'après les réglages globaux ; les manches ne sont pas listées.
export function usedBy(lighting, zoneGroups, id) {
  const out = []
  for (const zone of visibleZones(zoneGroups)) {
    for (const m of MOMENTS) {
      if (globalEffect(lighting, zone, m.key) !== id) continue
      const sp = globalSpeed(lighting, zone, m.key)
      out.push(`${ZONE_LABEL[zone]}, ${MOMENT_SHORT[m.key]}${sp !== 1 ? ` (${formatSpeed(sp)})` : ''}`)
    }
  }
  return out
}

const baseName = (file) => (typeof file === 'string' ? file.split('/').pop() : '')
const tickText = (ms) => `${String(ms / 1000).replace('.', ',')} s`

export default function LightEffectsCatalogue({
  catalog, lighting = {}, zoneGroups = { ambiance: 0, animateur: 0, public: 0 }, onReload, onTry, onStopTry,
  tryUnavailable = () => '', onManageGroups = null, focus = null,
}) {
  const [previewId, setPreviewId] = useState(null)
  const [trying, setTrying] = useState(null) // { id, durationMs }
  const [starting, setStarting] = useState(null)
  const [reloading, setReloading] = useState(false)
  const timer = useRef(null)
  // Version du catalogue : change à chaque (re)chargement ; invalide les aperçus en cache (trames périmées).
  const versionRef = useRef(0)
  const version = useMemo(() => { versionRef.current += 1; return versionRef.current }, [catalog])

  // Un test des réglages (bouton « Tester ») qui démarre remplace l'essai du catalogue.
  useEffect(() => {
    const onOther = (e) => { if (e.detail?.source !== 'catalogue') { clearTimeout(timer.current); setTrying(null) } }
    window.addEventListener(TRY_EVENT, onOther)
    return () => { window.removeEventListener(TRY_EVENT, onOther); clearTimeout(timer.current) }
  }, [])

  useEffect(() => {
    if (!focus?.id) return
    document.getElementById(`fx-card-${focus.id}`)?.scrollIntoView?.({ block: 'center' })
  }, [focus])

  const toggleTry = async (e) => {
    if (trying?.id === e.id) {
      clearTimeout(timer.current)
      setTrying(null)
      onStopTry?.()
      return
    }
    setStarting(e.id)
    try {
      // Essai « sans zone ni vitesse » (corps vide) : zone Ambiance, vitesse ×1, durée choisie par le serveur (§7.4 :
      // 5 s pour un effet qui ne déclare que « score », 10 s sinon) ; la réponse donne `duration_ms`.
      const res = await onTry(e.id)
      if (res?.ok) {
        window.dispatchEvent(new CustomEvent(TRY_EVENT, { detail: { source: 'catalogue' } }))
        const durationMs = Number.isFinite(res.duration_ms) ? res.duration_ms : TRY_SECONDS * 1000
        clearTimeout(timer.current)
        setTrying({ id: e.id, durationMs })
        timer.current = setTimeout(() => setTrying(null), durationMs)
      }
    } finally { setStarting(null) }
  }

  const reload = async () => {
    setReloading(true)
    try { await onReload?.() } finally { setReloading(false) }
  }

  const errors = catalog?.errors || []
  const effects = catalog?.effects || []
  const nAmb = zoneGroups.ambiance ?? 0

  return (
    <section className="lfx" aria-labelledby="lfx-title" data-testid="lfx-catalogue">
      <div className="lfx-head">
        <h3 id="lfx-title" className="lfx-title">Catalogue des effets</h3>
        <span className="lfx-spacer" />
        <Button variant="secondary" size="sm" onClick={reload} loading={reloading}>↻ Recharger les fichiers</Button>
      </div>
      <p className="ambiance-hint">
        Chaque effet est un fichier texte dans le dossier <code>{catalog?.dir || 'data/files/effects'}</code> du serveur : une liste
        de pas qui donnent, tick après tick, une couleur à des groupes (<code>g1</code>, <code>g2</code>… ; <code>g0</code> = toute
        la zone). Un même effet peut être joué sur la zone Ambiance, Animateur ou Public. Pour créer un effet, copiez un fichier du
        sous-dossier <code>natifs</code> dans ce dossier, changez son identifiant et modifiez-le : il apparaît ici en quelques
        secondes.
      </p>

      {errors.length > 0 && (
        <div className="lfx-err" role="alert">
          <b>{errors.length === 1 ? "1 fichier n'a pas pu être chargé." : `${errors.length} fichiers n'ont pas pu être chargés.`}</b>{' '}
          Les autres effets ne sont pas touchés.
          <ul>
            {errors.map((er, i) => (
              <li key={`${er.file}-${i}`}>
                <code>{er.file}</code>{er.line ? `, ligne ${er.line}` : ''} : {er.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      {effects.length === 0 && <p className="ambiance-hint">Aucun effet disponible.</p>}

      <div className="lfx-grid">
        {effects.map((e) => {
          const used = usedBy(lighting, zoneGroups, e.id)
          const applicable = e.applicable && typeof e.applicable === 'object' ? e.applicable.ambiance !== false : e.applicable !== false
          const min = e.min_groups || 0
          const isTrying = trying?.id === e.id
          const unavailable = isTrying ? '' : !applicable ? `Demande ${min} groupe${min > 1 ? 's' : ''}` : tryUnavailable()
          const simGroups = applicable ? (nAmb > 0 ? nAmb : 6) : Math.min(12, Math.max(6, min))
          const moment = e.moments?.includes('chrono') ? 'chrono' : (e.moments?.[0] || 'chrono')
          const playing = previewId === e.id
          return (
            <article key={e.id} id={`fx-card-${e.id}`} className={`lfx-fx${applicable ? '' : ' is-na'}`} data-testid={`fx-${e.id}`}>
              <h4 className="lfx-fx-title">
                {e.name}
                <span className={`lfx-b lfx-b-${e.origin}`}>{ORIGIN_BADGE[e.origin] || e.origin}</span>
                {(e.moments || []).map(m => <span key={m} className="lfx-m">{MOMENT_LABEL[m] || m}</span>)}
                {used.length > 0 && <span className="lfx-b lfx-b-use">utilisé : {used.join(' · ')}</span>}
              </h4>
              {e.description && <p className="lfx-desc">{e.description}</p>}
              {!applicable && (
                <div className="lfx-warn">
                  Demande {min} groupe{min > 1 ? 's' : ''} ; la zone Ambiance en a {nAmb}.{' '}
                  {onManageGroups && <button type="button" className="lfx-link" onClick={onManageGroups}>Gérer les groupes</button>}
                </div>
              )}
              <LightEffectPreview
                id={e.id} groups={simGroups} moment={moment} version={version}
                playing={playing} onUnavailable={() => setPreviewId(null)}
              />
              {!applicable && playing && <p className="lfx-sim">simulation avec {simGroups} groupes</p>}
              <div className="lfx-actions">
                <Button variant="secondary" size="sm" onClick={() => setPreviewId(playing ? null : e.id)}>
                  {playing ? '■ Arrêter' : '▶ Aperçu'}
                </Button>
                <Button
                  variant="secondary" size="sm"
                  disabled={!!unavailable || starting === e.id}
                  title={unavailable || "Joue l'effet sur la zone Ambiance, à la vitesse ×1, puis l'éclairage revient à son état normal"}
                  onClick={() => toggleTry(e)}
                >
                  {isTrying ? "■ Arrêter l'essai" : '💡 Essayer sur la salle'}
                </Button>
              </div>
              <div className="lfx-file">
                Fichier :{' '}
                {e.origin === 'override'
                  ? <><code>{baseName(e.file)}</code> remplace <code>natifs/{baseName(e.file)}</code>. Supprimez-le pour revenir à l'effet d'origine.</>
                  : <code>{e.file}</code>}
                {e.steps > 0 && ` · ${e.steps} pas`}
                {e.tick_ms > 0 && ` · tick de ${tickText(e.tick_ms)}`}
                {min > 0 && ` · demande ${min} groupe${min > 1 ? 's' : ''} dans la zone`}
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}
