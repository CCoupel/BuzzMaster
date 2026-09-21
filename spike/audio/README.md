# Spike #226 — programme de démonstration jetable

**Ceci n'est PAS du code de production.** Module Go isolé (son propre `go.mod`), il ne touche
ni `server-go/go.mod`, ni `cmd/server/`, ni `internal/`. Voir le rapport de verdict
`_work/reports/spike-226-<timestamp>.md` pour la synthèse décisionnelle.

Bibliothèque testée : `github.com/ebitengine/oto/v3` v3.5.1 (attention : le module a été
renommé — l'ancien import path `github.com/hajimehoshi/oto/v3` redirige encore, mais le
`go.mod` du module lui-même se déclare sous `github.com/ebitengine/oto/v3` depuis un moment ;
utiliser ce chemin directement évite un détour de résolution de module).

## Prérequis

- Go **≥ 1.25** (ce module fixe `go 1.25.0` — voir le rapport de verdict pour pourquoi ce n'est
  pas une régression mineure mais la conclusion de la tâche 0.1).
- Sur Raspberry Pi : une enceinte Bluetooth A2DP déjà appairée hors-bande (`bluetoothctl`).
- Sur Windows : une enceinte Bluetooth déjà appairée via les Paramètres.

## Sous-commandes

Depuis ce répertoire (`spike/audio/`) :

```bash
go run . diag           # QUOI CHERCHER EN PREMIER — voir ci-dessous
go run . play <cue>      # cue ∈ {temps-ecoule, reveal, gagne, entracte-fin}
go run . overlap         # deux sons à 50ms d'écart (tâche 0.6)
go run . robustness      # preuve structurelle "jamais de blocage" (tâche 0.8, partielle)
go run . compensation    # simulation du délai son/lumière (tâche 0.11) — ne touche à rien de réel
```

Ou compilé pour la cible réelle, exactement comme la CI (`CGO_ENABLED=0`, cross-compilé) :

```bash
# Depuis n'importe quelle machine Linux/macOS avec Go installé :
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o spike-audio-pi      .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o spike-audio-win.exe .
```

Puis copier le binaire correspondant sur le Raspberry Pi / le poste Windows.

> **Windows : déjà compilé, rien à faire.** `spike-audio-win.exe` est déjà présent dans ce
> dossier (`spike/audio/spike-audio-win.exe`), buildé avec exactement les flags CI
> (`CGO_ENABLED=0 GOOS=windows GOARCH=amd64`) et vérifié PE32+ x86-64 valide. Pas besoin de Go
> installé ni de `go build` sur le poste Windows : copier ce dossier (ou juste ce fichier) et
> lancer directement `spike-audio-win.exe diag` puis `spike-audio-win.exe play reveal` — voir
> la procédure 0.4 plus bas.

## Procédure par tâche (ce que l'utilisateur doit rejouer sur matériel réel)

### 0.2 — Preuve Linux/ARM64 + couche traversée

1. Appairer l'enceinte : `bluetoothctl` → `scan on` → `pair <MAC>` → `trust <MAC>` →
   `connect <MAC>`.
2. Copier `spike-audio-pi` sur le Pi, lancer **dans la session utilisateur graphique/SSH
   normale** (pas encore le service systemd — voir 0.3) :
   ```bash
   ./spike-audio-pi diag
   ```
3. Vérifier dans la sortie : la ligne `PulseAudio REACHABLE` et la présence d'un sink
   `bluez_output.<MAC>.1` (ou approchant) dans la liste. **Si absent**, PipeWire n'a pas encore
   créé le sink Bluetooth — reconnecter l'enceinte et relancer `diag`.
4. `./spike-audio-pi play temps-ecoule` — un son doit être audible sur l'enceinte. Noter
   **lequel des deux moteurs a servi** (Pulse ou repli ALSA) — visible dans la sortie de `diag`
   juste avant.

### 0.3 — Scénario de lancement réel (le point le plus probable de faire échouer une démo)

