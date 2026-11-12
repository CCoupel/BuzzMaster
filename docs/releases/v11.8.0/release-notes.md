# Release v11.8.0 - Créez vos effets de lumière, sans écrire de fichier à la main

**Date** : 2026-11-12

## Nouveautés

### Un éditeur d'effets pas à pas
Dans Ambiance › Lumière › Effets, **＋ Nouvel effet**, **✎ Modifier**, **⧉ Dupliquer** et **👁 Ouvrir** (effets livrés, en lecture seule) ouvrent un éditeur visuel :

- Une **grille** pas × groupes : on peint chaque case avec des pinceaux et une roue de couleur (6 groupes par défaut, 12 au plus).
- Un **panneau de variables** pour faire courir une lumière d'un groupe à l'autre, avec des **expressions** (départ et pas : nombres, nombre de groupes `N`, dernier groupe `g*`, `+ - * /`, parenthèses).
- Un onglet **Texte** pour ceux qui préfèrent le fichier ; vos commentaires sont conservés quand vous enregistrez depuis le Texte.
- Un **simulateur** pas à pas sur 1, 3, 6 ou 12 groupes, ou sur les zones réelles de votre salle, pour vérifier qu'un effet tient sur plusieurs tailles de salle.
- L'**essai du brouillon par zone**, sans enregistrer : Ambiance, Animateur ou Public.
- Une **garde de sortie** : fermer l'onglet ou quitter la page avec un effet modifié demande confirmation.

### Importer, télécharger, supprimer
Le catalogue accepte désormais vos fichiers d'effets : **⭱ Importer…** ou glisser-déposer, jusqu'à 20 fichiers `.yaml` à la fois. Chaque fichier est vérifié seul ; le résultat indique « importé », « remplacé », « gardé en double » ou « refusé » avec la ligne et la cause. Si l'identifiant existe déjà, choisissez **Remplacer**, **Garder les deux** ou **Annuler**.

- **⭳ Télécharger** donne le fichier de n'importe quel effet, livrés compris : un point de départ pour partager ou modifier.
- **🗑 Supprimer** retire un effet à vous ; **↺ Revenir à l'effet d'origine** restaure un effet livré modifié.
- **Essayer sur la salle** demande maintenant la zone (Ambiance, Animateur ou Public) ; un effet qui demande plus de groupes que la zone n'en a est grisé, avec la raison.

## À noter — changement cassant

- **`gN` devient `g*`.** Le dernier groupe de la zone s'écrit désormais `g*`. Seuls sont concernés les fichiers d'effets **écrits à la main pour la v11.7.0** qui emploient `gN` : ils sont refusés avec le message « … écrivez g* » (le catalogue les signale, les autres effets restent chargés). Correction : remplacer `gN` par `g*` (`gD: gN; -1` devient `gD: g*; -1`). **Aucun effet livré n'est concerné.**
- Un effet créé avec la v11.8.0 qui utilise `g*`, une expression ou plus de 8 variables ne se charge pas sur une installation v11.7.0 : ne le copiez pas vers une version plus ancienne.
- Les limites montent : jusqu'à 24 variables par effet et 40 actions par pas.

## Comment mettre à jour

Installez la version v11.8.0 depuis la page des releases, comme pour les versions précédentes, puis, si vous avez écrit des effets à la main, remplacez `gN` par `g*`.

## Liens

- [Documentation des effets](../../LIGHT_EFFECTS.md)
- [Guide administrateur](../../ADMIN_GUIDE.md)
- [GitHub Release](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.8.0)
