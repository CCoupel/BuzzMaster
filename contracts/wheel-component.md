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
  motion={null}               // null = bascule courte vers `current` ; objet = mouvement long : `spin` (montée + croisière) ou `land` (v12.0, §5)
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
motion = { mode: 'spin', periodMs: 2500, accelTurns: 2 }   // défilement : montée sur 2 tours puis croisière ; `current` ignoré
motion = { mode: 'spin', periodMs: 2500, accelTurns: 2, cruise: true }   // idem, vitesse de croisière imposée (voir `cruise`)
motion = { mode: 'land', to: 7, durationMs: 4000, periodMs: 2500, minTurns: 1 }   // freinage jusqu'à l'index `to`
```

> **Révision du 2026-11-17** (décisions utilisateur après les QUALIF 12.0.0.2 et 12.0.0.3, `contracts/roue.md` §6) :
> montée en vitesse **par distance** — la pleine vitesse est atteinte en `accelTurns` tours (2 pour la Roue de la
> Fortune) ; freinage à **vitesse continue**. `accelTurns` remplace `accelMs` (durée fixe de 1 500 ms, abandonnée).
> La vitesse est une grandeur interne du composant (segments par milliseconde), jamais une prop.

**Vitesse de croisière** : `vc = n / periodMs` (un tour en `periodMs`). C'est la vitesse maximale de la roue : aucun
profil ne la dépasse.

- **`spin`** : animation `requestAnimationFrame`, vers l'avant ; aucune mise en évidence du segment sous le pointeur ;
  la position est ramenée modulo `n` à chaque tour **sans saut visible** (les répétitions du ruban et la rotation
  du disque le permettent).
  - **Montée** (`accelTurns > 0`) : la roue part **de l'arrêt**, depuis sa position courante, et accélère **à taux
    constant** jusqu'à `vc`, atteinte après **exactement `accelTurns` tours** (`accelTurns × n` segments). Il en
    découle : durée de la montée `= 2 × accelTurns × periodMs` (10 s pour 2 tours à 2 500 ms), accélération
    `= vc / (2 × accelTurns × periodMs)`, position à l'instant `t` de la montée
    `= départ + vc × t² / (4 × accelTurns × periodMs)`. Aucun à-coup à la jonction : la vitesse en fin de montée
    vaut exactement `vc`, et la roue se retrouve sur le segment de départ.
  - **Croisière** : ensuite, vitesse constante `vc`.
  - **`cruise: true`** : l'adaptateur signale que la pleine vitesse est acquise (signal venu du serveur,
    `roue.md` §6). Si la montée n'est pas terminée, le composant passe **aussitôt** à `vc` depuis sa position
    courante (pas de saut de position ; l'écart de vitesse, en fin de montée, est négligeable). Sans effet une fois
    en croisière. Ne relance jamais la montée.
  - `accelTurns` absent, nul ou invalide : pas de montée, vitesse `vc` dès le départ (comportement antérieur).
  - **Montage direct en `spin`** (composant monté alors que la roue tourne déjà : client qui se connecte en cours
    de rotation) : pas de montée, croisière immédiate, que `cruise` soit `true` ou non. La montée ne joue que sur le
    passage `null` → `spin`.
- **`land`** : freinage jusqu'à l'index `to`, arrêt **exactement** sur `to`, au plus tard à `durationMs` ; à la fin,
  position normalisée (§4) et `to` mis en évidence ; amplitude bornée (`≤ 4n`, §4) ; jamais de marche arrière.
  - **Avec `periodMs`** (ROUE) — **vitesse continue** : le freinage part **toujours de la vitesse de croisière**
    `vc`, sans saut de vitesse. Dans la Roue de la Fortune le serveur n'accepte aucun arrêt avant la pleine vitesse
    (`roue.md` §5.1) : il n'existe **aucun freinage depuis la montée**, et le composant n'a pas à en prévoir.
    - Profil : la roue continue à `vc`, puis **décélération constante** jusqu'à l'arrêt sur `to`. La distance
      découle de `vc` et de `durationMs` : première occurrence de `to` vers l'avant, plus le nombre de tours
      (0 à 3) qui permet ce profil ; la décélération dure au moins 500 ms. `minTurns` est **ignoré** dans ce mode.
    - `land` reçu alors que la montée locale n'est pas terminée (écran en retard) : traité comme `cruise: true`
      — passage à `vc` depuis la position courante, puis freinage depuis `vc`. Pas de saut de position.
    - Arrêt avant `durationMs` admis quand aucun profil n'atterrit exactement à `durationMs` (le serveur applique
      l'effet de la case à `durationMs`, `roue.md` §6 : la roue est alors déjà sur `to`).
    - Aucun profil possible (fenêtre `durationMs` trop courte pour la vitesse) : repli sur le profil sans
      `periodMs` ci-dessous.
  - **Sans `periodMs`** (profil antérieur, conservé) : depuis la position courante, avance d'au moins `minTurns`
    tours puis s'arrête sur `to`, courbe ease-out cubique ; `minTurns` effectif plafonné par l'amplitude (§4).
  - **Montage direct en `land`** (client qui se connecte en cours de freinage) ou `prefers-reduced-motion` :
    placement immédiat sur `to`, sans animation (ni montée, ni freinage).
- `motion` absent ou `null` : comportement v11.4.0 strictement inchangé (tests existants verts sans modification).

**Fonctions pures** (`geometry.js`, testées sans DOM) : position et vitesse pendant la montée et la croisière, à
partir de `(départ, t, periodMs, n, accelTurns)` ; plan de freinage à partir de
`(from, to, n, durationMs, periodMs)` — vitesse de départ toujours `vc` — et position à l'instant `t`.

**Critères d'acceptation** (tests unitaires sur les fonctions pures, plus un test du composant) :
1. Montée : vitesse nulle à `t = 0` ; à `t = 2 × accelTurns × periodMs`, vitesse égale à `vc` (écart ≤ 1 %) et
   distance parcourue égale à `accelTurns × n` segments (écart ≤ 1 %) ; vitesse croissante entre les deux ;
   position continue à la jonction montée → croisière. Vérifié pour `accelTurns = 2` et au moins deux `periodMs`.
2. Freinage depuis la croisière : pour un jeu de `(from, to, durationMs)`, la vitesse au début du freinage vaut
   `vc` (écart ≤ 5 %), la position ne recule jamais, la vitesse ne dépasse jamais `vc`, l'arrêt se fait exactement
   sur `to`, en `durationMs` au plus.
3. `land` reçu avant la fin de la montée locale : aucun saut de position ; le freinage suit le plan calculé pour
   `vc` et s'arrête exactement sur `to`, en `durationMs` au plus.
4. `cruise: true` reçu avant la fin de la montée : vitesse `vc` dès l'image suivante, aucun saut de position ;
   reçu après : aucun effet.
5. Montage direct en `spin` : vitesse `vc` dès la première image. `accelTurns` absent : idem.
6. Normalisation (§4) respectée après 100 enchaînements `spin` → `land`.

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
- `ROUE_SUBPHASE = SPINNING` → `motion = { mode: 'spin', periodMs: SPIN_PERIOD_MS, accelTurns: SPIN_UP_MS / (2 × SPIN_PERIOD_MS), cruise: FULL_SPEED }`
  — soit `accelTurns = 2` : la durée de montée vient du serveur (`ROUE_WHEEL.SPIN_UP_MS`, `roue.md` §4), jamais
  d'une constante du client ; `SPIN_UP_MS` absent ou nul : pas de montée ;
  `STOPPING` → `motion = { mode: 'land', to: STOP.TO, durationMs: STOP.DURATION_MS, periodMs: SPIN_PERIOD_MS, minTurns: 1 }` ;
  sinon `motion = null`, `current = CURRENT` (ou `null` si `-1`).
- Barre de durée maximale : affichée seulement quand `FULL_SPEED = true`, décomptée localement depuis sa réception.
- Même adaptateur et mêmes règles sur la TV et sur la tablette joueur (`roue.md` §6, §12).
- Ajouts admis au composant, additifs : libellés longs (largeur de cellule du bandeau, taille de police de l'arc)
  et `aria-label` selon `kind`.
