# Commande /feature - Workflow Feature Complet

Orchestre le workflow complet de développement d'une feature via le **Chef De Projet (CDP)**.

## Argument reçu

$ARGUMENTS

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
Orchestre le workflow complet de développement d'une feature pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json
- Backlog : backlog/*.md
- Architecture : CLAUDE.md

**Demande utilisateur :** $ARGUMENTS

**Type de workflow :** FEATURE (nouvelle fonctionnalité)

---

## Phase 0 : Recherche Backlog

1. Lire `backlog/README.md` pour lister les entrées
2. Chercher une correspondance avec "$ARGUMENTS"
3. Si trouvée : Demander confirmation à l'utilisateur
4. Si confirmée : Utiliser le backlog pour le PLAN

⏸️ VALIDATION : Confirmer l'entrée backlog

---

## Phase 1 : Planification

1. Lancer l'agent `implementation-planner`
2. Créer branche `feature/<nom-court>`
3. Incrémenter version mineure (y) : 2.40.0 → 2.41.0
4. Commit initial et push
5. Produire le plan structuré

⏸️ VALIDATION : L'utilisateur doit valider le plan

---

## Phase 2 : Développement

Analyser le plan pour déterminer la stratégie :

**Si dépendances backend → frontend :**
1. Lancer `dev-backend` avec les tâches backend
2. Attendre le résumé (nouvelles actions WS, champs GameState)
3. Lancer `dev-frontend` avec les tâches frontend + résumé backend

**Si indépendant :**
1. Lancer `dev-backend` ET `dev-frontend` en parallèle

**Si backend seul :**
1. Lancer `dev-backend` uniquement

**Si frontend seul :**
1. Lancer `dev-frontend` uniquement

---

## Phase 3 : Revue de Code

1. Lancer l'agent `code-reviewer`
2. Analyser le verdict :
   - APPROVED → Phase 4
   - APPROVED WITH RESERVATIONS → Phase 4 (noter réserves)
   - REJECTED → Retour Phase 2 avec corrections (cycle++)

---

## Phase 4 : Tests QA

1. Lancer l'agent `QA`
2. Analyser le verdict :
   - VALIDATED → Phase 5
   - VALIDATED WITH RESERVATIONS → ⏸️ Demander confirmation
   - NOT VALIDATED → Retour Phase 2 avec erreurs (cycle++)

Si cycle > 3 → ⏸️ ESCALADE utilisateur

---

## Phase 5 : Documentation

1. Lancer l'agent `doc-updater`
2. Type : feature
3. Finaliser version (reset z à 0)

---

## Phase 6 : Déploiement QUALIF

1. Lancer l'agent `deploy` avec target=QUALIF
2. Build Windows + ARM64
3. Créer archive QUALIF

---

## Fin du workflow

Produire le rapport final avec :
- Durée totale
- Cycles effectués
- Livrables produits
- Prochaines étapes (valider QUALIF, puis /deploy PROD)
```

## Points de validation CDP

| Point | Qui décide | Options |
|-------|------------|---------|
| Backlog | Utilisateur | Confirmer / Refuser / Autre |
| Plan | Utilisateur | Valider / Modifier / Refuser |
| QA avec réserves | Utilisateur | Continuer / Corriger |
| Escalade (3 cycles) | Utilisateur | Continuer / Abandonner |

## Gestion des erreurs par CDP

| Situation | Action CDP |
|-----------|------------|
| Backlog non trouvé | Proposer création ou continuer sans |
| Plan refusé | Demander modifications |
| Review rejetée | Retour DEV avec corrections |
| QA échoue | Retour DEV avec erreurs |
| Build échoue | Retour DEV avec erreur build |
| 3 cycles atteints | Escalade utilisateur |

## Avantages du CDP

- **Décision intelligente** : Parallélise ou séquence selon les dépendances
- **Gestion des cycles** : Automatique jusqu'à 3 cycles
- **Reporting** : Progression en temps réel
- **Moins d'intervention** : Validation uniquement aux points clés

## Action immédiate

Lance maintenant le sous-agent **CDP** avec le Task tool pour orchestrer cette feature.
