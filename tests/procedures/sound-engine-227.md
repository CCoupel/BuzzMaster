# Procédure de Test — Bruitage d'événement : vocabulaire, moteur abstrait et câblage (#227, milestone v11.0)

**Version** : v11.0.0.x (QUALIF)
**Date** : 2026-09-21
**Testeur** : QA / CDP (voir note ci-dessous)
**Contrat** : `contracts/sound.md`, amendement `contracts/lighting.md` §2.5
**Plan** : `_work/reports/plan-dev-227-20260921-102500.md`
**Vérification complémentaire** : `_work/reports/plan-verif-front-227-20260921-103500.md`

## Important — aucun son audible à ce stade, et c'est voulu

**#227 ne produit AUCUN son que vous puissiez entendre.** Ce n'est ni un oubli ni un
bug : c'est le critère de fin explicite du plan de développement (§1) — *"le moteur
est testable de bout en bout avec une sortie factice, sans aucun matériel, et aucun
son n'est encore audible."* Le pilote audio réel (`oto`, périphérique physique) est
`#228`. Les sons livrés (fichiers `.wav`) sont `#229`. L'interface d'administration
est `#230`.

**Conséquence pour cette procédure** : il n'y a **aucun scénario "j'entends un son"**
à valider ici. Tout ce qui suit se vérifie par les **journaux serveur** et par
l'exécution de la **suite automatisée** (`internal/audio`, `cmd/server`), jamais à
l'oreille. La validation auditive réelle appartiendra à la procédure de `#228`/`#229`.

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL (aucun matériel audio requis)
- [ ] Binaire buildé depuis la branche `milestone/v11.0` (ou merge ultérieur)
- [ ] Au moins 1 quiz avec buzzers physiques ou virtuels (VJoueur) configuré, comme
      pour toute session de jeu normale — aucun matériel audio requis
- [ ] Accès aux journaux serveur (`/ws/logs` ou fichier de log)

## Scénarios

### Scénario 1 — Démarrage/arrêt serveur sans configuration son (coût nul)

**Objectif** : vérifier qu'aucun comportement/log parasite n'apparaît au démarrage
alors que le son n'est pas configuré (cas #227 — aucun `Output` réel n'existe avant
#228, le moteur est donc toujours construit désactivé).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Démarrer le serveur (`./server.exe` ou binaire QUALIF) | Démarrage normal, aucune erreur | | |
| 2 | Consulter les logs serveur au démarrage | **Aucune** ligne mentionnant un appel matériel audio, aucune goroutine parasite signalée | | |
| 3 | Laisser tourner une partie complète normalement (QCM, buzz, révélation, points) | Le jeu se comporte **exactement comme avant #227** — aucune latence, aucun log d'erreur son | | |
| 4 | Arrêter le serveur (`curl -s http://localhost/shutdown`) | Arrêt propre, pas de blocage ni de panic | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Suite automatisée #227 (exécution QA)

