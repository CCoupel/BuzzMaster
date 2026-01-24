# Commande /bugfix - Workflow Correction de Bug

Orchestre le workflow simplifié de correction de bug via les sous-agents spécialisés.

## Argument reçu

$ARGUMENTS

## Workflow simplifié

```
[Git] → DEV → QA → DOC → DEPLOY(QUALIF) → [FIN]
```

**Note** : Le déploiement en PROD se fait via `/deploy PROD` après validation de la QUALIF.

## Instructions

Cette commande orchestre le workflow bugfix jusqu'à la QUALIF. Elle lance les sous-agents dans l'ordre.

### Phase 0 : Préparation Git (manuelle)

Avant de lancer les sous-agents, prépare l'environnement Git :

```bash
git checkout main && git pull origin main
git checkout -b bugfix/<nom-court>
```

Puis incrémente z dans `server-go/config.json` : `2.39.0` → `2.39.1`

```bash
git add server-go/config.json
git commit -m "chore(version): Start v2.39.1 - bugfix <description>"
git push -u origin bugfix/<nom-court>
```

### Phase 1 : Correction (DEV)

Lance le sous-agent **dev-feature-implementation** via Task tool :

```
subagent_type: "dev-feature-implementation"
description: "Corriger bug"
prompt: "Corrige le bug pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json

**Description du bug :** $ARGUMENTS

**Actions :**
1. Analyser le bug décrit
2. Identifier la cause racine
3. Implémenter la correction minimale
4. Ajouter un test de non-régression
5. Commits atomiques + push

**Règles bugfix :**
- Correction minimale et ciblée
- Ne pas refactorer du code non lié
- Test de non-régression OBLIGATOIRE
- Ne pas incrémenter y (seulement z)"
```

### Phase 2 : Tests QA (QA)

Lance le sous-agent **QA** via Task tool :

```
subagent_type: "QA"
description: "Valider correction bug"
prompt: "Exécute la procédure de tests QA pour valider le bugfix BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/

**Actions :**
1. Build de production
2. Tests unitaires (vérifier non-régression)
3. Tests E2E
4. Vérifier que le bug est corrigé
5. Rapport QA

**Si échecs :** Retour à DEV avec les tests en échec."
```

### Phase 3 : Documentation (DOC)

Lance le sous-agent **doc-updater** via Task tool :

```
subagent_type: "doc-updater"
description: "Documenter bugfix"
prompt: "Mets à jour la documentation pour le bugfix BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Config version : server-go/config.json

**Type :** bugfix

**Actions :**
1. CHANGELOG.md : Ajouter entrée dans section **Fixed**
2. NE PAS reset z à 0 (garder la version patch)
3. Commit et push

**Format CHANGELOG :**
## [2.39.1] - YYYY-MM-DD

### Fixed
- **[Composant]**: Description du bug corrigé"
```

### Phase 4 : Déploiement QUALIF (DEPLOY)

Lance le sous-agent **deploy** via Task tool :

```
subagent_type: "deploy"
description: "Déploiement QUALIF bugfix"
prompt: "Déploie le bugfix BuzzControl vers l'environnement QUALIF.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json

**Environnement :** QUALIF

**Actions :**
1. Build Windows + ARM64
2. Tests post-build
3. Créer archive QUALIF"
```

## Fin du workflow /bugfix

**✅ Le workflow /bugfix s'arrête ici.**

Pour continuer vers la production :
1. **Valider la QUALIF** manuellement
2. **Lancer** `/deploy PROD` pour le déploiement en production

## Versioning Bugfix

| Version | Signification |
|---------|---------------|
| `2.39.0` | Version actuelle (feature) |
| `2.39.1` | Après bugfix (incrémenter z) |
| `2.39.2` | Deuxième bugfix sur même version |

**Important** : Le z n'est PAS remis à 0 pour un bugfix.

## Gestion des erreurs

| Situation | Action |
|-----------|--------|
| QA échoue | Retour à Phase 1 (DEV) avec tests en échec |
| Build échoue | Retour à Phase 1 (DEV) pour correction |
| Maximum 3 cycles DEV ↔ QA | Escalade vers utilisateur |

## Action immédiate

1. **Exécute la Phase 0** : Prépare l'environnement Git (branche bugfix, version z+1)
2. **Lance la Phase 1 (DEV)** avec le Task tool :

```
Task tool:
- subagent_type: "dev-feature-implementation"
- description: "Corriger bug"
- prompt: [prompt de la Phase 1 avec $ARGUMENTS]
```
