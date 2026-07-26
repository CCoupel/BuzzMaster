# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Corrections comportementales

- **GitHub Issues/PR** : ne jamais fermer/rouvrir/commenter une issue ou PR sans validation explicite de l'utilisateur — voir [feedback_github_actions.md](feedback_github_actions.md)
- **QA → DOC → QUALIF** : pas besoin de validation utilisateur entre ces phases — lancer directement — voir [feedback_qa_workflow.md](feedback_qa_workflow.md)
- **Binaire Windows obligatoire** : toujours builder le .exe Windows AVANT de demander validation QUALIF (l'utilisateur teste depuis Windows) — voir [feedback_windows_binary.md](feedback_windows_binary.md)
- **OTA firmware bootstrap** : si OTA échoue à ~20-30% sur un buzzer, flash USB avec firmware merged (0x0) — voir [project_v370_firmware_ota.md (dans /home/cyril/.claude/projects/...)]()


- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
- **Commande /start-session** : créer la TEAM **directement sans demander** confirmation ni sujet. Le nom de la TEAM est **toujours `TEAM-Buzz`**, quelle que soit la session.
- **Architecture team** : Claude principal (`main`) est l'orchestrateur — pas d'agent CDP séparé. Agents spawned à `/start-session` (IDLE), dispatched via `SendMessage` uniquement pendant la session. Rapports inter-agents vers `main`. Chemins rapports : `_work/reports/`.
- **Dispatcher test-writer en parallèle de chaque lot dev** : jamais laisser un dev écrire ses propres tests faute de handoff séparé — voir [feedback_test_writer_parallel.md](feedback_test_writer_parallel.md)
- **`gh issue create` : ne jamais passer `--body` deux fois** (écrase silencieusement en vide) — voir [feedback_gh_issue_body_flag.md](feedback_gh_issue_body_flag.md)

