# Procédure de Test — Communication Animateur (#167 + #168)

**Version** : v6.4.x (branche `feature/anim-communication`)
**Date** : 2026-08-18
**Testeur** : QA / Utilisateur
**Issues** : #167 (messagerie régie → tablettes animateur) + #168 (note d'explication par question)
**Référence** : Plan `_work/reports/plan-20260818-121500.md`, maquette
`docs/mockups/anim-communication-167-168.html`, `contracts/websocket-actions.md` §"Messagerie régie",
`contracts/models.md` §EXPLANATION

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Trois postes/onglets ouverts au minimum : **deux** `/admin/*` (deux sessions régie distinctes,
      ex. deux navigateurs ou une fenêtre privée), **une** `/anim` (tablette animateur) — idéalement
      une **seconde** `/anim` pour le Scénario 1 (acquittement croisé)
- [ ] Un quiz contenant au moins une question avec une note d'explication renseignée et une question
      sans note
- [ ] Suite automatisée exécutée et verte (voir section Non-Régression en fin de document) avant de
      démarrer la validation manuelle

---

## Scénario 1 — Deux tablettes simultanées, acquittement croisé (AC2, AC3, D3)

**Objectif** : Vérifier qu'un message envoyé par la régie atteint toutes les tablettes connectées, et
qu'un acquittement depuis N'IMPORTE LAQUELLE efface le message partout — un message unique, un
acquittement unique, jamais de comptage par tablette.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir deux tablettes `/anim` (A et B) côte à côte | Les deux affichent « Aucun message de la régie » | | |
| 2 | Depuis `/admin`, taper une consigne courte et attendre l'envoi automatique (voir Scénario 3 pour le détail des déclencheurs) | La consigne apparaît sur A **et** B, avec un bouton « Vu » sur chacune | | |
| 3 | Taper « Vu » depuis la tablette A **uniquement** | Le message disparaît immédiatement sur A **et** sur B (pas seulement A) | | |
| 4 | Observer la régie | Le bandeau passe à « Vu par l'animateur » | | |
| 5 | Répéter l'envoi, puis acquitter depuis B cette fois | Même résultat croisé : disparition sur A et B simultanément | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Mise en veille puis reconnexion d'une tablette (AC6, D6)

