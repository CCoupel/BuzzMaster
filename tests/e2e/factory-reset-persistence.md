# E2E Tests - Factory Reset NVS Persistence (v2.54.0)

## Objectif

Vérifier que le factory reset persiste correctement après reboot.

**Bug corrigé** : Avant le fix, un factory reset (bouton rouge 3s) effacait le NVS mais
le buzzer rebootait avec le WiFi qui redémarrait automatiquement car `nvsLoadConfig()`
était appelé après le watchdog, et le boot flow ne vérifiait pas l'état NVS avant de
lancer le WiFi.

**Fix appliqué** : `checkBootButton()` appelle `nvsClearConfig()` puis `ESP.restart()`
immédiatement. Au reboot, `nvsLoadConfig()` est appelé AVANT le watchdog : si NVS vide,
le buzzer entre en mode USB sans jamais démarrer le WiFi.

## Prérequis

- Un buzzer BuzzClick ESP32-C3 avec firmware v2.54.0+
- Cable USB-C pour connexion série
- Terminal série configuré à 115200 baud (PuTTY, minicom, ou Web Serial)
- Réseau WiFi fonctionnel avec serveur BuzzControl accessible

---

## Scénario 1 : Factory Reset physique (bouton) + Reboot

### Contexte
Buzzer avec config WiFi valide, connecté au serveur (LED verte).

### Étapes
1. Débrancher puis rebrancher le buzzer USB tout en maintenant le bouton rouge (GPIO 6) enfoncé
2. Observer la LED : bleu clignotant (4Hz) pendant le maintien
3. Maintenir le bouton rouge pendant au moins 3 secondes
4. Observer la LED : magenta fixe pendant 500ms = mode confirmé
5. Le buzzer reboot automatiquement (ESP.restart)

### Vérifications après reboot

| # | Vérification | Attendu | Méthode |
|---|-------------|---------|---------|
| 1 | LED au reboot | Magenta clignotante (1Hz) | Observation visuelle |
| 2 | Logs série avant reboot | `"NVS cleared, rebooting to apply factory reset..."` | Terminal série |
| 3 | Logs série après reboot | `"NVS config: EMPTY (using defaults)"` | Terminal série |
| 4 | Pas de tentative WiFi | Aucun log `"Connecting to WiFi"` | Terminal série |
| 5 | Port série | `"USB_READY"` affiché | Terminal série |
| 6 | Commande AT | `AT` retourne `OK` | Envoyer `AT` via terminal |

### Critère de réussite
Le buzzer reste en mode USB (magenta clignotante) après le reboot. Aucune tentative de connexion WiFi.

---

## Scénario 2 : Factory Reset via AT+FACTORY + Reboot

### Contexte
Buzzer avec config WiFi valide, connecté au serveur (LED verte), ou déjà en mode USB.

### Étapes
1. Connecter le buzzer en USB
2. Ouvrir un terminal série (115200 baud)
3. Envoyer la commande `AT+FACTORY`
4. Observer les réponses série
5. Attendre le reboot automatique

### Vérifications série

```
> AT+FACTORY
+FACTORY:Config cleared
OK
+REBOOTING
```

### Vérifications après reboot

| # | Vérification | Attendu | Méthode |
|---|-------------|---------|---------|
| 1 | LED au reboot | Magenta clignotante (1Hz) | Observation visuelle |
| 2 | Logs série | `"NVS config: EMPTY (using defaults)"` | Terminal série |
| 3 | Mode USB | `"USB_READY"` affiché | Terminal série |
| 4 | Pas de WiFi | Aucun log `"Connecting to WiFi"` | Terminal série |
| 5 | Config vide | `AT+SHOW` retourne tous les champs vides, `+VALID:NO` | Terminal série |

### Critère de réussite
Identique au Scénario 1 : mode USB persistant après reboot, pas de WiFi.

---

## Scénario 3 : Double reboot après factory reset

### Contexte
Vérifier que le NVS vide persiste même après un second reboot (pas de regression au 2e cycle).

### Étapes
1. Effectuer un factory reset (Scénario 1 ou 2)
2. Vérifier mode USB (magenta clignotante, `USB_READY`)
3. Débrancher puis rebrancher le buzzer USB (reboot physique)
4. Observer le comportement au boot

### Vérifications après 2e reboot

| # | Vérification | Attendu |
|---|-------------|---------|
| 1 | LED | Magenta clignotante (1Hz) |
| 2 | Logs série | `"NVS config: EMPTY (using defaults)"` |
| 3 | Mode | `"USB_READY"` |
| 4 | WiFi | Aucune tentative de connexion |

### Critère de réussite
Le NVS reste vide après le 2e reboot. Le buzzer ne revient jamais en mode WiFi sans reconfiguration explicite.

---

## Scénario 4 : Reconfiguration complète après factory reset

### Contexte
Buzzer en mode USB après factory reset (Scénario 1, 2 ou 3).

### Étapes
1. Ouvrir terminal série (115200 baud)
2. Vérifier mode USB : envoyer `AT` (doit retourner `OK`)
3. Configurer le WiFi via commandes AT :

```
> AT+SSID=MonReseauWiFi
+SSID:MonReseauWiFi
OK

> AT+PASS=MotDePasse123
+PASS:MotDePasse123
OK

> AT+SERVERIP=192.168.1.100
+SERVERIP:192.168.1.100
OK

> AT+SERVERPORT=1234
+SERVERPORT:1234
OK

> AT+SAVE
+SAVED:SSID=MonReseauWiFi,IP=192.168.1.100,PORT=1234
OK
+REBOOTING
```

