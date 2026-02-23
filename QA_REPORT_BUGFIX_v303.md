# QA Report - Bugfix v3.0.3

**Date** : 2026-02-15
**Branche** : `bugfix/wifi-usb-mode`
**Version** : 3.0.3
**Commit** : `f42d790` - fix(firmware): Force reboot after NVS clear to persist factory reset

---

## 1. Executive Summary

| Critere | Resultat |
|---------|----------|
| Build firmware | FAIL (erreur pre-existante, non liee au fix) |
| Code review du fix | PASS |
| Scope du fix | Minimal (3 lignes ajoutees) |
| Regression potentielle | Aucune |
| Tests E2E | Non executables (hardware requis) |
| Tests unitaires | Non executables (firmware embarque) |

**Status global** : VALIDATED WITH RESERVATIONS

---

## 2. Build

### Commande
```bash
pio run -e buzzclick
```

### Resultat : FAIL

**Erreur** :
```
src/BuzzClick/click_WifiManager.h:55:10: error: 'initBroadcastUDP' was not declared in this scope
    if (!initBroadcastUDP()) {
```

**Cause** : `initBroadcastUDP()` et `connectSRV()` sont definis dans `click_serverConnection.h` a l'interieur de blocs `#ifndef USE_WEBSOCKET`. Or, `platformio.ini` definit `-D USE_WEBSOCKET=1`, ce qui exclut ces fonctions de la compilation. Cependant, `click_WifiManager.h` les appelle dans `WiFiGotIP()` sans guard conditionnel.

**Impact** : Erreur **pre-existante** introduite par le commit `3e1777d` (feat(websocket): Implement native ESP-IDF WebSocket client). NON liee au fix v3.0.3.

**Version** : `VERSION='"3.0.3"'` et `FIRMWARE_VERSION='"3.0.3"'` correctement definis dans `platformio.ini` (lignes 25-26).

### Warnings (non-bloquants)
- 8 deprecation warnings `containsKey()` dans `click_serverConnection.h` - ArduinoJson recommande `obj[key].is<T>()` a la place. Pre-existant.

---

## 3. Code Review

### 3.1 Diff analyse (commit f42d790)

**Fichiers modifies** : 2
- `platformio.ini` : Version bump 3.0.2 -> 3.0.3
- `src/BuzzClick/click_MAIN.cpp` : 3 lignes ajoutees dans `checkBootButton()`

**Changement** (click_MAIN.cpp lignes 86-88) :
```cpp
ESP_LOGI(MAIN_TAG, "NVS cleared, rebooting to apply factory reset...");
delay(500);  // Let log flush before reboot
ESP.restart();  // Force immediate reboot to re-read empty NVS
```

### 3.2 Verification des criteres

| Critere | Status | Detail |
|---------|--------|--------|
| `ESP.restart()` apres `nvsClearConfig()` | PASS | Ligne 88, immediatement apres clear |
| Logs explicites | PASS | `"NVS cleared, rebooting to apply factory reset..."` ligne 86 |
| `delay(500)` avant restart | PASS | Ligne 87, permet le flush des logs |
| Scope minimaliste | PASS | 3 lignes ajoutees, aucune modification structurelle |
| Version bump | PASS | 3.0.2 -> 3.0.3 dans platformio.ini |

### 3.3 Analyse du flow

**Avant le fix** :
1. Bouton 3s -> `nvsClearConfig()` -> `usbOnlyMode = true` -> return true
2. WiFi.OFF + USB_READY (dans le meme boot)
3. Probleme : Au prochain reboot, le NVS pouvait ne pas etre vide si le clear n'avait pas ete flush sur flash

**Apres le fix** :
1. Bouton 3s -> `nvsClearConfig()` -> log -> delay(500) -> `ESP.restart()`
2. Reboot force -> `nvsLoadConfig()` retourne false -> mode USB (lignes 141-148)
3. Le NVS est definitivement vide sur flash apres le restart

### 3.4 Code mort

Lignes 90-92 apres `ESP.restart()` sont du code mort (jamais atteint car restart() ne retourne pas) :
```cpp
usbOnlyMode = true;
setLedColor(255, 0, 255, true);
return true;
```
Impact : Aucun. Ces lignes ne s'executent jamais. Nettoyage cosmetique possible mais non necessaire.

---

## 4. Tests E2E

### Status : NON EXECUTABLES

