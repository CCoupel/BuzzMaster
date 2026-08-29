# Interface React - BuzzControl

Ce document décrit l'interface web React de BuzzControl.

## Structure des Pages

**Architecture des routes :**
- Route `/` : Page d'inscription VJoueurs (EnrollPage)
- Route `/player` : Interface de jeu VJoueur (VPlayerPage)
- Route `/tv` : Affichage TV plein écran (PlayerDisplay)
- Routes `/admin/*` : Pages d'administration
- Route `/anim` : Interface animateur dédiée (AnimPage, tablette)

| Route | Page | Description |
|-------|------|-------------|
| `/` | EnrollPage | Page d'inscription VJoueurs |
| `/player` | VPlayerPage | Interface de jeu VJoueur (smartphone) |
| `/tv` | PlayerDisplay | Affichage TV (plein écran, statique) |
| `/admin` | GamePage | Interface admin principale (Jeu) |
| `/admin/scoreboard` | ScoresPage | Tableau des scores |
| `/admin/teams` | TeamsPage | Gestion des joueurs et équipes |
| `/admin/quiz` | QuestionsPage | Gestion des questions |
| `/admin/settings` | ConfigPage | Configuration |
| `/admin/history` | HistoryPage | Historique des événements |
| `/admin/palmares` | CategoryPalmaresPage | Palmarès par catégorie |
| `/admin/logs` | LogsPage | Logs serveur temps réel |
| `/anim` | AnimPage | Interface animateur (tablette, nouvelle en v6.2.0 — refonte conduite permanente + zone réponse en v6.2.0.15, #166) |

**Navbar (v2.48.0) :**
- Affiché uniquement sur les routes `/admin/*` et `/anim/*`
- Préfixe dynamique : détecte `/anim` ou `/admin` depuis l'URL et construit les liens en conséquence
- Fonction `getFullPath(path)` pour construire les chemins avec le bon préfixe
- **Menu déroulant sur l'abeille** : Clic sur le logo 🐝 ouvre un menu avec Config et Logs
  - État `isMenuOpen` géré via useState
  - Fermeture au clic extérieur via useRef + useEffect
  - Animation CSS slideDown (200ms)
  - Accessibilité : aria-label="Menu de navigation", title="Menu"

## Composants Clés

| Composant | Fichier | Description |
|-----------|---------|-------------|
| Podium | `components/Podium.jsx` | Podium 1-2-3 avec gestion égalités (variantes: default, compact) |
| QuestionPreview | `components/QuestionPreview.jsx` | Aperçu 16:9 de l'affichage TV (utilise Podium) |
| TeamCard | `components/TeamCard.jsx` | Carte équipe compacte (260px) |
| Timer | `components/Timer.jsx` | Chronomètre avec barre de progression |
| Navbar | `components/Navbar.jsx` | Navigation + versions + compteurs clients |
| CategoryBalance | `components/CategoryBalance.jsx` | Visualisation équilibre catégories |

### Podium Component (v2.4.0)

Shared component for displaying rankings with tie support:
- **Variants**: `default` (full size), `compact` (smaller for admin/preview)
- **Tie handling**: Multiple teams/players with same score share the same rank
- **Animation**: Framer-motion for entrance animations and score changes

```jsx
<Podium teams={sortedTeams} variant="compact" />
```

### QuestionPreview Component (v2.11.0)

TV preview as iframe - perfect sync with actual /tv display:
- **Implementation**: Simple iframe pointing to `/tv`
- **Benefits**: Zero maintenance, always in sync, ~15 lines of code
- **Trade-off**: Double WebSocket connection (acceptable for admin preview)

## Layout AnimPage (Interface Animateur) - v6.2.0, refonte v6.2.0.15 (#166)

> **#166 défait volontairement plusieurs choix de #163 et #165** au fil des retours visuels de
> QUALIF (puce "Suivante" supprimée, bouton "à suivre" déplacé, rendu conditionnel de la zone
> conduite remplacé par 5 lignes permanentes, bloc "Réponse" conditionnel remplacé par une zone
> permanente floutée) — ce ne sont **pas des régressions**, mais des itérations sur la même
> branche. La section ci-dessous décrit l'état **actuel** (v6.2.0.15) ; les mentions "#163"/"#165"
> qui suivent indiquent l'origine d'un élément **toujours en vigueur**, pas son état d'origine.

Page `/anim` (tablette paysage, `overflow: hidden`, pas de scroll), disposition en grille :

```
grid-template-areas: "context  teams"      colonnes : 2fr 1fr
                      "conduct  teams"      rangées  : auto 1fr auto
                      "regie    regie"
```

| Zone | Contenu |
|---|---|
| `anim-zone-context` (bandeau) | Ligne méta (`#ID` compris, v6.2.0.25) · énoncé · ligne réponse (badge de phase + zone réponse permanente, v6.2.0.25) · colonne chronomètre |
| `anim-zone-conduct` | 5 lignes permanentes : L1 (5 gestes globaux, haut fixe) → L2 (gestes du mode) → L3 (contenu question/QCM) → L4 (réservée, #168) → **L5 "à suivre", ancré en bas** (v6.2.0.25) |
| `anim-zone-teams` | Équipes, pleine hauteur — crédit universel depuis v6.2.0.25 (#171), médaille avant le nom |
| `anim-zone-regie` | Bande messagerie régie, réservée, vide (#167) |

### Zone contexte (`anim-zone-context`) — bandeau à 3 lignes + colonne chronomètre

Grid à 2 colonnes depuis #166 (E6 = option i) : les 3 lignes ci-dessous à gauche, le `Timer`
(composant existant, `size="lg"`) occupant toute la hauteur à droite — l'élément le plus regardé
de la partie garde une taille lisible à distance au lieu d'être un petit texte parmi d'autres.

1. **Ligne méta** (`anim-meta-row`) — **réordonnée et agrandie en v6.2.0.25 (#171, tâche F1)**,
   ordre fixe des chips (vérifié par `AnimPage.test.jsx`, T1) : statut de connexion (taille
   inchangée — ce n'est pas une information de jeu) → progression `n/total` → catégorie (chip
   icône + libellé) → type (icône via `questionTypeMeta.js`) → **`#ID`** (nouvelle classe
   `.anim-chip-title`, fond transparent + bordure, famille `.anim-chip`) → options conditionnelles
   (`POINTS_TARGET` → Individuel/Équipe, `QCM_HINTS_ENABLED` → Indices, `MEMORY_MODE`) → points
   (droite, ancrage inchangé depuis la retouche #163/v6.2.0.11). Toutes les chips (hors statut de
   connexion) sont désormais à la **taille du bouton "à suivre"**, pour la lisibilité "à bout de
   bras" demandée par #171.
   > **`#ID` est le "titre" de la question — un seul exemplaire sur la page.** Avant #171, `#ID`
   > apparaissait deux fois : discrètement en retrait sur cette ligne (depuis #166) et en toutes
   > lettres dans la puce "Suivante"/le bouton "à suivre" (#163/#165, format inchangé). #171
   > tranche : le numéro de question **est** le titre demandé, ni le nom du quiz ni l'énoncé — il
   > monte en évidence ici et **son ancien affichage en retrait disparaît**. `#ID` reste par
   > ailleurs affiché dans `AnimNextButton` (voir plus bas) : ce sont deux informations
   > différentes (question **courante** vs question **suivante**), pas un doublon.
2. **Énoncé** (`anim-question-statement`) : `question.QUESTION`, affiché dès qu'une question est
   chargée, sans garde de phase (dès `READY`, avant le lancement du chrono — écart assumé avec la
   TV, `PlayerDisplay.jsx`, qui n'affiche le contenu qu'à partir de `STARTED`). Tronqué à 2 lignes.
3. **Ligne réponse** (`anim-answer-row`, **nouveau wrapper v6.2.0.25, #171 tâche F2**) : badge de
   statut de partie (nouveau, voir ci-dessous) suivi de la **zone réponse permanente**
   (`AnimAnswerZone`, **nouveau composant #166, révélation par pression #169**) — voir ci-dessous
   pour le détail des deux.

Aucune donnée supplémentaire n'a été demandée au backend pour l'essentiel de la zone : la seule
addition protocole de #166 est `n/total` (`NEXT_QUESTION.CURRENT_POSITION`/`TOTAL_QUESTIONS`, voir
`docs/WEBSOCKET_PROTOCOL.md` §"Action NEXT_QUESTION") ; tout le reste (`QUESTION`, `QCM_ANSWERS`,
`QCM_CORRECT`, `ANSWER`) transitait déjà sur `/ws/anim` depuis #155. #169 et #171 sont 100%
interaction/présentation côté client, aucune donnée supplémentaire — #171 en particulier ne touche
**aucun fichier Go, aucun contrat** (plan §7).

### Statut de partie sur la ligne réponse (v6.2.0.25, #171) — dérogation documentée

Fichier : `web/src/utils/phaseBadge.js` (nouveau). Avant #171, la pastille de phase
(`phase-badge phase-*`) n'était visible sur `/anim` que dans le `Timer` de la colonne chronomètre.
#171 la déplace sur la ligne réponse, **juste avant** `AnimAnswerZone` — l'endroit où l'animateur
regarde au moment de créditer, plutôt que la colonne chrono qu'il ne consulte plus une fois la
question arrêtée.

**Contrainte** : `Timer.jsx` est **partagé** avec `/admin` et `/tv` — impossible de lui retirer sa
pastille sans régresser ces deux interfaces. Solution retenue : `Timer` porte déjà une prop de
rendu `showPhase` (défaut `true`, aucune retouche au composant n'a été nécessaire — la prop
existait avant même #171) ; `AnimPage.jsx` la désactive (`showPhase={false}`) **uniquement** sur
l'instance de la colonne chrono, et rend un badge indépendant sur la ligne réponse en réutilisant
les **mêmes classes CSS et libellés** que `Timer.css` (`phase-badge phase-*`, déjà chargées
globalement sur `/anim`).

**⚠️ Dérogation assumée à la règle anti-duplication** (documentée, pas silencieuse) : la table
phase → `{classe, libellé}` est dupliquée une fois dans `phaseBadge.js`, plutôt que de retoucher
l'intérieur de `Timer.jsx`. Justifiée et approuvée en revue (`code-review-20260816-194413.md`) —
l'alternative (exposer cette table depuis `Timer.jsx` pour un unique second consommateur) aurait
couplé un composant partagé avec `/admin`/`/tv` à un besoin propre à `/anim`, pour un gain
marginal. `/admin` et `/tv` non régressés : le `Timer` y garde sa pastille, comportement identique
à avant #171 (`showPhase` par défaut).

Couvre les 7 phases serveur : `STOPPED`/`PREPARE`/`READY`/`COUNTDOWN`/`STARTED`/`PAUSED`/
`REVEALED` — `COUNTDOWN` (compte à rebours avant le lancement, libellé "COMPTE A REBOURS", repris
du badge existant côté `/admin`) manquait initialement à `phaseBadge.js` (trou de couverture de
#171, révélé en pratique par MEMORY dont la phase de mémorisation traverse `COUNTDOWN` de façon
prolongée, mais générique à tous les modes), corrigé en v6.2.0.30.

### `AnimAnswerZone` Component (v6.2.0.15, #166 — révision v6.2.0.17, #169)

Fichiers : `components/AnimAnswerZone.{jsx,css}`.

- **Toujours rendue** dès qu'une question est chargée (`question` non nul) — pas une garde de
  phase comme en #163, une garde de **présence de question**. Absente/vide si aucune question
  n'est chargée (jamais un cadre sans contenu).
- Remplace le bloc conditionnel "Réponse" de #163 et **unifie** l'affichage pour tous les modes :
  hors QCM → `question.ANSWER` (tiret si vide — ARDOISE/MEMORY/MEMOTION) ; QCM → pastille de
  couleur (`QCM_CORRECT` via `QCM_COLORS`) + libellé (`QCM_ANSWERS[QCM_CORRECT]`).
- **Dimensions et position identiques dans les trois états** (masquée / révélée par pression /
  révélée par phase) — seuls le flou, l'opacité et le style de bordure changent, pour qu'aucune
  transition ne décale rien à l'écran (vérifié par `AnimAnswerZone.test.jsx`, comparaison
  avant/après `rerender`).

**Révélation (révisée #169 — remplace le flou passif permanent de #166, qui ne protégeait rien de
réel : une réponse floutée en continu reste lisible à qui insiste) :**

| État | Condition | Comportement |
|---|---|---|
| **Masquée** | `phase !== 'REVEALED'`, pas de pression en cours | Floutée + opacité réduite (défaut) |
| **Révélée par pression** | `phase !== 'REVEALED'`, pointeur maintenu sur la zone | Nette **tant que le pointeur reste appuyé** — relâché ou sorti de la zone → remasquée immédiatement |
| **Révélée par phase** | `phase === 'REVEALED'` | Nette **en permanence, sans interaction** — inchangé depuis #166, l'animateur n'a pas à garder le doigt appuyé pendant qu'il crédite les équipes |

Implémentation : état interne `peeking` (`useState`), réinitialisé au changement de
`question.ID` (garde-fou si un `pointerup` est manqué entre deux questions, ex. `STOP` puis
enchaînement pendant une pression). `visible = revealed || peeking` pilote la classe CSS
`revealed`/`masked` (réutilisée, sémantique élargie à l'état visuel). Handlers
`onPointerDown`/`onPointerUp`/`onPointerLeave`/`onPointerCancel` sur la racine — **Pointer Events
uniquement**, pas d'événements souris/touch séparés ; no-op une fois `revealed` vrai. Classe
`.anim-answer-zone-peekable` (présente uniquement si `!revealed`) ajoute `touch-action: none` pour
éviter qu'un scroll tactile parasite interrompe le geste.

`user-select: none` sur le contenu masqué — précaution d'usage, **pas un mécanisme de
confidentialité**, ni avec le flou (#166) ni avec la pression (#169) : la donnée transite déjà sur
`/ws/anim` dès le chargement de la question (constat #163, inchangé) ; ni l'un ni l'autre
n'empêche un regard appuyé ou les outils de développement du navigateur — la tablette animateur ne
doit pas être visible du public. Voir `contracts/ws-payload-serialization.md` §"Clarification
(#163 → révisée #166)".

La grille QCM (L3 de la conduite depuis #171, voir `AnimQcmOptions` ci-dessous) conserve en
parallèle son propre marquage de la bonne réponse en `REVEALED` — les deux lectures se confirment,
la zone réponse ne le remplace pas.

### Zone conduite (`AnimConductPanel`) — 5 lignes permanentes, mapping révisé et L5 ancré (v6.2.0.25, #171)

**Renversement de principe depuis #166** : le composant ne choisit plus *quels* boutons rendre
selon la phase (comme en #163/#165) — il rend **toujours les 5 mêmes emplacements** et calcule
l'**état** de chacun à partir d'`utils/phaseRules.js` (voir ci-dessous), jamais de condition
réécrite localement dans le composant.

**Mapping révisé par #171** (le contenu de L2 et L3 échange sa place par rapport à #166 ; le
bouton "à suivre" quitte sa position juste-après-L1 pour un ancrage en bas de zone) :

| Emplacement | Contenu | Position |
|---|---|---|
| **L1** | LANCER · PAUSE · CONTINUER · STOP · RÉPONSE — 5 emplacements fixes | **haut, fixe** — toujours montés, actifs ou éteints selon la phase |
| **L2** | Gestes spécifiques au mode — `AnimMotionActions` en MEMOTION (**v6.2.0+, #160**) ; emplacement réservé sinon | bloc central — première occupation ; *ex-L3 de #166* |
| **L3** | Contenu de la question — `AnimQcmOptions` en QCM, `AnimMemoryGrid` en MEMORY (**v6.2.0.27, #159**), `AnimMotionGrid` / `AnimMotionCard` en MEMOTION (**v6.2.0+, #160**) ; emplacement réservé sinon | bloc central — *ex-L2 de #166*, branche à 4 voies depuis #160 |
| **L4** | Note d'explication | bloc central — réservée, vide, préparée pour #168, aucun contrat, **hauteur libre sans plafond** (voir §ancrage ci-dessous) |
| **L5** | `AnimNextButton` ("à suivre") | **bas, ancré** — dernier enfant de `.anim-conduct`, position fixe quelle que soit la hauteur de L2/L3/L4 |

Lecture résultante, de haut en bas : ce qu'on déclenche pour **tous** les jeux (L1) · ce qu'on
déclenche pour **celui-ci** (L2) · ce qu'on **lit** de la question (L3) · ce qu'on **commente**
(L4) · puis, toujours au même endroit quoi qu'il arrive, ce qui **vient après** (L5).

**Mécanisme d'ancrage (F3, v6.2.0.25)** : `.anim-conduct` est une colonne flex. L2, L3 et L4 sont
regroupés dans un bloc central unique `.anim-conduct-mid` (`flex: 1; min-height: 0; overflow-y:
auto`) ; `AnimNextButton` est le **dernier enfant** de `.anim-conduct`, donc naturellement ancré
en bas. ⚠️ **`min-height: 0` sur le bloc central n'est pas optionnel** — sans lui, un enfant flex
refuse par défaut de rétrécir sous la taille de son contenu : une L4 longue (texte de #168 futur)
pousserait L5 hors de l'écran, exactement le défaut que l'ancrage est censé empêcher. C'est le
seul piège CSS nouveau introduit par #171, couvert par un test dédié (`AnimConductPanel.test.jsx`,
T4) et un point de revue explicite. Avant #171 (flux simple, #166), la hauteur de la conduite
était la somme de ses lignes, sans plafond — le jour où L4 grandirait, elle aurait poussé le reste
hors de l'écran ; ancrée, la hauteur totale est désormais **fixée** par la rangée de grille, et
c'est le bloc central qui absorbe l'excédent par défilement **interne**, jamais la page. Un
plafond de hauteur avait été envisagé pour L4 dans une révision antérieure du plan : devenu
inutile avec l'ancrage, il n'a pas été implémenté.

Chaque bouton de L1 a un des 4 états suivants (convention de couleur posée par #165, étendue par
#166 à un état permanent) :

| État | Couleur | Signification |
|---|---|---|
| `go` | **Vert** | Action normale/attendue du flux courant |
| `optional` | **Bleu** | Action optionnelle, court-circuite le flux normal |
| `danger` | **Rouge** | Action destructive (STOP) |
| `off` | **Gris** | Éteint — **non cliquable, n'émet aucune action** ; libellé secondaire indiquant le motif ("attendu", "optionnel", "indispo.", "après arrêt"...) |

`AnimNextButton` (L5, voir ci-dessous) suit la même palette avec un 3ᵉ état dédié `inert` (fin de
quiz). La matrice complète (10 situations de phase × 5 boutons de L1 + L5) est dérivée à la
main de `phaseRules.js`, recoupée avec la maquette de référence, et couverte exhaustivement par
`AnimConductPanel.test.jsx` (T6, test central du lot — 50 combinaisons vérifiées : présence,
état, libellé secondaire, absence d'action émise par un bouton éteint) et
`AnimNextButton.test.jsx` (T5). La logique d'état elle-même (`phaseRules.js`) est **inchangée par
#171** — seule la position de L5 dans la page a changé. Un extrait vérifié en conditions réelles
serveur (voir `_work/reports/qa-20260815-165224.md`) :

| Phase | LANCER | PAUSE | CONTINUER | STOP | RÉPONSE | L5 "à suivre" |
|---|---|---|---|---|---|---|
| `PREPARE` | off ("indispo.") | off | off | off | off | **go** (seule action) |
| `STARTED` | go ("en cours") | **optional** (bleu) | off | **danger** (rouge, "arrête") | off ("après arrêt") | inert (pointillé) |
| `PAUSED` | — | off | **go** | **danger** | off | inert |
| `REVEALED` | — | — | — | — | off ("déjà révélée") | **go** |
| `READY` (avec `nextQuestion`) | go | — | — | — | — | **optional** (bleu, bypass) |

### `AnimNextButton` Component (v6.2.0.15, #166 — repositionné en L5 ancré v6.2.0.25, #171)

Fichiers : `components/AnimNextButton.{jsx,css}`. **Déplacement de position, composant non
modifié par #171** : le bouton existait déjà (bandeau contexte en #163, puis zone conduite
juste-après-L1 avec couleurs en #165) — #171 le sort du bloc central de la conduite pour en faire
le **dernier enfant** de `.anim-conduct` (L5, ancré en bas — voir §"Zone conduite" ci-dessus).
Format, trois états et règle de couleur strictement intacts.

- **Trois états** : `go` (vert, seule action disponible), `optional` (bleu, LANCER également
  disponible — bypass), `inert` (fin de quiz, pointillé, libellé "Fin du quiz" — override quand
  `!nextQuestion`, une donnée et non une règle de phase, donc volontairement absent de
  `phaseRules.js`).
- Alimenté par `phaseRules.nextButtonState(phase, question)` — la prop `question` distingue
  `STOPPED` "jouée" de "non jouée" (deux lignes différentes de la matrice).
- Depuis #166 : pointe la **première question jouable du quiz** quand aucune question n'est en
  cours (voir `docs/WEBSOCKET_PROTOCOL.md` §"Action NEXT_QUESTION", règle 1 révisée) — permet de
  démarrer une partie directement depuis la tablette.
- Format inchangé depuis #165 : `nextQuestionFormat.js` (`formatNextQuestionStatement`/
  `formatNextQuestionMeta`), toujours l'unique source de ce format — **seul consommateur restant**
  depuis la suppression de la puce "Suivante" du bandeau (#166/F4).

### `AnimQcmOptions` Component (v6.2.0.10, #163 — déplacé v6.2.0.15/#166 puis v6.2.0.25/#171)

Fichiers : `components/AnimQcmOptions.jsx`, `components/AnimQcmOptions.css`. **Composant non
modifié depuis #163** (ni props, ni logique, ni CSS interne) — seul son point de montage a bougé
deux fois : bandeau contexte (#163) → L2 de la zone conduite (#166, arbitrage GATE 2 E5) → **L3**
(#171, échange de place avec les gestes du mode — voir §"Zone conduite" ci-dessus).

- **Props** : `answers` (`question.QCM_ANSWERS`), `correct` (`question.QCM_CORRECT`),
  `invalidated` (`gameState.qcmInvalidated`), `revealed` (booléen, `phase === 'REVEALED'`), `type`
  (garde de robustesse au montage isolé).
- **Rendu** : grille 2×2, une carte par couleur dans l'ordre `RED, GREEN, YELLOW, BLUE` (lettres
  A/B/C/D), pastille + libellé tronqué sur une ligne. Réutilise **exclusivement** `QCM_COLORS`
  (`constants/colors.js`) — aucune table couleur→lettre dupliquée (même source que la zone équipes,
  #157).
- **Invalidée** (indice) : opacité réduite + texte barré — parité visuelle avec
  `PlayerDisplay.jsx` (`.qcm-answer-item.invalidated`).
- **Bonne réponse** : liseré vert + coche, **uniquement** si `revealed` est vrai — seule condition
  d'affichage, aucun autre chemin. Se confirme avec le marquage d'`AnimAnswerZone` (ci-dessus).
- Ne rend rien si `question.TYPE !== 'QCM'` ou si `QCM_ANSWERS` est absent (montage isolé sûr).

### `AnimMemoryGrid` Component (v6.2.0.27, #159) — grille MEMORY tactile en L3

Fichiers : `components/AnimMemoryGrid.{jsx,css}` (nouveaux). **Créé, pas repris** du rendu de
`PlayerDisplay.jsx` (dense, pensé souris, gardé par `isAdminPreview`) — mais **réutilise
obligatoirement sa règle de disposition** via `utils/memoryGrid.js` (voir ci-dessous). Composant
unique rendant à la fois le bandeau de compteurs (paires, erreurs, équipe active) et la grille de
cartes.

**Quatre états de carte** :

| État | Condition | Rendu |
|---|---|---|
| **Face cachée** | Ni retournée, ni appariée | Cliquable si `canClick` (`phase === 'STARTED' && !isMatched && !isFlipped`) |
| **Retournée** | Retournée par le serveur, pas encore appariée | Contenu visible (texte ou image), non cliquable |
| **Paire trouvée** | Appariée | Couleur du propriétaire (`--anim-memory-owner-color`), non cliquable |
| **Inerte** | Hors phase `STARTED` | Non cliquable quel que soit son état, quelle que soit la carte |

**Aucune logique de jeu côté client** : pas de détection de paire, pas de minuterie de retour
automatique — le serveur seul décide (`Engine.FlipMemoryCard`), la tablette se contente
d'afficher l'état reçu et d'émettre `FLIP_MEMORY_CARD` (identifiant `"pairID-cardNum"`, format
exact attendu par le moteur) sur clic d'une carte face cachée cliquable. Cartes texte
(`card.TEXT`) et image (`card.IS_IMAGE && card.IMAGE`), cible tactile ≥ 62 px.

**Aucune restriction par équipe côté `/anim`** (contrairement au VPlayer, qui ne peut retourner
que ses propres cartes en mode équipe active) — l'animateur joue pour la table entière, comme la
TV. Voir `docs/WEBSOCKET_PROTOCOL.md` §"Liste blanche entrante" pour le détail de l'action
`FLIP_MEMORY_CARD`, désormais acceptée depuis `anim` en plus de `tv`/`vplayer`.

**Bandeau de compteurs** : "au tour de" (multi-équipes et phase `STARTED` uniquement), "paires"
(compteur global, appariées/total), "erreurs" (somme des équipes en mode multi-équipes, repli sur
le compteur global `memoryErrors` sinon).

### `utils/memoryGrid.js` — règle de disposition partagée avec `PlayerDisplay.jsx` (v6.2.0.27, #159)

Fichier : `web/src/utils/memoryGrid.js` (nouveau). **Motif : la correspondance positionnelle.**
Un joueur annonce "la deuxième carte en haut à gauche" ; l'animateur doit désigner **exactement la
même carte** que celle vue sur `/tv` et sur l'écran du joueur (`VPlayerPage.jsx` monte le même
`PlayerDisplay`). Seule la **taille** des cartes peut différer d'un appareil à l'autre — jamais
leur **position** ni leur **ordre**. Une formule de colonnes propre à la tablette, ou dépendante
de sa largeur, casserait cette correspondance : c'est l'erreur qu'une première recommandation
(colonnes responsives) avait failli introduire — vue à temps et corrigée avant implémentation.

Extrait **verbatim** de `PlayerDisplay.jsx` (ancien code source de vérité, désormais consommateur
de l'utilitaire au lieu de porter la règle lui-même — extraction pure, comportement TV strictement
inchangé, `PlayerDisplay.test.jsx` : 208/208 PASS sans modification) :

- `buildMemoryCards(question)` — mélange de Fisher-Yates **ensemencé par `question.ID`**
  (`seed = parseInt(question.ID)`, LCG `seed*1103515245+12345`) : ordre déterministe, identique
  sur tous les clients pour une même question, sans coordination réseau.
- `getMemoryGridCols(cardCount)` — nombre de colonnes **fixé par le nombre de cartes uniquement**,
  jamais par la largeur de l'écran : ≤4 → 2 · ≤6 → 3 · ≤16 → 4 · ≤20 → 5 · au-delà → 6.
- `getMemoryGridRows(cardCount, cols)` — `ceil(cardCount / cols)`.

Consommé par **`PlayerDisplay.jsx`** (TV, vue joueur, aperçu régie en iframe — un seul composant
pour les trois) **et** par `AnimMemoryGrid` — **aucune copie**. Seule la taille des cartes
s'adapte à l'écran (unités de conteneur `cqw`/`cqh` côté TV, calcul équivalent côté `/anim`) ; à
toutes les tailles supportées (jusqu'à 24 cartes/6 colonnes), la largeur d'une carte reste
largement au-dessus du minimum tactile de 62 px. Si la hauteur manque, c'est le bloc central de la
conduite qui défile (mécanique #171) — **le défilement ne déplace rien** : la disposition ne
change pas, seule une carte plus bas nécessite un geste pour être vue. Huitième mutualisation de
la série (après `pointsAward.js`, `nextQuestionFormat.js`, `phaseRules.js`, `ardoiseOrder.js`,
`canAwardPoints.js`, le verrou de crédit #170, et `questionTypeMeta.js`) — même motif à chaque
fois : deux surfaces qui doivent dire la même chose, sans qu'une divergence entre elles ne
produise ni erreur ni test rouge, seulement un désaccord silencieux entre animateur et joueurs.

### `AnimMotionGrid` Component (v6.2.0+, #160) — grille MEMOTION tactile en L3 / sous-phases MEMORIZE-GRID

Fichiers : `components/AnimMotionGrid.{jsx,css}` (nouveaux). Grille interactive pour les cinq sous-phases MEMOTION, montée en L3 durant `MEMORIZE` et `GRID`, remplacée par `AnimMotionCard` pour `SELECTED`/`QUESTION`/`REVEAL`.

**Quatre états de carte MEMOTION** :
- **`UNPLAYED` + subphase `GRID`** → cliquable (dégradé violet), sélection possible
- **`UNPLAYED` hors `GRID`** (notamment `MEMORIZE`) → éteinte (opacité réduite), non cliquable
- **`DONE`** → couleur de l'équipe gagnante, non cliquable, nom d'équipe en pied (gris neutre si pas de gagnant)
- **Autres états** → grises/inertes hors phase `STARTED`

**Contenu de carte** : `RECTO_THEME` + étoiles difficulté en mode normal ; coordonnée seule (A1, B2..., sans étoiles) en mode Secret — les étoiles trahiraient la difficulté de la carte.

**Disposition exclusivement via `motionGrid.js`** : colonnes, coordonnées, ordre de cartes (aucun recalcul local).

### `AnimMotionCard` Component (v6.2.0+, #160) — carte zoomée MEMOTION en L3 / sous-phases SELECTED-QUESTION-REVEAL

Fichiers : `components/AnimMotionCard.{jsx,css}` (nouveaux). Montée en L3 à la place de la grille en `SELECTED`, `QUESTION` et `REVEAL`.

**Trois faces** :
- **`SELECTED`** → `RECTO_THEME` + `RECTO_IMAGE` + étoiles + points (`getMotionCardPoints`)
- **`QUESTION`** → `QUESTION_TEXT` + `QUESTION_IMAGE`
- **`REVEAL`** → rappel `QUESTION_TEXT` (atténué) + `ANSWER_IMAGE` + `ANSWER_TEXT` en évidence

**Pas d'animation de flip** : le spectacle est sur `/tv` ; la tablette est un outil de conduite.

### `AnimMotionActions` Component (v6.2.0+, #160) — gestes MEMOTION en L2

Fichiers : `components/AnimMotionActions.{jsx,css}` (nouveaux). Première occupation de L2, rendu depuis `motionRules.motionGestures(...)`.

**Matrice des gestes par sous-phase** :
- **`MEMORIZE`** → bandeau d'attente (décompte) ; pas de bouton
- **`GRID`** → bandeau rappel équipe active ; pas de bouton (sélection via grille)
- **`SELECTED`** → boutons `DÉMARRER` (go, vert) + `ANNULER` (optional, bleu)
- **`QUESTION`** → boutons `STOP CHRONO` (optional) + `RÉVÉLER` (go si chrono ≥0) + `SANS VAINQUEUR` (optional)
- **`REVEAL`** → boutons équipe courante (go, couleur équipe) + `PERSONNE` (optional)

**Palette** : boutons `anim-conduct-btn-{go|optional|danger|off}`, réutilisée depuis `AnimConductPanel` (L1).

### `utils/motionGrid.js` — règle de disposition partagée MEMOTION avec `PlayerDisplay.jsx` (v6.2.0+, #160)

Fichier : `web/src/utils/motionGrid.js` (nouveau). **Motif : la correspondance positionnelle**, identique à `memoryGrid.js` pour MEMORY (#159). Les coordonnées annoncées aux joueurs (A1, B2...) doivent désigner exactement la même carte sur `/anim` et `/tv`.

**Extraction verbatim de `PlayerDisplay.jsx`** (formules copiées sans modification) :
- `getMotionGridCols(cardCount)` — nombre de colonnes : ≤4 → 2 · ≤6 → 3 · ≤12 → 4 · au-delà → 5
- `getMotionGridRows(cardCount, cols)` — `ceil(cardCount / cols)`
- `getMotionCardCoord(index, cols)` — format `"A1"`, `"B2"`, etc.
- `getMotionCardPoints(difficulty, motionConfig)` — barème `POINTS_n_STAR` (1/3/5 pts), repli 1/3/5 si config absent
- `isMotionSecretMode(question)` — détection mode Secret (`MOTION_MEMORIZE_DURATION > 0`)

⚠️ **Divergence assumée** : formule MEMOTION **diffère** de celle de MEMORY (`memoryGrid.js` : 2/3/4/5/6 colonnes). Les deux utilitaires restent séparés — ne pas les fusionner.

Consommé par **`AnimMotionGrid`** (tablette animateur) et **`PlayerDisplay.jsx`** (TV, vue joueur) — aucune copie, **source unique de vérité** pour les coordonnées.

### `utils/motionRules.js` — matrice des gestes MEMOTION (v6.2.0+, #160)

Fichier : `web/src/utils/motionRules.js` (nouveau). Source unique des gestes de L2 en MEMOTION, sur le modèle philosophique de `phaseRules.js` : le composant ne réécrit jamais une condition localement.

`motionGestures(subphase, { timerRunning, currentTeam, currentTeamColor, selectedCardId, cardPoints })` → tableau de gestes `[{ key, label, subLabel, state, action, payload }]` avec `state ∈ go|optional|danger|off`.

**Matrice intégrale** (cf. §`AnimMotionActions`) : chaque sous-phase retourne ses gestes avec états, libellés et actions.

⚠️ **Divergence assumée avec `/admin`** : la régie **masque** les boutons désactivés (`v6.2.0` et antérieurs, absent en #160) ; `/anim` les **éteint** sans les cacher, conforme au « renversement de principe » #166/#171 (emplacements permanents, états calculés).

### `nextQuestionFormat.js` — format partagé "question suivante" (v6.2.0.12, #165)

Fichier : `web/src/utils/nextQuestionFormat.js`. Source unique du format de la prochaine question
(décision GATE 2 D1) :
- `formatNextQuestionStatement(nextQuestion)` → `"#<ID> <type>: <énoncé>"`
- `formatNextQuestionMeta(nextQuestion)` → `"<points>pt <délai>s"`

Depuis #166, consommé uniquement par `AnimNextButton` (la puce "Suivante" du bandeau qui le
consommait aussi depuis #163 a été supprimée). Reste néanmoins l'unique source de ce format —
même piège évité que pour `QCM_COLORS` (#149/#155/#163).

### `questionTypeMeta.js` — icônes par type de question (v6.2.0.15, #166)

Fichier : `web/src/utils/questionTypeMeta.js` (nouveau). `getQuestionTypeMeta(type)` →
`{icon, label}` (⚡ SPEEDY, 🔠 QCM, 🖊️ ARDOISE, 🃏 MEMORY, 🎞️ MEMOTION), repli SPEEDY si type
inconnu. Utilisé par la ligne méta de la zone contexte pour l'icône de type.

### `phaseRules.js` — règles de phase mutualisées `/admin` + `/anim` (v6.2.0.15, #166)

Fichier : `web/src/utils/phaseRules.js` (nouveau). Extrait de `GamePage.jsx` (les prédicats
`canStart`/`isPlaying`/`canReveal` existaient déjà dupliqués à 3 endroits, dont une liste de
phases en dur à `GamePage.jsx:463`) — **extraction pure, `/admin` strictement invariante après ce
refactor** (`GamePage.*.test.jsx` : 78/78 PASS sans modification).

Expose : `canSelectQuestion`/`canStart`/`isPlaying`/`canReveal`/`isRevealed` (miroir de
`engine.go:585-593`), ainsi que les états dérivés pour `/anim` — `startButtonState`/
`pauseButtonState`/`continueButtonState`/`stopButtonState`/`revealButtonState`/
`nextButtonState(phase, question)` — consommés exclusivement par `AnimConductPanel`/
`AnimNextButton`. Chaque fonction est la **source unique** de son état : aucune condition
d'activation n'est réécrite localement dans les composants (point de revue explicite, confirmé
par `code-review-20260815-164335.md` §a).

### Zone équipes (`anim-zone-teams`)

Structure et disposition inchangées par #163/#165/#166/#170 — couleur de réponse QCM (#157). Seul
son `grid-area` change avec la disposition #166 (voir tableau de disposition ci-dessus) ;
`AnimTeamCard.test.jsx` vert sans la moindre modification de DOM/props depuis #155, y compris
après #170 (correctif CSS pur) — **hormis #171**, qui ajoute la prop `medal` et retouche
l'ancrage du bloc de crédit (voir ci-dessous). Cibles tactiles ≥ 62 px (règle #155).

**Crédit de points (v6.2.0.21, #170 — universel v6.2.0.25, #171)** : le bouton de crédit
historique (`anim-team-credit-btn`, #156) est remplacé par le composant `AnimCreditControl` (voir
ci-dessous) sur la carte d'équipe. Cible (équipe entière, ou bumper le plus rapide selon
`POINTS_TARGET` en SPEEDY) et montant (`getTeamAward`, #157) **inchangés** — seul le rendu du
geste et son verrouillage changent. `AnimConductPanel` ne porte aucun point de crédit (vérifié,
zone conduite non concernée).

**Médaille avant le nom (v6.2.0.25, #171, tâche F5)** : `AnimTeamCard` gagne une prop dédiée
`medal` (distincte de `children`, qui ne pouvait pas s'insérer *avant* le nom), rendue dans
l'en-tête de la carte. Rangs 1 à 3 uniquement (🏆🥈🥉). L'en-tête et le bloc `.anim-team-card-extra`
perdaient auparavant `justify-content: space-between` — la position du score et du bloc de crédit
dépendait alors du **nombre d'enfants réellement montés**, un bug d'alignement latent (le score se
retrouvait décalé selon la présence ou non d'une médaille ou d'infos de buzz). Remplacé par un
ancrage **déterministe** : `margin-left: auto` sur `.anim-team-card-score` et sur un nouveau
wrapper `.anim-team-credit-group` (englobant le motif "pas de tentative" + `AnimCreditControl`) —
le score et le bloc de crédit restent au même endroit, avec ou sans médaille, avec ou sans infos
de buzz. `AnimCreditControl` (#170) n'est **pas modifié** : l'ancrage se fait depuis l'extérieur.

**Équipe active et stats MEMORY (v6.2.0.27, #159)** : deux nouvelles props opt-in sur
`AnimTeamCard` — `active` (contour vert `.anim-team-card-active`, équipe dont c'est le tour,
`MEMORY_CURRENT_TEAM`) et `dimmed` (opacité réduite `.anim-team-card-dimmed`) — sans effet pour
les appelants existants (SPEEDY/QCM). Nouvelle classe `.anim-team-memory-stat` : s'affiche pour
**toute** question MEMORY, avec repli sur les compteurs globaux (`memoryMatchedPairs`/
`memoryErrors`) quand l'équipe n'a pas d'entrée propre dans `MEMORY_TEAM_PAIRS`/`TEAM_ERRORS`
(mode SOLO). Le motif "pas de tentative" (§`canAwardPoints.js`, #171) est explicitement **omis**
pour MEMORY — la ligne de stat le dit déjà. Montant de crédit calculé par `getTeamAward` étendu
d'une branche MEMORY (`resolvePointsAward(question, basePoints, {memory: {matchedPairs,
errors}})`, `calcMemoryScore` sous-jacent non modifié) — même montant que `/admin`, jamais
recalculé côté composant.

### `AnimCreditControl` Component (v6.2.0.21, #170) — composant de crédit unique

Fichiers : `components/AnimCreditControl.{jsx,css}` (nouveaux). **Composant unique, seul point
d'application de la règle de verrouillage** — aucun consommateur (carte d'équipe SPEEDY/QCM
aujourd'hui, future liste ARDOISE de #158) ne réimplémente la logique.

- **Deux gestes disponibles tant que l'équipe n'est pas verrouillée** : `"+N pts"` (montant calculé
  en amont, passé en prop, jamais recalculé dans le composant) et **`"0 pt"`** — le refus explicite
  d'une réponse, un crédit ordinaire à montant nul emprunté au même chemin que `"+N pts"`
  (`TEAM_POINTS`/`BUMPER_POINTS`), **sans aucune mécanique d'état local** (pas de
  `sessionStorage`, pas de réinitialisation manuelle à gérer par le composant).
- **État verrouillé** dès qu'une entrée existe pour l'équipe dans `awardedTeams` (state alimenté
  par l'action `AWARDED_TEAMS`, voir `docs/WEBSOCKET_PROTOCOL.md`) — affiche le geste qui a eu
  lieu : `"✓ +N pts"` pour un crédit, littéralement **`"✓ 0 pt"`** pour un refus (jamais
  `"+0 pts"` — c'est le libellé du geste effectué, pas une addition). Sert de confirmation visible
  du crédit à tous les animateurs (F5), dérivée du même payload, sans seconde action serveur.
- ⚠️ **Verrou testé sur la présence de l'entrée, jamais sur la valeur du montant** :
  `if (awardedTeams[team])`, pas `if (awardedTeams[team]?.POINTS)` — ce dernier serait faux pour un
  refus (`0` est falsy en JS) et déverrouillerait silencieusement la ligne, réexposant
  exactement les réponses refusées au double-crédit (risque R1 du plan #170). Voir
  `contracts/websocket-actions.md` §"AWARDED_TEAMS" §"⚠️ `POINTS` peut valoir `0`" pour l'exemple
  de code correct/incorrect.
- **Le verrouillage vient exclusivement du serveur** (présence dans `awardedTeams`), jamais d'une
  anticipation locale du clic — sinon deux sources de vérité coexisteraient et divergeraient au
  premier échec réseau.
- Aucun geste n'est émis en état verrouillé (double garde : bouton non rendu, pas seulement
  désactivé).

**Correctif associé (#170) — `flex-shrink` manquant sur `.anim-team-card`** : la vérification en
conditions réelles a révélé que le texte de confirmation (`"✓ +20 pts"`) était présent dans le DOM
et les styles calculés mais **invisible à l'écran** dès 6+ équipes en phase créditable. Cause :
`.anim-team-card` n'avait pas `flex-shrink: 0` dans `.anim-zone-teams` (flex-column,
`overflow-y: auto`) — flexbox comprimait chaque carte sous la hauteur de son propre contenu au
lieu de laisser le scroll s'enclencher, et le contenu débordant se retrouvait recouvert par la
carte suivante (ordre de peinture du DOM). Corrigé par `flex-shrink: 0` sur `.anim-team-card`.
**Conséquence à noter** : le critère #166/F7 ("six équipes visibles sans défilement" à 1280×800)
ne tenait que grâce à ce bug — l'affichage était silencieusement corrompu au-delà de la limite, pas
conforme. Le scroll interne (`overflow-y: auto`, déjà en place) fonctionne désormais correctement
mais peut être réellement sollicité en phase créditable avec beaucoup d'équipes ; densité des
cartes équipe non retravaillée par ce lot (hors périmètre #170, signalé pour arbitrage futur si
besoin).

### `canAwardPoints.js` — crédit universel par mode (v6.2.0.25, #171)

Fichier : `web/src/utils/canAwardPoints.js` (nouveau). Avant #171, une équipe n'ayant pas tenté de
répondre (pas buzzé en SPEEDY, pas répondu au QCM) ne voyait **aucun** geste de crédit — la
divergence de comportement entre SPEEDY/QCM et ARDOISE (qui proposait déjà "0 pt" à toute équipe
depuis #158/#170) devait être documentée et défendue en recette à chaque nouveau mode. #171
inverse la règle : **`AnimCreditControl` est désormais monté pour toutes les équipes** dès que la
phase l'autorise (`creditEnabled`, inchangé), et `canAwardPoints(question, teamBumpers)` ne décide
plus **si on affiche** le contrôle, seulement **si `"+N pts"` est proposé en plus de `"0 pt"`** :

```js
const amount = canAwardPoints(question, teamBumpers)
  ? getTeamAward(team).amount
  : null
```

`AnimCreditControl` (#170) rendait déjà "0 pt" **inconditionnellement** et "+N pts" seulement si
`amount != null` — la découverte qui a simplifié ce lot est qu'**aucune modification du composant
n'était nécessaire**, il suffisait de lui passer `amount = null` au lieu de le masquer entièrement.

**Règle par mode** :

| Mode | `"+N pts"` proposé si… | `"0 pt"` |
|---|---|---|
| SPEEDY | au moins un bumper de l'équipe a `TIME > 0` (même test que `getTeamQcmAnswer`/`handleCredit`) | toujours |
| QCM | idem (a répondu) | toujours |
| ARDOISE | géré par `AnimArdoiseList` (#158), inchangé | toujours |
| Autre type (MEMORY, MEMOTION…) | **toujours** — défaut permissif | toujours |

Le défaut permissif pour un type inconnu est un choix délibéré : une condition stricte bloquerait
silencieusement un mode le jour de son arrivée, sans erreur ni log — testé explicitement
(`canAwardPoints.test.js`).

Une équipe **sans tentative** affiche un motif discret (`.anim-team-no-attempt`, "pas de buzz" /
"pas de réponse" selon le type) **à côté** du geste de crédit, jamais à sa place.

⚠️ **Ne pas confondre créditabilité et verrouillage #170** : une équipe qui n'a rien tenté mais que
la régie a déjà créditée (`/admin` n'a aucune garde) reste **verrouillée avec son montant** —
`awardedTeams` (le verrou serveur de #170) reste l'unique source de vérité pour l'état verrouillé,
indépendante de `canAwardPoints`. Les deux logiques sont orthogonales : l'une décide ce qui est
*proposé* avant tout crédit, l'autre ce qui est *déjà arrivé*.

### Mode ARDOISE sur `/anim` — `AnimArdoiseList` Component (v6.2.0.24, #158)

Fichiers : `components/AnimArdoiseList.{jsx,css}` (nouveaux). En question de type `ARDOISE`
(phases `STARTED`/`PAUSED`/`STOPPED`/`REVEALED`), la colonne équipes **bascule automatiquement**
de `AnimTeamCard` vers `AnimArdoiseList` — `AnimTeamCard` reste strictement inchangée pour tous
les autres types de question (SPEEDY, QCM). Filtre équipes à joueur virtuel
(`bumper.IS_VPLAYER`), parité avec `/admin` (#93) — pas "au moins un joueur" comme `displayTeams`.

Affiche les copies (réponses écrites) **en direct** pendant la frappe, une ligne par équipe, dans
l'ordre de première saisie (rang 1/2/3...). Tri et calcul du délai affiché mutualisés avec
`/admin` via `utils/ardoiseOrder.js` (nouveau, extrait à l'identique de `GamePage.jsx`, aucune
règle dupliquée) : `formatArdoiseDelay` et `sortArdoiseEntries`. Copie longue : `overflow-wrap:
anywhere`, **jamais de troncature**.

**Trois états de ligne** (pas cinq — la synchronisation, autrefois envisagée comme un mécanisme
propre à ARDOISE dans une révision antérieure du plan, est intégralement portée par le composant
partagé de #170, sans rien de spécifique à ARDOISE) :

| État | Condition | Rendu |
|---|---|---|
| **A répondu, pas créditée** | Réponse reçue, `awardedTeams[team]` absent | Rang, délai, copie, montant via `calcArdoiseDefaultPoints` (mirror exact de `/admin`) → `"+N pts"`/`"0 pt"` |
| **Créditée** | `awardedTeams[team]` présent | État verrouillé rendu **entièrement** par `AnimCreditControl` (#170) — la liste ne porte aucune logique de crédit ou de verrouillage propre |
| **Sans réponse** | Aucune copie reçue | Pas de rang/délai, "Aucune copie" en pointillés, `amount=null` → seul `"0 pt"` proposé (déjà géré par #170, rien de dupliqué ici) |

**Gestes de crédit visibles uniquement à partir de la phase `REVEALED`** (`showCredit = revealed
|| awarded`) — comme le bouton ARDOISE historique de `/admin`, **écart assumé avec SPEEDY/QCM**
qui autorisent aussi le crédit dès `STOPPED`. Une ligne déjà verrouillée reste affichée quelle que
soit la phase suivante. Ce gate est à la charge de l'**appelant** (`AnimArdoiseList`), pas
d'`AnimCreditControl` lui-même : le composant partagé ne connaît que `team`/`amount`/`awarded`/
`onCredit`, jamais la phase — exactement le même schéma que `creditEnabled` pour SPEEDY/QCM.

Crédit ARDOISE cible **toujours l'équipe entière** (`setTeamPoints` direct, mirror exact de
`/admin`) — jamais un joueur individuel comme peut le faire SPEEDY.

`.anim-ardoise-row` porte le même correctif `flex-shrink: 0` que `.anim-team-card` (#170,
appliqué d'emblée pour éviter la même invisibilité de confirmation de crédit).

### Bande régie (`anim-zone-regie`) — réservée (v6.2.0.15, #166)

Bande pleine largeur en bas de page, hauteur fixe (~42 px), vide, sans interaction, sans état,
sans contrat — emplacement préparé pour la messagerie régie (#167), non implémentée. Signalé pour
que #167 ne prenne pas cet emplacement pour une décision de conception déjà prise (fil persistant
vs toast, par exemple) : c'est un espace vide, retirable pour le coût d'une règle CSS.

### Tenue sans scroll (#166 — nuancé #170 — re-vérifié et étendu à 1024×600 par #171)

`/anim` reste en `overflow: hidden` sur la **page**. Deux zones ont leur propre défilement
**interne**, jugé acceptable car il ne concerne jamais la page entière : la colonne équipes
(`overflow-y: auto`, inchangé depuis #155) et, depuis #171, le bloc central de la conduite
(`.anim-conduct-mid`, L2/L3/L4 — voir §"Zone conduite" ci-dessus). Ce qui ne doit **jamais**
arriver : que L1 ou L5 (les deux extrémités fixes de la conduite) sortent de l'écran.

**La colonne équipes** — le critère #166/F7 ("au moins 6 cartes équipe visibles sans défilement" à
1280×800) reposait en réalité sur le bug `flex-shrink` corrigé par #170 (voir §"`AnimCreditControl`
Component" ci-dessus) : l'affichage semblait tenir sans scroll parce que le contenu débordant était
silencieusement recouvert, pas parce qu'il tenait réellement. Le scroll interne fonctionne
désormais correctement et peut être sollicité en phase créditable avec plusieurs équipes — ce
n'est pas une régression, c'est la correction d'un affichage auparavant corrompu.

**La conduite (#171)** — vérifiée en conditions réelles à **1280×800, 1024×768 ET 1024×600**
(mesures géométriques via CDP `Runtime.evaluate`, pas seulement des captures) : `pageOverflows:
false` aux trois résolutions. Cas de contrôle explicite — une note L4 artificiellement longue
(texte injecté par script, simulant #168) — confirme `l1FullyVisible: true`,
`l5FullyVisible: true`, `pageScrollHeight === pageClientHeight` : le débordement reste
intégralement confiné à `.anim-conduct-mid`, jamais à la page. **1024×600 est donc désormais
vérifié en conditions réelles** (mesures géométriques CDP, dev-frontend,
`_work/handoff/dev-frontend-20260816-194004.md` — QA n'a pas eu besoin de rejouer ce point,
extension Chrome interactive non requise pour une mesure programmatique), levant la réserve
précédemment ouverte sur cette résolution pour la zone conduite — les autres réserves de la série
(flou/QCM_HINT/confirmation pixel/etc., qui nécessitent un œil humain) restent, elles, ouvertes.

## Layout GamePage (Admin) - v2.12.0

Layout avec timer pleine largeur + 3 colonnes harmonisées :
```
| Timer (pleine largeur, 95%)                    |  ligne 1
|------------------------------------------------|
| Questions | Contrôles + Aperçu TV | Équipes    |  ligne 2
| 280px     | 1fr (flexible)        | 280px      |
```

- **max-width** : 1800px (pour exploiter les grands écrans)
- **Breakpoints** : 1600px (250px), 1400px (220px), 1200px, 768px
- **Colonnes harmonisées** : Questions et Équipes ont la même largeur

**Responsive breakpoints:**
- `>1600px`: 3 columns (280px / 1fr / 280px)
- `1400-1600px`: 3 columns (250px / 1fr / 250px)
- `1200-1400px`: 3 columns (220px / 1fr / 220px)
- `768-1200px`: 2 columns (questions + controls / teams)
- `<768px`: 1 column (stacked)

## Affichage TV - Contrainte IMPORTANTE

**L'affichage TV (`/tv`) est STATIQUE et ne permet PAS de scroll.**
Toutes les vues TV doivent tenir entièrement à l'écran sans défilement :
- Utiliser `overflow: hidden` (jamais `auto` ou `scroll`)
- Dimensionner avec des unités viewport (`vh`, `vw`, `%`)
- Utiliser `flex` avec `min-height: 0` pour permettre le rétrécissement
- Limiter le contenu visible (ex: top 3, max 6 catégories)

### Vues TV disponibles (v2.34.0)

| Vue | Action REMOTE | Description |
|-----|---------------|-------------|
| JEU | `GAME` | Question, timer, réponses QCM |
| EQUIPES | `SCORE` | Podium des équipes (top 3) |
| JOUEURS | `PLAYERS` | Liste des joueurs par équipe |
| PALMARES | `PALMARES` | Classement par catégorie (grille 3x2, max 6 catégories) |

### PlayerDisplay 4-Zone Layout (v2.11.1)

Layout vertical en 4 zones avec hauteurs fixes pour l'affichage TV (/tv) :
- **Zone 1 - Timer** : 100px hauteur fixe, centré en haut
- **Zone 2 - Question** : 80px hauteur fixe, texte de la question
- **Zone 3 - Media** : flex: 1, remplit l'espace restant, image centrée
- **Zone 4 - Answers** : 120px hauteur fixe, `margin-top: auto` (aligné en bas)

**Timer couleur synchronisée :**
- Vert (`--success`) : > 50% du temps restant
- Orange (`--warning`) : 25-50% du temps (urgent)
- Rouge (`--error`) : < 25% du temps (critique)

**Affichage des phases de jeu (v2.40.0) :**

| Phase | Affichage TV | Description |
|-------|--------------|-------------|
| PREPARE | "NOUVELLE QUESTION" | Centré à l'écran, pas de catégorie |
| READY | Icône catégorie + Nom (fond coloré) | Grande icône animée (pulsante) |
| COUNTDOWN | Catégorie en haut + Décompte au centre | Animation de la catégorie du centre vers le haut |
| STARTED | Question + Média + Réponses | Affichage normal du jeu |

### Panneau ARDOISE - Grille Responsive (v5.8.2)

Affichage des réponses ARDOISE en grille responsive sur la page d'administration (GamePage).

**Disposition en grille :**
- **Conteneur** : `display: grid` avec `grid-template-columns: repeat(auto-fill, minmax(178px, 1fr))`
- **Auto-fill** : Génère automatiquement le nombre de colonnes selon la largeur disponible
- **Largeur minimale** : 178px par cellule (optimisé pour lisibilité rang + texte réponse)
- **Gap** : Espacements uniformes entre cellules

**Réduction de hauteur observée :**
- 6 équipes → 2 rangées (au lieu d'une colonne verticale précédente)
- 16 équipes → 4 rangées (gain significatif de hauteur du panneau)

**Rang (classement) :**
- **Affichage** : En-tête de chaque cellule avec fond plein (`rgba(99, 102, 241, 0.9)`)
- **Texte** : Blanc, taille légèrement augmentée pour lisibilité
- **Première réponse** : Encadrée visuellement avec bordure + `box-shadow` inset accentués
- **Données** : Rang issu du tri chronologique (#117) des arrivées de réponses

**Texte de réponse :**
- **Sans troncature** : `overflow-wrap: anywhere` permet le retour à la ligne intégral
- **Longues chaînes** : Même sans espaces, se distribuent sur plusieurs lignes sans déborder
- **Alignement vertical** : `.ardoise-answer-text-row` en `align-items: flex-start` pour que le bouton d'attribution reste ancré en haut

**Délai** : Conservé depuis #117, format milliseconde non modifié — visible dans le texte réponse

## VPlayer - Joueurs Virtuels (v2.45.0)

Permet aux joueurs de buzzer depuis leur smartphone en scannant un QR Code.

**Workflow :**
1. Admin ouvre `/anim/teams` → Zone "Inscriptions" → "Lancer Inscriptions"
2. QR Code s'affiche sur `/tv`
3. Joueurs scannent → arrivent sur `/` → saisissent pseudo → redirigés vers `/player`
4. Admin ferme inscriptions → QR Code disparaît, joueurs voient page d'attente
5. Joueurs sur `/player` peuvent buzzer pendant les questions
6. Si admin supprime un joueur → détection automatique → redirection vers `/`

**Page d'inscription (`/` - EnrollPage) :**
- Fond blanc pour lisibilité
- Si inscriptions fermées : "En attente de l'ouverture des inscriptions..."
- Formulaire : pseudo (2-20 caractères) + bouton "Rejoindre"
- Reconnexion auto : si joueur existe côté serveur → redirige vers `/player`
- Stockage localStorage : `vplayer_name`, `vplayer_session`

**Page de jeu (`/player` - VPlayerPage) :**

Layout responsive en 4 zones avec badges permanents non-intrusifs.

**BuzzButton États visuels :**

| Phase | Texte | Couleur | État |
|-------|-------|---------|------|
| NOT_ASSIGNED | "En attente..." | Gris | disabled |
| STOPPED | "En attente de question" | Gris | disabled |
| PREPARE | "Préparation..." | Orange | disabled |
| READY / COUNTDOWN | "Prêt !" | Cyan | disabled |
| STARTED | "BUZZ !" | Vert | active |
| PAUSED | "Déjà buzzé" | Bleu | disabled |

**Retour haptique :**
- Vibration 100ms au clic (si `navigator.vibrate` supporté)
- Animation visuelle pressing (300ms scale)

**Protection MEMORY :**
- Questions MEMORY ne peuvent pas être buzzées par VPlayers
- `engine.go:ProcessButtonPress()` ignore les buzz si TYPE="MEMORY"

**Actions WebSocket VPlayer :**
| Action | Direction | Description |
|--------|-----------|-------------|
| SHOW_QR_CODE | Admin→Server | Démarre enrollment |
| HIDE_QR_CODE | Admin→Server | Arrête enrollment |
| PLAYER_CONNECT | VPlayer→Server | Demande d'inscription |
| PLAYER_CONNECTED | Server→VPlayer | Inscription réussie |
| PLAYER_REJECTED | Server→VPlayer | Inscription refusée |
| ENROLLMENT_UPDATE | Server→All | Mise à jour compteur |

## Fonctionnalités UI

### Statuts des questions (couleurs)

| Statut | Couleur bordure | Fond | Apparence |
|--------|-----------------|------|-----------|
| AVAILABLE | Vert | Vert clair | Normal |
| STARTED | Orange | Orange clair | Normal |
| STOPPED | Rouge | Rouge clair | Normal |
| REVEALED | Gris | Gris | Compact (image/réponse masquées, opacité 50%) |

### Timer Phase Badges (v2.10.0)

Pastilles colorées indiquant l'état du jeu dans le composant Timer :
- **ARRET** (STOPPED) : Rouge (`--error`)
- **PREPARATION** : Orange (`--warning`)
- **PRET** : Cyan (`--accent-cyan`)
- **EN COURS** : Vert (`--success`)
- **PAUSE** : Bleu (`--primary-500`)
- **REPONSE** (REVEALED) : Gris (`--gray-400`)

### Points Animation (v2.12.0)

Animation visuelle quand des points sont ajoutés :
- **Confetti** : Particules avec la couleur de l'équipe
- **Animation flottante** : Nom de l'équipe + "+X pts" au centre de l'écran
- **Durée** : 2.5 secondes puis disparition
- **Vue JOUEURS** : Animation sur la ligne du joueur (scale + couleur verte)

### Debug Features (v2.12.0)

Fonctionnalités de test pour l'admin :
- **Ctrl+clic sur joueur** : Simule un appui buzzer (pendant STARTED/PAUSED)
- **Ctrl+clic sur question** : Force l'état READY sans attendre les PONGs

### Waiting States (v2.12.0)

États visuels pour équipes/joueurs :
- **PREPARE/READY** : Grisés jusqu'à réception du PONG
- **STARTED/PAUSED** : Grisés jusqu'au buzz
- **Après buzz** : Visibilité restaurée avec couleur d'équipe

### Question Reordering (v2.7.0)

Drag and drop pour réordonner les questions :
- **Interface** : Glisser-déposer les cartes de questions dans QuestionsPage
- **Poignée** : Icône sur chaque carte pour indiquer le drag
- **Feedback visuel** : Opacité réduite pendant le drag, bordure pointillée sur la cible
- **Persistance** : Champ `ORDER` dans chaque `question.json`
- **Tri** : Questions triées par `ORDER` si disponible, sinon par `ID`

### Teams Page - Drag & Drop (v2.5.0)

Interface de gestion des équipes avec drag & drop :
- **Gauche** : Grille des équipes (zones de dépôt)
- **Droite (320px)** : Joueurs non assignés
- **Drag & Drop** : Glisser un joueur sur une équipe pour l'assigner
- **Désassigner** : Glisser vers la zone "non assignés"

### Couleurs de Réponse (v2.5.0)

Chaque joueur peut avoir une couleur de réponse pour le mode QCM :
- **Couleurs disponibles** : Rouge (A), Vert (B), Jaune (C), Bleu (D)
- **Sélection** : Uniquement quand le joueur n'est PAS assigné à une équipe
- **Affichage** : La couleur devient le fond de l'avatar du joueur
- **Champ** : `ANSWER_COLOR` dans le modèle Bumper

### QCM Team Badges (v2.16.0)

Pastilles d'équipes sur les réponses QCM pendant STOPPED/REVEALED :
- **Affichage** : Pastilles colorées sur chaque réponse QCM montrant quelles équipes ont répondu
- **Couleur** : Couleur de l'équipe (pas la couleur de la réponse QCM)
- **Taille dégradée** : 70% (première) à 40% (dernière) de la taille de base (60px)
- **Tri** : Par temps de réponse (plus rapide = plus grand, à gauche)

### Media Answer (v2.14.0)

Support des images de réponse distinctes de l'image de question :
- **MEDIA** : Image affichée pendant les phases STARTED et PAUSED
- **MEDIA_ANSWER** : Image de réponse qui REMPLACE MEDIA pendant la phase REVEALED
- **Effet visuel** : Cadre vert pulsant autour de l'image de réponse pendant REVEALED
- **Thumbnails** : Vignette de l'image réponse affichée en bas à droite des cartes questions

### CategoryBalance Component (v2.23.0)

Visualisation de l'équilibre des catégories sur la page Questions :
- **Affichage** : Barre horizontale avec toutes les catégories représentées
- **Données par catégorie** : Nombre de questions, Total des points
- **Code couleur** :
  - <= 25% écart : Vert (Équilibré)
  - 25% - 50% écart : Orange (Attention)
  - > 50% écart : Rouge (Déséquilibré)

### History Page (v2.20.0)

Page d'historique des événements de jeu :
- **Route** : `/history-page`
- **Endpoint API** : `GET /history` retourne `[]GameEvent`
- **Fonctionnalités** :
  - Événements groupés par question (ordre chronologique)
  - Vue collapsible : clic sur l'en-tête pour ouvrir/fermer
  - Boutons "Tout ouvrir" / "Tout fermer"
  - **Vue réduite** : Résumé des points par équipe et par joueur (badges colorés)
  - **Vue détaillée** : Tableau avec Heure, Équipe, Joueur, Temps, Points

### Logs Page (v2.42.0)

Page de visualisation des logs serveur en temps réel :
- **Route** : `/admin/logs` et `/anim/logs`
- **Fonctionnalités** :
  - Affichage temps réel des logs via WebSocket dédiée
  - Filtrage par niveau : DEBUG (gris), INFO (blanc), WARN (orange), ERROR (rouge)
  - Filtrage par composant : App, Engine, HTTP, WebSocket, TCP, UDP
  - Recherche avec debounce 300ms et highlight des termes
  - Auto-scroll intelligent (pause au scroll manuel, reprise en bas)
  - Export des logs filtrés au format `.log`

## WebSocket Messages

Le hook `useWebSocket.js` gère la communication :

| Message reçu | Données | Utilisation |
|--------------|---------|-------------|
| UPDATE | `{GAME, teams, bumpers}` + VERSION | État du jeu + version serveur |
| QUESTIONS | `{questions}` + FSINFO + VERSION | Liste questions + espace disque |
| CLIENTS | `{ADMIN_COUNT, TV_COUNT, VPLAYER_COUNT}` | Compteurs clients connectés |
| BACKGROUND_CHANGE | `{INDEX}` | Index de l'image de fond courante (synchronisé) |

## CSS Specificity & Layout Fixes (v2.32.0)

**Problème résolu** : Conflits CSS entre GamePage.css et TeamsPage.css sur les mêmes classes `.teams-grid` et `.team-card`.

**Solution - Sélecteurs spécifiques** :
Les styles de GamePage utilisent des sélecteurs plus spécifiques pour éviter que TeamsPage.css ne les écrase :

```css
/* GamePage.css - Sélecteurs spécifiques à la page Jeu */
.game-page .teams-grid { display: flex; flex-direction: column; ... }
.game-page .teams-grid .team-card { overflow: visible; flex-shrink: 0; ... }
.game-page .team-card .team-buzzers { display: flex !important; ... }
.game-page .team-card .buzzer-mini { display: flex !important; ... }
```

---

## Architecture v7.0.0 — Délégation L2/L3 par hôte et sevrage de `phase` (#184, #185)

### Vocabulaire et contexte

L'architecture de v7.0.0 introduit deux concepts clés :

1. **Hôte** : ce qui fait tourner une manche d'un type. Deux hôtes existent :
   - **Hôte question** : le cycle `GamePhase` classique (`PREPARE`, `READY`, `STARTED`, `REVEALED`…)
   - **Hôte carte MEMOTION** : la sous-machine `MEMOTION_SUBPHASE` (`MEMORIZE`, `GRID`, `SELECTED`, `QUESTION`, `REVEAL`)

2. **Délégation par type** : dispatcher sur le **type du contenu actif** (type de question OU type de carte active) pour monter le composant d'affichage et d'édition approprié.

### Délégation L2/L3 généralisée par hôte

#### Avant v7.0.0

Composants et éditeurs pivotaient sur la **phase** ou le **type de question direct** :

```jsx
// AnimConductPanel.jsx — L2 et L3 pivotaient sur des conditions de phase
export function AnimConductPanel({ gameState }) {
  const question = gameState.question;
  
  // L2 : gestes du mode — branche sur phase unique
  let modeGestures = null;
  if (gameState.MEMOTION_SUBPHASE === 'SELECTED') {
    modeGestures = <AnimMotionActions subphase="SELECTED" />;
  }
  
  // L3 : contenu de la question — branche sur type
  let questionContent = null;
  if (question?.TYPE === 'QCM') {
    questionContent = <AnimQcmOptions answers={question.QCM_ANSWERS} revealed={phase === 'REVEALED'} />;
  } else if (question?.TYPE === 'MEMORY') {
    questionContent = <AnimMemoryGrid phase={phase} />;
  } else if (question?.TYPE === 'MEMOTION') {
    questionContent = <AnimMotionGrid subphase={gameState.MEMOTION_SUBPHASE} />;
  }
  
  return (
    <div>
      <L1>...</L1>
      {modeGestures}
      {questionContent}
    </div>
  );
}
```

**Problèmes** :
- Composants de type reçoivent `phase` directement
- Deux composants identiques (`AnimQcmOptions` pour question + carte MEMOTION) doivent avoir deux implémentations (variantes)
- Logique d'attribution dispersée entre le composant et ses parents
- Ajout d'une sous-machine (`MEMOTION_SUBPHASE`) brise la cohérence : `revealed` vaut `phase === 'REVEALED'` côté question, mais `MEMOTION_SUBPHASE === 'REVEAL'` côté carte — deux expressions différentes pour la même notion

#### Après v7.0.0

Délégation **par le type du contenu actif** — question OU carte MEMOTION. L'adaptateur « carte → question synthétique » résout le descripteur du type et copie les champs de même nom :

```jsx
// utils/hostContext.js — contexte d'hôte normalisé
export function resolveHostContext(gameState) {
  const { PHASE, MEMOTION_SUBPHASE, MEMOTION_SELECTED, timer } = gameState;
  
  if (MEMOTION_SUBPHASE && MEMOTION_SELECTED) {
    // Hôte carte MEMOTION
    const card = gameState.MEMOTION_CARDS.find(c => c.ID === MEMOTION_SELECTED);
    return {
      hostType: 'MOTION_CARD',
      cardID: MEMOTION_SELECTED,
      playable: MEMOTION_SUBPHASE === 'QUESTION',
      revealed: MEMOTION_SUBPHASE === 'REVEAL',
      timerRunning: MEMOTION_SUBPHASE === 'QUESTION' && timer > 0,
      type: card?.TYPE || 'SPEEDY',
    };
  }
  
  // Hôte question (par défaut)
  return {
    hostType: 'QUESTION',
    cardID: '',
    playable: PHASE === 'STARTED',
    revealed: PHASE === 'REVEALED',
    timerRunning: PHASE === 'STARTED' && timer > 0,
    type: gameState.question?.TYPE || 'SPEEDY',
  };
}

// utils/typeState.js — résolution de l'état par type
export function getTypeState(gameState, hostContext) {
  if (hostContext.cardID) {
    // Hôte carte : état dans MEMOTION_ACTIVE.STATE
    return gameState.MEMOTION_ACTIVE?.STATE || {};
  }
  // Hôte question : état question-scopé (champs plats de gameState)
  return {
    qcmInvalidated: gameState.QCM_INVALIDATED,
    ardoiseAnswers: gameState.ARDOISE_ANSWERS,
    memoryFlipped: gameState.MEMORY_FLIPPED_CARDS,
    // …
  };
}
```

**Adaptateur « carte → question synthétique »** dans `AnimPage.jsx` :

```jsx
// AnimPage.jsx — montage de L3
function resolveL3Content(gameState, hostContext) {
  let sourceObject = gameState.question;
  
  if (hostContext.cardID) {
    // Hôte carte : synthétiser une question de la carte
    const motionCard = gameState.MEMOTION_CARDS.find(c => c.ID === hostContext.cardID);
    sourceObject = {
      TYPE: hostContext.type,
      QUESTION: motionCard.RECTO_THEME, // adaptateur de champs
      QCM_ANSWERS: motionCard.QCM_ANSWERS,
      QCM_CORRECT: motionCard.QCM_CORRECT,
      ANSWER: motionCard.ANSWER_TEXT,
      // … copie des champs de même nom
    };
  }
  
  // Dispatch par type unique, même sourceName
  const typeDescriptor = getQuestionTypeMeta(hostContext.type);
  
  switch (hostContext.type) {
    case 'QCM':
      return (
        <AnimQcmOptions 
          answers={sourceObject.QCM_ANSWERS}
          correct={sourceObject.QCM_CORRECT}
          playable={hostContext.playable}
          revealed={hostContext.revealed}
        />
      );
    case 'MEMORY':
      return (
        <AnimMemoryGrid
          pairs={sourceObject.MEMORY_PAIRS}
          playable={hostContext.playable}
          revealed={hostContext.revealed}
        />
      );
    default:
      return null;
  }
}
```

**Avantages** :
- Composants de type ne voient **jamais** `PHASE` ni `MEMOTION_SUBPHASE`
- Même composant `AnimQcmOptions` montable dans les deux hôtes sans variante
- Logique centralisée dans `resolveHostContext` et `getTypeState`
- Ajout d'un nouveau type affecte **uniquement** : son registre (§7), ses propres composants et handlers

### Sevrage de la prop `phase`

#### Avant v7.0.0

Composants de type recevaient `phase` directement :

```jsx
// AnimQcmOptions.jsx — avant
export function AnimQcmOptions({ answers, correct, phase, invalidated }) {
  return (
    <div className="qcm-grid">
      {answers.map((ans, idx) => (
        <div
          key={idx}
          className={phase === 'REVEALED' ? 'revealed' : 'hidden'}
        >
          {phase === 'REVEALED' && <Checkmark />}
          {ans}
        </div>
      ))}
    </div>
  );
}

// AnimMemoryGrid.jsx — avant
export function AnimMemoryGrid({ pairs, phase }) {
  const [flipped, setFlipped] = useState({});
  
  const handleFlip = (idx) => {
    if (phase !== 'STARTED') return; // condition de phase directe
    setFlipped(prev => ({ ...prev, [idx]: !prev[idx] }));
  };
  
  return pairs.map((pair, idx) => (
    <div
      key={idx}
      className={phase === 'REVEALED' ? 'revealed' : 'hidden'}
      onClick={() => handleFlip(idx)}
    >
      {flipped[idx] ? pair.TEXT : '?'}
    </div>
  ));
}
```

**Problèmes** :
- Deux vérifications différentes pour la même notion : `phase === 'REVEALED'` côté question vs `MEMOTION_SUBPHASE === 'REVEAL'` côté carte
- Impossibilité de monter le même composant dans les deux hôtes : il fallait une variante par hôte
- Trop couplé à l'implémentation côté serveur (`GamePhase` vs `MEMOTION_SUBPHASE`)

#### Après v7.0.0

Remplacé par le triplet normalisé `playable`, `revealed`, `timerRunning` (+ `CardID` implicite via `HostContext`) :

```jsx
// AnimQcmOptions.jsx — après
export function AnimQcmOptions({ answers, correct, playable, revealed, invalidated }) {
  return (
    <div className="qcm-grid">
      {answers.map((ans, idx) => (
        <div
          key={idx}
          className={revealed ? 'revealed' : 'hidden'}
        >
          {revealed && <Checkmark />}
          {ans}
        </div>
      ))}
    </div>
  );
}

// AnimMemoryGrid.jsx — après
export function AnimMemoryGrid({ pairs, playable, revealed }) {
  const [flipped, setFlipped] = useState({});
  
  const handleFlip = (idx) => {
    if (!playable) return; // même notion dans les deux hôtes
    setFlipped(prev => ({ ...prev, [idx]: !prev[idx] }));
  };
  
  return pairs.map((pair, idx) => (
    <div
      key={idx}
      className={revealed ? 'revealed' : 'hidden'}
      onClick={() => handleFlip(idx)}
    >
      {flipped[idx] ? pair.TEXT : '?'}
    </div>
  ));
}
```

**Hook `useHostContext`** — wrapper optionnel pour accéder au contexte d'hôte :

```jsx
// hooks/useHostContext.js
export function useHostContext() {
  const gameState = useContext(GameContext);
  return resolveHostContext(gameState);
}

// Composant d'affichage
export function AnimQcmDisplay() {
  const hostContext = useHostContext();
  const typeState = getTypeState(gameState, hostContext);
  
  return (
    <AnimQcmOptions
      answers={question.QCM_ANSWERS}
      correct={question.QCM_CORRECT}
      playable={hostContext.playable}
      revealed={hostContext.revealed}
      invalidated={typeState.qcmInvalidated}
    />
  );
}
```

**Effet** :
- `AnimQcmOptions`, `AnimMemoryGrid`, `AnimMotionGrid` etc. fonctionnent de manière identique dans les deux hôtes
- Triplet `playable`/`revealed`/`timerRunning` **entièrement dérivé** à chaque render (pas de nouvel état, pas de cache)
- Aucune condition sur `PHASE` ou `MEMOTION_SUBPHASE` à l'intérieur des composants de type
- Nouveaux types hériteront automatiquement du triplet sans variante par hôte

### Table de dérivation du contexte d'hôte

Source unique : **`contracts/question-types.md` §4**.

| Hôte | `playable` | `revealed` | `timerRunning` |
|---|---|---|---|
| **Question** | `PHASE == STARTED` | `PHASE == REVEALED` | `PHASE == STARTED && CURRENT_TIME > 0` |
| **Carte MEMOTION** | `MEMOTION_SUBPHASE == QUESTION` | `MEMOTION_SUBPHASE == REVEAL` | `MEMOTION_SUBPHASE == QUESTION && CURRENT_TIME > 0` |
| **Aucun** (GRID, MEMORIZE, SELECTED, PREPARE, READY…) | `false` | `false` | `false` |

### Migration de composants existants

**Checklist pour chaque composant recevant `phase`** :

1. Remplacer `phase` par `playable`/`revealed`/`timerRunning`
2. Remplacer `if (phase === 'REVEALED')` par `if (revealed)`
3. Remplacer `if (phase === 'STARTED')` par `if (playable)`
4. Remplacer les autres vérifications de phase par leur équivalent du triplet
5. Vérifier que le composant n'importe pas `PHASE` ou `MEMOTION_SUBPHASE` de `gameState`
6. Tester en question **et** en carte MEMOTION du même type (s'il existe)

**Exemple concret** — migration de `AnimArdoiseList` :

```jsx
// Avant — recevait `phase` et branchiait dessus
export function AnimArdoiseList({ entries, phase, onCredit }) {
  const showCredit = phase === 'REVEALED' || phase === 'STOPPED'; // deux phases
  
  return entries.map(entry => (
    <div>
      <span>{entry.text}</span>
      {showCredit && <CreditButton amount={entry.points} />}
    </div>
  ));
}

// Après — reçoit `revealed`
export function AnimArdoiseList({ entries, revealed, awarded, onCredit }) {
  const showCredit = revealed || awarded;
  
  return entries.map(entry => (
    <div>
      <span>{entry.text}</span>
      {showCredit && <CreditButton amount={entry.points} />}
    </div>
  ));
}

// Appel (AnimPage.jsx) — plus besoin de transmettre `phase` directement
<AnimArdoiseList 
  entries={entries}
  revealed={hostContext.revealed}
  awarded={awarded}
  onCredit={handleCredit}
/>
```

### Éditeurs de type dans `QuestionsPage.jsx`

Les formulaires de question et de carte MEMOTION utilisent la même approche : **sous-éditeur monté selon le type, reçoit les champs typés**.

```jsx
// QuestionsPage.jsx — dispatcher sur le type
export function QuestionEditor({ question, onChange }) {
  const { TYPE = 'SPEEDY' } = question;
  
  return (
    <div>
      <TypeSelector
        value={TYPE}
        onChange={(newType) => onChange({ ...question, TYPE: newType })}
        locked={isTypeLockedByContent(question)}
      />
      
      {/* Sous-éditeur par type — reçoit seulement les champs pertinents */}
      {TYPE === 'QCM' && (
        <QcmEditor
          answers={question.QCM_ANSWERS}
          correct={question.QCM_CORRECT}
          hints={question.QCM_HINTS_ENABLED}
          onChange={(update) => onChange({ ...question, ...update })}
        />
      )}
      
      {TYPE === 'ARDOISE' && (
        <ArdoiseEditor
          keyboardType={question.ARDOISE_KEYBOARD_TYPE}
          onChange={(update) => onChange({ ...question, ...update })}
        />
      )}
      
      {/* … autres types */}
    </div>
  );
}

// Éditeur de carte MEMOTION — même dispatch
export function MotionCardEditor({ card, onChange }) {
  const { TYPE = 'SPEEDY' } = card;
  
  return (
    <div>
      <TypeSelector
        value={TYPE}
        onChange={(newType) => onChange({ ...card, TYPE: newType })}
        locked={isMotionCardTypeLockedByContent(card)}
      />
      
      {/* Même sous-éditeur, montable pour question ET carte */}
      {TYPE === 'QCM' && (
        <QcmEditor
          answers={card.QCM_ANSWERS}
          correct={card.QCM_CORRECT}
          hints={card.QCM_HINTS_ENABLED}
          onChange={(update) => onChange({ ...card, ...update })}
        />
      )}
      
      {/* … autres types */}
    </div>
  );
}
```

**Spécificité de `MotionCard`** — le type est optionnel (absent = `SPEEDY`) et verrouillé quand du contenu propre au type est saisi :

```jsx
function isMotionCardTypeLockedByContent(card) {
  const { TYPE = 'SPEEDY' } = card;
  const descriptor = getTypeDescriptor(TYPE);
  
  // Verrouillé si au moins un champ `OwnedFields` s'écarte de sa valeur de création
  return descriptor.OwnedFields.some(field => {
    const currentValue = card[field];
    const creationValue = getFieldCreationValue(TYPE, field);
    return !deepEqual(currentValue, creationValue);
  });
}
```

Voir **`contracts/question-types.md` §3.2** pour la table complète des valeurs de création par type.

### Synthèse des bénéfices

| Aspect | Avant v7.0.0 | Après v7.0.0 |
|---|---|---|
| **Emplacement de la logique** | Dispersée (phase dans le composant, type dispatché au niveau parent) | Centralisée (dispatcher sur l'hôte **et** le type en un seul endroit) |
| **Variantes par composant** | Une variante par hôte (question vs carte) — à maintenir | Zéro variante — même composant monte dans les deux hôtes |
| **Ajout d'un nouveau type** | Modifie l'hôte (ajout de branches conditionnelles) | Touche **uniquement** le registre, les composants du type et les handlers typés |
| **Risque de divergence** | Deux vérifications différentes de la même notion (`phase === 'REVEALED'` vs `MEMOTION_SUBPHASE === 'REVEAL'`) | Triplet unique `resolveHostContext` pour les deux hôtes |

---

## Organisation UI (v4.0.1+)

**Navbar** :
- Liens directs : Jeu, Scores, Équipes, Quiz, Historique, Palmarès
- Menu 🐝 dropdown : Config, Backup/Restaure, Logs, Mises à jour

**Pages admin** :
| Route | Fonctionnalités |
|-------|-----------------|
| `/admin/game` | Contrôle jeu, équipes, timer — bouton "NOUVELLE PARTIE" en phase STOPPED |
| `/admin/quiz` | Zone Quiz (Nom/Thème/Notes) + Zone Ambiance (fonds d'écran) + Zone Questions (CRUD) |
| `/admin/config` | Paramètres serveur, effet néon, WiFi defaults + SSID2, OTA firmware |
| `/admin/backup` | Sauvegarde, restauration, réinitialisation |
| `/admin/logs` | Logs serveur temps réel |
| `/admin/updates` | Vérification et installation mises à jour |

**WebSocket routing par page** (v3.8.0) :
- `/admin/*` → `GameProvider` endpoint `/ws/admin`
- `/tv` → `GameProvider` endpoint `/ws/tv`
- `/player`, `/enroll` → `GameProvider` endpoint `/ws/player`

**Contrainte TV** : affichage STATIQUE sans scroll — `overflow: hidden`, unités viewport, `flex + min-height: 0`.

**Décisions d'architecture** :
- Label "Questions" → "Quiz" (v4.0.1) — scope élargi (métadonnées + fonds + questions)
- Bouton "NOUVELLE PARTIE" déclenche `NEW_GAME` avec reset complet depuis STOPPED uniquement
- Métadonnées quiz dans Zone Quiz (centralisées), pas sur GamePage

**Fichiers clés** :
- `GamePage.jsx` : bouton NOUVELLE PARTIE + panneaux MEMOTION GRID/SELECTED/QUESTION/REVEAL
- `QuestionsPage.jsx` : 3 zones Quiz/Ambiance/Questions + éditeur MEMOTION
- `PlayerDisplay.jsx` : TV toutes phases (STATIQUE) — NEW_GAME, jeu normal, MEMOTION
- `GameContext.jsx` : GameProvider prop `endpoint`, routing WS
- `colorUtils.js` : `boostTeamColor()` (saturation TV), `nearestPaletteColorByHue()` (palette LED)

### Panneaux Admin MEMOTION (v5.1.0)

| Subphase | Panneau Admin (`GamePage.jsx`) | Affichage TV (`PlayerDisplay.jsx`) |
|----------|-------------------------------|-------------------------------------|
| `GRID` | Label informatif (mini-grille supprimée) — sélection via clic sur preview TV | Grille de cartes ; clic sur carte `UNPLAYED` en mode `isAdminPreview` envoie `MEMOTION_SELECT` |
| `SELECTED` | Boutons "DÉMARRER" (FLIP) et "ANNULER" | Carte zoomée plein écran (layoutId framer-motion) |
| `QUESTION` | Bouton "STOP TIMER" | Face VERSO plein écran + timer |
| `REVEAL` | Bouton équipe courante uniquement + bouton "Perdu" (remplace la liste de toutes les équipes) | Face REVEAL plein écran |

**Layout MotionCard TV (3 zones fixes — v5.1.0)** :
- `.memotion-card-header` : titre et thème (hauteur fixe, haut de la carte)
- `.memotion-card-body` : image (flex-grow, couvre l'espace central)
- Footer : étoiles + points (bas de la carte)
Supprime l'ancienne logique conditionnelle image/no-image.

---

## Mode ENTRACTE — Filtre Partagé et Panneau (v6.5.2, #119)

### Filtre d'Estompage

Quand `gameState.ENTRACTE = true`, un **filtre CSS identique** s'applique au contenu 
des quatre surfaces (sauf éléments explicitement nets) :

```css
filter: grayscale(0.85) brightness(0.55);
```

**Fichier source** : `web/src/styles/entracte.css` (classe `.entracte-dim`).

### Composant EntractePanel

Composant unique `components/EntractePanel.jsx` restitue l'affichage du panneau, 
idéal sur TV et VJoueur (mêmes proportions, même centrage). Pas de variante par surface.

Contenu :
- **Titre** (défaut `"ENTRACTE"`)
- **Sous-titre** (défaut `"Retour dans 20mn"`)
- **Image de fond optionnelle** (booléen `IMAGE_IS_CUSTOM`, URL stable 
  `/api/game/entracte-image` avec cache-buster)
- **Centrage flex** (jamais via `transform: translate(-50%, -50%)` qui serait écrasé par 
  la transformation d'animation)
- **Taille unique** (prop `PANEL_SIZE` %, largeur et hauteur identiques, borné 20–100)
- **Animation** (si `ANIM_INTENSITY > 0` et `prefers-reduced-motion: reduce` absent) : 
  zoom oscillant (±6% d'échelle) et balancement oscillant (±2° de rotation) combinés, 
  durée contrôlée par `ANIM_PERIOD`.

**Propriétés CSS** :
- `--ep-size` : pourcentage du panneau (défaut `65%`)
- `--ep-anim-duration` : durée en secondes
- `--ep-anim-intensity` : amplitude 0–100 (0 = pas de déclaration d'animation)

### Surface TV (`PlayerDisplay.jsx`, `/tv`)

**Rendu** :
- Contenu du jeu entouré de classe `.entracte-dim` (branche conditionnelle 
  `!isVPlayer`).
- `EntractePanel` monté en **frère**, jamais en enfant du nœud filtré (piège 
  `position: fixed` du QR code).

**z-index** : panneau à `500` (au-dessus du contenu TV max 100, sous QR code 1000).

**Contrainte TV statique** : aucun scroll, panneau centré et lisible en une seule 
vue ; tailles de police relatives au panneau, pas en `rem`.

### Surface VJoueur (`VPlayerPage.jsx`, `/player`)

**Rendu** :
- Contenu filtré avec classe `.entracte-dim`.
- `EntractePanel` identique à la TV — mêmes proportions, même centrage.
- **Cadenas 🔒** (`filter: none !important`) positionné au-dessus de la zone de buzz 
  (média click).

**Piège évité** : filtres ne s'additionnent pas (ex. un double filtrage rendrait l'écran 
presque noir). `PlayerDisplay` conditionne son filtre par `!isVPlayer` ; `VPlayerPage` 
porte son propre filtre indépendant.

**Garde additionnelle** : `handleBuzz` contient une garde `if (gameState.entracte) 
return` même si le serveur refuse la plupart des actions — évite l'envoi inutile et 
la rétroaction visuelle trompeuse.

### Surface Admin — Bouton Navbar

**Rendu** :
- **Bouton `ENTRACTE` / `FIN D'ENTRACTE`** dans la **Navbar** (entre badge version et groupe 
  "Jeu", visible sur toutes les pages admin).
- Reste net (pas filtré), cliquable, contrasté (couleur ambre inactif, rouge actif avec 
  halo, grisé désactivé si phase non autorisée).
- Accessible sur `/admin`, `/admin/quiz`, `/admin/config`, etc. — présent partout.

**Phases autorisées** : cf. `utils/phaseRules.js` (`canToggleEntracte(phase)`). 
Une seule source de vérité, partagée avec l'animateur.

### Surface Admin — Contenu Filtré (`GamePage.jsx`, `/admin`)

**Rendu** :
- Interface filtré (classe `.entracte-dim` appliquée aux enfants de grille, jamais 
  interposer de nœud — sinon la grille CSS casse).
- **Aucun panneau**.

### Surface Animateur (`AnimPage.jsx`, `/anim`)

**Rendu** :
- Interface filtré (classe `.entracte-dim` appliquée aux quatre zones de grille 
  séparément).
- **Aucun panneau, aucun bouton**.
- **Indicateur textuel net** : « ⏸ Entracte en cours — contrôle réservé à l'admin » 
  (positionné en angle, hors du nœud filtré).

### Utilitaires Partagés

#### `utils/phaseRules.js` — `canToggleEntracte(phase)`

Table booléenne centralisée des phases autorisées pour l'entrée en entracte :
```
STOPPED, PREPARE, READY, NEW_GAME, REVEALED → true
COUNTDOWN, STARTED, PAUSED, ENROLL → false
```

Sortie toujours permise (indépendant de la phase).

#### `web/src/styles/entracte.css`

Fichier CSS partagé déclarant :
- `.entracte-dim` — filtre + élément parent flex + animation (si applicable)
- `.entracte-panel*` — styles du panneau (centrage, taille, typographie relative)
- `@keyframes` combinant `scale` et `rotate`
- `@media (prefers-reduced-motion: reduce)` — animation neutralisée

---

## Pages et Composants — Mode RAFALE (v8.0.0, #16)

### Page : `/admin/rafale` — Éditeur du réservoir

**Fichier** : `web/src/pages/RafalePage.jsx` + `RafalePage.css`

**Fonctionnalités** :
- **CRUD complet** : création, affichage, modification, suppression questions
- **Filtres** : catégorie, difficulté, état "utilisée" (booléen dérivé)
- **Compteurs** : total questions, utilisées, disponibles
- **Validation** : énoncé/réponse non vides, difficulté 1–3, catégorie connue

**Routes** :
```javascript
<Route path="/admin/rafale" element={<RafalePage />} />
```

**Composants réutilisés** : `CategoryBadge`, `useCategories`, `useCategoryFilter`, `Button`, `Card`

### Composant : `RafaleTimers` — Double timer

**Fichier** : `web/src/components/RafaleTimers.jsx` + `RafaleTimers.css`

**Affichage** :
- Timer manche (~120 s) — barre longue
- Timer question (~3 s) — barre courte

**Props** :
- `matchTime` (ms) : décompte manche
- `questionTime` (ms) : décompte question

**Design** : Deux instances du composant `Timer.jsx` plutôt que modification mono-valeur (respect du contrat existant).

### Composant : `AnimRafaleActions` — Boutons validation

**Fichier** : `web/src/components/AnimRafaleActions.jsx`

**Affichage** :
- Bouton RÉPONSE VALIDE (vert, large)
- Bouton RÉPONSE INVALIDE (rouge, large)
- Réponse attendue + équipe active (mode-aware)

**Props** :
- `answer` (string) : réponse courante
- `currentTeam` (string) : équipe active
- `rafaleMode` (string) : mode courant (pour adapter libellé)

**CSS** : Réutilise `.anim-conduct-btn` / `-go` / `-danger` (aucun style nouveau)

**Emplacement** : Panneau L2 (`AnimConductPanel.jsx`) avec booléen `isRafale`

### Vue TV RAFALE — Affichage statique

**Fichier** : `web/src/pages/PlayerDisplay.jsx` + `PlayerDisplay.css`

**Affichage** (mode `isRafale`) :
- Double timer (manche + question, en haut)
- Question courante (centre, gros texte)
- Équipe active (bandeau fort, bas)
- Compteurs équipes (top 3–6 équipes)

**Invariants** :
- `overflow: hidden` (jamais auto/scroll)
- Unités viewport (`vh`, `vw`, `%`)
- Contenu plafonné (max 6 équipes visibles)

**Dispatch** :
```jsx
{questionType === "RAFALE" && <RafaleDisplay ...props />}
```

### Composant : `RafalePoolAlert` — Alerte pré-manche

**Fichier** : `web/src/components/RafalePoolAlert.jsx`

**États d'alerte** (3 booléens indépendants) :
- 🔴 **Bloquant** : `disponibles == 0` → démarrage refusé (bouton START grisé)
- 🟠 **Avertissement** : `disponibles < besoin_estimé` → démarrage autorisé, message risque
- ✅ **Neutre** : `disponibles ≥ besoin_estimé` → OK, message info

**Props** :
- `available` (int) : questions disponibles
- `estimatedNeed` (int) : besoin estimé

### Case à cocher — Sauvegarde sélective

**Fichier** : `web/src/pages/BackupPage.jsx`

**Ajout** :
```jsx
<label>
  <input type="checkbox" name="rafale" />
  Questions RAFALE (réservoir + flag utilisées)
</label>
```

**Comportement** :
- Inclus/exclus `data/files/rafale/` ET `data/config/rafale_used.json` ensemble
- Aucune sélection partielle
