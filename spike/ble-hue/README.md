# spike-ble-hue — démo de faisabilité BLE Philips Hue (issue #204)

> **Programme jetable**, hors serveur. Module Go séparé (`go.mod` propre) : il n'est ni compilé
> ni testé par `go build ./...` dans `server-go/`, et n'ajoute rien au `go.mod` du serveur.
> Il sert à répondre, **chiffres à l'appui**, aux 9 points de l'issue #204. La décision finale
> (tenable / tenable sous conditions / non tenable) est prise par l'utilisateur après exécution
> sur son matériel.

## 1. Ce que le programme fait

| Commande | Question de #204 | Ce qui est mesuré / produit |
|----------|------------------|-----------------------------|
| `scan`    | préparation | liste les ampoules Hue qui s'annoncent (MAC, RSSI, nom) — pour trouver les adresses **avant** l'appairage |
| `demo`    | pts 2, 3 | connexion à N ampoules déjà appairées, lecture d'état, puis ON → rouge → vert → bleu → blanc → 60 % → OFF. Latence de chaque écriture GATT. Détecte une écriture > 5 s (symptôme d'une invite Windows en arrière-plan) |
| `bench`   | pt 5 | latence d'écriture par ampoule (p50/p95/p99/max), **désynchronisme première↔dernière ampoule** d'un changement groupé, stratégie séquentielle *vs* parallèle, Write Request *vs* Write Command |
| `hold`    | pt 4 | connexions persistantes tenues pendant `-duration`, lecture de vie toutes les `-keepalive`, comptage des décrochages/reconnexions, **pic d'ampoules simultanées** |
| `coexist` | **pt 6** | protocole avant/après 2,4 GHz : ping continu des buzzers + compteurs des logs serveur (retries/expirations ACK LED, déconnexions buzzers) par phase `baseline` → `ble-idle` → `ble-traffic` → `ble-off` |
| `info`    | pt 1, 9 | affiche OS/arch, version Go et version de la bibliothèque BLE |

Chaque commande accepte `-out fichier.json` : le rapport JSON complet (toutes les stats) est ce
qu'il faut conserver et joindre à l'issue.

## 2. Prérequis

- **Ampoules** : Philips Hue **Bluetooth** (modèles ~2019+, logo Bluetooth sur la boîte).
- **Windows** : Windows 10 1803+ / Windows 11, Bluetooth 4.0+ intégré ou dongle. Aucun runtime
  à installer : le binaire est autonome (WinRT est appelé directement).
- **Raspberry Pi / Linux** : BlueZ ≥ 5.48 (`bluetoothd` actif : `systemctl status bluetooth`),
  D-Bus. Raspberry Pi OS Bookworm convient tel quel.
- Ping système disponible (`ping` est utilisé pour la mesure WiFi, sans privilège root/admin).

Compiler (depuis ce dossier, Go ≥ 1.24) :

```bash
# sur la machine cible
go build -o spike-ble-hue .

# croisé depuis n'importe où (aucun cgo)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o spike-ble-hue.exe .
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o spike-ble-hue-arm64 .
```

## 3. Appairage hors bande (une fois par ampoule et par machine)

C'est **le** point à valider : la bibliothèque Go ne sait pas appairer, l'ampoule l'exige.
On appaire donc avec l'OS, puis le programme se contente de se connecter et d'écrire.

### 3.1 Mettre l'ampoule en mode appairage

Une ampoule Hue BLE n'accepte un appairage que si elle est **en mode appairage** :
- ampoule neuve / jamais appairée : allumer, elle s'annonce et accepte l'appairage ;
- ampoule déjà appairée à un téléphone : dans l'app *Hue Bluetooth* → Paramètres → l'ampoule →
  *Réinitialiser* (ou reset physique selon la doc du modèle), **puis fermer l'app** (voir §6) ;
- éteindre/rallumer l'ampoule au mur juste avant l'appairage aide souvent.

Repérer son adresse MAC : `spike-ble-hue scan` (elle apparaît avec `HUE`, nom « Hue Lamp »,
« Hue color lamp »…). Une ampoule connectée à un autre appareil **ne s'annonce pas**.

### 3.2 Windows

1. Paramètres → Bluetooth et appareils → *Ajouter un appareil* → *Bluetooth*.
2. Choisir l'ampoule (« Hue Lamp… »). Windows appaire sans code PIN.
3. **Fermer la page Paramètres** ensuite : tant qu'elle est ouverte Windows peut garder une session
   GATT sur l'ampoule et la monopoliser.
