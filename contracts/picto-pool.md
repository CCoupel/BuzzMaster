# Contrat — Pool de mots PICTO sur le moteur de pool généralisé (v13.1.0, milestone #48)

**Issues** : #318 et, selon P1 du plan, une issue fille « pool de mots Picto ». Plan :
`_work/reports/plan-20261124-v1310.md`.
**Statut** : contrat de référence, écrit avant implémentation (contract-first) — **révision 3** (2026-11-25, relecture des docs du LOT-2 : formats de réponse et lecture de la liste
refusée pendant une manche, §4 ; révision 2 : GATE 2 du 2026-11-24).
**Révision 2** (version finale du GATE 2 : collage retenu, case « Pools » à trois états, libellés « Pool Speedy » / « Pool Picto », pool Picto utilisé par les types `PICTO` et `RAFALE_PICTO`) : le pool Picto n'est **plus un magasin parallèle**. Le **moteur de pool RAFALE est généralisé** :
stockage, marquage « utilisé », pioche stratifiée, HTTP, sauvegarde, job et modale IA. Il est instancié deux fois,
`rafale` et `picto`. La révision 1 (magasin `PictoWord` distinct, marquages `PRESENTED` / `REVEALED` par clé
repliée) est **abandonnée**.

**Arbitrages** :
- GATE 1.5 (Q1 : pool + IA + « Pools ») ;
- GATE 2 du 2026-11-24 :
  - P1 : généraliser plutôt que dupliquer ;
  - P4 : tirage **stratifié** catégorie × difficulté (`rafale.md` §7.2) ;
  - P5 (à valider, recommandé) : marquage RAFALE « utilisé » par entrée.

> **Vocabulaire** : « pool » = « réservoir ». L'interface dit **« Pool Speedy »** (pool du type `RAFALE`, affiché
> « Rafale Speedy », `picto.md` §17.13) et **« Pool Picto »** (pool des types `PICTO` et `RAFALE_PICTO`). La catégorie « Pools » de la sauvegarde les
> regroupe.

Les choix techniques du planner sont marqués **[planner]**.

---

## 1. Ce qui est généralisé — un moteur, deux instances

