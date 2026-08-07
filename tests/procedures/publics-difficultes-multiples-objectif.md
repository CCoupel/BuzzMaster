# Procédure de Test — Publics/difficultés multiples, objectif global, visibilité TV (#137 Batch 2b, v6.1.0)

**Version** : v6.1.0 (branche `feature/llm-gratuit-cloud`)
**Date** : 2026-08-06
**Testeur** : QA

---

## Contexte de la Feature

Les réglages globaux d'une partie (thème, publics, difficultés, langue, objectif) deviennent la
**source unique** : la section Quiz de `QuestionsPage` permet de sélectionner **plusieurs**
publics et **plusieurs** difficultés (chips, remplacent les anciens `<select>` à valeur unique),
de saisir un **objectif de partie** (texte libre, jamais diffusé aux joueurs) et de choisir, champ
par champ, ce qui est annoncé sur l'écran TV NEW_GAME (interrupteur `Afficher sur la TV`). Le
popup de génération IA n'affiche plus ces valeurs qu'en lecture seule (rappel + lien « modifier »).

Références :
- Contrats : `contracts/ai-generation.md` §3bis, `contracts/game-state.md` (§"Métadonnées Quiz",
  §"QUIZ_OBJECTIVES — champ à diffusion restreinte", §"QUIZ_HIDDEN_FIELDS — visibilité TV par
  champ"), `contracts/websocket-actions.md` (`UPDATE_QUIZ_META`)
- Plan : `_work/reports/planner-20260806-145021-plan-137-batch2b.md`
- Maquette : `_work/mockups/137-batch2b-globaux-multiples.html`
- Tests automatisés couvrant déjà cette feature (pour référence, pas à ré-exécuter manuellement) :
  - Backend : `server-go/internal/protocol/messages_quiz_objectives_test.go`,
    `server-go/cmd/server/vplayer_fanout_quiz_objectives_test.go`,
    `server-go/internal/server/ai_generation_test.go`
  - Frontend : `server-go/web/src/pages/QuestionsPage.quizChips.test.jsx`,
    `server-go/web/src/pages/PlayerDisplay.quizBadges.test.jsx`,
    `server-go/web/src/hooks/useWebSocket.quizMeta.test.js`

**Point à garder en tête** : `QUIZ_OBJECTIVES` est une règle de **confidentialité** (retirée du
payload serveur avant d'atteindre `/ws/tv` et `/ws/player`) — le scénario 4 le vérifie
explicitement dans l'onglet réseau, pas seulement à l'écran. `QUIZ_HIDDEN_FIELDS` est une
**préférence d'affichage** différente : la valeur masquée reste dans le payload TV, c'est le
client TV qui ne l'affiche pas — un champ masqué est donc normalement visible dans l'onglet
réseau, ce n'est pas un bug.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/llm-gratuit-cloud`)
- [ ] Accès admin (`/admin` ou `/anim`) → `QuestionsPage`
- [ ] Un écran ou une fenêtre `/tv` affichable à **1920×1080** (résolution de référence pour la
      contrainte « TV statique, aucun scroll »)
- [ ] Un client `/vplayer` accessible (peut être un second onglet du même navigateur)
- [ ] Outils réseau du navigateur (onglet Réseau / WS) sur le client `/tv` et le client `/vplayer`
- [ ] Une clé API IA configurée si le scénario 5 (génération réelle) est exécuté

---

## Scénarios

### Scénario 1 — Sélection multiple publics/difficultés + objectif, persistance après rechargement

**Objectif** : vérifier que la section Quiz permet une sélection multiple par chips et que
l'objectif est distinct du texte libre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `QuestionsPage`, section Quiz | Publics et Difficultés s'affichent en **chips** cliquables (pas un `<select>`) | | |
| 2 | Cliquer sur 2 publics (ex : « Ado (13-17 ans) » et « Adulte (18-64 ans) ») | Les 2 chips passent à l'état actif (coche visible), aucun appel réseau immédiat | | |
| 3 | Cliquer sur 2 difficultés (ex : « Moyen » et « Difficile ») | Les 2 chips passent à l'état actif | | |
| 4 | Saisir un texte dans le champ « Objectif de la partie » (ex : « Réviser le chapitre 3 ») | Le champ affiche la mention « 🔒 Non affiché aux joueurs » | | |
| 5 | Vérifier le champ « Texte libre » existant | Affiche la mention « 📺 Affiché aux joueurs » — visuellement distinct de l'Objectif | | |
| 6 | Cliquer sur « Enregistrer » | Confirmation visuelle (bouton devient « Enregistré ✓ ») | | |
| 7 | Recharger la page admin (F5) | Les 2 publics, les 2 difficultés et l'objectif sont toujours sélectionnés/affichés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Popup de génération IA : rappel lecture seule, pas de duplication de champ

**Objectif** : vérifier que le popup n'expose plus de champs dupliquant thème/publics/
difficultés/langue/objectif.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la section Quiz (état du scénario 1, enregistré), cliquer sur « ✨ Générer via IA » | Le popup s'ouvre | | |
| 2 | Observer le rappel en haut du popup | Thème / Publics / Difficultés / Langue / Objectif affichés en **lecture seule** (mini-chips pour publics/difficultés) | | |
| 3 | Chercher un champ éditable pour Thème/Publics/Difficultés/Langue/Objectif dans le popup | **Aucun** — seul un champ « Précisions pour cette génération » est éditable | | |
| 4 | Cliquer sur le lien « modifier » du rappel | Retour à la section Quiz de `QuestionsPage` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Bandeau « modifications non enregistrées » (bug de fraîcheur corrigé, T2.5)

**Objectif** : vérifier que la génération IA ne peut plus utiliser silencieusement une valeur de
formulaire non enregistrée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans la section Quiz, modifier un public (ajouter/retirer un chip) **sans cliquer sur Enregistrer** | Le chip change visuellement | | |
| 2 | Ouvrir le popup « ✨ Générer via IA » sans avoir enregistré | Un bandeau d'avertissement apparaît au-dessus du rappel (« Des modifications de la section Quiz ne sont pas enregistrées… ») | | |
| 3 | Observer les valeurs du rappel | Ce sont les valeurs **enregistrées** (avant la modification du champ), pas celles du formulaire modifié | | |
| 4 | Modifier uniquement `QUIZ_NOTES` (texte libre) sans enregistrer, rouvrir le popup | **Aucun** bandeau (ce champ n'affecte pas la génération) | | |
| 5 | Revenir à la section Quiz, cliquer « Enregistrer », rouvrir le popup | Le bandeau a disparu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Confidentialité `QUIZ_OBJECTIVES` (déjà couvert par tests automatisés — vérification bout-en-bout écran réel)

**Objectif** : confirmer dans un navigateur réel que l'objectif de la partie ne quitte jamais le
serveur vers TV/VPlayer.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Enregistrer un objectif de partie distinctif (ex : « SECRET-QA-137 ») dans la section Quiz | Enregistré | | |
| 2 | Ouvrir un client `/tv`, onglet réseau/WS du navigateur, inspecter les trames `UPDATE` reçues | Rechercher `SECRET-QA-137` et `QUIZ_OBJECTIVES` dans les trames → **absents** | | |
| 3 | Ouvrir un client `/vplayer`, même vérification | `SECRET-QA-137` / `QUIZ_OBJECTIVES` → **absents** | | |
| 4 | Ouvrir un client `/admin` (ou l'onglet déjà utilisé), même vérification | `QUIZ_OBJECTIVES` **présent** avec la valeur « SECRET-QA-137 » | | |
| 5 | Sur l'écran TV NEW_GAME, vérifier visuellement | L'objectif n'apparaît **nulle part** à l'écran | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Écran TV NEW_GAME : badges plafonnés, masquage par champ, cas plein et cas « tout masqué »

**Objectif** : vérifier le rendu réel à 1920×1080 (pas seulement en test automatisé JSX) —
contrainte projet « TV statique, aucun scroll ».

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner les **5 publics** et les **4 difficultés** disponibles + une langue, tous « Afficher sur la TV » activés, enregistrer | | | |
| 2 | Observer l'écran `/tv` à 1920×1080, phase NEW_GAME | Rangée de badges affiche **2 valeurs par famille max** + un badge `+N` groupé (`+3` publics, `+2` difficultés) — pas de retour à la ligne, pas de scroll | | |
| 3 | Désactiver l'interrupteur TV de « Difficultés » (sans changer la sélection), enregistrer | | | |
| 4 | Observer l'écran `/tv` | Les badges Difficultés (et leur éventuel `+N`) ont **disparu** ; Publics et Langue restent affichés normalement | | |
| 5 | Désactiver aussi Thème/Publics/Langue (les 4 interrupteurs éteints), enregistrer | | | |
| 6 | Observer l'écran `/tv` | **Aucune rangée de badges résiduelle** (pas de bloc vide, pas d'espace blanc anormal) — le nom du quiz (`QUIZ_NAME`, non pilotable) reste affiché s'il est renseigné | | |
| 7 | Réactiver les 4 interrupteurs, enregistrer | Retour à l'affichage du scénario 2 de cette section | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Génération réelle avec 2 publics + 2 difficultés + objectif

**Objectif** : vérifier que la génération produit des questions cohérentes avec toutes les valeurs
sélectionnées, pas seulement la première de chaque tableau.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Section Quiz : 2 publics, 2 difficultés, un objectif clair (ex : « faire marquer les plus jeunes »), enregistrer | | | |
| 2 | Ouvrir le popup IA, lancer une génération de ~10 questions | Génération en cours, puis succès | | |
| 3 | Inspecter les questions créées (catégories/difficultés) | Les 2 difficultés sélectionnées apparaissent toutes les deux dans le lot généré (pas uniquement la première) | | |
| 4 | Relire 2-3 énoncés | Cohérents avec le thème et, autant que possible, avec l'objectif annoncé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Rejet d'un client non redéployé (ancien champ `population` singulier)

**Objectif** : vérifier que le comportement `400` documenté (contrat §3bis, §3ter) est bien visible
et non silencieux — pertinent seulement si un test manuel via un client HTTP (curl/Postman) est
possible.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Envoyer `POST /api/generate-questions` avec un payload portant `"population": "Adulte (18-64 ans)"` (singulier) au lieu de `"populations"` | Réponse `400`, `code: "invalid_request"` | | |
| 2 | Vérifier qu'aucune question n'a été créée | Aucun nouveau fichier dans `data/files/questions/` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux (1, 2, 5, 6) passent
- [ ] Le bandeau de modifications non enregistrées (scénario 3) apparaît et disparaît correctement
- [ ] `QUIZ_OBJECTIVES` n'apparaît **jamais** dans le trafic réseau `/ws/tv` ni `/ws/player` (scénario 4) — criticité **haute**, à ne pas valider sur la seule foi des tests automatisés
- [ ] Aucune régression sur l'écran TV NEW_GAME existant (thème, nom du quiz, fond d'écran)
- [ ] Aucun scroll ni débordement visuel sur `/tv` à 1920×1080, y compris avec le jeu de valeurs complet (scénario 5)

## Notes QA

[Espace pour observations]
