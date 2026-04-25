# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Corrections comportementales

- **GitHub Issues/PR** : ne jamais fermer/rouvrir/commenter une issue ou PR sans validation explicite de l'utilisateur — voir [feedback_github_actions.md](feedback_github_actions.md)
- **Crash agents tmux** : CDP et backend-dev crashent (bun) après ~4-5MB contexte — re-spawner entre versions, garder messages courts — voir [feedback_agent_context_crash.md](feedback_agent_context_crash.md)
- **QA → DOC → QUALIF** : pas besoin de validation utilisateur entre ces phases — lancer directement — voir [feedback_qa_workflow.md](feedback_qa_workflow.md)
- **Binaire Windows obligatoire** : toujours builder le .exe Windows AVANT de demander validation QUALIF (l'utilisateur teste depuis Windows) — voir [feedback_windows_binary.md](feedback_windows_binary.md)


- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Team par feature/bugfix** : créer une nouvelle team à chaque nouvelle demande, pas réutiliser
- **Agents** : prompts génériques uniquement (rôle, pas tâche) — tâches via TaskCreate + TaskUpdate
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
- **Création team** : Liste complète (10 agents) : cdp (`cdp`) = **team leader**, planner (`implementation-planner`), dev-backend, dev-frontend, buzzclick-dev (`dev-buzzclick`), test-writer, code-reviewer, QA, doc-updater, deployer (`deploy`)
- **Commande /start-session** : créer la TEAM **directement sans demander** confirmation ni sujet. Le nom de la TEAM est **toujours `TEAM-Buzz`**, quelle que soit la session.
- **Architecture team** : le CDP est le team leader. Claude est l'interface utilisateur. Pour `/feature`, `/bugfix`, `/hotfix`, `/refactor` : Claude transmet au CDP via `SendMessage` puis relaie les retours. Claude ne coordonne pas les agents directement.

# currentDate
Today's date is 2026-03-01.
