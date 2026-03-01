# Commande /end-session - Fin de Session

Commande de clôture de session : archive, communique, nettoie et shutdown la team.

> **Périmètre** : Cette commande couvre uniquement ce que les workflows `/feature`, `/bugfix`, `/hotfix`
> **ne font pas** : archivage backlog, merge main, marketing, MEMORY et shutdown team.
> La documentation est déjà produite par ces workflows — elle n'est relancée que si absente.

## Argument reçu (optionnel)

$ARGUMENTS

---

## Instructions

Exécute les phases suivantes **dans l'ordre**, en t'arrêtant aux points de validation utilisateur.

---

### Phase 1 — Bilan de session

**Lire** l'état actuel :
1. `backlog/En-Cours/` → identifier la feature active
2. `git log --oneline -10` → détecter les commits récents (doc déjà faite ?)
3. `server-go/config.json` → version actuelle
4. `git status` → commits non pushés

**Détecter si la documentation a déjà été produite** :
- Chercher dans les 10 derniers commits un commit de type `docs:` ou `chore:` récent
- Vérifier que `CHANGELOG.md` mentionne la version actuelle de `config.json`

**Afficher le bilan** :

```
## Bilan de session

**Feature active** : [nom backlog/En-Cours/ ou "aucune"]
**Branche** : [nom branche actuelle]
**Version** : [X.Y.Z depuis config.json]
**Commits non pushés** : [N ou aucun]
**Documentation** : [✅ déjà produite par le workflow | ⚠️ absente — sera relancée]

### Livrables détectés
- [fichiers modifiés significatifs depuis git]

Continuer la clôture ? (oui / préciser si incomplet)
```

⏸️ **ATTENDRE CONFIRMATION UTILISATEUR** avant de continuer.

---

### Phase 2 — Documentation (CONDITIONNELLE)

**Condition** : Lancer uniquement si la documentation N'a PAS été produite par le workflow courant
(pas de commit docs récent, ou CHANGELOG.md ne mentionne pas la version actuelle).

Si absente → Lancer le sous-agent **doc-updater** :

```
subagent_type: "doc-updater"
description: "Mettre à jour documentation fin de session"
prompt:
  Mets à jour la documentation pour BuzzControl.
  Contexte projet : voir CLAUDE.md
  Auto-détecte les changements depuis git (git log et git diff).
  Fichiers à mettre à jour selon les changements :
  - CHANGELOG.md (format Keep a Changelog)
  - CLAUDE.md (sections architecture impactées)
  - docs/ADMIN_GUIDE.md (si nouvelles fonctionnalités utilisateur)
  - server-go/config.json (finaliser version si draft)
  Versionnement : Feature → Y, Bugfix → Z, Breaking → X.
```

Si déjà produite → Afficher "Documentation déjà à jour (produite par le workflow)" et passer à la suite.

---

### Phase 3 — Archivage backlog

Si un fichier existe dans `backlog/En-Cours/` :

1. Lire le fichier pour identifier la feature et la version cible
2. Déplacer le fichier : `backlog/En-Cours/<nom>.md` → `backlog/DONE/<nom>.md`
3. Modifier le statut dans le fichier déplacé :
   ```markdown
   **Statut** : ✅ DONE — vX.Y.Z
   ```
4. Mettre à jour `backlog/README.md` : retirer de la section En-Cours, ajouter en DONE avec version
5. Confirmer : "Feature `<nom>` archivée dans `backlog/DONE/`"

Si **aucune** feature En-Cours → passer à la phase suivante sans message d'erreur.

---

### Phase 4 — Merge vers main

Si on n'est **pas** déjà sur `main` :

1. Afficher le plan :
   ```
   Branche actuelle : feature/xxx (ou bugfix/xxx)
   Action : squash merge vers main, puis push

   Confirmer le merge ? (oui / non)
   ```

2. ⏸️ **ATTENDRE CONFIRMATION UTILISATEUR**

