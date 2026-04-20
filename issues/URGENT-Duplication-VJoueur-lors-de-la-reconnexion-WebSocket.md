### Issue Title: URGENT: Duplication VJoueur lors de la reconnexion WebSocket

#### Description:
Nous avons rencontré un problème critique concernant la duplication des instances de `VJoueur` lors de la reconnexion au WebSocket, ce qui pourrait entraîner des incohérences dans l'état du jeu.

#### Bug Analysis:
Ce bug semble surgir lorsque plusieurs connexions WebSocket sont établies sans que les précédentes soient correctement fermées, générant des duplications d'objets et nécessitant une gestion stricte de l'état de connexion.

#### Steps to Reproduce:
1. Connectez-vous à l'application Web via WebSocket.
2. Déconnectez-vous intentionnellement sans fermer la connexion.
3. Réessayez de vous reconnecter.
4. Observez que plusieurs instances de `VJoueur` sont maintenant créées.

#### Root Cause:
Le problème est probablement causé par la gestion inefficace des événements de connexion et de déconnexion, entraînant des fuites de mémoire et la création multiple d'instances d'objets `VJoueur`.

#### Proposed Solutions:
1. **Condtion de vérification avant création:** Implémentez une condition pour vérifier si une instance de `VJoueur` existe déjà pour un joueur avant de créer une nouvelle instance.
2. **Gestion améliorée des connexions WebSocket:** Assurez-vous que toutes les connexions WebSocket sont correctement fermées avant d'initialiser une nouvelle connexion.
3. **Déduplication des instances:** Introduisez une méthode de déduplication dans le manager de `VJoueur` pour fusionner les instances existantes lors de la reconnexion.

#### Priority: Critical
