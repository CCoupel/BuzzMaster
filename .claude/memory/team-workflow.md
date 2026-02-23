# Team Workflow — BuzzControl

## Principe général

Claude = Team Leader direct. Pas d'intermédiaire CDP.
Communication : utilisateur ↔ Claude ↔ agents spécialisés.

## Cycle complet

```
TeamCreate()
    │
    ├── Task(dev-backend)       prompts génériques
    ├── Task(dev-frontend)      (rôle, pas tâche)
    ├── Task(dev-buzzclick)
    ├── Task(test-writer)
    ├── Task(code-reviewer)
    ├── Task(QA)
    └── Task(doc-updater)
         │
    TaskCreate (tâches détaillées)
    TaskUpdate (owner: "agent-name")
         │
    [dev → review → QA → doc → deploy]
         │
    Cycles correction (max 3) via SendMessage
         │
    PROD validé → proposer TeamDelete à l'utilisateur
```

## Agents disponibles

| Agent | Domaine |
|-------|---------|
| `implementation-planner` | Plan d'implémentation |
| `dev-backend` | Go serveur (engine, protocol, HTTP, WS) |
| `dev-frontend` | React/JSX (interface web admin + TV) |
| `dev-buzzclick` | ESP32-C3 / PlatformIO (firmware buzzers) |
| `test-writer` | Écriture tests unitaires + E2E |
| `code-reviewer` | Review qualité/sécurité |
| `QA` | Exécution tests, rapport verdict |
| `doc-updater` | CHANGELOG.md, CLAUDE.md, config.json |
| `deploy` | QUALIF / PROD |

## Règles de création des agents

**OBLIGATOIRE : prompts génériques uniquement**

```javascript
// ✅ CORRECT
Task({
  subagent_type: "dev-backend",
  name: "backend-dev",
  prompt: "Tu es l'agent backend Go. Attends les tâches assignées via TaskUpdate."
})

// ❌ INTERDIT : tâche spécifique dans le prompt
Task({
  subagent_type: "dev-backend",
  prompt: "Implémente le bugfix WiFi selon les instructions..."
})
```

## Cycle de correction intra-feature

Au sein d'une même feature, les cycles review/QA → correction utilisent
`SendMessage` vers l'agent existant. **Jamais un nouvel agent.**

```javascript
// ✅ UN seul agent pour tous les cycles
Task({ name: "buzzclick-dev", ... })   // créé une fois

SendMessage({ recipient: "buzzclick-dev", content: "Corrections review..." })
SendMessage({ recipient: "buzzclick-dev", content: "Fix test QA..." })

// ❌ INTERDIT
Task({ name: "buzzclick-fix-ws", ... })    // cycle 2
Task({ name: "buzzclick-fix-frag", ... })  // cycle 3
```

## Règle d'or

> Nouvelle feature/bugfix → nouvelle team.
> Nouvelle correction dans la même feature → SendMessage.

## Cycle de vie d'une team

- **Créée** : au démarrage de chaque feature/bugfix
- **Suppression proposée** : après PROD validé
  → "La feature est en prod, voulez-vous supprimer la team ?"
- **Supprimée** : uniquement sur confirmation explicite
- ❌ Jamais TeamDelete sans confirmation utilisateur