Les tests E2E definis dans `tests/e2e/factory-reset-persistence.md` necessitent un hardware physique (buzzer BuzzClick ESP32-C3). 8 scenarios documentes :

| Scenario | Description | Status |
|----------|-------------|--------|
| 1. Factory reset physique (bouton) | Bouton rouge 3s + reboot | HARDWARE REQUIS |
| 2. Factory reset via AT+FACTORY | Commande AT + reboot | HARDWARE REQUIS |
| 3. Double reboot apres factory reset | Persistence au 2e reboot | HARDWARE REQUIS |
| 4. Reconfiguration complete apres reset | AT commands + AT+SAVE | HARDWARE REQUIS |
| 5. Reconfiguration partielle (SSID seul) | SSID minimal | HARDWARE REQUIS |
| 6. Feedback LED en mode USB | Blink patterns 1Hz/4Hz | HARDWARE REQUIS |
| 7. Factory reset pendant connexion WiFi | Reset depuis etat connecte | HARDWARE REQUIS |
| 8. Web Serial API (frontend) | Reconfiguration via Chrome | HARDWARE REQUIS |

**Note** : Le document E2E reference la version v2.54.0 dans son titre mais devrait indiquer v3.0.3.

---

## 5. Tests Unitaires

### Status : NON EXECUTABLES

Les specifications de tests unitaires dans `tests/unit/nvs-persistence-spec.md` documentent 10 tests conceptuels pour valider les invariants du firmware. Ces tests ne peuvent pas etre executes directement (firmware embarque ESP32-C3).

| Test | Description | Status |
|------|-------------|--------|
| 1. nvsClearConfig() efface NVS | Toutes les cles videes | CONCEPTUEL |
| 2. nvsLoadConfig() false apres clear | Retourne false si NVS vide | CONCEPTUEL |
| 3. nvsLoadConfig() true avec config | Retourne true si SSID present | CONCEPTUEL |
| 4. Sequence factory reset persiste | nvsClearConfig + ESP.restart | CONCEPTUEL |
| 5. Boot flow NVS vide | Pas de watchdog, mode USB | CONCEPTUEL |
| 6. connectToWifi() guard NVS vide | Refuse connexion si NVS vide | CONCEPTUEL |
| 7. checkWifiStatus() guard NVS vide | Pas de reconnexion si NVS vide | CONCEPTUEL |
| 8. AT+FACTORY | Clear NVS + reboot | CONCEPTUEL |
| 9. AT+SAVE valide | Sauvegarde + reboot | CONCEPTUEL |
| 10. AT+SAVE sans SSID echoue | Erreur sans reboot | CONCEPTUEL |

---

## 6. Regression

### Analyse statique du code

| Fonctionnalite existante | Impact du fix | Status |
|--------------------------|---------------|--------|
| Boot normal (sans bouton) | Aucun - le fix est dans le bloc `if (elapsed >= 3000)` qui n'est jamais atteint sans bouton | PASS |
| AT commands (usbConfig.h) | Aucun - fichier non modifie | PASS |
| WebSocket protocol | Aucun - fichier non modifie | PASS |
| NVS load/save/clear | Aucun - fichier non modifie, memes fonctions utilisees | PASS |
| WiFi connection | Aucun - WifiManager.h non modifie | PASS |
| LED management | Aucun - led.h non modifie | PASS |
| Button interrupts | Aucun - attachButtons() non modifie | PASS |

**Conclusion** : Le fix est isole dans `checkBootButton()` a l'interieur du bloc conditionnel "bouton maintenu 3 secondes". Aucune regression possible sur les fonctionnalites existantes.

---

## 7. Blocking Issues

| # | Type | Description | Impact | Action requise |
|---|------|-------------|--------|----------------|
| 1 | BUILD FAILURE | `initBroadcastUDP` non declare quand `USE_WEBSOCKET=1` | Important | Ajouter guards `#ifndef USE_WEBSOCKET` dans `WiFiGotIP()` de `click_WifiManager.h` |

**Note** : Cette erreur est **pre-existante** (commit `3e1777d`) et **non liee** au bugfix v3.0.3. Elle doit etre corrigee independamment.

---

## 8. Recommendations

### Obligatoires avant QUALIF
1. **Corriger l'erreur de build** : Ajouter des guards `#ifndef USE_WEBSOCKET` autour de `connectSRV()` et `initBroadcastUDP()` dans `WiFiGotIP()` (click_WifiManager.h:49-58), ou implementer les equivalents WebSocket
2. **Verifier la version** dans le document E2E (references v2.54.0 au lieu de v3.0.3)

