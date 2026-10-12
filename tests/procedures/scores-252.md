# Procédure de Test — Points libres et « Afficher sur la TV » (correction cachée) (#252, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-11
**Testeur** : **l'utilisateur uniquement** (modifie les scores, lumières et sons : ni `qa` ni `deployer` ne l'exécutent)
**Contrats** : `contracts/websocket-actions.md` §TEAM_POINTS « Affichage public », §BUMPER_POINTS, §TEAM_POINTS_BATCH ; `contracts/game-state.md` §SCORE_CHANGE ; `contracts/logging.md` §3.2 quinquies
**Maquette** : `docs/mockup/v11.2/ui/admin_scores__free_points_tv_toggle.html`
**Tests automatiques** : `tests/INDEX.md` (lignes #252)

> **Sauvegarder** `data/` avant. Pour juger « aucun effet », le **son**, les **LED des buzzers**, l'**ambiance Hue** et l'**écran TV** (vue Jeu, vue Scores, vue Joueurs) doivent être branchés et visibles.

## Prérequis

- [ ] QUALIF, `/admin/scores` ouverte, `/tv` sur un écran visible, au moins 1 tablette joueur (VJoueur) ouverte, 2 équipes avec buzzers physiques si possible
- [ ] `buzzcontrol.log` accessible

---

## Scénario 1 — Comportement inchangé (interrupteur activé)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Ouvrir la page Scores | Interrupteur « 📺 Afficher sur la TV » **activé** (« animation + son + LED »), aucun bandeau | | |
| 2 | Cliquer **+5** sur une équipe | Score +5 ; sur la TV : « +5 pts » flottant + célébration ; son « gagné », comète LED sur les buzzers de l'équipe, pulsation d'ambiance, flash | | |
| 3 | Cliquer **−2** | Score −2 ; aucun son ni LED ni animation de gain (comme avant) | | |
| 4 | Vue Scores de la TV | Surbrillance de la ligne et montée de rang comme avant | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Montant libre

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Saisir **7** dans « autre » | Tous les boutons deviennent « −7 / +7 » (équipes **et** joueurs) ; le bouton prédéfini n'est plus actif | | |
| 2 | Cliquer +7 sur une équipe puis sur un joueur | Le score de l'équipe et celui du joueur augmentent de 7 | | |
| 3 | Saisir **−4** | Boutons « −4 / +4 » (le signe se choisit avec −/+) | | |
| 4 | Saisir **999**, puis **−999** | Acceptés | | |
| 5 | Saisir **2,5**, **0**, **1000**, **−1000** | Message « Entier de −999 à 999 », champ marqué invalide, **boutons −/+ désactivés** | | |
| 6 | Effacer le champ / cliquer 1, 2, 5 ou 10 | Retour au montant prédéfini | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Correction cachée : aucun effet nulle part

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Désactiver l'interrupteur | Bandeau « 🔇 Correction cachée… » ; **tous** les boutons portent 🔇 et la mention « sans affichage TV » au survol | | |
| 2 | Cliquer **+5** sur une équipe | Le score passe à jour **partout** (admin, TV, anim) | | |
| 3 | Observer la salle | **Aucun** son, **aucune** comète LED, **aucune** pulsation d'ambiance, **aucun** flash | | |
| 4 | Observer la TV (vue Jeu) | **Aucun** « +5 pts », aucune célébration, aucun confetti | | |
| 5 | TV vue **Scores** (changer l'équipe de rang avec la correction) | Les chiffres changent, **sans** surbrillance ni animation de montée de rang | | |
| 6 | TV vue **Joueurs** : correction sur un joueur | Score mis à jour, sans surbrillance | | |
| 7 | **Tablette joueur** (vue Scores) | Même règle : mise à jour sans animation | | |
| 8 | Faire ensuite une attribution **visible** (+2) | Animation « +2 pts » uniquement (jamais +7 en cumulant la correction cachée) | | |
| 9 | Journal | `[WARN] SCORE_CORRECTION_HIDDEN … action=TEAM_POINTS targets=…` | | |
| 10 | Rouvrir la page Scores (rechargement) | Interrupteur **de nouveau activé** (jamais oublié désactivé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Correction cachée et attribution de la question

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Question jouée et **révélée** ; corriger **en caché** « Les Bleus » (+2) | Score à jour, pas d'effet | | |
| 2 | Page de jeu : la carte « Les Bleus » | **N'est pas grisée** « déjà créditée » | | |
| 3 | « Donner les points » du lot incluant « Les Bleus » | Les Bleus sont **créditées** (pas ignorées) ; total = correction + points de la question | | |
| 4 | Faire un crédit **visible** sur une autre équipe puis un lot qui l'inclut | Cette équipe est ignorée (« déjà créditée ») | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Attribution groupée sans affichage

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Lot de plusieurs équipes envoyé avec `DISPLAY:false` (script/console : `{"ACTION":"TEAM_POINTS_BATCH","MSG":{"QUESTION_ID":"…","ENTRIES":[…],"DISPLAY":false}}`) | Scores à jour, **aucun** effet groupé | | |
| 2 | Même lot sans `DISPLAY` | Un son, une comète par équipe positive, une pulsation, un flash (comportement du lot #244/#251) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Historique et « Dernières attributions »

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Page Scores, section « Dernières attributions » | Les **5** attributions les plus récentes ; la correction cachée porte 🔇 | | |
| 2 | Page Historique | L'entrée cachée est marquée « 🔇 correction cachée » ; les entrées visibles ne le sont pas | | |
| 3 | Aucune ligne « équipe supprimée » (audit) dans ces listes | OK | | |
| 4 | Montant **0** envoyé par un script | Ignoré : aucune entrée d'historique, aucune mise à jour | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de validation

- [ ] Interrupteur coupé : aucun effet animé sur aucun canal (son, LED, ambiance, flash, TV, tablettes) ; scores et historique à jour
- [ ] Sans le champ DISPLAY : comportement identique à v11.1 et au lot #244/#251
- [ ] Montant libre de −999 à 999 (non nul), équipes et joueurs ; saisie invalide bloquée
- [ ] Une correction cachée ne rend jamais une équipe « déjà créditée »
- [ ] Interrupteur réactivé à chaque ouverture de la page

## Notes

[Observations du testeur]
