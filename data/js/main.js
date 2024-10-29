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

export function createElement(tag, className, attributes = {}) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, value));
    return element;
}

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
    console.log(id)
    const team=teams[bumper["TEAM"]];
    if (!bumper["SCORE"]) { bumper["SCORE"]=0};
    console.log("MAIN: set bumper point id=%s score=%i",id,  bumper["SCORE"])

    bumper["SCORE"]=bumper["SCORE"]+inc;
    if (team) {
      if (!team["SCORE"]) { team["SCORE"]=0};
        team["SCORE"]=team["SCORE"]+inc;
    }
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

function webSocketColor() {
    let webSocketColorDiv = document.getElementById('websocket-tracker');

    // Vérifie si la div existe déjà
    if (!webSocketColorDiv) {
        webSocketColorDiv = document.createElement('div');
        webSocketColorDiv.id = 'websocket-tracker';
        document.body.appendChild(webSocketColorDiv);
    }

    function updateColor() {

        const wstracker = document.getElementById('websocket-tracker')

        if (ws.readyState === WebSocket.CONNECTING) {
            wstracker.style.backgroundColor = "orange"; // Connecting
        } else if (ws.readyState === WebSocket.OPEN) {
            wstracker.style.backgroundColor = "green"; // Open
        } else {
            wstracker.style.backgroundColor = "red"; // Closed or other
        }
    }

    updateColor();

};

// Fonction pour initialiser la connexion WebSocket
export function connectWebSocket(onMessageCallback) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        return ws;
    }

    ws = new WebSocket(wsUrl);

    webSocketColor();

    ws.onopen = function(event) {

        console.log('WebSocket connection opened.');
        
        // Nettoyage du tableau lors de la reconnexion
        cleanBoard();

        sendWebSocketMessage("HELLO", {} );

        webSocketColor();
    };

    ws.onerror = function(event) {
        console.error('WebSocket error:', event);

        webSocketColor();
    }

    ws.onclose = function(event) {

        console.log('WebSocket connection closed. Attempting to reconnect...');
        setTimeout(connectWebSocket, reconnectInterval); // Tente de se reconnecter après un délai

        webSocketColor();
    };

    ws.onmessage = onMessageCallback || function(event) {
        console.log('Message reçu:', event.data);
    };
    return ws;

}


export function sendWebSocketMessage (action, MSG= "{}")  {
    
    webSocketColor();

    if (ws.readyState === WebSocket.OPEN) {
        const message = {
            "ACTION": action,
            "MSG": MSG
        };

        ws.send(JSON.stringify(message));

        } else {
            console.error("WebSocket is not open. Current state: ", ws.readyState);
        }
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
