# Procédure de Test — Mode ENTRACTE (#119)

**Version** : milestone v6.5.2
**Date** : 2026-08-20 (mise à jour — delta C1-C4)
**Testeur** : Utilisateur (les agents n'ont pas de navigateur fiable — voir
`feedback_manual_qa_is_user_role.md`)
**Issue** : #119 — pause globale déclenchée depuis la Navbar, indépendante du cycle de question
**Référence** : Plan initial `_work/reports/plan-entracte-119-20260820-140825.md`, delta de
correction `_work/reports/plan-entracte-119-fixes-20260820-155123.md` (C1-C4, retours post-QUALIF),
contrats `contracts/game-state.md` §ENTRACTE, `contracts/websocket-actions.md` §ENTRACTE_SET/
§UPDATE_ENTRACTE_CONFIG

> **Ce qui a changé depuis la première version de cette procédure** :
> - Le bouton `ENTRACTE` / `FIN D'ENTRACTE` a déménagé de l'écran Jeu vers la **barre de
>   navigation** (Navbar) — visible sur **toutes** les pages admin, pas seulement `/admin`.
> - La configuration du panneau (titre, sous-titre, image, taille, animation, transition) a
>   déménagé de `/settings` (Config) vers la **page Quiz** (`/admin/quiz`) — c'est désormais une
>   propriété de la partie, pas un réglage serveur.
> - Modifier la configuration pendant une pause en cours **ne change plus le panneau affiché en
>   direct** : l'enregistrement réussit toujours, mais ne prend effet qu'au **prochain**
>   déclenchement (voir Scénario 8, entièrement réécrit).
> - Une **transition progressive** (~2 secondes par défaut) est apparue à l'entrée et à la sortie
>   de l'entracte (voir Scénario 14, nouveau).

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] 4 postes/onglets ouverts : `/admin`, `/anim`, `/tv`, `/player` (VJoueur enrôlé avec au
      moins un bumper physique connecté)
- [ ] Au moins 2 équipes actives, un bumper physique par équipe (pour vérifier les LEDs)
- [ ] Un quiz avec au moins une question chargeable (pour vérifier la non-régression et le
      blocage du lancement)
- [ ] Accès à la page **Quiz** (`/admin/quiz`) pour la section Entracte (titre, sous-titre,
      image, taille du panneau, animation, transition) — **plus dans `/settings`**
- [ ] Accès physique aux buzzers pour vérifier visuellement l'extinction des LEDs

---

## Scénario 1 — Bouton ENTRACTE grisé pendant une question en cours

**Objectif** : Vérifier que l'entracte ne peut pas interrompre une manche (D4), et que le
bouton — désormais dans la Navbar — répond à la même règle quelle que soit la page admin
affichée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question, LANCER (phase STARTED) | Bouton `ENTRACTE`, dans la barre de navigation (entre le badge de version et le groupe « Jeu »), grisé et non cliquable | | |
| 2 | Mettre en PAUSE | Bouton toujours grisé | | |
| 3 | Naviguer vers la page Quiz ou Équipes pendant que la question tourne | Le bouton reste visible ET grisé dans la Navbar sur ces pages aussi | | |
| 4 | Cliquer malgré tout sur le bouton grisé | Aucun effet — pas de panneau, pas de filtre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Déclenchement de l'entracte : les 4 surfaces s'estompent ensemble

**Objectif** : Vérifier l'effet visuel synchronisé sur TV, VJoueur, admin et anim.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Arrêter/révéler la question en cours (phase STOPPED ou REVEALED) | Bouton `ENTRACTE` (Navbar) redevient actif | | |
| 2 | Cliquer sur `ENTRACTE` | Le bouton passe à `FIN D'ENTRACTE` (rouge/actif), le reste de `/admin` s'assombrit progressivement (grisé/estompé), le bouton lui-même reste NET et cliquable | | |
| 3 | Observer `/tv` | Contenu existant visible mais estompé (gris/sombre), panneau ENTRACTE centré affiché par-dessus (titre + sous-titre) | | |
| 4 | Observer `/player` (VJoueur) | Même estompage, même panneau centré, proportions identiques à la TV, cadenas 🔒 net au-dessus de la zone de buzz | | |
| 5 | Observer `/anim` | Interface estompée, indicateur « ⏸ Entracte en cours — contrôle réservé à l'admin » net, aucun bouton de contrôle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Bouton visible sur toutes les pages admin (C2, portée élargie)

