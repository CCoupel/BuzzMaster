# Unit Test Specifications - NVS Persistence (v2.54.0)

## Contexte

Tests unitaires conceptuels pour valider la logique de persistance NVS
du firmware BuzzClick ESP32-C3. Ces tests ne peuvent pas être exécutés
directement (firmware embarqué), mais documentent les invariants à vérifier
lors du développement et des revues de code.

**Fichiers source testés** :
- `src/BuzzClick/click_nvsConfig.h` : `nvsLoadConfig()`, `nvsSaveConfig()`, `nvsClearConfig()`, `nvsGetConfig()`
- `src/BuzzClick/click_MAIN.cpp` : `checkBootButton()`, `setup()`
- `src/BuzzClick/click_usbConfig.h` : `usbProcessCommand()` (AT+FACTORY, AT+SAVE)
- `src/BuzzClick/click_WifiManager.h` : `connectToWifi()`, `checkWifiStatus()`

---

## Test 1 : nvsClearConfig() efface toutes les clés NVS

### Description
Après appel de `nvsClearConfig()`, toutes les clés du namespace `buzzclick` doivent être vides et `currentConfig.valid` doit être `false`.

### Pré-conditions
- NVS contient une config valide : `wifi_ssid="Test"`, `wifi_password="12345678"`, `server_ip="1.2.3.4"`, `server_tcp_port=1234`
- `currentConfig.valid == true`

### Action
```cpp
nvsClearConfig();
```

### Assertions
- `currentConfig.wifi_ssid == ""`
- `currentConfig.wifi_password == ""`
- `currentConfig.server_ip == ""`
- `currentConfig.server_tcp_port == 1234` (valeur par défaut)
- `currentConfig.valid == false`
- NVS namespace `buzzclick` ne contient aucune clé

### Fichier source
`click_nvsConfig.h:70-86`

---

## Test 2 : nvsLoadConfig() retourne false après nvsClearConfig()

### Description
Après un `nvsClearConfig()`, un appel à `nvsLoadConfig()` doit retourner `false`
car aucun SSID n'est configuré (condition de validité).

### Pré-conditions
- `nvsClearConfig()` a été appelé
- NVS namespace `buzzclick` est vide

### Action
```cpp
nvsClearConfig();
bool result = nvsLoadConfig();
```

### Assertions
- `result == false`
- `currentConfig.valid == false`
- `currentConfig.wifi_ssid == ""`

### Invariant
`nvsLoadConfig()` retourne `true` si et seulement si `wifi_ssid.length() > 0` (ligne 35 de click_nvsConfig.h).

### Fichier source
`click_nvsConfig.h:25-46`

---

## Test 3 : nvsLoadConfig() retourne true avec config valide

### Description
Après un `nvsSaveConfig()` avec un SSID non vide, `nvsLoadConfig()` doit retourner `true`.

### Pré-conditions
- `currentConfig.wifi_ssid = "TestWiFi"` (non vide)
- `nvsSaveConfig()` a été appelé avec succès

### Action
```cpp
// Simuler reset de la config en mémoire
currentConfig = BuzzClickConfig();
// Recharger depuis NVS
bool result = nvsLoadConfig();
```

### Assertions
- `result == true`
- `currentConfig.valid == true`
- `currentConfig.wifi_ssid == "TestWiFi"`

### Fichier source
`click_nvsConfig.h:25-46, 48-68`

---

## Test 4 : Séquence factory reset (bouton) persiste après ESP.restart()

### Description
La séquence complète du factory reset physique doit garantir que le NVS est vide
AVANT le reboot, de sorte qu'au prochain boot `nvsLoadConfig()` retourne `false`.

### Séquence testée (click_MAIN.cpp:78-88)
```cpp
// Bouton maintenu >= 3s
nvsClearConfig();                                    // 1. Effacer NVS
ESP_LOGI(MAIN_TAG, "NVS cleared, rebooting...");     // 2. Log
delay(500);                                           // 3. Attendre flush
ESP.restart();                                        // 4. Reboot immédiat
```

### Invariants à vérifier

| Étape | Invariant |
|-------|-----------|
| Avant reboot | `nvsClearConfig()` a été appelé |
| Avant reboot | NVS namespace `buzzclick` est vide sur le flash |
| Avant reboot | `ESP.restart()` est appelé immédiatement après clear |
| Après reboot | `nvsLoadConfig()` retourne `false` |
| Après reboot | `usbOnlyMode == true` |
| Après reboot | `WiFi.mode(WIFI_OFF)` est appelé |
| Après reboot | `"USB_READY"` est émis sur Serial |

### Point critique du fix
Ligne 84 de `click_MAIN.cpp` : `nvsClearConfig()` est appelé DANS `checkBootButton()`,
suivi immédiatement de `ESP.restart()` (ligne 88). Cela garantit que le NVS est vidé
sur le flash avant le reboot. Au reboot suivant, `nvsLoadConfig()` (ligne 138) détecte
le NVS vide et entre en mode USB (lignes 141-148).

### Fichier source
`click_MAIN.cpp:54-104, 106-184`

---

## Test 5 : Boot flow avec NVS vide (post factory reset)

