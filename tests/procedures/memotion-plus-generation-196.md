# Procédure de Test — Génération IA MEMOTION+ (#196, v7.1.0)

**Version** : v7.1.0 (branche `milestone/v7.1.0`)
**Date** : 2026-08-26, complétée 2026-08-27 (Scénario 5, cycle 2 — bugfix QUALIF v7.1.0.7)
**Testeur** : QA
**Issue** : #196 — nouvelle clé de distribution `MEMOTION_PLUS` (affichée « MEMOTION+ ») dans la
modale de génération IA : mélange de cartes SPEEDY/QCM choisies carte par carte par le modèle,
**mais jamais persisté en tant que type distinct** — une question générée depuis MEMOTION+ est
toujours écrite avec `TYPE: "MEMOTION"` sur disque. Le pseudo-type n'existe que pendant la
génération, jamais dans l'éditeur ni dans un `question.json`.
**Référence** : `contracts/ai-generation.md` §3ter, `_work/handoff/dev-backend-20260826-210653.md`
(SHA `658e5471`), `_work/handoff/dev-frontend-20260826-205623.md` (SHA `57e158ca`),
`_work/handoff/dev-backend-20260827-205646.md` (SHA `09bbd848` — cycle 2, schéma allégé par
distribution active, corrige un faux "rate limit" immédiat)

---

## ⚠️ Point d'attention central — ne pas confondre pseudo-type et type réel

`MEMOTION_PLUS` n'apparaît **que** dans la modale de génération (liste déroulante/sliders de
répartition). Il ne doit **jamais** apparaître :
- dans l'éditeur `/admin` → Questions (sélecteur de type d'une question ou d'une carte) ;
- dans un badge `QuestionCard` ou sur `PlayerDisplay` ;
- dans le contenu d'un fichier `question.json` (champ `TYPE` ou n'importe quel autre champ).

Si l'un de ces trois symptômes est observé, c'est un **bug bloquant** (fuite du pseudo-type),
pas une variation mineure — contrat §3ter, invariant central.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `milestone/v7.1.0`)
- [ ] Accès admin (`/admin` ou `/anim`) → `QuestionsPage`, bouton « ✨ Générer via IA »
- [ ] Une clé API valide configurée pour la génération (fournie hors de ce document — ne jamais la
      committer ni la coller dans un rapport QA)
- [ ] Au moins une catégorie existante dans le quiz cible
- [ ] Accès à `server-go/data/files/questions/` (ou équivalent QUALIF) pour inspecter les
      `question.json` générés — nécessaire pour vérifier l'absence de fuite du pseudo-type
- [ ] Postes `/anim` et `/tv` disponibles pour le Scénario 3 (carte QCM jouable de bout en bout)
- [ ] Rejouer `tests/procedures/generateur-ia.md` (non-régression générale du générateur, hors
      scope MEMOTION+) si ce n'est pas déjà fait sur ce build

---

## Scénario 1 — Activer MEMOTION+ dans la répartition (0% → X%), génération mixte SPEEDY/QCM

