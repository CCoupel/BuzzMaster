document.addEventListener('DOMContentLoaded', () => {
    const button = document.getElementById('addDivButton');
    const container = document.querySelector('.team-container');

    button.addEventListener('click', () => {
        // Créer la nouvelle div
        const newDiv = document.createElement('div');
        newDiv.className = 'dynamic-div';
        
        // Créer et ajouter le titre <h2>
        const title = document.createElement('h2');
        title.textContent = 'Titre de la div';
        newDiv.appendChild(title);
        
        // Créer et ajouter le champ de texte
        const textField = document.createElement('input');
        textField.type = 'text';
        textField.placeholder = 'Entrez du texte ici';
        newDiv.appendChild(textField);
        
        // Créer et ajouter la zone de dépôt
        const dropzone = document.createElement('div');
        dropzone.className = 'dropzone';
        dropzone.textContent = 'Déposez vos fichiers ici';
        newDiv.appendChild(dropzone);

        // Ajouter la nouvelle div au conteneur
        container.appendChild(newDiv);
        
        // Ajouter le bouton après la nouvelle div
        container.appendChild(button);
    });
});