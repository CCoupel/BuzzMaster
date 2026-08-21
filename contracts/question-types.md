# Contrat — Types de question, hôtes et cartes polymorphes

> **Statut** : contrat créé en phase Plan du milestone `v7.0.0 — MEMOTION+ : cœur` (#183, #184, #185).
> **Référence** : plan `_work/reports/plan-memotion-v700-20260821.md`.
> Ce fichier est la **référence unique** pour le discriminant de type porté par une carte MEMOTION,
> la portée carte de l'état et des actions, le contexte d'hôte normalisé et le barème de points.
> Les fichiers `models.md`, `game-state.md` et `websocket-actions.md` y renvoient sans le dupliquer.

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Type de jeu** | `SPEEDY` \| `QCM` \| `MEMORY` \| `MEMOTION` \| `ARDOISE` — l'énumération existante, inchangée |
| **Hôte** | Ce qui fait tourner une manche d'un type. Deux hôtes existent : l'hôte **question** (le cycle `GamePhase` classique) et l'hôte **carte MEMOTION** (la sous-machine `MEMOTION_SUBPHASE`) |
| **Contenu typé** | Les champs de données propres à un type (`QCM_ANSWERS`, `MEMORY_PAIRS`, `ARDOISE_KEYBOARD_TYPE`…) — portés indifféremment par une `Question` ou par une `MotionCard` |
| **Emplacement actif** | L'unique carte MEMOTION en cours de jeu. Il n'y en a jamais deux |

**Profondeur d'imbrication plafonnée à 1** : une carte MEMOTION ne peut pas porter le type
`MEMOTION`. Contrainte structurelle, validée à l'enregistrement et à la sélection.

---

## 2. Contenu typé partagé (`TypedContent`)

Les champs de contenu propres à un type sont regroupés dans une structure **partagée entre
`Question` et `MotionCard`**, embarquée à plat dans le JSON des deux (aucun niveau
d'imbrication supplémentaire).

```go
// internal/game/models.go
type TypedContent struct {
    // SPEEDY / ARDOISE
    Answer      string `json:"ANSWER,omitempty"`
    // QCM
    QCMAnswers        *QCMAnswers `json:"QCM_ANSWERS,omitempty"`
    QCMCorrect        string      `json:"QCM_CORRECT,omitempty"`
    QCMHintsEnabled   bool        `json:"QCM_HINTS_ENABLED,omitempty"`
    QCMHintThreshold1 float64     `json:"QCM_HINT_THRESHOLD_1,omitempty"`
    QCMHintThreshold2 float64     `json:"QCM_HINT_THRESHOLD_2,omitempty"`
    QCMPenalty1       float64     `json:"QCM_PENALTY_1,omitempty"`
    QCMPenalty2       float64     `json:"QCM_PENALTY_2,omitempty"`
    // ARDOISE
    ArdoiseKeyboardType KeyboardType `json:"ARDOISE_KEYBOARD_TYPE,omitempty"`
    // MEMORY
    MemoryPairs  []MemoryPair  `json:"MEMORY_PAIRS,omitempty"`
    MemoryConfig *MemoryConfig `json:"MEMORY_CONFIG,omitempty"`
    MemoryMode   string        `json:"MEMORY_MODE,omitempty"`
}
```

> ⚠️ **Invariant de non-régression, à garantir par test** : l'embarquement ne change **aucun** nom
> de champ JSON. Les 85 `question.json` existants doivent se relire et se réécrire **octet pour
> octet à l'identique**. C'est la condition d'acceptation la plus stricte du lot — voir §9.

> ⚠️ **Déviation d'implémentation (#184, B-B1) — asymétrie `omitempty` sur `Answer`.** Le snippet
> ci-dessus montre un unique champ `Answer` avec `omitempty` ; en pratique, `Question` **garde son
> propre champ `Answer` déclaré explicitement** (`json:"ANSWER"`, **sans** `omitempty`), à côté de
> l'embarquement de `TypedContent`. Raison : 26/85 `question.json` persistent `"ANSWER":""`
> explicitement (question sans réponse), et `Question.Answer` n'a jamais eu `omitempty` — lui en
> ajouter un via l'embarquement aurait fait disparaître la clé au premier réenregistrement,
> violant l'invariant octet-pour-octet ci-dessus. À l'inverse, `TypedContent.Answer` **garde**
> `omitempty` : aucune carte MEMOTION existante ne porte `ANSWER` (le SPEEDY de carte utilise
> `ANSWER_TEXT`/`ANSWER_IMAGE`), et sans `omitempty` chacune en gagnerait une vide. La règle de
> précédence JSON de Go sur les champs embarqués (le champ le moins imbriqué gagne en cas de
> collision de nom) fait que le `Answer` explicite de `Question` reste seul déterminant pour l'hôte
> question ; `TypedContent.Answer` ne s'applique qu'à `MotionCard` (ARDOISE-en-carte, #186,
> v7.1.0), qui n'a pas de champ `Answer` propre pour entrer en collision. Vérifié par test dédié
> avant implémentation et par le round-trip des 85 fixtures (§10.3).

**Pourquoi partager plutôt que dupliquer sur `MotionCard`** : c'est ce qui rend l'adaptateur
« carte → question synthétique » (déjà présent dans `AnimPage.jsx`) trivial — une copie de champs
de même nom — et ce qui permet à `AnimQcmOptions`, aux sous-éditeurs et aux vues TV de servir les
deux hôtes sans variante. Un futur type n'ajoute ses champs **qu'une seule fois**.

---

## 3. `MotionCard` — discriminant de type

```go
type MotionCard struct {
    ID         string       `json:"ID"`
    Type       QuestionType `json:"TYPE,omitempty"`   // absent ou "" ⇒ SPEEDY
    RectoTheme string       `json:"RECTO_THEME"`
    RectoImage string       `json:"RECTO_IMAGE,omitempty"`
    Difficulty int          `json:"DIFFICULTY"`
    PointsRule *PointsRule  `json:"POINTS_RULE,omitempty"`  // §6

    // Contenu SPEEDY historique — conservé tel quel
    QuestionText  string `json:"QUESTION_TEXT,omitempty"`
    QuestionImage string `json:"QUESTION_IMAGE,omitempty"`
    AnswerText    string `json:"ANSWER_TEXT,omitempty"`
    AnswerImage   string `json:"ANSWER_IMAGE,omitempty"`

    TypedContent  // embarqué à plat — §2
}
```

**Rétrocompatibilité** : une carte sans `TYPE` vaut `SPEEDY` et se comporte exactement comme
aujourd'hui. Aucune migration de fichier. Les 9 questions MEMOTION existantes sont inchangées.

**Validation à l'enregistrement** : `TYPE` doit appartenir à l'énumération **et** avoir
`NestableInMotionCard = true` dans le registre (§7). `MEMOTION` est refusé (profondeur 1).

### 3.1 Champs propres au type (`OwnedFields`) vs champs de la carte

Le registre (§7) déclare, pour chaque type, les champs de `TypedContent` qui **lui appartiennent**
(`OwnedFields`). Tout le reste appartient à la **carte**, pas au type.

| Portée | Champs | Change avec le type ? |
|---|---|---|
| **Carte** — face de grille, énoncé, barème | `RECTO_THEME`, `RECTO_IMAGE`, `DIFFICULTY`, `POINTS_RULE`, `QUESTION_TEXT`, `QUESTION_IMAGE` | ❌ jamais — conservés tels quels quel que soit le `TYPE` |
| **`SPEEDY`** | `ANSWER_TEXT`, `ANSWER_IMAGE` | ✅ |
| **`QCM`** | `QCM_ANSWERS`, `QCM_CORRECT`, `QCM_HINTS_ENABLED`, `QCM_HINT_THRESHOLD_1/2`, `QCM_PENALTY_1/2` | ✅ |
| **`ARDOISE`** *(v7.1.0)* | `ANSWER`, `ARDOISE_KEYBOARD_TYPE` | ✅ |
| **`MEMORY`** *(v7.1.0)* | `MEMORY_PAIRS`, `MEMORY_CONFIG`, `MEMORY_MODE` | ✅ |

`OwnedFields` sert à trois choses : le **verrou de type** (§3.2), la **cohérence serveur** (§3.2),
et le **montage du sous-éditeur** dans l'éditeur de carte.

### 3.2 Verrouillage du type d'une carte — décision utilisateur du 2026-08-21

**Le type d'une carte ne peut plus changer dès qu'elle porte du contenu propre à son type.**
Ni avertissement, ni perte silencieuse : c'est **interdit**.

Le thème, la difficulté, l'énoncé et le barème **ne verrouillent jamais** : ils appartiennent à la
carte, pas au type, et survivent intacts à toute bascule. Les compter reviendrait à verrouiller une
carte avant qu'il y ait quoi que ce soit à perdre.

Le verrou n'est pas définitif — il **oblige à vider explicitement** le contenu propre au type avant
de basculer. C'est tout l'objet de la décision : la destruction devient un geste délibéré de
l'utilisateur au lieu d'un effet de bord de l'enregistrement.

#### Prédicat exact

> **Une carte est déverrouillée tant qu'aucun de ses `OwnedFields` ne s'écarte de sa valeur de
> création.**

Le prédicat porte sur l'**écart à la valeur de création**, et non sur la non-nullité. La distinction
n'est pas théorique : plusieurs `OwnedFields` ont une valeur par défaut non vide dès la création
d'une carte, relevée dans `QuestionsPage.jsx` (`formData` initial et `handleAddMotionCard`).

| Type | `OwnedField` | Valeur de création | Verrouille quand |
|---|---|---|---|
| `SPEEDY` | `ANSWER_TEXT` / `ANSWER_IMAGE` | `""` / absente | non vide |
| `QCM` | `QCM_ANSWERS` / `QCM_CORRECT` | `{"","","",""}` / `""` | une réponse ou la désignation est saisie |
| `QCM` | `QCM_HINTS_ENABLED` | `false` | passe à `true` |
| `QCM` | `QCM_HINT_THRESHOLD_1/2` | **`0.25` / `0.125`** | s'écarte de ces valeurs |
| `QCM` | `QCM_PENALTY_1/2` | **`0.67` / `0.33`** | s'écarte de ces valeurs |
| `ARDOISE` *(v7.1.0)* | `ARDOISE_KEYBOARD_TYPE` | **`"AZERTY"`** | s'écarte de cette valeur |
| `MEMORY` *(v7.1.0)* | `MEMORY_MODE` | **`"SOLO"`** | s'écarte de cette valeur |
| `MEMORY` *(v7.1.0)* | `MEMORY_CONFIG` | **8 réglages non vides** | l'un d'eux s'écarte de son défaut |

> ⚠️ **Un prédicat écrit « au moins un `OwnedField` non vide » verrouillerait une carte QCM dès sa
> création** (`QCM_HINT_THRESHOLD_1` vaut `0.25`, jamais vide), et de même une carte ARDOISE
> (`"AZERTY"`) ou MEMORY (`"SOLO"`) en v7.1.0. Le sélecteur serait grisé en permanence sur ces
> types : la fonctionnalité serait morte à la livraison. `SPEEDY` seul y échappe, car ses deux
> `OwnedFields` naissent vides — d'où un piège qui ne se voit pas en testant le cas par défaut.

#### Deux niveaux d'application

| Niveau | Règle | Ce que cela garantit |
|---|---|---|
| **UI** (`QuestionsPage.jsx`) | Sélecteur **désactivé** dès qu'un `OwnedField` s'écarte de sa valeur de création, raison affichée à côté | La discipline de saisie |
| **Serveur** (`handleUploadQuestion`) | Une carte ne doit **jamais porter de contenu appartenant à un autre type que son `TYPE` déclaré** → **HTTP 400 `CARD_TYPE_CONTENT_MISMATCH`** | L'intégrité des données : aucune donnée orpheline |

> ⚠️ **La règle serveur porte sur la cohérence de la charge utile reçue, pas sur une comparaison
> avec la version stockée.** C'est délibéré :
> - elle reste **sans état** — `handleUploadQuestion` reconstruit la question de zéro et n'a jamais
>   eu besoin de relire l'ancienne version, il ne commence pas ici ;
> - elle autorise le parcours **« vider le contenu → le sélecteur se débloque → changer de type →
>   enregistrer »** en **un seul** enregistrement ;
> - elle ferme le contournement par appel direct à l'API : une carte `TYPE=QCM` porteuse d'un
>   `ANSWER_TEXT` est rejetée, au lieu de conserver une donnée orpheline.
>
> Le serveur **ne réplique pas** le verrou d'interface : une charge utile internement cohérente
> issue d'un appel hors interface est acceptée. Écart connu, borné, et sans effet sur l'intégrité
> des données.

**Cas limites :**

| Situation | Verdict |
|---|---|
| Carte neuve, `SPEEDY` ou `QCM`, aucun `OwnedField` écarté de son défaut | Type librement modifiable |
| Thème, difficulté, énoncé ou barème renseignés — rien d'autre | **Modifiable** — ces champs appartiennent à la carte |
| Un texte de réponse SPEEDY saisi | **Verrouillée** |
| Une réponse QCM saisie, ou un seuil d'indice déplacé | **Verrouillée** |
| `OwnedFields` ramenés à leurs valeurs de création | Redevient modifiable — le verrou est réactif |
| `TYPE` inchangé, contenu quelconque | Toujours accepté |
| Charge utile portant du contenu de deux types à la fois | **HTTP 400**, quelle que soit son origine |

**Rétrocompatibilité** : les cartes des 9 questions MEMOTION existantes valent `SPEEDY` et portent
un `ANSWER_TEXT` renseigné — elles s'ouvrent donc **verrouillées sur `SPEEDY`**. Sans effet : leur
type implicite est déjà le bon, rien ne cherche à le changer, et elles ne portent que du contenu
`SPEEDY`, donc cohérentes au sens de la règle serveur. Elles se rouvrent, se modifient et se
réenregistrent exactement comme aujourd'hui.

---

## 4. Contexte d'hôte normalisé

Une implémentation de type ne lit **jamais** `GamePhase` ni `MEMOTION_SUBPHASE` directement. Elle
reçoit de son hôte un triplet normalisé :

```go
type HostContext struct {
    Playable     bool   // les entrées sont acceptées, le contenu est en jeu
    Revealed     bool   // la réponse est montrée
    TimerRunning bool   // un chronomètre décompte pour cette manche
    CardID       string // "" pour l'hôte question ; ID de carte pour l'hôte carte MEMOTION
}
```

**Table de dérivation — spécification unique, implémentée des deux côtés :**

| Hôte | `Playable` | `Revealed` | `TimerRunning` |
|---|---|---|---|
| **Question** | `PHASE == STARTED` | `PHASE == REVEALED` | `PHASE == STARTED` **et** `CURRENT_TIME > 0` |
| **Carte MEMOTION** | `MEMOTION_SUBPHASE == QUESTION` | `MEMOTION_SUBPHASE == REVEAL` | `MEMOTION_SUBPHASE == QUESTION` **et** `CURRENT_TIME > 0` |
| **Aucun** (GRID, MEMORIZE, SELECTED, PREPARE, READY…) | `false` | `false` | `false` |

#### `TimerRunning` — toujours dérivé de `CURRENT_TIME`, jamais du ticker

> **`TimerRunning` vaut « l'hôte est dans son état courant **et** `CURRENT_TIME > 0` ». Il ne doit
> jamais être dérivé de l'existence d'un objet ticker côté serveur.**

Le motif est structurel, pas esthétique : `Engine.timer` est un `*time.Ticker`, **champ privé jamais
sérialisé**. Un client ne peut pas l'observer, donc **aucune implémentation JS ne pourra jamais s'y
conformer**. Une clause qu'un des deux côtés est structurellement incapable d'implémenter n'est pas
une spécification partagée. `CURRENT_TIME` est en revanche diffusé (tag JSON `CURRENT_TIME`, exposé
côté client sous `gameState.timer`) : c'est la seule base sur laquelle les deux côtés peuvent
converger.

S'y ajoute une **convention déjà en production** : `AnimPage.jsx` alimente
`motion.timerRunning = gameState.timer > 0` depuis #160, valeur consommée par `AnimConductPanel` →
`AnimMotionActions` → la matrice de gestes de `utils/motionRules.js`. Dériver `HostContext.TimerRunning`
autrement donnerait à `/anim` **deux notions contradictoires de « le chrono tourne » dans le même
panneau**.

> ⚠️ **La même expression dans les deux lignes, délibérément.** Un composant de type reçoit ce
> triplet sans savoir quel hôte le lui fournit : si `TimerRunning` signifiait « la phase est
> STARTED » chez l'un et « un décompte reste » chez l'autre, le même composant se comporterait
> différemment selon son hôte — exactement la classe de défaut que `HostContext` existe pour
> empêcher.

> 📌 **Ce que le contrat garantit, et ce qu'il ne garantit pas** : les deux côtés appliquent la
> **même expression au même état**. Ils ne sont pas tenus à l'égalité instantanée à travers le
> réseau — le serveur lit `CURRENT_TIME` à l'instant de l'appel, le client la dernière valeur
> diffusée, d'où un écart possible d'un tick. C'est de la latence, inhérente à toute valeur dérivée
> non sérialisée, et c'est exactement le même régime que `Playable` et `Revealed`. Ce n'est **pas**
> un motif pour sérialiser le triplet (§4, « Non sérialisé »).

#### `CardID` — règle unique, sans cas particulier

> **`CardID` vaut toujours `MEMOTION_SELECTED`. Sans condition, sans branchement, quelle que soit
> la sous-phase et quel que soit le type de la question.**

Cette formulation n'a **aucune** exception à énumérer, parce que `MEMOTION_SELECTED` est déjà exactement
l'identité de la carte en jeu à tout instant :

| Situation | `MEMOTION_SELECTED` | Donc `CardID` |
|---|---|---|
| Question non-MEMOTION | `""` (remis à zéro dans `Ready()` et `InitGame()`) | `""` — hôte question |
| MEMOTION en `MEMORIZE` ou `GRID` | `""` (posé par `initMotionStateUnsafe`, par l'expiration du timer de mémorisation et par chaque retour en `GRID`) | `""` — aucune carte en jeu |
| MEMOTION en `SELECTED`, `QUESTION` ou `REVEAL` | ID de la carte (posé par `SelectMotionCard`) | **l'ID de la carte** |

> ⚠️ **`CardID` est un discriminant d'hôte, pas un indicateur d'activité.** Il répond à « quel
> emplacement porte l'état typé ? », pas à « le contenu est-il jouable ? » — ce à quoi répondent
> `Playable` et `Revealed`. En sous-phase `SELECTED`, une carte **est** l'hôte (elle est choisie, son
> emplacement actif est peuplé) même si rien n'y est encore jouable. Renvoyer `""` y ferait router
> `getTypeState` (§5.3) vers les champs plats de l'hôte **question** — l'hôte inverse.

> **Cohérence interne exigée** : `CardID` doit valoir `MEMOTION_ACTIVE.CARD_ID` (§5.2) à tout
> instant, et l'invariant de portée des actions (§9.2) doit s'appuyer sur **la même** expression
> `MEMOTION_SELECTED`. Ces trois mécanismes désignent la même chose ; les faire diverger est un
> défaut, jamais un choix d'implémentation.

> 📌 **Historique** — cette cellule disait « selon le cas » dans la première version du contrat. Go
> et JS ont chacun tranché différemment, tous deux verts de leur côté (risque R5 du plan, matérialisé
> et détecté par `test-writer`). L'ambiguïté est levée ici : plus aucune cellule de cette table ne
> laisse de choix à l'implémentation.

**Non sérialisé.** Le contexte est **recalculé de part et d'autre** à partir de `PHASE` et
`MEMOTION_SUBPHASE`, tous deux déjà présents dans `GameState`. Motif : le sérialiser coûterait de
la charge utile pour une information intégralement dérivable. Contrepartie assumée : deux
implémentations d'une même règle — **cette table est la spécification, et chaque côté porte un test
dont les cas sont nommés à l'identique** (voir §9).

Côté JS : un unique `utils/hostContext.js` exposant `resolveHostContext(gameState)`. Aucun
composant de type ne reçoit plus `phase` en prop : il reçoit `playable` / `revealed`.

> **Conséquence directe** : `AnimMemoryGrid`, qui révèle tout sur `phase === 'REVEALED'`, et
> `AnimQcmOptions`, qui reçoit `revealed`, deviennent montables dans les deux hôtes sans variante.
> C'est le point qui débloque #185, #186 et #187.

---

## 5. État vivant par type — l'emplacement actif

### 5.1 Principe

Une seule carte MEMOTION est jouable à la fois. L'état vivant du type imbriqué n'est donc **pas**
une carte indexée par identifiant de carte, mais **un emplacement unique** décrivant la carte
active. Il est remis à zéro à chaque `MEMOTION_SELECT` et vidé au retour en `GRID`.

> **Décision de conception** : ce choix borne l'inflation de la charge utile `GAME` (déjà ~11 Ko en
> MEMOTION, voir `vplayer-payload-filter.md` §5) au coût **d'un seul** état de type, et non de N.
> Les *résultats* par carte, eux, restent dans les cartes existantes `MEMOTION_CARD_STATES` et
> `MEMOTION_CARD_TEAMS`, inchangées.

### 5.2 Champ `GameState`

```jsonc
"MEMOTION_ACTIVE": {
  "CARD_ID": "mc-3",          // "" hors SELECTED/QUESTION/REVEAL
  "TYPE": "QCM",              // "" hors emplacement actif ; "SPEEDY" pour une carte historique
  "STATE": {                  // état vivant du type — forme libre, propre au type
    "QCM_INVALIDATED": ["RED", "YELLOW"]
  }
}
```

**Jamais `omitempty`** — règle projet : toujours sérialisé, y compris vide
(`{"CARD_ID":"","TYPE":"","STATE":{}}`), pour éviter les réinitialisations manquées côté frontend.

**Non persisté** — rejoint `Motion*` dans les champs exclus de `state_persistence.go`.

### 5.3 Résolution de l'état par type — côté client

Les champs question-scopés existants (`QCM_INVALIDATED`, `ARDOISE_ANSWERS`,
`MEMORY_FLIPPED_CARDS`…) **restent inchangés** pour l'hôte question. Un seul accesseur partagé
tranche selon l'hôte :

```js
// utils/typeState.js
getTypeState(gameState, hostContext)
//  hostContext.CardID === ""  → les champs plats de gameState (hôte question)
//  hostContext.CardID !== ""  → gameState.MEMOTION_ACTIVE.STATE (hôte carte)
```

> **Duplication assumée et délibérée** : la même notion (ex. les réponses QCM invalidées) existe à
> deux emplacements selon l'hôte. L'alternative — faire lire au QCM classique un emplacement
> hôte-scopé — serait un changement **BREAKING** pour `/tv`, `/anim` et `/admin`, pour un bénéfice
> purement esthétique. La duplication est confinée à un seul accesseur, et c'est le prix explicite
> du « aucun changement BREAKING » de ce lot.

---

## 6. Barème de points

### 6.1 Autorité

Le barème appartient **toujours à l'hôte**, jamais au type imbriqué. Un type ne rend qu'un
**résultat**. C'est la décision utilisateur du 2026-08-21.

```go
type TypeOutcome struct {
    WinnerTeam string // "" = personne
    Units      int    // 1 = gagné, 0 = perdu ; > 1 réservé aux types à progression (MEMORY, #187)
}
```

### 6.2 Règle de points d'une carte

```jsonc
"POINTS_RULE": { "MODE": "STARS" | "FIXED" | "PER_UNIT", "VALUE": 10 }
```

| `MODE` | Points attribués | Usage |
|---|---|---|
| absent / `STARS` | Barème par étoiles existant : `MOTION_CONFIG.POINTS_<n>_STAR` si `> 0`, sinon `DIFFICULTY → 1/3/5` | **Défaut** — comportement actuel, inchangé |
| `FIXED` | `VALUE` si `Units > 0`, sinon 0 | Carte dont la valeur ne dépend pas de sa difficulté |
| `PER_UNIT` | `VALUE × Units` | Types à progression — **ouvre #187 (MEMORY au prorata) sans toucher le cœur** |

`Units` vaut 1 par défaut pour tout type binaire (gagné/perdu). Un type à progression rapporte son
propre décompte. Le tout-ou-rien pour MEMORY s'exprime en `STARS`/`FIXED` (le type ne rend `Units=1`
que si toutes les paires sont trouvées) ; le prorata s'exprime en `PER_UNIT`. **Les deux cas
demandés par #187 sont donc exprimables sans modification du cœur** — c'est le critère
d'acceptation du mécanisme.

