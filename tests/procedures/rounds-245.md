# Procédure de Test — Manches, entractes automatiques, déroulé piloté par l'anim (#245, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-12
**Testeur** : **l'utilisateur uniquement** (éditeur, tablette animateur, écran TV : ni `qa` ni `deployer` ne l'exécutent)
**Contrats** : `contracts/websocket-actions.md` §ROUNDS_UPDATE, §NEXT_QUESTION, §ENTRACTE_SET ; `contracts/models.md` §Manches ; `contracts/game-state.md` §ROUNDS / ROUND_CONTEXT
**Maquettes** : `docs/mockup/v11.2/ui/admin_questions_list__rounds_entractes_delete_choice.html` (f″), `docs/mockup/v11.2/ui/game_round_indicator__admin_anim_tv_next_entracte.html` (g‴)
**Tests automatiques** : `tests/INDEX.md` (lignes #245)

> **Sauvegarder** `data/` avant : le scénario 5 supprime des questions (DELETE). Ouvrir `/admin/quiz` (éditeur), `/admin` (jeu), `/anim` (tablette), `/tv` (écran visible).

## Prérequis

- [ ] QUALIF, un quiz d'au moins 8 questions (dont une ENTRACTE programmée), aucune manche au départ
- [ ] Tablettes/buzzers pour jouer quelques questions

---

## Scénario 1 — Sans manche, rien ne change

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Page Quiz sans manche | Liste identique à avant (pas de groupe), bouton **« + Nouvelle manche »** | | |
| 2 | Jouer 2 questions | « À suivre » (admin) et suivant (anim) suivent l'ordre habituel ; pas de pastille de manche sur l'anim, rien sur la TV | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Créer 2 manches avec leurs entractes

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | « + Nouvelle manche » | Ligne de saisie ; case **« Créer les entractes de début et de fin » cochée par défaut** | | |
| 2 | Nom « Échauffement », valider | Manche créée **avec deux entractes** « Début de la manche Échauffement » (en tête) et « Fin de la manche Échauffement » (à la fin), badge ENTRACTE | | |
| 3 | Seconde manche « Finale », case **décochée**, valider | Manche créée **sans** entractes | | |
| 4 | Nom vide / plus de 60 caractères | Validation impossible (champ limité à 60) | | |
| 5 | Renommer « Échauffement » en « Manche A » | Les entractes **ne sont pas renommées** | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Ranger les questions

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | « Sans manche » apparaît **en premier** | Puis les manches dans l'ordre | | |
| 2 | ▲▼ sur une manche | L'ordre change et est conservé après rechargement de la page | | |
| 3 | Glisser-déposer d'une question dans une manche ; d'une manche ; avec le doigt (tablette) : ▲▼ et **« Déplacer vers… »** | Chaque geste est enregistré ; **aucune question ne disparaît** | | |
| 4 | Déplacer une entracte de manche dans une autre manche ou dans « Sans manche » | Se déplace comme toute question | | |
| 5 | Replier / déplier une manche, « ⇕ Tout replier », recharger | Le repli est **conservé** (propre au navigateur) | | |
| 6 | Filtre de catégorie | Chaque groupe affiche « x / y », les groupes sans résultat restent affichés | | |
| 7 | « + Question » dans une manche | Le formulaire propose cette manche ; la question créée arrive **en fin de manche** | | |
| 8 | Modifier une question d'une manche, enregistrer | Elle **reste** dans sa manche | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Suppression d'une manche : Conserver

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | 🗑 sur une manche contenant des questions et ses 2 entractes | Fenêtre : « Conserver les N entrées » / « Supprimer aussi les N entrées » / Annuler ; elle dit combien d'**entractes** sont concernées ; **focus sur « Conserver »** | | |
| 2 | Annuler (ou Échap) | Rien n'est envoyé ni modifié | | |
| 3 | Conserver | La manche disparaît ; ses entrées (entractes comprises) passent à la **fin de « Sans manche »**, dans leur ordre ; **aucun fichier supprimé** | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Suppression d'une manche : Supprimer aussi

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | 🗑 sur une manche d'essai, « Supprimer aussi » | Manche **et** ses entrées (entractes, médias) supprimées définitivement | | |
| 2 | Dossiers `data/files/questions/<id>` | Supprimés | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Déroulé complet piloté par l'anim seule

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Quiz : « Sans manche » (1 question), manche A (entracte de début, 2 questions, entracte de fin), manche B (1 question) | | | |
| 2 | Anim : bouton « suivant » depuis le début | Propose la 1re question ; pastille inchangée | | |
| 3 | Jouer la question, suivant | « **Manche suivante : A** » ; pastille « Manche 1/2 · A » à l'arrivée | | |
| 4 | Suivant sur l'entracte de début | Bouton violet « **Lancer l'entracte** » ; cycle préparation → prêt → départ ; le panneau d'entracte s'affiche partout (TV, tablettes) | | |
| 5 | Pendant l'entracte | Bouton « **Terminer l'entracte** » (anim) ; l'entrée passe à STOPPED, « suivant » reprend sur la 1re question de la manche | | |
| 6 | TV | Pastille « Manche 1 · A » ; **titre de manche** seulement sur l'écran d'attente de la 1re question de la manche | | |
| 7 | Poursuivre : questions, entracte de fin, manche B | Aucune action de l'admin nécessaire | | |
| 8 | L'anim cherche à déclencher une entracte à la main | **Aucun bouton** pour cela (reste dans la barre admin) ; un envoi forcé est refusé et journalisé | | |
| 9 | Admin : « À suivre » | Montre aussi l'entracte (« À suivre : 🎬 … », violet) et « → Manche 2 · … » au changement de manche | | |
| 10 | Admin : liste de jeu | Titres de manche, repliables ; une entracte se lance par clic sur sa ligne | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 7 — Écran TV statique et sauvegarde

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Nom de manche de 60 caractères sur la TV (1280×720 puis 1920×1080) | Texte tronqué (…), **rien ne défile, rien ne déborde** | | |
| 2 | Sauvegarde « questions » puis restauration | `rounds.json` restauré ; manches et rattachements retrouvés | | |
| 3 | Journal | `ROUNDS_UPDATE` (INFO), refus journalisés en WARN | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Sans manche, aucun changement (ordre, écrans)
- [ ] Les manches se regroupent, se réordonnent (souris et doigt), se repliables ; jamais de question perdue
- [ ] Suppression : Conserver / Supprimer aussi, entractes comprises
- [ ] L'anim enchaîne seule le déroulé complet, y compris les entractes
