# Commande /review

Lancer une revue de code manuellement.

## Usage

```
/review [scope]
```

## Scopes

| Scope | Description |
|-------|-------------|
| (vide) | Revue des changements non commites |
| `staged` | Revue des fichiers stages uniquement |
| `branch` | Revue de toute la branche vs main |
| `commit <sha>` | Revue d'un commit specifique |
| `file <path>` | Revue d'un fichier specifique |

## Exemples

```
/review                    # Changements en cours
/review staged             # Fichiers stages
/review branch             # Toute la branche
/review commit abc123      # Commit specifique
/review file src/api.ts    # Fichier specifique
```

## Rapport

Le rapport inclut :
- Problemes critiques (bloquants)
- Problemes majeurs (a corriger)
- Suggestions mineures (optionnel)
- Points positifs

## Agent

Lance l'agent `code-reviewer` defini dans `.claude/agents/code-reviewer.template.md`
