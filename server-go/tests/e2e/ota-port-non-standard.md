# Scénario E2E — Non-régression OTA sur port non-standard (issue #50)

## Contexte

**Bug corrigé** : Lors d'une mise à jour OTA d'un buzzer BuzzClick, l'URL de téléchargement
du firmware était construite sans port HTTP (`http://IP/api/firmware/buzzclick/latest.bin`).
Si le serveur tournait sur un port non-standard (ex: 8080), le buzzer se connectait
sur le port 80 par défaut et l'OTA échouait silencieusement.

**Correction** (v3.6.1, `click_serverConnection.h`) : L'URL utilise désormais
`localUdpPort` (port de la connexion active, défini par `tryConnectToServer()`).
Chaîne de fallback :
1. `http://serverIP:localUdpPort/api/firmware/buzzclick/latest.bin`
2. `http://serverIP:server_tcp_port/api/firmware/buzzclick/latest.bin` (NVS)
3. URL du champ `URL` dans le message `OTA_UPDATE` (rétrocompat firmware < 3.1.2)

## Prérequis

- Serveur BuzzControl démarré sur un port **non-standard** (ex: 8080)
- Au moins un buzzer BuzzClick (firmware ≥ 3.6.1) connecté
- Un firmware `.bin` uploadé sur le serveur (via ConfigPage → OTA)
- Chrome avec MCP claude-in-chrome disponible

---

## Scénario 1 : Vérification de l'URL OTA dans les logs serveur

### Contexte
Lors du déclenchement d'un OTA, le firmware logue l'URL qu'il va utiliser.
Ce log confirme que le port non-standard est bien inclus.

### Configuration
- Serveur démarré sur le port **8080** (modifier `config.json` : `"http_port": 8080`)
- Buzzer connecté (HELLO reçu)

### Étapes
1. Ouvrir http://localhost:8080/admin/logs dans Chrome
2. Ouvrir http://localhost:8080/admin/config dans un second onglet
3. Dans l'onglet Config, aller dans la section "Mise à jour firmware"
4. Cliquer sur le bouton "Mettre à jour" (icône ▲) sur un buzzer connecté
5. Revenir dans l'onglet Logs et observer

### Résultat attendu
Le log du firmware (OTA_PROGRESS) contient `"STATUS": "downloading"` avec succès.
Côté serveur, la requête HTTP entrante pour `/api/firmware/buzzclick/latest.bin`
provient bien du buzzer (port source 8080).

### Vérification Chrome (onglet Logs)
```
Attendre présence: ligne de log contenant "OTA_PROGRESS"
Vérifier que STATUS = "downloading" (pas "error")
Vérifier absence: log contenant "HTTP error: -1" (connexion refusée)
```

---

## Scénario 2 : Vérification du format de l'URL de fallback dans OTA_UPDATE

### Contexte
Le serveur envoie un message `OTA_UPDATE` contenant un champ `URL` pour la
rétrocompatibilité avec les firmwares < 3.1.2. Ce scénario vérifie que
cette URL est valide (format HTTP correct) même si les firmwares récents l'ignorent.

### Étapes
1. Ouvrir http://localhost:8080/admin/logs dans Chrome
2. Déclencher un OTA (voir Scénario 1)
3. Rechercher dans les logs le message `OTA_UPDATE`

### Résultat attendu
Le message WebSocket `OTA_UPDATE` envoyé au buzzer contient :
```json
{
  "ACTION": "OTA_UPDATE",
  "MSG": {
    "VERSION": "x.y.z",
    "SIZE": 512000,
    "URL": "http://<IP>/api/firmware/buzzclick/latest.bin"
  }
}
```
- `URL` commence par `http://`
- `URL` se termine par `/api/firmware/buzzclick/latest.bin`
- `URL` contient une adresse IP (non vide)

---

## Scénario 3 : OTA complet sur port 8080 — du déclenchement au redémarrage

### Contexte
Test de bout en bout : le buzzer doit télécharger le firmware depuis le port 8080
et redémarrer avec succès.

### Configuration
- Serveur sur port 8080
- Firmware v3.6.1 uploadé
- Buzzer avec firmware v3.6.0 (marqué comme "obsolète")

### Étapes
1. Ouvrir http://localhost:8080/admin/game dans Chrome
2. Observer la carte du buzzer : badge orange "Mise à jour disponible" visible
3. Cliquer sur l'icône de mise à jour (▲) sur la carte du buzzer
4. Confirmer le déclenchement
5. Observer la progression dans la carte du buzzer
6. Attendre le redémarrage du buzzer (~30 secondes)
7. Observer la carte après reconnexion

### Résultat attendu
- **Phase downloading** : badge "..." ou spinner sur la carte, LEDs bleues clignotantes
- **Phase flashing** : progression 100%
- **Redémarrage** : buzzer disparaît de la liste puis réapparaît
- **Après reconnexion** : badge firmware affiche la nouvelle version (3.6.1)
- **Badge obsolète** : disparu (IS_OUTDATED = false après HELLO avec nouvelle version)
- Aucun log d'erreur OTA (`"STATUS": "error"` absent)

### Vérification Chrome
```
Attendre présence: .buzzer-card .badge-outdated (état initial)
Cliquer: bouton mise à jour
Attendre présence: .buzzer-card .ota-progress
Attendre (max 60s): buzzer se reconnecte
Vérifier: .firmware-version contient "3.6.1"
Vérifier absence: .badge-outdated
```

---

## Scénario 4 : Régression — Vérification port 80 standard non-impacté

### Contexte
Vérifier que la correction n'a pas cassé le cas nominal (port 80 par défaut).

### Configuration
- Serveur sur port **80** (configuration par défaut)
- Buzzer avec firmware < cible

### Étapes
1. Déclencher un OTA (même procédure que Scénario 3)

### Résultat attendu
- OTA fonctionne normalement (identique à avant le bugfix)
- `localUdpPort = 80` → URL = `http://IP:80/api/firmware/buzzclick/latest.bin`
  (équivalent à `http://IP/api/firmware/buzzclick/latest.bin`)

---

## Notes pour l'agent QA

**Criticité** : Ce bug rendait l'OTA **totalement non fonctionnelle** sur ports non-standard.
Il est impossible de le reproduire avec un serveur sur port 80.

**Préconditions spéciales** :
- Pour reproduire le bug corrigé : démarrer le serveur sur port 8080 (`config.json`)
- Firmware buzzer ≥ 3.6.1 requis pour avoir le fix côté firmware

**Test de non-régression automatisable** :
- Le test Go `TestBuildFirmwareURL_Format` dans `firmware_http_test.go` valide
  le format de l'URL de fallback côté serveur
- Le test Go `TestHandleAPIBuzzerUpdate_OTAPayloadURL_Format` valide que le payload
  OTA_UPDATE est correctement construit

**Points critiques** :
1. URL firmware inclut le port (non-standard)
2. Pas de `"STATUS": "error"` dans les logs OTA
3. Buzzer se reconnecte avec la nouvelle version après OTA
