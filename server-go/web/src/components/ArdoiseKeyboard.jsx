import './ArdoiseKeyboard.css'

// AZERTY keyboard layout rows
const AZERTY_ROWS = [
  ['A', 'Z', 'E', 'R', 'T', 'Y', 'U', 'I', 'O', 'P'],
  ['Q', 'S', 'D', 'F', 'G', 'H', 'J', 'K', 'L', 'M'],
  ['W', 'X', 'C', 'V', 'B', 'N', 'SPACE', 'BACKSPACE'],
]

// NUMPAD keyboard layout rows
const NUMPAD_ROWS = [
  ['7', '8', '9'],
  ['4', '5', '6'],
  ['1', '2', '3'],
  ['.', '0', 'BACKSPACE'],
]

// #246 — taille maximale d'une réponse ARDOISE : 2 000 points de code Unicode
// (contrat websocket-actions.md §Longueur maximale), comme le serveur.
export const ARDOISE_MAX_LENGTH = 2000

const KEY_LABELS = {
  BACKSPACE: '⌫',
  SPACE: '␣',
}

const KEY_CLASSES = {
  BACKSPACE: 'key-backspace',
  SPACE: 'key-space',
}

/**
 * @param {Object|null} [props.overlay] - #246 (maquette vplayer_ardoise_keyboard__final_ack) :
 *   message superposé quand le clavier est verrouillé, décidé par la page d'après
 *   l'ACCUSÉ du serveur (jamais « envoyé » sans preuve) :
 *   `{ tone: 'sending'|'ok'|'off'|'pause'|'wait', prefix?: string, spin?: boolean, text: string }`.
 *   `null`/absent = aucun message. Verrouillé et vide : texte indicatif « Aucune réponse ».
 */
export default function ArdoiseKeyboard({ keyboardType = 'AZERTY', value = '', onChange, disabled = false, overlay = null }) {
  const rows = keyboardType === 'NUMPAD' ? NUMPAD_ROWS : AZERTY_ROWS

  const handleKey = (key) => {
    if (disabled) return
    // #246 (F-LOG) : 2e argument = motif de l'envoi (diagnostic serveur,
    // `ARDOISE_CLEARED reason=…`) — `backspace_empty` quand ⌫ vide le champ.
    if (key === 'BACKSPACE') {
      const next = value.slice(0, -1)
      onChange(next, next === '' && value !== '' ? 'backspace_empty' : 'key')
    } else if ([...value].length >= ARDOISE_MAX_LENGTH) {
      // #246 : plafond de saisie (aligné sur la limite serveur) — saisie refusée au-delà
      return
    } else if (key === 'SPACE') {
      onChange(value + ' ', 'key')
    } else {
      onChange(value + key, 'key')
    }
  }

  const handleClear = () => {
    if (disabled) return
    onChange('', 'clear')
  }

  return (
    <div className={`ardoise-keyboard ${keyboardType === 'NUMPAD' ? 'numpad' : 'azerty'} ${disabled ? 'disabled' : ''}`}>
      {/* Text display */}
      <div className="ardoise-keyboard-display">
        <span className="ardoise-keyboard-text">
          {value || (
            <span className="ardoise-keyboard-placeholder">
              {disabled ? 'Aucune réponse' : 'Votre réponse…'}
            </span>
          )}
        </span>
        {!disabled && value.length > 0 && (
          <button
            className="ardoise-keyboard-clear"
            onClick={handleClear}
            type="button"
            title="Effacer tout"
          >
            ✕
          </button>
        )}
      </div>

      {/* Disabled overlay */}
      {disabled && overlay && (
        <div className="ardoise-keyboard-overlay">
          <span className={`ardoise-keyboard-overlay-text ov-${overlay.tone}`} data-testid="ardoise-overlay">
            {overlay.prefix}
            {overlay.spin && <span className="ardoise-spin">⏳</span>}
            {overlay.spin ? ' ' : ''}
            {overlay.text}
          </span>
        </div>
      )}

      {/* Keys */}
      <div className="ardoise-keyboard-rows">
        {rows.map((row, rowIdx) => (
          <div key={rowIdx} className="ardoise-keyboard-row">
            {row.map((key) => (
              <button
                key={key}
                type="button"
                className={`ardoise-key ${KEY_CLASSES[key] || ''}`}
                onClick={() => handleKey(key)}
                disabled={disabled}
              >
                {KEY_LABELS[key] || key}
              </button>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
