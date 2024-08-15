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
export let ws; // Déclare et exporte ws comme une variable globale
let teams = {};
let reconnectInterval = 5000; // Intervalle en millisecondes pour tenter de se reconnecter

// Fonction pour initialiser la connexion WebSocket
// Fonction pour initialiser la connexion WebSocket
function connectWebSocket() {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log('WebSocket connection opened.');
        
        // Nettoyage du tableau lors de la reconnexion
        cleanBoard();

        ws.send(JSON.stringify({ "ACTION": "HELLO", "MSG": "Salut, serveur WebSocket !" }));
        initializeDropzones(ws);
    };

    ws.onerror = (error) => console.error('WebSocket error:', error);

    ws.onclose = () => {
        console.log('WebSocket connection closed. Attempting to reconnect...');
        setTimeout(connectWebSocket, reconnectInterval); // Tente de se reconnecter après un délai
    };

    ws.onmessage = handleIncomingMessage;
}



// Fonction pour afficher une invite pour le nom de l'équipe et vérifier sa validité
function promptForTeamName() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '' && !teams[teamName]) {
        return teamName;
    }
    alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
    return null;
}

// Fonction pour envoyer les données de la nouvelle équipe au serveur via WebSocket
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

// Fonction pour réessayer d'envoyer les données de l'équipe si WebSocket n'est pas prêt
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
    }
}

// Gère la réinitialisation des équipes
function handleReset() {
    ws.send('{ "ACTION": "RESET", "MSG": "RESET"}')
}

// Nettoie les conteneurs d'équipes et de buzzers
function cleanBoard() {
    const teamContainer = document.querySelector('.team-container');
    const buzzerContainer = document.querySelector('.buzzer-container');
    
    if (teamContainer) {
        teamContainer.innerHTML = '';
    }

    if (buzzerContainer) {
        buzzerContainer.innerHTML = '';
    }
}

// Gère les messages entrants du serveur via WebSocket, mettant à jour les équipes et les buzzers
function handleIncomingMessage(event) {
    try {
        console.log(event.data);
        const data = JSON.parse(event.data);
        cleanBoard();

        if (data.teams) {
            createTeamDiv(data.teams);
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
        
        const resetButton = document.getElementById('resetButton');
        if (!resetButton) throw new Error('Le bouton de reset est introuvable.');
        
        // Démarre la connexion WebSocket
        connectWebSocket();

        addButton.addEventListener('click', handleAddTeam);
        resetButton.addEventListener('click', handleReset);
        
    } catch (error) {
        console.error('Erreur lors de l\'initialisation:', error);
    }
});