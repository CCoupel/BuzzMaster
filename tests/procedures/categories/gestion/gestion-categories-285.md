# Procédure de Test — Gestion des catégories : renommage, couleur, icône, suppression et fusion de l'historique (#285)

**Version** : v11.4.0 (LOT-4)
**Date** : 2026-11-05
**Testeur** : QA (scénarios AUTO) / utilisateur (scénarios HUMAIN — **pont Hue réel et TV réelle** pour les scénarios 1 à 3)
**Contrats** : `contracts/http-endpoints.md` §Catégories et §Historique ; `contracts/lighting.md` §8 ; `contracts/models.md`
**Maquette** : `docs/mockup/v11.4.0/ui/admin_category_manager__rename_color_delete.html` (§01 sélecteur, §02 dialogue) — sauf l'état « icône sur une catégorie intégrée » et « ↺ emoji », écartés (icône réservée aux personnalisées)
**Décisions utilisateur (GATE 2)** : toute écriture est **refusée pendant une partie** (409) · une icône ne s'applique qu'aux catégories **personnalisées** · à la suppression, les points d'historique sont regroupés dans **« Sans catégorie »** (sauvegarde `history.json.bak-…`) · en mode « supprimer », les **cartes MEMOTION** de la catégorie sont retirées de leur question hôte (conservée).

## Prérequis

