# Contrat — Composant `Wheel` (roue générique : lettres RAFALE v11.4.0, Roue de la Fortune v12.0)

> **Origine** : #282 (v11.4.0, LOT-2) ; cadrage `_work/reports/planner-wheel-20261105-113000.md` ; réutilisation
> prévue par #144/#145 (v12.0) et #146–#148 (v12.1). **Maquette normative** :
> `docs/mockup/v11.4.0/ui/tv_rafale_started__letter_wheel.html`. **Décisions** : `docs/mockup/DECISIONS.md`
> « tv_rafale_started (composant Wheel) ».
> Emplacement : `server-go/web/src/components/wheel/` (`Wheel.jsx`, `geometry.js`, `Wheel.css`, tests).

## 1. Principes

1. **Le composant ne sait rien du jeu** : il reçoit des segments, un index courant, une présentation ; il ne lit ni
   `GameState` ni le WebSocket. Les adaptateurs (`RafaleLetterWheel` en v11.4.0, `FortuneWheel` en v12.0) font la
   traduction.
2. **Pointeur fixe, contenu mobile** : le segment courant est celui qui se trouve sous le pointeur ; c'est le contenu
   (ruban ou disque) qui se déplace — `transform` CSS sur un seul groupe, jamais une mise en page recalculée.
3. **TV statique** : rendu SVG dans un conteneur `overflow: hidden`, dimensions en `vh`/`vw` (`size="tv"`) ou en px
   (`size="compact"`) ; aucun scroll, aucune hauteur dépendant du contenu.
