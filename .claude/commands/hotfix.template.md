# Commande /hotfix

Workflow accelere pour les corrections critiques en production.

## Usage

```
/hotfix <description du probleme critique>
```

## Quand utiliser

- Production cassee ou degradee
- Faille de securite active
- Perte de donnees en cours
- Impact business majeur

## Workflow Accelere

```
/hotfix <description>
    |
    v
[ANALYSE RAPIDE] --> Identification immediate
    |                 (pas de plan detaille)
    v
[FIX] --> Correction minimale
    |
    v
[TESTS CRITIQUES] --> Uniquement les tests essentiels
    |
    v
[DEPLOY PROD] --> Deploiement direct
    |
    v
[POST-MORTEM] --> Documentation de l'incident
```

## Etapes Detaillees

### 1. ANALYSE RAPIDE (max 15 min)

- Identifier le symptome exact
- Localiser le code responsable
- Determiner le fix minimal

**Pas de plan detaille** - On agit vite.

### 2. FIX

- Correction la plus simple possible
- Un seul commit
- Pas de refactoring
- Pas de features supplementaires

### 3. TESTS CRITIQUES

Uniquement :
- Test du scenario casse
- Smoke tests de base
- Build OK

**Pas de suite complete** - Sera fait apres.

### 4. DEPLOY PROD

Deploiement direct en production :

```bash
# Branche depuis main
git checkout -b hotfix/<name> main

# Fix + commit
git commit -m "fix: <description>"

# Merge et tag
git checkout main
git merge --no-ff hotfix/<name>
git tag v<version>-hotfix
git push origin main --tags
```

### 5. POST-MORTEM

Apres le fix, documenter :

```markdown
## Incident Report

**Date** : YYYY-MM-DD HH:MM
**Duree** : X heures
**Impact** : Description de l'impact

### Chronologie
- HH:MM - Detection du probleme
- HH:MM - Debut d'investigation
- HH:MM - Fix deploye
- HH:MM - Service restaure

### Cause Racine
Description technique de la cause.

### Fix Applique
Description du fix.

### Actions Preventives
- [ ] Action 1
- [ ] Action 2

### Lecons Apprises
- Point 1
- Point 2
```

## Exemples

```
/hotfix Base de donnees saturee, requetes timeout
/hotfix Faille XSS sur le formulaire de login
/hotfix Crash API suite au dernier deploy
/hotfix Certificat SSL expire
```

## Apres le Hotfix

1. **Tests complets** en background
2. **Revue de code** post-mortem
3. **Backport** vers les branches de dev si necessaire
4. **Communication** a l'equipe

## Regles Critiques

1. **Fix minimal** - Pas le moment d'ameliorer
2. **Un seul probleme** - Un hotfix = un bug
3. **Documenter** - Pour ne pas reproduire
4. **Communiquer** - Equipe informee
5. **Valider** - Monitoring post-deploy

## Agent

Mode special du CDP avec etapes reduites.
