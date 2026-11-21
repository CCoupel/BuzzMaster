// Placement vertical du plateau ROUE (v12.1.0, #310) — règle d'AFFICHAGE partagée par l'aperçu de l'éditeur (RoueBoard) et la TV /
// tablette joueur (RoueGameBoard) : la phrase est centrée verticalement dans les 4 lignes (1 ou 2 lignes → décalage d'une ligne :
// 2 lignes → lignes 2-3 ; 3 lignes → 1-3). Le serveur ne fournit que les lignes de la phrase (mise en lignes : roue.md §3.1).
export const BOARD_MAX_ROWS = 4

/** Nombre de lignes vides à placer AVANT la phrase. */
export const boardTopOffset = (rowCount, maxRows = BOARD_MAX_ROWS) => Math.max(0, Math.floor((maxRows - rowCount) / 2))
