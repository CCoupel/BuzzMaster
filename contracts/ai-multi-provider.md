# Contrat — Génération IA multi-provider et par lots (#137, v6.1.0)

> **Étend** `contracts/ai-generation.md` (#8). Tout ce qui n'est pas redéfini ici reste valable.
> **Plan** : `_work/reports/planner-20260805-204318-plan-137.md`
> **Recherche providers** : `_work/reports/planner-20260805-203550-providers-137.md`
>
> Deux changements structurels : (1) la génération passe d'**un appel unique** à une **suite de
> lots** avec reprise ; (2) elle passe d'une **réponse HTTP synchrone** à une **tâche de fond
> avec progression WebSocket**.

---

## 1. Pourquoi ce changement — contrainte dimensionnante

Le tier gratuit Groq impose **8 000 tokens/minute** sur `openai/gpt-oss-120b`
(30 RPM, 1 000 requêtes/jour, 200 000 tokens/jour — vérifié sur
https://console.groq.com/docs/rate-limits).

> ⚠️ Groq **ne documente ni ce que compte le TPM** (entrée seule, sortie seule, ou les deux)
> **ni le sort d'une requête unique qui le dépasse**. Les deux points sont à **calibrer
> empiriquement en début de développement** (tâche T1.0). Le dimensionnement ci-dessous suppose
> le cas défavorable : **TPM = entrée + sortie**.

Le modèle de #8 — 200 questions en un appel, ~20k tokens de contexte injecté — est donc
impossible. D'où : lots courts, contexte réduit, cadencement.

---

## 2. Découpage en lots — normatif

| Règle | Valeur |
|---|---|
| Taille de lot | `ai.batch_size`, défaut **20**, bornes 1..50 |
| Ordonnancement | **séquentiel strict** — jamais deux appels provider en parallèle |
| Écriture | chaque lot est **validé puis écrit sur disque immédiatement**, avant l'appel suivant |
| Broadcast | `OnQuestionUpload()` est appelé **après chaque lot**, pas seulement à la fin |
| Contexte anti-doublon | plafonné par un budget de tokens (§4), et **enrichi** à chaque lot des questions déjà produites dans le job courant (anti-doublon intra-job) |
| Reprise | un lot en échec **n'annule pas** les lots précédents — ils restent écrits |
| Arrêt | après `ai.max_consecutive_failures` échecs consécutifs (défaut **2**), le job passe en `FAILED` en conservant l'acquis |

**Garantie d'additivité inchangée** : chaque lot alloue ses IDs via le chemin verrouillé de
`ai-generation.md` §5.1. Le schéma reste sans champ d'identifiant.

> **Bénéfice indépendant de Groq** : sous décodage contraint, une troncature produit un JSON
> impartial irrécupérable. En un seul appel, on perd **tout**. Par lots, on perd **un lot**.
> Ce découpage doit donc s'appliquer **aussi au chemin Anthropic** de #8.

---

## 3. Cadencement et gestion des limites

| Règle | Détail |
|---|---|
| Délai inter-lots | `ai.inter_batch_delay_ms`, défaut **60000** (le plancher imposé par 8K TPM) |
| `429` / `413` | respecter l'en-tête `retry-after` s'il est présent ; sinon backoff exponentiel plafonné à 120 s |
| Compteur journalier | sur `429` persistant, remonter `code: "provider_quota"` — l'opérateur a épuisé son RPD/TPD |
| Jamais | aucune rotation de clés, aucun multi-compte — l'AUP Groq l'interdit expressément (« beyond published parameters, rate limits, or use limitations, including by registering multiple accounts ») |

---

## 4. Budget de contexte injecté — par provider

Le contexte anti-doublon de #8 (150 questions, 200 car.) est incompatible avec 8K TPM.

| Provider | Budget contexte | Questions injectées (indicatif) |
|---|---|---|
| `anthropic` | inchangé | 150 max |
| `groq` | `ai.context_token_budget`, défaut **1500** | ~25, énoncés tronqués à 120 car. |

Le budget est appliqué par **estimation de tokens côté serveur** (approximation
caractères/4 suffisante), pas par comptage exact.

---

## 5. Interface provider

Couture unique, sur le point d'appel existant `ai_generator.go:781`.

```go
// aiProvider abstrait l'appel au modèle. Une seule méthode : l'implémentation
// est responsable du transport, du format de requête et du mapping d'erreurs
// vers aiTimeoutError / aiUpstreamError (ai-generation.md §3).
type aiProvider interface {
    Generate(ctx context.Context, cfg config.AIConfig, prompt string, schema map[string]any) (string, error)
    // AdaptSchema laisse le provider ajuster le schéma à son dialecte (§7).
    AdaptSchema(schema map[string]any) map[string]any
    Name() string
}
```

Sélection : `config.Get().AI.Provider` ∈ `{"anthropic", "groq"}`, défaut `"anthropic"`.
**Aucune régression fonctionnelle attendue sur le chemin `anthropic`** — il devient une
implémentation de cette interface, à comportement identique hors découpage (§2).

> Cette interface est délibérément **minimale**. Elle ne préjuge d'aucune architecture à
> plugins et n'en interdit aucune (consigne #135).

---

## 6. Client Groq

| Point | Décision |
|---|---|
| Transport | **`net/http` stdlib**, motif `internal/server/github_client.go`. Pas de nouvelle dépendance. |
| Endpoint | API compatible OpenAI : `POST https://api.groq.com/openai/v1/chat/completions` |
| Auth | `Authorization: Bearer <ai.groq_api_key>` |
| Modèle | `ai.groq_model`, défaut **`openai/gpt-oss-120b`** |
| Sortie structurée | `response_format: {"type":"json_schema","json_schema":{"name":"buzz_questions","strict":true,"schema":{…}}}` |
| **Streaming** | **Aucun.** Groq ne supporte pas le streaming avec les sorties structurées. Appel bloquant, un lot à la fois. C'est précisément pourquoi les lots doivent rester courts. |
| Timeout | `ai.timeout_seconds` appliqué **par lot**, pas au job entier |
| Erreurs | même mapping que #8 : 401→`upstream_error`(401), 429/413→`provider_quota` ou backoff, réseau→`upstream_error` |

**Modèles écartés et pourquoi** : `llama-3.3-70b-versatile` n'a que le JSON object mode (pas de
schéma strict) ; `qwen/qwen3.6-27b` est un modèle *Preview*, explicitement déconseillé en
production par Groq.

---

## 7. Adaptation du schéma

Le schéma de `buildQuestionSchema` (`ai_generator.go:166-254`) satisfait **déjà** les trois
règles du mode strict de Groq — racine non-`anyOf`, toutes les propriétés dans `required`,
`additionalProperties: false` partout. `anyOf` est documenté et supporté.

> **Amendement (v6.1.1, issue #142, cause racine confirmée par appel réel à l'API Groq le
> 2026-08-07)** : la spéculation ci-dessous (`DIFFICULTY` entier de `MOTION_CARDS`) **n'était pas
> le problème réel**. Ce champ n'a jamais été signalé par Groq. Le vrai blocage, reproduit
> byte pour byte contre l'API réelle :
>
> ```
> invalid JSON schema for response_format: 'buzz_questions': /properties/questions/items/anyOf:
> anyOf disambiguation failed: anyOf: discriminator: multiple candidate properties
> CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]
> ```
>
> **Mécanisme confirmé** : le validateur strict de Groq traite **toute** propriété `required`,
> présente dans chaque branche de l'`anyOf`, portant une contrainte `enum`/`const`, comme une
> candidate discriminant — **sans vérifier si l'ensemble de valeurs varie réellement d'une
> branche à l'autre**. Le schéma a trois propriétés de ce type au niveau racine de chaque
> branche : `TYPE` (`const` distinct par branche — le discriminant voulu) et `CATEGORY`/
> `DIFFICULTY` (`enum` **identique** dans les 5 branches, hérité du bloc `common` partagé —
> elles ne peuvent rien discriminer puisqu'elles ne varient pas, mais le validateur ne le
> vérifie pas avant de les compter comme candidates elles aussi). Trois candidats → rejet
> total du schéma.
>
> **Correctif** (`groqProvider.AdaptSchema`, `internal/server/ai_provider.go`) : retirer `enum`
> de `CATEGORY`/`DIFFICULTY` dans chaque branche de l'`anyOf` — elles restent `required`,
> `"type":"string"`, seulement non restreintes à un ensemble de valeurs **pour Groq**. `TYPE`
> devient l'unique candidat discriminant. `validateGeneratedQuestions` (`ai_generator.go`)
> vérifie désormais `DIFFICULTY` côté serveur (comme `CATEGORY` l'était déjà, §5.1) — la garantie
> réelle (seules les valeurs demandées atteignent `question.json`) est donc inchangée pour la
> sortie Groq, seule la couche qui l'applique change. Le `DIFFICULTY` entier de `MOTION_CARDS`
> (imbriqué, hors de cette ambiguïté) reste inchangé.
>
> **Confirmé sans régression pour Anthropic** : `anthropicProvider.AdaptSchema` reste un no-op,
> chemin de code totalement séparé, jamais touché par ce correctif.
>
> Validé par appel réel pour les 5 types générables (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE) —
> chacun accepté (200) et produit une question valide respectant catégorie/difficulté demandées
> malgré l'absence de contrainte `enum` schéma (le prompt seul suffit à les respecter en
> pratique, cohérent avec le fait que les modèles restent guidés par les instructions même sans
> contrainte structurelle stricte sur ces deux champs précis).

> Aucune limite de profondeur ni de nombre de propriétés n'est documentée par Groq. Un `400`
> ne se révélera qu'à l'exécution : **envoyer le schéma réel dès la première tâche de dev**.

---

## 8. Configuration

```go
type AIConfig struct {
    // #8 — inchangé
    AnthropicAPIKey string `json:"anthropic_api_key"`
    Model           string `json:"model"`
    TimeoutSeconds  int    `json:"timeout_seconds"`
    MaxQuestions    int    `json:"max_questions"`
    APIKeyConfigured bool  `json:"api_key_configured,omitempty"`

    // #137
    Provider              string `json:"provider"`                 // "anthropic" | "groq"
    GroqAPIKey            string `json:"groq_api_key"`
    GroqModel             string `json:"groq_model"`               // défaut openai/gpt-oss-120b
    GroqAPIKeyConfigured  bool   `json:"groq_api_key_configured,omitempty"`
    ClearGroqAPIKey       bool   `json:"clear_groq_api_key,omitempty"`
    BatchSize             int    `json:"batch_size"`               // défaut 20
    InterBatchDelayMs     int    `json:"inter_batch_delay_ms"`     // défaut 60000
    ContextTokenBudget    int    `json:"context_token_budget"`     // défaut 1500
    MaxConsecutiveFailures int   `json:"max_consecutive_failures"` // défaut 2
}
```

**La clé Groq suit exactement les mêmes règles de secret que la clé Anthropic**
(`ai-generation.md` §2) : jamais renvoyée en `GET`, booléen dérivé `groq_api_key_configured`,
valeur vide en `POST` = préserver, effacement explicite via `clear_groq_api_key`.
Format attendu : préfixe `gsk_`.

---

## 9. `POST /api/generate-questions` — devient asynchrone

**BREAKING** par rapport à #8 : l'endpoint ne renvoie plus le résultat.

### Response 202 Accepted

```json
{ "status": "accepted", "job_id": "gen-20260805-204318-a1b2", "batches_total": 10 }
```

### Errors (inchangés, plus deux)

| Code | Cas |
|---|---|
| `409` | `no_api_key` — pas de clé pour le provider sélectionné |
| **`409`** | **`generation_in_progress`** — un job est déjà en cours (**un seul job à la fois, tout admin confondu**) |
| `400`, `405`, `507` | inchangés |

Les erreurs `502` / `504` de #8 ne sont plus renvoyées par cet endpoint : elles surviennent
pendant le job et transitent désormais par la progression (§10).

---

## 10. `AI_GENERATION_PROGRESS` — WebSocket serveur→client

| Propriété | Valeur |
|---|---|
| Direction | `Server→Client` |
| Endpoint | `/ws/admin` uniquement (jamais `/ws/tv`, `/ws/player`, `/ws/buzzer`) |
| Trigger | changement d'état du job : démarrage, fin de chaque lot, fin, erreur, annulation |

```json
{
  "ACTION": "AI_GENERATION_PROGRESS",
  "MSG": {
    "JOB_ID": "gen-20260805-204318-a1b2",
    "STATE": "FAILED",
    "BATCHES_DONE": 0,
    "BATCHES_TOTAL": 10,
    "CREATED_COUNT": 0,
    "SKIPPED_COUNT": 0,
    "ERROR_CODE": "upstream_error",
    "ERROR_MESSAGE": "invalid JSON schema for response_format: 'buzz_questions': /properties/questions/items/anyOf: anyOf disambiguation failed: anyOf: discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]",
    "PROVIDER": "groq"
  }
}
```

`STATE` ∈ `RUNNING` | `DONE` | `FAILED` | `CANCELLED`.
`ERROR_CODE` réutilise les codes stables de `ai-generation.md` §3, plus `provider_quota`.

### `ERROR_MESSAGE` (v6.1.1, additif — issue #142)

**Constat** : avant ce champ, un échec de génération n'était visible côté admin que via
`ERROR_CODE` (ex. `upstream_error`) — un code stable mais générique. Le détail réel de la cause
(ex. rejet du schéma JSON par Groq, message précis renvoyé par l'API) n'était accessible qu'en
ajoutant une trace de diagnostic temporaire dans le code, exactement le scénario qui a rendu #142
indiagnosticable en QUALIF. Voir `contracts/ai-generation.md` §8 S2 (amendé) pour la règle de
confidentialité qui encadre ce champ.

| Champ | Type | Description |
|---|---|---|
| `ERROR_MESSAGE` | string | Détail lisible de l'erreur, **assaini** (voir §8 S2 amendé) — vide sauf si `STATE = FAILED` |

**Règles** :
- Présent uniquement quand `STATE = "FAILED"` ; `""` sinon (démarrage, `RUNNING`, `DONE`,
  `CANCELLED` — ces états n'ont rien à afficher ici).
- Contenu : le message d'erreur réel renvoyé par le provider (`.error.message` de l'enveloppe
  Anthropic/Groq) quand disponible, sinon un message générique de repli (ex. « the AI generation
  service returned an error », inchangé par rapport à avant ce champ). Une erreur locale (écriture
  disque, `id_exhausted`) utilise le message de l'erreur Go elle-même — pas de secret à filtrer,
  mais passée par le même filtre par cohérence.
- **Assainissement obligatoire** avant diffusion (contract §8 S2) : toute sous-chaîne ressemblant
  à une clé API connue (`sk-ant-…`, `gsk_…`) est remplacée par `[redacted]`, et le message est
  tronqué à une longueur bornée (500 caractères + `…`). Défense en profondeur — un provider ne
  devrait de toute façon jamais réémettre la clé de l'appelant dans un corps de réponse, celle-ci
  n'étant transmise que via l'en-tête `Authorization`.
- `/ws/admin` uniquement, comme le reste de l'action — ne quitte jamais le serveur vers
  `/ws/tv`/`/ws/player`/`/ws/buzzer` (qui ne reçoivent de toute façon jamais `AI_GENERATION_PROGRESS`).

**Règles (existantes)** :
- Émis **après** le broadcast des questions du lot, pour que la liste soit déjà à jour côté client.
- Un message est émis **immédiatement à la connexion** d'un client admin si un job est en cours
  → un rechargement de page retrouve la progression (pas de reprise d'état à inventer côté client).
- `CREATED_COUNT` est cumulatif sur le job.

---

## 11. Annulation

Action WebSocket client→serveur `CANCEL_AI_GENERATION`, payload `{ "JOB_ID": "…" }`.

L'annulation prend effet **entre deux lots** — jamais au milieu d'un appel provider. Les
questions déjà écrites sont **conservées** ; le job termine en `CANCELLED` avec ses compteurs.

---

## 12. Cycle de vie du job

```
        POST /api/generate-questions
                    │
                    ▼
              RUNNING ──── lot k validé + écrit + broadcast ──┐
                    │                                          │
                    │◄─────────── délai inter-lots ────────────┘
                    │
      ┌─────────────┼──────────────┬─────────────────┐
      ▼             ▼              ▼                 ▼
    DONE         FAILED        CANCELLED      (redémarrage serveur)
 tous lots    N échecs         demande            job perdu
   traités   consécutifs        admin        questions conservées
```

**État en mémoire uniquement.** Un redémarrage serveur perd le job ; les questions déjà écrites
sont conservées (elles sont sur disque). Au démarrage, aucun job n'est restauré et aucun message
de progression n'est émis. Documenté comme limite acceptée du MVP.

---

## 13. Ce que ce contrat ne change pas

> **Amendement 2026-08-06** (#137 Batch 2a, `_work/reports/planner-20260806-121743-qualif-137.md`
> §2) : `ai-generation.md` §5-§6 a ajouté une 5ᵉ variante générable, `ARDOISE` (initialement
> décrite à tort par la spec #8 comme un mode d'affichage, pas un type de contenu — la
> caractérisation d'origine était inexacte, cf. §5 de `ai-generation.md`). Sans effet sur ce
> contrat : le découpage en lots, l'adaptation de schéma (§7) et le budget de contexte (§4)
> s'appliquent à `ARDOISE` exactement comme aux 4 types précédents, aucun traitement spécial au
> niveau provider.

- Le schéma de sortie du LLM (`ai-generation.md` §5) — hors adaptation §7.
- Le mapping vers `question.json` (§5.2), l'allocation d'ID verrouillée (§5.1), la garantie
  d'additivité.
- Les règles de secret (§2), appliquées à l'identique à la clé Groq.
- Le chemin Anthropic reste disponible et sélectionnable.
