# Commande /feature - Workflow Feature Complet

Tu es l'**Orchestrateur** du système d'agents BuzzControl.

## Argument reçu

$ARGUMENTS

## Workflow à exécuter

```
PLAN → DEV → REVIEW → QA → DOC → DEPLOY(QUALIF) → [validation] → DEPLOY(PROD) → MARKETING
```

## Instructions

### Étape 1 : Agent PLAN

1. Lire le fichier `.claude/agents/plan.md` pour connaître les responsabilités de l'agent PLAN
2. Lancer l'agent PLAN avec la description fournie :
   - Analyser le backlog ou la description
   - Créer la branche `feature/<nom-court>`
   - Incrémenter la version mineure (y)
   - Créer le plan d'implémentation détaillé
   - Commit et push initial

3. **ATTENDRE LA VALIDATION UTILISATEUR** avant de continuer

### Étape 2 : Agent DEV

1. Lire le fichier `.claude/agents/dev.md`
2. Implémenter selon le plan validé
3. Commits par tâche + push en fin de cycle

### Étape 3 : Agent REVIEW

1. Lire le fichier `.claude/agents/review.md`
2. Analyser le code (qualité, sécurité, architecture)
3. Si problèmes critiques → retour à DEV

### Étape 4 : Agent QA

1. Lire le fichier `.claude/agents/qa.md`
2. Exécuter tous les tests (unit, E2E)
3. Si échecs → retour à DEV

### Étape 5 : Agent DOC

1. Lire le fichier `.claude/agents/doc.md`
2. Mettre à jour CHANGELOG.md, CLAUDE.md
3. Finaliser la version (z → 0)
4. Commit et push

### Étape 6 : Agent DEPLOY (QUALIF)

1. Lire le fichier `.claude/agents/deploy.md`
2. Build Windows + Linux ARM64
3. Tests post-build
4. Créer archive QUALIF
5. **ATTENDRE LA VALIDATION UTILISATEUR** avant PROD

### Étape 7 : Agent DEPLOY (PROD)

1. Squash merge dans main
2. Tag de version
3. Push main + tag
4. Nettoyage branche feature
5. Créer GitHub Release (optionnel)

### Étape 8 : Agent MARKETING

1. Lire le fichier `.claude/agents/marketing.md`
2. Mettre à jour le site (branche gh-pages)
3. Créer les release notes publiques
4. Préparer le contenu social

## Points de validation

- **Après PLAN** : L'utilisateur valide le plan avant implémentation
- **Après DEPLOY QUALIF** : L'utilisateur valide avant mise en production

## Gestion des erreurs

- Si REVIEW trouve des problèmes critiques → relancer DEV avec feedback
- Si QA échoue → relancer DEV avec les tests en échec
- Maximum 3 cycles DEV ↔ REVIEW/QA avant escalade

## Commence maintenant

Lance l'agent PLAN avec la description : **$ARGUMENTS**
