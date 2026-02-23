# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Corrections comportementales

- **Début de session** : demander à l'utilisateur s'il faut créer une team avant de commencer tout travail
- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Team par feature/bugfix** : créer une nouvelle team à chaque nouvelle demande, pas réutiliser
- **Agents** : prompts génériques uniquement (rôle, pas tâche) — tâches via TaskCreate + TaskUpdate
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
