# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Procédure de début de session

1. Lire `.claude/memory/MEMORY.md` dans le projet
2. Comparer avec ce fichier privé — afficher les différences à l'utilisateur
3. Demander quelle version fait foi et synchroniser les deux
4. Demander à l'utilisateur s'il faut créer une team avant de commencer tout travail

## Corrections comportementales

- **Début de session** : suivre la procédure ci-dessus avant tout travail
- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Team par feature/bugfix** : créer une nouvelle team à chaque nouvelle demande, pas réutiliser
- **Agents** : prompts génériques uniquement (rôle, pas tâche) — tâches via TaskCreate + TaskUpdate
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
