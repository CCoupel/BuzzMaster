import { setBumperName, getBumpers, createElement} from './main.js';
import { configureDragElement } from './dragAndDrop.js';

export function createBuzzerDiv(buzzerData) {
    const container = document.querySelector('.buzzer-container');

    container.innerHTML = '';

    const newBuzzerText = createElement('h2', 'dropzone-text');
    newBuzzerText.textContent = 'Déposez un joueur  ou une équipe ici';
    container.appendChild(newBuzzerText);

    const createTextElement = (className, text) => {
        const textElement = document.createElement('p');
        textElement.className = className;
        textElement.textContent = text;
        return textElement;
    };

    const createForm = (id, buzzerDiv, playerName = '') => {
        const form = document.createElement('form');
        form.className = 'buzzer-form';
    
        const input = document.createElement('input');
        input.type = 'text';
        input.name = 'buzzer-text';
        input.id = `buzzer-text-${id}`;
        input.placeholder = 'Nom du joueur';
        input.value = playerName;  

        setTimeout(() => {
            input.focus(); 
        }, 0);

        let timeoutId;
        input.addEventListener('input', (e) => {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                const newPlayerName = e.target.value.trim();
                if (newPlayerName) {
                    setBumperName(id, newPlayerName);  
                }
            }, 500); // Délai de 500ms avant l'envoi
        });

        input.addEventListener('blur', () => {
            const currentPlayerName = input.value.trim() || playerName;  
            buzzerDiv.innerHTML = ''; 
            const nameElement = createTextElement('buzzer-name', `Nom: ${currentPlayerName}`);  
            buzzerDiv.appendChild(nameElement);
    
            nameElement.addEventListener('click', () => {
                buzzerDiv.innerHTML = '';
                createForm(id, buzzerDiv, currentPlayerName);
            });
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

                nameElement.addEventListener('click', () => {
                    buzzerDiv.innerHTML = '';  
                    createForm(id, buzzerDiv, playerName);
                });
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
    const nameElement = buzzerDiv.querySelector('.buzzer-name');
    if (nameElement && data.NAME) {
        nameElement.textContent = `Nom: ${data.NAME}`;
    }
}