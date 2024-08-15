import { createBuzzerDiv } from './buzzer.js';
import { createTeamDiv } from './team.js';
import { initializeDropzones } from './dragAndDrop.js';

// Connectez-vous au serveur WebSocket
  //const loc = window.location;
  const loc = "buzzcontrol.local";
  //const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
  const wsProtocol = "ws:"
  //const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
  const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

  // Initialiser l'objet WebSocket
  export const ws = new WebSocket(wsUrl);


let teams = {};

// Positionne le bouton "Ajouter une équipe" en bas du conteneur des équipes
function positionButtonAtBottom() {
    const button = document.getElementById('addDivButton');
    const container = document.querySelector('.team-container');
    
    if (container && button) {
        try {
            container.appendChild(button); 
        } catch (error) {
            console.error('Erreur lors du positionnement du bouton:', error);
        }
    } else {
        console.warn('Le bouton ou le conteneur n\'est pas disponible.');
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
                [teamName]: { "color": [255, 255, 255]}
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

function trySendTeamData(teamName) {
    if (ws.readyState === WebSocket.OPEN) {
        sendTeamDataToServer(teamName);
    } else {
        console.log('WebSocket not ready. Retrying...');
        setTimeout(() => trySendTeamData(teamName), 1000); // Réessaie après 1 seconde
    }
}


// Gère l'ajout d'une nouvelle équipe après validation du nom
function handleAddTeam() {
    const teamName = promptForTeamName();
    if (teamName) {
        teams[teamName] = {};
        trySendTeamData(teamName);
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
    try {
        const addButton = document.getElementById('addDivButton');
        if (!addButton) throw new Error('Le bouton d\'ajout est introuvable.');
        
        // Configuration de WebSocket
        ws.onopen = () => {
            console.log('WebSocket connection opened.');
            // Envoyez un message initial lorsque le WebSocket est ouvert
            ws.send('{ "ACTION": "HELLO", "MSG": "Salut, serveur WebSocket !"}');
        };

        ws.onerror = (error) => console.error('WebSocket error:', error);
        ws.onclose = () => console.log('WebSocket connection closed.');

        initializeDropzones(ws);
        positionButtonAtBottom();

        addButton.addEventListener('click', handleAddTeam);

        ws.onmessage = handleIncomingMessage;

    } catch (error) {
        console.error('Erreur lors de l\'initialisation:', error);
    }
});
