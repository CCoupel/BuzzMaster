import { setBumperName, getBumpers, createElement} from './main.js';
import { configureDragElement } from './dragAndDrop.js';

export function createBuzzerDiv(buzzerData) {
    const container = document.querySelector('.buzzer-container');


    // Vider le conteneur
    container.innerHTML = '';

    // Créer la nouvelle dropzone qui occupe toute la zone
    const newBuzzerText = createElement('h2', 'dropzone-text');
    newBuzzerText.textContent = 'Déposez un joueur  ou une équipe ici';
    container.appendChild(newBuzzerText);

    const createTextElement = (className, text) => {
        const textElement = document.createElement('p');
        textElement.className = className;
        textElement.textContent = text;
        return textElement;
    };

    const createForm = (id, buzzerDiv, updateView) => {
        const form = document.createElement('form');
        form.className = 'buzzer-form';

        const input = document.createElement('input');
        input.type = 'text';
        input.name = 'buzzer-text';
        input.id = `buzzer-text-${id}`;
        input.placeholder = 'Nom du joueur';

        let timeoutId;
        input.addEventListener('input', (e) => {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                const playerName = e.target.value.trim();
                if (playerName) {
                    setBumperName(id,playerName )
                    //updateView(playerName);
                }
            }, 500); // Délai de 500ms avant l'envoi
        });

        form.appendChild(input);
        buzzerDiv.appendChild(form);
    };

    const createBuzzerElement = (id, data) => {
        const buzzerDiv = document.createElement('div');
        buzzerDiv.id = `buzzer-${id}`;
        buzzerDiv.className = 'buzzer';
        configureDragElement(buzzerDiv);

        const idElement = createTextElement('buzzer-id', `ID: ${id}`);

        const updateView = (playerName) => {
            buzzerDiv.innerHTML = '';
            if (playerName) {
                const nameElement = createTextElement('buzzer-name', `Nom: ${playerName}`);
                buzzerDiv.appendChild(nameElement);

                const editButton = document.createElement('button');
                editButton.textContent = 'Modifier';
                editButton.addEventListener('click', () => {
                    createForm(id, buzzerDiv, updateView);
                });
                buzzerDiv.appendChild(editButton);
            } else {
                buzzerDiv.appendChild(idElement);
                createForm(id, buzzerDiv, updateView);
            }
        };

        updateView(data.NAME || "");

        return buzzerDiv;
    };

    for (const [id, data] of Object.entries(getBumpers())) {
        let buzzerDiv = document.getElementById(`buzzer-${id}`);
        
        if (!buzzerDiv) {
            buzzerDiv = createBuzzerElement(id, data);
        } else {
            updateBuzzerInfo(buzzerDiv, data);
        }

        if (data.TEAM && document.getElementById(data.TEAM)) {
            const teamDropzone = document.getElementById(data.TEAM).querySelector('.dropzone');
            teamDropzone.appendChild(buzzerDiv);
        } else {
            container.appendChild(buzzerDiv);
        }
    }
}


function updateBuzzerInfo(buzzerDiv, data) {
    // Mettre à jour les informations du buzzer ici si nécessaire
    // Par exemple, mettre à jour le nom si changé
    const nameElement = buzzerDiv.querySelector('.buzzer-name');
    if (nameElement && data.NAME) {
        nameElement.textContent = `Nom: ${data.NAME}`;
    }
}