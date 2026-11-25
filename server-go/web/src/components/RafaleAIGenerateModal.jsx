import PoolAIGenerateModal from './PoolAIGenerateModal'

// Constante partagée Go ↔ JS (`rafaleGenerationPresets`, ai_generator_rafale.go) : le test d'alignement
// internal/server/rafale_go_js_alignment_203_test.go la lit ICI. Valeur identique à
// POOL_INSTANCES.rafale.generationPresets (utils/poolInstances.js, vérifié par poolInstances.324.test.js).
export const RAFALE_GENERATION_PRESETS = [10, 20, 50, 100, 200]

/** Modale de génération IA du Pool Speedy (#203) — enveloppe de `PoolAIGenerateModal`, instance `rafale` (v13.1.0). */
export default function RafaleAIGenerateModal(props) {
  return <PoolAIGenerateModal instance="rafale" {...props} />
}
