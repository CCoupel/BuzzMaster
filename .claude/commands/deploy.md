# Commande /deploy - Déploiement QUALIF / PROD

Lance le sous-agent DEPLOY pour déployer le serveur vers l'environnement cible.

## Argument reçu

$ARGUMENTS

**Format** : `/deploy [QUALIF|PROD|hotfix]`

- **QUALIF** (défaut) : Déploiement de qualification
- **PROD** : Déploiement de production (squash merge + tag + release)
- **hotfix** : Mode urgence pour bugs critiques

## Instructions

Utilise le Task tool pour lancer le sous-agent deploy avec les paramètres suivants :

```
subagent_type: "deploy"
description: "Déploiement [ENV]"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Déploie le serveur BuzzControl vers l'environnement cible.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json
- Procédure QUALIF : docs/QUALIF_PROCEDURE.md
- Procédure PROD : docs/RELEASE_PROCEDURE.md

**Environnement cible :** $ARGUMENTS (QUALIF par défaut si vide)

**Étapes à exécuter :**

1. **Collecter les informations**
   - Version : lire server-go/config.json → champ "version"
   - Branche : git branch --show-current
   - Dernier commit : git log -1 --oneline

2. **Vérifier les prérequis**
   - Tests QA passés
   - Review approuvée
   - Documentation à jour
   - Version incrémentée

3. **Build**
   cd server-go
   # Windows
   go build -o server.exe ./cmd/server
   # Linux ARM64 (Raspberry Pi)
   GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

4. **Tests post-build**
   - Démarrer le serveur
   - Vérifier /version et /listGame
   - Arrêt graceful via /shutdown

5. **Git (PROD uniquement)**
   - Squash merge vers main
   - Tag annotée v<version>
   - Cleanup branche feature

6. **Générer le rapport de déploiement** avec :
   - Informations : Version, env, date, branche, commit
   - Builds : Résultats + tailles binaires
   - Tests : Résultats post-build
   - Git (PROD) : Merge, tag, cleanup
   - Décision : SUCCESS / FAILED

**Différences QUALIF vs PROD :**
| Action | QUALIF | PROD |
|--------|--------|------|
| Build Windows + ARM64 | Oui | Oui |
| Tests post-build | Oui | Oui |
| Squash merge main | Non | Oui |
| Tag Git | Non | Oui |
| Cleanup branche | Non | Oui |

**Règles critiques :**
- JAMAIS déployer PROD sans QUALIF validée
- JAMAIS créer des tags Git en QUALIF
- JAMAIS merge main en QUALIF
- TOUJOURS tester l'arrêt graceful
```

## Action immédiate

Lance maintenant le sous-agent deploy avec le Task tool.
