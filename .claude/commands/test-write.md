# Commande /test-write - Définition et Rédaction des Tests

Lance l'agent **test-writer** pour écrire les tests (unitaires, E2E) sans les exécuter.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/test-write` : Analyse les fichiers modifiés et écrit les tests manquants
- `/test-write <fichier>` : Écrit les tests pour un fichier spécifique
- `/test-write e2e` : Définit uniquement les scénarios E2E Chrome
- `/test-write unit` : Écrit uniquement les tests unitaires

## Différence avec /qa

| Commande | Action | Agent |
|----------|--------|-------|
| `/test-write` | **Écrit** les fichiers de tests | `test-writer` |
| `/qa` | **Exécute** les tests existants | `QA` |

## Instructions

Lance le sous-agent **test-writer** via Task tool :

```
subagent_type: "test-writer"
description: "Écrire les tests"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Écris les tests pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Tests Go : server-go/internal/**/*_test.go
- Tests E2E : server-go/tests/e2e/
- Frontend React : server-go/web/src/

**Input utilisateur :** $ARGUMENTS

**Actions :**

1. **Analyser les fichiers modifiés**
   - `git diff main --name-only` pour identifier les changements
   - Identifier les nouvelles fonctions/méthodes

2. **Écrire les tests unitaires Go**
   - Fichiers `*_test.go` correspondants
   - Pattern table-driven obligatoire
   - Couvrir : nominal, limites, erreurs

3. **Définir les scénarios E2E Chrome**
   - Créer/modifier `tests/e2e/scenarios.md`
   - Format : Étapes + Résultat attendu + Vérification Chrome
   - Utiliser MCP claude-in-chrome pour l'exécution

4. **Committer les tests**
   - Format : `test(<scope>): <description>`

**Types de tests à écrire :**

| Type | Fichier | Obligatoire |
|------|---------|-------------|
| Unitaire Go | `*_test.go` | ✅ Oui |
| Composant React | `*.test.jsx` | ⚠️ Si applicable |
| E2E Chrome | `tests/e2e/*.md` | ✅ Oui pour features |

**Rappel :**
- Tu ÉCRIS les tests, tu ne les EXÉCUTES PAS
- L'agent QA exécutera les tests ensuite
- Les tests E2E utilisent Chrome via MCP claude-in-chrome
```

## Tests E2E avec Chrome

Les scénarios E2E sont définis en Markdown et exécutés via **MCP claude-in-chrome** :

```markdown
## Scénario : Nom du scénario

### Prérequis
- Serveur démarré sur http://localhost

### Étapes
1. Ouvrir http://localhost/admin dans Chrome
2. Cliquer sur "Élément"
3. Vérifier le résultat

### Résultat attendu
- Description du comportement attendu

### Vérification Chrome (MCP)
- Attendre élément : `.selector`
- Vérifier texte : "contenu"
- Vérifier absence : `.error`
```

## Intégration dans le workflow CDP

```
PLAN → DEV → TEST-WRITER → REVIEW → QA → DOC → DEPLOY
              │
              └── Écrit les tests AVANT la review
                  pour que REVIEW vérifie aussi les tests
```

## Action immédiate

Lance maintenant le sous-agent **test-writer** avec le Task tool.