- [ ] Environnement : QUALIF avec **pont Hue réel** associé (scénarios 1–3) et une TV (`/tv`) ; page admin `/admin/questions` (éditeur) et `/admin/palmares`
- [ ] Une **sauvegarde** de la partie faite avant de commencer (scénarios 5–6 modifient l'historique)
- [ ] Jeu de données : catégorie personnalisée « Cinéma » (icône + couleur) avec **12 questions**, **1 carte MEMOTION** de cette catégorie dans une question hôte, **30 réponses RAFALE** ; **3 parties** déjà jouées créditant « Cinéma » (≈ 1 240 points) ; une catégorie personnalisée **vide** (« Vide »)
- [ ] Une question dont la catégorie est « Cinéma » prête à être lancée, et une question d'une catégorie **sans couleur**
- [ ] Les tests automatiques du lot passent (`tests/INDEX.md`, lignes #285)

## Scénarios

### Scénario 1 — Une catégorie colorée pilote l'éclairage Hue

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la question « Cinéma » (couleur bleu ciel `#0ea5e9`) | Les lampes passent au **bleu ciel** pendant la question ; néon TV de la même couleur | | |
| 2 | Pendant la question : tenter de changer la couleur de « Cinéma » | Refus « Impossible pendant une partie » (scénario 7) | | |
| 3 | Terminer la question (STOP) puis changer la couleur de « Cinéma » (palette → autre teinte) | Le dialogue applique la couleur ; aucune erreur | | |
| 4 | Relancer la question « Cinéma » | Les lampes prennent la **nouvelle** teinte ; badge et néon TV idem | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Une catégorie intégrée recolorée

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Hors partie : recolorer « Geographie » (intégrée) en vert | Pastille, badges (éditeur, anim, admin) et néon TV passent au vert | | |
| 2 | Lancer une question « Geographie » | Lampes **vertes** | | |
| 3 | Recharger la page, redémarrer le serveur | La couleur est conservée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — « Aucune » couleur ⇒ salle blanche

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Hors partie : palette de « Cinéma » → « aucune » | Pastille/badge **gris** ; aperçu 💡 « blanc » | | |
| 2 | Lancer une question « Cinéma » | Lampes **blanches** (pas de teinte) ; néon TV gris neutre | | |
| 3 | Lancer une question d'une catégorie personnalisée créée sans couleur | Lampes blanches | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Renommer une catégorie (intégrée et personnalisée)

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dialogue « Gérer les catégories » : ✎ sur « Histoire », saisir « Grande Histoire », Entrée | Le nouveau nom apparaît partout : éditeur, filtres, badges, palmarès, TV, anim | | |
| 2 | Échap pendant un renommage | Aucun changement | | |
| 3 | Ouvrir une question de cette catégorie | Son contenu et sa clé sont inchangés (seul l'affichage a changé) | | |
| 4 | Palmarès (`/admin/palmares`) | Les points anciens sont crédités sous le **nouveau** nom | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Supprimer une catégorie utilisée, « Garder les questions »

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter le total du palmarès (somme de tous les points) et la liste des fichiers de `data/config/` | Valeurs de référence | | |
| 2 | 🗑 sur « Cinéma » | Dialogue « Supprimer « Cinéma » — 12 questions et 30 … » ; **« Garder les questions, sans catégorie » présélectionné** ; encart 📊 « 3 parties ont marqué 1 240 points… » | | |
| 3 | Bouton « Supprimer la catégorie » avant de cocher la case | **Désactivé** | | |
| 4 | Cocher la case, confirmer | Catégorie disparue ; les 12 questions sont toujours là, « Sans catégorie » ; la carte MEMOTION aussi | | |
| 5 | Palmarès | Une ligne **« Sans catégorie »** crédite les 1 240 points ; **total du palmarès identique** à l'étape 1 | | |
| 6 | `data/config/` | Fichier `history.json.bak-AAAAMMJJ-HHMMSS` présent ; `history.json` valide | | |
| 7 | Réservoir RAFALE | Les 30 réponses existent mais ne sont pas tirables tant qu'elles n'ont pas de catégorie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Supprimer une catégorie utilisée, « Supprimer aussi les questions » (cartes MEMOTION comprises)

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Restaurer le jeu de données (étape de préparation) puis 🗑 « Cinéma » → « Supprimer aussi les questions » | Le texte compte les questions, les réponses RAFALE **et les cartes MEMOTION** supprimées ; la case de confirmation est **obligatoire** | | |
| 2 | Confirmer | Les 12 questions (médias inclus) et les 30 réponses du réservoir sont supprimées | | |
| 3 | Ouvrir la question hôte de la carte MEMOTION | **L'hôte existe encore**, sans la carte de « Cinéma » ; les autres cartes sont intactes | | |
| 4 | Palmarès | Points regroupés dans « Sans catégorie », total conservé ; `.bak` présent | | |
| 5 | Une manche RAFALE dont la liste de catégories contenait « Cinéma » | « Cinéma » n'y figure plus ; si la liste devient vide : « toutes catégories » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Pendant une partie, tout est bloqué (dialogue en lecture seule)

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question (phase en cours ou en pause) puis ouvrir « Gérer… » | Le dialogue s'ouvre et reste **consultable** | | |
| 2 | Tenter : changer une couleur, renommer, envoyer une icône, supprimer « Vide », créer une catégorie | Chaque action affiche **« Impossible pendant une partie »** ; rien n'est modifié | | |
| 3 | Arrêter la question (STOP) puis refaire une action | L'action réussit | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Icône : personnalisées seulement

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cliquer l'icône de « Cinéma », choisir un WebP | L'icône est remplacée partout (le navigateur ne montre plus l'ancienne) | | |
| 2 | Regarder une catégorie intégrée | Emoji statique avec la mention « emoji » ; **aucun** bouton d'envoi | | |
| 3 | Envoyer un fichier > 2 Mio, puis un fichier texte renommé `.png` | Refus nommé ; l'icône actuelle reste | | |
| 4 | Création d'une catégorie : nom + icône + couleur | Teinte proposée par défaut = première teinte libre ; la catégorie apparaît avec sa couleur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Restaurer l'historique depuis un `.bak`

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Serveur arrêté : remplacer `data/config/history.json` par un `history.json.bak-…` antérieur à la suppression, redémarrer | Le palmarès affiche de nouveau les points sous le nom d'origine pour les parties passées | | |
| 2 | Faire 6 suppressions successives de catégories utilisées | Au plus **5** fichiers `history.json.bak-…` conservés | | |
| 3 | Sauvegarde de la partie (`/game-backup`) | L'archive ne contient pas les `.bak` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 10 — Sécurité de l'envoi d'icône et des clés (équivalent automatisé)

**Validation** : AUTO — **Issues** : #285

**Objectif** : exécutable par QA ; équivalent dans `internal/server/categories_285_test.go`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `POST /api/categories/CINEMA/icon` avec un fichier nommé `../../x.png` (PNG réel) | Fichier écrit **dans** le dossier des catégories sous `CINEMA.png` ; rien hors dossier | | |
| 2 | Idem avec un HTML nommé `.png`, un PNG nommé `.webp`, un fichier de 2 Mio + 1 octet | 400, 400, 413 ; dossier inchangé | | |
| 3 | `PUT /api/categories/..%2F..%2Fetc` | 400 ou 404, aucun fichier créé | | |
| 4 | Toute écriture avec une partie en cours | 409 `{"error":"game in progress"}` (POST, PUT, icône, DELETE) | | |
| 5 | `DELETE` d'une intégrée / d'une inconnue | 400 / 404 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 11 — Rendu de l'interface vs maquette

**Validation** : HUMAIN — **Issues** : #285

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Comparer le sélecteur (« ⚙ Gérer… » à côté de « + Créer ») et la création à la maquette §01 | Conforme | | |
| 2 | Comparer le dialogue (colonnes, palette 12 teintes + « aucune », aperçu 💡, 🔒 sur les intégrées) et la confirmation de suppression à la maquette §02 | Conforme ; textes lisibles, rien ne déborde | | |
| 3 | Tablette (1024 px) et poste (1280 px) | Dialogue entièrement visible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux passent
- [ ] Aucun point d'historique perdu : somme du palmarès identique avant/après toute suppression
- [ ] Jamais de modification pendant une partie
- [ ] Les données existantes sont intactes tant qu'aucune action n'est faite (aucune migration)

## Notes QA

[Observations : couleur trop sombre ⇒ salle « éteinte » (palette bornée côté UI, hors palette assumé par le contrat) ; temps de réponse de `GET /api/categories` > 200 ms ⇒ à remonter]
