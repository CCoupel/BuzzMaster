# Procédure de Test — Persistance des métadonnées quiz (#141)

**Version** : Milestone v6.0.x (#23), Batch 4 — non livré au moment de la rédaction
**Date** : 2026-08-11
**Testeur** : QA
**Plan** : `_work/reports/plan-20260811-105859.md` §7 Phase 5 (tâches 18-24), §9

> ⚠️ **À exécuter uniquement une fois le Batch 4 livré** (`SaveState`/`LoadState`,
> câblage démarrage, intégration backup/restore — voir plan §7 tâches 19-22). Avant
> cela, le Scénario 1 doit **échouer** : `contracts/game-state.md:424-465` documente
> qu'aujourd'hui `GameState` n'est **jamais** écrit sur disque ni relu — les
> métadonnées quiz sont perdues à chaque redémarrage. C'est précisément le
> comportement que ce chantier corrige.
>
> Les libellés d'interface exacts (nom de la case à cocher backup/restore/reset
> pour les métadonnées quiz, si une nouvelle catégorie est ajoutée aux sections
> "Sauvegarde"/"Restauration"/"Réinitialisation" de `/backup` ou si elle est
> fondue dans une catégorie existante) ne sont pas encore arrêtés au moment de la
> rédaction (dépend de la décision d'implémentation du plan tâche 22). **Adapter
> les étapes 2b/3b/3c ci-dessous au libellé réellement livré** avant de dérouler
> ces scénarios — le comportement attendu (les métadonnées survivent) ne change
> pas, seul le nom du contrôle UI est à vérifier au moment du test.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Accès admin sur `/questions` (édition des métadonnées quiz) et `/backup`
      (sauvegarde/restauration)
- [ ] Un jeu de questions déjà chargé (peu importe le contenu — ce test ne porte
      pas sur les questions elles-mêmes)
- [ ] Redémarrage serveur possible : `curl -s http://localhost/shutdown && sleep 2 && ./server.exe`
      (méthode obligatoire du projet, voir `CLAUDE.md`) ou équivalent QUALIF (Docker restart)
- [ ] Pour le Scénario 3 : un moyen de télécharger le fichier `.tar` généré par
      "Sauvegarder" et de le ré-uploader ensuite via "Sélectionner un fichier"
      (Restauration)

---

## Scénario 1 — Un redémarrage serveur préserve les métadonnées quiz

**Objectif** : Vérifier que `QUIZ_NAME`, `QUIZ_THEME`, `QUIZ_NOTES`, les publics
(`QUIZ_POPULATIONS`), les difficultés (`QUIZ_DIFFICULTIES`), la langue
(`QUIZ_LANGUAGE`), l'objectif (`QUIZ_OBJECTIVES`) et les champs masqués à la TV
(`QUIZ_HIDDEN_FIELDS`) survivent à un redémarrage complet du process serveur —
et pas seulement à un rechargement de page (ceux-ci vivaient déjà en mémoire
serveur avant #141 ; ce qui est nouveau est la survie au redémarrage du
*process*).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller sur `/questions`, dérouler la section quiz (thème, publics, difficultés, langue, objectif) | Section visible avec les champs vides ou par défaut (base propre) | | |
| 2 | Renseigner : Thème = `Cinéma des années 80`, cocher un public (ex. `Adulte (18-64 ans)`), une difficulté (ex. `Moyen`), Langue = `Français`, Objectif = `Soirée test #141` | Les champs affichent bien les valeurs saisies, la sauvegarde se déclenche (auto-save, pas de bouton dédié — voir `QuestionsPage.jsx`) | | |
| 3 | Dans la section "Champs visibles à la TV", décocher un champ (ex. `DIFFICULTIES`) pour peupler `QUIZ_HIDDEN_FIELDS` | La case se décoche, l'état est enregistré | | |
| 4 | Recharger la page (F5) — **sans redémarrer le serveur** | Toutes les valeurs saisies à l'étape 2-3 sont toujours affichées (persistance mémoire, déjà acquise avant #141 — sert de témoin) | | |
| 5 | Redémarrer le **process** serveur : `curl -s http://localhost/shutdown && sleep 2 && ./server.exe` (ou équivalent QUALIF) | Le serveur redémarre proprement, sans erreur dans les logs au chargement de `game_state.json` (ou nom de fichier équivalent livré, voir plan tâche 19-20) | | |
| 6 | Retourner sur `/questions` après le redémarrage | Thème = `Cinéma des années 80`, public `Adulte (18-64 ans)` coché, difficulté `Moyen` cochée, Langue = `Français`, Objectif = `Soirée test #141` — **toutes les valeurs de l'étape 2 sont revenues** | | |
| 7 | Vérifier la section "Champs visibles à la TV" | Le champ décoché à l'étape 3 (`DIFFICULTIES`) est toujours décoché après redémarrage | | |
| 8 | Aller sur `/tv` (ou `/player`) | La difficulté n'est pas affichée (conforme au masquage de l'étape 3, qui a bien survécu) ; les autres métadonnées visibles (thème, etc.) sont correctes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — `NEW_GAME` ne réinitialise pas les métadonnées quiz (règle H5)

