/**
 * pictoFamily — prédicat unique d'appartenance à la famille Picto (contrat picto.md §17.1, v13.1.0, #318).
 *
 * Les types `PICTO` (un mot) et `RAFALE_PICTO` (Rafale Picto) partagent la manche dessinée : préparation, relais de
 * dessin, époque, confidentialité du mot, buzzers ignorés, crédit au niveau équipe. Chaque site qui testait
 * `TYPE === 'PICTO'` pour cette manche passe par `isPictoFamily`. Équivalent Go : `game.IsPictoFamily`.
 *
 * Ce qui est propre à un type reste testé sur le type : `isRafalePicto` (passes, fil des mots, compteur).
 */
export const PICTO_TYPE = 'PICTO'
export const RAFALE_PICTO_TYPE = 'RAFALE_PICTO'

/** @param {string|null|undefined} type - question.TYPE */
export function isPictoFamily(type) {
  return type === PICTO_TYPE || type === RAFALE_PICTO_TYPE
}

/** Rafale Picto seulement (passes, compteur, fil des mots). */
export function isRafalePicto(type) {
  return type === RAFALE_PICTO_TYPE
}