**Objectif** : Vérifier que MEMOTION+ apparaît dans la modale comme une ligne de répartition à part
entière (icône/libellé « MEMOTION+ »), désactivée à 0% par défaut, et qu'une génération avec
MEMOTION+ activé produit des questions dont les cartes mélangent bien SPEEDY et QCM.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la modale « ✨ Générer via IA » | La ligne de répartition affiche 6 types au total (SPEEDY/QCM/MEMORY/MEMOTION/**MEMOTION+**/ARDOISE), MEMOTION+ juste après MEMOTION, désactivée (0%, curseur grisé) par défaut | | |
| 2 | Activer MEMOTION+ (curseur/case à cocher) et lui affecter une part significative (ex: 30-40%), en désactivant/réduisant d'autres types pour que la somme reste 100% | Le bouton « ✨ Générer » reste actif (répartition valide, somme = 100%) | | |
| 3 | Lancer une génération (volume ≥ 5 questions pour avoir une chance raisonnable d'obtenir plusieurs cartes MEMOTION+) | Génération en cours puis liste de questions mise à jour, panneau de succès affiché | | |
| 4 | Ouvrir une question MEMOTION générée par ce lot dans l'éditeur | La question s'ouvre normalement en type `MEMOTION` (pas d'erreur, pas de type inconnu) | | |
| 5 | Observer les cartes de cette question | Le mélange attendu : certaines cartes affichent un contenu SPEEDY (thème + réponse texte), d'autres un contenu QCM (4 réponses à choix) — le type est choisi carte par carte, aucun contrôle de ratio n'est exposé côté utilisateur | | |
| 6 | Ouvrir le `question.json` correspondant (fichier sur disque) | `"TYPE": "MEMOTION"` — **jamais** `"MEMOTION_PLUS"`. Rechercher la chaîne `MEMOTION_PLUS` dans le fichier entier : **absente** | | |
| 7 | Observer le panneau de succès de la modale (liste des questions créées) | Le type reporté pour cette question est `MEMOTION` (jamais le pseudo-type) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Aucune fuite du pseudo-type dans l'éditeur ou les badges

**Objectif** : Confirmer que MEMOTION+ n'existe que dans la modale de génération, nulle part
ailleurs dans l'interface (contrat §3ter — `QUESTION_TYPES`/`questionTypeRegistry` ne doivent
**jamais** connaître ce pseudo-type).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans `QuestionsPage`, ouvrir le sélecteur de type d'une question (nouvelle question ou existante) | La liste des types proposés est la liste **habituelle** (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE) — **pas** de « MEMOTION+ » dans cette liste | | |
| 2 | Observer le badge de type sur la carte de la question générée en Scénario 1 (liste des questions, `QuestionCard`) | Badge « MEMOTION » standard — jamais un badge « MEMOTION+ » | | |
| 3 | Charger cette question en jeu, observer `PlayerDisplay`/`/tv` | Rendu MEMOTION standard, rien de spécifique à MEMOTION+ visible côté joueur | | |
| 4 | Dans l'éditeur, ouvrir le sous-éditeur d'une carte QCM issue de cette génération | Sous-éditeur QCM standard (mêmes champs qu'une carte QCM créée manuellement, cf. `memotion-card-type-184.md`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Une carte QCM générée par MEMOTION+ est jouable de bout en bout

**Objectif** : Vérifier qu'une carte QCM issue de la génération IA se comporte exactement comme une
carte QCM créée manuellement (#185) — la génération ne doit produire aucune carte dans un état
dégradé ou incomplet.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger en jeu la question MEMOTION générée contenant au moins une carte QCM, LANCER | Grille visible sur `/anim` et `/tv` | | |
| 2 | Sélectionner la carte QCM, DÉMARRER | Sous-phase `QUESTION` : les 4 réponses générées s'affichent sur `/anim` **et** `/tv`, aucun champ vide ou mal formé | | |
| 3 | Si indices activés sur la carte (ou activer un chrono assez long), laisser le chrono descendre | Indices progressifs comme une carte QCM classique (une réponse fausse s'invalide à la fois, jamais la bonne) — cf. `memotion-qcm-card-185.md` Scénario 2 | | |
| 4 | Taper RÉVÉLER | La bonne réponse générée est mise en évidence — cohérente avec le `QCM_CORRECT` produit par le modèle (une des 4 réponses affichées, jamais une réponse hors liste) | | |
| 5 | Désigner l'équipe gagnante | Carte `DONE`, points attribués normalement, retour à `GRID` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression : MEMOTION à 0% MEMOTION+ génère exactement comme avant #196

**Objectif** : Vérifier que le comportement de la clé `MEMOTION` (sans MEMOTION+) est **strictement
inchangé** — le contrat garantit cette non-régression *par construction du schéma* (une carte
MEMOTION classique ne peut structurellement pas porter de `TYPE`), ce scénario en est la
confirmation manuelle.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la modale, laisser MEMOTION+ à 0% (désactivé), activer MEMOTION seul (ex: 100%, ou dans un mélange avec d'autres types classiques) | Répartition valide, MEMOTION+ reste grisé/à 0% | | |
| 2 | Lancer une génération | Questions MEMOTION produites normalement | | |
| 3 | Ouvrir le `question.json` d'une question MEMOTION générée par ce lot | Chaque carte a exactement les champs `RECTO_THEME`/`QUESTION_TEXT`/`ANSWER_TEXT`/`DIFFICULTY` — **aucune carte ne porte de champ `TYPE`** (ni `"SPEEDY"` explicite, ni autre) — forme byte-identique à une génération MEMOTION d'avant #196 | | |
| 4 | Observer les cartes en jeu (`/anim`/`/tv`) | Toutes les cartes sont rendues comme des cartes SPEEDY classiques, aucune grille QCM n'apparaît jamais dans une question générée en MEMOTION seul (sans MEMOTION+) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Non-régression bug QUALIF v7.1.0.7 : pas de faux "rate limit" dès le 1er appel (cycle 2)

**Objectif** : Reproduire les conditions exactes du bug rapporté en QUALIF
(`_work/handoff/dev-backend-20260827-205646.md`, SHA `09bbd848`) et vérifier qu'il ne se manifeste
plus. **Cause** : le schéma envoyé au fournisseur IA embarquait systématiquement les 6 branches de
types (SPEEDY/QCM/MEMORY/MEMOTION/MEMOTION+/ARDOISE) à chaque appel, quelle que soit la
répartition réellement demandée — MEMOTION+ (variante la plus volumineuse) faisant basculer la
requête au-dessus du seuil de taille du fournisseur **dès le tout premier appel**, sans aucun
quota réellement consommé. Le message affiché ("rate limit exceeded") était donc **trompeur** :
une vraie erreur 413 (requête trop grande), pas un 429 (quota épuisé). **Fix** : le schéma
n'embarque désormais que les types réellement actifs (`> 0`) dans la répartition demandée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la modale, activer **plusieurs types simultanément dont MEMOTION+** (ex: SPEEDY 20% / QCM 20% / MEMORY 20% / MEMOTION+ 40%) — conditions exactes du bug rapporté | Répartition valide (somme 100%) | | |
| 2 | Lancer une génération, **dès la toute première tentative de la session** (pas de génération précédente sur ce même provider avant celle-ci) | La génération démarre normalement — **aucune erreur "rate limit" / "quota" immédiate** | | |
| 3 | Si une erreur survient malgré tout (vraie saturation du fournisseur) | Le message distingue désormais explicitement une requête trop grande d'un quota épuisé (texte différent selon la cause réelle, détail du fournisseur inclus) — ne devrait plus jamais être un faux positif dû à la taille du schéma | | |
| 4 | Répéter avec **tous les 6 types actifs simultanément** (cas le plus lourd possible) | Génération toujours fonctionnelle sans erreur de taille immédiate | | |

**Verdict** : [ ] PASS  [ ] FAIL

> ℹ️ **Pas de scénario manuel dédié à la mécanique interne du filtrage de schéma elle-même** — la
> construction du schéma (`activeGenerableTypes`/`buildQuestionSchema`) est un détail d'implémentation
> non observable depuis l'UI (le JSON envoyé au fournisseur n'est jamais exposé côté client) ; les 12
> tests Go de dev-backend (schéma filtré par distribution, classification d'erreur 413 vs 429) la
> couvrent exhaustivement en automatisé. Ce Scénario 5 valide uniquement le **symptôme observable**
> (plus de faux rate-limit), qui est la seule partie testable manuellement. Un test direct
> supplémentaire (`TestClassifyAnthropicError_413And429_SurfaceEnvelopeMessage`) a été ajouté pour
> combler une asymétrie de couverture signalée en revue (Groq avait un test direct de classification
> 413/429, Anthropic non) — même remarque : détail interne, pas de scénario manuel dédié nécessaire.

---

## Critères de Validation Globale

- [ ] MEMOTION+ apparaît dans la modale de génération (jamais ailleurs), désactivé à 0% par défaut
- [ ] Une génération MEMOTION+ produit un mélange SPEEDY/QCM au sein des cartes d'une même question
- [ ] Aucune fuite de la chaîne `MEMOTION_PLUS` : ni dans l'éditeur, ni dans les badges, ni dans un `question.json`, ni dans le panneau de succès de la modale
- [ ] Une carte QCM générée est jouable de bout en bout (`/anim`, `/tv`, indices sur timer, révélation)
- [ ] MEMOTION seul (MEMOTION+ à 0%) génère des cartes strictement identiques à avant #196 (aucun champ `TYPE` sur la carte)
- [ ] Aucune fausse erreur "rate limit"/quota dès le 1er appel avec plusieurs types actifs dont MEMOTION+ (bug QUALIF v7.1.0.7, cycle 2)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris `internal/server/ai_generator_memotion_plus_196_test.go` (11 tests), `ai_generator_schema_filtering_1710_7_test.go` (12 tests, cycle 2) et le fix de non-régression Groq (`ai_groq_schema_discriminator_test.go`) | | |
| 2 | `go test ./internal/server/... -run 'MemotionPlus' -v` | Les 11 tests #196 PASS, notamment le test critique de normalisation `TYPE→MEMOTION` (scan de la chaîne `MEMOTION_PLUS` sur le JSON marshalé entier) et le round-trip à travers les vrais types Go (`ValidateCardTypeContent`) | | |
| 3 | `go test ./internal/server/... -run 'ActiveGenerableTypes\|BuildQuestionSchema\|RateLimitError\|ClassifyGroqError\|ClassifyAnthropicError' -v` | Les 12 tests du cycle 2 PASS (dont le cas de régression directe "MEMOTION_PLUS seul") **et** `TestClassifyAnthropicError_413And429_SurfaceEnvelopeMessage` (comble l'asymétrie de couverture Anthropic/Groq signalée en revue, `code-review-20260827-210604.md` point INFO — même classification 413≠429 vérifiée côté Anthropic que côté Groq) | | |
| 4 | `cd server-go/web && npx vitest run` | Tous les tests PASS, y compris `questionTypeMeta.test.js` (6 tests neufs : `QUESTION_TYPES` toujours 5 entrées, `GENERABLE_TYPES` 6 entrées, `MEMOTION_PLUS` absent de `QUESTION_TYPE_META`) et `AIGenerateModal.test.jsx` (4 tests neufs + 8 mis à jour, rebalance des sliders sur 6 colonnes) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
