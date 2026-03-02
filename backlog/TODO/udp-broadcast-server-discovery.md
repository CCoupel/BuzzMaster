# UDP Broadcast Server Discovery

**Statut** : 📋 Planifié

## Description

Découverte automatique de l'adresse IP du serveur par les buzzers via un heartbeat UDP broadcast, éliminant le besoin de configuration manuelle par USB quand le serveur est en DHCP.

**Problème résolu** : Quand le serveur est en DHCP, son IP peut changer, obligeant à reconfigurer l'IP du serveur sur chaque buzzer individuellement via USB.

**Solution** : Le serveur envoie un heartbeat UDP broadcast toutes les 5 secondes (1s pendant l'enrollment), contenant **toutes ses IPs disponibles**. Les buzzers écoutent ce broadcast et mettent à jour automatiquement la liste des IPs du serveur en mémoire (RAM). Lors de la connexion TCP, le buzzer essaie toutes les IPs reçues jusqu'à succès. Si le serveur est UP, il broadcast et le buzzer se connecte. Si le serveur est DOWN, il ne broadcast pas et le buzzer ne se connecte pas. Si les IPs changent, le broadcast porte les nouvelles IPs. Plus besoin de config USB pour l'IP.

## Objectifs

- [ ] Serveur Go envoie heartbeat UDP broadcast avec **toutes ses IPs** (startup automatique)
- [ ] Buzzers écoutent et mettent à jour la **liste d'IPs** en RAM
- [ ] Buzzers essaient chaque IP jusqu'à succès lors de TCP connection
- [ ] LED boot sequence intègre l'étape "waiting for broadcast"
- [ ] Tests unitaires et E2E
- [ ] Documentation du protocole

## Architecture

### Backend (Go)

**BroadcasterManager** dans `internal/server/broadcaster.go` :
- Émettre UDP broadcast toutes les Xs (démarrage automatique au startup du serveur)
- Port UDP 1234 (partagé avec TCP bugzers protocol)
- Détecte **toutes les IPs actives** du serveur (IPv4, pas de localhost/127.0.0.1)
- Format : `BUZZ_SERVER|<IP1>|<IP2>|...|<SERVER_PORT>`
  - Exemple : `BUZZ_SERVER|192.168.1.50|10.0.0.50|80`
- Log chaque broadcast envoyé avec la liste des IPs

### Frontend (React)

N/A - Le broadcast est transparent pour l'interface (démarrage automatique au startup serveur)

### Firmware (ESP32-C3)

**click_broadcaster.h** (nouveau) :
- UDP listener sur port 1234
- Parser : `BUZZ_SERVER|IP1|IP2|...|PORT`
- Stockage temporaire en mémoire (RAM) de la **liste d'IPs du serveur**
- Mise à jour continue lors de chaque heartbeat reçu
- Aucune persistance NVS (stateless)

**click_MAIN.cpp - LED Boot Sequence (mise à jour)** :
1. ✅ Power ON → Rouge pulsant
2. ✅ Factory Reset check → Vert rapide (7s)
3. ✅ WiFi connection → Bleu pulsant
4. 🆕 **Waiting for broadcast** → Jaune pulsant (attente continue)
   - Écoute les heartbeats UDP
   - Dès que reçu → LED verte, passe à étape 5
5. 🆕 **Trying all IPs** → Bleu rapide
   - Essaie chaque IP de la liste reçue (TCP 1234)
   - Première IP qui répond → continue
   - Si aucune IP répond → reste en attente (LED jaune), remonte à étape 4 pour écouter prochains broadcasts
6. ✅ TCP/WebSocket connection → Vert fixe
7. ✅ Ready → Arc-en-ciel (success)

## Tâches

### Phase 1 : Backend (Go)
- [ ] Implémenter `BroadcasterManager` dans `internal/server/broadcaster.go`
- [ ] Détecter toutes les IPs actives du serveur (net.Interfaces, exclure localhost)
- [ ] Émettre UDP broadcast toutes les Xs au startup serveur
- [ ] Format : `BUZZ_SERVER|IP1|IP2|...|PORT`
- [ ] Tests unitaires (détection IPs, format broadcast)

### Phase 2 : Firmware (ESP32-C3)
- [ ] Créer `click_broadcaster.h` avec UDP listener (port 1234)
- [ ] Parser heartbeat et récupérer liste d'IPs
- [ ] Stocker liste d'IPs en RAM (mise à jour continue)
- [ ] Modifier TCP connection handler pour essayer chaque IP jusqu'à succès
- [ ] Intégrer au `click_MAIN.cpp` dans le boot sequence
- [ ] **LED Boot Sequence : Ajouter étapes**
   - Étape 4 : "Waiting for broadcast" (jaune pulsant, attente continue)
   - Étape 5 : "Trying all IPs" (bleu rapide, boucle sur liste d'IPs)
   - Si trouvé : continue vers WebSocket
   - Si aucune IP répond : retour étape 4 (attente nouveau broadcast)
- [ ] Logging détaillé (heartbeat reçu, IPs essayées, résultat)
- [ ] Tests matériel sur ESP32-C3 (multi-IP, failover)

### Phase 3 : Tests & Documentation
- [ ] Tests unitaires Go (détection IPs multiples, format)
- [ ] Tests E2E : Buzzer reçoit et applique liste d'IPs
- [ ] Tests réseau : Changement d'IPs, failover entre IPs
- [ ] Mettre à jour [docs/PROTOCOLS.md](../../docs/PROTOCOLS.md) avec UDP broadcast multi-IP
- [ ] Documenter la séquence LED enrichie (étapes 4 et 5)
- [ ] Log de startup serveur indiquant les IPs broadcastées
- [ ] CHANGELOG.md

## Version cible

v3.2.0

## Notes d'implémentation

- **Port UDP** : 1234 (partagé avec TCP buzzer protocol, même pour toutes les IPs)
- **Intervalle** : 5s en mode normal, 1s pendant enrollment (flag à définir)
- **Format** : `BUZZ_SERVER|IP1|IP2|...|PORT` (plaintext, null-terminated)
  - Exemple : `BUZZ_SERVER|192.168.1.50|10.0.0.50|80\0`
  - Max 3-4 IPs recommandé pour tenir dans MTU (~1500 bytes)
- **Détection IPs serveur** : `net.Interfaces()` en Go, exclure localhost/loopback
- **Stockage buzzer** : RAM uniquement (liste d'IPs, stateless, mise à jour continue)
- **Failover** : Buzzer essaie chaque IP dans l'ordre jusqu'à TCP connection réussie
- **Logique** : Si serveur UP → broadcast reçu + au moins 1 IP accessible → buzzer connecté. Si serveur DOWN → pas de broadcast → buzzer reste en attente
- **LED Boot** :
  - Étape 4 "Waiting for broadcast" : Attente continue (LED jaune). Dès réception → passe à étape 5
  - Étape 5 "Trying IPs" : Essaie chaque IP jusqu'à succès. Si aucune IP répond → retour étape 4
- **Logging buzzer** : Détaillé (heartbeat reçu, IPs, résultats TCP) pour débugage
- **Logging serveur** : Au startup, affiche les IPs qui seront broadcastées

## Dépendances

- Aucune (utilise Go stdlib + ESP-IDF standard)
