import { ws } from './webSocket.js';
import { configureDropzone } from './dragAndDrop.js'; 

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

function updateUsedColors(teams) {
    const usedColors = new Set();
    Object.values(teams).forEach(team => {
        if (team.color) {
            usedColors.add(team.color.join(','));
        }
    });
    return usedColors;
}

function updateColorSelectOptions(colorSelect, usedColors) {
    colorSelect.innerHTML = ''; // Clear existing options
    colors.forEach(color => {
        if (!usedColors.has(color.rgb.join(','))) {
            const option = document.createElement('option');
            option.value = color.name;
            option.textContent = color.name.charAt(0).toUpperCase() + color.name.slice(1);
            colorSelect.appendChild(option);
        }
    });

    if (colorSelect.options.length === 0) {
        const option = document.createElement('option');
        option.value = '';
        option.textContent = 'Aucune couleur disponible';
        colorSelect.appendChild(option);
    }
}

function updateAllColorSelectors(colorSelectors, teams) {
    const usedColors = updateUsedColors(teams);
    Object.values(colorSelectors).forEach(colorSelect => {
        updateColorSelectOptions(colorSelect, usedColors);
    });
}

export function createTeamDiv(teams) {
    const container = document.querySelector('.team-container');
    const colorSelectors = {}; // Object to keep track of color select elements by ID

    function addTeam(id, teamData) {
        if (document.getElementById(id)) {
            return; // Skip if team already exists
        }

        const newDiv = document.createElement('div');
        newDiv.className = 'dynamic-div';
        newDiv.id = id;

        const colorDiv = document.createElement('div');
        colorDiv.className = 'color-div';

        if (teamData && teamData.color) {
            colorDiv.style.backgroundColor = `rgb(${teamData.color.join(',')})`;
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

        // Store the color selector for later updates
        colorSelectors[id] = colorSelect;

        teamInfoDiv.appendChild(colorSelect);

        // Initial color options setup
        updateColorSelectOptions(colorSelect, updateUsedColors(teams));

        if (teamData && teamData.color) {
            const existingColor = colors.find(color => 
                color.rgb[0] === teamData.color[0] && 
                color.rgb[1] === teamData.color[1] && 
                color.rgb[2] === teamData.color[2]
            );
            if (existingColor) {
                colorSelect.value = existingColor.name;
            }
        }

        colorSelect.addEventListener('change', () => {
            const selectedColorName = colorSelect.value;
            const selectedColor = colors.find(color => color.name === selectedColorName);

            if (selectedColor && !updateUsedColors(teams).has(selectedColor.rgb.join(','))) {
                const message = {
                    "teams": {
                        [id]: {
                            "color": selectedColor.rgb
                        }
                    }
                };

                ws.send(JSON.stringify(message));

                colorDiv.style.backgroundColor = `rgb(${selectedColor.rgb.join(',')})`;

                // Update all color selectors
                updateAllColorSelectors(colorSelectors, { ...teams, [id]: { ...teamData, color: selectedColor.rgb } });

                console.log(`Couleur envoyée pour l'équipe ${id}: ${selectedColor.rgb}`);
            } else {
                alert('Cette couleur est déjà utilisée par une autre équipe.');
                colorSelect.value = ''; // Reset the selection
            }
        });

        container.appendChild(newDiv);
    }

    // Add existing teams
    Object.keys(teams).forEach(id => addTeam(id, teams[id]));
}