**Objectif** : Vérifier que la règle H5 (`contracts/game-state.md:404` :
« la valeur persiste en mémoire d'une partie à l'autre et n'est pas réinitialisée
par `NEW_GAME` ») reste vraie une fois les métadonnées rendues durables — le
chantier #141 doit uniquement ajouter la survie au redémarrage, **sans changer**
le comportement `NEW_GAME` existant.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Repartir de l'état obtenu à la fin du Scénario 1 (métadonnées renseignées, serveur déjà redémarré une fois) | Métadonnées toujours visibles sur `/questions` | | |
| 2 | Lancer une partie complète (`START`) puis la terminer, ou déclencher directement `NEW_GAME` depuis l'admin (bouton "Nouvelle partie") | La partie/le score repart à zéro (comportement normal de `NEW_GAME`) | | |
| 3 | Retourner sur `/questions` juste après `NEW_GAME` | Thème, publics, difficultés, langue, objectif et champs masqués **sont identiques à l'étape 1** — rien n'a été réinitialisé | | |
| 4 | Redémarrer le serveur une seconde fois (comme Scénario 1 étape 5) | Redémarrage propre | | |
| 5 | Retourner sur `/questions` | Les métadonnées sont **toujours** celles de l'étape 1 — `NEW_GAME` suivi d'un redémarrage ne perd rien de plus qu'un redémarrage seul | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Backup → Restore → Redémarrage

**Objectif** : Vérifier que les métadonnées quiz font partie du fichier `.tar`
produit par "Sauvegarder" (`/backup`), et qu'une restauration sur une base vierge
les remet en place, y compris après un redémarrage qui suit la restauration
(plan tâche 22 : intégration au backup/restore/reset).