### Suggerees
3. **Supprimer le code mort** apres `ESP.restart()` dans `checkBootButton()` (lignes 90-92)
4. **Migrer les deprecation warnings** `containsKey()` vers `obj[key].is<T>()` dans click_serverConnection.h (8 occurrences)

---

## 9. Verdict

### VALIDATED WITH RESERVATIONS

**Raison** : Le bugfix v3.0.3 est **correct et bien isole**. L'analyse statique du code confirme que :
- `nvsClearConfig()` + `ESP.restart()` garantit la persistence du factory reset
- Le fix n'affecte aucune autre fonctionnalite (scope minimal : 3 lignes)
- Les logs sont clairs et le delay avant restart est appropriate

**Reserves** :
1. Le build echoue a cause d'une erreur **pre-existante** (`initBroadcastUDP` non declare avec `USE_WEBSOCKET=1`). Cette erreur doit etre corrigee avant de pouvoir compiler et flasher le firmware v3.0.3.
2. Les tests E2E et unitaires sont **non executables** sans hardware physique. La validation finale necessite un test sur un buzzer reel.

**Pret pour QUALIF** : NON - l'erreur de build pre-existante doit d'abord etre corrigee.

---

## Synthese pour Validation Utilisateur

### Ce qui a ete implemente
Bugfix factory reset : apres un factory reset (bouton rouge maintenu 3s), le buzzer reboot immediatement pour garantir que le NVS est vide sur le flash. Au reboot, le buzzer entre en mode USB (magenta clignotante) de facon persistante.

### Tests de Non-Regression (analyse statique)
| Fonctionnalite existante | Status |
|--------------------------|--------|
| Boot normal WiFi (sans bouton) | PASS - code non affecte |
| Commandes AT (USB config) | PASS - fichier non modifie |
| Protocole WebSocket | PASS - fichier non modifie |
| Gestion LED | PASS - fichier non modifie |
| NVS load/save/clear | PASS - fichier non modifie |

### Tests de la Nouvelle Fonctionnalite
| Test | Status |
|------|--------|
| ESP.restart() apres nvsClearConfig() | PASS (code review) |
| Log "NVS cleared, rebooting..." present | PASS (code review) |
| delay(500) avant restart | PASS (code review) |
| Version 3.0.3 dans platformio.ini | PASS |
| Build firmware | FAIL (erreur pre-existante, non liee au fix) |

### Comment Tester Manuellement

1. **Corriger l'erreur de build** : Ajouter `#ifndef USE_WEBSOCKET` / `#endif` autour des appels `connectSRV()` et `initBroadcastUDP()` dans `click_WifiManager.h` lignes 49-58
2. **Builder et flasher** : `pio run -e buzzclick -t upload`
3. **Factory reset** : Debrancher le buzzer, rebrancher en maintenant le bouton rouge 3s, observer LED magenta, puis verifier mode USB persistant apres reboot
4. **Reconfigurer** : Envoyer `AT+SSID=...`, `AT+PASS=...`, `AT+SAVE` via terminal serie pour verifier le retour en mode WiFi

### Resultat Attendu
Apres factory reset (bouton 3s), le buzzer reboot et reste en mode USB (LED magenta clignotante 1Hz, "USB_READY" sur serie). Le mode persiste meme apres des reboots supplementaires. La reconfiguration via AT commands restaure le WiFi.

---

## Annexe - Fichiers analyses

| Fichier | Lignes | Role |
|---------|--------|------|
| `src/BuzzClick/click_MAIN.cpp` | 54-104, 106-211 | Setup, checkBootButton(), loop() |
| `src/BuzzClick/click_nvsConfig.h` | 1-91 | NVS load/save/clear |
| `src/BuzzClick/click_usbConfig.h` | 1-225 | AT commands USB |
| `src/BuzzClick/click_WifiManager.h` | 1-143 | WiFi connection management |
| `src/BuzzClick/click_serverConnection.h` | 1-556 | Server connection, broadcast, protocol |
| `platformio.ini` | 1-57 | Build configuration, version |
| `tests/e2e/factory-reset-persistence.md` | 1-279 | E2E test scenarios (8 scenarios) |
| `tests/unit/nvs-persistence-spec.md` | 1-313 | Unit test specifications (10 tests) |
