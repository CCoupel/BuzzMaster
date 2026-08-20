# Procédure de Test — Mode ENTRACTE (#119)

**Version** : milestone v6.5.2
**Date** : 2026-08-20
**Testeur** : Utilisateur (les agents n'ont pas de navigateur fiable — voir
`feedback_manual_qa_is_user_role.md`)
**Issue** : #119 — pause globale déclenchée depuis l'admin, indépendante du cycle de question
**Référence** : Plan `_work/reports/plan-entracte-119-20260820-140825.md`, maquette validée
`docs/mockups/entracte-119.html`, contrats `contracts/game-state.md` §ENTRACTE,
`contracts/websocket-actions.md` §ENTRACTE_SET

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] 4 postes/onglets ouverts : `/admin`, `/anim`, `/tv`, `/player` (VJoueur enrôlé avec au
      moins un bumper physique connecté)
- [ ] Au moins 2 équipes actives, un bumper physique par équipe (pour vérifier les LEDs)
- [ ] Un quiz avec au moins une question chargeable (pour vérifier la non-régression et le
      blocage du lancement)
- [ ] Accès à `/settings` (ConfigPage) pour la section Entracte (titre, sous-titre, image, taille
      du panneau, animation)
- [ ] Accès physique aux buzzers pour vérifier visuellement l'extinction des LEDs

---

## Scénario 1 — Bouton ENTRACTE grisé pendant une question en cours

**Objectif** : Vérifier que l'entracte ne peut pas interrompre une manche (D4).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question, LANCER (phase STARTED) | Bouton `ENTRACTE` sur `/admin` grisé, non cliquable | | |
| 2 | Mettre en PAUSE | Bouton toujours grisé | | |
| 3 | Cliquer malgré tout sur le bouton grisé | Aucun effet — pas de panneau, pas de filtre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Déclenchement de l'entracte : les 4 surfaces s'estompent ensemble

**Objectif** : Vérifier l'effet visuel synchronisé sur TV, VJoueur, admin et anim.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Arrêter/révéler la question en cours (phase STOPPED ou REVEALED) | Bouton `ENTRACTE` redevient actif sur `/admin` | | |
| 2 | Cliquer sur `ENTRACTE` | Le bouton passe à `FIN D'ENTRACTE` (rouge/actif), le reste de `/admin` s'assombrit (grisé/estompé), le bouton lui-même reste NET et cliquable | | |
| 3 | Observer `/tv` | Contenu existant visible mais estompé (gris/sombre), panneau ENTRACTE centré affiché par-dessus (titre + sous-titre) | | |
| 4 | Observer `/player` (VJoueur) | Même estompage, même panneau centré, proportions identiques à la TV, cadenas 🔒 net au-dessus de la zone de buzz | | |
| 5 | Observer `/anim` | Interface estompée, indicateur « ⏸ Entracte en cours — contrôle réservé à l'admin » net, aucun bouton de contrôle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Extinction physique des LEDs des buzzers

**Objectif** : Vérifier B5 — les LEDs des buzzers physiques s'éteignent réellement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec l'entracte déjà actif (suite du scénario 2), observer physiquement chaque buzzer | Toutes les LEDs sont ÉTEINTES (aucune couleur, aucun clignotement) | | |
| 2 | Attendre quelques secondes | Les LEDs restent éteintes en continu (pas de réveil intempestif) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Appui buzzer physique inerte pendant l'entracte