**Objectif** : Vérifier qu'une tablette qui se reconnecte alors qu'un message est actif le reçoit —
livraison différée, jamais perdue.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur une tablette `/anim`, couper le Wi-Fi (ou fermer l'onglet) | Déconnexion observée (indicateur de statut) | | |
| 2 | Depuis `/admin`, envoyer une consigne pendant que la tablette est hors ligne | Aucune erreur côté régie ; le bandeau régie affiche la consigne comme active | | |
| 3 | Reconnecter la tablette (Wi-Fi rétabli ou onglet rouvert sur `/anim`) | La consigne apparaît **immédiatement** à la reconnexion, sans action supplémentaire — pas d'attente d'un prochain événement de jeu | | |
| 4 | Acquitter depuis cette même tablette | Le message s'efface partout comme au Scénario 1 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Envoi automatique, 140 caractères accentués (AC1c, AC9, AC13)

**Objectif** : Vérifier les trois déclencheurs d'envoi (aucun bouton « Envoyer »), la troncature
serveur à 140 caractères sur du texte accentué, et l'affichage intégral côté tablette.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, observer le bandeau du bas de l'écran | Un champ de saisie et un compteur (140), **aucun bouton « Envoyer »** nulle part sur la page | | |
| 2 | Taper une courte consigne puis appuyer sur **Entrée** | Envoi immédiat, la tablette affiche la consigne sans délai perceptible | | |
| 3 | Effacer, taper une nouvelle consigne puis cliquer **ailleurs sur la page** (perte de focus) | Envoi déclenché par le blur, même résultat | | |
| 4 | Effacer, taper une nouvelle consigne et **ne rien faire** pendant ~2 secondes | Envoi automatique après la pause de frappe, sans Entrée ni clic | | |
| 5 | Composer un texte de plus de 140 caractères, **entièrement accentué** (é/è/à/ç, ex. copier-coller un paragraphe français riche en accents) et l'envoyer | Le texte reçu sur la tablette est tronqué à 140 caractères **exactement**, lisible, **sans caractère coupé/corrompu en fin de texte** (pas de `�` ni de glyphe cassé) | | |
| 6 | Observer la bande `/anim` avec ce message de 140 caractères | Le texte tient **en entier** dans la bande (elle s'agrandit verticalement si besoin), rien n'est coupé visuellement, le bouton « Vu » reste accessible, et le bloc « à suivre » n'est pas repoussé hors écran | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Résurrection après acquittement (AC1d — cas limite critique)

**Objectif** : Reproduire précisément le risque documenté au plan : une pause de frappe suivie d'un
blur sur un texte **identique**, après acquittement, ne doit **jamais** faire réapparaître le message.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, taper une consigne et attendre la pause de frappe (2s) pour déclencher l'envoi | La consigne est active, visible sur `/anim` | | |
| 2 | Depuis `/anim`, acquitter (« Vu ») | Le bandeau régie passe à « Vu par l'animateur » | | |
| 3 | Sans modifier le texte dans le champ régie, **cliquer ailleurs sur la page** (déclenche un blur sur le même texte, encore présent) | **Le message ne réapparaît PAS** sur les tablettes ni en régie — le bandeau régie reste sur « Vu par l'animateur » | | |
| 4 | Cliquer sur « Nouveau message », vérifier que le champ est vide | Le champ a bien été vidé à l'acquittement (pas de résidu de l'ancien texte) | | |
| 5 | Taper un texte **différent** et l'envoyer | Cette fois le message apparaît normalement sur les tablettes — seule l'identité stricte au texte acquitté était bloquée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Retrait régie (AC4, AC5, D4)

**Objectif** : Vérifier que la régie peut retirer son propre message avant tout acquittement animateur,
avec un statut distinct de l'acquittement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, envoyer une consigne (erreur de frappe, par exemple) | Message actif sur `/admin` et `/anim` | | |
| 2 | Depuis `/admin` (PAS `/anim`), cliquer sur « Effacer » | Le message disparaît immédiatement sur `/admin` **et** `/anim` | | |
| 3 | Observer l'état régie après ce retrait | Le bandeau régie repasse **directement au champ de saisie** (repos), **PAS** à « Vu par l'animateur » — distinction `CLEARED_BY` REGIE vs ANIM | | |
| 4 | Observer `/anim` après ce retrait | La bande affiche « Aucun message de la régie », aucune trace de l'ancien texte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Synchronisation multi-régie (AC1e, F3b)

**Objectif** : Vérifier que deux sessions `/admin` ouvertes simultanément restent synchronisées sur
l'état du message, sans état local optimiste divergent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir deux sessions `/admin` (poste 1 et poste 2), les placer côte à côte | Les deux affichent le champ de saisie au repos | | |
| 2 | Depuis le poste 1, taper et envoyer une consigne | Le poste 2 affiche **immédiatement** la consigne active et le bouton « Effacer », sans avoir rien tapé lui-même | | |
| 3 | Depuis le poste 2, cliquer sur « Effacer » | Le message disparaît sur les deux postes | | |
| 4 | Renvoyer une consigne depuis le poste 1, puis l'acquitter depuis une tablette `/anim` | Les deux postes régie passent à « Vu par l'animateur » simultanément | | |
| 5 | Sur le poste 2, cliquer sur « Nouveau message » PENDANT que le poste 1 est encore sur l'écran acquitté | Seul le poste 2 bascule sur le champ de saisie ; le poste 1 reste sur « Vu par l'animateur » tant qu'il n'a pas cliqué lui-même (bascule locale d'affichage, pas un état serveur partagé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Enchaînement de questions avec message actif (AC12, D5)

**Objectif** : Vérifier qu'aucune transition de jeu n'efface automatiquement un message régie non
acquitté — canal orthogonal au déroulé de la partie.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Envoyer une consigne régie, la laisser **non acquittée** | Message actif sur `/anim` | | |
| 2 | Enchaîner normalement : LANCER → STOP → RÉVÉLER → question suivante (READY) | À chaque étape, le message régie reste affiché sur `/anim`, sans interruption ni clignotement | | |
| 3 | Effectuer un RAZ (si accessible en environnement de test) | Le message régie reste actif malgré le RAZ — comportement voulu, pas une régression | | |
| 4 | Acquitter enfin le message | Efface normalement, comme dans les scénarios précédents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Emplacement du bandeau régie (AC1, AC1a, AC1b)

**Objectif** : Vérifier la portée du bandeau (présent sur tout `/admin/*`, absent ailleurs) et son
absence de recouvrement sur les pages longues.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Naviguer sur plusieurs pages `/admin/*` (jeu, équipes, questions, logs, scores) | Le bandeau régie est présent **en bas de l'écran, pleine largeur**, sur **toutes** ces pages | | |
| 2 | Naviguer sur `/anim` | Le bandeau régie (bas d'écran, saisie) est **absent** — seule la bande de réception (Scénario 1-4) est présente, à un autre endroit de la page | | |
| 3 | Naviguer sur `/tv` | Aucune trace du bandeau régie ni de la bande de réception | | |
| 4 | Naviguer sur `/player` (VJoueur) | Aucune trace non plus | | |
| 5 | Sur `/admin/quiz` (page longue, liste de questions) avec de nombreuses questions listées | Le bandeau du bas ne recouvre **aucun** contenu — le bas de la liste reste consultable en défilant | | |
| 6 | Sur `/admin/logs` (page longue) | Même vérification — aucun recouvrement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Note d'explication : édition, persistance, réédition (#168, AC14, AC15, AC20)

**Objectif** : Vérifier que la note survit à une réédition de la question (piège `handleUploadQuestion`)
et que la vider l'efface correctement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/quiz`, créer ou éditer une question, renseigner « Note d'explication (animateur seul) » | Le champ accepte un texte long, sans limite visible | | |
| 2 | Enregistrer | Question sauvegardée sans erreur | | |
| 3 | Rouvrir la MÊME question en édition | La note est toujours présente, identique | | |
| 4 | Modifier **uniquement** le texte de la question (pas la note), enregistrer | Rouvrir à nouveau : la note **est toujours là**, inchangée (c'est le piège du plan — une régression ferait disparaître la note à cette étape précisément) | | |
| 5 | Vider complètement le champ note, enregistrer, rouvrir | La note est bien effacée (champ vide, pas de résidu) | | |
| 6 | Ouvrir une question qui n'a **jamais** eu de note | Le champ note est vide, aucune erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Note d'explication sur `/anim` (#168, AC16, AC17, AC18, AC19)

**Objectif** : Vérifier l'affichage floutée/révélée par pression, l'emplacement au repos, et
l'absence totale ailleurs.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger sur `/anim` une question **avec** note, avant révélation de la réponse | La zone note (sous la zone réponse) est floutée, libellé « Note — maintenir pour lire » | | |
| 2 | Maintenir un doigt/clic sur la zone note | Le texte devient lisible tant que la pression est maintenue | | |
| 3 | Relâcher | Zone à nouveau floutée | | |
| 4 | Amener la question en RÉVÉLÉ (RÉPONSE) | La note est visible **en permanence**, sans avoir besoin de maintenir quoi que ce soit | | |
| 5 | Charger une question **sans** note | L'emplacement affiche « Aucune note pour cette question » — jamais un blanc | | |
| 6 | Charger une question avec une note **très longue** (plusieurs paragraphes) | Le bloc central défile pour l'afficher, **sans** déplacer le reste de la mise en page (méta, chrono, bouton « à suivre ») | | |
| 7 | Observer `/tv`, `/player` et `/admin` pendant qu'une question avec note est active | La note n'apparaît **nulle part** sur ces trois surfaces | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Acquittement croisé entre deux tablettes fonctionne, message unique (Scénario 1)
- [ ] Livraison différée à la reconnexion d'une tablette (Scénario 2)
- [ ] Trois déclencheurs d'envoi actifs, aucun bouton « Envoyer », troncature 140 runes propre sur
      texte accentué, affichage intégral sur la bande `/anim` (Scénario 3)
- [ ] **Aucune résurrection** d'un message acquitté sur un renvoi automatique du même texte (Scénario 4 —
      critère de non-régression le plus sensible du lot)
- [ ] Retrait régie distinct de l'acquittement animateur (`CLEARED_BY`) (Scénario 5)
- [ ] Deux sessions régie restent synchronisées sur l'état serveur, jamais sur un état local optimiste
      (Scénario 6)
- [ ] Aucune transition de jeu n'efface un message régie non acquitté (Scénario 7)
- [ ] Bandeau régie présent sur tout `/admin/*`, absent de `/anim`/`/tv`/`/player`, aucun recouvrement
      de contenu (Scénario 8)
- [ ] Note d'explication : survit à une réédition de la question, effacée si vidée explicitement
      (Scénario 9)
- [ ] Note d'explication : geste identique à la réponse, jamais rendue hors `/anim` (Scénario 10)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./...` | Build OK, tous les tests PASS, y compris `regie_message_test.go` (T2-T4b, NOUVEAU), `inbound_allowlist_anim_test.go` (T1), `broadcast_anim_test.go`/`send_state_to_client_anim_test.go` (T5/T6), `messages_anim_test.go` (T7), `http_test.go` (T8) | | |
| 2 | `cd server-go/web && npm test` (suite Vitest complète) | Tous les tests PASS, y compris `RegieMessageBar.test.jsx`, `AnimExplanationNote.test.jsx` (NOUVEAUX), `QuestionsPage.explanation.test.jsx`, `PlayerDisplay.explanation.test.jsx` (NOUVEAUX), `AnimPage.test.jsx` et `AnimConductPanel.test.jsx` (blocs réécrits) | | |
| 3 | `AnimAnswerZone.test.jsx` passe **sans la moindre modification** | Preuve que l'extraction du geste de révélation en `useHoldToPeek` (F6) n'a rien changé au comportement #169 | | |
| 4 | Manche QCM/SPEEDY/ARDOISE/MEMORY/MEMOTION sur `/anim` (hors #167/#168) | Aucune régression : conduite, crédit, colonne équipes inchangés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
