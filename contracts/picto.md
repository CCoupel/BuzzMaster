# Contrat — Type de jeu PICTO « dessiner un mot et le faire deviner » (v13.0.0, milestone #30)

**Issues** : #191 (type de question PICTO et éditeur), #192 (équipe et dessinateur, remise secrète du mot), #193 (canal
de dessin temps réel), #194 (manche jouable de bout en bout). Découpage : plan `_work/reports/plan-20261123-picto.md`.

**Statut** : contrat de référence, écrit avant implémentation (contract-first) — **révision 7** (2026-11-23, issue entrante : audit `security` du LOT-2.1).

**Arbitrages** :
- cadrage : `_work/reports/plan-pictionary-cadrage-20261123-135359.md` ;
- réponses de l'utilisateur : `_work/tasks/planner-20261123-plan-picto.md` (Q1–Q12) ;
- corrections : `planner-20261123-plan-picto-correction.md` (pas de passe, source des mots) ;
- GATE 2 : `planner-20261123-gate2-corrections.md` (I1–I7, maquettes) ;
- allègement : `planner-20261123-scope-reduction.md` (réservoir et IA reportés) ;
- révision 5 (GATE 2) : **tirage libre** parmi les mots saisis, chronomètre TV commun à tous les types ;
- révision 6 (GATE 2) : dessinateur **choisi en `READY` ou tiré au hasard** parmi les joueurs connectés de l'équipe ;
- révision 7 (issue entrante, audit `security` du LOT-2.1) : liaison des tablettes par jeton de session
  (`contracts/vplayer-session.md`, M1), purge de l'état de débit (m1), `PICTO_CANVAS_SYNC` borné (m2).

Les choix techniques du planner sont marqués **[planner]**.

**Périmètre v13.0.0** : une manche = une équipe et **un seul mot**, sans passe. Le mot est **saisi dans la question**
(1 à 20 mots, dont un est tiré au hasard). Il n'y a ni réservoir global ni pioche.

**Reporté en v13.1.0** (milestone #48, issue #318) : réservoir global de mots (stockage, HTTP, onglet, pioche, filtre,
marquages persistants présenté / révélé, doublons par catégorie), génération IA des mots, catégorie « Pools » de la
sauvegarde, mode RAFALE de PICTO (maximum de mots, passes). Le §12 garde les décisions déjà prises et le §14 ce que le
modèle laisse ouvert.

**Hors périmètre définitif** : relais multi-équipes ; outils de dessin (couleurs, gomme, annuler, « tout effacer »).

> Ce fichier est la **référence unique** pour `dev-backend` et `dev-frontend` sur le type PICTO.
> En cas de divergence backend/frontend, ce document tranche.
> Le backend PEUT modifier un contrat sous contrainte technique — en documentant la raison ici.

> **Nom** : identifiant `PICTO`, libellé « Picto » (icône ✏️). Aucun nom de jeu commercial déposé n'apparaît
> **nulle part** : code, interface, documentation, CHANGELOG, marketing (décision utilisateur, droits de marque).

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Manche PICTO** | Une question de type `PICTO` jouée de `START` à la révélation. Une manche = **une équipe, un seul mot à trouver**. Il n'y a pas de passe. |
| **Mots de la question** | Les 1 à 20 mots saisis dans la question (`PICTO_WORDS`). Un seul est dessiné par manche. |
| **Mot de la manche** | Le mot tiré au `START` parmi les mots de la question (§3). **Secret** pour la TV et les tablettes jusqu'à la révélation (§10). |
| **Équipe qui dessine** | L'équipe choisie pour la manche (`PICTO_TEAM`). Ses joueurs devinent **à l'oral** ; elle seule peut marquer. |
| **Dessinateur** | Le joueur de l'équipe désigné en préparation (`PREPARE` / `READY`) par l'animateur ou l'admin, par choix direct ou par tirage au hasard parmi les joueurs connectés (`PICTO_DRAWER`). C'est forcément un bumper **VJoueur**, avec un écran : un buzzer physique ne peut pas dessiner. |
| **Feuille** | Surface de dessin carrée (1:1), trait noir sur fond blanc, coordonnées normalisées (§8.2). **Elle ne s'efface jamais pendant une manche** : il n'y a ni passe, ni « tout effacer », ni remplacement du dessinateur. |
| **Époque de feuille** | Numéro (`PICTO_CANVAS_EPOCH`) attribué à chaque feuille vierge, c'est-à-dire à chaque `START`. Tout trait d'une autre époque est ignoré (§8.4). |
| **Relance de la question** | L'admin sélectionne de nouveau la question PICTO en cours : retour en `PREPARE`, par le mécanisme existant. Au `START` suivant, un autre mot est tiré si la question en a plusieurs, et la feuille repart vierge (§6.6). |

## 2. Décisions d'architecture

1. **`PICTO` est un `QuestionType`, sans nouvelle phase principale** (patrons RAFALE / ROUE). Les phases existantes
   `PREPARE → READY → COUNTDOWN → STARTED (↔ PAUSED) → STOPPED → REVEALED` portent la manche. Le détail vit dans des
   champs `PICTO_*` (§5). Les modes existants ne sont pas modifiés.
2. **Les mots vivent dans la question** (`question.json`). Il n'y a aucun nouveau stockage : la sauvegarde, la
   restauration et le statut « jouée » sont acquis sans code supplémentaire.
3. **Le mot de la manche ne transite jamais par `GameState` avant la révélation** (patron RAFALE §2.3). Il passe par
   l'action dédiée `PICTO_WORD`, envoyée au seul dessinateur (`SendToPlayerID`) et aux seuls admin et animateur
   (`BroadcastToTypes(Admin, Anim)`). Seul le mot **révélé** entre dans `GameState` (`PICTO_REVEALED_WORD`).
