# Contrat — Validation de clé API par appel réel au fournisseur

> **Étend** `contracts/ai-generation.md` (#8) et `contracts/ai-multi-provider.md` (#137).
> Tout ce qui n'est pas redéfini ici reste valable.
> **Plan** : `_work/reports/plan-20260809-104602.md`
> **Maquette** : `_work/reports/mockup-ai-key-validation-20260809-104602.html`

---

## 1. Pourquoi

Aujourd'hui, `POST /config.json` accepte une clé API sur la seule base d'un **contrôle de
préfixe** (`sk-ant-` / `gsk_`, `http.go:1248-1255`). Une clé bien formée mais révoquée,
tronquée ou copiée depuis le mauvais compte est enregistrée sans broncher : l'échec n'apparaît
que bien plus tard, au premier « ✨ Générer via IA », sous la forme d'une erreur de génération
asynchrone (`AI_GENERATION_PROGRESS.ERROR_MESSAGE`) — à distance du geste qui l'a causée.

Ce contrat déplace la détection au moment de l'enregistrement, par un **appel réel au
fournisseur**, tout en laissant l'opérateur passer outre (poste hors-ligne, coupure temporaire,
fournisseur en incident).

---

## 2. Appel de validation — normatif

| Fournisseur | Requête | En-têtes |
|---|---|---|
| `anthropic` | `GET {ANTHROPIC_BASE_URL:-https://api.anthropic.com}/v1/models?limit=1` | `x-api-key: <clé>`, `anthropic-version: 2023-06-01` |
| `groq` | `GET {GROQ_BASE_URL:-https://api.groq.com}/openai/v1/models` | `Authorization: Bearer <clé>` |

**Pourquoi `/models` et non un appel de génération minimal** — trois raisons cumulées :

1. **Coût nul.** `/models` ne consomme ni tokens ni crédit. Un appel de génération, même
   minimal, débiterait le compte Anthropic à chaque clic sur « Enregistrer ».
2. **Budget Groq.** Le tier gratuit plafonne à **8 000 TPM** (`ai-multi-provider.md` §1), et la
   porte TPM de Groq compte `prompt_tokens + max_tokens demandé` **avant** génération (§6,
   calibration T0.1). Une validation par génération mangerait le budget de la génération réelle
   qui suit immédiatement.
3. **Diagnostic net.** `/models` isole strictement l'authentification. Un appel de génération
   mélange l'échec d'auth avec les échecs de schéma, de modèle indisponible et de quota — c'est
   exactement l'ambiguïté qui a rendu #142 difficile à diagnostiquer.

> **Limite assumée et documentée** : `/models` valide **l'authentification**, pas que le modèle
> configuré (`ai.model` / `ai.groq_model`) soit accessible ni que la génération structurée
> aboutisse. Une clé `valid` ici peut encore échouer en génération pour une autre raison. Le
> libellé UI dit « Clé vérifiée », jamais « Génération garantie ».

**Timeout** : **10 s**, en dur pour ce chemin. `ai.timeout_seconds` (défaut **300 s**) régit la
génération de fond et est inutilisable pour une action interactive.

**Aucune reprise** : pas de retry, pas de backoff. L'utilisateur a un bouton « Réessayer ».

---

## 3. Taxonomie du résultat — normatif

Exactement **trois** issues. Tout le comportement UI en découle.

| `result` | Déclencheurs | Sens |
|---|---|---|
| `valid` | `200` | Le fournisseur a authentifié la clé. |
| `invalid_key` | `401`, `403` | Le fournisseur a explicitement **refusé** la clé. |
| `unreachable` | erreur réseau/DNS/TLS, timeout, `5xx`, `429`, tout autre non-`2xx` | La clé **n'a pas pu être vérifiée**. Elle n'est ni confirmée ni infirmée. |

> **`429` → `unreachable`, délibérément.** Un rate-limit signifie que l'auth est probablement
> passée, mais rien n'est confirmé. Le classer `valid` mentirait ; le classer `invalid_key`
> ferait rejeter une clé correcte. `unreachable` — « je n'ai pas pu vérifier » — est la seule
> lecture honnête, et c'est aussi celle qui présente la bonne question à l'utilisateur.

