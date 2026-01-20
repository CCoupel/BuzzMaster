# Page Joueur (/player)

**Statut** : 📋 Planifié

## Concept

Version adaptée de la page `/tv` pour les joueurs individuels, permettant de jouer depuis un smartphone/tablette sans buzzer physique et d'avoir une expérience personnalisée avec ses statistiques et son interface dédiée.

---

## Phase 1 - Interface joueur de base

### Connexion et identification

- [ ] **Page de connexion `/player`**
  - Sélection de l'équipe
  - Saisie du nom du joueur (ou sélection parmi les joueurs existants)
  - Choix de la couleur de réponse QCM (Rouge/Vert/Jaune/Bleu)
  - Génération d'un ID de session unique
  - Persistance de l'identité dans localStorage

- [ ] **Enregistrement côté serveur**
  - Action WebSocket `PLAYER_CONNECT`
  - Création d'un bumper virtuel dans le GameEngine
  - Payload : `{NAME, TEAM, ANSWER_COLOR}`
  - Le serveur assigne un ID unique au joueur virtuel

### Affichage de jeu personnalisé

- [ ] **Header personnalisé**
  - Avatar du joueur avec couleur de son équipe
  - Nom du joueur et nom de l'équipe
  - Score personnel affiché en permanence
  - Indicateur de connexion (connecté/déconnecté)

- [ ] **Zone de question**
  - Question en cours (identique à /tv)
  - Timer synchronisé avec le serveur
  - Média de la question (image/vidéo)

- [ ] **Zone d'action joueur**
  - Bouton BUZZ principal (grand, tactile)
  - État visuel : disponible / bloqué / a buzzé
  - Feedback immédiat au buzz

---

## Phase 2 - Contrôles interactifs

### Bouton de buzz virtuel

- [ ] **Bouton tactile responsive**
  - Taille minimum 80x80px (accessible au doigt)
  - Couleur de l'équipe du joueur
  - Animation au tap (vibration haptique sur mobile)
  - Désactivé si le joueur n'est pas en état READY

- [ ] **Envoi de l'action BUTTON**
  - Action WebSocket identique aux buzzers physiques
  - Timestamp côté client + serveur
  - Feedback visuel immédiat (changement de couleur/animation)

- [ ] **États du bouton**
  - **PREPARE** : Grisé "En attente..."
  - **READY** : Actif, couleur équipe "BUZZ !"
  - **STARTED** : Actif pulsant "BUZZ MAINTENANT !"
  - **Buzzé** : Bloqué "Vous avez buzzé" (affiche temps de réaction)
  - **PAUSED** : Bloqué "En pause"
  - **STOPPED/REVEALED** : Bloqué "Terminé"

### QCM - Boutons de réponse

- [ ] **Affichage des 4 choix**
  - 4 boutons colorés (Rouge A, Vert B, Jaune C, Bleu D)
  - Texte de la réponse sur chaque bouton
  - Disposition verticale ou grille 2x2 selon espace

- [ ] **Sélection et envoi**
  - Clic sur un bouton = buzz automatique avec couleur
  - Action `BUTTON` avec payload `{button: "A|B|C|D"}`
  - Feedback visuel : bouton sélectionné mis en évidence
  - Les autres boutons grisés après sélection

- [ ] **Affichage des indices (si activés)**
  - Réponses invalidées barrées/grisées en temps réel
  - Badge de pénalité affiché (67% / 33%)
  - Message "Indice donné ! Points réduits"

---

## Phase 3 - Memory - Contrôle des cartes

### Interface Memory pour joueur

- [ ] **Grille de cartes interactive**
  - Affichage identique à /tv
  - Cartes cliquables pour les retourner
  - Animation flip au tap

