# Procédure de Test — Attribution groupée équipe ou joueur, bloc compact, pénalité QCM, commutateur « indices » (#254, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-11
**Testeur** : **l'utilisateur uniquement** (modifie les scores, lumières et sons, plusieurs tablettes : ni `qa` ni `deployer` ne l'exécutent)
**Contrats** : `contracts/websocket-actions.md` §TEAM_POINTS_BATCH (« Niveau d'attribution », « Montant », « QCM »), §BUMPER_POINTS_BATCH, §AWARDED_TEAMS ; `contracts/models.md` (`QCM_WRONG_PENALTY`, `POINTS_TARGET`) ; `contracts/http-endpoints.md` (`qcm_wrong_penalty`)
**Maquettes** : `docs/mockup/v11.2/ui/admin_game_award__compact_block_hints_toggle.html`, `admin_game_award__team_or_player_level_hints_toggle.html`, `admin_question_editor__qcm_wrong_penalty_default0.html`
**Tests automatiques** : `tests/INDEX.md` (lignes #254)

> **Sauvegarder** `data/` avant. Prévoir une partie avec 4 équipes (2 joueurs chacune si possible, tablettes VJoueur et/ou buzzers physiques), et au moins : un QCM avec indices progressifs, un QCM sans indices, un SPEEDY, une ARDOISE, un MEMORY. Poste admin `/admin`, anim `/anim`, TV `/tv`.

## Prérequis

- [ ] QUALIF, partie en cours, 4 équipes ayant des joueurs connectés
- [ ] `buzzcontrol.log` accessible

---

## Scénario 1 — Éditeur : « Pénalité mauvaise réponse » (QCM seulement)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Page Quiz : nouvelle question de type **QCM** | Sur la ligne Points / Temps, champ « Pénalité mauvaise réponse » avec signe « − » fixe, valeur **0** (« par défaut ») | | |
| 2 | Ouvrir une **ancienne** question QCM (sans le champ) | Champ affiché à **0 « par défaut »** | | |
| 3 | Saisir **2**, enregistrer, rouvrir | Champ à **2** (enregistré en −2) | | |
| 4 | Saisir **150** | Message d'erreur « Entier de 0 à 100 », bouton Modifier/Ajouter **désactivé** | | |
| 5 | Saisir **0**, enregistrer | Accepté (aucune pénalité) | | |
| 6 | Types SPEEDY, ARDOISE, MEMORY, MEMOTION | **Aucun** champ de pénalité | | |
| 7 | Fichiers des anciennes questions (`data/files/questions/*/question.json`) | **Non réécrits** tant qu'on ne les enregistre pas depuis l'éditeur | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Bloc compact (2 lignes) et icônes

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Question QCM, passer en REVEALED | En tête de la colonne Équipes : bloc **sur 2 lignes** (titre + source du montant ; icônes + bouton « Attribuer les points (0) » désactivé), **sans** champ de montant propre | | |
| 2 | Survoler / focaliser les deux icônes | Infobulles « Tout sélectionner » / « Tout désélectionner » ; nom accessible identique (lecteur d'écran ou Tab) | | |
| 3 | « Tout sélectionner » avec une équipe déjà créditée | Les équipes déjà créditées restent **grisées** et non cochées | | |
| 4 | Avant le REVEAL | Ni cases, ni bloc, ni réponses cliquables | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Niveau équipe ou joueur selon le type

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | QCM (par défaut), ARDOISE (sans réglage), après REVEAL | Cases **dans l'en-tête des cartes d'équipe**, aucune case sur les joueurs | | |
| 2 | SPEEDY (par défaut), QCM réglé « Points : joueur » | Cases **sur les lignes de joueurs**, aucune case d'équipe | | |
| 3 | MEMORY, MEMOTION, RAFALE (même réglé « joueur » dans un ancien fichier) | Toujours niveau **équipe** | | |
| 4 | Ancienne question ARDOISE sans réglage d'attribution | Crédite l'**équipe** (changement de défaut) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — QCM : clic sur une réponse, pénalité

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Cliquer la **bonne** réponse | Équipes (ou joueurs) de cette couleur sélectionnés, champ « Points » = base, « Attribuer les points (N) » | | |
| 2 | QCM avec pénalité −3 : cliquer une **mauvaise** réponse | Désélection puis sélection de ses équipes ; champ « Points » = **−3** ; « Appliquer les pénalités (N) » (rouge), actif | | |
| 3 | QCM **sans** pénalité (0) : cliquer une mauvaise réponse | Sélection faite, champ « Points » = **0**, bouton **désactivé**, raison « Aucune pénalité définie pour cette question » | | |
| 4 | Taper −1 dans « Points » | Bouton « Appliquer les pénalités » actif ; taper 2 → « Attribuer les points » | | |
| 5 | Rebasculer sur la bonne réponse | « Points » revient à la base ; libellé « Attribuer les points » | | |
| 6 | Pendant la pénalité, regarder la tablette animateur | Son bouton de crédit reste sur la **dernière valeur positive** (jamais ≤ 0) | | |
| 7 | Mauvaise réponse avec 0, 1 puis 2 indices révélés | **Même** pénalité (indices sans effet) ; pas de commutateur « indices » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Base du barème et commutateur « Tenir compte des indices »

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Régler « Points » à **6** AVANT le REVEAL, puis REVEAL | « Points » = 6 ; la bonne réponse y revient après une mauvaise réponse | | |
| 2 | QCM avec indices, 2 indices révélés, équipes buzzées à 0, 1, 2 indices ; bonne réponse | Aperçus **+3 / +2 / +1** (base 3) ; commutateur visible, **activé** ; libellé « …ajusté selon les indices de chaque équipe » | | |
| 3 | Désactiver le commutateur | **+3 partout** ; libellé « montant uniforme » | | |
| 4 | Forcer « Points » à 6 après le REVEAL | Activé : +6 / +4 / +2 (« montant saisi, ajusté… ») ; désactivé : +6 partout | | |
| 5 | Cliquer « Attribuer… » | Les montants crédités sont **exactement** les aperçus ; scores et historique concordent | | |
| 6 | Commutateur masqué | QCM sans indices activés ; QCM avec indices mais aucun révélé ; SPEEDY, ARDOISE, MEMORY, RAFALE ; montant ≤ 0 | | |
| 7 | Nouvelle question | Commutateur **réactivé**, « Points » = nouvelle base | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Niveau joueur : lot, déjà crédités, double clic

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Question « joueur » : cocher 3 joueurs de 2 équipes, « Attribuer les points (3) » | Scores des **joueurs** et des équipes à jour ; message « ✓ Points attribués à 3 joueurs. » ; sélection vidée | | |
| 2 | Joueurs crédités | Lignes grisées « ✓ déjà crédité +N », cases non cochables | | |
| 3 | Re-sélectionner les mêmes joueurs (autre onglet ou clic rapide) | Aucun **double crédit** (message « … déjà crédité(s) ignoré(s) ») | | |
| 4 | Double clic rapide sur le bouton | Bouton désactivé pendant l'envoi ; **un seul** lot | | |
| 5 | Supprimer un joueur coché dans un autre onglet, puis envoyer | Refus « Joueur introuvable (supprimé) », joueur retiré de la sélection | | |
| 6 | TV / son / LED | **Un** son, une comète par équipe concernée, **une** pulsation, un flash ; aucun effet si tout est négatif | | |
| 7 | `buzzcontrol.log` | INFO `BUMPER_POINTS_BATCH` (ou WARN `…_REFUSED` pour un refus) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 7 — Non-régression

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Clic unitaire sur une carte d'équipe / un joueur | Toujours libre (même après un lot) ; même montant par défaut que le lot | | |
| 2 | Attribution depuis l'anim | Inchangée (valeur positive uniquement) | | |
| 3 | Correction cachée (page Scores, interrupteur « Afficher sur la TV » désactivé) | Toujours sans effet public (#252) | | |
| 4 | Popup « Lancer la partie » ouverte sur un autre poste | Attribution groupée **suspendue** (#253) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Niveau équipe ou joueur suit la question (SPEEDY joueur, QCM/ARDOISE équipe, MEMORY/MEMOTION/RAFALE équipe)
- [ ] Bloc compact sur 2 lignes, sans montant propre, icônes accessibles
- [ ] Mauvaise réponse : pénalité de la question ; 0 = bouton désactivé avec la raison
- [ ] Indices : base × coefficient au buzz, commutateur, montant uniforme pour une pénalité ; aperçu = montant crédité
- [ ] Aucun double crédit, aucun crédit partiel (validation avant effet)
- [ ] L'anim ne reçoit jamais un montant ≤ 0

## Notes

[Observations]
