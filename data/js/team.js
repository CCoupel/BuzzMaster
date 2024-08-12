import { ws } from './webSocket.js';

export function createTeamDiv(teams) {
    const container = document.querySelector('.team-container');

    for (const id in teams) {
        if (document.getElementById(id)) {
            continue;
        }

        const newDiv = document.createElement('div');
        newDiv.className = 'dynamic-div';
        newDiv.id = id;

        const title = document.createElement('h2');
        title.className = 'team-name';
        title.textContent = id;
        newDiv.appendChild(title);

        const text = document.createElement('p');
        text.className = 'team-member-p';
        text.textContent = 'Membres de la team :';
        newDiv.appendChild(text);

        const dropzone = document.createElement('div');
        dropzone.className = 'dropzone';
        dropzone.textContent = 'Glissez les membres de la team ici';
        newDiv.appendChild(dropzone);

        // Ajoute des événements de drag-and-drop à la dropzone
        dropzone.addEventListener('dragover', (e) => {
            e.preventDefault(); // Nécessaire pour permettre le drop
            dropzone.classList.add('dropzone-hover'); // Optionnel : pour ajouter un style lorsque la zone est survolée
        });

        dropzone.addEventListener('dragleave', () => {
            dropzone.classList.remove('dropzone-hover'); // Optionnel : pour retirer le style lorsque la zone n'est plus survolée
        });

        dropzone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropzone.classList.remove('dropzone-hover');

            const draggedElementId = e.dataTransfer.getData('text');
            const draggedElement = document.getElementById(draggedElementId);

            if (draggedElement && draggedElement.classList.contains('buzzer')) {
                dropzone.appendChild(draggedElement);

                const updatedBuzzerMessage = {
                    "bumpers": {
                        [draggedElementId.replace('buzzer-', '')]: { // Remplace 'buzzer-' dans l'ID pour obtenir l'ID original du bumper
                            "TEAM": id,
                            // Vous pouvez inclure les autres propriétés de buzzer ici si nécessaire
                        }
                    }
                };

                ws.send(JSON.stringify(updatedBuzzerMessage));

                console.log(`Élément ${draggedElementId} mis à jour avec la team ${id}`);
            }
        });

        const colorLabel = document.createElement('label');
        colorLabel.htmlFor = `color-select-${id}`;
        colorLabel.textContent = 'Choisir une couleur:';
        newDiv.appendChild(colorLabel);

        const colorSelect = document.createElement('select');
        colorSelect.id = `color-select-${id}`;

        const colors = [
            { name: 'bleu', rgb: [0, 0, 255] },
            { name: 'rouge', rgb: [255, 0, 0] },
            { name: 'jaune', rgb: [255, 255, 0] },
            { name: 'vert', rgb: [0, 128, 0] },
            { name: 'violet', rgb: [128, 0, 128] },
            { name: 'orange', rgb: [255, 165, 0] },
            { name: 'rose', rgb: [255, 192, 203] }
        ];

        colors.forEach(color => {
            const option = document.createElement('option');
            option.value = color.name;
            option.textContent = color.name.charAt(0).toUpperCase() + color.name.slice(1);
            colorSelect.appendChild(option);
        });
        newDiv.appendChild(colorSelect);

        colorSelect.addEventListener('change', () => {
            const selectedColorName = colorSelect.value;
            const selectedColor = colors.find(color => color.name === selectedColorName);

            const message = {
                "teams": {
                    [id]: {
                        "color": selectedColor.rgb
                    }
                }
            };

            ws.send(JSON.stringify(message));

            console.log(`Couleur envoyée pour l'équipe ${id}: ${selectedColor.rgb}`);
        });

        container.appendChild(newDiv);
    }

    const button = document.getElementById('addDivButton');
    container.appendChild(button);
}