# Commande /bugfix - Workflow Correction de Bug

Orchestre le workflow simplifié de correction de bug via le **Chef De Projet (CDP)**.

## Argument reçu

$ARGUMENTS

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
Orchestre le workflow de correction de bug pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json
- Architecture : CLAUDE.md

**Description du bug :** $ARGUMENTS

**Type de workflow :** BUGFIX (correction de bug)

---

## Phase 0 : Préparation Git

1. Checkout main et pull
2. Créer branche `bugfix/<nom-court>`
3. Incrémenter version patch (z) : 2.40.0 → 2.40.1
4. Commit initial et push

---

## Phase 1 : Analyse et Correction

1. Analyser le bug décrit
2. Identifier la cause racine (backend/frontend/les deux)
3. Déterminer la stratégie de correction :
   - Backend seul → `dev-backend`
   - Frontend seul → `dev-frontend`
   - Les deux → Séquentiel ou parallèle selon dépendances

**Règles bugfix :**
- Correction minimale et ciblée
- Ne pas refactorer du code non lié
- Test de non-régression OBLIGATOIRE
- Ne pas incrémenter y (seulement z)

---

## Phase 2 : Revue de Code

1. Lancer l'agent `code-reviewer`
2. Vérifier que la correction est minimale
3. Analyser le verdict :
   - APPROVED → Phase 3
   - REJECTED → Retour Phase 1 avec corrections (cycle++)

---

## Phase 3 : Tests QA

1. Lancer l'agent `QA`
2. Vérifier que le bug est corrigé
3. Vérifier la non-régression
4. Analyser le verdict :
   - VALIDATED → Phase 4
   - NOT VALIDATED → Retour Phase 1 avec erreurs (cycle++)

Si cycle > 3 → ⏸️ ESCALADE utilisateur

---

## Phase 4 : Documentation

1. Lancer l'agent `doc-updater`
2. Type : bugfix
3. CHANGELOG.md : Section **Fixed** uniquement
4. NE PAS reset z à 0

---

## Phase 5 : Déploiement QUALIF

1. Lancer l'agent `deploy` avec target=QUALIF
2. Build Windows + ARM64
3. Créer archive QUALIF

---

## Fin du workflow

Produire le rapport final avec :
- Bug corrigé (description)
- Cause racine identifiée
- Fichiers modifiés
- Tests de non-régression ajoutés
- Prochaines étapes (valider QUALIF, puis /deploy PROD)
```

## Versioning Bugfix

| Version | Signification |
|---------|---------------|
| `2.39.0` | Version actuelle (feature) |
| `2.39.1` | Après bugfix (incrémenter z) |
| `2.39.2` | Deuxième bugfix sur même version |

**Important** : Le z n'est PAS remis à 0 pour un bugfix.

## Points de validation CDP

| Point | Qui décide | Options |
|-------|------------|---------|
| QA avec réserves | Utilisateur | Continuer / Corriger |
| Escalade (3 cycles) | Utilisateur | Continuer / Abandonner |

## Gestion des erreurs par CDP

| Situation | Action CDP |
|-----------|------------|
| Review rejetée | Retour DEV avec corrections |
| QA échoue | Retour DEV avec erreurs |
| Build échoue | Retour DEV avec erreur build |
| 3 cycles atteints | Escalade utilisateur |

## Différences avec /feature

| Aspect | /feature | /bugfix |
|--------|----------|---------|
| Version | Incrémente y | Incrémente z |
| Branche | `feature/xxx` | `bugfix/xxx` |
| Scope | Large | Minimal |
| Refactoring | Autorisé | Interdit |
| Test non-régression | Recommandé | Obligatoire |

## Action immédiate

Lance maintenant le sous-agent **CDP** avec le Task tool pour orchestrer ce bugfix.