**Objectif** : Non-régression — le buzz physique reste sans effet (garde de phase déjà en place).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, appuyer physiquement sur un buzzer | Aucun effet visible sur `/admin`, `/anim`, `/tv` — LED toujours éteinte | | |
| 2 | Répéter sur 2-3 buzzers différents | Même résultat pour chacun | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Boutons estompés d'admin et d'anim réellement bloqués (le cœur de #119)

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

## Scénario 6 — Rechargement du VJoueur pendant l'entracte

**Objectif** : Vérifier que la reconnexion pendant la pause revient directement en entracte
(D6 : HELLO/PLAYER_CONNECT restent autorisés).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, recharger la page `/player` (F5) | La page revient directement avec le filtre et le panneau ENTRACTE affichés, sans flash de l'écran de jeu normal | | |
| 2 | Vérifier l'identité du joueur (nom, équipe) | Conservée, pas de nouvelle inscription | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Nouvel écran TV connecté pendant l'entracte

**Objectif** : Vérifier D2 — la config du panneau arrive dans le même UPDATE que le drapeau,
y compris pour un client qui se connecte pendant la pause.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, ouvrir un NOUVEL onglet/écran sur `/tv` | Panneau et filtre affichés IMMÉDIATEMENT, avec le bon titre/sous-titre (pas de panneau vide ni de délai visible) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Modification des textes en direct pendant un entracte actif

**Objectif** : Vérifier que la config est diffusée en direct (B6, F8).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte toujours actif, ouvrir `/settings` dans un autre onglet | Section « Entracte » visible avec les valeurs courantes | | |
| 2 | Modifier le titre et le sous-titre, cliquer Enregistrer | Confirmation de sauvegarde | | |
| 3 | Observer `/tv` et `/player` (déjà ouverts) | Le nouveau titre/sous-titre apparaît SANS rechargement de page | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Aucun élément flottant ne se déplace (piège `filter`/`position:fixed`)

**Objectif** : Vérifier D6/F4/F5 — QR code, bandeau régie, navbar restent bien positionnés.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Entracte actif, phase ENROLL sur un autre cycle (ou juste après sortie), observer le QR code sur `/tv` si affiché | Position et taille normales, pas de décalage ni de passage derrière le panneau | | |
| 2 | Sur `/admin`, envoyer un message régie pendant l'entracte (`REGIE_MESSAGE_SEND` reste autorisé, D6) | Le bandeau régie reste ancré en bas d'écran sur `/anim`, non estompé, non décalé | | |
| 3 | Ouvrir le menu de la navbar sur `/admin` (si applicable) pendant l'entracte | Le menu déroulant s'affiche par-dessus tout le reste, sans être coupé ni mal positionné | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Sortie de l'entracte : retour à l'identique

**Objectif** : Vérifier que rien n'est perdu — la question sélectionnée, les scores.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avant l'entracte, sélectionner une question en `PREPARE` (sans lancer) | Question visible en aperçu sur `/admin` | | |
| 2 | Déclencher l'entracte, attendre quelques secondes, cliquer sur `FIN D'ENTRACTE` | Panneau et filtre disparaissent sur les 4 surfaces, LEDs reprennent la couleur de phase courante | | |
| 3 | Vérifier la question sélectionnée sur `/admin` | Toujours la même question, en PREPARE, prête à lancer | | |
| 4 | Vérifier les scores des équipes | Inchangés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Redémarrage du serveur pendant un entracte

**Objectif** : Vérifier D5 — l'entracte n'est pas persisté, la config l'est.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Déclencher l'entracte, personnaliser titre/sous-titre si pas déjà fait | Entracte actif avec textes personnalisés | | |
| 2 | Redémarrer le serveur (méthode habituelle : `/shutdown` puis relance) | Redémarrage réussi | | |
| 3 | Reconnecter `/admin` et `/tv` | Le serveur repart HORS entracte (bouton `ENTRACTE` normal, pas de panneau) | | |
| 4 | Rouvrir `/settings` | Les textes personnalisés (titre/sous-titre) sont toujours là (persistés dans game-config.json) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 12 — Non-régression : partie normale hors entracte

**Objectif** : Vérifier qu'aucune régression n'affecte le déroulé normal d'une question.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer une question complète (LANCER → RÉPONSE) de chaque type disponible (SPEEDY, ARDOISE, QCM, MEMORY, MEMOTION si présents dans le quiz) | Déroulé strictement identique à avant #119 — aucun filtre, aucun panneau, boutons normaux | | |
| 2 | Vérifier les LEDs pendant cette partie normale | Comportement inchangé (couleurs d'équipe, clignotements habituels) | | |
| 3 | Vérifier `/anim` pendant cette partie | Aucun indicateur ni filtre entracte visible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 13 — Configuration du panneau (image, taille, animation)

**Objectif** : Vérifier F8/ConfigPage — réglages complets du panneau.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/settings`, section Entracte, uploader une image de fond | Aperçu mis à jour, sauvegarde confirmée | | |
| 2 | Déclencher un entracte | L'image de fond apparaît dans le panneau sur TV et VJoueur, texte toujours lisible | | |
| 3 | Retour à `/settings`, ajuster le curseur de taille du panneau | Valeur affichée change | | |
| 4 | Observer TV et VJoueur après sauvegarde | La taille du panneau change identiquement sur les deux surfaces | | |
| 5 | Ajuster la vitesse et l'intensité de l'animation, dont un passage à intensité 0 | Le libellé « animation désactivée » apparaît à 0 | | |
| 6 | Observer le panneau sur TV | Zoom/balancement visibles à intensité > 0, panneau parfaitement fixe à intensité 0 | | |
| 7 | Supprimer l'image de fond | Le panneau retombe sur son fond par défaut, reste lisible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Le bouton ENTRACTE respecte la table des phases (Scénario 1)
- [ ] Les 4 surfaces s'estompent ensemble, panneau centré identique TV/VJoueur, cadenas net (Scénario 2)
- [ ] LEDs physiquement éteintes pendant toute la durée de l'entracte (Scénario 3)
- [ ] Buzz physique inerte pendant l'entracte (Scénario 4)
- [ ] **Blocage serveur réel** — aucun clic sur un bouton estompé n'a d'effet (Scénario 5, critère central)
- [ ] Reconnexion VJoueur et nouvel écran TV pendant l'entracte fonctionnent sans accroc (Scénarios 6-7)
- [ ] Modification des textes en direct visible sans rechargement (Scénario 8)
- [ ] Aucun élément flottant déplacé ou masqué (Scénario 9)
- [ ] Sortie d'entracte : état de jeu et scores intacts, LEDs restaurées (Scénario 10)
- [ ] Redémarrage serveur : entracte non persisté, configuration persistée (Scénario 11)
- [ ] Aucune régression sur une partie normale (Scénario 12)
- [ ] Configuration complète du panneau (image, taille, animation) fonctionnelle (Scénario 13)

---

## Notes QA

[Espace pour observations]