La distinction porte tout le sens de la fonctionnalité : `invalid_key` dit *« corrige ta clé »*,
`unreachable` dit *« ta clé est peut-être bonne, réessaie plus tard »*. Elles ne doivent jamais
être fusionnées en un « erreur de validation » générique.

---

## 4. Endpoint dédié plutôt qu'extension de `POST /config.json`

Deux routes étaient possibles. Le choix est **un endpoint dédié**, pour quatre raisons.

**A — `POST /config.json` est déjà le point le plus délicat du serveur.** Sa section `ai` porte
une fusion champ-par-champ introduite en correction d'un bug bloquant trouvé en QA sur #137
(`http.go:1155-1171` : enregistrer la seule clé Groq remettait `provider` à `anthropic`), plus
la sémantique « clé vide = préservée » des deux secrets. Y greffer une branche « valider d'abord,
puis peut-être écrire, sauf si forcé » superpose une machine à états à un handler qui en porte
déjà une.

**B — Un endpoint de validation n'écrit rien.** Le séparer rend cette propriété
*structurelle* plutôt que conditionnelle : impossible qu'un chemin de validation persiste par
accident, puisque le code de validation n'a pas accès à l'écriture. Avec un drapeau
`validate: true` sur l'endpoint de sauvegarde, cette garantie ne tiendrait que par la
discipline des branches.

**C — Le forçage exige de toute façon un second aller-retour.** L'utilisateur doit voir le
verdict avant de décider. Un drapeau inline n'économise donc un appel que dans le cas nominal,
et en ajoute un dans les cas d'échec — précisément ceux qui comptent ici.

**D — Le chantier `ConfigPage.jsx` est actif.** Les tâches #7/#8 modifient ce fichier au moment
où ce plan est écrit. Un endpoint séparé, avec son propre fichier serveur et ses propres tests,
réduit la surface de conflit à la portion strictement nécessaire.

> **Contrepartie assumée** : la clé transite deux fois sur le réseau dans le cas nominal
> (validation puis sauvegarde). Sur une boucle locale ou un LAN d'administration — le contexte de
> déploiement de BuzzControl — c'est sans conséquence, et `POST /config.json` transporte déjà
> cette même clé en clair aujourd'hui. Aucune exposition nouvelle.

---

## 5. `POST /api/ai/validate-key`

**Description** : valide une clé API auprès de son fournisseur, sans rien persister.
**Auth** : aucune (cohérent avec le reste de la surface admin BuzzControl, LAN).
**Effet de bord** : **aucun**. N'écrit ni `config.json` ni le singleton. Purement en lecture.

### Request body

```json
{
  "provider": "anthropic",
  "api_key": "sk-ant-..."
}
```

| Champ | Type | Règle |
|---|---|---|
| `provider` | string | `"anthropic"` \| `"groq"`. Requis. Toute autre valeur → `400`. |
| `api_key` | string | Optionnel. **Absent ou vide** → valide la clé **effective** actuellement stockée pour ce fournisseur (`EffectiveAnthropicAPIKey()` / `EffectiveGroqAPIKey()`, donc variable d'environnement prioritaire sur `config.json`). |

Contrôle de préfixe appliqué **avant** tout appel réseau, identique à celui de
`POST /config.json` (`sk-ant-` / `gsk_`) → `400` si violé. Inutile de déranger le fournisseur
pour une chaîne qui ne peut pas être une clé.

`http.MaxBytesReader` à **64 Ko** (une clé, pas un document).

### Response 200

Le **résultat de validation vit dans le corps, jamais dans le statut HTTP**. `200` signifie
« la validation a été menée à son terme et voici son verdict » — y compris quand le verdict est
`invalid_key`. Cela évite au frontend de confondre « le fournisseur a refusé la clé » avec
« notre propre serveur a échoué ».

```json
{
  "result": "invalid_key",
  "provider": "anthropic",
  "http_status": 401,
  "detail": "invalid x-api-key"
}
```

| Champ | Type | Présence |
|---|---|---|
| `result` | string | Toujours. `"valid"` \| `"invalid_key"` \| `"unreachable"` |
| `provider` | string | Toujours. Écho du fournisseur validé. |
| `http_status` | int | Statut HTTP amont, `0` si aucune réponse reçue (réseau/timeout). |
| `detail` | string | Optionnel. Message amont **assaini** via `sanitizeUpstreamMessage` (`ai_client.go:111`) — jamais la clé, jamais le corps brut. Vide si indisponible. |

### Errors

| Statut | Cas |
|---|---|
| `400` | JSON malformé, `provider` absent/inconnu, préfixe de clé invalide |
| `405` | Méthode ≠ POST |
| `413` | Corps > 64 Ko |
| `429` | Cooldown serveur dépassé (§8) |

> **Aucun `5xx` pour un échec fournisseur** : un fournisseur injoignable est un résultat
> `unreachable` en `200`, pas une erreur de notre serveur.

---

## 6. Extension de l'abstraction `aiProvider`

L'abstraction #137 (`ai_provider.go:14-30`) **ne possède aujourd'hui aucun mécanisme de
vérification de connectivité** — ses trois méthodes sont `Generate`, `AdaptSchema`, `Name`.
Elle est étendue d'une quatrième :

```go
// ValidateKey vérifie l'authentification auprès du fournisseur (§2) sans
// consommer de tokens. key vide => clé effective stockée.
ValidateKey(ctx context.Context, cfg config.AIConfig, key string) keyValidationResult
```

Elle retourne un résultat, **pas une `error`** : `invalid_key` et `unreachable` sont deux issues
nominales de la validation, pas des pannes. Les types d'erreur existants (`aiTimeoutError`,
`aiUpstreamError`, `aiRateLimitError`) restent réservés au chemin de génération.

