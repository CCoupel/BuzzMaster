import { createBuzzerDiv } from './buzzer.js';
import { createTeamDiv } from './team.js';
import { ws } from './webSocket.js';
import { initializeDropzones } from './dragAndDrop.js';

document.addEventListener('DOMContentLoaded', () => {
    const button = document.getElementById('addDivButton');

    initializeDropzones(ws);

    button.addEventListener('click', () => {
        const titleText = prompt('Nom de la team :', 'Team ');

        if (titleText !== null && titleText.trim() !== '') {
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

            createTeamDiv(teamData.teams);
        } else {
            alert('Le titre ne peut pas être vide.');
        }
    });
});

ws.onmessage = function(event) {
    try {
        console.log(event.data);
        const data = JSON.parse(event.data);

        if (data.teams) {
            createTeamDiv(data.teams);
        }
        if (data.bumpers) {
            createBuzzerDiv(data);
        }
    } catch (error) {
        console.error('Erreur lors du traitement des données JSON:', error);
    }
};
