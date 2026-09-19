---
name: publish.qualif
description: "Adaptations projet BuzzControl pour PUBLISH QUALIF. Base generique : publish.qualif.template.md."
---

# Publish QUALIF — Adaptations BuzzControl

> **Base** : Voir `publish.qualif.template.md` pour le mecanisme `promote` generique (verification,
> echec, rollback). Ce fichier ne remplace que l'etape [2. Promotion] avec le nommage reel de
> l'artefact BuzzControl (exe Windows, pas de tar.gz).

```bash
# 2. Promotion — memes conventions de dossier/nommage que le template generique, artefact reel
REPO_ROOT=$(git rev-parse --show-toplevel)
MILESTONE_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9]*\.[0-9]*\.[0-9]*\)\..*/\1/')
FULL_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
BUILD_DIR="$REPO_ROOT/build/candidate_v${MILESTONE_VERSION}"
QUALIF_DIR="$REPO_ROOT/build/qualif_v${MILESTONE_VERSION}"

test -f "$BUILD_DIR/buzzcontrol-candidate-${FULL_VERSION}-windows-amd64.exe" || {
  echo "Aucun candidat trouve — executer BUILD d'abord"; exit 1;
}

mkdir -p "$QUALIF_DIR"
cp "$BUILD_DIR/buzzcontrol-candidate-${FULL_VERSION}-windows-amd64.exe" \
   "$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe"

echo "Publication QUALIF terminee - $FULL_VERSION -> $QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe"
```

> ⚠️ **Règle** : ne jamais demander la validation QUALIF à l'utilisateur avant que cette
> promotion soit terminée — l'utilisateur teste systématiquement depuis Windows.
