import { createContext } from 'react'

/**
 * Dernier message CATEGORIES_CHANGED reçu ({ categories, change, ts }) ou null (#285).
 * Contexte volontairement séparé de GameContext : useCategories reste utilisable hors
 * GameProvider (valeur null ⇒ GET au montage + refetch()).
 */
export const CategoriesPushContext = createContext(null)