**Objectif** : Vérifier que le déménagement du bouton dans la Navbar l'expose bien partout,
et que son état actif reste évident même sur des pages qui ne s'estompent pas.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, naviguer vers Quiz, Équipes, Scores, Palmarès, Historique | Le bouton `ENTRACTE` est visible dans la Navbar sur CHACUNE de ces pages | | |
| 2 | Déclencher l'entracte depuis la page Quiz (pas depuis l'écran Jeu) | Fonctionne — panneau/filtre apparaissent sur TV/VJoueur/anim comme au Scénario 2 | | |
| 3 | Observer la page Quiz elle-même pendant l'entracte | Le contenu de la page N'EST PAS estompé (seuls `/admin` [écran Jeu] et `/anim` le sont) — mais le bouton `FIN D'ENTRACTE` reste visuellement net et contrasté (rouge) | | |
| 4 | Terminer l'entracte depuis cette même page | Fonctionne, tout redevient normal | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Extinction physique des LEDs des buzzers

**Objectif** : Vérifier B5 — les LEDs des buzzers physiques s'éteignent réellement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec l'entracte déjà actif, observer physiquement chaque buzzer | Toutes les LEDs sont ÉTEINTES (aucune couleur, aucun clignotement) | | |
| 2 | Attendre quelques secondes | Les LEDs restent éteintes en continu (pas de réveil intempestif) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Appui buzzer physique inerte pendant l'entracte

**Objectif** : Non-régression — le buzz physique reste sans effet (garde de phase déjà en place).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, appuyer physiquement sur un buzzer | Aucun effet visible sur `/admin`, `/anim`, `/tv` — LED toujours éteinte | | |
| 2 | Répéter sur 2-3 buzzers différents | Même résultat pour chacun | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Boutons estompés d'admin et d'anim réellement bloqués (le cœur de #119)

**Objectif** : Vérifier que l'estompage n'est qu'un signal visuel — la vraie protection est
côté serveur (D6). C'est le scénario le plus important de cette procédure.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, cliquer malgré l'estompage sur LANCER / STOP / RAZ / crédit de points, etc. | Aucun effet — aucune question ne démarre, aucun score ne change | | |
| 2 | Sur `/anim`, cliquer sur les zones de conduite estompées (démarrer, révéler, créditer) | Aucun effet | | |
| 3 | Sur `/admin`, tenter de sélectionner une nouvelle question | Aucun changement de sélection | | |
| 4 | Vérifier les logs serveur (`/admin/logs` ou équivalent) | Entrées `WARN` mentionnant les actions refusées « blocked during ENTRACTE » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Rechargement du VJoueur et nouvel écran TV pendant l'entracte

**Objectif** : Vérifier que la reconnexion pendant la pause revient directement en entracte
(D6 : HELLO/PLAYER_CONNECT restent autorisés), et qu'un client qui se connecte pendant la
pause reçoit immédiatement le bon état (D2).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, recharger la page `/player` (F5) | La page revient directement avec le filtre et le panneau ENTRACTE affichés, sans flash de l'écran de jeu normal | | |
| 2 | Vérifier l'identité du joueur (nom, équipe) | Conservée, pas de nouvelle inscription | | |
| 3 | Ouvrir un NOUVEL onglet/écran sur `/tv` | Panneau et filtre affichés IMMÉDIATEMENT, avec le bon titre/sous-titre (pas de panneau vide ni de délai visible) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Configuration figée à l'activation (C4, réécrit — remplace l'ancien Scénario 8)

