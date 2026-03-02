# Scénarios E2E - UDP Broadcast Server Discovery (v3.2.0)

## Prérequis
- Serveur BuzzControl démarré sur http://localhost:80
- Chrome avec MCP claude-in-chrome disponible
- Au moins un buzzer BuzzClick connecté en WebSocket (pour les scénarios réseau)

---

## Scénario 1 : Bandeau d'information UDP visible dans ConfigPage

### Contexte
La section WiFi de ConfigPage doit afficher un bandeau indiquant que l'IP du serveur
est découverte automatiquement par les buzzers (plus de champs IP/Port manuels).

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Faire défiler jusqu'à la section "Paramètres WiFi"
3. Observer le bandeau informatif

### Résultat attendu
- Le bandeau `.udp-discovery-info` est visible
- Le texte contient "decouverte automatiquement" ou "UDP broadcast"
- Aucun champ de saisie "IP serveur" ou "Port serveur" n'est présent dans la section WiFi

### Vérification Chrome
```
Attendre présence: .udp-discovery-info
Vérifier texte: contient "automatiquement"
Vérifier absence: input[name="server_ip"] (dans la section WiFi)
Vérifier absence: input[name="server_port"] (dans la section WiFi)
```

---

## Scénario 2 : Formulaire WiFi sans champs IP/Port serveur

### Contexte
Suite à la suppression des champs IP/Port, la section WiFi ne doit contenir que
les champs SSID, mot de passe et réseau de secours.

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Faire défiler jusqu'à la section "Paramètres WiFi"
3. Vérifier les champs présents dans le formulaire

### Résultat attendu
- Champs présents : SSID principal, mot de passe principal
- Champs présents (optionnels) : SSID secondaire, mot de passe secondaire
- Champs ABSENTS : IP serveur, Port serveur

### Vérification Chrome
```
Dans .wifi-form :
  Vérifier présence: input pour SSID
  Vérifier présence: input pour mot de passe
  Vérifier absence: input avec label "IP" ou "Port"
  Vérifier présence: bouton "Sauvegarder"
```

---

## Scénario 3 : Sauvegarde des paramètres WiFi sans IP/Port

### Contexte
La sauvegarde WiFi ne doit plus inclure les champs IP/Port et doit fonctionner
correctement sans eux.

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Aller dans la section "Paramètres WiFi"
3. Modifier le SSID (ex : "TestWifi")
4. Cliquer sur "Sauvegarder"
5. Observer le retour utilisateur

### Résultat attendu
- Un message de succès apparaît (toast ou texte vert)
- Aucune erreur console JavaScript
- L'API `/api/wifi-config` reçoit une requête POST sans champs `server_ip` / `server_port`

### Vérification Chrome
```
Modifier: champ SSID → "TestWifi"
Cliquer: button "Sauvegarder" (dans section WiFi)
Attendre présence: .toast-success ou .wifi-toast[type="success"]
Vérifier absence: message d'erreur
```

---

## Scénario 4 : Bouton "Appliquer à tous les buzzers connectés"

### Contexte
Le bouton de broadcast WiFi doit être présent et fonctionnel, envoyant la configuration
WiFi actuelle à tous les buzzers connectés.

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Faire défiler jusqu'à la section "Paramètres WiFi"
3. Vérifier la présence du bouton dans `.wifi-broadcast-section`
4. Cliquer sur le bouton

### Résultat attendu
- Le bouton "Appliquer a tous les buzzers connectes" est visible
- Un avertissement "buzzers qui changent de reseau WiFi vont redemarrer" est affiché
- Au clic, le bouton affiche un état "chargement" (spinner ou disabled)
- Un toast de résultat apparaît après l'appel API

### Vérification Chrome
```
Attendre présence: .wifi-broadcast-section
Vérifier présence: .wifi-broadcast-warning contenant "redemarrer"
Vérifier présence: button contenant "Appliquer a tous les buzzers"
Cliquer: button "Appliquer a tous les buzzers"
Attendre: état loading (button disabled ou spinner)
Attendre présence: .toast (succès ou erreur selon buzzers connectés)
```

---

## Scénario 5 : Vérification du heartbeat UDP via les logs

### Contexte
Le serveur envoie des heartbeats UDP périodiques au format BUZZ_SERVER|<IP>|<PORT>.
Les logs doivent confirmer que le broadcaster est actif.

### Prérequis spécifiques
- Serveur démarré depuis moins de 10 secondes (pour voir les premiers logs)

### Étapes
1. Ouvrir http://localhost/admin/logs dans Chrome
2. Attendre 6 secondes (intervalle heartbeat = 5s)
3. Observer les logs

### Résultat attendu
- Au moins une ligne contient "[UDP] Broadcasting BUZZ_SERVER heartbeat"
- La ligne inclut des IPs et le port 80

### Vérification Chrome
```
Ouvrir: http://localhost/admin/logs
Attendre 6 secondes
Vérifier présence: texte "[UDP] Broadcasting BUZZ_SERVER heartbeat"
Vérifier texte contient: "port=80"
```

---

## Scénario 6 : Format du heartbeat UDP

### Contexte
Vérifier que le format du message BUZZ_SERVER respecte la spécification :
`BUZZ_SERVER|<IP1>|<IP2>|...|<PORT>\0`

### Étapes (test réseau avec netcat ou listener UDP)
1. Sur la machine hôte, écouter le port 1234 UDP
2. Démarrer le serveur BuzzControl
3. Attendre 1 seconde (heartbeat immédiat au démarrage)
4. Capturer le paquet UDP reçu
5. Vérifier le format

### Résultat attendu
- Le message commence par "BUZZ_SERVER|"
- Contient au moins une adresse IPv4 valide
- Se termine par "|80" (ou le port HTTP configuré)
- Se termine par un octet null (\0)
- Ne contient pas d'adresses loopback (127.x) ni link-local (169.254.x)

### Vérification format
```
Message reçu : BUZZ_SERVER|192.168.1.X|80\0
Regex attendu : ^BUZZ_SERVER(\|[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)+\|\d+\x00$
```

---

## Scénario 7 : Synchronisation TV - Aucun impact de la suppression IP/Port

### Contexte
La suppression des champs IP/Port dans ConfigPage ne doit pas affecter l'affichage
TV ni le fonctionnement du jeu.

### Étapes
1. Ouvrir http://localhost/tv dans Chrome (onglet 1)
2. Ouvrir http://localhost/admin/game dans Chrome (onglet 2)
3. Vérifier que la vue TV fonctionne normalement
4. Dans l'admin, changer la vue TV
5. Vérifier la synchronisation

### Résultat attendu
- La vue TV se charge sans erreur
- La synchronisation WebSocket fonctionne
- Aucune erreur console liée à IP/Port manquant

### Vérification Chrome
```
Onglet 1: ouvrir http://localhost/tv
Vérifier: page charge sans erreur (pas de "Cannot read properties of undefined")
Onglet 2: ouvrir http://localhost/admin/game
Changer: vue TV depuis admin
Vérifier onglet 1: TV se met à jour (synchronisation < 1s)
```

---

## Notes pour l'agent QA

Ces scénarios sont destinés à être exécutés par l'agent QA via MCP claude-in-chrome.

**Points critiques à vérifier** :
1. Absence des champs IP/Port dans le formulaire WiFi (régression UI)
2. Présence du bandeau UDP discovery
3. Logs serveur confirment le heartbeat périodique
4. Format du message BUZZ_SERVER conforme à la spécification

**Tests de non-régression** :
- Scénario 7 : l'affichage TV n'est pas impacté
- Le bouton "Appliquer à tous les buzzers" fonctionne toujours
