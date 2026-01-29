# Commande /bugfix

Workflow pour la correction d'un bug.

## Usage

```
/bugfix <description du bug>
```

## Workflow

```
/bugfix <description>
    |
    v
[ANALYSE] --> Identifier la cause racine
    |
    v
[PLAN] --> Plan de correction (si complexe)
    |
    v
[DEV] --> Implementation du fix
    |
    v
[TEST] --> Test de non-regression
    |
    v
[REVIEW] --> Revue de code
    |
    v
[QA] --> Validation
    |
    v
[DOC] --> CHANGELOG (Fixed)
```

## Etapes Detaillees

### 1. ANALYSE

- Explorer le code pour comprendre le probleme
- Identifier le(s) fichier(s) concerne(s)
- Reproduire le bug si possible
- Determiner la cause racine

### 2. PLAN (optionnel)

Pour les bugs complexes uniquement :
- Plusieurs fichiers impactes
- Risque de regression
- Changement d'architecture

### 3. DEV

- Correction minimale et ciblee
- Eviter les changements non lies au bug
- Ajouter des commentaires si logique complexe

### 4. TEST

**Obligatoire** : Test de non-regression
- Reproduit le bug avant le fix
- Valide que le fix corrige le probleme
- S'assure qu'il n'y a pas de regression

### 5. REVIEW

- Verification que le fix est correct
- Pas d'effets de bord
- Code propre et maintenable

### 6. QA

- Execution de tous les tests
- Verification specifique du scenario du bug
- Build OK

### 7. DOC

Mise a jour CHANGELOG.md :
```markdown
### Fixed
- Description du bug corrige (#issue)
```

## Exemples

```
/bugfix Le score ne s'affiche pas apres une partie
/bugfix Crash au demarrage sur iOS 15
/bugfix L'API retourne 500 sur /users sans parametres
/bugfix Le bouton submit reste desactive apres erreur
```

## Differences avec /hotfix

| Aspect | /bugfix | /hotfix |
|--------|---------|---------|
| Urgence | Normal | Critique (prod down) |
| Tests | Complets | Critiques uniquement |
| Review | Standard | Acceleree |
| Deploy | Via workflow normal | Direct PROD |

## Agent

Delegue au CDP (`cdp.md`) avec mode bugfix.
