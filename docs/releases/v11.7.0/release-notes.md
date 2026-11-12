# Release v11.7.0 - La lumière de la salle prend vie

**Date** : 2026-11-12

## Nouveautés

### Trois zones de lumière
La lumière d'ambiance se règle désormais par trois zones : **Ambiance** (la salle), **Animateur** (pupitre, régie) et **Public**. Chaque zone compte jusqu'à 12 groupes d'ampoules numérotés, que l'on réorganise par glisser-déposer dans le sous-onglet **Groupes** de Ambiance › Lumière. Un essai en séquence permet de repérer un groupe dans la salle : éteint, blanc, rouge, vert, bleu, puis retour à l'état de jeu.

- La zone Animateur reste en blanc fixe tant qu'aucun effet n'est choisi.
- La zone Public n'est pas touchée tant qu'aucun effet n'y est réglé : ses ampoules gardent leur état.

### Des effets de lumière pour chaque moment du jeu
Chaque moment (chronomètre, calme, score) reçoit son effet et sa vitesse, zone par zone. Cinq effets sont livrés :

- **Pulsation** : toute la zone respire au rythme du chronomètre.
- **Chenillard** : un point de lumière fait le tour des groupes, un par seconde.
- **Chenillard à traînée** : le même parcours, avec une queue lumineuse (à partir de 5 groupes).
- **Couleur d'équipe** : la salle prend la couleur de l'équipe qui marque.
- **Dégradé** : une couleur différente par groupe.

La vitesse se choisit parmi ×0,5, ×0,75, ×1, ×1,5 et ×2. Le bouton **Tester** joue l'effet sur sa zone avant la partie. Les réglages se font dans le sous-onglet **Effets** et, manche par manche, dans « Ambiance de la manche ».

### Un catalogue avec aperçu animé
Le sous-onglet Effets présente chaque effet sur une carte : sa description, ses moments, sa provenance (livré, personnalisé ou livré modifié) et l'endroit où il est utilisé. **▶ Aperçu** le rejoue sur des ampoules dessinées, sans toucher à la salle. **💡 Essayer sur la salle** le joue en vrai sur la zone Ambiance. Un fichier d'effet en erreur est signalé avec sa ligne et sa cause, sans bloquer les autres.

### Créez vos propres effets
Un effet est un simple fichier texte, modifiable à la main, déposé dans le dossier `data/files/effects/` du serveur. Il est rechargé automatiquement en quelques secondes. Le guide `docs/LIGHT_EFFECTS.md` explique comment partir d'un effet livré. Les effets personnalisés sont inclus dans la sauvegarde et la restauration (catégorie Ambiance).

## À noter

- **La pulsation change d'aspect** : phase basse blanche au-dessus de 10 s, rouge plus doux et battement plus régulier sous 5 s.
- Les groupes de lumière n'ont plus de nom : ils sont numérotés. Les anciens noms sont ignorés et disparaissent au prochain enregistrement.
- Les anciens réglages d'effets de l'animateur sont ignorés : la zone Animateur revient au blanc fixe, à régler de nouveau dans Effets › Animateur.
- Les anciens fichiers d'effets (format précédent) sont refusés avec un message explicite : repartez d'un effet livré.
- Avec un pont Philips Hue, les groupes du pont sont recréés par zone.

## Comment mettre à jour

Installez la version v11.7.0 depuis la page des releases, comme pour les versions précédentes, puis vérifiez les réglages d'effets de la zone Animateur.

## Liens

- [Documentation des effets](../../LIGHT_EFFECTS.md)
- [Guide administrateur](../../ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.7.0)