3. Si confirmé →
   - **Mode TEAM** (myTEAM actif) :
     ```
     SendMessage(recipient: "cdp", content: "Squash merge vers main. Message: feat/fix: <titre>. Version: vX.Y.Z", summary: "Merge: main")
     ```
   - **Mode SOLO** : Lancer le sous-agent **git-squash-merge** :
     ```
     subagent_type: "git-squash-merge"
     description: "Squash merge vers main"
     prompt:
       Committe les changements en cours, pousse la branche, puis squash-merge dans main.
       Message de commit squash : "feat: <titre feature depuis backlog/DONE>" (ou "fix:" si bugfix)
       Version : vX.Y.Z (depuis config.json)
     ```

Si déjà sur `main` → commit + push des changements de documentation/backlog non encore commités.

---

### Phase 5 — Site marketing

- **Mode TEAM** (myTEAM actif) :
  ```
  SendMessage(recipient: "cdp", content: "Communication marketing demandée pour la release vX.Y.Z (auto-détectée depuis config.json)", summary: "Marketing: release")
  ```
- **Mode SOLO** : Lancer le sous-agent **marketing-release** :
  ```
  subagent_type: "marketing-release"
  description: "Mise à jour site marketing"
  prompt:
    Crée les contenus de communication pour la release BuzzControl.
    Auto-détecte la version depuis server-go/config.json.
    Livrables :
    - Mise à jour du site marketing (MARKETING/)
    - Release notes : releases/vX.Y.Z/
    - Posts réseaux sociaux
    - Newsletter si version majeure (Y incrémenté)
    Ton selon type de version :
    - Major (x.0.0) : Très enthousiaste
    - Minor (x.y.0) : Modéré
    - Patch (x.y.z) : Calme, rassurant
  ```

Attendre la complétion.

---

### Phase 6 — Mise à jour MEMORY projet

Mettre à jour `.claude/memory/MEMORY.md` avec les apprentissages de la session :

1. Lire `.claude/memory/MEMORY.md`
2. Identifier les informations nouvelles à mémoriser :
   - Patterns ou conventions confirmés durant la session
   - Décisions d'architecture prises
   - Problèmes rencontrés et solutions trouvées
   - Préférences utilisateur exprimées
3. Mettre à jour via Edit (ne pas dupliquer les entrées existantes)
4. Confirmer : "MEMORY mise à jour"

---

### Phase 7 — Shutdown team

1. Envoyer `shutdown_request` à chaque agent actif dans la team `myTEAM` :
   - `cdp`, `planner`, `backend-dev`, `frontend-dev`, `buzzclick-dev`
   - `test-writer`, `code-reviewer`, `qa`, `doc-updater`, `deployer`

2. Attendre les `shutdown_response` (approve)

3. Appeler **TeamDelete** pour supprimer la team `myTEAM`

4. Confirmer : "Team myTEAM dissoute."

---

### Phase 8 — Résumé final

```markdown
## Session terminée

**Feature** : <nom>
**Version** : vX.Y.Z
**Branche** : mergée sur main

### Livrables
- Code : X fichiers modifiés
- Documentation : [déjà produite par workflow | produite en Phase 2]
- Backlog : <nom> → DONE
- Marketing : site mis à jour, release notes créées
- MEMORY : mise à jour

### Prochaine étape
- Valider en QUALIF : `/deploy QUALIF`
- Ou déployer directement : `/deploy PROD`
```

---

## Règles

| Règle | Détail |
|-------|--------|
| **Phase 1 toujours en premier** | Bilan avant toute action |
| **Phase 2 conditionnelle** | Skip si doc déjà produite par feature/bugfix/hotfix |
| **Phase 3 après Phase 2** | Version finale doit être dans config.json |
| **Phases 5 et 6 parallélisables** | Peuvent tourner en parallèle après le merge (Phase 4) |
| **Phase 7 en dernier** | Jamais avant que marketing et MEMORY soient terminés |
| **Pas de feature En-Cours** | Sauter Phase 3 silencieusement |
| **Déjà sur main** | Sauter la confirmation de merge en Phase 4 |
