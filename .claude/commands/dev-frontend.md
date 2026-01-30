# Commande /dev-frontend - Implémentation Frontend React

Lance le sous-agent dev-frontend pour implémenter du code React selon un plan.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/dev-frontend [plan]` : Plan d'implémentation frontend
- `/dev-frontend fix "description"` : Correction de bug frontend
- `/dev-frontend review "corrections"` : Corrections post-review

## Instructions

Utilise le Task tool pour lancer le sous-agent dev-frontend avec les paramètres suivants :

```
subagent_type: "dev-frontend"
description: "Implémenter frontend React"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Implémente le code frontend React pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Frontend React : server-go/web/src/
- Config version : server-go/config.json
- Procédure : docs/DEV_PROCEDURE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter le contexte**
   - Version actuelle : server-go/config.json → "version"
   - Branche : git branch --show-current
   - Si DEV-BACKEND a terminé, lire son résumé pour :
     - Nouvelles actions WebSocket à gérer
     - Nouveaux champs GameState à afficher

2. **Incrémenter la version (OBLIGATOIRE)**
   - AVANT tout code, incrémenter z : 2.40.1 → 2.40.2
   - Commit : chore(version): Bump to X.Y.Z

3. **Implémenter selon l'ordre frontend**
   | Étape | Fichier | Actions |
   |-------|---------|---------|
   | 1 | hooks/useWebSocket.js | Nouveaux handlers |
   | 2 | components/*.jsx | Composants réutilisables |
   | 3 | pages/*Page.jsx | Pages admin |
   | 4 | pages/PlayerDisplay.jsx | Affichage TV (STATIQUE!) |
   | 5 | pages/*.css | Styles |

4. **Standards React**
   - Composants fonctionnels + hooks
   - useMemo/useCallback pour optimisation
   - CSS variables (pas de hardcoded)
   - PropTypes si nécessaire

5. **CONTRAINTE CRITIQUE : TV STATIQUE**
   - PlayerDisplay.jsx : JAMAIS de scroll
   - overflow: hidden (pas auto/scroll)
   - Unités viewport (vh, vw, %)
   - Tester à 1920x1080

6. **Vérifications finales**
   - Test visuel dans navigateur
   - Test responsive
   - Test TV à 1920x1080
   - git push origin <branche>

7. **Générer le résumé** :
   - Fichiers modifiés avec changements
   - Changements UI (Admin / TV)
   - Vérifications visuelles effectuées
   - Commits créés

**Règles critiques :**
- Frontend UNIQUEMENT : Ne pas toucher aux fichiers Go
- Version first : Incrémenter z AVANT tout code
- TV STATIQUE : Jamais de scroll sur PlayerDisplay
- CSS variables : Utiliser les design tokens
- Commits atomiques : 1 commit par tâche logique
```

## Action immédiate

Lance maintenant le sous-agent dev-frontend avec le Task tool.
