import { ws } from './webSocket.js';
import { configureDragElement } from './dragAndDrop.js'; 

export function createBuzzerDiv(buzzerData) {
    const container = document.querySelector('.buzzer-container');
    container.innerHTML = '';

    const createBuzzerElement = (id, data) => {
        const buzzerDiv = document.createElement('div');
        buzzerDiv.id = `buzzer-${id}`;
        buzzerDiv.className = 'buzzer';

        configureDragElement(buzzerDiv);

        const idElement = document.createElement('p');
        idElement.className = 'buzzer-id';
        idElement.textContent = `ID: ${id}`;
        buzzerDiv.appendChild(idElement);

        const createForm = () => {
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
            input.value = data.NAME || '';

            form.appendChild(label);
            form.appendChild(input);

            form.addEventListener('submit', (e) => {
                e.preventDefault();
                const playerName = input.value;

                const message = {
                    "bumpers": {
                        [id]: {
                            "NAME": playerName,
                            "TEAM": data.TEAM,
                            "IP": data.IP,
                            "TIME": data.TIME,
                            "BUTTON": data.BUTTON
                        }
                    }
                };

                ws.send(JSON.stringify(message));

                const nameElement = document.createElement('p');
                nameElement.className = 'buzzer-name';
                nameElement.textContent = `Nom: ${playerName}`;

                const deleteButton = document.createElement('button');
                deleteButton.textContent = 'Supprimer le nom';
                deleteButton.addEventListener('click', () => {
                    const deleteMessage = {
                        "bumpers": {
                            [id]: {
                                "NAME": "",
                                "TEAM": data.TEAM,
                                "IP": data.IP,
                                "TIME": data.TIME,
                                "BUTTON": data.BUTTON
                            }
                        }
                    };

                    ws.send(JSON.stringify(deleteMessage));

                    buzzerDiv.removeChild(nameElement);
                    buzzerDiv.removeChild(deleteButton);
                    createForm();
                });

                buzzerDiv.replaceChild(nameElement, form);
                buzzerDiv.appendChild(deleteButton);
            });

            buzzerDiv.appendChild(form);
        };

        if (data.NAME) {
            const nameElement = document.createElement('p');
            nameElement.className = 'buzzer-name';
            nameElement.textContent = `Nom: ${data.NAME}`;

            const deleteButton = document.createElement('button');
            deleteButton.textContent = 'Supprimer le nom';
            deleteButton.addEventListener('click', () => {
                const message = {
                    "bumpers": {
                        [id]: {
                            "NAME": "",
                            "TEAM": data.TEAM,
                            "IP": data.IP,
                            "TIME": data.TIME,
                            "BUTTON": data.BUTTON
                        }
                    }
                };

                ws.send(JSON.stringify(message));

                buzzerDiv.removeChild(nameElement);
                buzzerDiv.removeChild(deleteButton);
                createForm();
            });

            buzzerDiv.appendChild(nameElement);
            buzzerDiv.appendChild(deleteButton);
        } else {
            createForm();
        }

        return buzzerDiv;
    };

    for (const [id, data] of Object.entries(buzzerData.bumpers)) {
        if (document.getElementById(`buzzer-${id}`)) {
            continue;
        }

        const buzzerDiv = createBuzzerElement(id, data);

        if (data.TEAM && document.getElementById(data.TEAM)) {
            const teamDropzone = document.getElementById(data.TEAM).querySelector('.dropzone');
            teamDropzone.appendChild(buzzerDiv);
        } else {
            container.appendChild(buzzerDiv);
        }
    }
}
