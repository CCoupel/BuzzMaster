const filePathWebVersionPrimary = '/config/version.txt';
const filePathWebVersionFallback = '../config/version.txt';
const CoreVersion = ''


export function getWebVersion() {
    fetch(filePathWebVersionPrimary)
        .then(response => {
            if (!response.ok) {
                if (response.status === 404) {
                    // Si le fichier n'est pas trouvé, essayer le chemin alternatif
                    return fetch(filePathWebVersionFallback).then(fallbackResponse => {
                        if (!fallbackResponse.ok) {
                            throw new Error('Impossible de trouver le fichier de version.');
                        }
                        return fallbackResponse.text();
                    });
                }
                throw new Error('Erreur lors du chargement du fichier.');
            }
            return response.text();
        })
        .then(data => {
            document.getElementById('web-version').textContent = "Web Version: " + data;
        })
        .catch(error => {
            console.error('Erreur :', error);
            document.getElementById('web-version').textContent = "Erreur lors de l'affichage de la version.";
        });
};

export function getCoreVersion(data) {
    const version = data;
    console.log(version)
    if (version) {
        document.getElementById('core-version').textContent = "Core Version: " + version;
    } else {
      document.getElementById('core-version').textContent = "Erreur lors de l'affichage de la version.";  
    }
}


getWebVersion()