**Objectif** : Vérifier l'arbitrage C4 : enregistrer les réglages pendant une pause en cours
**réussit toujours**, mais **ne change pas** le panneau déjà affiché — l'effet est reporté au
**prochain** entracte.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif (panneau visible sur TV/VJoueur avec le titre courant) | Noter le titre affiché | | |
| 2 | Ouvrir la page Quiz (`/admin/quiz`), section Entracte, modifier le titre et cliquer Enregistrer | Confirmation de sauvegarde ; une mention « Prendra effet au prochain entracte » est affichée à côté du bouton | | |
| 3 | Observer `/tv` et `/player` (déjà ouverts, toujours en pause) | Le titre affiché sur le panneau **N'A PAS CHANGÉ** — toujours l'ancien titre | | |
| 4 | Quitter la page Quiz, y revenir | Le formulaire affiche bien le **nouveau** titre enregistré (pas l'ancien) — l'enregistrement n'est pas perdu | | |
| 5 | Cliquer sur `FIN D'ENTRACTE` puis relancer un entracte | Le panneau affiche maintenant le **nouveau** titre sur les 4 surfaces | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Modification hors entracte : effet immédiat (non-régression du chemin normal)

**Objectif** : Vérifier que le gel (Scénario 8) est spécifique à une pause active — hors
entracte, enregistrer met à jour la configuration normalement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Hors entracte, page Quiz, modifier le sous-titre et Enregistrer | Confirmation de sauvegarde, **aucune** mention « Prendra effet au prochain entracte » (elle n'apparaît que pendant une pause active) | | |
| 2 | Déclencher un entracte | Le panneau affiche immédiatement le nouveau sous-titre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Aucun élément flottant ne se déplace (piège `filter`/`position:fixed`)

**Objectif** : Vérifier D6/F4/F5 — QR code, bandeau régie, navbar restent bien positionnés.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, phase ENROLL sur un autre cycle (ou juste après sortie), observer le QR code sur `/tv` si affiché | Position et taille normales, pas de décalage ni de passage derrière le panneau | | |
| 2 | Sur `/admin`, envoyer un message régie pendant l'entracte (`REGIE_MESSAGE_SEND` reste autorisé, D6) | Le bandeau régie reste ancré en bas d'écran sur `/anim`, non estompé, non décalé | | |
| 3 | Ouvrir le menu ☰ de la Navbar pendant l'entracte | Le menu déroulant s'affiche par-dessus tout le reste, sans être coupé ni mal positionné — et le bouton `FIN D'ENTRACTE`, juste à côté, reste net | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Sortie de l'entracte : retour à l'identique

**Objectif** : Vérifier que rien n'est perdu — la question sélectionnée, les scores.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avant l'entracte, sélectionner une question en `PREPARE` (sans lancer) | Question visible en aperçu sur `/admin` | | |
| 2 | Déclencher l'entracte, attendre quelques secondes, cliquer sur `FIN D'ENTRACTE` | Panneau et filtre disparaissent progressivement sur les 4 surfaces, LEDs reprennent la couleur de phase courante | | |
| 3 | Vérifier la question sélectionnée sur `/admin` | Toujours la même question, en PREPARE, prête à lancer | | |
| 4 | Vérifier les scores des équipes | Inchangés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 12 — Redémarrage du serveur pendant un entracte

**Objectif** : Vérifier D5 — l'entracte (booléen) n'est pas persisté, la configuration l'est
désormais dans les métadonnées de la partie (`game_state.json`), plus dans `game-config.json`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Déclencher l'entracte, personnaliser titre/sous-titre depuis la page Quiz si pas déjà fait | Entracte actif avec textes personnalisés | | |
| 2 | Redémarrer le serveur (méthode habituelle : `/shutdown` puis relance) | Redémarrage réussi | | |
| 3 | Reconnecter `/admin` et `/tv` | Le serveur repart HORS entracte (bouton `ENTRACTE` normal dans la Navbar, pas de panneau) | | |
| 4 | Rouvrir la page Quiz | Les textes personnalisés (titre/sous-titre) sont toujours là (persistés dans `game_state.json`, comme les métadonnées de quiz) | | |
| 5 | (Si testé) Lancer une `NOUVELLE PARTIE` | La configuration d'entracte survit — c'est un réglage de session, pas remis à zéro à chaque partie (comportement voulu, confirmé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 13 — Non-régression : partie normale hors entracte

**Objectif** : Vérifier qu'aucune régression n'affecte le déroulé normal d'une question.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer une question complète (LANCER → RÉPONSE) de chaque type disponible (SPEEDY, ARDOISE, QCM, MEMORY, MEMOTION si présents dans le quiz) | Déroulé strictement identique à avant #119 — aucun filtre, aucun panneau, boutons normaux | | |
| 2 | Vérifier les LEDs pendant cette partie normale | Comportement inchangé (couleurs d'équipe, clignotements habituels) | | |
| 3 | Vérifier `/anim` pendant cette partie | Aucun indicateur ni filtre entracte visible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 14 — Transition progressive à l'entrée et à la sortie (C3, nouveau)

**Objectif** : Vérifier que le basculement n'est plus instantané — un fondu d'environ 2
secondes (valeur par défaut) s'applique au filtre et au panneau, à l'aller comme au retour.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cliquer sur `ENTRACTE` en observant attentivement `/tv` | L'estompage (grisé/assombri) se produit PROGRESSIVEMENT (~2s), pas d'un coup | | |
| 2 | Observer l'apparition du panneau au même moment | Le panneau apparaît en fondu (fade-in), pas en un seul instant | | |
| 3 | Cliquer sur `FIN D'ENTRACTE` en observant `/tv` | L'estompage disparaît PROGRESSIVEMENT (~2s) — même durée qu'à l'entrée | | |
| 4 | Observer la disparition du panneau au même moment | Le panneau disparaît en fondu (fade-out), reste visible pendant toute la transition (ne disparaît pas instantanément avant la fin du fondu) | | |
| 5 | Répéter l'observation sur `/player` (VJoueur) — cadenas 🔒 inclus | Même comportement de fondu, cadenas disparaît/apparaît avec le panneau, pas net sur un fond encore en transition | | |
| 6 | Sur `/anim`, observer l'indicateur « Entracte en cours » à l'entrée et à la sortie | Fondu également, pas d'apparition/disparition brutale | | |
| 7 | Sur un système avec « Réduire les animations » activé (accessibilité système), répéter l'entrée/sortie | Le FONDU reste présent (ce n'est pas neutralisé) — seule l'animation de respiration du panneau (zoom/balancement pendant la pause) doit être neutralisée | | |
| 8 | Page Quiz, section Entracte, régler le curseur de transition à 0 puis Enregistrer, redéclencher un entracte | Bascule redevient INSTANTANÉE (comportement pré-C3) — confirme que `0` = bascule instantanée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 15 — Configuration complète du panneau, depuis la page Quiz (image, taille, animation, transition)

**Objectif** : Vérifier la configuration complète, désormais sur la page Quiz plutôt que
Config.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la page Quiz (`/admin/quiz`), section Entracte, uploader une image de fond | Aperçu mis à jour, sauvegarde confirmée | | |
| 2 | Déclencher un entracte (hors pause au moment de l'upload, pour un effet immédiat) | L'image de fond apparaît dans le panneau sur TV et VJoueur, texte toujours lisible | | |
| 3 | Retour à la page Quiz, ajuster le curseur de taille du panneau | Valeur affichée change | | |
| 4 | Enregistrer, observer TV et VJoueur (hors pause, ou après un cycle sortie/entrée si testé pendant — cf. Scénario 8) | La taille du panneau change identiquement sur les deux surfaces | | |
| 5 | Ajuster la vitesse et l'intensité de l'animation, dont un passage à intensité 0 | Le libellé « animation désactivée » apparaît à 0 | | |
| 6 | Observer le panneau sur TV | Zoom/balancement visibles à intensité > 0, panneau parfaitement fixe à intensité 0 | | |
| 7 | Ajuster le curseur de transition (durée du fondu) | Valeur affichée en millisecondes/secondes change, mention « transition instantanée » à 0 | | |
| 8 | Supprimer l'image de fond | Le panneau retombe sur son fond par défaut, reste lisible | | |
| 9 | Vérifier que la section Entracte n'apparaît PLUS du tout sur `/settings` (Config) | Absente — entièrement déménagée vers la page Quiz | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Le bouton ENTRACTE, dans la Navbar, respecte la table des phases et reste visible sur
      toutes les pages admin (Scénarios 1, 3)
- [ ] Les 4 surfaces s'estompent ensemble, panneau centré identique TV/VJoueur, cadenas net (Scénario 2)
- [ ] LEDs physiquement éteintes pendant toute la durée de l'entracte (Scénario 4)
- [ ] Buzz physique inerte pendant l'entracte (Scénario 5)
- [ ] **Blocage serveur réel** — aucun clic sur un bouton estompé n'a d'effet (Scénario 6, critère central)
- [ ] Reconnexion VJoueur et nouvel écran TV pendant l'entracte fonctionnent sans accroc (Scénario 7)
- [ ] **Configuration figée à l'activation** — un enregistrement pendant la pause ne change pas
      le panneau affiché, prend effet au prochain entracte, mention explicite affichée
      (Scénario 8) ; hors entracte, effet immédiat (Scénario 9)
- [ ] Aucun élément flottant déplacé ou masqué (Scénario 10)
- [ ] Sortie d'entracte : état de jeu et scores intacts, LEDs restaurées (Scénario 11)
- [ ] Redémarrage serveur : entracte non persisté, configuration persistée dans les métadonnées
      de la partie, survit à une nouvelle partie (Scénario 12)
- [ ] Aucune régression sur une partie normale (Scénario 13)
- [ ] Transition progressive (~2s) à l'entrée et à la sortie sur les 4 surfaces, fondu conservé
      sous « réduire les animations » (contrairement à la respiration du panneau), `0` = bascule
      instantanée (Scénario 14)
- [ ] Configuration complète du panneau, désormais depuis la page Quiz, absente de `/settings`
      (Scénario 15)

---

## Notes QA

[Espace pour observations]
