# Release v11.2.2 - Un barème de points par carte et une alerte quand le réseau tombe

**Date** : 2026-10-15

## Nouveautés

### Un barème de points par carte MEMOTION
L'éditeur de carte MEMOTION propose une ligne « Points » pour choisir comment la carte rapporte des points. Pour une
carte à réponse simple (SPEEDY, QCM) : les étoiles (par défaut) ou un montant fixe. Pour une carte à progression
(MEMORY, RAFALE) : le prorata des étoiles (par défaut), les étoiles pleines, un montant fixe ou un montant par unité.
Le montant de la carte s'affiche dans le zoom et le bandeau de l'animateur, sur le bouton de crédit et dans le panneau
admin ; avant le jeu, le prorata indique « jusqu'à N pts ». Un montant hors de 1 à 999 est signalé dans l'éditeur.

### Une alerte plein écran quand le serveur n'a plus de réseau
Si le serveur perd son adresse réseau, l'admin voit un message plein écran « Le serveur n'a pas de réseau disponible »
sur toutes les pages d'administration. « Continuer quand même » le replie en bandeau rouge qui indique depuis combien de
temps le réseau est perdu. Au retour du réseau, un message « ✅ Réseau rétabli » s'affiche quelques secondes. Les
interfaces virtuelles (WSL, Hyper-V, VMware, VirtualBox, docker…) ne comptent pas comme un réseau. La TV, l'animateur et
les tablettes ne sont pas concernés.

### Des sélecteurs de catégorie et de difficulté cohérents
Partout où l'on choisit une catégorie ou une difficulté (éditeurs Quiz et RAFALE, filtres, génération par IA), les
sélecteurs sont les mêmes et indiquent clairement si l'on peut en choisir une seule ou plusieurs.

## Corrections

- Un refus du serveur à l'enregistrement d'une carte MEMORY, MEMOTION, RAFALE ou ENTRACTE s'affiche désormais au-dessus du bouton d'enregistrement (auparavant l'enregistrement échouait sans explication).
- Le bandeau « serveur sans réseau », ajouté en v5.7.0 mais jamais affiché, fonctionne enfin : c'est l'alerte ci-dessus.
- Les cartes QCM MEMOTION n'ont plus de réglages d'indices sans effet ; ceux déjà enregistrés sont retirés sans erreur.
- Un client qui envoyait son premier message dès l'ouverture de sa connexion pouvait ne pas être reconnu (joueur, tablette, buzzer) : corrigé.

## Sous le capot

Fiabilité renforcée : tests rendus plus robustes, correction d'une course dans le journal de debug, mise à jour des
dépendances et de l'outillage web (Node 22 requis pour compiler l'interface), CI sur Node 24. Sans impact sur le jeu.

## Comment mettre à jour

Installez la version v11.2.2 depuis la page des releases (ou via la mise à jour OTA de l'interface d'administration).
Les anciens clients restent compatibles.

## Liens

- [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.2)
