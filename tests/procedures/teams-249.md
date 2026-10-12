# Procédure de Test — Équipes : identité = clé, liste verrouillée pendant une question (#249, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-11
**Testeur** : **l'utilisateur uniquement** — ni `qa` ni `deployer` n'exécutent cette procédure
**Contrats** : `contracts/models.md` §Identité d'une équipe, `contracts/game-state.md` §`TEAMS_LOCKED`, `contracts/websocket-actions.md` §UPDATE / §`TEAMS_UPDATE_REFUSED`, `contracts/logging.md` §3.2 bis
**Maquette** : `docs/mockup/v11.2/ui/admin_teams__lock_during_question.html`
**Tests automatiques** : `tests/INDEX.md` (lignes #249)

> **Contexte** : après le renommage d'une équipe, les réponses ARDOISE de l'équipe renommée s'affichaient « — » sur
> l'admin, l'anim et la TV (le nom stocké dans l'équipe n'était pas celui de la clé). Désormais le nom de l'équipe **est** sa clé,
> et la liste des équipes est **verrouillée pendant une question** (de sa sélection jusqu'à la révélation de la réponse).

## Prérequis

- [ ] Environnement : QUALIF (binaire `server-go/buzzcontrol-v11.2.x.y-windows-amd64.exe`)
- [ ] 3 équipes (ex. « Les Bleus », « Les Rouges », « Les Verts ») avec au moins 1 VJoueur (tablette) chacune ; 1 buzzer physique si possible
- [ ] Un quiz avec : 1 question **ARDOISE**, 1 **QCM**, 1 **MEMORY**, 1 **RAFALE** (si disponible)
- [ ] Postes ouverts : `/admin` (pages Équipes et Jeu), `/tv`, `/anim` (tablette animateur), 2 onglets admin pour l'étape de course
- [ ] Accès au fichier `data/logs/buzzcontrol.log` et à `data/files/teams.json` (selon l'emplacement de vos données)

---

## Scénario 1 — Le bug d'origine : renommer hors question, puis jouer l'ARDOISE

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Aucune question sélectionnée. Page **Équipes** → renommer « Les Bleus » en « Les Azur » (clic dans le nom, saisir, Entrée) | Le nom change ; les joueurs de l'équipe suivent ; le score est conservé | | |
| 2 | Lire `teams.json` | La clé « Les Azur » porte `"NAME": "Les Azur"` (jamais l'ancien nom) | | |
| 3 | Sélectionner la question ARDOISE, lancer, faire taper un mot par la tablette de « Les Azur » | La réponse apparaît **sous « Les Azur »** sur l'admin, l'anim, et — au REVEAL — sur la TV (**jamais « — »**) | | |
| 4 | Révéler | Les réponses de toutes les équipes sont affichées normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Réparation d'un `teams.json` ancien

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Arrêter le serveur. Éditer `teams.json` : `"Les Bleus": {"NAME": "Equipe 1", ...}` (NAME ≠ clé). Relancer | Le serveur démarre ; la page Équipes affiche « Les Bleus » | | |
| 2 | Relire `teams.json` | `NAME` vaut maintenant « Les Bleus » (fichier réécrit) | | |
| 3 | Lire `buzzcontrol.log` | `[WARN] TEAM_NAME_NORMALIZED … key="Les Bleus" … old_name="Equipe 1" … source=load` | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Équipes verrouillées pendant une question (maquette)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Page Équipes, aucune question : champs de nom et ✕ | Actifs, aucun bandeau | | |
| 2 | **Sélectionner** une question (ARDOISE) sans la lancer, retourner sur la page Équipes | Bandeau **« 🔒 Une question est en cours : renommer ou supprimer une équipe sera possible après la révélation de la réponse. »** ; tous les noms et tous les ✕ sont **grisés** ; au survol : *« Pendant une question, on ne peut ni renommer ni supprimer une équipe. Réessayez après la révélation de la réponse. »* | | |
| 3 | Lancer, stopper (STOPPED, pas encore révélée) | Toujours verrouillé | | |
| 4 | Pendant le verrou : **ajouter** une équipe « Les Oranges » ; **changer la couleur** d'une équipe ; **glisser un joueur** d'une équipe vers une autre | Tout fonctionne normalement | | |
| 5 | **Révéler** la réponse | Le bandeau disparaît, noms et ✕ redeviennent actifs | | |
| 6 | Sélectionner une autre question (sans la jouer) puis **NEW GAME** | Verrouillé après la sélection ; déverrouillé après NEW GAME | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Course : renommage envoyé juste avant le verrou, refus serveur

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Onglet admin A sur la page Équipes (non verrouillée). Dans l'onglet admin B, sélectionner une question | Le bandeau apparaît dans l'onglet A à la réception de l'état | | |
| 2 | Pour simuler la course : désactiver brièvement le réseau de l'onglet A, saisir un nouveau nom pour « Les Rouges », rétablir le réseau et valider | Message **« Renommage refusé : une question est en cours. « Les Rouges » n'a pas été renommée. »** ; le **nom d'origine** revient dans le champ | | |
| 3 | Lire `buzzcontrol.log` | `[WARN] TEAMS_CHANGE_REFUSED … teams=["Les Rouges"] …` | | |
| 4 | Vérifier l'onglet B et l'anim | Aucun message de refus (seul l'émetteur est prévenu) ; les équipes sont inchangées | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Renommage après la révélation : tout est reporté

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | ARDOISE jouée par 3 équipes, **révélée**, puis « Les Verts » **créditée** (+points) | — | | |
| 2 | Sans changer de question, page Équipes → renommer « Les Verts » en « Les Menthe » (REVEALED : permis) | Le renommage est accepté | | |
| 3 | Retour sur la page de jeu et sur l'anim | La réponse ARDOISE de l'équipe s'affiche toujours (sous « Les Menthe ») ; son **score est conservé** ; elle reste marquée **« déjà créditée »** (le geste de crédit ne se rouvre pas) | | |
| 4 | Lire `buzzcontrol.log` | `[INFO] TEAM_RENAMED old="Les Verts" new="Les Menthe" migrated=…` (ARDOISE_ANSWERS, HISTORY…) | | |
| 5 | Refaire avec une question **MEMORY** (une paire trouvée par « Les Menthe ») puis révélée, et **RAFALE** si disponible | La paire reste attribuée à l'équipe renommée ; les compteurs RAFALE de l'équipe suivent | | |
| 6 | Supprimer une équipe en REVEALED | Supprimée ; ses réponses/compteurs disparaissent ; les autres équipes sont inchangées | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 6 — Non-régression

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Hors question : ajouter, changer la couleur, glisser-déposer des joueurs, supprimer une équipe | Comme avant | | |
| 2 | Scores par équipe (page Scores) et ARDOISE d'une partie complète (liaison des tablettes, STOP, REVEAL) | Comme avant (#246) | | |
| 3 | Renommer une équipe puis relancer le serveur | Le nouveau nom persiste (clé = nom) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de validation

- [ ] Après un renommage, aucune réponse ARDOISE n'affiche « — » à tort (admin, anim, TV)
- [ ] Un `teams.json` ancien est réparé avec une trace
- [ ] Impossible de renommer/supprimer pendant une question (interface ET serveur), sans modification partielle
- [ ] Après la révélation, le renommage conserve scores, verrou « déjà créditée » et états de la question
- [ ] Aucun changement de comportement hors renommage / suppression

## Notes

[Observations du testeur]
