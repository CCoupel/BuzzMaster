# Procédure de Test — Fermeture de la course d'inscription VJoueur (#120)

**Version** : à définir (branche `bugfix/vplayer-enroll-redirect`)
**Date** : 2026-07-28
**Branche** : bugfix/vplayer-enroll-redirect
**Testeur** : QA

---

## Contexte du Bug

Un VJoueur accepté à l'inscription (fiche créée, visible côté admin/TV) pouvait être renvoyé
silencieusement à l'écran d'inscription quelques instants plus tard, sans aucun message. En se
réinscrivant avec le même pseudo, il se voyait refuser « ce pseudo est déjà utilisé » — à propos
de son propre pseudo.

**Cause racine (plan `_work/reports/plan-20260728-101500.md`)** : `VPlayerPage` montait avant
l'arrivée de l'`UPDATE` listant le bumper fraîchement créé. Le roster local était donc, par
construction, encore celui d'avant l'inscription. Une ancienne détection déduisait « supprimé par
l'animateur » de la seule absence du bumper dans ce roster — vrai uniquement pour le tout premier
inscrit (roster vide, garde-fou existant), faux pour tous les suivants dès qu'un autre VJoueur ou
un buzzer physique était déjà présent. D'où le caractère « parfois » du bug, plus fréquent en
soirée chargée.

**Fix attendu** :
- Le serveur notifie désormais explicitement l'éviction (`PLAYER_EVICTED{REASON}`) — l'absence
  d'un bumper dans un roster n'est plus jamais, à elle seule, un motif de renvoi.
- L'identité locale se vérifie par ID (repli sur le nom pour une session antérieure à cette
  version).
- Un état d'attente visuel (« Connexion à la partie… ») remplace l'écran de jeu incomplet pendant
  la fenêtre entre l'inscription et la première mise à jour listant le joueur.
- Un renvoi légitime (suppression admin, nouvelle partie) affiche désormais un message expliquant
  le motif, et n'empêche plus de se réinscrire avec le même pseudo.

