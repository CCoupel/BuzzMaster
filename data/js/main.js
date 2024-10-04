import { createTeamDiv } from './team.js';

// Connectez-vous au serveur WebSocket
//const loc = window.location;
const loc = "buzzcontrol.local";
//const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
const wsProtocol = "ws:"
//const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

// Initialiser l'objet WebSocket
export let ws; // Déclare et exporte ws comme une variable globale
let reconnectInterval = 5000; // Intervalle en millisecondes pour tenter de se reconnecter

let teams = {};
let bumpers = {};

export function getTeams() {
    return teams;
}

export function getBumpers() {
    return bumpers;
}

export function updateTeams(newTeams) {
    teams = newTeams;
}

export function updateBumpers(newBumpers) {
    bumpers = newBumpers;
}

export function setBumperName(id, name) {
    for (const bumperId in bumpers) {
        if (bumpers[bumperId]["NAME"] === name && bumperId !== id) {
            console.error(`Le nom "${name}" est déjà utilisé par l'ID ${bumperId}`);
            return; // On stoppe la fonction pour éviter de définir un nom dupliqué
        }
    }
    bumpers[id]["NAME"]=name;
    sendTeamsAndBumpers();
}

export function setBumperPoint(id, inc=0) {
    const bumper=bumpers[id];
    if (!bumper["SCORE"]) { bumper["SCORE"]=0};
    console.log("MAIN: set bumper point id=%s score=%i",id,  bumper["SCORE"])

    bumper["SCORE"]=bumper["SCORE"]+inc;
    sendTeamsAndBumpers();
}


export function setTeamColor(id, color) {
    console.log("MAIN: set team color id=%s",id)
    teams[id]["COLOR"]=color;
    sendTeamsAndBumpers();
}

export function addNewTeam(id) {
  teams[id]={"COLOR": [64, 64, 64]};
  sendTeamsAndBumpers();
};

export function deleteTeam(id) {
    delete teams[id];
    sendTeamsAndBumpers();
}

export function addBumperToTeam(bId, tId) {
    bumpers[bId]["TEAM"]=tId;
    sendTeamsAndBumpers()
}

export function removeBumperFromTeam(bId) {
    bumpers[bId]["TEAM"]='';
    sendTeamsAndBumpers()
}


export function sendTeamsAndBumpers() {
    if (ws.readyState === WebSocket.OPEN) {
        sendWebSocketMessage("FULL", {"bumpers": bumpers, "teams": teams});
    } else {
        console.log('WebSocket not ready. Retrying...');
        setTimeout(() => sendTeamsAndBumpers(), 1000); // Réessaie après 1 seconde
    }
}
// Fonction pour initialiser la connexion WebSocket
export function connectWebSocket(onMessageCallback) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        return ws;
    }

    ws = new WebSocket(wsUrl);

    ws.onopen = function(event) {

        console.log('WebSocket connection opened.');
        
        // Nettoyage du tableau lors de la reconnexion
        cleanBoard();

        sendWebSocketMessage("HELLO", "Salut, serveur WebSocket !" );
    };

    ws.onerror = function(event) {
        console.error('WebSocket error:', error);
    }

    ws.onclose = function(event) {

        console.log('WebSocket connection closed. Attempting to reconnect...');
        setTimeout(connectWebSocket, reconnectInterval); // Tente de se reconnecter après un délai
    };

    ws.onmessage = onMessageCallback || function(event) {
        console.log('Message reçu:', event.data);
    };
    return ws;

}

/*
// Fonction pour afficher une invite pour le nom de l'équipe et vérifier sa validité
function promptForTeamName() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '' && !teams[teamName]) {
        return teamName;
    }
    alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
    return null;
}
*/

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
export function trySendTeamData(teamName) {
    if (ws.readyState === WebSocket.OPEN) {
        sendTeamDataToServer(teamName);
    } else {
        console.log('WebSocket not ready. Retrying...');
        setTimeout(() => trySendTeamData(teamName), 1000); // Réessaie après 1 seconde
    }
}

/*
// Gère l'ajout d'une nouvelle équipe après validation du nom
function handleAddTeam() {
    const teamName = promptForTeamName();
    if (teamName) {
        teams[teamName] = {};
        trySendTeamData(teamName);
        createTeamDiv(teams);
    }
}
*/
/*
// Gère la réinitialisation des équipes
function handleReset() {
    sendWebSocketMessage("RESET", "RESET")
}
*/

export function sendWebSocketMessage (action, MSG= "")  {
    const message = {
        "ACTION": action,
        "MSG": MSG
    };
    ws.send(JSON.stringify(message));
};


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

/*
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
*/