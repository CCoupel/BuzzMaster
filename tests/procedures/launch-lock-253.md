# Procédure de Test — « Lancer la partie » : menu Préparation, popup plein écran, verrou serveur (#253, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-11
**Testeur** : **l'utilisateur uniquement** (plusieurs postes, tablette, coupure réseau : ni `qa` ni `deployer` ne l'exécutent)
**Contrats** : `contracts/websocket-actions.md` §HELLO_ACK, §LAUNCH_LOCK, §LAUNCH_LOCK_REFUSED / ACTION_REFUSED ; `contracts/game-state.md` §LAUNCH_LOCK ; `contracts/logging.md` §3.2 sexies
**Maquettes** : `docs/mockup/v11.2/ui/admin_navbar__launch_game_entry.html`, `docs/mockup/v11.2/ui/admin_new_game__launch_overlay.html`
**Tests automatiques** : `tests/INDEX.md` (lignes #253)

> **Sauvegarder** `data/` avant : le scénario 7 lance réellement une nouvelle partie. Les postes sont : **A** = régie (`/admin`), **B** = second navigateur admin (`/admin`, autre onglet ou autre machine), **C** = tablette animateur (`/anim`), **T** = TV (`/tv`), **P** = au moins une tablette joueur.

## Prérequis

- [ ] QUALIF, partie en cours avec une question lancée, des scores non nuls
- [ ] Postes A, B, C, T, P ouverts en même temps
- [ ] `data/logs/buzzcontrol.log` accessible (niveau WARN minimum par défaut)

---

## Scénario 1 — L'entrée est dans le menu Préparation, et nulle part ailleurs

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A, écran large : regarder la barre de navigation | Groupe **Préparation** : 👥 Joueurs, ❓ Quiz, 🎭 Backstage, **🚀 Lancer la partie** (dans cet ordre) | | |
| 2 | Rétrécir la fenêtre jusqu'au menu unique « Préparation » ; l'ouvrir | « 🚀 Lancer la partie » entre Backstage et la section INTERFACE | | |
| 3 | Survoler l'entrée | Info-bulle « Choisir ce qu'il faut effacer et relancer la partie » | | |
| 4 | Page **Jeu** | **Plus de bouton** « LANCER LA PARTIE » dans l'en-tête | | |
| 5 | Page **Backstage** | **Plus de bouton** « LANCER LA PARTIE » | | |
| 6 | L'entrée est-elle disponible dans toutes les phases (STOPPED, STARTED, REVEALED, PAUSED) ? | Oui | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — La popup couvre toute la page

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A, page **Jeu** : cliquer « 🚀 Lancer la partie » | La popup s'affiche **par-dessus toute la page** avec un fond assombri : la barre de navigation, la colonne Questions, la colonne Équipes **et l'aperçu TV** sont derrière le fond | | |
| 2 | Vérifier que la popup **n'est pas rognée** ni limitée au carré de l'aperçu TV | Popup entière et centrée dans la fenêtre | | |
| 3 | Cliquer sur le fond assombri / sur une carte d'équipe derrière | Rien ne se passe (l'écran est bloqué) ; la popup reste | | |
| 4 | Contenu | Identique à la validation #243 : 4 cases (Scores + Questions cochées), avertissements, « Lancer sans rien effacer » si tout est décoché | | |
| 5 | Redimensionner la fenêtre (étroite, haute) | La popup reste dans l'écran (défilement interne si nécessaire) | | |
| 6 | Touche **Échap** | La popup se ferme | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Les autres postes sont suspendus (pour tout le monde)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A : ouvrir la popup et **la laisser ouverte** | | | |
| 2 | Poste B : regarder la page | Bandeau « 🔒 Lancement d'une nouvelle partie en cours sur un autre poste : actions de jeu suspendues. » ; START/STOP/RÉPONSE, attributions de points et édition des équipes **grisés** | | |
| 3 | Poste B : menu Préparation | Entrée « 🚀 Lancer la partie » **grisée** (info-bulle : déjà ouverte sur un autre poste) | | |
| 4 | Poste C (anim) | Bandeau « …sur le poste régie : conduite suspendue. » ; boutons de conduite grisés « suspendu » ; attribution de points impossible | | |
| 5 | Poste C : forcer un geste (taper un bouton grisé, ou donner des points) | Rien n'est appliqué ; message court « Action suspendue » | | |
| 6 | Poste A (propriétaire) : tenter START / un crédit **pendant que la popup est ouverte** | Impossible aussi (écran bloqué par la popup) | | |
| 7 | Poste T (TV) et poste P (tablette joueur) | **Aucun** changement visuel (pas de bandeau) | | |
| 8 | Poste P : taper une réponse ARDOISE ou répondre à un QCM / buzzer | **Fonctionne** (les actions des joueurs ne sont pas suspendues) | | |
| 9 | `buzzcontrol.log` | `LAUNCH_LOCK_OPENED` (INFO) puis, par action tentée, `ACTION_REFUSED_LAUNCH client=… type=… action=…` (WARN) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Un seul poste à la fois

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A : popup ouverte. Poste B : essayer d'ouvrir « Lancer la partie » (entrée grisée, ou via une page non rafraîchie) | Pas de popup ; message « Une fenêtre « Lancer la partie » est déjà ouverte sur un autre poste » | | |
| 2 | `buzzcontrol.log` | WARN `LAUNCH_LOCK_REFUSED … reason=LOCKED_BY_OTHER` | | |
| 3 | Poste A : « Annuler » | Popup fermée ; poste B et poste C retrouvent leurs commandes immédiatement (bandeaux disparus) | | |
| 4 | Poste B : ouvrir à son tour | La popup s'ouvre ; le bandeau apparaît maintenant sur A et C | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Entracte

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A : lancer une ENTRACTE | | | |
| 2 | Cliquer « 🚀 Lancer la partie » | Message « Impossible pendant une entracte : sortez d'abord de l'entracte. » ; **pas de popup** | | |
| 3 | Sortir de l'entracte, recliquer | La popup s'ouvre | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Le verrou ne bloque jamais la partie plus de 45 s

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A : ouvrir la popup et **ne rien faire pendant 2 minutes** (onglet visible, réseau normal) | Popup **toujours ouverte**, le poste B reste suspendu (le poste A renouvelle toutes les 15 s) | | |
| 2 | Poste A : popup ouverte, puis **couper le réseau du poste A** (mode avion / débrancher) | Au bout de ~45 s, les postes B et C **retrouvent leurs commandes** (bandeaux disparus) | | |
| 3 | `buzzcontrol.log` | WARN `LAUNCH_LOCK_RELEASED … reason="disconnect"` (coupure détectée) ou `reason="expired"` (45 s sans signe de vie) avec `held_ms` | | |
| 4 | Rebrancher le poste A (la popup était restée affichée) | « La fenêtre a expiré (plus de 45 s sans signe de vie de ce poste) : rouvrez « Lancer la partie ». » ; le bouton de lancement est **désactivé** | | |
| 5 | Poste A : popup ouverte, **fermer l'onglet** puis rouvrir /admin | Le verrou est levé immédiatement (B et C libres) ; WARN `reason="disconnect"` | | |
| 6 | Poste A : popup ouverte, **recharger la page** (F5) | Idem : verrou levé, popup fermée | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 7 — Lancement effectif

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Poste A : popup (défauts), « Lancer la partie » | Envoi (« ⏳ Lancement de la partie… »), popup fermée, partie remise à zéro (scores, historique, questions) | | |
| 2 | Postes B et C | Bandeaux disparus aussitôt ; commandes de nouveau actives | | |
| 3 | `buzzcontrol.log` | `LAUNCH_LOCK_RELEASED … reason="new_game"` (INFO) ; `NEW_GAME_PURGE …` (WARN) | | |
| 4 | Poste B : pendant qu'un poste A a la popup ouverte, envoyer un NEW_GAME (autre onglet obsolète) | Refusé : `NEW_GAME_REFUSED` motif LAUNCH_IN_PROGRESS ; rien n'est modifié | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 8 — Accessibilité

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Clavier seul : ouvrir le menu Préparation, atteindre l'entrée, Entrée | La popup s'ouvre, le **focus est dans la popup** | | |
| 2 | Tab / Maj+Tab | Le focus reste dans la popup | | |
| 3 | Échap puis vérifier le focus | Il revient sur l'entrée du menu | | |
| 4 | Lecteur d'écran (si disponible) | Titre « Lancer la partie » annoncé (dialogue modal) ; les erreurs et l'expiration sont annoncées | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] L'entrée n'existe que dans le menu Préparation
- [ ] La popup couvre toute la page, y compris l'aperçu TV
- [ ] Tant qu'elle est ouverte, aucun poste (régie, 2ᵉ admin, anim) ne peut conduire le jeu ; les joueurs, eux, continuent
- [ ] Une popup oubliée, un onglet fermé ou un poste coupé ne bloquent jamais la partie plus de 45 s
- [ ] Journal : OPENED / RELEASED (INFO ou WARN selon le motif) / REFUSED / ACTION_REFUSED_LAUNCH

## Notes

[Observations]
