import { useRef, useState } from 'react'

// #210 (v11.8.0, LOT-2) — roue chromatique maison (aucune dépendance) : teinte / saturation sur la roue, luminosité au curseur,
// champ #RRGGBB, aperçu et couleurs récentes (mémorisées par navigateur). Maquette effect_editor §01 (« Pinceau »).
//
// Props : value ('#RRGGBB'), onChange(hex, { commit }) — `commit` vrai quand la couleur est « posée » (clic sur la roue, saisie du code,
// couleur récente) : elle entre alors dans les couleurs récentes ; faux pendant le glissé du curseur de luminosité.

const RECENT_KEY = 'buzzcontrol.effects.recentColors'
const RECENT_MAX = 8

export const readRecents = () => {
  try {
    const list = JSON.parse(window.localStorage.getItem(RECENT_KEY) || '[]')
    return Array.isArray(list) ? list.filter(isHex).slice(0, RECENT_MAX) : []
  } catch { return [] }
}
const writeRecents = (list) => { try { window.localStorage.setItem(RECENT_KEY, JSON.stringify(list)) } catch { /* non mémorisées */ } }

export const isHex = (h) => typeof h === 'string' && /^#[0-9a-fA-F]{6}$/.test(h)

// Conversions teinte (0-360) / saturation (0-1) / valeur (0-1) ↔ #RRGGBB.
export function hsvToHex(h, s, v) {
  const c = v * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = v - c
  const [r, g, b] = [[c, x, 0], [x, c, 0], [0, c, x], [0, x, c], [x, 0, c], [c, 0, x]][Math.floor((((h % 360) + 360) % 360) / 60) % 6]
  return '#' + [r, g, b].map(k => Math.round((k + m) * 255).toString(16).padStart(2, '0')).join('').toUpperCase()
}
export function hexToHsv(hex) {
  const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255)
  const max = Math.max(r, g, b), min = Math.min(r, g, b), d = max - min
  let h = 0
  if (d) h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return { h: ((h * 60) + 360) % 360, s: max ? d / max : 0, v: max }
}

export default function ColorWheel({ value = '#14B8A6', onChange }) {
  const hex = isHex(value) ? value.toUpperCase() : '#14B8A6'
  const [hsv, setHsv] = useState(() => hexToHsv(hex))
  const [recents, setRecents] = useState(readRecents)
  const [typed, setTyped] = useState(null)
  const wheel = useRef(null)

  const emit = (next, commit) => {
    const out = hsvToHex(next.h, next.s, next.v)
    setHsv(next)
    setTyped(null)
    if (commit) remember(out)
    onChange?.(out, { commit })
  }
  const remember = (h) => {
    const list = [h, ...readRecents().filter(x => x !== h)].slice(0, RECENT_MAX)
    writeRecents(list)
    setRecents(list)
  }
  const pick = (e) => {
    const r = wheel.current.getBoundingClientRect()
    const x = (e.clientX - r.left) / (r.width || 1) - 0.5
    const y = (e.clientY - r.top) / (r.height || 1) - 0.5
    if (Number.isNaN(x) || Number.isNaN(y)) return
    emit({ h: (Math.atan2(y, x) * 180 / Math.PI + 360) % 360, s: Math.min(1, Math.sqrt(x * x + y * y) * 2), v: hsv.v || 1 }, true)
  }
  const angle = (hsv.h * Math.PI) / 180
  const dot = { left: `${50 + hsv.s * 50 * Math.cos(angle)}%`, top: `${50 + hsv.s * 50 * Math.sin(angle)}%` }

  const commitTyped = () => {
    if (typed === null) return
    const t = typed.startsWith('#') ? typed : `#${typed}`
    if (isHex(t)) { const h = t.toUpperCase(); setHsv(hexToHsv(h)); remember(h); onChange?.(h, { commit: true }) }
    setTyped(null)
  }

  return (
    <div className="efx-picker" data-testid="color-wheel">
      <div
        ref={wheel} className="efx-wheel" role="img" aria-label="Roue de couleur : cliquez pour choisir la teinte et la saturation"
        onClick={pick}
      >
        <i style={dot} />
      </div>
      <div className="efx-picker-side">
        <label className="efx-picker-l">Luminosité de la couleur
          <input
            type="range" min="20" max="100" value={Math.round(hsv.v * 100)} aria-label="Luminosité de la couleur"
            onChange={(e) => emit({ ...hsv, v: Number(e.target.value) / 100 }, false)}
            onMouseUp={() => remember(hex)} onKeyUp={() => remember(hex)}
          />
        </label>
        <span className="efx-picker-l">Couleur choisie</span>
        <div className="efx-picker-now">
          <span style={{ background: hex }} aria-hidden="true" />
          <input
            className="efx-hex" maxLength={7} aria-label="Code de la couleur (#RRGGBB)" value={typed ?? hex}
            onChange={(e) => setTyped(e.target.value)} onBlur={commitTyped} onKeyDown={(e) => { if (e.key === 'Enter') commitTyped() }}
          />
        </div>
        {recents.length > 0 && (
          <>
            <span className="efx-picker-l">Couleurs récentes</span>
            <div className="efx-recent">
              {recents.map(h => (
                <button
                  key={h} type="button" title={h} aria-label={`Couleur récente ${h}`} style={{ background: h }}
                  onClick={() => { setHsv(hexToHsv(h)); onChange?.(h, { commit: true }) }}
                />
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
