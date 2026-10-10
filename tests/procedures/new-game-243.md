# Procédure de Test — « Lancer la partie » : popup de purge (#243, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-11
**Testeur** : **l'utilisateur uniquement** (action destructrice : ni `qa` ni `deployer` ne l'exécutent)
**Contrats** : `contracts/websocket-actions.md` §NEW_GAME et §NEW_GAME_REFUSED, `contracts/logging.md` §3.2 ter
**Maquette** : `docs/mockup/v11.2/ui/admin_new_game__purge_popup_details.html`
**Tests automatiques** : `tests/INDEX.md` (lignes #243)

> **Sauvegarder avant** : copier le dossier `data/` (scores, joueurs, équipes, historique, statuts des questions) pour pouvoir
> rejouer chaque scénario. **Chaque scénario détruit des données.**

## Prérequis

- [ ] Environnement : QUALIF (binaire `server-go/buzzcontrol-v11.2.x.y-windows-amd64.exe`)
- [ ] 10 équipes, 10 tablettes (VJoueurs) inscrites dont quelques-unes hors ligne, 2 buzzers physiques si possible
- [ ] Quelques questions déjà jouées (statut « jouée »), des points attribués, l'historique non vide
- [ ] Postes : `/admin` (page de jeu et **Backstage**), `/tv`, `/anim`, une ou deux tablettes joueurs ouvertes
- [ ] Accès à `data/logs/buzzcontrol.log`

---

## Scénario 1 — Le bouton et ses deux points d'entrée

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Page de jeu `/admin` : repérer le bouton | Il s'appelle **« LANCER LA PARTIE »** (plus « NOUVELLE PARTIE ») | | |
| 2 | Cliquer | Une popup s'ouvre ; **rien n'est effacé ni envoyé** | | |
| 3 | « Annuler » ; ou Échap ; ou clic hors popup | La popup se ferme, rien ne change (vérifier scores/joueurs) | | |
| 4 | Backstage (`/admin/backstage`, section Quiz) | Même bouton, même popup, mêmes défauts | | |
| 5 | Au clavier : Tab dans la popup | Le focus reste dans la popup ; à la fermeture, il revient sur le bouton | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Contenu et défauts de la popup

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Ouvrir la popup | 4 cases : **Scores ☑**, **Questions ☑**, Joueurs des tablettes ☐, Équipes ☐ ; compteurs exacts : « (N équipes, M joueurs) », « (X jouées sur Y) », « (K connectés) », « (N) » | | |
| 2 | Lire les phrases | « Conservé : joueurs des tablettes, équipes. » ; « Dans tous les cas : la question en cours s'arrête. » | | |
| 3 | Cocher **Joueurs des tablettes** | **Scores se coche et se grise** avec « 🔒 Obligatoire quand les joueurs des tablettes sont supprimés… » ; avertissement rouge « ⚠ Irréversible : les N joueurs des tablettes seront supprimés. » ; bouton **rouge** | | |
| 4 | Cocher aussi Équipes | L'avertissement cite les deux ; | | |
| 5 | Décocher Joueurs | Scores redevient modifiable (reste cochée) | | |
| 6 | Tout décocher (Scores, Questions) | Le bouton devient **« Lancer sans rien effacer »** | | |
| 7 | Avec une question en cours (STARTED/PAUSED) | « ⚠ Une question est en cours : elle sera interrompue. » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Comportement par défaut (Scores + Questions) *(restaurer `data/` avant)*

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Lancer avec les défauts | Phase **NEW_GAME** ; scores d'équipe et de joueur à 0 ; historique des points vide ; toutes les questions « à jouer » | | |
| 2 | Tablettes | **Toujours inscrites** (aucun renvoi à l'inscription) | | |
| 3 | Équipes | Toutes conservées | | |
| 4 | Le bouton se désactive à l'envoi puis la popup se ferme | Pas de double envoi | | |
| 5 | Log | `[WARN] NEW_GAME_PURGE … options=scores,questions players_removed=0 teams_removed=0 scores_reset=true questions_reset=true …` | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Chaque option isolée *(restaurer `data/` entre chaque)*

| Étape | Option cochée | Résultat attendu | OK ? |
|-------|---------------|------------------|------|
| 1 | **Questions seule** (Scores décochée) | Toutes les questions « à jouer » ; scores et historique **conservés** | |
| 2 | **Scores seule** (Questions décochée) | Scores/historique à 0 ; les questions déjà jouées **restent jouées** ; « question suivante » = première non jouée | |
| 3 | **Joueurs des tablettes** (+ Scores verrouillée) | Tablettes supprimées : chacune est renvoyée à l'inscription (`GAME_RESET`) ; buzzers physiques conservés ; scores à 0 | |
| 4 | **Équipes** | Plus aucune équipe ; tous les joueurs conservés sont **sans équipe** ; les buzzers passent en couleur « sans équipe » ; les tablettes affichent l'attente d'affectation ; il faut recréer les équipes (page Équipes) | |
| 5 | **Équipes**, puis recréer une équipe du même nom que l'une des anciennes | La nouvelle équipe n'est **pas** « déjà créditée » (aucun verrou hérité) | |
| 6 | **Tout coché** | Il ne reste que les buzzers physiques, sans équipe, scores à 0 | |
| 7 | **Rien coché** (« Lancer sans rien effacer ») | Seuls : question en cours arrêtée, phase NEW_GAME, plus de question courante ; scores, joueurs, équipes, statuts conservés | |

## Scénario 5 — NEW_GAME pendant une question : la partie ne repart pas toute seule

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Lancer une question (chrono 20 s) ; à 10 s, « Lancer la partie » (défauts) | Phase **NEW_GAME**, chrono arrêté, son de question coupé | | |
| 2 | Attendre plus de 30 s | La phase reste **NEW_GAME** (jamais de retour à « STOP ») ; aucun tick de chrono sur la TV | | |
| 3 | Refaire pendant le **décompte 3-2-1** | Idem : pas de démarrage tardif de l'ancienne question | | |
| 4 | Refaire pendant une **RAFALE** et pendant une carte **MEMOTION** | Idem : plus aucun minuteur ne tourne | | |
| 5 | Sélectionner et lancer une nouvelle question juste après | Le chrono de la nouvelle question va à vitesse normale (1 s par seconde) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Refus serveur (combinaison interdite)

> Le client n'envoie jamais « joueurs sans scores » ; ce scénario simule un script ou un ancien client.

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Console navigateur de `/admin` : envoyer `{"ACTION":"NEW_GAME","MSG":{"PURGE_SCORES":false,"PURGE_PLAYERS":true,"PURGE_TEAMS":false,"RESET_QUESTIONS":true}}` sur la WebSocket ouverte | L'émetteur reçoit `NEW_GAME_REFUSED {REASON:"PLAYERS_REQUIRE_SCORES"}` ; aucun autre client ne reçoit rien | | |
| 2 | Vérifier la partie | **Rien n'a changé** : phase, question en cours, chrono qui continue, scores, joueurs, équipes, statuts | | |
| 3 | Log | `[WARN] NEW_GAME_REFUSED … reason=PLAYERS_REQUIRE_SCORES` et **pas** de `NEW_GAME_PURGE` | | |
| 4 | Envoyer `{"ACTION":"NEW_GAME","MSG":{}}` (ancien client) | Comportement d'avant (scores + joueurs purgés, équipes conservées, questions à jouer) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 7 — Double validation et journal

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Cliquer très vite deux fois sur « Lancer la partie » | Un seul envoi (bouton désactivé) ; pas d'erreur | | |
| 2 | Lire `buzzcontrol.log` après chaque scénario 3 à 6 | Une ligne `NEW_GAME_PURGE` par lancement appliqué, avec les options et les compteurs corrects (`players_removed`, `teams_removed`, `scores_reset`, `questions_reset`, `phase_before`, `question_before`) | | |
| 3 | Redémarrer le serveur après un « Équipes » | Les équipes restent supprimées (persistance de `teams.json`), les joueurs restent sans équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de validation

- [ ] Rien n'est effacé sans confirmation ; seuls Scores et Questions sont cochées par défaut
- [ ] « Joueurs des tablettes » coche et verrouille « Scores » ; le serveur refuse la combinaison sans changer l'état
- [ ] Chaque option produit exactement les effets du contrat ; un client sans options garde le comportement v11.1
- [ ] Un NEW_GAME pendant une question laisse la partie en NEW_GAME (aucun minuteur survivant)
- [ ] Chaque lancement est tracé dans le journal disque

## Notes

[Observations du testeur]