### Description
Quand `nvsLoadConfig()` retourne `false` dans `setup()`, le boot flow doit :
1. Ne PAS créer le watchdog
2. Activer `usbOnlyMode`
3. Désactiver le WiFi
4. Émettre `USB_READY`

### Séquence testée (click_MAIN.cpp:136-149)
```cpp
bool hasNvsConfig = nvsLoadConfig();                  // false
if (!hasNvsConfig) {
    usbOnlyMode = true;                               // Mode USB activé
    setLedColor(255, 0, 255, true);                   // Magenta
    WiFi.mode(WIFI_OFF);                              // WiFi désactivé
    WiFi.disconnect(true);
    esp_log_level_set("*", ESP_LOG_NONE);
    Serial.println("USB_READY");
    return;                                            // Pas de watchdog, pas de WiFi
}
```

### Assertions

| Variable/Appel | Valeur attendue |
|----------------|-----------------|
| `hasNvsConfig` | `false` |
| `usbOnlyMode` | `true` |
| `WiFi.mode()` appelé avec | `WIFI_OFF` |
| Watchdog créé | NON (return avant xTaskCreate) |
| `setupWifi()` appelé | NON (return avant) |
| `attachButtons()` appelé | NON (return avant) |
| Serial output | `"USB_READY"` |

### Fichier source
`click_MAIN.cpp:136-149`

---

## Test 6 : connectToWifi() guard NVS vide

### Description
`connectToWifi()` doit refuser de connecter le WiFi quand `cfg.valid == false`.

### Pré-conditions
- `nvsGetConfig().valid == false` (après factory reset)

### Action
```cpp
bool result = connectToWifi();
```

### Assertions
- `result == false`
- Log contient `"No WiFi config in NVS, skipping WiFi connection (USB mode)"`
- LED orange (255, 128, 0)
- `WiFi.begin()` n'est PAS appelé

### Fichier source
`click_WifiManager.h:72-81`

---

## Test 7 : checkWifiStatus() guard NVS vide

### Description
`checkWifiStatus()` ne doit PAS tenter de reconnexion WiFi quand le NVS est vide.

### Pré-conditions
- `nvsGetConfig().valid == false`
- WiFi déconnecté

### Action
```cpp
checkWifiStatus();
```

### Assertions
- Aucun appel à `connectToWifi()`
- Aucun appel à `ESP.restart()`
- Fonction retourne immédiatement

### Fichier source
`click_WifiManager.h:124-142`

---

## Test 8 : AT+FACTORY appelle nvsClearConfig() + ESP.restart()

### Description
La commande AT `AT+FACTORY` doit effacer le NVS et reboot, avec les bonnes réponses série.

### Action
```cpp
usbProcessCommand("AT+FACTORY");
```

### Assertions (séquence série)
1. `+FACTORY:Config cleared`
2. `OK`
3. `+REBOOTING`
4. `nvsClearConfig()` appelé
5. `Serial.flush()` appelé
6. `delay(500)` appelé
7. `ESP.restart()` appelé

### Fichier source
`click_usbConfig.h:162-171`

---

## Test 9 : AT+SAVE avec config valide persiste et reboot

### Description
Après staging des valeurs AT, `AT+SAVE` doit persister en NVS et reboot.

### Pré-conditions
- `staged_ssid = "TestWiFi"` (via AT+SSID=TestWiFi)
- `staged_pass = "12345678"` (via AT+PASS=12345678)

### Action
```cpp
usbProcessCommand("AT+SAVE");
```

### Assertions
- `nvsSaveConfig()` appelé et retourne `true`
- `currentConfig.wifi_ssid == "TestWiFi"`
- `currentConfig.wifi_password == "12345678"`
- Réponse série : `+SAVED:SSID=TestWiFi,...`, `OK`, `+REBOOTING`
- `ESP.restart()` appelé

### Fichier source
`click_usbConfig.h:134-159`

---

## Test 10 : AT+SAVE sans SSID échoue

### Description
`AT+SAVE` sans avoir stagé un SSID (ou avec SSID vide) doit échouer sans reboot.

### Pré-conditions
- `staged_ssid = ""` (rien stagé)
- `currentConfig.wifi_ssid = ""` (NVS vide)

### Action
```cpp
usbProcessCommand("AT+SAVE");
```

### Assertions
- Réponse série : `ERROR:SSID is required. Use AT+SSID=<value> first`
- `nvsSaveConfig()` n'est PAS appelé
- `ESP.restart()` n'est PAS appelé

### Fichier source
`click_usbConfig.h:143-146`

---

## Résumé de couverture

| Fonction | Tests | Cas couverts |
|----------|-------|--------------|
| `nvsClearConfig()` | 1, 2 | Effacement NVS, invalidation config |
| `nvsLoadConfig()` | 2, 3 | NVS vide (false), NVS valide (true) |
| `checkBootButton()` | 4 | Séquence factory reset complète |
| `setup()` boot flow | 5 | NVS vide -> mode USB, pas de watchdog |
| `connectToWifi()` | 6 | Guard NVS vide |
| `checkWifiStatus()` | 7 | Guard NVS vide (pas de reconnexion) |
| `AT+FACTORY` | 8 | Clear NVS + reboot |
| `AT+SAVE` | 9, 10 | Sauvegarde valide, refus sans SSID |
