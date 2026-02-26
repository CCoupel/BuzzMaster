# Scénarios E2E - USBConfigModal unifiée (feature/usb-unified-modal)

## Prérequis
- Serveur BuzzControl démarré sur http://localhost:80
- Chrome avec MCP claude-in-chrome disponible
- Un firmware .bin uploadé sur le serveur (pour tester la section Flash)

---

## Scénario 1 : Présence de la section Flash Firmware dans la modale USB

### Contexte
Vérifier que la section "Flash Firmware via USB" est bien visible dans la modale USB quand un firmware est disponible sur le serveur.

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Faire défiler jusqu'à la section "Firmware Buzzers"
3. Vérifier la présence du bouton "Flash via USB"
4. Cliquer sur "Flash via USB"
5. Vérifier que la modale USB s'ouvre

### Résultat attendu
- Le bouton "Flash via USB" est visible dans la section firmware
- La modale s'ouvre avec le titre "Configuration USB Buzzer"
- La section "Flash Firmware via USB" est visible dans la modale

### Vérification Chrome
```
Attendre élément: button contenant "Flash via USB"
Cliquer: button "Flash via USB"
Vérifier présence: h2 "Configuration USB Buzzer"
Vérifier présence: h3 "Flash Firmware via USB"
Vérifier présence: button "Flasher via USB"
```

---

## Scénario 2 : Bouton "Flasher via USB" désactivé sans port sélectionné

### Contexte
Le bouton de flash doit être désactivé tant qu'aucun port USB n'est sélectionné.

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Cliquer sur "Flash via USB" pour ouvrir la modale
3. Ne sélectionner aucun port dans la liste
4. Vérifier l'état du bouton "Flasher via USB"

### Résultat attendu
- Le bouton "Flasher via USB" est `disabled` (grisé, non cliquable)
- Aucune action de flash n'est déclenchée

### Vérification Chrome
```
Attendre présence: button "Flasher via USB"
Vérifier attribut: button "Flasher via USB" → disabled=true
```

---

## Scénario 3 : Message d'indisponibilité si aucun firmware sur le serveur

### Contexte
Quand aucun firmware n'est disponible sur le serveur, la modale doit l'indiquer clairement.

### Prérequis spécifiques
- Aucun firmware .bin uploadé sur le serveur

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Cliquer sur "Flash via USB"
3. Observer la section "Flash Firmware via USB"

### Résultat attendu
- Le message "Aucun firmware disponible sur le serveur." est affiché
- Le bouton "Flasher via USB" est `disabled`

### Vérification Chrome
```
Vérifier présence: texte contenant "Aucun firmware disponible"
Vérifier attribut: button "Flasher via USB" → disabled=true
```

---

## Scénario 4 : Sélection d'un port puis activation du bouton Flash

### Contexte
Quand un port USB est sélectionné et qu'un firmware est disponible, le bouton Flash doit être activé.

### Prérequis spécifiques
- Un buzzer ESP32-C3 branché en USB sur le PC
- Un firmware .bin disponible sur le serveur

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Cliquer sur "Flash via USB"
3. Cliquer sur "Ajouter un port USB" pour autoriser le port
4. Sélectionner le port apparu dans la liste
5. Vérifier l'état du bouton "Flasher via USB"

### Résultat attendu
- Le port apparaît dans la liste avec son label (ex: "Port USB #1 (VID:303A PID:1001)")
- Le port sélectionné affiche un badge "Sélectionné"
- Le bouton "Flasher via USB" passe en état actif (non disabled)

### Vérification Chrome
```
Vérifier présence: .usb-port-item.selected
Vérifier présence: .usb-port-selected-badge avec texte "Selectionne"
Vérifier attribut: button "Flasher via USB" → disabled=false (ou absent)
```

---

## Scénario 5 : Déroulement d'un flash firmware complet

### Contexte
Simulation d'un flash firmware complet avec progression visible.

### Prérequis spécifiques
- Un buzzer ESP32-C3 en mode bootloader branché en USB
- Un firmware merged .bin disponible sur le serveur

### Étapes
1. Ouvrir http://localhost/admin/config
2. Cliquer sur "Flash via USB"
3. Autoriser et sélectionner le port USB du buzzer
4. Cliquer sur "Flasher via USB"
5. Observer la progression

### Résultat attendu
- Le bouton affiche "Flash en cours... X%" pendant le flash
- La barre de progression `.usb-flash-progress-bar` apparaît et avance
- Les logs de flash apparaissent dans `.usb-flash-logs`
- À la fin : "Flash terminé ! Le buzzer redémarre." apparaît dans les logs

### Vérification Chrome
```
Attendre présence: .usb-flash-progress (barre de progression)
Vérifier texte button: contient "Flash en cours"
Attendre texte: "Flash terminé"
```

---

## Scénario 6 : Bouton "Flash via USB" présent dans la section Firmware de ConfigPage

### Contexte
Vérifier que ConfigPage expose bien le bouton "Flash via USB" dans la section firmware (et non plus ailleurs).

### Étapes
1. Ouvrir http://localhost/admin/config dans Chrome
2. Faire défiler jusqu'à la section "Firmware Buzzers"
3. Vérifier la présence du bouton "Flash via USB"

### Résultat attendu
- Le bouton "Flash via USB" est visible dans la section "Firmware Buzzers"
- Il n'y a pas d'autre section de flash inline sur la page (section unifiée dans la modale)

### Vérification Chrome
```
Attendre présence: section.config-section contenant h3 "Firmware Buzzers"
Dans cette section, vérifier présence: button "Flash via USB"
Vérifier absence: section flash inline séparée
```

---

## Scénario 7 : Déconnexion AT automatique avant le flash

### Contexte
Si un buzzer est connecté en AT (mode serial), la modale doit le déconnecter automatiquement avant de lancer le flash.

### Prérequis spécifiques
- Un buzzer connecté en mode AT (via le bouton "Connecter" de la modale)
- Un port USB sélectionné

### Étapes
1. Ouvrir la modale USB
2. Sélectionner un port et cliquer "Connecter" → attendre "Connecte" dans le statut
3. Cliquer "Flasher via USB"

### Résultat attendu
- La connexion AT se ferme automatiquement
- Le flash démarre immédiatement après
- Les logs de flash apparaissent dans `.usb-flash-logs`

### Vérification Chrome
```
Vérifier avant click: .usb-status contient "Connecte"
Après click "Flasher via USB":
  Vérifier présence: .usb-flash-logs
  Vérifier absence rapide: .usb-status "Connecte" (déconnexion)
```
