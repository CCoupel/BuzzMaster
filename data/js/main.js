import { createBuzzerDiv } from './buzzer.js';
import { createTeamDiv } from './team.js';
import { ws } from './webSocket.js';
import { initializeDropzones } from './dragAndDrop.js';

let teams = {}; // Object to store team data

function postionButtonAtBottom() {
    const button = document.getElementById('addDivButton');
    const container = document.querySelector('.team-container');
    
    if (container && button) {
        container.appendChild(button); // Place le bouton à la fin du conteneur des équipes
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const button = document.getElementById('addDivButton');

    initializeDropzones(ws);

    postionButtonAtBottom();

    button.addEventListener('click', () => {
        const titleText = prompt('Nom de la team :', 'Team ');

        if (titleText !== null && titleText.trim() !== '' && !teams[titleText]) {
            const teamData = {
                teams: {}
            };
            teamData.teams[titleText] = {};

            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify(teamData));
                console.log('Team data sent:', teamData);
            } else {
                console.log('WebSocket is not open. Cannot send team data.');
            }

            teams[titleText] = {}; // Add the new team to the local teams object
            createTeamDiv(teams); // Recreate team divs with updated teams data

            // Positionner le bouton en bas de la dernière équipe
            postionButtonAtBottom();
        } else {
            alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
        }
    });
});

ws.onmessage = function(event) {
    try {
        console.log(event.data);
        const data = JSON.parse(event.data);

        if (data.teams) {
            Object.assign(teams, data.teams); // Merge new team data into the existing teams object
            createTeamDiv(teams); // Recreate team divs with updated teams data

            // Positionner le bouton en bas de la dernière équipe
            postionButtonAtBottom();
        }
        if (data.bumpers) {
            createBuzzerDiv(data);
        }
    } catch (error) {
        console.error('Erreur lors du traitement des données JSON:', error);
    }
};
