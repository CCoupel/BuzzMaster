# Procédure de Qualification

Ce document décrit la phase de qualification avant mise en production.

---

## Vue d'ensemble du Cycle

```
┌─────────────────┐     Validation     ┌─────────────────┐     Validation     ┌─────────────────┐
│  DEV + TEST     │ ─────────────────► │  QUALIF + TEST  │ ─────────────────► │  RELEASE        │
│  (Développement)│       ✓            │  (Qualification)│                    │  (Production)   │
└─────────────────┘                    └─────────────────┘                    └─────────────────┘
                                              ▲
                                              │ Vous êtes ici
```

---

## Prérequis

- [ ] Phase DEV terminée et validée par l'utilisateur
- [ ] Serveur fonctionnel en local
- [ ] Toutes les fonctionnalités implémentées

---

## 1. Préparation de l'Environnement de Qualification

### 1.1 Nettoyage

```bash
cd server-go

# Arrêter le serveur en cours
taskkill /IM server.exe /F 2>nul

# Nettoyer les fichiers temporaires
rm -f nul server-output.txt server-error.txt
rm -f test-report.txt test-summary.txt
rm -f *.bak web/src/pages/*.bak
```

### 1.2 Rebuild complet

**⚠️ IMPORTANT : TOUJOURS rebuilder le frontend AVANT le backend Go.**

```bash
cd server-go

# 1. Frontend d'abord (OBLIGATOIRE)
cd web
npm run build
cd ..

# 2. Backend Go ensuite (embarque les fichiers web)
go build -o server.exe ./cmd/server
```

**Note** : Le serveur Go embarque automatiquement les fichiers `web/dist/`. Si vous inversez l'ordre, les modifications frontend ne seront pas prises en compte.

### 1.3 Démarrer le serveur

```bash
./server.exe
```

Vérifier :
- [ ] `[HTTP] Server starting on port 80`
- [ ] `[TCP] Server started on port 1234`
- [ ] `[HTTP] Using embedded web files (portable mode)`

---

## 2. Tests Unitaires Go

### 2.1 Exécution complète

```bash
cd server-go
go test ./... -v -cover 2>&1 | tee test-report.txt
```

### 2.2 Critères de validation

| Critère | Seuil | Commande de vérification |
|---------|-------|--------------------------|
| Tests passés | 100% | `grep -c "FAIL" test-report.txt` → doit être 0 |
| Couverture globale | ≥60% | Visible dans la sortie |
| Pas de race condition | 0 | `go test -race ./...` |

### 2.3 En cas d'échec

```
┌─────────────────────────────────────────┐
│  ÉCHEC TESTS UNITAIRES                  │
│                                         │
│  → Retour en phase DEV                  │
│  → Corriger les tests en échec          │
│  → Relancer la qualification            │
└─────────────────────────────────────────┘
```

---

## 3. Tests Fonctionnels (E2E)

### 3.1 Checklist Interface Admin (/)

| Test | Action | Résultat attendu | ✓ |
|------|--------|------------------|---|
| Chargement | Ouvrir http://localhost/ | Page admin s'affiche | |
| Version | Vérifier navbar | Version correcte affichée | |
| WebSocket | Vérifier indicateur | Point vert "Connecté" | |
| Questions | Liste des questions | Questions affichées | |
| Équipes | Liste des équipes | 6 équipes avec scores | |
| Navigation | Cliquer chaque onglet | Toutes les pages s'ouvrent | |

### 3.2 Checklist Interface TV (/tv)

| Test | Action | Résultat attendu | ✓ |
|------|--------|------------------|---|
| Chargement | Ouvrir http://localhost/tv | Page TV s'affiche | |
| Fond | Vérifier background | Image de fond visible | |
| Sync | Changer vue admin | TV se met à jour | |
| Plein écran | F11 | Affichage plein écran | |

### 3.3 Checklist Flux de Jeu

| Test | Action | Résultat attendu | ✓ |
|------|--------|------------------|---|
| Sélection question | Cliquer une question | Question sélectionnée (PREPARE) | |
| Démarrage | Cliquer START | Timer démarre | |
| Buzz simulé | Ctrl+clic joueur | Joueur marqué comme ayant buzzé | |
| Pause | Cliquer PAUSE | Timer en pause | |
| Attribution | Cliquer sur joueur | Points attribués | |
| Révélation | Cliquer RÉPONSE | Réponse affichée | |

