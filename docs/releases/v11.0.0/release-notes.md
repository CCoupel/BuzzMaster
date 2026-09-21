# BuzzControl v11.0.0 - "La salle prend le son"

**Date de sortie** : 21 septembre 2026

---

## 🎉 Quoi de neuf ?

### 🔊 Sept bruitages qui rythment la soirée, sans que l'animateur y pense

BuzzControl réagit désormais aux temps forts de la partie par le son : départ du chronomètre, fin du temps, bonne réponse, mauvaise réponse, révélation de la réponse, début et fin d'entracte. Sept signaux sonores couvrent tout le déroulé d'une manche — l'animateur pilote la partie, le son suit tout seul.

**Bénéfice** : l'ambiance sonore d'un vrai jeu télévisé, sans un bouton de plus à gérer pendant le jeu.

### 🔈 Une vraie sortie audio, sur Windows comme sur Raspberry Pi

Les sons sont diffusés directement sur l'enceinte branchée au serveur — y compris en **Bluetooth**, aussi bien sur un PC Windows que sur un Raspberry Pi. Pas de matériel supplémentaire à installer : le pilote pense à préchauffer la connexion au démarrage pour que le tout premier son ne soit jamais raté, et se reconnecte tout seul si l'enceinte se déconnecte en cours de soirée.

### 🎵 Des sons livrés d'origine, remplaçables par les vôtres

Chaque version de BuzzControl embarque ses sept bruitages par défaut, générés directement par le logiciel (aucun fichier à télécharger, aucune question de droits). Vous pouvez à tout moment remplacer un ou plusieurs sons par les vôtres, écouter le résultat dans le navigateur, l'envoyer en test réel sur l'enceinte, ou revenir au son d'origine en un clic.

### 🎛️ Un nouvel onglet "Son" dans l'administration

La page `Ambiance` (qui pilotait déjà votre éclairage Philips Hue depuis la v10.0.0) accueille un nouvel onglet **Son**, juste à côté de l'onglet Lumière. Vous y retrouvez les sept bruitages sous forme de tableau : état (par défaut ou personnalisé), durée, et trois actions rapides — écouter, tester sur l'enceinte, restaurer. Un interrupteur général coupe tous les sons d'un coup (effet immédiat), et chaque bruitage a aussi son propre interrupteur pour l'ajuster finement.

### 🚦 Une pastille d'état dans la barre de navigation

Comme pour l'éclairage Hue, une pastille dédiée dans la Navbar indique en un coup d'œil si le son est actif ou non — pratique pour vérifier avant de lancer la soirée que tout est prêt.

---

## ℹ️ À savoir avant de mettre à jour

- Cette fonctionnalité est **entièrement optionnelle** : sans configuration particulière, les sons par défaut sont prêts à l'emploi ; un simple interrupteur permet de tout désactiver.
- **Activer le son pour la première fois demande un redémarrage du serveur** (initialisation de la connexion audio) ; le désactiver, en revanche, est immédiat.
- Fonctionne aussi bien avec l'enceinte intégrée d'un PC qu'avec une enceinte Bluetooth appairée au serveur (Windows et Raspberry Pi).

## Comment mettre à jour

1. Téléchargez la dernière release sur [GitHub Releases](https://github.com/CCoupel/BuzzMaster/releases)
2. Remplacez l'exécutable existant par la nouvelle version
3. Relancez le serveur — vos questions, équipes et scores existants sont conservés
4. Rendez-vous dans `Ambiance` → onglet `Son` pour écouter, personnaliser ou désactiver les bruitages

## Liens

- [Documentation](https://github.com/CCoupel/BuzzMaster/blob/main/docs/ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.0.0)
- [Signaler un problème](https://github.com/CCoupel/BuzzMaster/issues)
