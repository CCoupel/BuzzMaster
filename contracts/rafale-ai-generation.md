# Contrat — Génération IA du réservoir RAFALE (#203, v8.1.0)

> **Statut** : contrat écrit par `planner` **avant** développement (contract-first).
> **Issue** : #203 · milestone v8.1.0 · branche `milestone/v8.1.0`
> **Maquette** : `docs/mockups/rafale-ai-generation-203.html`
> **Révision** : GATE 2 du 2026-09-01 — retours utilisateur intégrés (§2 paliers, §3 catégories, §6bis coquille UI partagée, suppression de l'état « ajouté »)
> **Plan** : `_work/reports/plan-20260901-*.md`
>
> `dev-backend` implémente et peut ajuster (en documentant la raison ici).
> `dev-frontend` consomme, ne modifie pas.
>
> **Contrats liés** — ce document ne les remplace pas, il s'y adosse :
> `contracts/rafale.md` (§3.1 modèle, §7 pioche, §9 endpoints réservoir) ·
> `contracts/ai-generation.md` (§4 prompt, §5 schéma, §5.1 validation) ·
> `contracts/ai-multi-provider.md` (§2 lots, §9 job asynchrone, §10 progression, §11 annulation).

---

## 1. Décision d'architecture — chemin dédié, infrastructure partagée

RAFALE **n'entre pas** dans `generableQuestionTypes`. La génération du réservoir est un
**second chemin de génération**, avec son endpoint, son schéma et sa persistance — mais qui
**réutilise** l'intégralité de l'infrastructure de job existante.

### 1.1 Ce qui est séparé, et pourquoi

| Élément | Chemin Quiz (5 types + MEMOTION+) | Chemin RAFALE (#203) |
|---|---|---|
| Endpoint | `POST /api/generate-questions` | **`POST /api/rafale/generate-questions`** |
| Schéma LLM | `anyOf` à 6 branches discriminées par `TYPE` | **objet plat, aucun `anyOf`** (§4) |
| Persistance | `files/questions/<ID>/question.json`, un répertoire par question | **`files/rafale/reservoir.json`**, fichier unique |
| Identifiants | `1..999`, `os.Mkdir` exclusif | **`r-NNN`**, `nextRafaleIDUnsafe` |
| Difficulté | 4 libellés (`Facile`…`Expert`) | **entier 1..3** (étoiles) |
| Volume | `count` **ou** `duration` (minutes de partie) | **`count` seul, par paliers** (§2) |
| Champs dérivés | `TIME`, `POINTS`, `ORDER`, `POINTS_TARGET`, `QCM_HINTS_ENABLED`… | **aucun** |
| Longueur du texte | non contrainte | **plafonnée** (§5) |

> **Justification — quatre incompatibilités, pas une préférence de style.**
>
> 1. **`RAFALE` est déjà un `QuestionType` réel** (`questionTypeRegistry`, `OwnedFields` =
>    `RAFALE_DIFFICULTY`/`RAFALE_MODE`/`RAFALE_QUESTION_TIME`/`RAFALE_MAX_QUESTIONS`). Une
>    « question RAFALE » dans `question.json` est une **configuration de manche**, qui ne porte
>    aucun énoncé. Mettre `RAFALE` dans `generableQuestionTypes` demanderait au modèle de produire
>    l'objet qui *n'est pas* le contenu voulu. Le pseudo-type `MEMOTION_PLUS` n'est pas un
>    précédent transposable : il se **normalise** vers un type réel écrit au même endroit
>    (`MEMOTION` → `question.json`) ; ici il n'existe aucune normalisation possible, la
>    destination elle-même diffère.
> 2. **Une génération écrirait dans deux magasins.** Le curseur « % » répartit un volume unique
>    entre des types qui atterrissent tous dans `files/questions/`. Une part RAFALE la ferait
>    écrire simultanément dans `reservoir.json` — deux modèles, deux schémas d'ID, deux formats
>    de fichier, pour une seule barre de progression et un seul `CREATED_COUNT`.
> 3. **Le formulaire diverge sur ses deux axes.** Difficultés (4 libellés vs 3 étoiles) et volume
>    (durée de partie vs nombre d'entrées) n'ont pas le même domaine. Un rendu aujourd'hui
>    entièrement générique (`TYPES.map`, aucun `if`) gagnerait des branches `type === 'RAFALE'`
>    sur chaque ligne.
> 4. **Budget de tokens Groq.** `groqMaxTokensBudget = 8000` (`ai_groq.go:53`). Chaque branche
>    `anyOf` supplémentaire alourdit le schéma envoyé à **chaque** lot — c'est exactement ce qui
>    a produit le faux « rate limit » 413 de #196 (QUALIF v7.1.0.7), corrigé en ne transmettant
>    que les branches actives. Un schéma RAFALE **plat et séparé** (§4) ne touche pas ce budget
>    et **ne peut pas** réintroduire l'ambiguïté de discriminateur `anyOf` de #142.

### 1.2 Ce qui est partagé — obligatoire, pas optionnel

La séparation est **UI + schéma + persistance**. Tout le reste est réutilisé **tel quel** :

| Élément partagé | Conséquence |
|---|---|
| **`aiJobRegistry` (`tryStart`)** | **Un seul job à la fois, tous chemins confondus.** Une génération RAFALE demandée pendant un job Quiz (et réciproquement) reçoit `409 generation_in_progress`. Deux jobs concurrents videraient le quota provider et fausseraient la progression, qui est un singleton global (`CurrentAIJobProgress`). |
| Découpage en lots, `batch_size`, relances, `max_consecutive_failures`, temporisation inter-lots, gestion 429/413 | inchangés |
| `AI_GENERATION_PROGRESS` (WS `/ws/admin`) | **même action**, étendue d'un champ `TARGET` (§6) |
| `CANCEL_AI_GENERATION` | inchangée — annule le job courant quel qu'il soit |
| `selectProvider`, clés API, `AdaptSchema`, `sanitizeUpstreamMessage` | inchangés |
| Codes d'erreur stables (`no_api_key`, `upstream_error`, `timeout`, `provider_quota`…) | inchangés |

> ⚠️ **`id_exhausted` ne s'applique pas** au chemin RAFALE : les identifiants `r-NNN` ne sont pas
> bornés à 999 par un balayage de répertoire. Ce code n'est jamais émis par ce chemin.

---

## 2. `POST /api/rafale/generate-questions`

| Propriété | Valeur |
|---|---|
| Auth | Aucune (aligné sur `/api/generate-questions` et sur les endpoints réservoir §9 de rafale.md) |
| Content-Type | `application/json` |
| Méthode | `POST` uniquement — `405` sinon, renvoyé par le handler (le mux n'a pas de motif de méthode) |
| Corps max | 64 KB (`http.MaxBytesReader`, idiome de `handleGenerateQuestions`) |
| Réponse | **asynchrone** — `202` puis progression WebSocket |

### Request

```json
{
  "theme": "Culture générale — France",
  "populations": ["Ado (13-17 ans)", "Adulte (18-64 ans)"],
  "language": "Français",
  "instructions": "Éviter l'actualité récente",
  "categories": ["HISTORY", "GEOGRAPHY", "SCIENCE"],
  "difficulties": [1, 2],
  "count": 50
}
```

| Champ | Type | Obligatoire | Règle de validation → `400 invalid_request` |
|---|---|---|---|
| `theme` | string | ✅ | non vide après trim, ≤ 200 car. |
| `populations` | string[] | ✅ | ≥ 1 élément, chacun ∈ `quizPopulations`, sans doublon |
| `language` | string | ✅ | ∈ `quizLanguages` |
| `instructions` | string | ❌ | ≤ 2000 car. **Jamais persisté** |
| `categories` | string[] | ✅ | ≥ 1 élément, sans doublon ; chaque clé doit passer `isKnownRafaleCategory` (§3) |
| `difficulties` | **int[]** | ✅ | ≥ 1 élément, chacun ∈ `{1,2,3}`, sans doublon |
| `count` | int | ✅ | ∈ **`{10, 20, 50, 100, 200}`** (§2bis) **et** ≤ `ai.max_questions`. Toute autre valeur → `400` |

> **Différences de forme assumées avec `/api/generate-questions`** — chacune est une conséquence
> directe du §1.1, pas une divergence gratuite :
> - **`difficulties` est un tableau d'entiers**, pas de libellés — c'est l'échelle native de
>   `RafaleQuestion.Difficulty`. Aucune conversion 4→3 n'est introduite : elle serait lossy
>   (`Facile`/`Moyen`/`Difficile`/`Expert` → 3 niveaux) et invisible pour l'admin.
> - **`count` remplace l'objet `volume`** — le mode `duration` n'a pas de sens : un réservoir
>   n'a pas de durée, il alimente un nombre indéterminé de manches.
> - **`objectives` est absent** — c'est une propriété de la **partie** (`QUIZ_OBJECTIVES`,
>   `GameState`), alors que le réservoir est global et survit à toutes les parties. L'y injecter
>   lierait un contenu permanent au contexte d'une partie éphémère.
> - **`distribution` est absent** — un seul type est produit.
> - **`theme`, `populations`, `language` sont saisis dans la modale**, et non lus en lecture
>   seule depuis `GameState` comme le fait la modale Quiz (`ai-generation.md` §3bis) — même
>   raison que `objectives`.

### 2bis. `count` — paliers fermés, pas un entier libre

`count` n'accepte que les valeurs de **`rafaleGenerationPresets = {10, 20, 50, 100, 200}`**,
elles-mêmes filtrées par `ai.max_questions` (un palier au-delà du plafond configuré est refusé
en `400` et masqué côté UI).

> **Justification** (arbitrage utilisateur, GATE 2) : on ne remplit pas un réservoir « à la
> question près ». La grandeur que l'admin manipule est une **taille de lot** — « il me faut 50
> questions de plus » — et non une quantité fine à calibrer. Une énumération fermée supprime la
> classe entière des saisies aberrantes (`0`, `7`, `10000`), rend le contrôle utilisable au doigt
> sur tablette, et réutilise le motif de boutons segmentés déjà en place pour le sélecteur de mode
> de volume de la modale Quiz. Le mode `duration` reste exclu : un réservoir n'a pas de durée.
>
> La liste est **une constante partagée Go ↔ JS** — un palier proposé par l'interface et refusé
> par le serveur serait un `400` que l'admin ne pourrait pas comprendre.

### Répartition du volume

Les `count` questions sont réparties **uniformément** sur les
`len(categories) × len(difficulties)` couples. Le reste de la division entière est attribué aux
premiers couples dans l'ordre de la requête.

```
parCouple  = count / (len(categories) * len(difficulties))
reste      = count % (len(categories) * len(difficulties))
```

> **Pourquoi le couple, et pas la catégorie seule.** Le filtre de pioche d'une manche est
> `CATEGORY ∩ DIFFICULTY ∩ ¬used` (`rafale.md` §7). Un réservoir riche en Histoire ★☆☆ mais vide
> en Histoire ★★★ rend la manche « Histoire difficile » injouable, alors que le total par
> catégorie paraît confortable. La cellule du filtre est donc la bonne unité de remplissage.

Le couple visé de chaque lot est transmis au modèle dans le prompt (§4).

### Response 202 Accepted

```json
{ "status": "accepted", "job_id": "gen-20260901-162300-a1b2", "batches_total": 3 }
```

Forme **identique** à `/api/generate-questions` (`ai-multi-provider.md` §9) — le frontend réutilise
le même code de suivi.

### Errors

| Code | `code` | Cas |
|---|---|---|
| `400` | `invalid_request` | payload invalide (voir tableau ci-dessus) |
| `405` | — | méthode ≠ POST (texte brut) |
| `409` | `no_api_key` | aucune clé configurée pour le provider sélectionné |
| `409` | `generation_in_progress` | un job (Quiz **ou** RAFALE) est déjà en cours |
| **`409`** | **`rafale_round_in_progress`** | une manche RAFALE est en cours (§7) |

Les erreurs `502`/`504` ne sont jamais renvoyées par cet endpoint : elles surviennent pendant le
job et transitent par la progression (§6).

---

## 3. Catégories — `isKnownRafaleCategory` fait foi

La validation des catégories utilise **`isKnownRafaleCategory`** (`http.go:2186`), c'est-à-dire
exactement le contrôle déjà appliqué par `POST /api/rafale/questions` (rafale.md §9), et **non**
`ResolveCategoryMeta` utilisé par `validateGenerateRequest` du chemin Quiz.

> **Justification** : une question générée doit être **indiscernable** d'une question saisie à la
> main dans l'éditeur du réservoir. Toute clé acceptée par la génération et refusée par l'éditeur
> (ou l'inverse) produirait des entrées de réservoir non ré-éditables. Le point d'entrée de
> validation doit être le même que celui du magasin de destination.

### 3.1 Choisies par l'admin, jamais déduites du réservoir

**Décision (arbitrage utilisateur, GATE 2)** : l'interface propose **toutes** les catégories
connues — pas seulement celles déjà représentées dans `reservoir.json` — et l'admin sélectionne
librement. Le serveur n'impose **aucune** restriction supplémentaire à l'existant.

> **Pourquoi ne pas restreindre aux catégories déjà présentes.** Ce serait un **blocage
> d'amorçage** : une catégorie neuve resterait inatteignable par la génération tant qu'une
> première question n'y aurait pas été saisie **à la main** — exactement le travail que cette
> feature existe pour éviter. Le cas est loin d'être théorique : ouvrir une catégorie est
> précisément le moment où l'on veut générer en volume.

**En revanche, l'existant est affiché** : chaque catégorie proposée est annotée du **nombre de
questions déjà présentes dans le réservoir** pour les difficultés sélectionnées, et la
répartition (§2) est présentée sous la forme `existant → après génération`.

> **Ce que cela résout.** La question sous-jacente de l'admin n'est pas « quelles catégories
> existent ? » mais « **où sont mes trous ?** ». Le filtre de pioche étant
> `CATEGORY ∩ DIFFICULTY ∩ ¬used` (`rafale.md` §7), un total général confortable peut masquer une
> cellule vide qui rend une manche injouable. L'annotation transforme le sélecteur en carte des
> manques — l'information est **donnée**, la décision reste **libre**.

**Source des comptes** : calculés **côté client** à partir de la liste que `RafalePage` a déjà
chargée (`GET /api/rafale/questions`). **Aucun endpoint nouveau, aucun appel supplémentaire** —
`GET /api/rafale/pool` reste réservé à l'alerte pré-manche, qui interroge un couple unique.

---

## 4. Schéma de sortie du LLM — plat, sans `anyOf`

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": ["questions"],
  "properties": {
    "questions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["QUESTION", "ANSWER", "CATEGORY", "DIFFICULTY"],
        "properties": {
          "QUESTION":   { "type": "string" },
          "ANSWER":     { "type": "string" },
          "CATEGORY":   { "type": "string", "enum": [<categories de la requête>] },
          "DIFFICULTY": { "type": "integer", "enum": [<difficulties de la requête>] }
        }
      }
    }
  }
}
```

**Règle d'or reconduite de `ai-generation.md` §5** : le schéma ne contient **aucun** champ
d'identifiant. `ID` est absent et serait rejeté par `additionalProperties: false` — le modèle est
structurellement incapable de désigner, et donc d'écraser, une question existante du réservoir.

**Propriétés de ce schéma** :
- **Aucun `anyOf`** → l'ambiguïté de discriminateur strict de Groq (#142,
  `ai-multi-provider.md` §7) est **structurellement impossible** ici.
- `AdaptSchema` du provider reste appliqué (no-op pour Anthropic ; pour Groq, le stripping
  d'`enum` ne concerne que les branches d'un `anyOf` et n'a donc rien à retirer — la validation
  serveur du §5 couvre le cas de toute façon).
- Schéma court → charge de tokens par lot très inférieure à celle du chemin Quiz.

> ⚠️ **`minLength` / `maxLength` ne sont pas supportés** par les sorties structurées
> (`ai-generation.md` §5). La contrainte de longueur (§5) est donc **inapplicable au niveau du
> schéma** et repose entièrement sur le prompt + la validation serveur.

---

## 5. 🔴 Contrainte de brièveté — exigence centrale de #203

Une manche RAFALE laisse **~3 secondes** par question (`RAFALE_QUESTION_TIME`, défaut 3). Un
énoncé ou une réponse longs sont injouables, et — plus grave — **silencieusement tronqués** par
les surfaces d'affichage.

### 5.1 Plafonds

| Champ | Plafond serveur (dur) | Cible du prompt |
|---|---|---|
| `QUESTION` | **100 caractères** | ≤ 80, une seule phrase interrogative |
| `ANSWER` | **40 caractères** | ≤ 25, idéalement 1 à 3 mots |

La longueur est mesurée en **runes** (`utf8.RuneCountInString`), après `strings.TrimSpace` —
jamais en octets : « é » ne doit pas coûter double.

> **D'où viennent ces deux nombres** — mesure sur les surfaces d'affichage réelles, pas une
> estimation :
>
> | Surface | Style | Capacité avant troncature |
> |---|---|---|
> | `/anim` énoncé courant (`.rafale-anim-qcard-text`) | `clamp(1.3rem, 4.2vh, 2.2rem)`, `-webkit-line-clamp: 3` | **≈ 85–100 car.** — surface la plus contraignante |
> | `/tv` énoncé (`.rafale-tv-qcard-text`) | `clamp(1.3rem, 4.5vh, 2.6rem)`, `-webkit-line-clamp: 4` | ≈ 120 car. |
> | `/anim` réponse (`.rafale-anim-qcard-answer`) | `1.15rem`, flex **une seule ligne**, aucun clamp | déborde au-delà de ≈ 40–45 car. |
>
> Le plafond est calé sur la surface la plus contraignante : une question acceptée est
> **toujours** lisible en entier sur `/anim` comme sur `/tv`. La cible du prompt est
> volontairement **plus basse** que le plafond — le modèle vise court, et le plafond ne rejette
> que ce qui est réellement injouable, sans perdre une bonne question pour deux caractères.

### 5.2 🔴 Écarter, jamais tronquer

> **Une question hors plafond est ÉCARTÉE et comptée dans `SKIPPED_COUNT`. Elle n'est jamais
> tronquée.**

**Justification** — la troncature produit des données fausses, pas des données courtes :
- tronquer un **énoncé** produit une question sans fin (« Quel est le nom du premier… ») ;
- tronquer une **réponse** produit une réponse **fausse**, que l'animateur validerait de bonne
  foi contre la bonne réponse d'un joueur — un faux négatif de scoring, indétectable en relecture.

Le rejet réutilise le mécanisme éprouvé de `validateGeneratedQuestions` (`ai-generation.md`
§5.1) : la génération n'est pas interrompue, la question fautive est simplement absente, et
l'admin voit le compte dans `SKIPPED_COUNT`.

> Ce point **amende explicitement** la formulation de l'issue #203 (« validée/**tronquée** côté
> serveur »), qui était une piste, pas une exigence — l'issue demandait avant tout la défense en
> profondeur, qui est bien assurée ici (prompt + serveur).

### 5.3 Les mêmes plafonds s'appliquent à l'éditeur manuel

`POST /api/rafale/questions` (rafale.md §9) gagne les **mêmes** contrôles, en `400` :

| Erreur | Condition |
|---|---|
| `400` | `QUESTION` > 100 runes après trim |
| `400` | `ANSWER` > 40 runes après trim |

> **Justification** : sans cela, la contrainte serait contournable en trois clics dans l'éditeur,
> et le réservoir contiendrait deux qualités de questions selon leur origine. Le plafond est une
> propriété **du réservoir**, pas de la génération. C'est un **[BREAKING] mineur** : une question
> plus longue devient non enregistrable — voir §9 pour la migration.

### 5.4 Autres contrôles de validation

En complément des plafonds, chaque question renvoyée par le modèle est écartée si :

| Contrôle | Règle |
|---|---|
| `QUESTION` | vide après trim |
| `ANSWER` | vide après trim |
| `CATEGORY` | ∉ `categories` de la requête |
| `DIFFICULTY` | ∉ `difficulties` de la requête (garantie serveur, seule restante pour Groq) |
| **Doublon interne** | énoncé identique (comparaison insensible à la casse et aux espaces) à une question **déjà dans le réservoir** ou **déjà créée par ce job** → écartée |
| Volume | au-delà de `count`, l'excédent est écarté |

> Le contrôle de doublon est **nouveau par rapport au chemin Quiz** (qui se repose uniquement sur
> le contexte anti-doublon injecté au prompt). Il est justifié ici parce que le réservoir est
> **cumulatif** : il est alimenté par générations successives sur les mêmes catégories, là où une
> liste Quiz est repartie de zéro à chaque partie. Sans ce contrôle, la 3ᵉ génération « Histoire
> ★☆☆ » reproduirait mécaniquement les énoncés des deux premières.

### 5.5 Raisons d'écartement

`SKIPPED_COUNT` est un compteur. Les raisons sont **journalisées** (`LogWarn`), agrégées par
famille, et exposées à l'admin de façon agrégée par la modale :

`énoncé trop long` · `réponse trop longue` · `champ vide` · `catégorie hors filtre` ·
`difficulté hors filtre` · `doublon` · `plafond de volume atteint`

---

## 6. Progression — `AI_GENERATION_PROGRESS` étendu

L'action **existante** est réutilisée, **étendue d'un seul champ**. Aucune nouvelle action
WebSocket n'est créée.

| Champ | Type | Description |
|---|---|---|
| **`TARGET`** | string | **`"QUIZ"`** (défaut, chemin existant) ou **`"RAFALE"`** (#203) |

```json
{
  "ACTION": "AI_GENERATION_PROGRESS",
  "MSG": {
    "JOB_ID": "gen-20260901-162300-a1b2",
    "TARGET": "RAFALE",
    "STATE": "RUNNING",
    "BATCHES_DONE": 2, "BATCHES_TOTAL": 3,
    "CREATED_COUNT": 24, "SKIPPED_COUNT": 2,
    "ERROR_CODE": "", "ERROR_MESSAGE": "", "PROVIDER": "groq"
  }
}
```

**Règles** :
- **Additif et rétrocompatible** : `TARGET` absent ⇒ `"QUIZ"`. Tous les champs existants et leurs
  sémantiques sont inchangés, destinataires inchangés (`/ws/admin` uniquement — jamais `/ws/tv`,
  `/ws/player`, `/ws/buzzer`).
- `TARGET` est **obligatoire pour la correction du frontend** : la progression est un singleton
  global, et **deux modales distinctes** l'écoutent désormais (Questions et Rafale). Sans ce
  champ, un job RAFALE ferait basculer la modale de la page Questions en « génération en cours »
  et lui ferait afficher un delta de questions Quiz vide.
- Émis **après** la persistance du lot, pour que le réservoir soit déjà à jour côté client —
  même règle que le chemin Quiz.
- Rejoué à la connexion d'un client admin si un job est en cours (`pushAIJobProgressToNewAdmin`),
  `TARGET` inclus.

### Rafraîchissement de la liste du réservoir

**Aucun broadcast WebSocket du réservoir n'est créé.** `RafalePage` refetch déjà
`GET /api/rafale/questions` après chaque mutation (`loadQuestions`) ; la modale déclenche le même
refetch à chaque progression où `CREATED_COUNT` a augmenté et `TARGET === "RAFALE"`.

> **Justification** : le réservoir n'a aujourd'hui aucun équivalent de `OnQuestionUpload`. En
> créer un (nouvelle action WS, nouveau sérialiseur, nouvelle liste de destinataires à filtrer)
> pour un usage strictement admin serait une surface nouvelle — dont la classe de risque est
> précisément celle qu'a connue RAFALE avec les fuites `/tv` (rafale.md §2.3). Le refetch réutilise
> un chemin déjà exercé par les 5 mutations existantes de la page.

---

## 6bis. Composant d'interface — une coquille partagée, deux formulaires

**Décision (arbitrage utilisateur, GATE 2)** : « modale dédiée » (§1.1) porte sur le
**formulaire**, pas sur le composant. Tout ce qui n'est pas le formulaire est **extrait de
l'existant et partagé**.

| Élément | Sort |
|---|---|
| Enveloppe, fermeture au clavier, `role="dialog"` | **extrait** — partagé |
| Machine à états `form / loading / success / cancelled / failed / submit-error / unavailable` | **extrait** — partagé |
| Cycle de vie du job (suivi de `JOB_ID`, décompte inter-lots, annulation, transitions terminales) | **extrait** — partagé |
| Les 6 corps d'état (`RunningBody`, `DoneBody`, `CancelledBody`, `FailedBody`, `SubmitErrorBody`, `UnavailableBody`) | **extraits** — partagés |
| Pied de modale par état | **extrait** — partagé |
| Filtrage du job sur `TARGET` (§6) | **extrait** — partagé, résolu une seule fois pour les deux |
| `jobErrorMessage`, `providerLabel`, `mapSubmitError`, `clampInt` | **extraits** — partagés |
| **Le formulaire et la construction du payload** | **propres à chaque modale** |

Chaque modale se réduit donc à : son formulaire, son endpoint, son `buildPayload()`, sa règle
`canSubmit` et son `TARGET`.

> **Pourquoi c'est sûr.** Les six corps d'état existants sont **déjà entièrement génériques** —
> aucun ne référence quoi que ce soit de propre au Quiz. Le seul paramètre spécifique,
> `breakdown` de `DoneBody`, est **déjà** rendu sous condition (`breakdown.length > 0`) : la
> modale RAFALE passe un tableau vide et l'écran s'affiche sans lui. L'extraction est un
> **déplacement**, pas une réécriture.

> 🔴 **Critère d'acceptation, non négociable** : les **props publiques de `AIGenerateModal` sont
> inchangées** et ses quatre fichiers de test existants
> (`AIGenerateModal.test.jsx`, `.progress.test.jsx`, `.tooltip.test.jsx`, `.unsavedBanner.test.jsx`)
> passent **sans être modifiés**. Même discipline que l'extraction du seam de persistance côté
> backend (§8) : on déplace du code éprouvé, on ne le réécrit pas.

### 6ter. Aucun état « fraîchement ajouté »

**Décision (arbitrage utilisateur, GATE 2)** : la modale ne mémorise **aucun instantané** du
réservoir avant lancement, les questions générées ne portent **aucun marquage** dans la liste, et
l'écran « terminé » ne détaille **pas** la ventilation par catégorie.

> **Justification** : distinguer une question fraîchement générée n'aurait servi qu'à raisonner
> sur un ajout survenant pendant une manche — cas **déjà exclu** par le `409
> rafale_round_in_progress` (§7). Sans ce cas, il n'existe aucun moment où la distinction change
> quoi que ce soit, ni pour l'animateur, ni pour la pioche, ni pour l'éditeur.
>
> Conséquence directe : la dérivation du delta (`startingQuestionIdsRef` du chemin Quiz) n'est
> **pas** reproduite côté RAFALE. L'écran « terminé » affiche `CREATED_COUNT`, `SKIPPED_COUNT` et
> le nouveau total du réservoir — trois compteurs déjà disponibles. Une mécanique de moins, sans
> perte fonctionnelle.

---

## 7. 🔴 Refus pendant une manche RAFALE en cours

> **Une génération est refusée si une manche RAFALE est en cours** — `409` avec
> `code: "rafale_round_in_progress"`.

**Condition exacte** : la question courante est de type `RAFALE` **et** la phase est `STARTED`
ou `PAUSED`.

**Justification** — la pioche lit le réservoir à chaque tirage et `RAFALE_POOL_REMAINING` est
recalculé et diffusé à chaque question (`rafale.md` §7). Injecter des questions au milieu d'une
manche ferait **augmenter** un compteur que l'animateur voit décroître, et invaliderait
l'estimation de besoin calculée avant le démarrage (§7.2). Le coût de la restriction est nul —
on ne génère pas un réservoir pendant qu'on joue — et elle ferme une classe d'incohérence
d'état par construction plutôt que par correctif ultérieur.

---

## 8. Persistance — écriture par lot, une seule sauvegarde

Les questions validées d'un lot sont écrites **en une seule opération** :

```go
func (e *Engine) AppendRafaleQuestions(qs []RafaleQuestion) ([]RafaleQuestion, error)
```

| Règle | Détail |
|---|---|
| **Ajout seul** | N'écrase **jamais** une entrée existante. Tout `ID` fourni en entrée est ignoré : l'ID est **toujours** alloué par le serveur. |
| **Allocation** | `r-NNN` séquentiels, alloués **sous un unique verrou** pour tout le lot (`nextRafaleIDUnsafe` appliqué de façon incrémentale) |
| **Une seule sauvegarde** | `SaveRafale()` est appelé **une fois** après l'insertion de tout le lot, **hors du verrou** |
| **Drapeau « déjà utilisée »** | `rafale_used.json` n'est **ni lu ni écrit**. Les nouvelles questions sont disponibles par construction : un ID absent du fichier est disponible (rafale.md §3.2) |

> **Pourquoi une méthode dédiée plutôt que `UpsertRafaleQuestion` en boucle.** `UpsertRafaleQuestion`
> appelle `SaveRafale()` à **chaque** question, et `SaveRafale()` réécrit **l'intégralité** du
> fichier. Un lot de 20 sur un réservoir de 200 produirait 20 réécritures complètes — coût en
> O(N×M), et surtout **20 fenêtres** pendant lesquelles un arrêt laisse le réservoir dans un état
> partiel non voulu. La discipline de verrouillage est imposée : `SaveRafale()` prend `RLock` et
> **ne peut donc pas** être appelée en tenant le verrou d'écriture (le `RWMutex` n'est pas
> réentrant) — d'où `Lock → insérer tout le lot → Unlock → SaveRafale()`.

`UpsertRafaleQuestion` reste **inchangée** — l'éditeur manuel continue de l'utiliser.

---

## 9. Migration et compatibilité

| Point | Effet |
|---|---|
| `reservoir.json` existant | **Aucune migration.** Format et modèle `RafaleQuestion` strictement inchangés. Une question générée est indiscernable d'une question saisie. |
| `rafale_used.json` | Jamais touché par la génération |
| Questions existantes hors plafond (§5.3) | **Jamais supprimées ni tronquées.** Le plafond s'applique à l'**écriture** : une question trop longue déjà en base reste jouable et lisible en liste. Elle ne peut simplement plus être **ré-enregistrée** telle quelle depuis l'éditeur — l'admin doit la raccourcir, ce que le formulaire signale avec un compteur de caractères. |
| Chemin Quiz | **Strictement inchangé.** `generableQuestionTypes` conserve ses 6 entrées, `buildQuestionSchema` ses 6 branches, `GENERABLE_TYPES` ses 6 entrées et son filtre `t.key !== 'RAFALE'`. |
| Clients non rechargés | Un onglet servant le JS d'avant #203 ignore `TARGET` (champ inconnu) et continue de fonctionner sur le chemin Quiz. Il afficherait la progression d'un job RAFALE comme si c'était la sienne — d'où le rechargement des interfaces admin après déploiement, mode opératoire habituel (`ai-generation.md` §3ter). |

---

## 10. Sécurité

Points reconduits de `ai-generation.md` §8, sans exception :

| Règle | Application ici |
|---|---|
| La clé API n'est jamais journalisée, ni renvoyée, ni exposée dans un message d'erreur | inchangé (`sanitizeUpstreamMessage`) |
| `ERROR_MESSAGE` assaini avant diffusion (redaction `sk-ant-…`/`gsk_…`, troncature 500) | inchangé |
| Progression jamais diffusée hors `/ws/admin` | inchangé — **`TARGET` n'y change rien** |
| `instructions` n'est jamais persisté | inchangé |
| Le LLM ne peut pas désigner une entrée existante | garanti par l'absence d'`ID` au schéma (§4) |
| Corps de requête borné | `MaxBytesReader` 64 KB |

> ⚠️ **La réponse attendue reste hors de `GameState`.** Ce contrat n'introduit aucun nouveau
> canal : les questions générées rejoignent le réservoir, et leur `ANSWER` ne devient visible que
> par les chemins déjà contractés (`RAFALE_ANSWER` vers `admin`+`anim`, rafale.md §2.3 et §13.3,
> et `GET /api/rafale/questions` côté admin). Aucun élargissement de destinataires.

---

## 11. Ce que ce contrat ne couvre pas

- La génération de **configurations de manche** RAFALE (`RAFALE_MODE`, `RAFALE_DIFFICULTY`…) —
  hors sujet, ce sont des réglages d'animateur, pas du contenu.
- La génération de **médias** pour le réservoir — les questions RAFALE sont texte seul
  (`rafale.md` §2.4, arbitrage D3), inchangé.
- Le **remplacement** du réservoir. La suppression reste explicite et séparée
  (`POST /reset-select?rafale=true`, rafale.md §10).
- La **relecture/édition en lot** des questions générées — l'éditeur existant, question par
  question, est inchangé.
- Toute modification du chemin de génération Quiz.