4. **Deux présentations, une API** : `layout="strip"` (bandeau horizontal, défaut) et `layout="arc"` (disque de grand
   rayon dont la partie haute émerge du bas de la zone). `circle` (disque complet) est **réservé** (rejeté par
   l'utilisateur pour la TV, non implémenté en v11.4.0).

## 2. API

```jsx
<Wheel
  segments={[{ id: 'A', label: 'A', color: '#e5e7eb', kind: 'letter', state: '' }]}
      // kind : 'letter' | 'value' | 'special' ; state : '' | 'disabled' | 'played'
  current={2}                 // index dans `segments`, ou null (aucun segment sous le pointeur mis en évidence)
  layout="strip"              // 'strip' | 'arc' ; défaut 'strip'
  size="tv"                   // 'tv' | 'compact'
  accent="#3b82f6"            // couleur du pointeur / du halo (ex. couleur de l'équipe active)
  motion={null}               // null = bascule courte vers `current` ; objet = mouvement long (v12.0, §5)
  stepMs={600}                // durée de la bascule courte
/>
```

- `segments` : 1 à 64 éléments ; `label` texte court (lettre, montant) ; `color` optionnelle (palette cyclique par
  défaut) ; `special` (`kind === 'special'`) rendu en étiquette blanche sur fond sombre.
- `current` change ⇒ mouvement **du plus court chemin vers l'avant** (sens de lecture : A→Z, index croissant), y compris
  pour un retour Z→A (le contenu continue dans le même sens : la file est cyclique).
- États : `disabled` ⇒ opacité .25 ; `played` ⇒ opacité .5 ; courant ⇒ halo/cadre `accent`.

## 3. Géométrie (`geometry.js`, fonctions pures, testées sans DOM)

- `indexToOffset(index, n)` : position continue en « segments » ; `offsetToIndex(offset, n)` : index normalisé.
- `strip` : ruban de `n` cellules répété `REP` fois (`REP ≥ 2 + ⌈amplitudeMax / n⌉ + 1`, **7** en v11.4.0) ;
  `translateX(-(base + pos) × cellWidth − cellWidth/2)` avec `base = 2n` ; cellules hors cadre fondues sur les bords.
- `arc` : disque `viewBox 0 0 1000 1000`, rayons `R = 480`, `r = 330`, secteur `i` centré sur `-90° + i × 360/n`,
  rotation `rotate(-(pos / n) × 360°)` ; pointeur fixe au sommet.
- `angleFor(index, n)`, `positionAt(t, motion)` : exportées, couvertes par tests unitaires.

## 4. Normalisation — règle non négociable (piège trouvé sur la maquette)

La **position continue est ramenée modulo `n` à la fin de chaque mouvement**, et l'amplitude d'un mouvement est bornée
(`≤ 4n`) : sans cela, après plusieurs mouvements, le ruban sort de ses répétitions (bandeau vide) ou l'angle croît sans
borne (précision). **Critère d'acceptation** : après 100 mises à jour consécutives de `current` (ou 100 mouvements),
tous les segments visibles sont rendus et le segment courant est sous le pointeur. **Tests** : unitaire
(`positionAt` → résultat dans `[0, n)` après 1 000 mouvements aléatoires) et vitest (100 `rerender` ⇒ cellule/secteur
courant présent dans le cadre, aucune `translateX` au-delà de `REP × n` cellules).

## 5. Mouvement long (`motion`) — **révisé en v12.0.0** (#144, `contracts/roue.md` §6)

> Remplace la forme réservée en v11.4.0 (`{ from, to, turns, durationMs, startedAt }`, horloge serveur), jamais
> implémentée. Décision utilisateur du 2026-11-16 : le serveur tire la case au buzz, la position pendant la
> rotation n'a aucune valeur de jeu — **aucune horloge serveur, aucun offset client/serveur**.

```js
motion = null                                          // bascule courte vers `current` (RAFALE, inchangé)
motion = { mode: 'spin', periodMs: 2500 }              // défilement continu, un tour en periodMs ; `current` ignoré
motion = { mode: 'land', to: 7, durationMs: 4000, minTurns: 1 }   // freinage jusqu'à l'index `to`
```

- `spin` : animation `requestAnimationFrame` à vitesse constante, vers l'avant ; la position est ramenée modulo `n`
  à chaque tour **sans saut visible** (les répétitions du ruban et la rotation du disque le permettent) ; aucune
  mise en évidence du segment sous le pointeur.
- `land` : depuis la position courante (quelle qu'elle soit), avance d'au moins `minTurns` tours puis s'arrête sur
  `to`, courbe ease-out cubique ; à la fin, position normalisée (§4) et `to` mis en évidence. L'amplitude reste
  bornée (`≤ 4n`, §4) : `minTurns` effectif plafonné en conséquence.
- Passage `spin` → `land` sans à-coup (la vitesse initiale du freinage part de la vitesse de défilement, à
  l'approximation de la courbe près). Montage direct en `land` (client qui se connecte en cours de freinage) ou
  `prefers-reduced-motion` : placement immédiat sur `to`.
- `motion` absent ou `null` : comportement v11.4.0 strictement inchangé (tests existants verts sans modification).

## 6. Accessibilité et performance

`role="img"` + `aria-label` « Lettre courante : C » (ou « Segment : 500 ») ; `prefers-reduced-motion` ⇒ bascule
instantanée ; aucune ré-allocation du SVG à chaque mouvement (seul le `transform` change) ; `will-change: transform`.

## 7. Hors contrat

Segments configurables côté serveur, état de la roue dans `GameState` (`ROUE_WHEEL`) et actions de lancer / d'arrêt
(`ROUE_SPIN`, `ROUE_STOP`) : `contracts/roue.md` (v12.0.0). Son de rotation et éclairage synchronisé sur la
rotation : non prévus en v12.0.0.

## 8. Adaptateur `FortuneWheel` (v12.0.0)

`web/src/components/roue/FortuneWheel.jsx` traduit `GameState.ROUE_WHEEL` en props de `Wheel` :
- `segments` : `KIND: value` → `{ kind: 'value', label: String(VALUE) }` ; `bankrupt` / `lose_turn` →
  `{ kind: 'special', label: … }` (libellés et couleurs : maquette `tv_roue_started`) ;
- `layout` = `ROUE_WHEEL.LAYOUT` ; `accent` = `ROUE_CURRENT_TEAM_COLOR` ;
- `ROUE_SUBPHASE = SPINNING` → `motion = { mode: 'spin', periodMs: SPIN_PERIOD_MS }` ;
  `STOPPING` → `motion = { mode: 'land', to: STOP.TO, durationMs: STOP.DURATION_MS, minTurns: 1 }` ;
  sinon `motion = null`, `current = CURRENT` (ou `null` si `-1`).
- Ajouts admis au composant, additifs : libellés longs (largeur de cellule du bandeau, taille de police de l'arc)
  et `aria-label` selon `kind`.
