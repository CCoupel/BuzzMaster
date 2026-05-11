# Commande /context-audit

Audit du contexte documentaire projet : cartographie tous les fichiers de définition, détecte doublons, incohérences et références cassées, puis propose un découpage optimisé pour minimiser le contexte chargé par chaque agent.

## Usage

```
/context-audit [scope]
```

## Argument recu

$ARGUMENTS

## Scopes

| Scope | Description |
|-------|-------------|
| (vide) | Audit complet |
| `quick` | CLAUDE.md + context/ uniquement |
| `agents` | Focus optimisation découpage agents |
| `refs` | Références cassées uniquement |

Si `$ARGUMENTS` est vide → audit complet.
Sinon → restreindre aux phases correspondant au scope.

## Workflow

```
/context-audit
    |
    v
[DÉCOUVERTE] --> Cartographier tous les fichiers de définition
    |
    v
[ANALYSE] --> Détecter doublons, incohérences, refs cassées
    |         Étudier le découpage pour optimisation contexte agents
    v
[RAPPORT] --> Présenter écarts + plan de correction + gains estimés
    |
    v
[VALIDATION] --> Attendre confirmation utilisateur (NE PAS modifier avant)
    |
    v
[APPLICATION] --> Appliquer uniquement les corrections validées
```

---

## Phase 1 : DÉCOUVERTE

Scanner dans cet ordre :

**Fichiers racine :**
- `CLAUDE.md`
- `README.md` (si présent)

**Répertoire `.claude/` :**
- `agents/*.md` — agents spécialisés
- `commands/*.md` — commandes slash
- `commands/context/*.md` — contextes partagés
- `memory/MEMORY.md` — mémoire projet (périmètre limité : références structurelles uniquement)
- `project-config.json` — config projet générée par /init-project
- `settings.json`, `settings.local.json`

**Répertoire `TEMPLATE_claude/` (si présent) :**
- Même structure que ci-dessus (composants template)

