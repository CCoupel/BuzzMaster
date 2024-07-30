from os.path import join
from SCons.Script import DefaultEnvironment

env = DefaultEnvironment()

# Définir des répertoires de build distincts
env.Replace(
    BUILD_DIR_FIRMWARE=join("$PROJECT_DIR", ".pio", "build", "nodemcu_firmware"),
    BUILD_DIR_FS=join("$PROJECT_DIR", ".pio", "build", "nodemcu_filesystem")
)


# Appliquer le répertoire de build pour le système de fichiers
def set_fs_build_dir(source, target, env):
    print("CCO: source: ", source[0].name)
    print ("CCO: target: ", target[0].name)
    # Modifier le répertoire de build si la cible est le système de fichiers
    if "buildfs" in target[0].name:
        env.Replace(BUILD_DIR=env['BUILD_DIR_FS'])
        env.Replace(build_dir=env['BUILD_DIR_FS'])
        print("Setting build directory to filesystem:", env['BUILD_DIR'])
    else:
        env.Replace(BUILD_DIR=env['BUILD_DIR_FIRMWARE'])
        env.Replace(platformio_BUILD_DIR=env['BUILD_DIR_FIRMWARE'])
        print("Setting build directory to firmware:", env['BUILD_DIR'])


# Enregistre la fonction à appeler avant le build de chaque cible
print("CCO: change build dir: ")
env.AddPreAction("buildfs", set_fs_build_dir)
env.AddPreAction("buildprog", set_fs_build_dir)
