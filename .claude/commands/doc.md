# Commande /doc - Mise à Jour de la Documentation

Lance le sous-agent DOC pour mettre à jour la documentation après validation d'une feature.

## Argument reçu (optionnel)

$ARGUMENTS

**Formats possibles** :
- `/doc` : Auto-détecte depuis git
- `/doc "description"` : Feature spécifique
- `/doc bugfix "description"` : Bugfix
- `/doc breaking "description"` : Breaking change

## Instructions

Utilise le Task tool pour lancer le sous-agent doc-updater avec les paramètres suivants :

```
subagent_type: "doc-updater"
description: "Mettre à jour documentation"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Mets à jour la documentation pour BuzzControl après validation d'une feature.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Config version : server-go/config.json
- CHANGELOG : CHANGELOG.md
- Documentation technique : CLAUDE.md
- Guide utilisateur : docs/ADMIN_GUIDE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter les informations**
   - Version actuelle : lire server-go/config.json → "version"
   - Branche : git branch --show-current
   - Commits récents : git log --oneline -20
   - Fichiers modifiés : git diff main --name-only

2. **Analyser les changements**
   - Type : Added / Changed / Fixed / Deprecated / Removed
   - Composants impactés : Backend, Frontend, Protocol, Models
   - Fichiers concernés

3. **Mettre à jour la documentation**

   | Fichier | Action |
   |---------|--------|
   | CHANGELOG.md | Entrée version format Keep a Changelog |
   | CLAUDE.md | Sections impactées (Models, Protocol, UI) |
   | docs/ADMIN_GUIDE.md | Instructions user-facing (si applicable) |
   | server-go/config.json | Finaliser version (z=0 pour features) |

4. **Format CHANGELOG.md**
   ```markdown
   ## [X.Y.Z] - YYYY-MM-DD

   ### Added
   - **[Composant]**: Description
     - Détail 1
     - Détail 2

   ### Changed
   - **[Composant]**: Ce qui a changé

   ### Fixed
   - **[Composant]**: Bug corrigé
   ```

5. **Commit et push**
   git add server-go/config.json
   git commit -m "docs(version): Finalize vX.Y.0"
   git add CHANGELOG.md CLAUDE.md docs/ADMIN_GUIDE.md
   git commit -m "docs: Update documentation for vX.Y.0"
   git push origin <branche>

6. **Générer le résumé de documentation** :
   - Fichiers mis à jour
   - Contenu ajouté (extraits)
   - Vérifications effectuées
   - Statistiques

**Règle de versionnement :**
| Type | Version finale |
|------|----------------|
| Feature (minor) | Reset z à 0 → 2.40.0 |
| Bugfix (patch) | Garder z → 2.40.1 |
| Breaking (major) | Incrémenter x → 3.0.0 |

**Checklist finale :**
- CHANGELOG.md : Format correct + date du jour
- CLAUDE.md : Sections impactées mises à jour
- ADMIN_GUIDE.md : Instructions claires (si applicable)
- config.json : Version finalisée
- Commits créés et poussés

**Règles critiques :**
- Toujours mettre à jour CHANGELOG.md
- Ne documenter que ce qui a été implémenté
- Vérifier les exemples de code
- Reset z à 0 pour les features
- Commit et push obligatoires
```

## Action immédiate

Lance maintenant le sous-agent doc-updater avec le Task tool.
