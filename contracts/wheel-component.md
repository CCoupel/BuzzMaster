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

## 5. Mouvement long (`motion`, **réservé** — implémenté en v12.0)

```js
motion = { from, to, turns, durationMs, startedAt }   // startedAt : horloge serveur (ms)
```
Animation `requestAnimationFrame`, courbe ease-out cubique identique à la bascule courte, position dérivée du temps
serveur (offset client/serveur à ajouter au hook WS en v12.0) pour que TV, admin et tablettes montrent le même angle au
même instant. En v11.4.0 : `motion` est accepté et **ignoré** (bascule courte), documenté comme tel.

## 6. Accessibilité et performance

`role="img"` + `aria-label` « Lettre courante : C » (ou « Segment : 500 ») ; `prefers-reduced-motion` ⇒ bascule
instantanée ; aucune ré-allocation du SVG à chaque mouvement (seul le `transform` change) ; `will-change: transform`.

## 7. Hors contrat

Son de rotation, éclairage synchronisé, segments configurables côté serveur, nœud `WHEEL` de `GameState`, action
`WHEEL_SPIN` — v12.0 (`_work/reports/planner-wheel-20261105-113000.md` §4).