**Constat déjà fait sans matériel, à vérifier en priorité** : `docs/ADMIN_GUIDE.md` documente le
déploiement Pi de référence comme un **service systemd *system*** :
```ini
# /etc/systemd/system/buzzcontrol.service
[Service]
ExecStart=/opt/buzzcontrol/server
```
Un service *system* (par opposition à *user*) n'a **par défaut ni `XDG_RUNTIME_DIR` ni bus D-Bus
de session** — exactement la configuration où PipeWire-pulse est injoignable et où `oto`
basculerait sur ALSA (silencieusement, sur la prise jack, si une carte ALSA existe sur le Pi ;
en échec explicite sinon, comme observé dans ce sandbox — voir le rapport de verdict).

À tester dans cet ordre exact :

1. `./spike-audio-pi diag` en session SSH normale → doit réussir (référence).
2. Le **même binaire**, lancé comme le service `.service` ci-dessus (ou une version de test
   copiée), avec `journalctl -u buzzcontrol -f` ouvert en parallèle pour voir sa sortie. Si
   `diag` y rapporte `PulseAudio UNREACHABLE`, c'est confirmé : le mode de lancement documenté
   casse l'audio, indépendamment de tout le reste.
3. Si confirmé, deux réparations possibles à arbitrer (aucune n'est appliquée par ce spike) :
   - ajouter au fichier `.service` :
     ```ini
     Environment="XDG_RUNTIME_DIR=/run/user/1000"
     Environment="PULSE_SERVER=unix:/run/user/1000/pulse/native"
     ```
     (adapter `1000` à l'UID réel de l'utilisateur dont la session PipeWire tourne, et
     `loginctl enable-linger <user>` pour que cette session existe même hors connexion) ;
   - ou convertir le service en unité **utilisateur** (`~/.config/systemd/user/`), lancée dans
     la session qui possède déjà PipeWire.

### 0.4 — Preuve Windows/AMD64

1. Appairer l'enceinte via Paramètres Windows.
2. `spike-audio-win.exe diag` puis `spike-audio-win.exe play reveal` — son attendu par le
   backend WASAPI. `diag` affiche désormais explicitement `N/A on windows` pour la section
   PulseAudio/ALSA (ces concepts n'existent pas sur cette plateforme — voir « Correctifs
   appliqués » en bas de ce fichier).

### 0.5 — Mesure de latence réelle

**Nécessite un chronomètre ou un enregistrement vidéo/audio** — ce programme ne peut mesurer
que le coût logiciel (voir sa sortie `play` : coût d'ouverture du contexte, coût de l'appel
`Play()`), jamais le délai acoustique réel introduit par le lien A2DP.

Protocole suggéré : filmer l'écran affichant un chronomètre de précision pendant que
`play temps-ecoule` est lancé (ou frapper `Entrée` et lancer simultanément un chrono), puis
relire image par image jusqu'au son perçu. Répéter 5 fois par plateforme, garder la médiane.

### 0.6 — Superposition

`overlap` en sortie de terminal indique si les deux sons ont été perçus comme simultanément
« en train de jouer » par l'API — **mais seule l'écoute réelle confirme l'absence de coupure ou
de distorsion au mixage.** Écouter le résultat sur l'enceinte cible.

### 0.7 — Sélection du périphérique

Déjà tranché par lecture de code (voir rapport de verdict) : `oto` n'expose pas la sélection de
sink, mais `github.com/jfreymuth/pulse` (déjà tiré en dépendance transitive par `oto` v3.5.x)
l'expose (`ListSinks`, `SinkByID`, `PlaybackSink`). Rien à valider sur matériel pour ce point ;
c'est un fait de bibliothèque, indépendant du device.

### 0.8 — Robustesse (enceinte éteinte/déconnectée en cours de partie)

`robustness` prouve la moitié structurelle (le point d'entrée ne bloque jamais l'appelant, avec
un faux périphérique bloqué en dur). **La moitié matérielle reste à faire** :

1. Lancer `play temps-ecoule` en boucle (`while true; do ./spike-audio-pi play temps-ecoule;
   sleep 1; done`).
2. Éteindre l'enceinte pendant que la boucle tourne. Observer : le programme doit rendre une
   erreur propre sur l'appel suivant (pas de gel du terminal).
3. Rallumer l'enceinte, relancer `diag` : PipeWire doit re-proposer le sink sans redémarrage du
   programme.

## Limite de sandbox constatée pendant le développement de ce spike

Dans l'environnement Linux (WSLg) où ce programme a été écrit, `diag` a montré un socket
PulseAudio **atteignable** (sink virtuel `RDPSink`, propre à WSLg) et un client brut
`jfreymuth/pulse` capable de s'y connecter et d'y ouvrir un flux de lecture en quelques
millisecondes — mais l'appel équivalent fait par `oto.NewContext` lui-même a échoué de façon
intermittente (« context deadline exceeded »), sans qu'aucune différence de paramètres
n'explique l'écart (reproduit avec les mêmes options exactes en direct : succès constant).
Interprétation retenue : artefact de ce sandbox précis (sink RDP partagé, contention possible
entre exécutions rapprochées), **pas un défaut d'oto/pulse** — mais cela illustre concrètement
pourquoi ce spike ne peut PAS remplacer la validation sur le Pi et le poste Windows réels : un
socket « atteignable » ne garantit pas un flux réellement jouable, ce qui est exactement l'objet
des tâches 0.2/0.3.

Mise à jour (correctif du 2026-09-21, voir section suivante) : après de nombreuses connexions
répétées pendant le développement, ce socket RDP est passé d'intermittent à indisponible en
continu (« resource temporarily unavailable »). Aucune preuve audio supplémentaire n'a donc pu
être obtenue dans ce sandbox pour re-confirmer le correctif ci-dessous à l'oreille — la
correction s'appuie sur la lecture directe du code source d'`oto` v3.5.1, pas sur une nouvelle
exécution réussie ici. Voir « Correctifs appliqués » pour le détail du raisonnement.

## Correctifs appliqués (2026-09-21, retour utilisateur Windows)

**Symptôme rapporté** : aucun son audible sur poste Windows, ni sortie par défaut ni casque
Bluetooth, avec `play gagne` affichant `buffered=0 bytes, elapsed=0s`.

**Cause racine identifiée par lecture du code source d'`oto` v3.5.1**
(`internal/mux/mux.go`, fonctions `playImpl`/`finishSourceRead`/`BufferedSize`) : `Play()`
positionne l'état "en lecture" **de façon synchrone**, mais **ne remplit pas** le tampon interne
tout de suite — ce remplissage n'a lieu que plus tard, sur le cycle de lecture propre du mux.
La boucle d'attente de `play.go`/`overlap.go` exigeait à tort `IsPlaying() ET BufferedSize()>0`
pour continuer à attendre ; juste après `Play()`, `BufferedSize()` valait légitimement 0, donc
la condition était fausse dès la première vérification, la boucle ne s'exécutait **jamais**, et
la fonction retournait immédiatement — fermant le lecteur/contexte avant que le mux n'ait eu la
moindre chance de lire un seul octet de la source. Explique exactement le symptôme observé (zéro
octet, zéro seconde) et pourquoi c'était indifférent au périphérique de sortie (PC ou Bluetooth) :
ce n'était jamais un problème de transport audio, la lecture n'avait simplement jamais commencé.

**Correctif** : la condition d'attente ne porte plus que sur `IsPlaying()` (fiable d'après
`finishSourceRead` : l'état ne repasse à "en pause" qu'une fois EOF atteint **et** le tampon
réellement vidé), bornée par une échéance de sécurité de 3 s. Un délai de grâce de 300 ms a été
ajouté après la fin de lecture signalée, avant de fermer le lecteur/contexte, pour ne pas couper
la fin du son encore dans le tampon propre au pilote (latence PulseAudio par défaut ~100 ms,
tampon WASAPI équivalent).

**Correctif cosmétique additionnel** : `diag` n'affiche plus le message "oto will fall back to
ALSA" sur Windows (ALSA n'existe pas sur cette plateforme) — remplacé par un message explicite
"N/A on windows".

**Confiance dans le correctif** : basée sur la lecture directe et exacte du code source de la
version d'`oto` utilisée (pas une supposition) — voir le rapport de verdict `_work/reports/` pour
le détail de l'extrait de code cité. **N'a pas pu être re-testée à l'oreille** dans ce sandbox
(voir note ci-dessus) : `spike-audio-win.exe` a été recompilé après correctif et vérifié comme
exécutable Windows valide (PE32+), mais seul un nouveau test utilisateur sur le poste Windows
réel confirmera le son.
