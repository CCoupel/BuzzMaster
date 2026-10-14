# Procédure de Test — #267 : alerte plein écran « le serveur n'a pas de réseau » (admin)

**Version** : 11.2.2
**Date** : 2026-10-14
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — une panne réseau ne se simule pas en session agent)
**Maquette** : `docs/mockup/v11.2.2/ui/admin_network_overlay__no_network.html` (3 états : overlay, bandeau replié, toast de retour)
**Contrat** : `contracts/game-state.md` §`NETWORK` ; journal : `contracts/logging.md` §3.2 nonies
**Automatique associé** : `server-go/web/src/components/NetworkLostOverlay.267.test.jsx`, `server-go/web/src/hooks/useWebSocket.network.267.test.js`, tests Go `network_267_*`

## Rappel de la règle
- **Sans réseau** = aucune adresse IPv4 *utilisable* sur une interface active, après exclusion de `127/8`, `169.254/16` (APIPA) et des **interfaces virtuelles connues** (par nom, sans tenir compte de la casse) : Windows `vEthernet`, `Virtual`, `WSL`, `Hyper-V`, `VMware`, `VirtualBox`, `Loopback` ; Linux/macOS préfixes `docker`, `br-`, `veth`, `virbr`, `tun`, `tap`, `wg`, `lxc`, `vmnet`, `utun`.
- Détection au démarrage puis **toutes les 30 s** : la disparition de l'alerte n'est donc pas instantanée (compter jusqu'à 30 s dans chaque scénario).
- L'alerte n'existe **que** sur les pages `/admin/*` ; jamais sur `/tv`, `/anim`, `/player`, `/`.
- Aucun conseil propre à un système d'exploitation dans le texte (décision Q3) : le texte est le même partout.

## Prérequis
- [ ] Environnement : QUALIF avec le binaire du LOT-3 (version affichée dans la Navbar notée dans les observations)
- [ ] Accès **physique ou console** à la machine du serveur (on y coupe le réseau) ; le navigateur de test tourne **sur cette machine** (`http://localhost`)
- [ ] Un second appareil (poste ou tablette) sur le même réseau, pour vérifier l'admin distant (scénario 7)
- [ ] La page **Logs** (`/admin/logs`) ouverte dans un second onglet pour lire le journal (`NETWORK_LOST`, `NETWORK_RESTORED`, `NETWORK_IPS_CHANGED`) ; ou le fichier de journal si activé
- [ ] Une partie ou un quiz chargé, pour constater que l'alerte ne casse pas la page en dessous
- [ ] Noter les interfaces réseau de départ : Windows `ipconfig` ; Raspberry/Linux `ip -4 -br addr`

---

## Scénario 1 — Windows : perte du réseau (WiFi ou câble coupé)
**Objectif** : l'overlay apparaît quand il ne reste que des interfaces virtuelles (cas `vEthernet (WSL)` : le bug d'origine, une adresse existe mais n'est joignable par personne).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réseau normal : ouvrir `http://localhost/admin` | Page de jeu normale, **aucun** overlay, aucun bandeau rouge | | |
| 2 | Couper le WiFi **et** débrancher le câble (ne laisser que les interfaces virtuelles, dont `vEthernet (WSL)` si WSL/Hyper-V est installé) ; attendre jusqu'à 30 s | Overlay plein écran « Le serveur n'a pas de réseau disponible » par-dessus la page | | |
| 3 | Lire le bloc de faits | « Adresses détectées : **aucune** — seulement `127.0.0.1` » ; « Interfaces ignorées » liste `vEthernet (WSL)` (et son adresse, ex. `172.x.x.x`) avec « virtuelle — non joignable par les appareils » ; « Dernière vérification : il y a N s — contrôle automatique toutes les 30 s » | | |
| 4 | Lire tout le texte de la fenêtre | Texte identique à la maquette ; **aucune** mention de Windows, Linux, macOS ni commande à taper | | |
| 5 | Journal (`/admin/logs` dans l'autre onglet, ou après fermeture de l'alerte) | **Une seule** ligne `NETWORK_LOST` (WARN) avec `ignored` contenant `vEthernet (WSL):172…` ; pas de ligne répétée à chaque passage du contrôle de 30 s | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Windows : « Continuer quand même », bandeau persistant, retour du réseau
**Objectif** : cycle complet overlay → bandeau → toast.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réseau toujours coupé, overlay affiché : cliquer « Continuer quand même » | Overlay fermé ; **bandeau rouge** sous la Navbar : « Serveur sans réseau — les buzzers, les tablettes, la TV et l'animateur ne peuvent pas se connecter. », « Vérifié il y a N s », bouton « Détails » | | |
| 2 | Observer « Vérifié il y a N s » pendant ~10 s | Le compteur augmente chaque seconde | | |
| 3 | Naviguer : Équipes, Questions, Scores, Configuration | Le bandeau rouge reste visible sur **toutes** les pages admin ; l'overlay ne se rouvre pas | | |
| 4 | Cliquer « Détails » | L'overlay se rouvre | | |
| 5 | Appuyer sur la touche Échap | Même effet que « Continuer quand même » : overlay fermé, bandeau visible | | |
| 6 | Rétablir le réseau (WiFi ou câble) et attendre jusqu'à 30 s | Bandeau retiré, toast vert « Réseau rétabli — <adresse IP du poste> » pendant environ 6 s, puis il disparaît seul | | |
| 7 | Journal | Une ligne `NETWORK_RESTORED` (INFO) avec `ips` et `lost_for_s` ; aucune alerte résiduelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Nouvelle perte : l'overlay revient toujours
**Objectif** : après un repli, une *nouvelle* panne rouvre l'overlay (pas seulement le bandeau).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Provoquer une perte, cliquer « Continuer quand même » (bandeau affiché) | Bandeau rouge | | |
| 2 | Rétablir le réseau, attendre le toast « Réseau rétabli » | Plus de bandeau | | |
| 3 | Recouper le réseau, attendre jusqu'à 30 s | L'**overlay plein écran** réapparaît (pas seulement le bandeau) | | |
| 4 | Journal | Deux paires `NETWORK_LOST` / `NETWORK_RESTORED` au total, dans l'ordre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Rafraîchissement et ouverture de l'admin pendant la panne
**Objectif** : un admin qui (re)charge la page pendant la panne voit l'alerte immédiatement.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réseau coupé : appuyer sur F5 sur `/admin` | L'overlay est affiché dès le chargement, sans attendre 30 s | | |
| 2 | Ouvrir directement `/admin/quiz`, `/admin/teams`, `/admin/settings` (nouvel onglet à chaque fois) | Overlay présent sur chacune | | |
| 3 | Ouvrir `/tv`, `/anim` et `/player` sur la même machine, réseau toujours coupé | **Aucune** alerte sur ces trois pages (affichage TV inchangé, statique) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Accessibilité de l'overlay
**Objectif** : clavier et lecteur d'écran.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Overlay ouvert : appuyer plusieurs fois sur Tab, puis Maj+Tab | Le focus reste dans la fenêtre (sur « Continuer quand même ») ; la barre de navigation de derrière est inatteignable | | |
| 2 | Cliquer dans la zone sombre derrière la fenêtre | Pas d'action sur la page de derrière | | |
| 3 | Si un lecteur d'écran est disponible (Narrateur sous Windows) | La fenêtre est annoncée comme une alerte (« Le serveur n'a pas de réseau disponible ») et son explication est lue | | |
| 4 | Ouvrir la popup « Lancer la partie » puis provoquer la perte | L'overlay réseau est **au-dessus** de la popup | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Raspberry Pi : coupure du WiFi et vérification du point d'accès (risque R2)
**Objectif** : confirmer sur la vraie machine que le point d'accès hostapd n'est pas pris pour une interface virtuelle, et que la perte est bien détectée.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Serveur démarré sur le Raspberry en mode point d'accès ; noter la sortie de `ip -4 -br addr` | L'interface du point d'accès (ex. `wlan0`) porte une adresse IPv4 du sous-réseau du point d'accès. **Noter son nom exact.** | | |
| 2 | **Vérification R2** : comparer ce nom à la liste d'exclusion (préfixes `docker`, `br-`, `veth`, `virbr`, `tun`, `tap`, `wg`, `lxc`, `vmnet`, `utun`) | Le nom ne commence par aucun de ces préfixes (cas attendu : `wlan0`). **Si le point d'accès est monté sur un pont nommé `br-…`, ou sur `tun`/`tap`/`wg…`, c'est un ÉCHEC bloquant** : la perte d'un vrai réseau serait masquée — à remonter au CDP | | |
| 3 | Admin ouvert (sur le Raspberry ou depuis un appareil connecté au point d'accès) en fonctionnement normal | Aucun overlay, aucun bandeau | | |
| 4 | Sur le Raspberry : `sudo ip link set wlan0 down`, attendre jusqu'à 30 s ; regarder l'admin **sur le Raspberry** (écran local ou navigateur local) | Overlay « Le serveur n'a pas de réseau » (adresses : aucune) | | |
| 5 | `sudo ip link set wlan0 up` (relancer le point d'accès si besoin) et attendre jusqu'à 30 s | Toast « Réseau rétabli — <IP du point d'accès> » ; overlay/bandeau retirés | | |
| 6 | Journal | `NETWORK_LOST` puis `NETWORK_RESTORED`, une seule fois chacun | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Admin distant : jamais concerné (risque R3)
**Objectif** : un admin ouvert depuis un autre appareil a, par construction, du réseau — il ne doit jamais voir l'alerte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réseau normal : ouvrir `http://<IP du serveur>/admin` depuis le second appareil | Page normale, aucune alerte | | |
| 2 | Pendant ce temps, aucune coupure : l'état reste « réseau présent » côté serveur | Aucune alerte, aucun toast, même après plusieurs minutes | | |
| 3 | Couper le réseau **du serveur** (scénario 1) ; observer le second appareil | L'admin distant perd sa connexion (indicateur déconnecté) ; il n'affiche **pas** l'overlay « sans réseau » tant qu'il est déconnecté (il n'a plus d'état) | | |
| 4 | Rétablir le réseau, laisser l'admin distant se reconnecter | Reconnexion normale ; aucun toast « Réseau rétabli » ne s'affiche sur un état qu'il n'a pas vu changer | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — VPN ou interface `tun` seule (Q1)
**Objectif** : une interface de tunnel active ne compte pas comme un réseau joignable par les buzzers.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Établir un VPN (ou créer une interface `tun`/`wg`) puis couper le WiFi et le câble ; attendre jusqu'à 30 s | Overlay affiché ; « Interfaces ignorées » liste l'interface du VPN avec son adresse | | |
| 2 | Couper le VPN | Aucun changement visible ; l'alerte reste affichée | | |
| 3 | Remettre le WiFi ou le câble | Toast « Réseau rétabli — <IP> » ; l'adresse annoncée est celle du WiFi/câble, **pas** celle du VPN | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Changement d'adresse sans perte
**Objectif** : changer de réseau (ex. passer d'un WiFi à un autre) ne déclenche ni overlay ni toast.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Changer de réseau en moins de 30 s, de façon qu'une adresse utilisable subsiste à chaque contrôle (ex. basculer du câble au WiFi déjà connecté) | Aucun overlay, aucun bandeau, aucun toast | | |
| 2 | Journal | Une ligne `NETWORK_IPS_CHANGED` (INFO) avec `ips` et `prev_ips` ; aucun `NETWORK_LOST` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Windows : l'overlay apparaît quand seules des interfaces virtuelles restent, avec l'interface ignorée listée (scénario 1)
- [ ] Cycle overlay → bandeau persistant → toast de retour correct (scénario 2) ; une nouvelle perte rouvre l'overlay (scénario 3)
- [ ] Alerte immédiate au rechargement ; jamais sur `/tv`, `/anim`, `/player` (scénario 4)
- [ ] Focus piégé, Échap = « Continuer quand même », annonce correcte (scénario 5)
- [ ] **R2 levé** : le point d'accès du Raspberry n'est pas ignoré, la coupure de `wlan0` est détectée (scénario 6)
- [ ] Admin distant jamais concerné (scénario 7)
- [ ] VPN seul = alerte ; changement d'adresse sans perte = silence (scénarios 8 et 9)
- [ ] Aucun conseil par système d'exploitation dans le texte
- [ ] Journal : un seul événement par transition (`NETWORK_LOST`, `NETWORK_RESTORED`, `NETWORK_IPS_CHANGED`)

## Notes QA

(version affichée, système et version du serveur, noms exacts des interfaces vus dans « Interfaces ignorées », durée réelle entre la coupure et l'alerte, anomalies)
