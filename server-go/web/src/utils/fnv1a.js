// #246 (v11.2) — empreinte FNV-1a 32 bits du texte encodé en UTF-8, rendue en
// 8 caractères hexadécimaux minuscules. Contrôle d'intégrité de ARDOISE_FINAL
// (contracts/websocket-actions.md §ARDOISE_FINAL), identique au calcul Go :
// on hache les OCTETS UTF-8, jamais les unités UTF-16.
const OFFSET_BASIS = 0x811c9dc5
const PRIME = 0x01000193

export function fnv1a(text) {
  const bytes = new TextEncoder().encode(String(text ?? ''))
  let hash = OFFSET_BASIS
  for (const b of bytes) {
    hash ^= b
    hash = Math.imul(hash, PRIME) >>> 0
  }
  return hash.toString(16).padStart(8, '0')
}

export default fnv1a
