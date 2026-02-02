# Commande /bugfix - Workflow Correction de Bug

Orchestre le workflow simplifié de correction de bug via le **Chef De Projet (CDP)**.

## Argument reçu

$ARGUMENTS

## Mots-clés de contrôle

**Référence :** Voir `context/COMMON.md` section 12

| Mot-clé | Action |
|---------|--------|
| `status` | Affiche l'état du workflow en cours |
| `plan` | Affiche le plan sans exécuter |
| `resume <phase>` | Reprend à une phase (init/dev/review/qa/doc/deploy) |
| `skip <phase>` | Saute une phase |
| `jumpto <tâche>` | Démarre à une tâche précise du plan |

Si `$ARGUMENTS` commence par un mot-clé → exécuter l'action correspondante.
Sinon → workflow normal.

## Workflow orchestré par CDP

```
CDP → [Git] → DEV → REVIEW → QA → DOC → DEPLOY(QUALIF) → [FIN]
```

**Note** : Le déploiement en PROD se fait via `/deploy PROD` après validation de la QUALIF.

## Instructions

Cette commande lance le sous-agent **CDP** qui orchestre automatiquement le workflow bugfix.

### Lancement du CDP

Lance le sous-agent **cdp** via Task tool :

```
subagent_type: "cdp"
description: "Orchestrer bugfix"
prompt: voir ci-dessous
```

### Prompt à transmettre au CDP

```
Orchestre le workflow BUGFIX pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : BUGFIX
- Phases : section 3 (variante BUGFIX)
- Dispatch DEV : section 4 (Phase Dev)
- Validation : section 5
- Erreurs : section 6
- Règles : section 8 (BUGFIX)

**Description du bug :** $ARGUMENTS

**Spécificités BUGFIX :**
- Incrémente Z : 2.40.0 → 2.40.1
- Branche : `bugfix/<nom-court>`
- Scope minimal obligatoire
- Test non-régression OBLIGATOIRE
- Pas de refactoring
- CHANGELOG : section Fixed uniquement
```

## Références CDP

**Versioning :** Voir `context/CDP_WORKFLOWS.md` section 4 (Phase Versionnement)
**Validation :** Voir `context/CDP_WORKFLOWS.md` section 5
**Erreurs :** Voir `context/CDP_WORKFLOWS.md` section 6
**Comparaison types :** Voir `context/CDP_WORKFLOWS.md` section 2

## Action immédiate

Lance maintenant le sous-agent **CDP** avec le Task tool pour orchestrer ce bugfix.
