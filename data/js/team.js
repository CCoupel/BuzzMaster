import { ws } from './webSocket.js';
import { configureDropzone } from './dragAndDrop.js'; 

export function createTeamDiv(teams) {
    const container = document.querySelector('.team-container');

    for (const id in teams) {
        if (document.getElementById(id)) {
            continue;
        }

        const newDiv = document.createElement('div');
        newDiv.className = 'dynamic-div';
        newDiv.id = id;

        const colorDiv = document.createElement('div');
        colorDiv.className = 'color-div';

        if (teams[id] && teams[id].color) {
            colorDiv.style.backgroundColor = `rgb(${teams[id].color.join(',')})`;
        }

        newDiv.appendChild(colorDiv);

        const teamInfoDiv = document.createElement('div');
        teamInfoDiv.className = 'team-info';
        newDiv.appendChild(teamInfoDiv);

        const title = document.createElement('h2');
        title.className = 'team-name';
        title.textContent = id;
        teamInfoDiv.appendChild(title);

        const dropzone = document.createElement('div');
        dropzone.className = 'dropzone';
        dropzone.textContent = 'Glissez les membres de la team ici';
        teamInfoDiv.appendChild(dropzone);

        configureDropzone(dropzone, ws, id);

        const colorLabel = document.createElement('label');
        colorLabel.htmlFor = `color-select-${id}`;
        colorLabel.textContent = 'Choisir une couleur:';
        teamInfoDiv.appendChild(colorLabel);

        const colorSelect = document.createElement('select');
        colorSelect.id = `color-select-${id}`;

        const colors = [
            { name: 'choisir une couleur', rgb: [255, 255, 255] },
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
        teamInfoDiv.appendChild(colorSelect);

        if (teams[id] && teams[id].color) {
            const existingColor = colors.find(color => 
                color.rgb[0] === teams[id].color[0] && 
                color.rgb[1] === teams[id].color[1] && 
                color.rgb[2] === teams[id].color[2]
            );
            if (existingColor) {
                colorSelect.value = existingColor.name;
            }
        }

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

            colorDiv.style.backgroundColor = `rgb(${selectedColor.rgb.join(',')})`;

            console.log(`Couleur envoyée pour l'équipe ${id}: ${selectedColor.rgb}`);
        });

        container.appendChild(newDiv);
    }

    const button = document.getElementById('addDivButton');
    container.appendChild(button);
}