4. Vérifier : `spike-ble-hue -bulbs AA:BB:CC:DD:EE:FF demo`. Si une notification « Ajouter un
   appareil ? / Autoriser… » apparaît pendant l'exécution → **c'est l'invite runtime que le spike
   doit exclure** (point 3 de l'issue). Le programme signale aussi toute écriture > 5 s.

> Note d'implémentation : sur Windows, le programme n'ouvre pas de « connexion » explicite. Il
> obtient l'objet `BluetoothLEDevice` par adresse (l'ampoule doit être connue du cache
> Bluetooth de Windows — c'est le cas après appairage) et pose une `GattSession` avec
> `MaintainConnection=true`, ce qui garde le lien ouvert.

### 3.2 bis — Diagnostic : les écritures échouent en `ATT error 0x000F` (Windows)

Symptôme observé au premier essai réel (2026-09-03) : connexion et découverte OK, aucune invite,
mais **toutes** les lectures échouent (`Paramètre incorrect.`) et toutes les écritures renvoient
`Bluetooth ATT error (code 0x000F)`. Les caractéristiques Hue exigent un lien **chiffré et
authentifié** ; 0x0F/0x05 signifie que le lien ne l'était pas au moment de l'écriture. Deux
causes, à départager dans cet ordre :

| # | Hypothèse | Test | Lecture du résultat |
|---|-----------|------|---------------------|
| **T1** | La liaison (LTK) existe mais Windows n'a jamais chiffré le lien, parce que la bibliothèque laisse `ProtectionLevel = Plain` | Même appairage, relancer avec **`-protection auth`** :<br>`spike-ble-hue.exe -bulbs MAC -protection auth -out demo-win-auth.json demo` | ✅ écritures OK ⇒ **correctif logiciel suffisant**, l'appairage Windows était bon.<br>🔔 une notification Windows « Ajouter un appareil / Autoriser » apparaît ⇒ il n'y avait **pas** de liaison réelle (voir T2) — c'est aussi l'« invite runtime » que #204 doit exclure.<br>❌ `AccessDenied` ou toujours 0x0F ⇒ pas de liaison réelle ⇒ T2. |
| **T2** | L'appairage fait dans Paramètres **n'a pas créé de liaison LE** : l'ampoule n'était pas en mode appairage, ou était encore liée à l'app du téléphone (même symptôme que HueBLE issue #9, réponse du mainteneur : « la lampe était-elle en mode appairage ? ») | 1. Paramètres → Bluetooth → ampoule → **Supprimer l'appareil**.<br>2. Téléphone : fermer l'app Hue, **couper le Bluetooth**.<br>3. Mettre l'ampoule **en mode appairage** : app Hue Bluetooth → Paramètres → Assistants vocaux → Alexa/Google → *Rendre détectable* — ou reset usine (⚠️ **la MAC change** : refaire `scan`).<br>4. Paramètres → Ajouter un appareil ; attendre « Connecté/Couplé » ; **fermer Paramètres**.<br>5. Relancer `demo` **d'abord avec `-protection plain`**, puis avec `-protection auth`. | plain ✅ ⇒ l'appairage seul suffisait (cause = mode appairage).<br>plain ❌ / auth ✅ ⇒ les deux étaient nécessaires : documenter `-protection auth` comme prérequis Windows.<br>les deux ❌ ⇒ conclure sur Windows (voir §7 du rapport de diagnostic). |
| **T3** | Contrôle croisé | Même ampoule sur le **Raspberry Pi** après `bluetoothctl pair` + `trust` | Linux ✅ / Windows ❌ ⇒ le problème est spécifique au chemin WinRT, pas à l'ampoule ni au protocole. |

Ce que la sortie `demo` affiche désormais pour trancher : une ligne **`security probe`** par
caractéristique Hue (`props=…` et `protection=Plain|EncryptionAndAuthenticationRequired` tel que vu
par Windows), les lignes `security: hue-0002 protection: Plain → …` quand `-protection` est actif,
et des erreurs de lecture explicites (`read refused: status=ProtocolError…` au lieu de
`Paramètre incorrect.`). Tout est aussi dans le JSON (`security_probe`, `security_notes`).

Sous Linux, `-protection` est ignoré : BlueZ élève lui-même la sécurité du lien à partir de la
liaison créée par `bluetoothctl pair`.

### 3.3 Raspberry Pi / Linux

```bash
sudo bluetoothctl
[bluetooth]# power on
[bluetooth]# agent on
[bluetooth]# default-agent
[bluetooth]# scan on            # attendre "Hue Lamp" et noter la MAC
[bluetooth]# scan off
[bluetooth]# pair AA:BB:CC:DD:EE:FF
[bluetooth]# trust AA:BB:CC:DD:EE:FF   # autorise les reconnexions sans agent
[bluetooth]# disconnect AA:BB:CC:DD:EE:FF
[bluetooth]# quit
```

Vérifier : `./spike-ble-hue -bulbs AA:BB:CC:DD:EE:FF demo` (utilisateur dans le groupe
`bluetooth`, ou `sudo`). Les appairages BlueZ sont persistés dans `/var/lib/bluetooth/` : l'objet
D-Bus `/org/bluez/hci0/dev_AA_BB_…` existe au redémarrage, ce dont dépend la connexion par
adresse du programme.

Dongle USB séparé : `-adapter hci1`.

## 4. Exécution

```bash
# 1) trouver les ampoules
spike-ble-hue scan -scan-duration 15s

# 2) preuve fonctionnelle (≥ 2 ampoules) + JSON
spike-ble-hue -bulbs MAC1,MAC2 -out demo-windows.json demo

# 3) latence / désynchronisme (30 changements groupés par stratégie)
spike-ble-hue -bulbs MAC1,MAC2,MAC3 -iterations 30 -interval 400ms -out bench.json bench

# 4) tenue dans le temps — ajouter des ampoules jusqu'à ce que la connexion échoue
spike-ble-hue -bulbs MAC1,...,MACn -duration 10m -keepalive 15s -out hold.json hold

# 5) coexistence 2,4 GHz (voir §5)
spike-ble-hue -bulbs MAC1,MAC2 -server http://127.0.0.1 -phase 90s -interval 500ms -out coexist-pi.json coexist
```

Flags communs : `-timeout 20s` (connexion/découverte par ampoule), `-write-mode auto|request|command`
(procédure ATT ; `request` = l'ampoule acquitte, latence = vrai aller-retour), `-v`.

Répéter `demo`, `bench`, `hold` **sur Windows et sur le Raspberry Pi** : l'issue exige les deux.

## 5. Protocole de mesure coexistence 2,4 GHz (point 6 — le plus important)

Objectif : savoir si N connexions BLE persistantes dégradent le WiFi qui porte les buzzers.
Le programme mesure, il ne juge pas.

**Où l'exécuter** : sur la machine qui héberge le serveur BuzzControl (le Raspberry Pi en
conditions réelles, puis le PC Windows). Le chemin radio du ping est alors celui du serveur.

**Mise en place**
1. Serveur BuzzControl démarré, **≥ 4 buzzers connectés** et visibles dans l'admin
   (`GET /api/buzzers` renvoie leurs IP — le programme les lit avec `-buzzers auto`).
2. Optionnel mais recommandé : passer le serveur en niveau de log DEBUG pour que les
   « ACK received » soient comptés (sinon seuls retries/expirations le sont — ils sont en INFO/WARN).
3. Les ampoules appairées et allumées, à l'emplacement réel prévu (distance ↔ Pi réaliste).

**Déroulé** (automatique) : 4 phases de `-phase` secondes chacune ; pendant tout le run, chaque
buzzer est pingé toutes les `-ping-interval` (250 ms).

| Phase | BLE | Ce que vous faites côté jeu |
|-------|-----|-----------------------------|
| `baseline` | aucun | jouer un **script identique** à chaque phase : NEW_GAME → PREPARE → START → appuyer sur les 4 buzzers → valider (LED_SET avec ACK) → recommencer |
| `ble-idle` | N ampoules connectées, silence | même script |
| `ble-traffic` | N ampoules, changement de couleur groupé toutes les `-interval` | même script |
| `ble-off` | déconnectées | même script (contrôle : retour au niveau `baseline` ?) |

**Sortie** : un tableau par phase et par buzzer — `sent / lost / loss% / p50 / p95 / max` et
`Δp95` par rapport à `baseline` ; un tableau des compteurs serveur par phase
(`ack_retry`, `ack_expired`, `bz_disc`, `button`, `warn`) ; la latence des écritures BLE pendant
`ble-traffic`. Tout est aussi dans le JSON `-out`.

**Lecture** (repères, pas de verdict automatique) :
- `Δp95` et `loss%` de `ble-idle` / `ble-traffic` vs `baseline` → c'est la dégradation du chemin
  WiFi côté serveur. Le timing de buzz est horodaté **à la réception serveur**
  (`incoming.Timestamp`), donc toute latence WiFi ajoutée décale directement l'ordre des buzz.
- `ack_retry` / `ack_expired` qui augmentent pendant les phases BLE → les LED_SET n'arrivent
  plus dans les 2 000 ms.
- `ble-off` doit revenir au niveau `baseline`, sinon l'écart n'est pas dû au BLE.

Refaire le run 2 à 3 fois et avec N croissant (2, 4, 6 ampoules) : une seule série ne suffit pas.

## 6. Point 7 — exclusivité avec l'app Hue du téléphone

Comportement attendu (à **observer** et noter) : une ampoule BLE n'accepte qu'**une centrale
connectée à la fois**. Si l'app du téléphone est connectée, `connect` du programme échoue
(timeout ou « device not found ») ou l'app perd la main quand le programme se connecte.
Ce qui n'est pas connu : si l'ampoule conserve **plusieurs liaisons (bonds)** — téléphone + PC +
Pi — ou si chaque nouvel appairage efface le précédent. Test : appairer téléphone → PC → Pi,
puis vérifier que chacun peut encore se connecter à tour de rôle (les deux autres déconnectés).

## 7. Points 1 et 9 — bibliothèque, backends, impact `go.mod`

- **Bibliothèque** : `tinygo.org/x/bluetooth` **v0.15.0** (pas v0.16.0 : cette dernière impose
  `go 1.25.0` alors que le serveur et la CI sont en 1.24 ; v0.15.0 exige `go 1.23.8`).
  Les fichiers Linux (`gap_linux.go`, `gattc_linux.go`) sont identiques entre les deux versions ;
  les fichiers Windows ne diffèrent que par des libérations d'objets COM.
- **Un seul backend applicatif** : le même code Go appelle `Connect` / `DiscoverServices` /
  `Write`. Sous le capot la bibliothèque a **deux implémentations** — BlueZ via D-Bus
  (`godbus`) sur Linux, WinRT via `saltosystems/winrt-go` + `go-ole` sur Windows — sans cgo.
  Ce que ce spike ne couvre pas : l'appairage programmatique, possible seulement via BlueZ.
- **Impact mesuré sur `server-go/go.mod`** (expérience sur une copie, `go get v0.15.0` +
  `go mod tidy`) : **+1 dépendance directe, +10 indirectes** (15 → 25), directive `go 1.24.0`
  inchangée, builds `windows/amd64`, `linux/arm64`, `linux/amd64` verts avec `CGO_ENABLED=0`,
  **+197 Ko** sur le binaire arm64 (import seul). La promesse « 100 % Go » tient.
- Ce module : 2 dépendances directes (`bluetooth`, `gorilla/websocket` pour `/ws/logs`).

## 8. Limites connues du spike

- `Connect` n'a pas de timeout propre côté Linux (attente du signal BlueZ) ; le programme borne
  l'appel (`-timeout`) et abandonne la goroutine — acceptable pour un jetable, pas pour le serveur.
- Sur Windows, une caractéristique sans bit « write » fait échouer `Write` ; le mode `auto`
  bascule alors sur Write Command et le mémorise.
- Le protocole GATT Hue est rétro-ingénieré (source : projet HueBLE) — UUID `932c32bd-…` :
  `0002` on/off (1 octet), `0003` luminosité 1-254, `0004` température mirek uint16 LE,
  `0005` couleur xy 2×uint16 LE (échelle 0xFFFF) ; nom `97fe6561-0003-…`.
- Le ping mesure le chemin *machine → buzzer* ; les ACK LED sont comptés depuis les logs serveur,
  pas chronométrés (le serveur ne trace pas la latence ACK aujourd'hui).
- Pas de test sur matériel dans l'environnement de développement : seuls les encodages, le parsing
  et les statistiques sont couverts par `go test` (voir `*_test.go`).

## 9. Que renvoyer

Pour chaque OS : `demo-<os>.json`, `bench-<os>.json`, `hold-<os>.json`, `coexist-<os>.json`,
plus une ligne pour chaque observation du §6 et toute invite/notification vue à l'écran.
