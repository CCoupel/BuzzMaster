# Commande /feature - Workflow Feature Complet

Orchestre le workflow complet de développement d'une feature via le **Chef De Projet (CDP)**.

## Argument reçu

$ARGUMENTS

## Mots-clés de contrôle

**Référence :** Voir `context/COMMON.md` section 12

| Mot-clé | Action |
|---------|--------|
| `help` | Affiche l'aide et les mots-clés disponibles |
| `status` | Affiche l'état du workflow en cours |
| `plan` | Affiche le plan sans exécuter |
| `resume <phase>` | Reprend à une phase (init/plan/dev/review/qa/doc/deploy) |
| `skip <phase>` | Saute une phase |
| `jumpto <tâche>` | Démarre à une tâche précise du plan |

Si `$ARGUMENTS` commence par un mot-clé → exécuter l'action correspondante.
Sinon → workflow normal.

## Workflow orchestré par CDP

```
CDP → [Backlog] → PLAN → DEV → REVIEW → QA → DOC → DEPLOY(QUALIF) → [FIN]
```

**Note** : Le déploiement en PROD se fait via `/deploy PROD` après validation de la QUALIF.

## Instructions

Cette commande lance le sous-agent **CDP** qui orchestre automatiquement le workflow complet.

### Lancement du CDP

Lance le sous-agent **cdp** via Task tool :

```
subagent_type: "cdp"
description: "Orchestrer feature complète"
prompt: voir ci-dessous
```

### Prompt à transmettre au CDP

```
Orchestre le workflow FEATURE pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : FEATURE
- Phases : section 3 (workflow standard)
- Dispatch DEV : section 4 (Phase Dev)
- Validation : section 5
- Erreurs : section 6
- Règles : section 8 (FEATURE)

**Demande utilisateur :** $ARGUMENTS

**Spécificités FEATURE :**
- Incrémente Y : 2.40.0 → 2.41.0
- Branche : `feature/<nom-court>`
- Backlog : Rechercher dans backlog/*.md, demander confirmation
- Scope large autorisé
- Documentation complète requise
```

## Références CDP

**Points de validation :** Voir `context/CDP_WORKFLOWS.md` section 5
**Gestion erreurs :** Voir `context/CDP_WORKFLOWS.md` section 6
**Rapport final :** Voir `context/CDP_WORKFLOWS.md` section 7

## Action immédiate

Lance maintenant le sous-agent **CDP** avec le Task tool pour orchestrer cette feature.