**Objectif** : faire exécuter par QA la suite automatisée complète et consigner le
résultat — c'est le cœur de la validation de #227 (moteur + câblage + garde-fous),
aucune interface à observer.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | `cd server-go && go build ./...` | Compile sans erreur | | |
| 2 | `go test ./internal/audio/... -v` | 100% vert (paquet pur : vocabulaire, moteur, `FakeOutput`) | | |
| 3 | `go test ./internal/audio/... -race` | Vert — aucune race détectée (file bornée, goroutine unique, `PlayCue` non bloquant) | | |
| 4 | `go test ./cmd/server/... -run 'TestSoundSites|TestSoundNoRegression|TestSoundChain|TestSoundForbiddenFuncs227' -v` | 100% vert (un seul `t.Skip` documenté — timeout RAFALE, voir Scénario 5) | | |
| 5 | `go test ./cmd/server/... -run 'TestSoundSites|TestSoundChain' -race` | Vert — aucune race détectée | | |
| 6 | `go test ./cmd/server/... -run 'TestLEDSet|TestLEDBroadcast132|TestAmbianceExhaustiveness'` | Vert (non-régression LED/ambiance stricte, #227 n'y touche pas) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Non-régression lumineuse (KindCountdown = KindReady)

**Objectif** : confirmer par les logs que le rendu de la phase COUNTDOWN (3-2-1) est
resté visuellement identique à READY — #227 amende `contracts/lighting.md` pour
introduire `KindCountdown`/`KindTimeUp`, et une régression possible est que
l'éclairage change sans que personne ne l'ait demandé (la scène de décompte dédiée
appartient à #212, pas à #227).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Lancer une question normale (QCM ou SPEEDY), observer la salle pendant le décompte 3-2-1 puis au démarrage | Le rendu lumineux du décompte est **visuellement identique** à celui d'avant #227 (aucun changement perceptible) | | |
| 2 | `go test ./cmd/server/... -run TestSoundNoRegression -v` | Vert — preuve automatisée que `ambianceSceneFor(KindCountdown) == ambianceSceneFor(KindReady)` bit à bit | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Vérification volontaire du test-garde AST (une fois)

**Objectif** : confirmer que le test-garde des sites sonores rougit vraiment s'il est
contourné — build de confiance dans le filet de sécurité, pas un test de
non-régression à répéter à chaque QUALIF. Symétrique au Scénario 4 de
`tests/procedures/ambiance-lighting-205.md`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Dans une copie de travail jetable, ajouter temporairement dans `cmd/server/ambiance.go` un appel `a.notifySound(audio.CueGagne)` à l'intérieur de `runChronoPulse` (la pulsation 100 ms) | — | | |
| 2 | `go test ./cmd/server/... -run TestSoundSites_ForbiddenFamiliesNeverCallSoundFanOut -v` | **Échoue**, message nommant `runChronoPulse` et la frontière `contract §6.1` | | |
| 3 | Retirer l'appel ajouté (`git checkout -- server-go/cmd/server/ambiance.go`) | Le dépôt de travail retrouve son état propre | | |
| 4 | Relancer le test | Redevient vert | | |

**Note** : `TestSoundSites_ForbiddenCheck_CatchesInjectedViolation` et
`TestSoundSites_CatchesUnregisteredSite` (automatisés, `cmd/server/sound_sites_test.go`)
font déjà cette preuve sans toucher au dépôt — ce scénario manuel est une
contre-vérification ponctuelle, à faire une fois avant la clôture de #227, pas à
chaque QUALIF.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Point de vigilance connu : timeout RAFALE (couverture partielle)

**Objectif** : documenter explicitement une limite de couverture automatisée, pour
que QA sache où porter une attention manuelle supplémentaire.

`TestSoundChain_RafaleInvalidate_PlaysPerdu` couvre l'action explicite
RAFALE_INVALIDATE de bout en bout (cue `perdu` bien émise). Le **déclencheur**
"timeout d'une question RAFALE" (le chronomètre de la question expire sans action de
la régie) n'est PAS couvert par un test automatisé — seule l'action explicite l'est.
Les deux chemins appellent le même callback `OnRafaleInvalid`, donc la cue elle-même
est déjà vérifiée ; seul le déclenchement par expiration reste à couvrir
manuellement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Démarrer une manche RAFALE (mode CHACUN_SON_TOUR ou LIBRE), temps de question court (ex. 5s) | La manche démarre normalement | | |
| 2 | Ne rien faire — laisser le chronomètre de la question RAFALE expirer sans VALIDATE/INVALIDATE | La question suivante est automatiquement posée (comportement inchangé) | | |
| 3 | Consulter les journaux serveur juste après l'expiration | Aucune erreur, aucun log anormal — le passage à la question suivante se fait normalement (le son lui-même n'est pas audible à ce stade, voir note de périmètre en tête de fichier) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (coût nul sans configuration) : PASS
- [ ] Scénario 2 (suite automatisée) : PASS
- [ ] Scénario 3 (non-régression lumineuse COUNTDOWN) : PASS
- [ ] Scénario 4 (test-garde AST, une fois) : PASS
- [ ] Scénario 5 (timeout RAFALE, vigilance manuelle) : PASS
- [ ] Aucune régression constatée sur une partie QCM/MEMORY/MEMOTION/RAFALE normale
- [ ] **Rappel** : aucun son n'est audible à ce stade — ce n'est PAS un critère
      d'échec, c'est le périmètre attendu de #227 (voir note en tête de fichier)

## Notes QA

- Cette procédure ne couvre **aucun scénario auditif** (voir la note de périmètre en
  tête de fichier) — la validation manuelle utilisateur du son réel interviendra avec
  `#228`/`#229`.
- Tous les scénarios ci-dessus sont essentiellement des commandes `go test` plus
  quelques observations de logs/comportement de jeu déjà familier : ils peuvent être
  exécutés par `qa` (agent) sans navigateur, contrairement à une procédure
  fonctionnelle classique nécessitant un rendu visuel/auditif (règle projet : les
  scénarios manuels nécessitant un navigateur ou une oreille restent du ressort de
  l'utilisateur — ici, aucun des deux n'est nécessaire).
- Le Scénario 1 (logs au démarrage) nécessite d'observer le serveur réel — à la
  charge de `deployer`/QA selon la procédure QUALIF standard.
