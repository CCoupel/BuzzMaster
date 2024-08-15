import { ws } from './webSocket.js';
import { configureDragElement } from './dragAndDrop.js';

// Fonction principale pour créer et afficher les éléments buzzer en fonction des données fournies
export function createBuzzerDiv(buzzerData) {
    const container = document.querySelector('.buzzer-container');
    container.innerHTML = '';

    // Fonction pour créer un élément (paragraphe) avec un texte donné
    const createTextElement = (className, text) => {
        const textElement = document.createElement('p');
        textElement.className = className;
        textElement.textContent = text;
        return textElement;
    };

    // Fonction pour envoyer un message WebSocket
    const sendWebSocketMessage = (id, playerName = "") => {
        const message = {
            "bumpers": {
                [id]: {
                    "NAME": playerName,
                }
            }
        };
        ws.send(JSON.stringify(message));
    };

    // Fonction pour créer un formulaire de saisie de nom
    const createForm = (id, buzzerDiv, updateView) => {
        const form = document.createElement('form');
        form.className = 'buzzer-form';

        const label = document.createElement('label');
        label.textContent = 'Nom du joueur';
        label.htmlFor = `buzzer-text-${id}`;

        const input = document.createElement('input');
        input.type = 'text';
        input.name = 'buzzer-text';
        input.id = `buzzer-text-${id}`;
        input.placeholder = 'Nom du joueur';

        form.appendChild(label);
        form.appendChild(input);

        form.addEventListener('submit', (e) => {
            e.preventDefault();
            const playerName = input.value;
            sendWebSocketMessage(id, playerName);
            updateView(playerName);
        });

        buzzerDiv.appendChild(form);
    };

    // Fonction pour créer un élément buzzer complet
    const createBuzzerElement = (id, data) => {
        const buzzerDiv = document.createElement('div');
        buzzerDiv.id = `buzzer-${id}`;
        buzzerDiv.className = 'buzzer';
        configureDragElement(buzzerDiv);

        const idElement = createTextElement('buzzer-id', `ID: ${id}`);
        buzzerDiv.appendChild(idElement);

        const updateView = (playerName) => {
            buzzerDiv.innerHTML = '';
            if (playerName) {
                const nameElement = createTextElement('buzzer-name', `Nom: ${playerName}`);
                const deleteButton = document.createElement('button');
                deleteButton.textContent = 'Supprimer le nom';
                deleteButton.addEventListener('click', () => {
                    sendWebSocketMessage(id);
                    updateView(""); 
                });
                buzzerDiv.appendChild(nameElement);
                buzzerDiv.appendChild(deleteButton);
            } else {
                buzzerDiv.appendChild(idElement);
                createForm(id, buzzerDiv, updateView);
            }
        };

        updateView(data.NAME || "");

        return buzzerDiv;
    };

    // Boucle sur chaque buzzer pour les créer et les afficher
    for (const [id, data] of Object.entries(buzzerData.bumpers)) {
        if (document.getElementById(`buzzer-${id}`)) continue;

        const buzzerDiv = createBuzzerElement(id, data);

        if (data.TEAM && document.getElementById(data.TEAM)) {
            const teamDropzone = document.getElementById(data.TEAM).querySelector('.dropzone');
            teamDropzone.appendChild(buzzerDiv);
        } else {
            container.appendChild(buzzerDiv);
        }
    }
}