- [ ] **Envoi de l'action FLIP_MEMORY_CARD**
  - Clic sur carte → envoi au serveur
  - Payload : `{CARD_ID: "pairID-cardNum"}`
  - Feedback immédiat (pas d'attente serveur pour le flip visuel)

- [ ] **Synchronisation avec le serveur**
  - État des cartes mis à jour via WebSocket
  - Cartes matchées restent révélées
  - Cartes non-matchées se cachent après délai

- [ ] **Indicateurs de progression**
  - Compteur "Paires trouvées : X/Y"
  - Compteur "Erreurs : Z" (si pénalité active)
  - Temps restant (si timer global)

---

## Phase 4 - Statistiques et historique personnel

### Statistiques de session

- [ ] **Tableau de bord personnel**
  - Score total de la session
  - Nombre de questions jouées
  - Taux de réussite (si question attribuée au joueur)
  - Temps de réaction moyen
  - Classement dans l'équipe

- [ ] **Historique des réponses**
  - Liste des questions jouées
  - Pour chaque question :
    - Texte de la question (tronqué)
    - Temps de réaction
    - Points gagnés/perdus
    - Icône ✅ / ❌ (si attribution points)
  - Scroll vertical pour naviguer

- [ ] **Graphiques et visualisations**
  - Évolution du score au fil du temps
  - Répartition des points par catégorie (graphique en barres)
  - Comparaison avec les autres joueurs de l'équipe

---

## Phase 5 - Expérience utilisateur avancée

### Notifications et feedback

- [ ] **Notifications push (PWA)**
  - "C'est à votre équipe !" quand question démarre
  - "Vous avez buzzé le plus vite !" si premier
  - "Bonne réponse ! +X points" si attribution
  - "Mauvaise réponse, dommage" si pas de points

- [ ] **Feedback haptique (mobile)**
  - Vibration au buzz
  - Vibration double si premier à buzzer
  - Vibration pattern pour bonne/mauvaise réponse

- [ ] **Animations et transitions**
  - Confetti si bonne réponse
  - Animation de score montant
  - Transition fluide entre les phases de jeu

### Mode spectateur

- [ ] **Vue spectateur quand équipe ne joue pas**
  - Affichage de la question en cours (lecture seule)
  - Bouton BUZZ grisé et désactivé
  - Message "C'est au tour de [Équipe X]"
  - Possibilité de voir les stats d'autres équipes

- [ ] **Mode entraînement (hors partie)**
  - Répondre aux questions sans impact sur le score
  - Timer personnel pour s'entraîner
  - Sauvegarde des stats d'entraînement séparément

---

## Phase 6 - Social et collaboration

### Chat d'équipe

- [ ] **Chat textuel entre membres de l'équipe**
  - Zone de chat repliable
  - Messages visibles uniquement par l'équipe
  - Notifications de nouveaux messages
  - Émojis et réactions rapides

- [ ] **Stratégie collaborative (Memory)**
  - Marqueurs visuels partagés sur les cartes
  - "J'ai vu cette carte ici" (pointer une carte)
  - Vote pour la prochaine carte à retourner

### Avatars et personnalisation

- [ ] **Avatar personnalisé**
  - Upload d'image ou sélection dans bibliothèque
  - Génération automatique d'avatar (initiales + couleur)
  - Affichage de l'avatar dans le header et les classements

- [ ] **Personnalisation de l'interface**
  - Thème clair/sombre
  - Taille de police ajustable (accessibilité)
  - Réduction des animations (mode économie batterie)

---

## Phase 7 - Progressive Web App (PWA)

### Installation et offline

- [ ] **Manifest PWA**
  - Fichier `manifest.json` avec métadonnées
  - Icônes pour écran d'accueil (mobile)
  - Mode standalone (fullscreen)
  - Orientation portrait/paysage

- [ ] **Service Worker**
  - Cache des assets statiques (HTML/CSS/JS)
  - Fonctionnement offline pour l'interface (pas le jeu)
  - Mise à jour automatique des assets

- [ ] **Expérience native**
  - Pas de barre d'adresse en mode standalone
  - Splash screen au lancement
  - Retour haptique natif
  - Gestion des notifications push

---

## Cas d'usage identifiés

| Cas d'usage | Description | Avantages |
|-------------|-------------|-----------|
| **Jeu sans buzzers** | Jouer uniquement avec des smartphones | Pas besoin de matériel dédié, accessible à tous |
| **Joueur hybride** | Joueur avec buzzer physique + smartphone pour stats | Meilleure expérience, stats en temps réel |
| **Spectateur actif** | Suivre la partie depuis son téléphone sans jouer | Engagement même en observation |
| **Entraînement solo** | S'entraîner sur les questions hors partie | Amélioration des performances |
| **Grand groupe** | Parties avec 20+ joueurs sans buzzers physiques | Scalabilité sans limite matérielle |
| **Accessibilité** | Interface adaptée aux personnes à mobilité réduite | Boutons tactiles plus accessibles que buzzers |

---

## Architecture technique

### Routing

```
/player                     # Page de connexion
/player/:sessionId          # Interface de jeu personnalisée
/player/:sessionId/stats    # Page de statistiques détaillées
```

### WebSocket Protocol Extensions

| Action | Direction | Description |
|--------|-----------|-------------|
| `PLAYER_CONNECT` | Client→Server | Connexion d'un joueur virtuel |
| `PLAYER_DISCONNECT` | Client→Server | Déconnexion propre |
| `PLAYER_STATS` | Server→Client | Mise à jour des stats personnelles |
| `PLAYER_NOTIFICATION` | Server→Client | Notification push pour le joueur |

**PLAYER_CONNECT payload :**
```json
{
  "NAME": "Alice",
  "TEAM": "Les Rouges",
  "ANSWER_COLOR": "RED",
  "DEVICE_INFO": {
    "type": "mobile|tablet|desktop",
    "os": "iOS|Android|Windows",
    "browser": "Safari|Chrome|Firefox"
  }
}
```

**PLAYER_STATS payload :**
```json
{
  "SESSION_SCORE": 50,
  "QUESTIONS_PLAYED": 10,
  "AVG_REACTION_TIME": 1234,
  "SUCCESS_RATE": 0.7,
  "RANK_IN_TEAM": 2
}
```

### State Management (Frontend)

```javascript
// Contexte joueur dans React
const PlayerContext = {
  playerId: string,
  sessionId: string,
  playerName: string,
  teamName: string,
  answerColor: string,
  score: number,
  stats: PlayerStats,
  connected: boolean,
}
```

---

## Considérations techniques

### Performance

- [ ] **Optimisation mobile**
  - Bundle JS < 100KB (gzip)
  - Images optimisées (WebP, lazy loading)
  - Polling réduit, WebSocket uniquement
  - Throttling des animations sur mobile

- [ ] **Gestion de la latence**
  - Feedback optimiste (UI update immédiat)
  - Synchronisation serveur en arrière-plan
  - Gestion des conflits (ex: 2 joueurs buzzent simultanément)
  - Indicateur de latence réseau

### Sécurité

- [ ] **Authentification joueur**
  - Token de session unique généré par le serveur
  - Validation du token à chaque action
  - Expiration de session après inactivité
  - Protection contre usurpation d'identité

- [ ] **Rate limiting**
  - Limite de buzz par seconde (anti-spam)
  - Cooldown entre les actions
  - Détection de comportement anormal

- [ ] **Validation côté serveur**
  - Vérifier que le joueur peut buzzer (état READY/STARTED)
  - Vérifier que c'est bien son tour
  - Ignorer les actions invalides

### Accessibilité (WCAG 2.1)

- [ ] **Navigation clavier**
  - Boutons accessibles au clavier (Tab)
  - Raccourcis clavier (Espace = buzz)
  - Focus visible sur les éléments interactifs

- [ ] **Screen readers**
  - Labels ARIA sur tous les boutons
  - Annonces des changements d'état
  - Descriptions alternatives des images

- [ ] **Contraste et lisibilité**
  - Contraste minimum 4.5:1 pour le texte
  - Taille de texte minimum 16px
  - Mode haut contraste disponible

---

## Différences avec /tv

| Aspect | /tv (Affichage public) | /player (Interface joueur) |
|--------|------------------------|----------------------------|
| **Contenu** | Question + réponses + classements | Question + contrôles personnalisés + stats |
| **Interactivité** | Lecture seule (sauf admin) | Boutons de buzz/QCM/Memory |
| **Personnalisation** | Générique pour tous | Adapté au joueur et son équipe |
| **Layout** | Horizontal (TV 16:9) | Vertical (mobile portrait) |
| **Statistiques** | Globales (tous les joueurs) | Personnelles (joueur uniquement) |
| **Notifications** | Aucune | Push notifications |
| **État de connexion** | Toujours connecté | Peut se déconnecter/reconnecter |

---

## Priorités de développement

**Court terme (MVP)** :
- Phase 1 : Interface de base avec connexion et affichage
- Phase 2 : Bouton de buzz virtuel fonctionnel
- QCM : Boutons de réponse colorés

**Moyen terme** :
- Phase 3 : Contrôle Memory
- Phase 4 : Statistiques de base (score, historique)
- Phase 5 : Notifications et feedback haptique

**Long terme** :
- Phase 6 : Chat d'équipe et collaboration
- Phase 7 : PWA complète avec offline

---

## Maquettes (à créer)

### Écrans principaux

1. **Connexion** : Sélection équipe + nom + couleur QCM
2. **Attente** : "En attente du lancement de la question..."
3. **Jeu Normal** : Question + grand bouton BUZZ
4. **Jeu QCM** : Question + 4 boutons de réponse
5. **Jeu Memory** : Grille de cartes interactive
6. **Résultat** : Feedback + points gagnés + classement
7. **Statistiques** : Dashboard personnel

### Composants réutilisables

- `<PlayerHeader>` : Avatar + nom + score
- `<BuzzButton>` : Bouton de buzz avec états
- `<QCMButtons>` : 4 boutons de réponse
- `<MemoryGrid>` : Grille de cartes Memory
- `<PlayerStats>` : Widget de statistiques
- `<PlayerHistory>` : Liste des questions jouées
- `<TeamChat>` : Interface de chat

---

## Technologies suggérées

| Composant | Technologies |
|-----------|-------------|
| **Frontend** | React + TypeScript, TailwindCSS, Framer Motion |
| **State** | React Context + useReducer (ou Zustand) |
| **PWA** | Workbox (service worker), Web Push API |
| **Animations** | Framer Motion, CSS animations |
| **Haptique** | Vibration API (navigator.vibrate) |
| **Charts** | Recharts, Chart.js (stats) |

---

## Métriques de succès

| Métrique | Cible |
|----------|-------|
| **Latence buzz** | < 100ms (optimiste) |
| **Temps de chargement** | < 2s (3G) |
| **Taux d'adoption** | 50% des joueurs utilisent /player |
| **Satisfaction** | > 4/5 en feedback utilisateur |
| **Taux d'erreur** | < 1% d'actions échouées |

---

## Questions ouvertes

- [ ] **Gestion des déconnexions** : Comment gérer un joueur qui se déconnecte en plein jeu ?
  - Proposition : Garder son état 5 minutes, puis le marquer comme "absent"

- [ ] **Conflits de buzz** : Que se passe-t-il si un joueur buzz à la fois avec buzzer physique et virtuel ?
  - Proposition : Premier arrivé compte, ignorer le second

- [ ] **Limite de joueurs virtuels** : Combien de joueurs /player simultanés maximum ?
  - Proposition : Pas de limite technique, mais recommander < 50 pour performance

- [ ] **Mode spectateur pur** : Autoriser des spectateurs sans équipe ?
  - Proposition : Oui, avec route `/spectator` dédiée (lecture seule)

- [ ] **Reconnexion automatique** : Que faire en cas de perte WebSocket ?
  - Proposition : Tentatives exponentielles, restaurer l'état avec le sessionId