Maquette de référence : https://claude.ai/code/artifact/fa51d314-8508-4829-bfb3-8e997896f60f

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vplayer-enroll-redirect`)
- [ ] Au moins 5 appareils/onglets distincts pouvant ouvrir `/player` (smartphones ou onglets
      séparés — le bug dépend d'un roster non vide au moment de l'inscription)
- [ ] Accès admin (`/` ou `/game`) pour supprimer un joueur et lancer une nouvelle partie
- [ ] Un buzzer physique déjà appairé (pour le scénario avec zéro VJoueur mais roster non vide)
- [ ] Accès au terminal serveur pour l'arrêter/relancer (scénario B2)

---

## Scénario 1 — Reproduction : un second joueur ne doit plus être renvoyé

**Objectif** : Vérifier que la course est fermée — le cas le plus simple qui la déclenchait avant
le fix.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir les inscriptions (admin) | Écran d'inscription accessible sur `/player` | | |
| 2 | Depuis l'appareil A, s'inscrire avec un pseudo (ex. « Alice ») | Alice rejoint l'écran de jeu normalement (roster vide au départ, cas non affecté même avant le fix) | | |
| 3 | Depuis l'appareil B (autre appareil), s'inscrire avec un second pseudo (ex. « Bob ») | Bob voit brièvement « Connexion à la partie… » puis l'écran de jeu s'affiche normalement — **aucun retour à l'inscription** | | |
| 4 | Vérifier côté admin | Les deux joueurs (Alice et Bob) apparaissent, chacun avec sa fiche | | |

**Verdict** : [ ] PASS  [ ] FAIL

### Variante — buzzer physique déjà connecté, zéro VJoueur

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un buzzer physique est déjà appairé et connecté, aucun VJoueur encore inscrit | Roster non vide (le buzzer), mais 0 VJoueur | | |
| 2 | Inscrire un VJoueur (premier VJoueur, mais roster déjà non vide à cause du buzzer) | Le VJoueur n'est PAS renvoyé — ce cas à lui seul suffisait à armer la course avant le fix (le garde-fou « roster vide » ne portait que sur les VJoueurs) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Cinq inscriptions en rafale

**Objectif** : Confirmer l'absence de course avec plusieurs inscriptions quasi simultanées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis 5 appareils distincts, s'inscrire avec 5 pseudos différents le plus rapidement possible (à quelques secondes d'intervalle) | Les 5 rejoignent l'écran de jeu, aucun n'est renvoyé à l'inscription | | |
| 2 | Vérifier côté admin | 5 cartes joueurs distinctes, 5 ID différents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Suppression depuis l'admin : motif affiché puis retour

**Objectif** : Vérifier qu'une éviction réelle (décision de l'animateur) est bien notifiée avec
son motif, contrairement à un renvoi silencieux.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un joueur (ex. « Alice ») est inscrit et en jeu | Écran de jeu normal côté Alice | | |
| 2 | Depuis l'admin, supprimer la fiche d'Alice | Sur l'appareil d'Alice : message « Ta place a été libérée par l'animateur. Tu peux te réinscrire. » affiché quelques secondes | | |
| 3 | Attendre (ou cliquer « Rejoindre à nouveau ») | Retour automatique à l'écran d'inscription, sans action manuelle nécessaire | | |
| 4 | Observer l'écran d'inscription | Un bandeau reprend le même message (« Ta place a été libérée… ») au-dessus du formulaire | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Réinscription avec le même pseudo après suppression

**Objectif** : Vérifier la levée de la cascade E du plan — plus de `NAME_TAKEN` sur son propre
pseudo après un renvoi légitime.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Suite au scénario 3, l'appareil d'Alice est revenu à l'écran d'inscription | Bandeau « place libérée » visible | | |
| 2 | Ressaisir exactement le même pseudo « Alice » et valider | L'inscription est **acceptée** — pas de message « pseudo déjà utilisé » | | |
| 3 | Vérifier côté admin | Une nouvelle fiche « Alice » apparaît, aucune fiche orpheline résiduelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Nouvelle partie : tous les VJoueurs reçoivent le motif

**Objectif** : Vérifier la notification `GAME_RESET` pour chaque VJoueur purgé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 VJoueurs sont inscrits et en jeu | Écran de jeu normal sur les deux appareils | | |
| 2 | Depuis l'admin, lancer une nouvelle partie (NEW_GAME) | Les deux appareils affichent « Une nouvelle partie a commencé. Tous les joueurs doivent se réinscrire. » quasi simultanément | | |
| 3 | Attendre le retour automatique | Les deux appareils reviennent à l'écran d'inscription avec le bandeau correspondant | | |
| 4 | Réinscrire les deux joueurs | Acceptés normalement, y compris avec leurs pseudos précédents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Coupure réseau puis retour : reconnexion normale

**Objectif** : Non-régression #109 — la reconnexion après coupure ne doit pas être affectée par
le changement d'identité par ID ni par le nouvel état d'attente.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un joueur est inscrit et en jeu | Écran de jeu normal | | |
| 2 | Couper le réseau de son appareil (mode avion) quelques secondes | Côté admin, badge de connexion passe orange puis éventuellement rouge | | |
| 3 | Rétablir le réseau | L'appareil affiche brièvement « Connexion à la partie… » (F5) puis retrouve l'écran de jeu normal, sans passer par l'écran d'inscription | | |
| 4 | Vérifier côté admin | Le badge de connexion revient à l'état normal (masqué), le score/l'équipe du joueur sont intacts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Redémarrage serveur après inscriptions en rafale (couverture B2)

**Objectif** : Vérifier que la sauvegarde atomique du roster (`SaveBumpers`) résiste à des
inscriptions concurrentes — aucune perte de joueur après redémarrage.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire 5 joueurs en rafale depuis 5 appareils (comme scénario 2) | 5 joueurs visibles côté admin | | |
| 2 | Arrêter le serveur (`/shutdown` ou Ctrl+C) puis le relancer immédiatement | Redémarrage sans erreur dans les logs | | |
| 3 | Observer le roster au redémarrage | Les 5 joueurs sont rechargés depuis `bumpers.json`, aucun manquant, aucune fiche corrompue | | |
| 4 | Vérifier le fichier `data/bumpers.json` (ou équivalent) sur le disque | Fichier JSON valide, lisible, pas de fichier `.tmp-*` résiduel à côté | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Absence d'affichage incomplet à l'arrivée sur l'écran de jeu (F5)

**Objectif** : Vérifier qu'aucun écran de jeu à moitié rendu (nom vide, boutons manquants)
n'apparaît jamais entre l'inscription et l'affichage complet.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | S'inscrire depuis un appareil avec un réseau volontairement un peu lent (limitation débit navigateur, ou simplement observer attentivement) | Un état d'attente clair (« ⏳ Connexion à la partie… ») s'affiche, jamais un écran de jeu vide ou à moitié construit | | |
| 2 | Attendre la bascule vers l'écran de jeu complet | Transition nette : nom du joueur, couleur d'équipe et zone de jeu apparaissent tous ensemble | | |
| 3 | Répéter 3-4 fois avec des inscriptions successives | Comportement identique à chaque fois | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Un joueur accepté n'est jamais renvoyé à l'inscription dans la foulée (scénarios 1, 2)
- [ ] Le cas « buzzer physique déjà connecté, zéro VJoueur » n'arme plus la course (scénario 1 variante)
- [ ] Une éviction réelle affiche son motif avant de renvoyer à l'inscription (scénario 3)
- [ ] Le bandeau de motif est visible sur l'écran d'inscription après un renvoi (scénarios 3, 5)
- [ ] Un joueur supprimé peut se réinscrire avec le même pseudo sans `NAME_TAKEN` (scénario 4)
- [ ] Une nouvelle partie notifie chaque VJoueur individuellement (scénario 5)
- [ ] La reconnexion après coupure réseau reste fonctionnelle (non-régression #109, scénario 6)
- [ ] Le roster survit à un redémarrage serveur après inscriptions concurrentes (scénario 7)
- [ ] Aucun affichage incomplet n'apparaît jamais sur l'écran de jeu VJoueur (scénario 8)
- [ ] Aucune régression sur les refus existants (pseudo pris à la soumission initiale, partie
      complète, inscriptions fermées, pseudo invalide)

## Notes QA

[Espace pour observations]
