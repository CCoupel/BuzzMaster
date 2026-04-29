# Commande /bugfix

Workflow pour la correction d'un bug.

## Usage

```
/bugfix <description du bug>
```

## Argument recu

$ARGUMENTS

## Mots-cles de controle

**Reference :** Voir `context/COMMON.md` section 12

| Mot-cle | Action |
|---------|--------|
| `help` | Affiche l'aide et les mots-cles disponibles |
| `status` | Affiche l'etat du workflow en cours |
| `plan` | Affiche le plan sans executer |
| `resume <phase>` | Reprend a une phase |
| `skip <phase>` | Saute une phase |
| `jumpto <tache>` | Demarre a une tache precise du plan |

Si `$ARGUMENTS` commence par un mot-cle -> executer l'action correspondante.
Sinon -> workflow normal.

## Workflow

```
/bugfix <description>
    |
    v
[CLARIFICATION] --> Etude spec + issues GitHub + questions si besoin
    |
    v
[ANALYSE] --> Identifier la cause racine
    |
    v
[PLAN] --> Plan de correction (si complexe)
    |
    v
[DEV] --> Implementation du fix
    |           |
    v           v
[REVIEW]   [TEST-WRITER] --> test de regression (en parallele)
    \           /
     v         v
      [QA] --> Execution + procedure manuelle
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

### 4. TEST-WRITER (parallele avec REVIEW)

**Obligatoire** : Test de non-regression ecrit par test-writer
- Script qui reproduit le bug avant le fix (red) et passe apres (green)
- Procedure manuelle dans `tests/procedures/` pour que QA valide le scenario
- Execute en parallele avec le code-reviewer — independants l'un de l'autre

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

## Prompt a transmettre au CDP

Orchestre le workflow BUGFIX pour {PROJECT_NAME}.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : BUGFIX
- Phases : section 3
- Clarification : section 4
- Dispatch DEV : section 5
- Validation : section 6
- Erreurs : section 7
- Regles : section 9

**Contexte DEV :** Voir `context/DEVELOPMENT.md`
**Contexte Qualite :** Voir `context/QUALITY.md`

**Demande utilisateur :** $ARGUMENTS

## Agent

Delegue au CDP (`cdp.md`) avec mode bugfix.