---

## 7. Registre des types

Source unique côté Go — `internal/game/question_types.go` :

```go
type TypeDescriptor struct {
    Type                 QuestionType
    OwnedFields          []string // champs de TypedContent propres à ce type — §3.1.
                                  // Sert au verrou de type, a la coherence serveur
                                  // et au montage du sous-editeur.
    MediaSlots           []string // §8
    NestableInMotionCard bool
    HasPlayerInput       bool     // documente le besoin d'élargissement de liste blanche
}
```

| Type | `MediaSlots` (hôte carte) | `NestableInMotionCard` | `HasPlayerInput` |
|---|---|---|---|
| `SPEEDY` | `recto`, `question`, `answer` | ✅ | ❌ |
| `QCM` | `recto`, `question` | ✅ **(#185)** | ❌ — voir §7.1 |
| `ARDOISE` | `recto`, `question` | ⬜ *(#186, v7.1.0)* | ✅ |
| `MEMORY` | `recto` + N paires | ⬜ *(#187, v7.1.0)* | ✅ |
| `MEMOTION` | — | ❌ **jamais** (profondeur 1) | ❌ |

Un registre **en dur et exhaustif**. Pas de DSL, pas d'architecture à plugins — décision explicite
de #184 (hors portée : « toute abstraction spéculative pour des types qui n'existent pas encore »).

### 7.1 Carte QCM : affichage et désignation, sans entrée joueur

Une carte MEMOTION de type `QCM` **n'ouvre aucune action entrante nouvelle**. Les quatre réponses
sont **affichées** (`/tv` et `/anim`), les indices invalident progressivement des réponses à
l'écran, et l'animateur **désigne** l'équipe gagnante par le `MEMOTION_DONE` existant — exactement
comme une carte SPEEDY aujourd'hui.

> ⚠️ Il n'y a **ni buzz, ni `VPLAYER_QCM_ANSWER`, ni modification de la liste blanche entrante**
> pour #185. `engine.go` ignore les buzz en MEMOTION et cela reste vrai. C'est ce qui fait de QCM
> le premier type imbriqué le moins coûteux (aucune frontière franchie), et c'est le périmètre
> exact validé au GATE. L'entrée joueur en carte est le sujet de #186/#187, milestone v7.1.0.

---

## 8. Pipeline média piloté par emplacements

Aujourd'hui, `handleUploadQuestion` code en dur trois emplacements par carte :
`motion_card_<cardID>_recto`, `_question`, `_answer` → écrits dans `RECTO_IMAGE`,
`QUESTION_IMAGE`, `ANSWER_IMAGE`.

Nouvelle règle : le nom du champ de formulaire reste `motion_card_<cardID>_<slot>`, mais **la liste
des `<slot>` est fournie par le descripteur du type de la carte** (§7), au lieu d'être écrite en
dur.

**Rétrocompatibilité totale** : `SPEEDY` déclare exactement `recto`, `question`, `answer` — les
charges utiles d'éditeur existantes restent valides sans changement, octet pour octet.

---

## 9. Portée des actions et invariant de scope

### 9.1 Champ de portée partagé

```go
type CardScope struct {
    MotionCardID string `json:"MOTION_CARD_ID,omitempty"`
}
```

Embarqué dans les charges utiles des actions par type. **Aucune action existante ne devient
obligatoirement porteuse de ce champ** — il est optionnel et son absence conserve le comportement
actuel.

### 9.2 Invariant, vérifié côté serveur

| Situation | `MOTION_CARD_ID` | Verdict |
|---|---|---|
| Aucune manche MEMOTION en cours (`MEMOTION_SUBPHASE == ""`) | absent | ✅ accepté |
| Aucune manche MEMOTION en cours | présent | ❌ refusé — `CARD_SCOPE_UNEXPECTED` |
| Manche MEMOTION, emplacement actif | `== MEMOTION_SELECTED` | ✅ accepté |
| Manche MEMOTION, emplacement actif | absent ou `!= MEMOTION_SELECTED` | ❌ refusé — `CARD_SCOPE_MISMATCH` |

C'est une **frontière d'autorisation** : elle empêche une action typée de s'appliquer à une carte
qui n'est pas celle en jeu. Refus silencieux interdit — l'erreur est renvoyée comme les
`MotionError` existants.

> Pour v7.0.0 (#185, QCM sans entrée joueur), aucune action typée n'est effectivement portée par une
> carte : l'invariant est **posé et testé**, mais son premier consommateur réel est #186.
> Il est livré maintenant parce qu'il fait partie du contrat que #186/#187 doivent pouvoir supposer
> acquis — et parce que le poser après coup obligerait à rouvrir le cœur, ce que le test
> d'agnosticité de #184 interdit.

### 9.3 `MEMOTION_DONE` — champ `UNITS`

```jsonc
{ "ACTION": "MEMOTION_DONE", "MSG": { "CARD_ID": "mc-1", "WINNER_TEAM": "team_A", "UNITS": 1 } }
```

`UNITS` est **optionnel**, défaut `1`. Absent ⇒ comportement actuel strictement identique.
Consommé par `POINTS_RULE.MODE == "PER_UNIT"` (§6.2).

---

## 10. Test d'agnosticité — critère de recette du cœur

> Ajouter un type à une carte doit toucher **uniquement** : son entrée de registre (§7), ses propres
> composants et handlers, et ses propres lignes de liste blanche. **Zéro ligne à l'intérieur de
> l'hôte MEMOTION.**

Vérification outillée, à livrer avec #184 :

1. **Test d'exhaustivité** — énumère les types du registre et vérifie que chacun possède : un
   descripteur, une liste d'emplacements média, une entrée dans `questionTypeMeta.js`, et une
   décision de liste blanche explicite. Un type ajouté sans l'une de ces entrées **casse la
   compilation ou un test**, au lieu de retomber silencieusement sur `SPEEDY`.
2. **Test de dérivation d'hôte** — les cas de la table §4 sont nommés à l'identique côté Go et
   côté JS.
3. **Test de non-régression des fixtures** — les 85 `question.json` se relisent et se réécrivent
   octet pour octet (§2).
4. **Test de charge utile** — la taille du nœud `GAME` d'une manche MEMOTION reste dans une borne
   explicite par rapport à la référence v6.5.2.

#186 et #187 doivent **rapporter dans leur DONE** toute ligne d'hôte qu'elles ont dû modifier, et
pourquoi. C'est la boucle « de plus en plus agnostique » demandée, rendue observable.

---

## 11. Bilan de compatibilité

**Aucun changement BREAKING dans ce lot.**

| Élément | Nature | Justification |
|---|---|---|
| `MotionCard.TYPE` | **NEW**, optionnel | absent ⇒ `SPEEDY` ⇒ comportement actuel |
| `TypedContent` embarqué | **CHANGED**, interne Go | JSON strictement inchangé — garanti par test de round-trip |
| `MotionCard.POINTS_RULE` | **NEW**, optionnel | absent ⇒ barème par étoiles actuel |
| `GameState.MEMOTION_ACTIVE` | **NEW**, jamais `omitempty` | champ ajouté ; aucun client existant ne le lit |
| `MEMOTION_DONE.UNITS` | **NEW**, optionnel | absent ⇒ 1 ⇒ comportement actuel |
| `MOTION_CARD_ID` (`CardScope`) | **NEW**, optionnel | absent hors manche MEMOTION ⇒ comportement actuel |
| Emplacements média par descripteur | **CHANGED**, interne | `SPEEDY` déclare les 3 noms actuels ⇒ charges utiles d'éditeur inchangées |
| Champs question-scopés (`QCM_INVALIDATED`…) | **inchangés** | volontairement non migrés — voir §5.3 |
| Liste blanche entrante | **inchangée** | aucun élargissement en v7.0.0 — voir §7.1 |
| Verrouillage du type sur contenu propre au type | **NEW**, comportement | UI : sélecteur désactivé dès qu'un `OwnedField` s'écarte de sa valeur de création (§3.2). Thème, difficulté, énoncé et barème ne verrouillent jamais. Serveur : **HTTP 400** `CARD_TYPE_CONTENT_MISMATCH` sur charge utile incohérente. Les 9 questions MEMOTION existantes s'ouvrent verrouillées sur `SPEEDY` — sans effet, leur type est déjà le bon |
