# Procédure de Test — Pilote de sortie audio (#228, milestone v11.0)

**Version** : v11.0.0.x (QUALIF)
**Date** : 2026-09-21
**Testeur** : Utilisateur, sur matériel réel (Windows **et** Raspberry Pi obligatoires)
**Contrat** : `contracts/sound.md` (amendement 2026-09-21 : `Play` DOIT bloquer)
**Plan** : `_work/reports/plan-dev-228-229-20260921-114500.md` Partie 1

## Important — ce qui est réellement audible à ce stade, et ce qui ne l'est pas encore

**#228 livre le pilote (le tuyau), pas les sons (#229 livre le contenu).** Le moteur
(#227) n'a toujours pas de banque de sons : chaque cue envoie au pilote un
**paravent** — le nom de la cue lui-même en texte brut (ex. `"depart"`, 6 octets),
interprété comme s'il s'agissait de PCM 16 bits stéréo. Concrètement, sur du
matériel réel :

- **Un événement de jeu isolé (`depart`, `gagne`, …) ne produira PAS un son
  distinct et reconnaissable** — le paravent ne représente qu'une fraction de
  milliseconde d'audio (quelques octets à 44100 Hz stéréo), le plus souvent
  inaudible ou perçu comme un micro-clic sans caractère.
- **Le pré-armement du flux (`prime`, au démarrage) joue 200 ms de silence
  numérique** — volontairement silencieux, pour absorber le coût d'ouverture du
  flux avant le premier vrai son, pas pour être entendu.

**Ce que cette procédure valide donc réellement** : que le **pilote atteint le
matériel** (contexte créé, flux ouvert, aucune erreur, aucun crash, aucun blocage
du jeu), pas encore le **caractère** de chaque bruitage — cette dernière validation
appartient à la procédure de `#229`, une fois les vrais fichiers `.wav` livrés. Ne
soyez donc pas surpris de n'entendre presque rien pendant cette procédure : c'est le
signal recherché, vérifié par les journaux plutôt qu'à l'oreille pour l'essentiel des
scénarios ci-dessous.

## Prérequis

- [ ] Binaire buildé depuis la branche `milestone/v11.0` (ou merge ultérieur), pour
      **Windows** ET pour **Raspberry Pi (Linux/ARM64)**
- [ ] Accès aux journaux serveur (`/ws/logs` ou fichier de log) sur les deux machines
- [ ] `config.json` : section `sound.enabled = true` (désactivé par défaut) sur les
      deux machines
- [ ] Un haut-parleur ou une enceinte Bluetooth appairée sur chaque machine
- [ ] Au moins 1 quiz avec buzzers physiques ou virtuels (VJoueur) configuré, comme
      pour toute session de jeu normale

## Scénarios

### Scénario 1 — Windows : le pilote atteint le périphérique sans erreur

**Objectif** : vérifier que le contexte audio se construit et devient disponible sur
Windows, sans jamais bloquer ni faire planter le serveur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Démarrer le serveur Windows avec `sound.enabled=true` | Démarrage normal, pas de blocage prolongé | | |
| 2 | Consulter les journaux au démarrage | **Aucune** ligne "sound bruitage disabled" / "oto context degraded" — le contexte doit devenir prêt normalement | | |
| 3 | Lancer une question, faire crédit de points à une équipe (`gagne`), révéler (`reveal`) | Le jeu se déroule normalement, **sans latence perceptible** liée au son (contract §5 : `PlayCue` n'attend jamais le pilote) | | |
| 4 | Consulter les journaux après ces événements | Aucune ligne d'erreur, aucun `PlayErrors` anormal (voir Scénario 4 pour la mesure précise) | | |
| 5 | Arrêter le serveur (`curl -s http://localhost/shutdown`) | Arrêt **rapide**, sans attendre la fin d'un son en cours (voir aussi Scénario 3) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Raspberry Pi : première vérification réelle de l'hypothèse systemd

**Objectif** : `#227`/`#228` ont fait un pari documenté, **jamais vérifié sur Pi
réel** (`contracts/sound.md` note de traçabilité) : l'unité systemd *system* doit
exporter `XDG_RUNTIME_DIR`/`PULSE_SERVER` pour joindre le `pipewire-pulse` de la
session utilisateur. **C'est le scénario le plus important de cette procédure.**

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Déployer le binaire Linux/ARM64 sur le Raspberry Pi, unité systemd *system* (`docs/ADMIN_GUIDE.md`), avec `XDG_RUNTIME_DIR`/`PULSE_SERVER` exportés selon la documentation `#228` | — | | |
| 2 | `loginctl enable-linger <user>` appliqué (sans quoi `/run/user/<uid>` n'existe pas sans session ouverte) | — | | |
| 3 | Démarrer/redémarrer le service (`systemctl restart buzzcontrol`), `sound.enabled=true` | Démarrage normal | | |
| 4 | Consulter les journaux (`journalctl -u buzzcontrol` ou `/ws/logs`) | **Le contexte devient prêt** — aucune ligne "PulseAudio client initialization failed" / "ALSA error" / "oto context degraded" | | |
| 5 | **Si l'étape 4 échoue** : documenter le message d'erreur exact ici, et vérifier `loginctl show-user <user>` (`Linger=yes` attendu) et l'existence de `/run/user/<uid>` | Cause identifiée | | |
| 6 | Lancer une question, déclencher plusieurs événements de jeu | Aucune dégradation du jeu, quelle que soit l'issue de l'étape 4 (dégradation silencieuse garantie même en cas d'échec) | | |

**Note** : si cette vérification échoue, **la correction reste confinée au pilote et
à la documentation de déploiement** (par conception, contract §8/§228 plan §1.2) —
ce n'est pas un blocage pour la suite de la QUALIF, mais un retour à consigner
précisément (message d'erreur exact, sortie de `loginctl show-user`) pour corriger le
pilote #228 sans toucher au moteur #227.

**Verdict** : [ ] PASS  [ ] FAIL  [ ] ÉCHEC DOCUMENTÉ (hypothèse systemd fausse — voir notes)

---

### Scénario 3 — Enceinte Bluetooth éteinte puis rallumée en cours de partie

**Objectif** : vérifier le ré-armement et la dégradation silencieuse (risque R.5 du
plan) — la déconnexion d'une enceinte ne doit jamais remonter d'erreur dans le jeu
ni bloquer le serveur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Partie en cours, `sound.enabled=true`, enceinte Bluetooth appairée et connectée | — | | |
| 2 | Éteindre l'enceinte (ou couper le Bluetooth) **en cours de partie** | Le jeu continue **normalement** — aucun gel, aucune erreur visible côté buzzers/interface | | |
| 3 | Déclencher plusieurs événements de jeu (points, révélation) pendant que l'enceinte est éteinte | Consulter les journaux : dégradation journalisée (compteur d'erreurs), **jamais remontée au jeu** | | |
| 4 | Rallumer l'enceinte / reconnecter le Bluetooth | Le pilote se ré-arme — journaux sans blocage | | |
| 5 | Déclencher un nouvel événement de jeu | Le jeu continue de fonctionner normalement, que le pilote ait pu se ré-armer ou non | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Latence perçue et durée réelle mesurées par les journaux/compteurs

**Objectif** : le plan (Partie 0) prévient qu'avec de vrais sons, `Play` bloquera
pour la durée réelle du son, et la file se vide **séquentiellement** — une rafale de
cues s'étale au lieu de se superposer. Avant que `#229` ne livre de vrais sons, cette
mesure se fait sur le paravent actuel (quelques microsecondes), mais la **méthode**
doit être en place dès maintenant pour être réutilisée telle quelle en `#229`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Lancer une question SPEEDY, faire buzzer 4-5 fois d'affilée (rafale de `PAUSE`/`CONTINUE`, donc de crédits `gagne`/`perdu` rapprochés) | Le jeu répond **instantanément** à chaque action régie — aucune latence perceptible, même si le pilote joue en séquence en arrière-plan (contract §5.1 : `PlayCue` n'attend jamais) | | |
| 2 | Consulter les journaux / `Engine.Stats()` (si exposé par un endpoint de diagnostic, sinon journaux) après la rafale | `Accepted`/`Played`/`Dropped` cohérents avec le nombre d'événements déclenchés ; aucun `Dropped` inattendu pour une rafale de taille normale | | |
| 3 | Noter le temps écoulé entre le premier et le dernier événement sonore dans les journaux (si horodatés) | Écart cohérent avec une lecture strictement séquentielle (jamais de chevauchement, contract §5.3) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Suite automatisée #228 (exécution QA, complément non-auditif)

**Objectif** : faire exécuter par QA la suite automatisée complète — complète les
scénarios matériels ci-dessus, ne les remplace pas.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | `cd server-go && go build ./...` | Compile sans erreur | | |
| 2 | `go test ./internal/audio/... -run 'TestNewOutput|TestPlayContract|TestCrossCompile' -v` | 100% vert (init/dégradation/blocage/compilation croisée, sans matériel) | | |
| 3 | `go test ./internal/audio/... -race` | Vert — aucune race détectée | | |
| 4 | `go test ./cmd/server/... -run 'TestSound|TestCA7' -v` | 100% vert (non-régression #227 + gel de dépendances amendé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (Windows, pilote atteint le périphérique) : PASS
- [ ] Scénario 2 (Raspberry Pi, hypothèse systemd) : PASS **ou** échec documenté
      précisément (voir note du scénario)
- [ ] Scénario 3 (enceinte éteinte/rallumée) : PASS
- [ ] Scénario 4 (latence/durée mesurées) : PASS
- [ ] Scénario 5 (suite automatisée) : PASS
- [ ] **Rappel** : l'absence de son distinctement audible par cue N'EST PAS un
      critère d'échec à ce stade (voir note en tête de fichier) — seule une erreur,
      un blocage ou un crash le serait

## Notes QA

- Cette procédure valide le **pilote**, pas le **caractère** des sons — la
  validation à l'oreille des sept bruitages appartient à la procédure de `#229`.
- Le Scénario 2 (Raspberry Pi) nécessite un accès matériel réel — ce n'est **pas**
  exécutable par un agent sans navigateur ni matériel (règle projet : scénarios
  nécessitant du matériel réel restent du ressort de l'utilisateur).
- Le Scénario 5 est une commande `go test` : exécutable par `qa` (agent) sans
  matériel ni navigateur.
- Si le Scénario 2 échoue, consigner le message d'erreur exact et le comportement de
  `loginctl show-user` — cette information est directement exploitable pour corriger
  le pilote sans toucher au moteur (`#227`, inchangé).