| Brique RAFALE existante | Généralisation | Instance `rafale` | Instance `picto` |
|---|---|---|---|
| `rafale_store.go` (fichier unique typé, écriture atomique, ID `r-NNN`) | Type **`EntryPool`** paramétré : chemins, préfixe d'ID, validateur | `files/rafale/reservoir.json`, `r-NNN` | `files/picto/reservoir.json`, `p-NNN` |
| Marquage `rafale_used.json` (`USED` par ID, posé au tirage, remis à zéro par `NEW_GAME` / reset) | Même mécanisme, un fichier par instance | `config/rafale_used.json` | `config/picto_used.json` |
| Prédicat `rafaleQuestionMatchesFilter`, `CountRafalePool` | `pool.matches(cat, diff)`, `pool.Count(cats, diffs)` | inchangé | idem |
| Pioche stratifiée `rafaleDrawState` (couples, permutation, retrait d'un couple épuisé, §7.2–§7.4) | **`poolDrawState`** + fonctions qui prennent `(pool, state)` | état de la manche RAFALE (pré-tirage, alphabétique : **inchangés**) | état de la manche Picto (§3) |
| Endpoints `/api/rafale/questions*`, `/api/rafale/pool` | **Fabrique de handlers** paramétrée par instance | chemins inchangés | `/api/picto/words*`, `/api/picto/pool` (§4) |
| Sauvegarde : drapeau `rafale` en dur aux 4 sites | **Liste des pools** (`key`, dossier, fichier « utilisé »), parcourue aux 4 sites | clé `rafale` inchangée | clé `picto` |
| `ai_generator_rafale.go` (validation, répartition, lots, contexte existant, persistance) | **Descripteur de génération** : paliers, schéma, prompt, validateur, instance, garde de manche | comportement et prompt **identiques** | §5 |
| `RafaleAIGenerateModal` (formulaire, coquille partagée) | **`PoolAIGenerateModal`** paramétré : endpoint, titre, paliers, `TARGET` | inchangé à l'écran | titre « mots Picto », paliers 10 / 20 / 50 |
| `RafalePage.jsx` (tableau filtrable + formulaire) | **`PoolPage`** paramétrée : colonnes, champs, plafonds, libellés, API | inchangée à l'écran | colonne « Mot », pas de colonne réponse (§6) |
| `RafalePoolAlert` (éditeur de question) | Paramétré par instance | inchangé | éditeur Picto (`picto.md` §17.2) |

**Ce qui n'est pas généralisé** (propre à la manche RAFALE, inchangé) : pré-tirage de la question suivante (§13),
mode alphabétique (§15), cartes MEMOTION RAFALE (§14), compteurs d'équipe.

## 2. Entrée de pool — `PoolEntry` **[planner]**

```go
type PoolEntry struct {         // = RafaleQuestion, renommé ; alias conservé : type RafaleQuestion = PoolEntry
    ID         string           `json:"ID"`
    Question   string           `json:"QUESTION"`   // RAFALE : énoncé ; PICTO : le mot
    Answer     string           `json:"ANSWER"`     // RAFALE : réponse ; PICTO : toujours ""
    Category   QuestionCategory `json:"CATEGORY"`
    Difficulty int              `json:"DIFFICULTY"` // 1..3
}
```

- **Format disque et format réseau de RAFALE inchangés, octet pour octet.** Fichier `{"QUESTIONS": [...]}` pour les
  deux instances.
- Picto range le mot dans `QUESTION` et laisse `ANSWER` vide. C'est un **compromis de nommage assumé** : c'est ce
  qui permet de « pointer vers un second pool » sans type ni code parallèles. L'interface affiche « Mot ». Un type
  générique Go distinct par pool coûterait environ 1 jour de plus pour un gain de lisibilité seulement.
- **Validateur par instance** :
  - RAFALE : inchangé (énoncé ≤ 100, réponse ≤ 40, réponse obligatoire) ;
  - PICTO : `QUESTION` suit les règles du mot (`picto.md` §3.1 : 1 à 40 caractères, jeu de caractères, repli),
    `ANSWER` doit être vide.
- **Doublons interdits par catégorie** dans l'instance `picto` (I7, clé repliée `rafaleAlphaKey`). C'est une règle
  du validateur Picto. RAFALE ne change pas.
- Plafond **[planner]** : 5 000 entrées pour l'instance `picto` (`POOL_FULL`).

## 3. Marquage et pioche — modèle RAFALE (P4, P5)

- **Marquage unique « utilisé »** par ID d'entrée, posé **au tirage**, comme RAFALE §3.2. Un mot tiré est montré au
  dessinateur, donc « utilisé ».
- Il est remis à zéro par `NEW_GAME` (les deux instances, comme aujourd'hui pour RAFALE), par
  `POST /api/picto/words/{id}/reset` et par `POST /api/picto/words/reset-all`.
- **P5 (recommandé)** : on abandonne la distinction présenté / révélé et le marquage par clé repliée commun aux mots
  saisis (I3 de v13.0.0). Les mots saisis dans une question restent en tirage libre, sans marquage (`picto.md`
  §3.2, inchangé).
- **Pioche** : **stratifiée** par couple catégorie × difficulté (`rafale.md` §7.2 à §7.4, P4), avec un état de
  tirage propre à la manche Picto.
  - Un couple épuisé sort de la liste active ; il y a une alerte, jamais de blocage (§7.3).
  - Picto (un mot) : un seul tirage.
  - Rafale Picto : tirages successifs dans la manche, puis rebouclage sur les passés (`picto.md` §17.6). Le
    rebouclage est **propre à Picto**, hors moteur de pool.
- **Prévention** (`rafale.md` §7.5, paramétrée) :
  - `disponibles = 0` bloque `READY` / `START` (`POOL_EMPTY`) ;
  - en Rafale Picto, un avertissement si `disponibles < plafond(TIME / 10)` **[planner]**.

## 4. HTTP — instance `picto` (mêmes handlers que RAFALE, chemins propres)

Mêmes formes de requête et de réponse que `rafale.md` §9 ; seuls les chemins et le validateur changent. Sans
authentification (#259, dette connue).

| Méthode et chemin | Équivalent RAFALE | Réponse |
|---|---|---|
| `GET /api/picto/words?categories=A,B&difficulty=N` | `GET /api/rafale/questions` | `{ "QUESTIONS": [ {ID, QUESTION, ANSWER:"", CATEGORY, DIFFICULTY, USED} ], "TOTAL": n }` |
| `POST /api/picto/words` | `POST /api/rafale/questions` | `{ "ID": "p-001" }` · `400` : `WORD_INVALID`, `CATEGORY_UNKNOWN`, `DIFFICULTY_INVALID`, `DUPLICATE`, `POOL_FULL` |
| `DELETE /api/picto/words/{id}` | idem | `{ "DELETED": "p-001" }` · `404` |
| `POST /api/picto/words/{id}/reset` | idem | `{ "ID": "p-001", "AVAILABLE": true }` |
| `POST /api/picto/words/reset-all` | idem | `{ "RESET": n }` |
| `GET /api/picto/pool?categories=A,B&difficulties=1,2` | `GET /api/rafale/pool` | `{ "AVAILABLE", "USED", "TOTAL" }` |
| `POST /api/picto/words/batch` (M1 A retenue au GATE 2) | — (nouveau, dans la fabrique, désactivé pour RAFALE). Corps `{ "CATEGORY", "DIFFICULTY", "WORDS": [string, 1 à 200] }`. Lignes valides créées en une seule sauvegarde ; refus listés, jamais tronqués ; doublon interne au lot refusé dès sa 2ᵉ occurrence | `{ "CREATED": [{ID, QUESTION}], "REFUSED": [{QUESTION, CODE}] }` |
| `POST /api/picto/generate-words` | `POST /api/rafale/generate-questions` | `202` (§5) |

**Refus pendant une manche** **[planner]** (`poolGuardPicto`, `pool_http.go`) :
- **Instance `picto` seulement.** Pendant une manche de la famille Picto (`IsPictoFamily`, phase `STARTED` ou
  `PAUSED`), les opérations suivantes sont refusées en `409 picto_round_in_progress` :
  - **la lecture de la liste** `GET /api/picto/words` (révision 3, décision `ec8ab5d1`, #324). La liste porte
    `USED`, et le dernier mot « utilisé » est le mot secret en jeu ;
  - **toutes les écritures** : `POST /api/picto/words`, `POST /api/picto/words/batch`,
    `DELETE /api/picto/words/{id}` ;
  - les remises à disposition : `POST /api/picto/words/{id}/reset` et `/reset-all` ;
  - la génération IA.
- **Lisibles hors manche et à l'arrêt** : en `PREPARE`, `READY`, `COUNTDOWN`, `STOPPED` et `REVEALED`, et quand la
  question courante n'est pas de la famille Picto.
- `GET /api/picto/pool` (comptes `AVAILABLE` / `USED` / `TOTAL`, sans aucun mot) n'est **pas** gardé.
- C'est la même garde, paramétrée, que `rafale_round_in_progress` (`rafale-ai-generation.md` §7).
- **Pool Speedy non concerné** : l'instance `rafale` n'a aucune garde sur ses lectures ni sur ses écritures ; sa
  génération IA garde son refus existant.

**Formats de réponse** (révision 3, alignés sur le code `pool_http.go` / `http_picto.go`) :

| Cas | Statut | Corps |
|---|---|---|
| Refus de validation d'un ajout ou d'une modification (`POST /api/picto/words`) | `400` | **texte brut** `CODE: message` (ex. `DUPLICATE: …`, `WORD_INVALID: …`, `CATEGORY_UNKNOWN: unknown CATEGORY`, `DIFFICULTY_INVALID: …`, `POOL_FULL: …`). Le client lit le code avant le premier `:` |
| Requête mal formée (JSON illisible, lot vide ou de plus de 200 mots, `ID` manquant, filtre invalide) | `400` | texte brut, **sans code** (ex. `Invalid JSON body`, `WORDS must contain 1 to 200 words`) |
| Collage (`POST /api/picto/words/batch`), lot accepté | `200` | **JSON** `{ "CREATED": [{ "ID", "QUESTION" }], "REFUSED": [{ "QUESTION", "CODE" }] }`. Les refus ligne par ligne sont **dans le JSON**, jamais en `400`. Une catégorie ou une difficulté de lot invalide reste un `400` texte `CODE: message`, et rien n'est créé |
| Manche Picto en cours (liste, écritures, remises, génération) | `409` | **JSON** `{ "status": "error", "code": "picto_round_in_progress", "message": "Une manche Picto est en cours — le Pool Picto est verrouillé jusqu'à sa fin." }` (même enveloppe que les erreurs de génération IA, `writeGenerateError`) |
| ID inconnu / méthode | `404` / `405` | texte brut |

## 5. Génération IA — descripteur `picto`

Le chemin, le job unique (`aiJobRegistry`), les lots, les relances, la répartition par couple, la progression
(`AI_GENERATION_PROGRESS`) et l'annulation sont **ceux de RAFALE**, sans copie. Le descripteur `picto` fournit :
- **paliers** `{10, 20, 50}` : constante partagée Go ↔ JS, filtrée par `ai.max_questions` ;
- **schéma** plat `{ "QUESTIONS": [ { "QUESTION": "<mot>", "CATEGORY", "DIFFICULTY" } ] }` ;
- **prompt requalifié** : mot ou expression courte (au plus 3 mots), **dessinable**, sans nom propre ni marque, dans
  la langue demandée ; les mots existants du couple visé sont transmis en contexte ;
- **validateur** = celui de l'éditeur (§2) : un mot invalide ou un doublon de catégorie est **écarté, jamais
  tronqué** ;
- `TARGET = "PICTO"` dans `AI_GENERATION_PROGRESS` ;
- garde `409 picto_round_in_progress`.

Requête identique à RAFALE (`theme`, `populations`, `language`, `instructions`, `categories`, `difficulties`,
`count`).

Critère non négociable : **le prompt, le schéma et les réponses du chemin RAFALE sont inchangés**. Les tests
`ai_generator_rafale_203_test.go` et `RafaleAIGenerateModal.test.jsx` passent sans modification (hors renommage
d'import). Les tests `AIGenerateModal*` sont inchangés.

## 6. Interface — page de pool paramétrée

- **Onglet ✏️ Picto** de `QuestionsPage` (`?tab=picto`) = `PoolPage` avec l'instance `picto`. C'est la **même page
  que l'onglet Rafale** (DECISIONS « pools » : même présentation pour tous les pools) :
  - tableau filtrable (catégories, difficulté, « Non utilisés »), « ↺ Remettre tout disponible (K) » ;
  - formulaire « Nouveau mot » à droite ;
  - « ✨ Générer via IA » dans l'en-tête.
- Colonnes Picto : Mot · Catégorie · Diff. · État · actions. Il n'y a pas de colonne réponse.
- **Collage d'une liste** (M1 A retenue) : le formulaire de droite propose « Un mot » ou « Une liste ». Le collage
  n'existe que pour le Pool Picto (option de la fabrique, désactivée pour le Pool Speedy). Les refus sont listés dans le
  formulaire.
- Pendant une manche Picto : message explicite, actions d'écriture grisées (§4).

## 7. Sauvegarde, restauration, réinitialisation — catégorie « Pools »

- Les 4 sites (`handleBackupSelect`, `handleResetSelect`, `detectTARContents`, `handleRestore`) parcourent la
  **liste des pools** au lieu du drapeau `rafale` codé en dur.
- Clé `rafale` (`files/rafale`, `config/rafale_used.json`) **inchangée** ; nouvelle clé `picto` (`files/picto`,
  `config/picto_used.json`).
- `BackupPage` : catégorie « Pools » (M2 B retenue) = **case parente à trois états** qui déplie « Pool Speedy »
  (case existante, même clé `rafale`) et « Pool Picto ». Cela vaut en sauvegarde comme en réinitialisation.
- Une archive antérieure (sans `files/picto`) se restaure à l'identique.

## 8. Checklist d'accroche backend

1. `EntryPool` (store, marquage « utilisé », validateur, préfixe) ; `RafaleQuestion` devient un alias ; deux
   instances dans l'`Engine` (`e.rafalePool`, `e.pictoPool`). Les références `e.rafaleQuestions` / `e.rafaleUsed`
   (~40, 4 fichiers) passent par l'instance `rafale`.
2. Pioche stratifiée : `poolDrawState` et ses fonctions prennent `(pool, state)`. La manche RAFALE garde son état
   (`e.rafaleDraw`), son pré-tirage et l'alphabétique, sans changement de comportement.
3. Fabrique de handlers HTTP ; routes `picto` ; garde de manche paramétrée.
4. Liste des pools pour la sauvegarde (4 sites).
5. Descripteur de génération ; extraction du chemin RAFALE vers le descripteur `rafale` (prompt identique) ;
   descripteur `picto`.
6. `NEW_GAME` remet à zéro les deux instances.
7. `isKnownRafaleCategory` → `isKnownCategory` (alias conservé).

## 9. Non-régression exigée

| Zone | Vérification |
|---|---|
| RAFALE (pool, pioche, manche, alphabétique, pré-tirage, cartes MEMOTION) | Les ~20 fichiers de test `rafale_*` passent **sans modification** (hors renommage mécanique) |
| Fichiers RAFALE | `reservoir.json` et `rafale_used.json` relus et réécrits octet pour octet |
| API RAFALE | Chemins, formes et codes d'erreur inchangés |
| IA RAFALE | Prompt, schéma, répartition et paliers inchangés ; job unique croisé RAFALE ↔ Picto ↔ Quiz (`409 generation_in_progress`) |
| Sauvegarde | Archive v13.0.0 restaurée à l'identique ; clé `rafale` inchangée |
| Questions v13.0.0 | Une question PICTO sans `PICTO_SOURCE` reste en source `WORDS`, en tirage libre |
