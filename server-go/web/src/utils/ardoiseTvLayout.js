// #250 (v11.2) — mise en page de la grille ARDOISE de la TV au REVEAL
// (maquette validée tv_ardoise_reveal__adaptive_grid). TV STATIQUE : toutes les
// équipes ayant un VJoueur sont affichées, jamais de défilement ni de pages
// qui alternent ; plus il y a d'équipes, plus cartes et texte rétrécissent.
//
//   N      lignes  colonnes   lignes de texte max
//   1–5      1       N          2
//   6–12     2       ⌈N/2⌉      2
//   13–16    2       ⌈N/2⌉      1
//   17+      3       ⌈N/3⌉      1
//
// Les tailles de police sont des fractions de la largeur de l'écran (à
// multiplier par 100vw) proportionnelles à la largeur d'une carte.
export function getArdoiseTvLayout(n) {
  const count = Math.max(1, Math.floor(n) || 1)
  let rows, cols, lines
  if (count <= 5) { rows = 1; cols = count; lines = 2 }
  else if (count <= 12) { rows = 2; cols = Math.ceil(count / 2); lines = 2 }
  else if (count <= 16) { rows = 2; cols = Math.ceil(count / 2); lines = 1 }
  else { rows = 3; cols = Math.ceil(count / 3); lines = 1 }
  const cardW = 0.95 / cols
  return {
    rows,
    cols,
    lines,
    headerFont: Math.min(0.016, cardW * 0.12),
    answerFont: Math.min(0.026, cardW * (lines === 1 ? 0.15 : 0.13)),
  }
}