Pour chaque fichier, extraire :
- Titres et sections (## / ###)
- Toutes les références croisées (`Voir context/...`, `Spec : .claude/agents/...`, `context/XXX.md section N`)
- Placeholders non remplacés (`{VARIABLE}`)
- Noms d'agents et commandes mentionnés

Produire en interne une carte : `fichier → [sections] → [références sortantes]`

---

## Phase 2 : ANALYSE

### 2.1 Doublons

Détecter tout bloc d'information présent dans ≥ 2 fichiers avec >70% de similarité :
- Règles répétées mot pour mot
- Listes identiques (agents, commandes, conventions)
- Descriptions de workflows redondantes

**Sévérité :**
- 🔴 Si la duplication crée un risque de divergence (ex : règle métier copiée)
- 🟡 Si redondant mais cohérent (ex : exemple répété à titre illustratif)

### 2.2 Incohérences

Détecter toute contradiction entre fichiers :
- Valeurs différentes pour la même variable (`{BUILD_CMD}`, noms de branches, etc.)
- Instructions contradictoires pour le même agent ou la même situation
- Versions ou stacks qui diffèrent entre les fichiers de définition

**Sévérité :** 🔴 systématiquement (une incohérence produit un comportement imprévisible).

### 2.3 Références cassées

Détecter dans les fichiers de définition :
- `Voir context/XXX.md` → fichier absent
- `Spec : .claude/agents/XXX.md` → agent absent
- `context/XXX.md section N` → section N inexistante dans le fichier cible
- `{PLACEHOLDER}` non remplacé dans un projet déjà initialisé
- Agent mentionné dans un workflow mais sans fichier `.md` correspondant

Détecter dans `MEMORY.md` (périmètre restreint — ne pas auditer le contenu opérationnel) :
- Noms d'agents mentionnés (`cdp`, `qa`, `dev-backend`...) → agent absent de `.claude/agents/`
- Noms de commandes mentionnées (`/feature`, `/deploy`...) → commande absente de `.claude/commands/`
- Chemins de fichiers mentionnés explicitement → fichier absent du repo
- Ne PAS signaler : notes de session, décisions, résumés d'activité, branches git fermées

**Sévérité :**
- 🔴 Si la référence est chargée par un agent actif
- 🟡 Si dans une commande rarement utilisée ou dans MEMORY.md

### 2.4 Migration MEMORY → documents projet

MEMORY.md est volatile (état de session). Les docs projet sont stables (source de vérité structurelle).
Détecter les éléments de MEMORY qui auraient dû être promus vers les docs :

**Signes qu'un élément doit migrer :**
- Règle ou convention formulée comme permanente ("toujours faire X", "ne jamais faire Y")
- Décision d'architecture stabilisée ("on utilise X pour Y")
- Correction comportementale récurrente (notée plusieurs sessions de suite)
- Information présente dans MEMORY ET dans un doc projet avec des valeurs différentes
  → migration avec contradiction : classifier 🔴, appliquer les règles d'arbitrage des incohérences
    (résolution évidente → Auto ; ambiguë → ❓ demander laquelle fait référence)

**Classification de la migration (sans contradiction) :**

- **Évidente et pertinente → Auto** : la formulation est clairement une règle permanente
  (langage normatif : "toujours", "jamais", "obligatoire"), le doc cible est identifiable sans ambiguïté,
  et le contenu est de nature structurelle (convention, architecture, comportement agent)
- **Ambiguë → confirmation** : doute sur la permanence (décision récente pas encore stabilisée ?),
  doute sur la pertinence (spécifique à une session ou générique ?),
  ou doc cible incertain (CLAUDE.md ? context/COMMON.md ? un agent spécifique ?)

**Ce qui doit rester dans MEMORY (ne pas migrer) :**
- Branche courante, version, milestone en cours
- Travail en cours (phase, issues actives)
- Notes de session, résumés d'activité
- Décisions récentes pas encore stabilisées

**Sévérité :**
- 🔴 Contradiction entre MEMORY et un doc projet sur une règle structurelle
- 🟡 Règle permanente dans MEMORY non reflétée dans les docs projet
- 🔵 Information présente dans MEMORY qui pourrait enrichir un doc existant

### 2.5 Optimisation du découpage agents

Pour chaque agent dans `.claude/agents/` :

1. Lister tous les contextes qu'il charge explicitement (`Voir ...`)
2. Identifier les sections réellement utilisées vs le fichier entier chargé
3. Estimer les lignes inutilement chargées (sections d'autres agents, workflows non pertinents)

Proposer des actions :
- **Fusion** : deux contextes toujours chargés ensemble → un seul fichier
- **Extraction** : section lourde utilisée par un seul agent → fichier dédié
- **Suppression** : section orpheline (plus référencée par aucun agent)

---

## Phase 3 : RAPPORT

Présenter dans cet ordre, sans modifier aucun fichier.

### Tableau des écarts

Chaque écart porte une résolution provisoire : `Auto` si elle est évidente, `❓` si un arbitrage est nécessaire.

```
| ID    | Sévérité | Type              | Fichier(s)                    | Description                                       | Résolution |
|-------|----------|-------------------|-------------------------------|---------------------------------------------------|------------|
| EC-01 | 🔴       | Contradiction     | CLAUDE.md, DEVELOPMENT.md     | BUILD_CMD défini différemment                     | ❓         |
| EC-02 | 🔴       | Référence cassée  | commands/feature.md           | Voir context/DEPLOY.md → absent                   | Auto       |
| EC-03 | 🔴       | Contradiction     | MEMORY.md, CLAUDE.md          | Règle de branche dans MEMORY ≠ CLAUDE.md          | Auto       |
| EC-04 | 🟡       | Doublon           | CLAUDE.md, context/COMMON.md  | Conventions de commit répétées                    | Auto       |
| EC-05 | 🟡       | Migration MEMORY  | MEMORY.md                     | Convention de commit permanente absente des docs  | Auto       |
| EC-06 | 🔵       | Optimisation      | agents/qa.md                  | Charge DEVELOPMENT.md (inutile pour QA)           | Auto       |
```

**Résolution automatique (sans question) :**
- Doublon → conserver la version la plus complète, pointer les autres vers elle
- Référence cassée → corriger ou supprimer la référence
- Optimisation du découpage agents → appliquer directement

**Nécessite validation :**
- Contradiction/Incohérence → arbitrage si ambigu (❓), puis confirmation individuelle
- Migration MEMORY sans contradiction, évidente et pertinente → Auto (aucune question)
- Migration MEMORY sans contradiction, ambiguë → confirmation individuelle
- Migration MEMORY avec contradiction → traiter comme une incohérence : arbitrage si ambigu (❓), puis confirmation individuelle

### Arbitrage des contradictions ambiguës

Pour chaque écart marqué ❓, poser la question **avant de construire le plan**, et **attendre la réponse** :

```
❓ EC-01 — Contradiction : BUILD_CMD
  CLAUDE.md ligne 42      : `make build`
  DEVELOPMENT.md ligne 15 : `go build ./...`

  Quelle valeur fait référence ?
    [1] CLAUDE.md    → make build
    [2] DEVELOPMENT.md → go build ./...
    [3] Autre → préciser
```

Une fois toutes les ❓ résolues, construire le plan complet.

### Plan de correction

```
[EC-01] 🔴 Contradiction — BUILD_CMD  [résolution : CLAUDE.md fait foi]
  Action : Remplacer DEVELOPMENT.md ligne 15 par "Voir CLAUDE.md"
  Gain   : 1 source de vérité éliminée

[EC-02] 🔴 Référence cassée — context/DEPLOY.md  [Auto]
  Action : Remplacer la référence dans commands/feature.md
           par "Voir context/COMMON.md section 5"
  Gain   : Référence résolue

[EC-03] 🔴 Contradiction MEMORY — règle de branche  [Auto : doc fait foi]
  Action : Corriger MEMORY.md pour aligner sur CLAUDE.md
  Gain   : MEMORY cohérente avec la source de vérité

[EC-04] 🟡 Doublon — Conventions de commit  [Auto]
  Action : Remplacer la section CLAUDE.md par "Voir context/COMMON.md section 7"
  Gain   : -11 lignes de contexte chargé systématiquement

[EC-05] 🟡 Migration MEMORY — Convention de commit  [Auto]
  Action : Ajouter dans context/COMMON.md section 7, retirer de MEMORY.md
  Gain   : Règle permanente dans la source de vérité structurelle

[EC-06] 🔵 Optimisation — agents/qa.md  [Auto]
  Action : Extraire 3 sections pertinentes vers context/QUALITY.md
  Gain   : -80 lignes par invocation QA
```

### Gains estimés

```
| Métrique                             | Avant | Après | Delta |
|--------------------------------------|-------|-------|-------|
| Lignes totales (fichiers définition) | X     | Y     | -Z    |
| Contexte moyen chargé / agent        | X     | Y     | -Z    |
| Sources de vérité dupliquées         | X     | 0     | -X    |
| Références cassées                   | X     | 0     | -X    |
```

---

## Phase 4 : VALIDATION

Pas de question globale. Les règles par type :

| Type | Comportement |
|------|-------------|
| Doublon | Auto — aucune question |
| Référence cassée | Auto — aucune question |
| Optimisation | Auto — aucune question |
| Contradiction/Incohérence | Arbitrage si ❓, puis confirmation individuelle |
| Migration MEMORY sans contradiction, évidente | Auto — aucune question |
| Migration MEMORY sans contradiction, ambiguë | Confirmation individuelle |
| Migration MEMORY avec contradiction | Arbitrage si ❓, puis confirmation individuelle |

Pour chaque **contradiction** (arbitrage déjà résolu en phase 3) et chaque **migration MEMORY**, présenter l'action et attendre la réponse avant de passer au suivant :

```
EC-01 🔴 Contradiction — BUILD_CMD  [résolution : CLAUDE.md fait foi]
  Action : Remplacer DEVELOPMENT.md ligne 15 par "Voir CLAUDE.md"
  Appliquer ? [O/n]
```

```
EC-05 🟡 Migration MEMORY — Convention de commit
  Action : Ajouter dans context/COMMON.md section 7, retirer de MEMORY.md
  Appliquer ? [O/n]
```

- Si **O** → noter comme validé, passer au suivant
- Si **n** → noter comme ignoré, passer au suivant

**Ne modifier aucun fichier pendant cette phase** — collecter toutes les réponses, appliquer ensuite en phase 5.

---

## Phase 5 : APPLICATION

Appliquer uniquement ce qui a été validé, dans l'ordre de dépendance (si EC-02 crée un fichier référencé par EC-05, appliquer EC-02 d'abord).

Pour chaque correction :
1. Annoncer : `Correction EC-XX : <description courte>`
2. Modifier le(s) fichier(s)
3. Vérifier que les références croisées restent cohérentes
4. Confirmer : `✓ EC-XX appliqué`

Résumé final — tableau récapitulatif complet de tous les écarts :

```
| ID    | Sévérité | Type              | Description                        | Résolution       | Statut     |
|-------|----------|-------------------|------------------------------------|------------------|------------|
| EC-01 | 🔴       | Contradiction     | BUILD_CMD différent                | CLAUDE.md        | ✅ Appliqué |
| EC-02 | 🔴       | Référence cassée  | context/DEPLOY.md absent           | Auto             | ✅ Appliqué |
| EC-03 | 🟡       | Doublon           | Conventions de commit répétées     | Auto             | ✅ Appliqué |
| EC-04 | 🟡       | Migration MEMORY  | Convention permanente              | Auto             | ✅ Appliqué |
| EC-05 | 🟡       | Migration MEMORY  | Décision ambiguë                   | —                | ⏭ Ignoré  |
| EC-06 | 🔵       | Optimisation      | agents/qa.md charge DEVELOPMENT.md | Auto             | ✅ Appliqué |
```

Suivi des métriques :

```
| Métrique                             | Avant | Après | Delta |
|--------------------------------------|-------|-------|-------|
| Lignes totales (fichiers définition) | X     | Y     | -Z    |
| Contexte moyen chargé / agent        | X     | Y     | -Z    |
| Sources de vérité dupliquées         | X     | 0     | -X    |
| Références cassées                   | X     | 0     | -X    |
```

Fichiers modifiés : liste des chemins touchés.

---

## Règles Critiques

1. **Jamais de modification avant la phase 5** — rapport, arbitrages et validations sont read-only
2. **Contradictions ambiguës : toujours arbitrer avant de planifier** — ne jamais choisir unilatéralement
3. **Auto sans question** : doublons, références cassées, optimisations
4. **Validation individuelle** : contradictions/incohérences et migrations MEMORY ambiguës
5. **Migration MEMORY évidente et pertinente** : Auto, comme un doublon
6. **Migration MEMORY avec contradiction** : traiter comme une incohérence — arbitrage si ambigu, puis validation individuelle
4. **Conserver le comportement** — une correction ne change pas la sémantique, seulement l'organisation
5. **Références vérifiées** — après chaque modification, vérifier que les liens entrants/sortants sont cohérents

## Agent

Exécute directement dans la session principale — ne pas déléguer.
Accès complet aux fichiers requis pour lire et, après validation, modifier.
