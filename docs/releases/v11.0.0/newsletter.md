# BuzzControl v11.0.0 est disponible !

Depuis la v10.0.0, la salle s'allumait avec vous. Avec cette version, elle se met aussi à parler : BuzzControl accompagne désormais chaque temps fort de la partie d'un bruitage, sans que l'animateur n'ait à y penser.

## Les grandes nouveautés

### Sept sons pour rythmer toute une manche

Départ du chronomètre, fin du temps, bonne réponse, mauvaise réponse, révélation de la réponse, début et fin d'entracte : sept bruitages couvrent l'intégralité du déroulé d'une question. L'animateur continue de piloter la partie normalement — le son se déclenche tout seul, au bon moment, à chaque fois.

[Capture à ajouter — onglet Son de la page Ambiance]

### Une vraie sortie audio, y compris sans fil

Les sons sont diffusés directement sur l'enceinte branchée au serveur — Bluetooth compris, aussi bien sur un PC Windows que sur un Raspberry Pi. Le pilote préchauffe la connexion au démarrage pour que le tout premier son ne soit jamais manqué, et se reconnecte automatiquement si l'enceinte se déconnecte en pleine soirée, sans jamais bloquer la partie.

### Vos propres sons, en un upload

Chaque bruitage est livré avec un son par défaut généré directement par le logiciel — zéro fichier à télécharger, zéro question de licence. Depuis le nouvel onglet **Son** de la page `Ambiance` (juste à côté de l'onglet Lumière introduit avec Philips Hue), vous pouvez écouter chaque son dans le navigateur, le remplacer par le vôtre, l'envoyer en test réel sur l'enceinte du serveur, ou revenir au son d'origine en un clic.

### Deux niveaux de contrôle, pour ne jamais être pris au dépourvu

Un interrupteur général coupe tous les sons d'un coup (effet immédiat), et chaque bruitage dispose aussi de son propre interrupteur pour un réglage plus fin. Une pastille d'état dans la barre de navigation — juste à côté de celle de l'éclairage Hue — indique en un coup d'œil si le son est prêt avant de lancer la soirée.

## Ce qu'il vous faut

Cette fonctionnalité est **entièrement optionnelle** — les sons par défaut fonctionnent sans aucune configuration, et un interrupteur général permet de tout désactiver à tout moment. Pour la sortie audio, il vous faut simplement une enceinte branchée (filaire ou Bluetooth) sur la machine qui héberge le serveur BuzzControl.

## Migration

Aucune action requise. Mettez à jour votre exécutable comme d'habitude : vos questions, équipes, scores et réglages existants sont conservés à l'identique. Un point d'attention : **activer le son pour la première fois nécessite un redémarrage du serveur** (la connexion audio ne s'initialise qu'au démarrage) ; le désactiver, en revanche, est immédiat.

## Merci

Un grand merci à celles et ceux qui ont testé cette fonctionnalité sur le terrain, sur Windows comme sur Raspberry Pi, jusqu'à la validation finale — cette version leur doit beaucoup.

[Télécharger](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.0.0)  [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)  [GitHub](https://github.com/CCoupel/BuzzMaster)
