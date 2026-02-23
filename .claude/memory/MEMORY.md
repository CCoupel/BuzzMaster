# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/session` pour démarrer chaque session : sync MEMORY + création optionnelle de la TEAM.
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Corrections comportementales

- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Team par feature/bugfix** : créer une nouvelle team à chaque nouvelle demande, pas réutiliser
- **Agents** : prompts génériques uniquement (rôle, pas tâche) — tâches via TaskCreate + TaskUpdate
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
- **Création team** : lire `cdp.md` pour la liste complète, puis spawner TOUS les teammates avec instruction d'attendre les ordres. Liste complète (9 agents) : planner (`implementation-planner`), dev-backend, dev-frontend, buzzclick-dev (`dev-buzzclick`), test-writer, code-reviewer, QA, doc-updater, deployer (`deploy`)
- **Commande /session** : créer la TEAM **directement sans demander** confirmation ni sujet. Le nom de la TEAM est **toujours `myTEAM`**, quelle que soit la session.

# currentDate
Today's date is 2026-02-23.
