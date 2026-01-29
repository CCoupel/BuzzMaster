# Commande /feature

Workflow complet pour l'implementation d'une nouvelle fonctionnalite.

## Usage

```
/feature <description de la fonctionnalite>
```

## Workflow

```
/feature <description>
    |
    v
[PLAN] --> Plan d'implementation detaille
    |
    v
[DEV] --> Implementation (agents tech selon stack)
    |
    v
[TEST] --> Ecriture des tests
    |
    v
[REVIEW] --> Revue de code
    |
    v
[QA] --> Execution tests + validation
    |
    v
[DOC] --> Documentation
    |
    v
[DEPLOY] --> Deploiement (sur demande)
```

## Etapes Detaillees

### 1. PLAN

L'agent `implementation-planner` analyse la demande et cree :
- Liste des composants impactes
- Taches detaillees avec fichiers concernes
- Tests requis
- Risques identifies

### 2. DEV

Les agents de developpement implementent selon la stack :
- `dev-backend` : API, services, modeles
- `dev-frontend` : UI, composants, pages
- `dev-firmware` : Code embarque (si applicable)

**Parallelisation** : Backend et Frontend peuvent etre developpes en parallele si independants.

### 3. TEST

L'agent `test-writer` cree :
- Tests unitaires
- Tests integration
- Tests E2E (si necessaire)

### 4. REVIEW

L'agent `code-reviewer` verifie :
- Qualite du code
- Securite (OWASP)
- Performance
- Conformite aux standards

### 5. QA

L'agent `qa` execute :
- Tests unitaires
- Tests integration
- Tests E2E
- Verification du build

### 6. DOC

L'agent `doc-updater` met a jour :
- CHANGELOG.md
- Documentation technique
- README si necessaire

### 7. DEPLOY

Sur demande de l'utilisateur :
- `/deploy qualif` : Environnement de test
- `/deploy prod` : Production (apres validation QUALIF)

## Exemples

```
/feature Ajouter l'authentification OAuth2
/feature Implementer la page de profil utilisateur
/feature Creer l'endpoint API pour les notifications
/feature Ajouter le support du mode sombre
```

## Options

Le CDP peut demander des clarifications si la description est ambigue :
- Scope (backend seul, full-stack, etc.)
- Priorite des sous-fonctionnalites
- Contraintes specifiques

## Sortie Anticipee

A tout moment, l'utilisateur peut :
- Demander de passer une etape
- Arreter le workflow
- Modifier le plan

## Agent

Delegue au CDP (`cdp.md`) qui orchestre les agents specialises.
