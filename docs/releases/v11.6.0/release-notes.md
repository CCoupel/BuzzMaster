# Release v11.6.0 - Une ambiance différente à chaque manche

**Date** : 2026-11-09

## Nouveautés

### Ambiance réglable par manche
Le son coupé, le décompte, les quinze bruitages et le mode d'éclairage se règlent désormais à trois niveaux : global, manche, geste direct (champ par champ). Une manche applique ses réglages à l'entrée de ses questions ; un geste direct de l'animateur vaut jusqu'au changement de manche ; une manche ne modifie jamais les réglages globaux. Le réglage se fait depuis le bouton 🎛 « Ambiance de la manche » de l'éditeur. « Tout remettre à Hérité » revient à l'ambiance globale. Quand la manche impose un réglage, l'interface indique « Appliqué à la manche en cours seulement ».

### Des sons propres à chaque manche
Une manche peut remplacer le son de chacun des 15 bruitages et celui du décompte par un fichier à elle, depuis la fenêtre « Ambiance de la manche ». Un fichier absent ou invalide joue le son global. Supprimer une manche supprime ses fichiers.

### Huit nouveaux bruitages par type de jeu
QCM (indice), MEMORY (carte, paire), MEMOTION (sélection, fin de carte) et RAFALE (bonne, mauvaise, fin de lot). Chacun a un son par défaut, remplaçable et désactivable. L'onglet Son de l'administration est réparti en Global, QCM, MEMORY, MEMOTION et RAFALE. À noter : le bruitage « perdu » ne sonne plus que pour une paire ratée en MEMORY ; en RAFALE, « rafale-mauvaise » le remplace.

### Du son sur plus de types de question
Le son d'une question s'étend à MEMORY, à l'hôte MEMOTION (« Son de la manche ») et à l'entracte (« Son de l'entracte », une fois ou en boucle). Chaque carte MEMOTION peut aussi avoir son propre son : au retournement, il remplace le son en cours. Pour MEMORY, le réglage est enregistré et n'agit que sur le chronomètre global existant.

### Des WAV de tout format
Un WAV non canonique n'est plus refusé : il est converti à l'envoi (44 100 Hz, stéréo 16 bits). Sont acceptés : PCM 8, 16, 24 ou 32 bits, flottant 32 bits, 1 ou 2 canaux, 8 000 à 192 000 Hz. Les durées maximales sont inchangées ; le MP3 reste refusé.

### Diagnostic de la sortie audio
L'onglet Son affiche la cause d'un son inactif (indisponible, désactivé) et une bannière « Redémarrage requis » quand c'est nécessaire.

## Compatibilité
Aucun changement incompatible : les fichiers existants (`config.json`, `rounds.json`) se chargent et se jouent comme avant. Nouveau réglage optionnel `lighting.default_mode` (absent = AUTO).

## Comment mettre à jour
Remplacez l'exécutable du serveur comme d'habitude. Le firmware des buzzers n'est pas concerné.

## Liens
- [Journal des modifications](https://github.com/CCoupel/BuzzMaster/blob/main/CHANGELOG.md)
- [Releases GitHub](https://github.com/CCoupel/BuzzMaster/releases)
