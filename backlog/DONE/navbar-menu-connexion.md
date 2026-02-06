# Menu déroulant sur la pastille de connexion

**Statut** : ✅ Complété (v2.49.0)

## Description

Transformer la pastille de connexion dans la navbar en un menu déroulant (abeille 🐝) contenant les liens vers Configuration, Logs, Backup et Mises à jour, simplifiant la barre de navigation principale.

## Objectifs

- [x] Simplifier la navbar en réduisant le nombre de liens visibles
- [x] Regrouper les fonctions utilitaires (Config, Logs, Backup, Mises à jour) dans un menu secondaire
- [x] Conserver l'affichage du compteur de connexions visible en permanence

## Tâches

### Phase 1 - Menu déroulant ✅
- [x] Transformer la pastille en bouton cliquable (logo abeille 🐝)
- [x] Créer le composant menu déroulant
- [x] Déplacer les liens Config et Logs dans le menu
- [x] Fermeture au clic extérieur

### Phase 2 - Styles ✅
- [x] Style du menu cohérent avec le reste de l'UI
- [x] Animation d'ouverture/fermeture
- [x] Indicateur visuel que c'est cliquable (hover, curseur)

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `web/src/components/Navbar.jsx` | Menu déroulant abeille |
| `web/src/components/Navbar.css` | Styles du menu, animations |

## Version

v2.49.0
