# Commande /cdp - Contrôle de l'Orchestrateur CDP

Commande de contrôle direct de l'agent CDP (Chef De Projet) pour interroger, modifier ou contrôler les workflows en cours.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/cdp help` → Affiche :

```
## /cdp - Aide

**Description** : Contrôle direct de l'orchestrateur CDP

**Usage** :
  /cdp help                          Afficher cette aide
  /cdp status                        État global des workflows
  /cdp abort                         Abandonner le workflow en cours
  /cdp pause                         Mettre en pause le workflow
  /cdp resume                        Reprendre un workflow en pause
  /cdp context "information"         Ajouter du contexte
  /cdp note "remarque"               Ajouter une note au rapport
  /cdp priority [high|normal|low]    Changer la priorité
  /cdp config                        Afficher la config CDP actuelle

**Différence avec /feature status** :
  /feature status → État du workflow FEATURE en cours
  /cdp status     → Vue globale de TOUS les workflows
```

## Mots-clés disponibles

| Mot-clé | Description | Exemple |
|---------|-------------|---------|
| `help` | Affiche cette aide | `/cdp help` |
| `status` | Vue globale des workflows | `/cdp status` |
| `abort` | Abandonner le workflow actuel | `/cdp abort` |
| `pause` | Mettre en pause | `/cdp pause` |
| `resume` | Reprendre après pause | `/cdp resume` |
| `context` | Ajouter du contexte | `/cdp context "Client utilise Safari"` |
| `note` | Ajouter note au rapport | `/cdp note "À revoir avec l'équipe"` |
| `priority` | Changer priorité | `/cdp priority high` |
| `config` | Afficher config CDP | `/cdp config` |

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- État persistant : section 9
- Phases : section 3

## Comportement par mot-clé

### `status` - Vue globale

Affiche l'état de tous les workflows :

```markdown
## CDP - État Global

**Workflow actif** : FEATURE "Ajouter mode Memory"
- Branche : feature/memory-mode
- Phase : DEV (3/7)
- Tâches : 2/5 complétées
- Démarré : il y a 2h

**Historique récent** :
- BUGFIX "Score calculation" → Complété (il y a 1j)
- FEATURE "QCM hints" → Complété (il y a 3j)

[Voir détails] | [Reprendre] | [Abandonner]
```

### `abort` - Abandonner

1. Demande confirmation
2. Nettoie l'état du workflow
3. Optionnel : propose de supprimer la branche

```markdown
⚠️ Abandonner le workflow FEATURE "Mode Memory" ?

**État actuel** : Phase DEV, 2/5 tâches complétées
**Branche** : feature/memory-mode

Options :
- [Confirmer] Abandonner et garder la branche
- [Confirmer + Supprimer] Abandonner et supprimer la branche
- [Annuler] Retourner au workflow
```

### `pause` / `resume` - Contrôle du flux

- `pause` : Sauvegarde l'état, permet de faire autre chose
- `resume` : Reprend exactement où on s'était arrêté

### `context` - Ajouter du contexte

Ajoute une information contextuelle utilisée par les sous-agents :

```
/cdp context "Le client utilise Safari, pas Chrome"
/cdp context "L'API externe a une limite de 100 req/min"
/cdp context "Le design doit suivre la maquette Figma v2"
```

Le contexte est transmis à tous les sous-agents (dev-backend, dev-frontend, etc.)

### `note` - Ajouter une note

Ajoute une note qui apparaîtra dans le rapport final :

```
/cdp note "À discuter avec l'équipe avant merge"
/cdp note "Performance à surveiller en production"
/cdp note "Tests manuels requis pour cette feature"
```

### `priority` - Changer la priorité

```
/cdp priority high    → Priorité haute (moins de validations)
/cdp priority normal  → Priorité normale (workflow standard)
/cdp priority low     → Priorité basse (plus de validations)
```

### `config` - Configuration CDP

Affiche la configuration actuelle de l'orchestrateur :

```markdown
## CDP - Configuration

**Mode** : Standard
**Validation utilisateur** : Activée
**Max cycles review/QA** : 3
**Auto-commit** : Désactivé
**Parallel agents** : Activé

[Modifier] | [Réinitialiser]
```

## État persistant CDP

Le CDP maintient un état global (voir `context/CDP_WORKFLOWS.md` section 9) :

```yaml
cdp_state:
  active_workflow:
    type: FEATURE
    description: "..."
    # ... détails du workflow
  context_additions:
    - "Client utilise Safari"
    - "API limite 100 req/min"
  notes:
    - "À discuter avec l'équipe"
  priority: normal
  paused: false
  history:
    - {type: BUGFIX, completed_at: "..."}
    - {type: FEATURE, completed_at: "..."}
```

## Intégration avec les commandes CDP

`/cdp` complète les commandes existantes :

| Besoin | Commande |
|--------|----------|
| Lancer une feature | `/feature <description>` |
| État de cette feature | `/feature status` |
| Vue globale CDP | `/cdp status` |
| Ajouter contexte | `/cdp context "..."` |
| Abandonner | `/cdp abort` |

## Action immédiate

Analyser `$ARGUMENTS` :

1. Si `help` → Afficher l'aide
2. Si `status` → Afficher état global
3. Si `abort` → Demander confirmation puis abandonner
4. Si `pause` → Sauvegarder état et mettre en pause
5. Si `resume` → Reprendre le workflow en pause
6. Si `context "..."` → Extraire le texte et l'ajouter au contexte
7. Si `note "..."` → Extraire le texte et l'ajouter aux notes
8. Si `priority <level>` → Changer la priorité
9. Si `config` → Afficher la configuration
10. Si vide ou non reconnu → Afficher l'aide
