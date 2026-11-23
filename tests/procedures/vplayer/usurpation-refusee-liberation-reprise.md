# Procedure de Test — Jeton de session des tablettes : usurpation refusée, libération puis reprise (LOT-2.2, #319)

**Version** : 13.0.0.x (QUALIF)
**Date** : 2026-11-23
**Testeur** : utilisateur (téléphones/tablettes réels + PC admin) — QA ne rejoue pas ces scénarios
**Références** : `contracts/vplayer-session.md` §2 à §7 — audit `_work/reports/security-20261123-170500.md` (faille M1)

## Prerequis

- [ ] Environnement : QUALIF, serveur Windows lancé, inscriptions ouvertes
- [ ] Matériel : 2 téléphones/tablettes VJoueur (A = Alice, B = Bob) + 1 PC avec les outils de développement du navigateur (console) + écran TV (`/tv`) + page admin et animateur
- [ ] Une partie avec au moins 2 équipes ; une question Picto prête (3 mots)
- [ ] **Outil d'attaque** (console du navigateur du PC, onglet quelconque du serveur) — remplacer `<ID>` par l'ID d'Alice lu sur `/tv` (clé de `bumpers` dans la trame d'état, ou `PICTO_DRAWER`) :

```js
const ws = new WebSocket(`ws://${location.host}/ws/player`);
ws.onmessage = e => console.log(e.data);
ws.onopen = () => ws.send(JSON.stringify({ ACTION: 'PLAYER_CONNECT', MSG: { NAME: 'Alice', ID: '<ID>' } }));
```

## Scenarios

### Scenario 1 — Le jeton est invisible et l'usage normal est inchangé

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire Alice (A) et Bob (B) depuis leurs appareils | Inscription normale, aucun écran ni champ nouveau | | |
| 2 | Console de A : lire `localStorage.vplayer_token` | Une chaîne de 43 caractères existe (jamais affichée à l'écran) | | |
| 3 | Couper puis rétablir le Wi-Fi de A | A se reconnecte seule, sans se réinscrire, et garde son score | | |
| 4 | Ouvrir la même page dans un second onglet de A (même navigateur) | Le second onglet fonctionne (même session) ; le plus récent l'emporte, l'ancien est informé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Usurpation refusée

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | A est connectée ; lancer l'outil d'attaque depuis le PC avec l'ID d'Alice, **sans jeton** | La console reçoit `PLAYER_REJECTED` avec `REASON: "INVALID_SESSION"` ; aucun autre message | | |
| 2 | Regarder A pendant l'attaque | A n'est **pas évincée** : pas de message, elle continue de jouer | | |
| 3 | Regarder la page admin (liste des joueurs) | Alice reste « connectée », aucun doublon ni changement d'état | | |
| 4 | Rejouer l'attaque avec un faux jeton (`TOKEN: 'abc'`) puis avec le jeton de Bob | Même refus `INVALID_SESSION`, aucun effet sur A ni B | | |
| 5 | Lire le journal temps réel (page Logs) | `VPLAYER_TOKEN_INVALID id=…` en avertissement ; **le jeton n'apparaît nulle part** | | |
| 6 | Lancer une manche Picto, Alice dessinatrice ; refaire l'attaque avec l'ID lu sur `/tv` (`PICTO_DRAWER`) | Refus ; l'attaquant ne reçoit ni mot, ni feuille ; Alice dessine toujours normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Appareil sans jeton : message clair et retour à l'inscription

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur un autre appareil (C), ouvrir la page joueur avec un stockage contenant l'ID d'Alice mais pas le jeton (console : `localStorage.vplayer_name='Alice'; localStorage.vplayer_session='1'; localStorage.vplayer_id='<ID>'` puis recharger) | Texte : « Cette place est déjà liée à un autre appareil. Demandez à l'animateur de libérer la place, puis reconnectez-vous avec votre nom. » | | |
| 2 | Attendre quelques secondes | Retour automatique à l'écran d'inscription (la session locale de C est effacée) ; le bouton de retour immédiat fonctionne | | |
| 3 | A (Alice légitime) | Toujours connectée, non évincée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Libération par l'animateur, puis reprise avec le nom

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Alice a des points ; l'animateur/admin libère la place d'Alice (#134) | A reçoit l'information de libération | | |
| 2 | Sur l'appareil C, s'inscrire avec le nom « Alice » (sans ID) | C reprend la place d'Alice, **score conservé** | | |
| 3 | Console de C : `localStorage.vplayer_token` | Un **nouveau** jeton (différent de celui noté au scénario 1) | | |
| 4 | Sur A, recharger la page avec l'ancien stockage | Refus `INVALID_SESSION` (l'ancien jeton ne vaut plus rien), texte du scénario 3 | | |
| 5 | Refaire le test pour une place **déconnectée** (couper A, libérer, reprendre par le nom — chemin #122) | Reprise acceptée ; ancien jeton mort ; nouveau jeton émis | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Migration : une tablette ouverte avant la mise à jour continue de jouer

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avant le déploiement : une tablette inscrite sur la version précédente (sans jeton), partie en cours | Elle joue normalement | | |
| 2 | Déployer la nouvelle QUALIF (le serveur redémarre) | La tablette se reconnecte d'elle-même en quelques secondes, **sans se réinscrire**, score conservé | | |
| 3 | Console de la tablette : `localStorage.vplayer_token` | Un jeton existe désormais (émis au premier retour) | | |
| 4 | Lancer l'outil d'attaque avec son ID | Refus `INVALID_SESSION` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Sauvegarde et restauration « Joueurs », buzzers physiques

**Validation** : HUMAIN — **Issues** : #319

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Sauvegarde : sauvegarder la catégorie « Joueurs », puis la restaurer | Restauration réussie | | |
| 2 | Recharger la page de A (avec son jeton) | A **reste valable** et se reconnecte sans réinscription | | |
| 3 | Lancer l'outil d'attaque avec l'ID d'Alice | Toujours refusé après la restauration | | |
| 4 | Avec un buzzer physique BuzzClick : l'allumer, le laisser se connecter, appuyer | Connexion et appui comme avant la mise à jour (aucun changement de firmware ni de protocole) | | |
| 5 | Supprimer un joueur depuis l'admin | Il disparaît ; son ID ne peut plus être rejoué | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Aucune usurpation possible avec l'ID seul
- [ ] Le joueur légitime n'est jamais évincé par une tentative refusée
- [ ] Le jeton n'apparaît ni à l'écran, ni dans la TV, ni dans les journaux
- [ ] Libération puis reprise par le nom : place retrouvée, score conservé
- [ ] Buzzers physiques inchangés
- [ ] Aucune régression sur l'inscription et la reconnexion dans les autres types de jeu (Ardoise, QCM, Memory, Roue)

## Notes QA

Automatique : `server-go/cmd/server/vplayer_session_319_test.go` (matrice §4.2, non-fuite), `web/src/pages/VPlayerPage.session319.test.jsx`,
`web/src/hooks/useWebSocket.session319.test.js`.