4. **Les mots de la question sont retirés** des messages `/ws/tv` et `/ws/player` (`QUESTION.PICTO_WORDS`), sur les
   trois sites de sérialisation (patron ROUE §9, `ROUE_PHRASE`).
5. **Le dessin est relayé hors moteur**. L'action entrante `PICTO_STROKES` vient du seul VJoueur dessinateur. Le
   serveur la vérifie, l'ajoute au **journal de feuille**, puis la **relaie** à la TV, à l'admin et à l'animateur.
   Il ne la relaie **jamais aux tablettes** et **jamais par un `UPDATE`**. Le moteur ne voit que l'époque (§8).
6. **Relais au mieux** **[planner]** : un client dont le tampon d'envoi est plein **perd la trame de traits** au lieu
   d'être déconnecté. Il détecte le trou par `SEQ` et redemande la feuille entière (§8.5). Le comportement de
   `BroadcastToTypes` est inchangé pour tous les autres messages.
7. **Désignation du dessinateur en préparation seulement**, par l'animateur ou l'admin : **choix direct** dans la
   liste des joueurs connectés de l'équipe, **ou tirage au hasard** parmi eux, rejouable tant que la manche n'a pas
   démarré (Q3 révisée, révision 6). En cours de manche, le dessinateur ne change pas ; le recours est la relance de
   la question (§6.6).
