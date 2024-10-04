import { getTeams, setTeamColor, addNewTeam, addBumperToTeam } from './main.js';
import { configureDropzone, configureDragElement } from './dragAndDrop.js'; 

const colors = [
    { name: 'choisir une couleur', rgb: [255, 255, 255] },
    { name: 'blanc', rgb: [255, 255, 255] },
    { name: 'bleu', rgb: [0, 0, 255] },
    { name: 'rouge', rgb: [255, 0, 0] },
    { name: 'jaune', rgb: [255, 255, 0] },
    { name: 'vert', rgb: [0, 128, 0] },
    { name: 'violet', rgb: [128, 0, 128] },
    { name: 'orange', rgb: [255, 165, 0] },
    { name: 'rose', rgb: [255, 192, 203] }
];

function getUsedColors(teams) {
    return new Set(Object.values(teams)
        .filter(team => team.COLOR)
        .map(team => team.COLOR.join(','))
    );
}

function createElement(tag, className, attributes = {}) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, value));
    return element;
}

function createColorOptions(selectElement, usedColors) {
    selectElement.innerHTML = '';
    colors.forEach(color => {
        if (!usedColors.has(color.rgb.join(','))) {
            const option = createElement('option', '', { value: color.name });
            option.textContent = color.name.charAt(0).toUpperCase() + color.name.slice(1);
            selectElement.appendChild(option);
        }
    });

    if (!selectElement.options.length) {
        const noColorOption = createElement('option', '', { value: '' });
        noColorOption.textContent = 'Aucune couleur disponible';
        selectElement.appendChild(noColorOption);
    }
}

function updateColorSelectors(colorSelectors, teams) {
    const usedColors = getUsedColors(teams);
    Object.values(colorSelectors).forEach(select => createColorOptions(select, usedColors));
}

function handleColorChange(id, colorSelect, colorDiv, colorSelectors) {
    return () => {
        const selectedColor = colors.find(color => color.name === colorSelect.value);
        if (selectedColor && !getUsedColors(getTeams()).has(selectedColor.rgb.join(','))) {
            setTeamColor(id, selectedColor.rgb);
            colorDiv.style.backgroundColor = `rgb(${selectedColor.rgb.join(',')})`;
            updateColorSelectors(colorSelectors, getTeams());
        } else {
            alert('Cette couleur est déjà utilisée par une autre équipe.');
            colorSelect.value = '';
        }
    };
}

export function createTeamDiv(teams) {
    const container = document.querySelector('.team-container');
    const colorSelectors = {};

    // Vider le conteneur
    container.innerHTML = '';

    // Créer la nouvelle dropzone qui occupe toute la zone
    const newTeamDropzone = document.createElement('div', 'new-team-dropzone');
    const dropzoneText = document.createElement('p', 'team-dropzone-text');
    dropzoneText.textContent = 'Déposez un joueur ici pour créer une nouvelle équipe';
    newTeamDropzone.appendChild(dropzoneText);

    container.appendChild(newTeamDropzone);

    configureDropzone(newTeamDropzone, '', (playerId) => {
        console.log("Création d'une nouvelle équipe pour le joueur:", playerId);
        const playerElement = document.getElementById(`buzzer-${playerId}`);
        const playerName = playerElement?.querySelector('.buzzer-name')?.textContent.split(': ')[1] || 'Nouvelle équipe';
        const newTeamId = playerName.replace(/\s+/g, '_').toLowerCase();
        const newTeamData = { COLOR: [255, 255, 255] }; // Couleur par défaut        

        // Ajouter la nouvelle équipe localement
        addNewTeam(newTeamId);
        setTeamColor(newTeamId, [255, 255, 255]);
        addBumperToTeam(playerId, newTeamId);
        //teams[newTeamId] = newTeamData;
        addTeam(newTeamId);

        // Déplacer le joueur dans la nouvelle équipe
        const teamDropzone = document.getElementById(newTeamId).querySelector('.dropzone');
        if (playerElement && teamDropzone) {
            teamDropzone.appendChild(playerElement);
        }
    });

    function addTeam(id, teamData) {
        const teamDiv = createElement('div', 'dynamic-div', { id });
        configureDragElement(teamDiv);
        
        const colorDiv = createElement('div', 'color-div');
        const teamInfoDiv = createElement('div', 'team-info');
        const title = createElement('h2', 'team-name');
        const dropzone = createElement('div', 'dropzone');
        const colorLabel = createElement('label', '', { for: `color-select-${id}` });
        const colorSelect = createElement('select', '', { id: `color-select-${id}` });

        title.textContent = id;

        const dropzoneText = createElement('p', 'dropzone-text');
        dropzoneText.textContent = 'Glissez les membres de la team ici';
        dropzone.appendChild(dropzoneText);

        colorLabel.textContent = 'Choisir une couleur:';

        configureDropzone(dropzone);

        if (teamData?.COLOR) {
            colorDiv.style.backgroundColor = `rgb(${teamData.COLOR.join(',')})`;
            const existingColor = colors.find(color => 
                color.rgb.every((value, index) => value === teamData.COLOR[index])
            );
            if (existingColor) colorSelect.value = existingColor.name;
        }

        createColorOptions(colorSelect, getUsedColors(getTeams()));
        colorSelectors[id] = colorSelect;

        colorSelect.addEventListener('change', handleColorChange(id, colorSelect, colorDiv, colorSelectors));

        teamInfoDiv.append(title, dropzone, colorLabel, colorSelect);
        teamDiv.append(colorDiv, teamInfoDiv);
        container.appendChild(teamDiv);
    }

    Object.keys(teams).forEach(id => addTeam(id, teams[id]));
}