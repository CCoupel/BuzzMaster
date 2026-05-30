# Procédure de Test — Vérification QR Codes (Fix #85)

**Version** : v5.7.x  
**Date** : 2026-05-30  
**Testeur** : QA  
**Issue** : #85 — URL QR "Rejoindre le jeu" corrigée (`/player` → `/`) + escaping WiFi QR

---

## Contexte du fix

Deux corrections dans `PlayerDisplay.jsx` (phase ENROLL) :

1. **URL QR jeu** : l'URL encodée dans le QR "Rejoindre le jeu" passait vers `/player`
   (une page sans connexion WebSocket). Après fix : redirige vers `/` (EnrollPage).

2. **Escaping WiFi** : les caractères spéciaux (`;`, `,`, `"`, `\`) dans le SSID ou
   le mot de passe WiFi n'étaient pas échappés, rendant le QR WiFi illisible pour
   les lecteurs QR stricts.

---

## Prérequis

- [ ] Environnement : **QUALIF** (serveur démarré sur port 9090, ex : `./buzzcontrol-qualif`)
- [ ] Version déployée confirmée : `v5.7.x` (vérifier via `/api/config/version`)
- [ ] Routeur WiFi accessible avec un SSID connu
- [ ] Smartphone Android + smartphone iOS disponibles
- [ ] Application lecteur QR native (caméra) sur les deux smartphones
- [ ] Navigateur desktop (Chrome ou Firefox) ouvert sur `http://localhost:9090/tv`
- [ ] Le serveur est en **phase ENROLL** (commande admin ou démarrage frais)

---

## Scénario 1 — QR WiFi : connexion réseau

**Objectif** : Vérifier que le QR WiFi connecte correctement le smartphone au réseau local.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `http://localhost:9090/tv` sur un écran TV ou grand affichage | Affichage de la page ENROLL avec deux QR codes côte à côte | | |
| 2 | Vérifier le titre du QR gauche | Titre "1. Rejoindre le WiFi" | | |
| 3 | Scanner le QR gauche (WiFi) avec un iPhone | Proposition de connexion au réseau `<SSID configuré>` | | |
| 4 | Accepter la connexion sur iPhone | Smartphone connecté au réseau WiFi BuzzControl | | |
| 5 | Scanner le QR gauche (WiFi) avec un Android | Proposition de connexion au réseau `<SSID configuré>` | | |
| 6 | Accepter la connexion sur Android | Smartphone connecté au réseau WiFi BuzzControl | | |
| 7 | Vérifier le sous-titre sous le QR WiFi | Affiche "Réseau : `<SSID>`" | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — QR jeu : redirection vers `/` (EnrollPage)

**Objectif** : Vérifier que scanner le QR "Rejoindre le jeu" redirige vers l'EnrollPage
(`/`), et **non plus** vers `/player`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer le QR droit sur la page TV ENROLL | Titre "2. Rejoindre le jeu", QR vert | | |
| 2 | Scanner le QR droit avec l'iPhone (après connexion WiFi) | URL prévisualisée : `http://<host>/` (sans `/player`) | | |
| 3 | Ouvrir l'URL sur iPhone | La page d'inscription joueur (EnrollPage) s'affiche correctement | | |
| 4 | Scanner le QR droit avec l'Android | URL prévisualisée : `http://<host>/` (sans `/player`) | | |
| 5 | Ouvrir l'URL sur Android | La page d'inscription joueur (EnrollPage) s'affiche correctement | | |
| 6 | Vérifier l'URL dans le navigateur mobile | L'URL est bien `http://<host>/` et NON `http://<host>/player` | | |
| 7 | Vérifier que la page affiche le formulaire d'inscription | Champ nom + bouton rejoindre visible, pas d'erreur WebSocket | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Sous-titre QR jeu : URL affichée

**Objectif** : Vérifier que le texte affiché sous le QR jeu correspond à l'URL encodée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lire le sous-titre sous le QR "Rejoindre le jeu" | Affiche `<host>/` (ex : `192.168.1.100:9090/`) — sans `/player` | | |
| 2 | Comparer le sous-titre avec l'URL scannée par un QR reader | Les deux correspondent | | |
| 3 | Sur écran desktop, vérifier dans les DevTools (Sources) que l'URL passée au canvas QR est `http://<host>/` | Pas de `/player` dans l'URL | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Lisibilité QR sur smartphones Android + iOS

**Objectif** : Vérifier que les deux QR codes sont correctement décodables sur les deux
plateformes mobiles.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Scanner QR WiFi avec caméra native iPhone | Détection instantanée, notification WiFi apparaît | | |
| 2 | Scanner QR WiFi avec caméra native Android | Détection instantanée, notification WiFi apparaît | | |
| 3 | Scanner QR jeu avec caméra native iPhone | Lien URL détecté, ouverture dans Safari | | |
| 4 | Scanner QR jeu avec caméra native Android | Lien URL détecté, ouverture dans Chrome | | |
| 5 | Vérifier la taille des QR codes (260px) | Lisibles à ~30 cm de distance | | |
| 6 | Tester en conditions de luminosité réduite | QR codes toujours lisibles | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Escaping : SSID avec caractères spéciaux

**Objectif** : Vérifier que le QR WiFi est correct même avec un SSID contenant des
caractères spéciaux (`;`, `,`, `"`, `\`).

> ⚠️ Ce scénario nécessite de modifier le SSID WiFi configuré dans `config.json`
> (champ `wifi.ssid`) ou via l'API `/api/wifi/defaults`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer un SSID avec `;` (ex : `Buzz;Control`) | — | | |
| 2 | Recharger la page TV en phase ENROLL | QR WiFi généré | | |
| 3 | Scanner le QR WiFi | Smartphone propose de rejoindre `Buzz;Control` correctement | | |
| 4 | Configurer un SSID avec `"` (ex : `Buzz"Control`) | — | | |
| 5 | Scanner le QR WiFi | Smartphone propose de rejoindre `Buzz"Control` correctement | | |
| 6 | Restaurer le SSID d'origine | SSID normal rétabli | | |

> Note : Si l'escaping est absent, le QR WiFi généré avec un SSID contenant `;` sera
> invalide (le lecteur QR interprétera le `;` comme une fin de champ SSID).

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation (Pass/Fail global)

- [ ] **[CRITIQUE]** QR jeu ne contient PAS `/player` dans l'URL encodée
- [ ] **[CRITIQUE]** Scanner le QR jeu ouvre l'EnrollPage (`/`), pas `/player`
- [ ] **[CRITIQUE]** QR WiFi connecte correctement au réseau sur iOS et Android
- [ ] **[IMPORTANT]** Sous-titre QR jeu correspond à l'URL encodée (sans `/player`)
- [ ] **[IMPORTANT]** QR codes lisibles sur les deux plateformes mobiles
- [ ] **[IMPORTANT]** SSID avec `;` correctement échappé dans le QR WiFi
- [ ] Aucune régression visible sur les autres phases (PREPARE, READY, STARTED…)

---

## Tests de non-régression à valider en parallèle

- [ ] Phase ENROLL toujours cachée pour les VPlayers (`/player?name=...`)
- [ ] `npm run test` — suite `PlayerDisplay.qrcode.test.jsx` : tous les tests PASS
- [ ] `npm run test` — suite `wifiUtils.test.js` : tous les tests PASS

---

## Notes QA

_[Espace pour observations, captures d'écran, numéros de bugs trouvés]_

---

**Résultat global** : [ ] PASS — tous les critères CRITIQUE + IMPORTANT validés  
**Résultat global** : [ ] FAIL — au moins un critère CRITIQUE non satisfait
