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

const KEY_LABELS = {
  BACKSPACE: '⌫',
  SPACE: '␣',
}

const KEY_CLASSES = {
  BACKSPACE: 'key-backspace',
  SPACE: 'key-space',
}

export default function ArdoiseKeyboard({ keyboardType = 'AZERTY', value = '', onChange, disabled = false }) {
  const rows = keyboardType === 'NUMPAD' ? NUMPAD_ROWS : AZERTY_ROWS

  const handleKey = (key) => {
    if (disabled) return
    if (key === 'BACKSPACE') {
      onChange(value.slice(0, -1))
    } else if (key === 'SPACE') {
      onChange(value + ' ')
    } else {
      onChange(value + key)
    }
  }

  const handleClear = () => {
    if (disabled) return
    onChange('')
  }

  return (
    <div className={`ardoise-keyboard ${keyboardType === 'NUMPAD' ? 'numpad' : 'azerty'} ${disabled ? 'disabled' : ''}`}>
      {/* Text display */}
      <div className="ardoise-keyboard-display">
        <span className="ardoise-keyboard-text">
          {value || <span className="ardoise-keyboard-placeholder">Votre réponse…</span>}
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
      {disabled && (
        <div className="ardoise-keyboard-overlay">
          <span className="ardoise-keyboard-overlay-text">
            {value ? '✓ Réponse envoyée' : '⏳ En attente…'}
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