4. Attendre le reboot automatique

### Vérifications après reboot

| # | Vérification | Attendu | Méthode |
|---|-------------|---------|---------|
| 1 | Logs série | `"NVS config: FOUND"` | Terminal série |
| 2 | Logs série | `"Connecting to WiFi SSID=MonReseauWiFi (source=NVS)"` | Terminal série |
| 3 | LED | Jaune (connexion) puis vert (connecté) | Observation visuelle |
| 4 | Connexion serveur | `"READY"` dans les logs | Terminal série |

### Critère de réussite
Le buzzer se reconnecte au WiFi et au serveur après reconfiguration AT.

---

## Scénario 5 : Reconfiguration partielle (SSID seul)

### Contexte
Buzzer en mode USB après factory reset.

### Étapes
1. Envoyer uniquement `AT+SSID=TestWiFi` puis `AT+SAVE`
2. Observer le comportement

### Vérifications

| # | Vérification | Attendu |
|---|-------------|---------|
| 1 | Sauvegarde | `+SAVED:SSID=TestWiFi,...` |
| 2 | NVS valid | `nvsLoadConfig()` retourne `true` (SSID non vide) |
| 3 | Server IP | Utilise fallback mDNS (`getServerIP()`) |
| 4 | Port | Utilise défaut 1234 |

### Critère de réussite
La config minimale (SSID seul) est suffisante pour sortir du mode USB.

---

## Scénario 6 : Feedback LED en mode USB (blink patterns)

### Contexte
Buzzer en mode USB après factory reset.

### Étapes
1. Observer la LED pendant 10 secondes en idle (pas de commande AT)
2. Envoyer une commande AT (ex: `AT`)
3. Observer la LED pendant 3 secondes après la commande

### Vérifications

| # | État | LED attendue | Fréquence |
|---|------|-------------|-----------|
| 1 | Idle (pas de commande récente) | Magenta clignotante | 1Hz (500ms ON / 500ms OFF) |
| 2 | Après commande AT (< 2s) | Magenta clignotante rapide | 4Hz (125ms ON / 125ms OFF) |
| 3 | 2s après dernière commande | Retour au clignotement lent | 1Hz |

### Critère de réussite
Le feedback LED indique clairement l'activité USB et le mode de fonctionnement.

---

## Scénario 7 : Factory reset pendant connexion WiFi active

### Contexte
Buzzer connecté au WiFi et au serveur (LED verte, partie en cours possible).

### Étapes
1. Buzzer connecté (LED verte)
2. Débrancher le buzzer USB
3. Rebrancher en maintenant le bouton rouge 3s
4. Vérifier le factory reset

### Vérifications

| # | Vérification | Attendu |
|---|-------------|---------|
| 1 | Pendant maintien | LED bleue clignotante (4Hz) |
| 2 | Après 3s | LED magenta fixe (500ms) |
| 3 | Log série | `"NVS cleared, rebooting to apply factory reset..."` |
| 4 | Après reboot | LED magenta clignotante (1Hz), `USB_READY` |
| 5 | WiFi | Aucune tentative de connexion |

### Critère de réussite
Le factory reset fonctionne même si le buzzer était précédemment connecté au WiFi.

---

## Scénario 8 : Vérification Web Serial API (frontend)

### Prérequis
- Serveur BuzzControl lancé sur `http://localhost` (port 80)
- Chrome/Edge 89+ avec MCP claude-in-chrome actif
- Buzzer en mode USB (magenta clignotante)

### Étapes
1. Naviguer vers `http://localhost/admin/settings`
2. Localiser la section "Configuration WiFi USB" ou le bouton USB
3. Cliquer pour ouvrir la modale USBConfigModal
4. Cliquer "Connecter" pour ouvrir le port série via Web Serial API
5. Envoyer `AT` depuis l'interface
6. Vérifier la réponse `OK`
7. Remplir les champs WiFi (SSID, Password, Server IP, Port)
8. Cliquer "Enregistrer" (envoie AT+SSID, AT+PASS, AT+SERVERIP, AT+SERVERPORT, AT+SAVE)
9. Observer le reboot du buzzer

### Vérifications Chrome

| # | Vérification | Méthode |
|---|-------------|---------|
| 1 | Modale ouverte | Chercher `.usb-config-modal` visible |
| 2 | Connexion série | Bouton "Connecter" change en "Connecté" |
| 3 | Réponse AT | Zone de log affiche `OK` |
| 4 | Envoi config | Les commandes AT sont envoyées séquentiellement |
| 5 | Reboot | Message `+REBOOTING` dans les logs |
| 6 | Déconnexion série | Port série fermé automatiquement après reboot |

### Critère de réussite
L'interface web permet de reconfigurer un buzzer en mode USB après factory reset.

---

## Résumé des critères de non-régression

| Bug original | Test de non-régression | Scénario |
|---|---|---|
| Factory reset ne persiste pas après reboot | NVS vide persiste, mode USB maintenu | 1, 2, 3 |
| WiFi redémarre automatiquement après reboot | Aucun log WiFi après factory reset + reboot | 1, 2, 3 |
| `nvsLoadConfig()` appelé après watchdog | `nvsLoadConfig()` appelé AVANT watchdog | 1, 2 |
| Pas de `nvsClearConfig()` dans `checkBootButton()` | `nvsClearConfig()` + `ESP.restart()` immédiat | 1 |
| Pas de guard NVS vide dans boot flow | `if (!hasNvsConfig)` entre en mode USB | 1, 2, 3 |
| Reconfiguration impossible après reset | AT commands + AT+SAVE restaurent le WiFi | 4, 5, 8 |
