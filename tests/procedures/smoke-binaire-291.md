# Procédure de Test — Smoke du binaire après découpage de `cmd/server` (#291)

**Version** : v11.5.0 · **Issue** : #291 (extraction de `vplayer`, `colors`, `coalesce`, `rounds`, `startup`, `diag`, `demo` hors de `cmd/server`)
**Testeur** : utilisateur, sous Windows, au GATE 4 (pas de QUALIF dans ce milestone) · **Durée** : ~10 min

> Le découpage ne change aucun comportement : cette recette vérifie qu'une session de jeu normale
> fonctionne de bout en bout avec l'exe construit depuis la branche. Un écart = NOT VALIDATED.

## Prérequis
- [ ] L'exe candidat fourni par le teamleader : `build/candidate_v11.5.0/buzzcontrol-candidate-<version>-windows-amd64.exe`
      (`<version>` = 11.5.0.N du dernier commit du milestone).
- [ ] Un navigateur desktop ; un second onglet ou écran pour `/tv`. Buzzers physiques facultatifs.

## Scénarios

### 1. Démarrage
1. Lancer : `.\buzzcontrol-candidate-<version>-windows-amd64.exe --port 9090`
2. Attendu : pas de crash, la console affiche la version et `HTTP Port: 9090`.
3. Ouvrir `http://localhost:9090/version` → texte = `<version>` du candidat.

### 2. Pages de démarrage
1. `http://localhost:9090/admin` s'affiche sans erreur (pas d'écran blanc).
2. `http://localhost:9090/tv` s'affiche (fond TV, pas de scroll).
3. `http://localhost:9090/player` s'affiche (écran d'inscription / QR code selon la config).
4. Si le serveur n'a pas de réseau : l'écran « pas de réseau disponible » apparaît avec les adresses détectées.

### 3. Une question complète (/admin + /tv)
1. Dans `/admin`, créer ou ouvrir une question (ex. la question de démo « Canberra »).
2. Cliquer **Préparer / READY** : `/tv` affiche la question, la phase passe à PREPARE puis READY (« Forcer prêt » si aucun buzzer).
3. Cliquer **Démarrer** : décompte puis phase STARTED, chronomètre visible sur `/tv`.
4. (Facultatif) un buzzer ou un joueur virtuel buzze : la TV affiche l'équipe, les points peuvent être attribués.
5. **Arrêter** : phase STOPPED, la réponse peut être révélée sur `/tv`, les scores se mettent à jour.
6. Rafraîchir `/admin` et `/tv` : l'état reste cohérent (pas de remise à zéro intempestive).

### 4. Fonctions touchées par le découpage
- [ ] Joueur virtuel : `/player` s'inscrit (QR code depuis `/admin`), son statut apparaît côté admin.
- [ ] Rounds / catégories : l'ordre et le contexte de manche s'affichent comme avant.
- [ ] Mode démo (si activé dans la config) : démarre et s'arrête sans erreur.
- [ ] Logs temps réel (`/ws/logs`, page Logs) : les lignes défilent.

### 5. Arrêt propre
1. `http://localhost:9090/shutdown` (ou Ctrl+C) → la console indique l'arrêt, le processus se termine.
2. Relancer l'exe : redémarrage sans erreur, données (équipes, scores) conservées.

## Critères
- VALIDATED : scénarios 1, 2, 3 et 5 conformes, aucun message d'erreur ni écran blanc.
- NOT VALIDATED : crash, phase bloquée, page vide, régression visible → noter le scénario et le message de la console.

## Pré-vérification automatique (QA, SHA 0b27d34a)
Binaire Linux construit depuis ce SHA : `/version` = 11.5.0.20 ; `/admin`, `/tv`, `/player` en HTTP 200 ;
cycle WebSocket admin READY → PREPARE → FORCE_READY → READY → START → COUNTDOWN → STARTED → STOP → STOPPED ;
`/shutdown` arrête le processus proprement.
