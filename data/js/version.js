const filePathWebVersion = '../config/version.txt'
const CoreVersion = ''


export function getWebVersion() {
    fetch(filePathWebVersion)
        .then(response => {
            if (!response.ok) {
            throw new Error('Erreur lors du chargement du fichier');
            }
            return response.text(); // Lit le contenu comme texte
        })
        .then(data => {
            document.getElementById('web-version').textContent = " Web Version: " + data;
        })
        .catch(error => {
            console.error('Erreur :', error);
            document.getElementById('web-version').textContent = "Erreur lors de l'affichage de la version.";
        });
}

export function getCoreVersion(data) {
    const version = data;
    console.log(version)
    if (version) {
        document.getElementById('core-version').textContent = "Core Version: " + version;
    } else {
      document.getElementById('core-version').textContent = "Erreur lors de l'affichage de la version.";  
    }
}