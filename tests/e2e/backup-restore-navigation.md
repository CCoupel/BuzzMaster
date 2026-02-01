# Scénarios E2E - Backup/Restaure Navigation

Teste l'accès et la navigation vers la nouvelle page Backup/Restaure via le menu abeille.

## Prérequis
- Serveur démarré sur http://localhost
- Admin interface accessible
- Tests exécutés avec MCP claude-in-chrome

---

## Scénario 1 : Accès via menu abeille (Admin)

### Objectif
Vérifier que le lien "Backup/Restaure" apparaît dans le menu abeille et mène à la bonne page.

### Étapes
1. Ouvrir http://localhost/admin
2. Attendre chargement navbar
3. Cliquer sur le logo abeille (menu déroulant)
4. Vérifier présence du lien "Backup/Restaure" avec icon 💾
5. Cliquer sur "Backup/Restaure"
6. Vérifier navigation vers /admin/backup
7. Vérifier titre "Sauvegarde et Restauration"
8. Vérifier présence de 3 sections : Sauvegarde, Restauration, Réinitialisation

### Vérifications
- URL: http://localhost/admin/backup
- Titre page: "Sauvegarde et Restauration"
- Textes présents: "Selectionnez les elements", "Sauvegarder", "Restaurer", "Reinitialiser"
- Menu abeille se ferme après clic

---

## Scénario 2 : Accès via menu abeille (Anim)

### Objectif
Vérifier que le lien fonctionne aussi avec le préfixe /anim

### Étapes
1. Ouvrir http://localhost/anim
2. Cliquer sur logo abeille
3. Cliquer sur "Backup/Restaure"
4. Vérifier navigation vers /anim/backup
5. Vérifier titre et sections présentes

### Vérifications
- URL: http://localhost/anim/backup
- Page charge correctement avec same layout

---

## Scénario 3 : Checkboxes Backup

### Objectif
Vérifier que les checkboxes de sauvegarde sont fonctionnels

### Étapes
1. Naviguer vers http://localhost/admin/backup
2. Dans section Sauvegarde, vérifier 5 checkboxes : Questions, Equipes, Joueurs, Historique, Fonds
3. Tous les checkboxes doivent être cochés par défaut
4. Décocher "Questions"
5. Vérifier que "Questions" reste décochée

### Vérifications
- Checkboxes présents : questions, teams, bumpers, history, backgrounds
- État initial : tous cochés
- Changement d'état fonctionne

---

## Scénario 4 : Checkboxes Réinitialisation

### Objectif
Vérifier que les checkboxes de réinitialisation sont fonctionnels

### Étapes
1. Dans section Réinitialisation, vérifier 5 checkboxes identiques
2. Tous les checkboxes doivent être décochés par défaut
3. Cocher "Equipes"
4. Vérifier que "Equipes" reste coché

### Vérifications
- Checkboxes présents : questions, teams, bumpers, history, backgrounds
- État initial : tous décochés
- Changement d'état fonctionne

---

## Scénario 5 : Configuration page (ConfigPage)

### Objectif
Vérifier que les sections Backup/Reset ont été supprimées de ConfigPage

### Étapes
1. Naviguer vers http://localhost/admin/settings
2. Vérifier présence des sections : Neon, Server Params, Demo
3. Vérifier ABSENCE des sections : Sauvegarde, Réinitialisation
4. Vérifier présence de "Remettre les scores a zero" (section Système)

### Vérifications
- Sections Backup/Reset NON présentes dans ConfigPage
- Bouton "Remettre les scores a zero" toujours présent
- Page charge correctement

---

## Scénario 6 : Responsive Design (Mobile)

### Objectif
Vérifier l'affichage sur mobile

### Étapes
1. Redimensionner fenêtre à 768x1024 (tablet)
2. Naviguer vers /admin/backup
3. Vérifier que les 3 sections s'affichent en colonne unique
4. Vérifier que les checkboxes restent lisibles
5. Vérifier que les boutons sont cliquables

### Vérifications
- Layout single-column
- Texte lisible
- Boutons accessibles

---

## Scénario 7 : Menu abeille navigation courante

### Objectif
Vérifier que la page Backup est bien intégrée dans la navigation

### Étapes
1. Ouvrir /admin
2. Cliquer abeille → vérifier menu items
3. Compter nombre d'items dans menu déroulant : Config, Backup/Restaure, Logs = 3 items
4. Vérifier ordre : Config, Backup/Restaure, Logs (dans cet ordre)

### Vérifications
- 3 items dans menu
- Ordre correct
- Icône 💾 affichée pour Backup/Restaure