`selectProvider` (`ai_provider.go:149`) est réutilisé tel quel pour router vers l'implémentation.
Les deux implémentations utilisent **`net/http` stdlib** (motif `ai_groq.go` / `github_client.go`),
y compris côté Anthropic : la validation classe sur le **statut HTTP brut**, ce que le SDK
enveloppe. Les surcharges `ANTHROPIC_BASE_URL` / `GROQ_BASE_URL` sont honorées explicitement,
pour que les tests pointent vers un `httptest` local — comme le fait déjà `generateViaGroq`.

**Aucune modification du chemin de génération.** `Generate`, `AdaptSchema`, la boucle de lots et
la classification d'erreurs existante sont strictement inchangées.

---

## 7. Persistance de l'état « vérifiée »

Deux champs ajoutés à `config.AIConfig`, **persistés** :

```go
AnthropicAPIKeyVerified bool `json:"anthropic_api_key_verified"`
GroqAPIKeyVerified      bool `json:"groq_api_key_verified"`
```

| Événement | Effet |
|---|---|
| Clé enregistrée après `result: "valid"` | flag → `true` |
| Clé enregistrée en forçant (`invalid_key` ou `unreachable`) | flag → `false` |
| `clear_api_key` / `clear_groq_api_key` | flag → `false` |
| Clé changée sans validation (appel direct à l'API, hors UI) | flag → `false` |

**Pourquoi persister plutôt que dériver** : le flag enregistre un **événement passé** (« une
validation a réussi pour cette clé »), pas un état calculable. Sans persistance, le badge
afficherait « ✅ Clé configurée » après un simple rechargement de page pour une clé que
l'utilisateur avait justement enregistrée de force malgré un refus — l'UI mentirait exactement
dans le cas où l'avertissement compte le plus.

Ces champs suivent la règle de fusion champ-par-champ de la section `ai` (`http.go:1242-1328`) et
sont retournés tels quels par `GET /config.json` (ce ne sont pas des secrets).

> **Cas variable d'environnement** : une clé fournie par `BUZZCONTROL_*_API_KEY` ne transite
> jamais par l'UI, donc son flag reste `false`. C'est le cas d'usage que couvre §9 — champ vide
> + « Enregistrer » valide la clé effective, ce qui permet à un déploiement PROD configuré par
> variable d'environnement de passer son flag à `true` sans jamais écrire de secret sur disque.

---

## 8. Sécurité

| Point | Règle |
|---|---|
| Clé en réponse | **Jamais.** Ni écho, ni fragment, ni longueur. |
| Clé en log | **Jamais.** On journalise `provider` + `result` + `http_status`, rien d'autre (motif M4, `ai_groq.go:188`). |
| `detail` | Obligatoirement passé par `sanitizeUpstreamMessage` avant sortie. |
| Corps amont | Jamais relayé brut. |
| Cooldown | **1 validation / 2 s**, global au serveur, dépassement → `429`. Borne un oracle de test de clés en rafale et protège le quota du fournisseur. |
| Cible réseau | URL **jamais** dérivée d'une entrée utilisateur — uniquement la constante du fournisseur ou sa surcharge d'environnement. Pas de surface SSRF. |

> L'endpoint permet à un appelant du LAN de tester une clé arbitraire contre le fournisseur.
> Ce n'est **pas une nouvelle classe d'exposition** : `POST /config.json` accepte déjà l'écriture
> de clés sans authentification, et toute la surface admin BuzzControl est non authentifiée par
> conception (réseau local, `docs/ADMIN_GUIDE.md`). Le cooldown borne l'abus ; l'audit `security`
> confirme le raisonnement en Batch 1.

---

## 9. Séquence d'enregistrement — normatif

Déclencheur : bouton **« Enregistrer »** de la carte de clé (Claude ou Groq).

```
1. Champ vide ET aucune clé effective stockée
   → aucune validation, comportement actuel inchangé (enregistre les autres réglages ai.*)

2. Sinon → POST /api/ai/validate-key
   ├─ valid        → POST /config.json (clé + *_verified: true)   → « ✅ Clé vérifiée et enregistrée »
   ├─ invalid_key  → dialogue REFUS       → [Corriger] | [Enregistrer quand même]
   └─ unreachable  → dialogue INJOIGNABLE → [Réessayer] | [Corriger] | [Enregistrer quand même]

3. « Enregistrer quand même » → POST /config.json (clé + *_verified: false)
   → « ⚠️ Clé enregistrée sans vérification »
```

**« Enregistrer quand même » est offert dans les deux cas d'échec**, y compris sur
`invalid_key` : le refus vient du fournisseur, pas de nous, et l'opérateur peut légitimement
vouloir enregistrer (clé pas encore activée côté fournisseur, propagation en cours, compte en
cours de création).

**Champ vide + clé stockée → on valide la clé stockée.** Ce cas donne gratuitement un
« tester ma clé actuelle » sans bouton supplémentaire, et c'est le seul chemin par lequel une
clé issue d'une variable d'environnement peut être vérifiée (§7).

Les deux dialogues sont **modaux et bloquants** : l'enregistrement forcé est une décision
consciente, pas une bannière qu'on ignore.

`clear_api_key` / `clear_groq_api_key` (bouton « Supprimer la clé ») **ne déclenchent aucune
validation** — on ne valide pas une clé qu'on efface.

### 9.1 Libellés

La demande citait « Clé invalide » et « Serveur injoignable ». La maquette retient des libellés
plus explicites, qui **nomment le fournisseur et disent quoi faire** :

| Cas | Titre du dialogue | Corps |
|---|---|---|
| `invalid_key` | « Claude a refusé cette clé » | « La clé a bien été transmise, mais Claude ne la reconnaît pas. Vérifiez que vous l'avez copiée en entier et qu'elle n'a pas été révoquée. » |
| `unreachable` | « Impossible de joindre Groq » | « La clé n'a pas pu être vérifiée — elle n'est ni confirmée ni refusée. Si ce serveur est hors ligne, vous pouvez l'enregistrer telle quelle et la vérifier plus tard. » |

« Clé invalide » ne dit pas *qui* l'a jugée invalide (nous ou le fournisseur), et « Serveur
injoignable » peut se lire comme « le serveur BuzzControl est injoignable » — alors que c'est
précisément le fournisseur distant qui l'est. Nommer le fournisseur lève les deux ambiguïtés.

**Point ouvert (D4)** : si l'utilisateur préfère les libellés courts d'origine, seuls ces deux
titres changent — la logique, les codes et la structure des dialogues sont inchangés.

---

## 10. Impact sur l'existant

| Élément | Impact |
|---|---|
| `POST /config.json` | **Aucun changement de sémantique.** Deux champs `ai.*` supplémentaires suivant la fusion champ-par-champ existante. Un client qui ne les envoie pas se comporte exactement comme avant. |
| `GET /config.json` | Additif : deux booléens en plus. |
| Chemin de génération | **Strictement inchangé.** |
| `aiProvider` | Une méthode ajoutée. Aucune signature existante modifiée. |
| Contrôle de préfixe | Conservé, appliqué en amont de la validation réseau. |

**Aucun changement BREAKING.**