### 3a — Sauvegarde avec métadonnées connues

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/questions`, renseigner des métadonnées distinctives : Thème = `Backup Test 141`, Objectif = `Vérif restauration` | Valeurs enregistrées (auto-save) | | |
| 2 | Aller sur `/backup`, section "Sauvegarde" | Sélectionner les catégories nécessaires — **vérifier si une nouvelle catégorie dédiée aux métadonnées quiz est apparue**, sinon confirmer dans quelle catégorie existante elles sont désormais incluses (voir avertissement en tête de ce document) | | |
| 3 | Cliquer "Sauvegarder" | Un fichier `.tar` est téléchargé (`buzzcontrol-backup-<date>.tar`) | | |

### 3b — Réinitialisation puis restauration

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 4 | Sur `/backup`, section "Réinitialisation", cocher l'équivalent des métadonnées quiz (ou la catégorie qui les contient désormais) et cliquer "Réinitialiser" | Confirmation demandée puis exécutée | | |
| 5 | Retourner sur `/questions` | Thème/Objectif/etc. sont revenus à vide (état réinitialisé) — confirme que la réinitialisation couvre bien les métadonnées quiz, pas seulement les questions | | |
| 6 | Sur `/backup`, section "Restauration", sélectionner le fichier `.tar` de l'étape 3 via "Sélectionner un fichier" | Upload accepté, pas d'erreur affichée | | |
| 7 | Retourner sur `/questions` | Thème = `Backup Test 141`, Objectif = `Vérif restauration` — les métadonnées de l'étape 1 sont revenues | | |

### 3c — La restauration résiste à un redémarrage

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 8 | Redémarrer le serveur (comme Scénario 1 étape 5) | Redémarrage propre | | |
| 9 | Retourner sur `/questions` | Thème = `Backup Test 141`, Objectif = `Vérif restauration` toujours présents — la restauration a bien persisté sur disque, pas seulement en mémoire jusqu'au prochain redémarrage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : PASS — toutes les métadonnées quiz (y compris `QUIZ_HIDDEN_FIELDS`) survivent à un redémarrage process, pas seulement à un F5
- [ ] Scénario 2 : PASS — `NEW_GAME` ne touche à aucune métadonnée quiz (règle H5 inchangée), avant **et** après redémarrage
- [ ] Scénario 3 : PASS — le fichier de sauvegarde `.tar` contient les métadonnées quiz, la restauration les remet en place, et cela résiste à un redémarrage ultérieur
- [ ] Aucune régression sur les autres éléments de backup/restore/reset déjà couverts (Questions, Équipes, Joueurs, Historique, Médias) — vérifier qu'ils ne sont pas devenus accidentellement dépendants de la nouvelle catégorie
- [ ] Aucun champ éphémère (phase de jeu en cours, question affichée, scores, `ArdoiseAnswers`, `MemoryState`, `VirtualPlayerCount`, `NetworkOnlyLocalhost`) ne réapparaît après redémarrage — seules les métadonnées quiz listées en tête de ce document sont concernées (plan tâche 18, liste d'exclusion explicite)
- [ ] Le fichier persisté sur disque (ex. `data/config/game_state.json`, nom exact à confirmer selon implémentation) n'est **jamais** vide/corrompu après un arrêt brutal du serveur pendant une écriture — hors scope de cette procédure manuelle (couvert par les tests unitaires Go d'écriture atomique, motif `SaveTeams`/`SaveBumpers`), mais à signaler si observé

## Notes QA

- Ce document prépare le Batch 4 (plan §8) : il a été rédigé **avant** l'implémentation de `SaveState`/`LoadState` (dev-backend), à partir des critères d'acceptation du plan (§5 « #141 ») et de la liste de champs à persister (plan tâche 18). Si l'implémentation finale diverge du sous-ensemble de champs listé ici (ex. `VirtualPlayerLimit`/`Delay` finalement inclus ou exclus, plan tâche 18 « candidats complémentaires »), mettre à jour ce document en conséquence plutôt que de considérer un écart comme une anomalie.
- Si le format persisté inclut un champ de version (plan tâche 20, premier versionnement du projet), vérifier au passage qu'un fichier `game_state.json` absent ou vide au démarrage ne fait pas planter le serveur (démarrage à froid, première installation) — comportement attendu : démarrage avec des métadonnées vides, pas d'erreur bloquante.
- Vérifier explicitement le piège d'ordre signalé par le planner (plan tâche 21) : les arrière-plans (`Backgrounds`/`NewGameBackgrounds`) ne doivent **pas** être écrasés par le chargement de `game_state.json` — si un arrière-plan personnalisé était configuré avant le test, confirmer qu'il est toujours actif après le redémarrage du Scénario 1.

[Espace libre pour observations complémentaires]
