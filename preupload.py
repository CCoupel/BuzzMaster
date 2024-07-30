Import("env")

# Commande pour téléverser le système de fichiers
def before_upload(source, target, env):
    # Exécuter la commande pour téléverser les fichiers du système de fichiers
    env.Execute("pio run --target uploadfs")

# Enregistre la fonction à appeler avant le téléversement
env.AddPostAction("upload", before_upload)