### 3.4 Checklist par Type de Question

#### Questions NORMAL
- [ ] Création fonctionne
- [ ] Affichage question/réponse
- [ ] Attribution points joueur/équipe

#### Questions QCM
- [ ] 4 réponses colorées affichées
- [ ] Réponse correcte marquée
- [ ] Pastilles équipes sur réponses

#### Questions MEMORY (si applicable)
- [ ] Grille de cartes affichée
- [ ] Phase mémorisation fonctionne
- [ ] Sélection de cartes
- [ ] Matching correct

---

## 4. Tests de Non-Régression

### 4.1 Fonctionnalités Critiques

| Fonctionnalité | Test | ✓ |
|----------------|------|---|
| Persistance équipes | Redémarrer serveur, vérifier équipes | |
| Persistance scores | Redémarrer serveur, vérifier scores | |
| Persistance historique | Vérifier page Historique | |
| Upload question | Créer nouvelle question avec image | |
| Suppression question | Supprimer une question | |
| Backup | GET /backup télécharge un .tar | |
| Restore | POST /restore restaure les données | |

### 4.2 Multi-clients

| Test | Action | Résultat attendu | ✓ |
|------|--------|------------------|---|
| 2 admins | Ouvrir 2 onglets admin | Les 2 se synchronisent | |
| Admin + TV | Ouvrir admin + TV | TV suit les actions admin | |
| Compteurs | Vérifier navbar | Compteurs A et TV corrects | |

---

## 5. Tests de Performance (optionnel)

### 5.1 Temps de réponse

| Opération | Seuil acceptable |
|-----------|------------------|
| Chargement page | < 2s |
| Changement de vue | < 500ms |
| WebSocket latence | < 100ms |

### 5.2 Mémoire

```bash
# Surveiller la mémoire du serveur
# Windows : Task Manager
# Linux : htop ou top
```

Seuil : < 100 MB en utilisation normale

---

## 6. Rapport de Qualification

### Template

```
========================================
RAPPORT DE QUALIFICATION
Date: YYYY-MM-DD
Version: x.y.z
========================================

TESTS UNITAIRES
---------------
Total: XX/XX PASS
Couverture: XX.X%
Race conditions: 0

TESTS FONCTIONNELS
------------------
Interface Admin: XX/XX PASS
Interface TV: XX/XX PASS
Flux de jeu: XX/XX PASS
Types de questions: XX/XX PASS

NON-RÉGRESSION
--------------
Persistance: PASS/FAIL
Multi-clients: PASS/FAIL
Backup/Restore: PASS/FAIL

ANOMALIES DÉTECTÉES
-------------------
1. [Description anomalie]
   Sévérité: Bloquant / Majeur / Mineur

========================================
RÉSULTAT: QUALIFIÉ / NON QUALIFIÉ
========================================
```

---

## 7. Gestion des Anomalies

### 7.1 Anomalie Bloquante

```
┌─────────────────────────────────────────┐
│  ANOMALIE BLOQUANTE                     │
│                                         │
│  → STOP qualification                   │
│  → Retour en phase DEV                  │
│  → Corriger l'anomalie                  │
│  → Reprendre qualification depuis le    │
│    début                                │
└─────────────────────────────────────────┘
```

### 7.2 Anomalie Majeure

- Documenter l'anomalie
- Décision utilisateur : corriger maintenant ou reporter
- Si correction : retour en DEV
- Si report : documenter dans le backlog

### 7.3 Anomalie Mineure

- Documenter l'anomalie
- Peut continuer la qualification
- Ajouter au backlog pour correction ultérieure

---

## 8. Checklist Fin de Qualification

- [ ] Tous les tests unitaires passent
- [ ] Tous les tests fonctionnels passent
- [ ] Non-régression validée
- [ ] Aucune anomalie bloquante
- [ ] Rapport de qualification complété
- [ ] Version stable identifiée

---

## 9. Passage en Phase RELEASE

Quand la qualification est réussie :

1. **Présenter le rapport** : Montrer le rapport de qualification à l'utilisateur
2. **Attendre validation** : L'utilisateur doit explicitement valider
3. **Passer à** : [RELEASE_PROCEDURE.md](RELEASE_PROCEDURE.md)

**Note** : Ne jamais passer en RELEASE sans :
- Qualification complète réussie
- Accord explicite de l'utilisateur
