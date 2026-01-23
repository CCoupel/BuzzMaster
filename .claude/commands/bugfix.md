# Commande /bugfix - Workflow Correction de Bug

Tu es l'**Orchestrateur** du système d'agents BuzzControl.

## Argument reçu

$ARGUMENTS

## Workflow à exécuter (simplifié)

```
DEV → QA → DOC → DEPLOY(QUALIF) → [validation] → DEPLOY(PROD)
```

**Note** : Pas d'agent PLAN pour les bugfixes (la correction est généralement claire).

## Instructions

### Étape 0 : Préparation Git

1. Créer la branche de bugfix :
   ```bash
   git checkout main
   git pull origin main
   git checkout -b bugfix/<nom-court>
   ```
2. Incrémenter la version patch (z) dans `server-go/config.json`
3. Commit et push initial

### Étape 1 : Agent DEV

1. Lire le fichier `.claude/agents/dev.md`
2. Analyser le bug décrit
3. Implémenter la correction
4. Ajouter un test de non-régression
5. Commits + push

### Étape 2 : Agent QA

1. Lire le fichier `.claude/agents/qa.md`
2. Exécuter tous les tests
3. Vérifier que le bug est corrigé
4. Vérifier qu'il n'y a pas de régression
5. Si échecs → retour à DEV

### Étape 3 : Agent DOC

1. Lire le fichier `.claude/agents/doc.md`
2. Mettre à jour CHANGELOG.md (section Fixed)
3. Commit et push

### Étape 4 : Agent DEPLOY (QUALIF)

1. Lire le fichier `.claude/agents/deploy.md`
2. Build et tests post-build
3. **ATTENDRE LA VALIDATION UTILISATEUR** avant PROD

### Étape 5 : Agent DEPLOY (PROD)

1. Squash merge dans main
2. Tag de version (ex: v2.39.1)
3. Push main + tag
4. Nettoyage branche bugfix

## Versioning Bugfix

- **Version actuelle** : `2.39.0`
- **Après bugfix** : `2.39.1` (incrémenter z)
- Le z n'est PAS remis à 0 pour un bugfix

## Gestion des erreurs

- Si QA échoue → relancer DEV avec les tests en échec
- Maximum 3 cycles DEV ↔ QA avant escalade

## Commence maintenant

1. Crée la branche `bugfix/<nom-court>`
2. Analyse le bug : **$ARGUMENTS**
3. Lance l'agent DEV pour implémenter la correction
