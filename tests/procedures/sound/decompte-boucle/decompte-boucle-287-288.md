# Procédure de Test — Son de décompte du chronomètre et média de question en boucle (#287, #288)

**Version** : v11.4.0 (LOT-3)
**Date** : 2026-11-05
**Testeur** : QA (scénarios AUTO) / utilisateur (scénarios HUMAIN — **enceinte réelle obligatoire**)
**Contrats** : `contracts/sound.md` §12 (décompte), §13 (boucle) ; `contracts/http-endpoints.md` §Sound (`/api/sounds` tableau `long`, `/api/sounds/decompte*`), §Questions (`sound_loop`) ; `contracts/models.md` (`SOUND_LOOP`)
**Maquette** : `docs/mockup/v11.4.0/ui/admin_sounds_manager__countdown_loop.html` (§01 SoundsManager, §02 éditeur — **amendée** : plus d'exclusivité boucle/« À la fin »)
**Décisions utilisateur (2026-11-05)** : décompte **par manche** en RAFALE (jamais par question) · **au buzz tout continue** · **jamais de décompte si la question a un média** · boucle + « À la fin » combinables (chrono au **1ᵉʳ passage**) · « Son coupé » respecté · PAUSE/CONTINUE suspend/reprend.

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL) avec **une vraie sortie audio** (enceinte de la salle) ; page Ambiance › Son affichant la pastille « ● Son actif »
- [ ] « Bruitages activés » coché ; popover « Ambiance » : son **non coupé**
- [ ] Questions de test : 1 SPEEDY **sans média** (chrono 30 s) ; 3 SPEEDY **avec média** (un simultané, un « À la fin », un « En boucle ») ; 1 manche RAFALE (chrono de manche 60 s, réservoir ≥ 8 questions) ; 1 question MEMOTION avec carte à chrono de carte ≥ 10 s
- [ ] Un buzzer (physique ou simulé) et au moins une équipe
- [ ] Les tests automatiques du lot passent (`tests/INDEX.md`, lignes #287/#288) : cette procédure ne couvre que ce qu'un script ne peut pas juger (l'écoute)

## Scénarios

### Scénario 1 — Décompte en boucle (réglage par défaut) sur une question sans média

**Validation** : HUMAIN — **Issues** : #287

**Objectif** : le tic-tac démarre avec le chronomètre, tourne en boucle sans trou audible, et s'arrête avec lui.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ambiance › Son : ligne « Décompte du chronomètre » : « En boucle » sélectionné, Actif coché | Réglages par défaut | | |
| 2 | Lancer la question SPEEDY sans média, laisser passer le 3-2-1 | **Rien** pendant le 3-2-1 ; le tic-tac démarre au premier tic du chronomètre | | |
| 3 | Écouter 10 s | Tic-tac continu, régulier, sans coupure audible entre deux tours | | |
| 4 | Laisser le temps s'écouler à zéro | Tic-tac arrêté ; le bruitage « Temps écoulé » est audible (mixé, non coupé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Décompte « Une fois »

**Validation** : HUMAIN — **Issues** : #287

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ambiance › Son : basculer la ligne Décompte sur « Une fois » | Réglage enregistré (rechargement de la page : toujours « Une fois ») | | |
| 2 | Relancer la SPEEDY sans média | Le tic-tac est joué **une seule fois** (2 s) au démarrage du chrono, puis silence jusqu'à zéro | | |
| 3 | Revenir sur « En boucle » | Au chrono suivant, le tic-tac boucle de nouveau | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Jamais de décompte quand la question a un média

**Validation** : HUMAIN — **Issues** : #287, #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la question avec média **simultané** | Le média joue ; **aucun** tic-tac | | |
| 2 | Lancer la question avec média **« À la fin »** | Le média joue seul, le chrono démarre à sa fin ; **aucun** tic-tac, ni pendant le média ni après | | |
| 3 | Lancer la question avec média **en boucle** | Le média boucle ; **aucun** tic-tac | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Au buzz, tout continue

**Validation** : HUMAIN — **Issues** : #287, #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | SPEEDY sans média, décompte en boucle en cours : appuyer sur un buzzer | Le chrono se fige à l'écran ; le **tic-tac continue sans interruption** | | |
| 2 | Question avec média en boucle en cours : appuyer sur un buzzer | La **boucle continue** sans coupure | | |
| 3 | Terminer la question (STOP) | Le son s'arrête | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — PAUSE manuelle puis CONTINUE

**Validation** : HUMAIN — **Issues** : #287, #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Décompte en cours : cliquer PAUSE | Le tic-tac se **suspend** | | |
| 2 | Cliquer CONTINUE | Le tic-tac **reprend là où il en était** (pas de redémarrage à zéro audible) | | |
| 3 | Idem avec une question à média en boucle | Suspendu à PAUSE, repris à CONTINUE | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Manche RAFALE : une seule lecture pour toute la manche

**Validation** : HUMAIN — **Issues** : #287

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la manche RAFALE (chrono de manche 60 s) | Le tic-tac démarre au chrono de manche | | |
| 2 | Enchaîner 5 questions (valider, invalider — le bruitage « Erreur » est audible —, laisser expirer une question) | Le tic-tac **ne se relance pas et ne se coupe pas** à chaque question ; il se mixe avec les bruitages | | |
| 3 | Laisser la manche se terminer (temps écoulé) | Le tic-tac s'arrête à la fin de manche | | |
| 4 | Manche sans chrono de manche (temps = 0) | Aucun tic-tac (comportement attendu) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Carte MEMOTION

**Validation** : HUMAIN — **Issues** : #287

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant la phase de mémorisation | **Aucun** tic-tac | | |
| 2 | Retourner une carte à chrono | Le tic-tac démarre au retournement | | |
| 3 | Laisser le chrono de carte expirer | Le tic-tac s'arrête à l'expiration (la phase reste en cours, l'animateur agit) | | |
| 4 | Retourner une autre carte et la résoudre avant la fin du chrono | Le tic-tac s'arrête à la résolution | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Son coupé, décompte désactivé, bruitages désactivés

**Validation** : HUMAIN — **Issues** : #287

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Décompte en cours : couper le son depuis le popover « Ambiance » | Le tic-tac s'arrête immédiatement ; au chrono suivant, rien | | |
| 2 | Rétablir le son pendant un chrono déjà lancé | Rien n'est repris ; au chrono suivant le tic-tac revient | | |
| 3 | Décompte en cours : décocher « Actif » sur la ligne Décompte | Arrêt immédiat ; aucun tic-tac aux chronos suivants ; les bruitages jouent toujours | | |
| 4 | Décocher « Bruitages activés » | Aucun tic-tac (ni bruitage) | | |
| 5 | Ligne Décompte désactivée et son coupé : cliquer « Tester » | Le test **joue quand même** une fois (geste explicite) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Remplacer, restaurer, tester le fichier de décompte

**Validation** : HUMAIN — **Issues** : #287

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Remplacer » par un WAV de ~5 s | Badge « Personnalisé », durée ~5,00 s, bouton « Restaurer » visible | | |
| 2 | « Tester » → « Jouer sur l'enceinte » | Le fichier personnalisé est entendu **une fois** | | |
| 3 | Lancer une question : le décompte | C'est le fichier personnalisé qui boucle | | |
| 4 | Tenter de remplacer par un WAV > 30 s, puis par un fichier non WAV | Refus nommé (message du serveur) ; fichier actuel inchangé | | |
| 5 | « Restaurer » | Retour au tic-tac de 2,00 s, badge « Défaut » | | |
| 6 | « Restaurer les sons par défaut » (après avoir remplacé un bruitage et le décompte) | Les 7 bruitages **et** le décompte sont redevenus « Défaut » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 10 — Média de question en boucle (#288)

**Validation** : HUMAIN — **Issues** : #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : question avec média, ligne « Lecture » sur « En boucle » (conseil « rejoué jusqu'à la fin du chronomètre ») ; enregistrer, rouvrir | La ligne reflète « En boucle » après rechargement | | |
| 2 | Lancer la question | Le média joue et **reboucle** sans trou audible jusqu'à la fin du chrono | | |
| 3 | Laisser le temps s'écouler à zéro | La boucle s'arrête avec « Temps écoulé » | | |
| 4 | Relancer, puis cliquer STOP ; relancer, puis REVEAL ; relancer, puis question suivante | Dans chaque cas la boucle s'arrête et **ne survit pas** à la question | | |
| 5 | « Rejouer » le son pendant la question | Repart au début, toujours en boucle | | |
| 6 | Même question avec « Lecture » sur « Une fois » | Un seul passage, puis silence (comportement v11.1) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 11 — Boucle + « Chrono démarre : à la fin » (#288)

**Validation** : HUMAIN — **Issues** : #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : activer « En boucle » **et** « À la fin » | Aucun avertissement d'exclusivité ; conseil « Le chronomètre démarre à la fin du premier passage ; le son continue en boucle. » | | |
| 2 | Lancer la question | Le média joue, le chrono attend ; **à la fin du 1ᵉʳ passage** le chrono démarre | | |
| 3 | Écouter la suite | Le média **continue en boucle** pendant le chrono ; aucun tic-tac de décompte | | |
| 4 | Laisser le temps s'écouler | La boucle s'arrête à zéro | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 12 — Réglages persistés et contrat HTTP (équivalent automatisé)

**Validation** : AUTO — **Issues** : #287, #288

**Objectif** : exécutable par QA sans enceinte ; l'équivalent automatisé est dans `tests/INDEX.md` (`internal/server/sound_countdown_287_test.go`, `question_sound_loop_288_test.go`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `curl -s http://localhost/api/sounds` | `cues` : 7 entrées ; `long` : 1 entrée `decompte` (`kind: countdown`, `enabled: true`, `loop: true`, `custom: false`, ~2.0 s) | | |
| 2 | `curl -s -X POST localhost/config.json -d '{"sound":{"countdown_once":true}}'` puis relire `/api/sounds` | `loop: false`, `muted`/`cues_disabled` inchangés | | |
| 3 | `POST /api/sounds/decompte/restore` puis `restore-defaults` | `custom: false` ; `restored` liste 8 noms | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 13 — Rendu de l'interface vs maquette

**Validation** : HUMAIN — **Issues** : #287, #288

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ambiance › Son, comparer à la maquette §01 | Séparateurs « Bruitages (impulsions, ≤ 5 s) » / « Sons longs (≤ 30 s) », ligne Décompte, colonne Lecture, textes d'aide conformes | | |
| 2 | Éditeur de question (SPEEDY avec son), comparer à la maquette §02 amendée | Ligne « Lecture » sous « Chrono démarre », libellés et conseils lisibles ; pas de mention « au buzz » dans le conseil | | |
| 3 | Éditeur : question sans son | Ligne « Lecture » visible mais désactivée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux passent
- [ ] Aucun tic-tac sur une question à média, ni pendant 3-2-1 ou la mémorisation
- [ ] Au buzz rien ne s'interrompt ; PAUSE suspend, CONTINUE reprend
- [ ] Les 7 bruitages et le média de question hors boucle se comportent comme en v11.4.0 LOT-2

## Notes QA

[Observations : trou audible entre deux tours de boucle sur le Raspberry Pi → à remonter (optimisation interne prévue, contrat inchangé)]
