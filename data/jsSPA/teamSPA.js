import { getTeams, setTeamColor, setTeamName } from './configSPA.js';
import { createElement} from './interface.js';
import { configureDropzone, configureDragElement } from './dragAndDropSPA.js'; 

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

function getUsedColors(teams) {
    return new Set(Object.values(teams)
        .filter(team => team.COLOR)
        .map(team => team.COLOR.join(','))
    );
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
    if (!container) return;
    
    const colorSelectors = {};

    // Vider le conteneur
    container.innerHTML = '';

    // Créer la nouvelle dropzone qui occupe toute la zone
    const dropzoneText = createElement('h2', 'dropzone-text');
    dropzoneText.textContent = 'Déposez un joueur ici pour créer une nouvelle équipe';
    container.appendChild(dropzoneText);


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
        const editButton = createElement('button', 'edit-button');
        editButton.innerHTML = '✏️';
        editButton.title = 'Modifier le nom de l’équipe';
        
        editButton.addEventListener('click', () => {
            const newName = prompt('Nouveau nom pour cette équipe :', id);
        
            if (!newName || newName.trim() === '') {
                alert('Le nom est invalide.');
                return;
            }
        
            if (newName === id) {
                return;
            }
        
            if (teams[newName]) {
                alert('Le nom est déjà utilisé.');
                return;
            }
        
            setTeamName(id, newName);
        });


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
        const titleWrapper = createElement('div', 'team-title-wrapper');
        titleWrapper.appendChild(title);
        titleWrapper.appendChild(editButton);
        teamInfoDiv.append(titleWrapper, dropzone, colorLabel, colorSelect);
        teamDiv.append(colorDiv, teamInfoDiv);
        container.appendChild(teamDiv);
    }

    Object.keys(teams).forEach(id => addTeam(id, teams[id]));
};