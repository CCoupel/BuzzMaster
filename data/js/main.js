import { createBuzzerDiv } from './buzzer.js';
import { createTeamDiv } from './team.js';
import { ws } from './webSocket.js';
import { initializeDropzones } from './dragAndDrop.js';

let teams = {};

// Positionne le bouton "Ajouter une équipe" en bas du conteneur des équipes
function positionButtonAtBottom() {
    const button = document.getElementById('addDivButton');
    const container = document.querySelector('.team-container');
    
    if (container && button) {
        container.appendChild(button); 
    }
}

// Affiche une invite pour saisir le nom de la nouvelle équipe et vérifie sa validité
function promptForTeamName() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '' && !teams[teamName]) {
        return teamName;
    }
    alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
    return null;
}

// Envoie les données de la nouvelle équipe au serveur via WebSocket
function sendTeamDataToServer(teamName) {
    const teamData = { 
        "ACTION": "UPDATE", 
        "MSG": { 
            "teams": { 
                [teamName]: { "COLOR": [255, 255, 255]}
            }
        } 
    };

    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(teamData));
        console.log('Team data sent:', teamData);
    } else {
        console.log('WebSocket is not open. Cannot send team data.');
    }
}


// Gère l'ajout d'une nouvelle équipe après validation du nom
function handleAddTeam() {
    const teamName = promptForTeamName();
    if (teamName) {
        teams[teamName] = {};
        sendTeamDataToServer(teamName);
        createTeamDiv(teams); 
        positionButtonAtBottom(); 
    }
}

// Gère les messages entrants du serveur via WebSocket, mettant à jour les équipes et les buzzers
function handleIncomingMessage(event) {
    try {
        console.log(event.data);
        const data = JSON.parse(event.data);

        if (data.teams) {
            Object.assign(teams, data.teams); 
            createTeamDiv(teams);
            positionButtonAtBottom();
        }
        if (data.bumpers) {
            createBuzzerDiv(data);
        }
    } catch (error) {
        console.error('Erreur lors du traitement des données JSON:', error);
    }
}

// Initialisation des événements et du WebSocket à la fin du chargement de la page
document.addEventListener('DOMContentLoaded', () => {
    const addButton = document.getElementById('addDivButton');
    
    initializeDropzones(ws);
    positionButtonAtBottom();

    addButton.addEventListener('click', handleAddTeam);

    ws.onmessage = handleIncomingMessage;
    ws.send('{ "ACTION": "HELLO", "MSG": "Salut, serveur WebSocket !"}');
});