8. **Tout se règle dans la question** (Q8, Q9) : aucune section `game-config.json`, aucun 🔗/✏️.
9. **Crédit par le mécanisme existant** (Q4, Q5) : `TEAM_POINTS`, `AnimCreditControl` et le verrou `AWARDED_TEAMS`
   (#170). Le montant est celui de la question (`POINTS`), ajustable sur la ligne de crédit. **Aucune action
   WebSocket nouvelle pour le score.**
10. **Buzzers ignorés** pendant toute la manche (précédents MEMORY / MEMOTION / RAFALE / ROUE dans
    `ProcessButtonPress`) : la devinette est orale.

## 3. Mots de la question et tirage

### 3.1 Règles d'un mot — une seule implémentation, côté serveur (l'éditeur affiche le refus du serveur)

- **Format** : espaces de début et de fin retirés, espaces internes compactés, **1 à 40 caractères**. Caractères
  admis : lettres (accents compris), chiffres, espace, apostrophe, tiret. Tout autre caractère est refusé.
- **Clé repliée** : majuscules et accents repliés, avec le même repli que `rafaleAlphaKey`
  (`internal/game/rafale_alpha.go`), **factorisé et non dupliqué**. « Éléphant » et « elephant » ont la même clé.
- **Liste** : 1 à 20 mots par question. **Aucun doublon** de clé repliée dans une même question (I7, côté question).

### 3.2 Tirage au `START` — tirage libre (décision utilisateur, révision 5)

- Le mot de la manche est **tiré au hasard parmi tous les mots de la question**, à **chaque** `START` : premier
  lancement comme relance. Tirage uniforme avec `rand.Intn`, le générateur global.
- **Aucune mémoire** des mots déjà dessinés en v13.0.0 : le même mot peut ressortir après une relance ou dans une
  autre partie.
- **Un seul mot** dans la question : c'est toujours le mot de la manche.
- Les tests portent sur des **propriétés** (« le mot tiré appartient à la question »), jamais sur un mot attendu : le
  générateur global n'est pas injectable (cf. #188).
- En v13.1.0, la pioche dans le réservoir prendra en compte les mots déjà présentés grâce à des marquages
  persistants (§12). Ce mécanisme ne concerne pas les mots saisis de v13.0.0.

## 4. Modèle de données — question de type `PICTO`

Champs **réutilisés** de `Question` :

| Champ existant | Usage en PICTO |
|---|---|
| `TIME` | Durée de dessin de la manche, en secondes, **obligatoire, de 30 à 300**. L'éditeur pré-remplit **90** à la création |
| `POINTS` | Points d'un mot trouvé (Q5). Pré-rempli à la valeur par défaut de l'éditeur, comme pour tous les types |
| `CATEGORY` (générique) | Catégorie de la question (sélection unique), affichée comme indice sur la TV et utilisée pour le crédit (§7) |
| bloc son de question | Existant, inchangé |

Champ **nouveau** dans `TypedContent` (`omitempty`, garde de round-trip des fixtures, `rafale.md` §3.3) ; c'est
l'`OwnedFields` du type `PICTO` :

| Champ JSON | Type | Règle |
|---|---|---|
| `PICTO_WORDS` | string[] | 1 à 20 mots (§3.1). **Secret** : retiré des messages TV et tablette (§10) |

Registre (`question-types.md` §7, `question_types.go`) :

| Propriété | Valeur | Raison |
|---|---|---|
| `OwnedFields` | `PICTO_WORDS` | — |
| `MediaSlots` | aucun | Pas de média : le mot est du texte, la feuille est produite en jeu |
| `NestableInMotionCard` | `false` | Une manche demande un dessinateur et un relais, ce qui n'a pas de sens dans une carte MEMOTION |
| `HasPlayerInput` | `true` | `PICTO_STROKES` est une entrée joueur (VJoueur) |
| `DefaultPointsRule` | vide | Sans objet hors carte |

**Déclaré dans les sources qui doivent rester cohérentes**, sous la protection du test d'exhaustivité :
- `questionTypeRegistry` et `AllQuestionTypes()` ;
- `QuestionTypePicto = "PICTO"` (`models.go`) ;
- `questionTypeMeta.js` : `{ key: 'PICTO', label: 'Picto', icon: '✏️', nestable: false }`.

**Le type n'est pas ajouté à `generableQuestionTypes`** : la génération IA des mots est reportée en v13.1.0 et
portera sur le réservoir (§12).

## 5. Champs `GameState`

Tous **sans `omitempty`**, tableaux jamais `null`, **jamais persistés** (commentaire d'exclusion dans
`state_persistence.go`). Diffusés à tous les clients web (admin, anim, TV, tablettes) et **jamais aux buzzers**.

| Champ | Type | Contenu |
|---|---|---|
| `PICTO_TEAM` | string | Équipe qui dessine (`""` tant qu'elle n'est pas choisie) |
| `PICTO_TEAM_COLOR` | couleur (même forme que `ROUE_CURRENT_TEAM_COLOR`) | Accent de l'équipe |
| `PICTO_DRAWER` | string | ID du bumper VJoueur dessinateur (`""` sinon) |
| `PICTO_DRAWER_NAME` | string | Nom affiché du dessinateur |
| `PICTO_DRAWER_CONNECTED` | bool | Le dessinateur a une connexion `/ws/player` liée (#246) |
| `PICTO_CANDIDATES` | `{ "ID": string, "NAME": string, "CONNECTED": bool }[]` | VJoueurs de `PICTO_TEAM`, dans l'ordre des bumpers. Seuls ceux avec `CONNECTED = true` sont sélectionnables ; les buzzers physiques n'y figurent pas |
| `PICTO_WORDS_COUNT` | int | Nombre de mots de la question (préparation, affiché sur l'admin et l'anim) |
| `PICTO_CANVAS_EPOCH` | int | Époque de la feuille (§8.4) : +1 à chaque `START` d'une manche PICTO, jamais remise à zéro pendant la vie du serveur ; 0 avant la première manche |
| `PICTO_OUTCOME` | string | `""` \| `FOUND` \| `TIMEOUT` \| `STOPPED` \| `NO_WORD` (§6.4) |
| `PICTO_REVEALED_WORD` | string | Mot de la manche, **seulement en `REVEALED`** ; `""` dans toute autre phase |
| `PICTO_REVEALED_CATEGORY` | string | Catégorie du crédit (§7), en `REVEALED` ; `""` sinon |

**Remises à vide** :
- La préparation (`PICTO_TEAM`, `PICTO_DRAWER`, `PICTO_CANDIDATES`) est remise à vide au **changement de question**,
  à `InitGameWith` et à `NEW_GAME`. Elle est **conservée** entre `PREPARE` et `READY`, et lors d'une **relance de la
  même question** (§6.6) **[planner]**.
- `PICTO_OUTCOME`, `PICTO_REVEALED_WORD` et `PICTO_REVEALED_CATEGORY` sont remis à vide au changement de question,
  à la relance, à `InitGameWith` et à `NEW_GAME`. Ils **survivent** à `stopUnsafe`, jusqu'à `REVEALED`.

## 6. Déroulé d'une manche

### 6.1 Préparation — `PREPARE` / `READY`

| Étape | Action | Émetteurs | Effet |
|---|---|---|---|
| Choisir l'équipe | `PICTO_SET_TEAM {TEAM}` | admin, anim | Pose `PICTO_TEAM` et sa couleur, recalcule `PICTO_CANDIDATES`. **Le dessinateur est effacé** s'il n'est pas de cette équipe. `TEAM` vide efface l'équipe. Équipe inconnue : `PICTO_ERROR INVALID_TEAM` |
| Choisir le dessinateur | `PICTO_SET_DRAWER {PLAYER}` | admin, anim | `PLAYER` est l'ID d'un bumper de `PICTO_CANDIDATES` avec `CONNECTED = true`. Sinon : `PICTO_ERROR NOT_A_CANDIDATE` ou `DRAWER_OFFLINE`. Rejouable à volonté ; `PLAYER` vide efface le dessinateur. **Refusé hors `PREPARE` / `READY`** (`WRONG_PHASE`) |
| Tirer le dessinateur au hasard | `PICTO_DRAW_DRAWER {}` | admin, anim | Le serveur tire au hasard (`rand.Intn`, tirage uniforme) un bumper de `PICTO_CANDIDATES` avec `CONNECTED = true`, et le pose comme dessinateur. Le tirage n'a **aucune mémoire** : il peut retomber sur le dessinateur déjà désigné. Il est **rejouable** tant que la manche n'a pas démarré. Sans équipe : `PICTO_ERROR INVALID_TEAM` ; sans joueur connecté : `PICTO_ERROR NO_CONNECTED_PLAYER`, aucune désignation silencieuse. **Refusé hors `PREPARE` / `READY`** (`WRONG_PHASE`) |
| Garde `PREPARE → READY` et `FORCE_READY` (`participantsConform`) | — | serveur | `READY` seulement si `PICTO_TEAM` est défini, si `TIME` est entre 30 et 300 et si la question a au moins un mot. **Le dessinateur n'est pas exigé pour `READY`** : on le désigne « au READY » (révision 6) **[planner]**. Après chaque `PICTO_SET_TEAM` accepté, la phase est réévaluée (patron `ROUE_SET_TEAMS`) |
| Garde de lancement (`START` en `READY`) | — | serveur | `START` est **refusé** sans dessinateur désigné et connecté (`PICTO_ERROR NO_DRAWER`, l'état reste inchangé). Les boutons « Lancer » de l'admin et de l'anim sont grisés tant que `PICTO_DRAWER` est vide ou que `PICTO_DRAWER_CONNECTED = false` **[planner]** |

Le mot **n'est pas** remis pendant la préparation : le dessinateur sait seulement qu'il va dessiner.

### 6.2 Lancement — `COUNTDOWN` → `STARTED`

Au passage en `STARTED` (fin du décompte existant), sous le verrou du moteur :
1. `PICTO_CANVAS_EPOCH + 1`, et le journal de feuille est vidé (§8.3).
2. Tirage libre du mot de la manche (§3.2). **Aucun mot** (question modifiée à la main entre `READY`
   et `START`) : c'est le filet de sécurité. La manche reçoit l'issue `NO_WORD` et s'arrête immédiatement, avec un
   journal.
3. Hors verrou : `PICTO_WORD` est envoyé au dessinateur et à l'admin / animateur (§9.2), puis un `UPDATE` général.

Le chronomètre global (`TIME` → `CURRENT_TIME`) démarre comme pour toute question. Le son de décompte existant
s'applique.

### 6.3 Dessin — `STARTED`

| Déclencheur | Émetteurs | Phases | Effet |
|---|---|---|---|
| `PICTO_STROKES` | VJoueur **dessinateur** | `STARTED` | §8 : la trame est vérifiée, ajoutée au journal et relayée. Aucun `UPDATE` |
| `PICTO_FOUND {}` | admin, anim | `STARTED`, `PAUSED` | « Trouvé ! » : fin de manche avec l'issue `FOUND` (§6.4) |
| `PAUSE` / reprise (existants) | admin | — | Chronomètre suspendu. `PICTO_STROKES` est refusé en `PAUSED` : la feuille du dessinateur est verrouillée |
| `STOP` (existant) | selon les droits actuels | `STARTED`, `PAUSED` | Abandon : fin de manche avec l'issue `STOPPED` (§6.4) |
| Appui `BUTTON` (buzzer, tablette, simulation) | — | toutes | **Ignoré** : aucun effet, aucun `UPDATE` général, aucune erreur. L'écho d'état ciblé #248 est conservé pour une tablette |

**La feuille n'est jamais effacée pendant la manche.** Le dessinateur peut continuer par-dessus. Le journal est borné
(§8.3) ; à 300 s de dessin, les bornes laissent une marge d'environ ×10 sur un usage réel. Pour repartir d'une
feuille vierge, l'admin relance la question (§6.6).

### 6.4 Fin de manche — quatre issues, un seul chemin de révélation

| Issue `PICTO_OUTCOME` | Déclencheur | Transition | Crédit |
|---|---|---|---|
| `FOUND` | `PICTO_FOUND` en `STARTED` ou `PAUSED`, **ou en `STOPPED` après `TIMEOUT`** (réponse donnée sur le gong, I5) | arrêt (`stopUnsafe`) **puis** révélation (`Reveal`) dans la **même action**, vers `REVEALED` | ligne de crédit pré-remplie à `POINTS` (§7) |
| `TIMEOUT` | fin du chronomètre global (chemin existant : `OnTimeUp`, puis `Stop()`) | `STOPPED`, mot **pas encore public**. Ensuite, `PICTO_FOUND` mène à `FOUND`, et `REVEAL` existant à `REVEALED` | aucun, sauf si `PICTO_FOUND` suit |
| `STOPPED` | `STOP` existant pendant la manche (abandon) | `STOPPED`. Ensuite, `REVEAL` existant mène à `REVEALED`, ou l'admin relance la question (§6.6) | aucun |
| `NO_WORD` | aucun mot au `START` (§6.2, filet de sécurité) | `STOPPED` immédiat. `REVEAL` mène à `REVEALED`, avec un mot révélé vide | aucun |

- `PICTO_FOUND` en `STOPPED` n'est accepté que si `PICTO_OUTCOME = TIMEOUT`. Dans tous les autres cas, il est refusé
  (`WRONG_PHASE`).
- Une issue posée n'est jamais remplacée, sauf `TIMEOUT` → `FOUND`.
- Le son et l'éclairage « temps écoulé » existants (`OnTimeUp`) sont inchangés.

### 6.5 Révélation — `REVEALED`

À l'entrée en `REVEALED`, quelle que soit l'issue :
- `PICTO_REVEALED_WORD` et `PICTO_REVEALED_CATEGORY` sont posés. C'est le **seul** moment où le mot entre dans
  `GameState`. Les autres mots de la question ne sont jamais révélés.
- La feuille reste affichée (le journal est conservé) jusqu'au changement de question. Le dessinateur garde sa
  feuille, verrouillée.

### 6.6 Enchaînement et relance

- **Question suivante** : l'animateur passe à la question suivante (mécanisme existant). Une nouvelle question PICTO
  repart au §6.1 : **aucune rotation automatique**, aucune mémoire de qui a déjà dessiné. Les champs du §5 et le
  journal de feuille sont vidés.
- **Relance de la même question** (I6) : c'est le recours si le dessinateur décroche ou si la manche est mal partie.
  L'admin sélectionne de nouveau la question, ce qui la ramène en `PREPARE` (mécanisme existant de resélection, garde
  « question déjà jouée » #200 comprise).
  - L'équipe et le dessinateur sont **conservés**. L'admin ou l'animateur peut en changer, puisqu'on est en
    préparation (choix direct ou tirage au hasard). La garde de lancement exige de nouveau un dessinateur
    **connecté**.
  - L'issue, le mot révélé et le journal sont vidés.
  - Au `START` suivant, un **nouveau tirage libre** a lieu (§3.2) : le même mot peut ressortir, et c'est toujours
    le même si la question n'en a qu'un. La feuille repart vierge (nouvelle époque).
  - La tablette du dessinateur revient à l'écran de préparation (« C'est toi qui dessines ! »), puis reçoit le mot au
    `START`.

### 6.7 Déconnexions

| Cas | Comportement |
|---|---|
| Dessinateur déconnecté en `PREPARE` / `READY` | `PICTO_DRAWER_CONNECTED = false` et `PICTO_CANDIDATES` est mis à jour. La phase ne change pas, mais `START` est refusé (§6.1) tant qu'aucun dessinateur connecté n'est désigné : un nouveau choix ou un tirage règle le cas |
| Dessinateur déconnecté en `STARTED` | La manche **continue**, chronomètre inchangé. L'admin et l'anim voient l'alerte « Alice est déconnectée — elle retrouvera sa feuille en revenant ; sinon, relancez la question ». Aucun remplacement en cours de manche |
| Dessinateur reconnecté (même bumper, liaison #246) | `PICTO_WORD` et `PICTO_CANVAS` lui sont renvoyés : il reprend sa feuille et son mot |
| TV, admin ou anim reconnecté pendant la manche | `PICTO_CANVAS` est envoyé à la connexion ; l'admin et l'anim reçoivent aussi `PICTO_WORD` |
| Bumper du dessinateur supprimé, ou équipe renommée / supprimée | Purge : `PICTO_DRAWER` est vidé ; `PICTO_TEAM` suit le renommage ou est vidé (crochets de renommage et de purge, §13) |

## 7. Points

- Le crédit est **manuel**, par le chemin existant. L'équipe qui dessine reçoit une ligne de crédit pré-remplie aux
  **points de la question** (« +N pts »), ajustable, ou « 0 pt ». Sur l'anim, c'est `AnimCreditControl` ; sur l'admin,
  c'est la colonne Équipes. `TEAM_POINTS` met à jour le score d'équipe, émet l'événement `POINTS_AWARDED`, joue
  l'animation de gain et pose le verrou `AWARDED_TEAMS`.
- La ligne est proposée en `REVEALED` avec l'issue `FOUND`. Pour les autres issues, seul « 0 pt » est proposé sur
  l'anim ; l'admin garde ses outils d'attribution existants.
- **Catégorie du crédit** = `PICTO_REVEALED_CATEGORY` = `CATEGORY` de la question.
- **Équipe seule** : pas de score individuel pour le dessinateur. Une correction passe par la page Scores existante.

## 8. Canal de dessin

### 8.1 Principe

```
VJoueur dessinateur ──PICTO_STROKES──► serveur : garde (§8.6) ─► journal de feuille (§8.3) ─► relais (§8.5)
                                                                                             ├─► /ws/tv
                                                                                             ├─► /ws/admin
                                                                                             └─► /ws/anim
                                       (jamais /ws/player, jamais le moteur, jamais GameState)
```

### 8.2 Coordonnées — contrat de normalisation (implémenté une fois en Go, une fois en JS)

- La feuille est **carrée**. Un point s'écrit `[x, y]`, en **entiers de 0 à 10 000** :
  `x = round(px / largeur × 10000)`, `y = round(py / hauteur × 10000)`. L'origine est en haut à gauche ; les valeurs
  sont bornées (`< 0` → 0, `> 10000` → 10000).
- L'épaisseur du trait est **fixe** : `PICTO_LINE_WIDTH = 80` unités (0,8 % du côté), extrémités et jonctions
  arrondies. Trait noir sur fond blanc, aucune autre option.
- Rendu : segments droits entre points successifs. Un trait d'un seul point est dessiné comme un disque du diamètre
  du trait.
- **Cas de test nommés à l'identique** en Go (`picto_geometry_test.go`) et en JS (`pictoGeometry.test.js`), avec les
  mêmes valeurs attendues des deux côtés : `normalize_center`, `normalize_corners`, `clamp_outside`, `round_half`,
  `single_point_dot`, `line_width_units`. C'est l'antidote à R5 (dérive Go/JS).

### 8.3 Journal de feuille

- Le serveur garde **la feuille de l'époque courante** : la liste ordonnée des traits et leurs points.
- **Bornes** **[planner, ajustées après la mesure de charge]** : 6 000 traits et 60 000 points par époque. Au-delà,
  `PICTO_STROKES` est refusé (`PICTO_ERROR CANVAS_FULL`) et le dessinateur voit « Feuille pleine ».
- Le journal est vidé au `START` d'une manche, au changement de question et à `NEW_GAME`. Il est **conservé** en
  `REVEALED`.
- Il vit hors moteur (patron REGIE_MESSAGE, « état hors moteur »), protégé par son propre verrou. L'époque est lue
  dans le moteur.

### 8.4 Époque de feuille

- Chaque `PICTO_STROKES` porte `EPOCH`. Un message d'une **autre** époque que `PICTO_CANVAS_EPOCH` est **ignoré en
  silence** : pas d'erreur, pas de relais. C'est le cas d'un trait en vol pendant un changement de manche.
- Les clients (TV, admin, anim) effacent leur feuille quand `PICTO_CANVAS_EPOCH` change dans un `UPDATE`. Ils ignorent
  toute trame `PICTO_STROKES` / `PICTO_CANVAS` d'une autre époque.

### 8.5 Relais au mieux et rattrapage

- Chaque trame relayée porte un `SEQ` qui augmente de 1 par trame **dans l'époque** (1, 2, 3…).
- **Envoi non bloquant dédié** `TrySendToTypes(msg, types...)` **[planner]** : un client dont le tampon `Send` est
  plein **ne reçoit pas cette trame** et **reste connecté**. Seul `PICTO_STROKES` utilise ce chemin ;
  `BroadcastToTypes` est inchangé pour tous les autres messages (la déconnexion au tampon plein est conservée, #127).
- Un client qui reçoit `SEQ ≠ dernier + 1` (trou), ou qui (re)charge la page, envoie `PICTO_CANVAS_SYNC {}`. Le
  serveur lui répond **à lui seul** par `PICTO_CANVAS` (feuille entière de l'époque + `SEQ` courant), puis le client
  reprend le flux.
- `PICTO_CANVAS` est aussi envoyé d'office pendant une manche à la connexion d'une TV, d'un admin ou d'un animateur,
  et au dessinateur qui se reconnecte (§6.7).
- **Cadence côté client** : le VJoueur regroupe ses points et envoie au plus **une trame toutes les 60 ms**, avec au
  plus **200 points** par trame. Il simplifie : un point n'est gardé que s'il est à au moins 15 unités du précédent.
- L'action `PICTO_STROKES` est **exclue du `console.log` systématique** de `sendMessage` (`hooks/useWebSocket.js`).

### 8.6 Garde d'autorisation de `PICTO_STROKES` — règle de sécurité

- L'identité de l'émetteur vient **de sa connexion** (`client.PlayerID`, lié par `BindClientPlayerID` #246),
  **jamais du payload**. Le payload ne contient aucun identifiant de joueur. Depuis la révision 7, la liaison exige
  le **jeton de session** de la place (`contracts/vplayer-session.md`, faille M1 de l'audit du LOT-2.1) : connaître
  l'ID du dessinateur ne suffit plus pour se faire passer pour lui.
- La trame est acceptée seulement si le client est de type `vplayer`, si `PlayerID == PICTO_DRAWER`, si la question
  courante est de type `PICTO`, si la phase est `STARTED`, si `EPOCH` est l'époque courante (sinon ignorée, §8.4) et
  si la forme est valable (§9.1).
- Un refus est **explicite**, envoyé à l'émetteur seul (`PICTO_ERROR`) : `NOT_DRAWER`, `WRONG_PHASE`,
  `INVALID_STROKES`, `CANVAS_FULL`, `RATE_LIMITED`. Une trame refusée n'a jamais d'effet et n'est jamais relayée.
- **Débit** : au plus 40 trames/s par connexion. Au-delà, la trame est refusée (`RATE_LIMITED`, une erreur au plus
  par seconde).
  L'état de débit d'une connexion est **purgé à sa fermeture** (`ForgetClient` appelé par `onClientClosed`, révision 7,
  mineure m1), comme l'état de débit de `PICTO_CANVAS_SYNC`.
- **À ne pas** prendre pour modèle : `FLIP_MEMORY_CARD`, qui ne reçoit aucune identité et ne vérifie aucune équipe.
- **Liste blanche entrante** (`inbound_allowlist.go`) : `PICTO_STROKES` et `PICTO_CANVAS_SYNC` sont ouverts au type
  `vplayer`. Ce sont les **seuls élargissements** de la liste pour ce type. `PICTO_CANVAS_SYNC` est aussi ouvert à
  `tv`, `admin` et `anim`.

## 9. Actions WebSocket

### 9.1 Client → serveur

| Action | Payload | Émetteurs | Phases |
|---|---|---|---|
| `PICTO_SET_TEAM` | `{ "TEAM": string }` | admin, anim | `PREPARE`, `READY` |
| `PICTO_SET_DRAWER` | `{ "PLAYER": string }` (ID de bumper) | admin, anim | `PREPARE`, `READY` |
| `PICTO_DRAW_DRAWER` | `{}` | admin, anim | `PREPARE`, `READY` |
| `PICTO_FOUND` | `{}` | admin, anim | `STARTED`, `PAUSED`, et `STOPPED` après `TIMEOUT` |
| `PICTO_STROKES` | `{ "EPOCH": int, "STROKES": [ { "ID": int, "PTS": [[x, y], ...], "END": bool } ] }` | vplayer (dessinateur) | `STARTED` |
| `PICTO_CANVAS_SYNC` | `{}` | tv, admin, anim ; vplayer **seulement si `PlayerID == PICTO_DRAWER`**, sinon `PICTO_ERROR NOT_DRAWER` (révision 7, mineure m2) | toutes (réponse vide hors manche). **Débit** : au plus 2 demandes/s par connexion, quel que soit l'émetteur ; au-delà, la demande est ignorée (aucune copie de la feuille) et `PICTO_ERROR RATE_LIMITED` est émise **au plus une fois par seconde** et par connexion, puis plus rien jusqu'à la seconde suivante (même règle que pour `PICTO_STROKES`, §8.6) |

Détail de `PICTO_STROKES` :
- `ID` : numéro de trait, attribué par le VJoueur et croissant dans l'époque. Une trame peut poursuivre un trait
  ouvert par la trame précédente (même `ID`, points suivants) et en ouvrir d'autres.
- `END = true` : le trait est fini (doigt levé).
- Forme valable : 1 à 20 traits par trame, 1 à 200 points au total, entiers de 0 à 10 000, `ID` ≥ 1. La trame
  sérialisée reste **très en dessous** de la limite `SetReadLimit(65536)` : 200 points font environ 2,4 Ko.

Pendant une entracte, les actions `PICTO_*` sont refusées (comportement par défaut de la liste blanche d'entracte),
sauf `PICTO_CANVAS_SYNC`.

Admin et animateur peuvent agir en même temps. Le serveur applique la première action valable et refuse la suivante
(`WRONG_PHASE`), sans double application : deux « Trouvé » simultanés ne créent qu'une issue.

### 9.2 Serveur → client

| Action | Destinataires | Payload |
|---|---|---|
| `UPDATE` (existant) | tous les clients web | `GameState` avec les champs du §5 |
| `PICTO_WORD` | **dessinateur seul** (`SendToPlayerID`) + admin + anim (`BroadcastToTypes(Admin, Anim)`) | `{ "WORD": string, "CATEGORY": string, "EPOCH": int }` |
| `PICTO_STROKES` | tv, admin, anim (`TrySendToTypes`, §8.5) | `{ "EPOCH": int, "SEQ": int, "STROKES": [...] }` (même forme que l'entrée) |
| `PICTO_CANVAS` | le demandeur seul | `{ "EPOCH": int, "SEQ": int, "STROKES": [ { "ID": int, "PTS": [[x, y], ...], "END": bool } ] }` |
| `PICTO_ERROR` | l'émetteur seul | `{ "CODE": string, "MESSAGE": string }` |

`PICTO_ERROR.CODE` ∈ `INVALID_TEAM | NOT_A_CANDIDATE | DRAWER_OFFLINE | NO_CONNECTED_PLAYER | NO_DRAWER | WRONG_PHASE | NOT_DRAWER | INVALID_STROKES |
CANVAS_FULL | RATE_LIMITED`.

⚠️ `serializeForClientType` (`websocket.go`) : `ClientTypeAnim` doit figurer au `case` des actions ciblées. Sinon,
l'anim reçoit la charge admin complète (`rafale.md` §2.3).

## 10. Confidentialité du mot — règle non négociable

- Le mot de la manche ne figure **jamais** dans `GameState`. Il ne figure dans aucun message à `/ws/tv`, ni à une
  tablette autre que celle du dessinateur. Il est absent de tout journal diffusé (`/ws/logs`) et de tout message aux
  buzzers. **Seule exception** : `PICTO_REVEALED_WORD` en `REVEALED`.
- `QUESTION.PICTO_WORDS` est **retiré** de tout message envoyé à `/ws/tv` et `/ws/player`, dans **toutes les phases**,
  `REVEALED` compris : les autres mots d'une question à plusieurs mots ne sont jamais montrés.
  - Le retrait s'applique aux trois sites : `SerializeForWebClient`, `SerializeForVPlayer` et le fan-out de
    `cmd/server/main.go`.
  - Il passe par une liste partagée, sur le modèle du retrait de `ROUE_PHRASE`.
  - Les mots restent présents pour `/ws/admin` et `/ws/anim`.
- **Tests exigés** (modèle `internal/protocol/rafale_leak_test.go` et ROUE §9), pour chaque phase de `PREPARE` à
  `STOPPED` :
  - le mot de la manche et `PICTO_WORDS` sont absents des messages sérialisés pour la TV et pour une tablette non
    dessinateur ;
  - le mot de la manche est absent des `UPDATE` de tous les clients ;
  - le dessinateur, l'admin et l'anim reçoivent bien `PICTO_WORD`.

## 11. HTTP — `POST /questions` (multipart, existant), champs ajoutés pour `type=PICTO`

| Champ de formulaire | Champ enregistré | Refus `400` |
|---|---|---|
| `time` (existant) | `TIME` | `PICTO_TIME_INVALID` hors 30 à 300 |
| `points` (existant) | `POINTS` | règles existantes |
| `category` (existant) | `CATEGORY` | règles existantes (catégorie connue) |
| `picto_words` (un mot par ligne) | `PICTO_WORDS` | `PICTO_WORDS_INVALID` : liste vide ou de plus de 20 mots, mot non valable (§3.1), ou doublon dans la question |

`handleUploadQuestion` **relit explicitement** `picto_words` à chaque enregistrement. Sans cela, il le détruirait :
il reconstruit la question de zéro (cf. RAFALE, bugfix 2026-08-31). Aucun autre endpoint n'est ajouté en v13.0.0.

## 12. Reporté en v13.1.0 — décisions déjà prises (milestone #48, issue #318)

Rien de cette section n'est implémenté en v13.0.0. Elle garde les décisions du GATE 2 pour que #318 les reprenne.

- **Source du mot au choix** : `PICTO_SOURCE` ∈ `RESERVOIR` | `WORDS`. ⚠️ **Absent = `WORDS`**, pour que les
  questions créées en v13.0.0, qui n'ont pas ce champ, restent valables. Le défaut de l'éditeur pour une **nouvelle**
  question sera le réservoir (I4), avec un avertissement si le filtre ne trouve aucun mot jamais présenté.
- **Réservoir global** : patron RAFALE (fichier unique typé `data/files/picto/reservoir.json`, `PictoWord {ID, WORD,
  CATEGORY, DIFFICULTY}`). Filtre de question `PICTO_CATEGORIES` × `PICTO_DIFFICULTIES`, prédicat unique.
  **Doublons interdits par catégorie** (I7) : un même mot peut exister dans deux catégories.
- **Pioche** (I2) : seulement parmi les mots **jamais présentés**. Deux marquages persistants
  (`data/config/picto_marks.json`) : `PRESENTED`, posé au `START` (et par la passe du mode RAFALE), et `REVEALED`,
  posé à la révélation ; les deux excluent le mot. **Clé = mot replié** : le même marquage s'applique au mot saisi
  et au mot du réservoir (I3). Remise à zéro par `NEW_GAME` et par « remettre à jouer » (un mot, une liste, tout).
  Ces marquages ne concernent que le réservoir ; les mots saisis restent en tirage libre (§3.2), sauf décision
  contraire en v13.1.0.
- **Onglet ✏️ Picto** de la page Questions : **tableau filtrable comme les autres pools** (maquette M6 A, brouillon
  `_work/mockup/v13.0.0/ui/admin_picto_reservoir__words.html`). Colonnes : statut présenté / révélé avec ↺, collage
  d'une liste, filtre « jamais présentés ».
- **Sauvegarde** : catégorie **« Pools »** qui réunit Rafale (case existante, déplacée) et Picto (maquette M7,
  brouillon `_work/mockup/v13.0.0/ui/admin_backup__pools.html`) ; drapeau `picto` dans `handleBackupSelect`,
  `handleResetSelect`, `detectTARContents` et `handleRestore`.
- **Génération IA des mots** : chemin dédié, infrastructure partagée (patron `rafale-ai-generation.md`), validation
  identique à l'éditeur, refus `409` pendant une manche.
- Endpoints prévus et découpage recommandé de #318 : plan `_work/reports/plan-20261123-picto.md`, section « À ajouter
  au corps de l'issue #318 ».

## 13. Checklist d'accroche backend

1. `models.go` : `QuestionTypePicto`, `AllQuestionTypes()`, champ `TypedContent` `PICTO_WORDS` (`omitempty`).
2. `question_types.go` : entrée du registre du §4 ; `TestQuestionTypeRegistry_Exhaustive` au vert.
3. Champs `GameState` du §5 : initialisés non nil dans `NewEngine`, remis à vide aux sites du §5.
4. Fichier dédié `internal/game/picto.go` (pas `engine.go`) : préparation, garde, tirage libre (§3.2),
   lancement, fin de manche, révélation, relance.
5. `participantsConform` : cas PICTO (§6.1 : équipe, durée, mots), réévalué après `PICTO_SET_TEAM`. Garde de `START`
   (dessinateur désigné et connecté, sinon `NO_DRAWER`). `PICTO_DRAW_DRAWER` : tirage parmi les candidats connectés.
6. `ProcessButtonPress` : cas PICTO, toujours ignoré.
7. Fin du chronomètre global : pose `PICTO_OUTCOME = TIMEOUT` **avant** `Stop()`, au même endroit que `OnTimeUp`.
8. `Reveal` : à l'entrée en `REVEALED` d'une question PICTO, applique le §6.5. `Ready` sur la même question :
   relance du §6.6.
9. Renommage et purge d'équipe, suppression de bumper : `PICTO_TEAM`, `PICTO_DRAWER`, `PICTO_CANDIDATES`.
10. Actions dans `messages.go`, liste blanche entrante par émetteur (§8.6, §9.1), `handleWebMessage`,
    `serializeForClientType`.
11. Retrait de `QUESTION.PICTO_WORDS` sur les trois sites de sérialisation (§10).
12. Journal de feuille et relais : `internal/server/picto_canvas.go` (hors moteur) ; `TrySendToTypes` dans
    `websocket.go`.
13. `SendToPlayerID` pour `PICTO_WORD` ; renvoi à la connexion et à la reconnexion (§6.7).
14. `handleUploadQuestion` (§11) ; repli `rafaleAlphaKey` factorisé.
15. `InitGame()` : rien de propre à PICTO à persister ni à purger (aucun fichier de données en v13.0.0).
16. LED, ambiance, sons (§15).
17. `state_persistence.go` : commentaire d'exclusion des `PICTO_*`.

## 14. Ouverture vers v13.1.0

Rien n'est codé pour v13.1.0, et **aucun champ spéculatif** n'est créé (règle #184). Le modèle ne ferme pourtant pas
la porte :
- **réservoir** (§12) : l'ajout de `PICTO_SOURCE` (absent = `WORDS`) et des champs de filtre ne change rien aux
  questions existantes. Le tirage du §3.2 devient une des deux sources ;
- **mode RAFALE** : un futur `PICTO_MODE` (absent = un mot) ferait enchaîner les mots dans le temps imparti, avec des
  passes réglables (coût, limite, rebouclage ou fin de pool). Une passe = nouvelle époque (feuille vierge, mécanique
  déjà prévue) + marquage `PRESENTED` ;
- se réutilisent **tels quels** : le relais, le rattrapage, l'époque, la confidentialité et le tirage ;
- un « tout effacer » resterait peu coûteux si l'utilisateur revient sur ce choix (nouvelle époque).

## 15. Périphériques, ambiance, sons

| Élément | Comportement |
|---|---|
| Buzzers (LED) | Dès que `PICTO_TEAM` est choisie, et pendant la manche : les buzzers de l'équipe qui dessine sont en `SOLID` à la couleur d'équipe, ceux des autres équipes en `DIM` (helper actif / participants de `led.go`, déjà partagé par MEMORY, RAFALE et ROUE). Révélation, pause, reconnexion : règles existantes. Appui ignoré (§6.3) |
| Éclairage d'ambiance | `ActiveTeam` = `PICTO_TEAM` (scène « tour d'équipe » existante) |
| Sons | Bruitages existants seulement : décompte au départ du chronomètre, « temps écoulé » (`OnTimeUp`), gain au crédit. **Aucun nouveau bruitage** |
| TV | **Structure commune à tous les types** (`game-content-zones`, maquette M3) : **chronomètre commun à tous les types** (composant `Timer` : mm:ss et barre de progression, à sa place habituelle, `zone-timer`) ; **bandeau haut** (`zone-question`) = l'indice, soit la catégorie et « ✏️ *Nom* dessine pour *Équipe* » ; **zone média** (`zone-media`) = la feuille carrée, centrée à ratio fixe ; **zone réponse** (`zone-answers`) = le mot, seulement en `REVEALED`. **Écran statique** : `overflow: hidden`, unités viewport, `min-height: 0` |
| Tablette du dessinateur | Préparation : « C'est toi qui dessines ! » (sans le mot). `STARTED` : le mot en grand + la feuille carrée sur toute la largeur, **aucun bouton**. `PAUSED` : feuille verrouillée, « Jeu en pause ». `REVEALED` : feuille finale + mot. Paysage et portrait |
| Tablettes de l'équipe qui dessine | « Devinez ! *Nom* dessine — regardez la TV », sans feuille |
| Tablettes des autres équipes | « *Équipe* devine le dessin de *Nom* », sans feuille |

## 16. Non-régression exigée

| Zone | Vérification |
|---|---|
| Registre de types | `TestQuestionTypeRegistry_Exhaustive` ; un type inconnu ne retombe pas sur `SPEEDY` (#183) |
| Fixtures | Round-trip octet pour octet des fixtures `testdata/questions/*.json` et des 85 `question.json` de `data/files/questions` (`models_roundtrip_test.go`) |
| `BroadcastToTypes` | Comportement inchangé (déconnexion au tampon plein) pour tout message autre que `PICTO_STROKES` |
| Liste blanche entrante | Aucune action existante n'est ouverte à un nouveau type ; seuls `PICTO_STROKES` / `PICTO_CANVAS_SYNC` sont ajoutés pour `vplayer` |
| Filtres de sérialisation | Le retrait de `ROUE_PHRASE` et des champs de réponse est inchangé ; seul `PICTO_WORDS` est ajouté à la liste |
| Repli `rafaleAlphaKey` factorisé | Suites RAFALE et ROUE au vert |
| LED MEMORY / RAFALE / ROUE | Grilles inchangées |
| Chronomètre global | Questions SPEEDY / QCM / MEMORY : fin de chronomètre inchangée (`OnTimeUp` puis `Stop()`) |
| Resélection d'une question (#200) | Comportement inchangé pour les autres types |
| Types existants | Les 8 types existants : aucune régression (suites complètes) |
