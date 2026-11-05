import { useMemo } from 'react'
import Wheel from './wheel/Wheel'

const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')

/**
 * Adaptateur RAFALE → Wheel (contrat rafale.md §15.8) : 26 segments A–Z,
 * current = lettre courante (« # » / vide ⇒ null, aucune surbrillance),
 * lettre absente de `letters` ⇒ grisée SAUF la courante.
 *
 * @param {string} props.currentLetter - RAFALE_CURRENT_LETTER
 * @param {string[]} props.letters - RAFALE_LETTERS (lettres encore à jouer)
 * @param {'strip'|'arc'|''} [props.layout] - RAFALE_WHEEL_LAYOUT ; vide ⇒ strip
 * @param {string} [props.accent] - couleur d'équipe
 */
export default function RafaleLetterWheel({ currentLetter = '', letters = [], layout = 'strip', accent, size = 'tv' }) {
  const segments = useMemo(() => {
    const remaining = new Set(letters || [])
    return ALPHABET.map((l) => ({
      id: l,
      label: l,
      kind: 'letter',
      state: remaining.has(l) || l === currentLetter ? '' : 'disabled',
    }))
  }, [letters, currentLetter])
  const idx = ALPHABET.indexOf(currentLetter)
  return (
    <div className="rafale-letter-wheel" data-testid="rafale-wheel" style={{ width: '100%', height: '100%', minHeight: 0, overflow: 'hidden' }}>
      <Wheel
        segments={segments}
        current={idx >= 0 ? idx : null}
        layout={layout === 'arc' ? 'arc' : 'strip'}
        size={size}
        accent={accent}
        motion={null}
      />
    </div>
  )
}
