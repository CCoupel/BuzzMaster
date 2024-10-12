const filePathVersion = '../config/version.txt'


export function getVersion() {
    fetch(filePathVersion)
        .then(response => {
            if (!response.ok) {
            throw new Error('Erreur lors du chargement du fichier');
            }
            return response.text(); // Lit le contenu comme texte
        })
        .then(data => {
            document.getElementById('web-version').textContent = " Web Version: " + data; // Affiche le texte dans l'élément <pre>
        })
        .catch(error => {
            console.error('Erreur :', error);
            document.getElementById('web-version').textContent = "Erreur lors de l'affichage de la version.";
        });
}