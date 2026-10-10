# Procédure de Test — ARDOISE : identité des tablettes, journalisation, garde-fous au STOP (#246, lot A, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-10
**Testeur** : **l'utilisateur uniquement** — ni `qa` ni `deployer` n'exécutent cette procédure (elle demande de vraies tablettes, plusieurs onglets et le mode avion)
**Contrats** : `contracts/websocket-actions.md` (PLAYER_CONNECT, PLAYER_EVICTED, ARDOISE_INPUT, ARDOISE_FINAL, ARDOISE_FINAL_ACK, REVEAL, REVEAL_REFUSED, CLIENT_TRACE), `contracts/game-state.md` (ARDOISE_ANSWERS), `contracts/logging.md`
**Plan** : `_work/reports/planner-20261010-131500.md` · **Tests automatiques** : `tests/INDEX.md` (lignes #246)

> **Contexte** : à la semaine bleue 2026, le nom d'un joueur a changé tout seul et le texte d'ARDOISE de certaines
> équipes s'est vidé, sans qu'aucun log ne soit récupérable. Cette procédure vérifie que (1) une tablette ne peut plus
> écrire pour un autre joueur, (2) un incident laisse une trace sur disque, (3) le REVEAL attend les réponses.
>
> **Les scénarios 7 à 10 ne se jouent qu'après la livraison du Batch 2** (ARDOISE_FINAL + garde de REVEAL) et la
> validation des 3 maquettes. Les scénarios 1 à 6 et 11 sont jouables dès le Batch 1.

## Prérequis

- [ ] Environnement : QUALIF (binaire `server-go/buzzcontrol-v11.2.x.y-windows-amd64.exe`, port 80 libre)
- [ ] **10 tablettes** (ou téléphones) avec un navigateur chacune, sur le WiFi du serveur ; à défaut 4 minimum dont 1 avec **deux onglets**
- [ ] Un poste admin (`/admin`), un écran TV (`/tv`), une tablette animateur (`/anim`)
- [ ] Un quiz contenant : 1 question **ARDOISE** (clavier AZERTY), 1 question **SPEEDY**, 1 question **QCM**, 1 question **MEMORY**
- [ ] 10 équipes créées, 1 VJoueur par équipe (10 enrôlements via QR code), noms distincts (ex. Alice, Bob, … sauf indication)
- [ ] Accès au fichier de logs : `data/logs/buzzcontrol.log` à côté de l'exécutable (créé au démarrage)
- [ ] Recharger **toutes** les tablettes après le déploiement (elles doivent charger le nouveau bundle ; une tablette restée sur l'ancien ne peut plus agir)

---

## Scénario 1 — Les logs survivent à la partie (instrumentation)

**Objectif** : un incident laisse une trace persistante, la ligne par frappe n'est plus en INFO.

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Démarrer le serveur sans toucher à `config.json` (pas de section `logging`) | `data/logs/buzzcontrol.log` existe (dossier créé) | | |
| 2 | Lancer la question ARDOISE ; 3 tablettes tapent quelques mots | Aucune ligne `ARDOISE_INPUT: team=… text=…` dans le fichier (elle est en DEBUG) | | |
| 3 | Sur une tablette : taper « PARIS » puis appuyer sur **✕ Effacer tout** | Le fichier contient `[WARN] … ARDOISE_CLEARED … reason=clear … prev_len=5` | | |
| 4 | Sur une tablette : taper « A » puis **⌫** | `ARDOISE_CLEARED … reason=backspace_empty` | | |
| 5 | Ouvrir `/admin/logs` (journal temps réel) pendant la saisie | Le tampon temps réel garde son comportement habituel (les INFO y sont) | | |
| 6 | Dans `config.json`, mettre `"logging": {"file_enabled": false}`, relancer | Plus de fichier créé ; le serveur démarre normalement | | |
| 7 | Remettre `"file_max_size_mb": 1, "file_max_files": 3`, relancer, générer du volume (plusieurs parties, ou `file_min_level: "DEBUG"`) | Le dossier contient au plus `buzzcontrol.log`, `.1`, `.2` ; aucun fichier > ≈ 1 Mo | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Ancien onglet resté ouvert + nouvel onglet (cause probable du changement de nom)

**Objectif** : un onglet périmé ne renomme personne et n'efface pas la session de l'onglet actif.

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Sur la tablette T : onglet **B** → s'enrôler « Mamie » (équipe Rouges), rester sur la page de jeu | Mamie jouable | | |
| 2 | Admin : **NEW GAME** (purge des VJoueurs), puis réouvrir l'enrôlement | Mamie évincée ; l'onglet B reste ouvert sur son écran | | |
| 3 | Sur la même tablette T, ouvrir un **nouvel onglet A** → s'enrôler « Paul » (équipe Bleus) | Paul jouable dans l'onglet A | | |
| 4 | Revenir sur l'onglet **B** ; attendre 1 minute (couper/rétablir le WiFi de T si possible pour forcer la reconnexion) | B affiche **« Session ouverte dans un autre onglet »** et n'envoie plus rien | | |
| 5 | Admin : regarder le nom du joueur de l'équipe Bleus | Toujours **« Paul »** (jamais « Mamie ») | | |
| 6 | Sur l'onglet A : lancer ARDOISE, taper un mot | Le texte arrive dans l'équipe **Bleus** uniquement | | |
| 7 | Lire `buzzcontrol.log` | Une ligne `CLIENT_TRACE … kind=STALE_TAB` ; **aucune** ligne `VPLAYER_NAME_MISMATCH` | | |
| 8 | Fermer l'onglet B, recharger A | La session de A est intacte (pas de retour à l'inscription) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Deux onglets avec la même session (DUPLICATE_SESSION)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Tablette T, onglet 1 : joueur « Bob » connecté | OK | | |
| 2 | Même navigateur, **second onglet** sur `/player` (même `localStorage`) | L'onglet 2 se lie ; l'onglet 1 affiche **« Cette session est ouverte ailleurs »** + bouton **« Reprendre ici »** | | |
| 3 | Onglet 1 : recharger la page → vérifier le `localStorage` (outils dev) ou simplement constater qu'on n'est pas renvoyé à l'inscription | La session de Bob est conservée | | |
| 4 | Onglet 2 : lancer ARDOISE, taper | Le texte arrive ; l'onglet 1 n'envoie rien | | |
| 5 | Onglet 1 : cliquer **Reprendre ici** | L'onglet 1 reprend la main ; l'onglet 2 passe à son tour sur « session ouverte ailleurs » | | |
| 6 | Lire les logs | `VPLAYER_DUPLICATE_SESSION` (WARN) à chaque bascule ; `CLIENT_TRACE kind=DUPLICATE_SESSION` | | |
| 7 | Fermer l'onglet évincé | Bob reste **connecté** côté admin (badge vert) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Plus de liaison par nom

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Tablette T enrôlée « Mamie ». Dans les outils dev, supprimer **uniquement** la clé `vplayer_id` du `localStorage` (garder `vplayer_name`) | — | | |
| 2 | Recharger `/player` | Retour à l'inscription (ou demande de place #122) ; **aucune** reprise silencieuse du bumper de Mamie par son nom | | |
| 3 | Admin : renommer un autre VJoueur en « Mamie » | Refus : le nom reste inchangé (déjà pris) ; `VPLAYER_DUPLICATE_NAME` dans le log | | |
| 4 | S'enrôler avec « mamie  » (casse/espaces différents) | Refus « nom déjà pris » (`NAME_TAKEN`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Une tablette ne peut pas écrire pour une autre (côté serveur)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Tablette sans onglet lié (page `/player` ouverte, WiFi coupé puis rétabli sans que `PLAYER_CONNECT` ait abouti — ou ancienne version en cache non rechargée) tente de taper | Rien n'arrive dans les réponses ; `PLAYER_ACTION_UNBOUND` (ou `PLAYER_FOREIGN_ID`) dans le log | | |
| 2 | Après rechargement, la tablette retape | Le texte arrive dans la bonne équipe | | |
| 3 | Admin : simuler un BUTTON sur un bumper (bouton de test de la page de jeu) | Fonctionne comme avant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression d'une partie à 10 tablettes (identité)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Les 10 tablettes enrôlées, partie ARDOISE de 60 s, tout le monde tape en continu | Chaque équipe voit son texte monter côté admin/anim ; aucun texte ne régresse ni ne se vide tout seul | | |
| 2 | Pendant la saisie, couper le WiFi de 2 tablettes 5 s | À la reconnexion, leur texte complet est renvoyé ; pas de doublon de joueur | | |
| 3 | Admin : supprimer un VJoueur | La tablette concernée est renvoyée à l'inscription (`PLAYER_REMOVED`) ; les autres ne bougent pas | | |
| 4 | Admin : NEW GAME | Toutes les tablettes sont renvoyées à l'inscription (`GAME_RESET`) | | |
| 5 | Une tablette évincée demande une place (#122), l'admin la libère (#134) | Comportement identique à la version précédente | | |
| 6 | Rejouer une question SPEEDY, QCM, MEMORY | Aucune régression (buzz, réponse couleur, retournement de carte) | | |
| 7 | Fin de partie : lire `buzzcontrol.log` | Aucun `ARDOISE_STATE_DIVERGED`, aucun `ARDOISE_SEQ_GAP/STALE` inexpliqué, aucun `CLIENT_EVICTED_SLOW` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## ⏸ Scénarios 7 à 10 — à jouer après le Batch 2 (garde-fous au STOP)

## Scénario 7 — STOP en pleine frappe (plus de troncature)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | ARDOISE démarrée ; une tablette tape « PARIS EN FRANCE » **sans pause** et l'admin clique STOP au milieu de la frappe | La réponse affichée à la révélation = le texte **complet** tapé jusqu'au gel du clavier | | |
| 2 | Regarder la tablette | « Envoi de votre réponse… » puis **« ✓ Réponse reçue »** ; le clavier est gelé | | |
| 3 | Équipe qui n'a rien écrit | Clavier verrouillé avec le texte indicatif **« Aucune réponse »** (jamais « Votre réponse… ») et message **« ✓ Reçu — aucune réponse écrite »** ; compte comme reçue | | |
| 3b | Tablette dont le WiFi est coupé juste après STOP | « ⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau » ; au retour du réseau, passe à « ✓ Réponse reçue » | | |
| 4 | Admin/anim | Pastilles « reçue » au vert au fil des accusés ; compteur **n/10 reçues** exact | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 8 — Tablette en mode avion au STOP : REVEAL bloqué puis forçage

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Question ARDOISE ; **mettre une tablette en mode avion** pendant la saisie ; STOP | Sa pastille passe à « hors ligne » ; compteur **9/10** | | |
| 2 | Admin : bouton **REPONSE** | **Désactivé** avec le motif (équipe en attente, hors ligne) | | |
| 3 | Anim : tenter de révéler | Même blocage | | |
| 4 | Cliquer **« Révéler quand même… »** | Fenêtre de confirmation listant l'équipe manquante | | |
| 5 | Annuler | Rien ne se passe, la phase reste STOPPED | | |
| 6 | Reconfirmer | REVEAL ; l'équipe manquante affiche son dernier texte reçu marqué **« non confirmée »** | | |
| 7 | Log | `ARDOISE_REVEAL_FORCED` (WARN) avec l'équipe manquante | | |
| 8 | Refaire le STOP puis **rétablir le WiFi de la tablette avant le REVEAL** | Son texte final arrive, sa coche passe au vert, **REPONSE se débloque sans action** | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 9 — PAUSE / CONTINUE

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Pendant la saisie : PAUSE | Les finals arrivent (coches vertes) ; clavier gelé. Sur la tablette : **« ⏸ Jeu en pause · ⏳ envoi de votre réponse… »** puis **« ⏸ Jeu en pause · ✓ réponse reçue »** — **aucun** message du type « vous pourrez continuer » ni « Votre réponse… » | | |
| 2 | CONTINUE | Coches remises à zéro partout (`RECEIVED=false`), la saisie reprend avec le texte déjà écrit | | |
| 3 | STOP → tout reçu → REPONSE | REVEAL sans blocage | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 10 — Hors ARDOISE : REVEAL inchangé

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Question SPEEDY, QCM, MEMORY : STOP puis REPONSE | Aucun blocage ni message « en attente » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Lire un incident dans le fichier de logs

**Objectif** : savoir retrouver la cause (`contracts/logging.md` §4).

| Trace observée dans `buzzcontrol.log` | Conclusion attendue |
|---------------------------------------|----------------------|
| `ARDOISE_STATE_DIVERGED` | Réécriture serveur sans saisie ; le champ `site` désigne le code fautif |
| `PLAYER_FOREIGN_ID` / `PLAYER_ACTION_UNBOUND` | Tablette qui écrit pour un autre / sans liaison (désormais bloqué) |
| `VPLAYER_NAME_MISMATCH`, `VPLAYER_DUPLICATE_SESSION`, `CLIENT_TRACE kind=STALE_TAB` | Onglet périmé ou session en double |
| `ARDOISE_CLEARED reason=clear / backspace_empty` | Effacement fait sur la tablette |
| `CLIENT_TRACE kind=ARDOISE_CLEARED_SEEN` sans `ARDOISE_CLEARED` | Problème d'affichage côté admin/anim |
| `CLIENT_TRACE kind=WS_GAP` avec `DROPPED_SENDS` > 0 | Frappes perdues pendant une coupure de la tablette |
| `CLIENT_EVICTED_SLOW` | Un écran trop lent (admin/TV/anim) a été coupé par le serveur |

- [ ] Provoquer au moins un cas (scénarios 2, 3, 8) et vérifier que la ligne correspondante est bien dans le fichier **après redémarrage du serveur**.

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de validation

- [ ] Scénarios 1 à 6 : PASS (Batch 1)
- [ ] Scénarios 7 à 10 : PASS (Batch 2, après validation des maquettes)
- [ ] Le nom d'un joueur ne change jamais sans action de l'admin
- [ ] Un texte ARDOISE ne se vide jamais sans que `ARDOISE_CLEARED` (tablette) en donne la raison
- [ ] Aucune régression : enrôlement, reconnexion, éviction, demande de place, SPEEDY/QCM/MEMORY

## Notes

[Observations du testeur — heure, tablette, extrait de log]
