import { handleConfigSocketMessage } from './interface.js';

// Connectez-vous au serveur WebSocket
//const loc = window.location;
const loc = "buzzcontrol.local";
//const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
const wsProtocol = "ws:"
//const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

export let ws; // Déclare et exporte ws comme une variable globale
let reconnectInterval = 5000; // Intervalle en millisecondes pour tenter de se reconnecter
let pingInterval = 100000000;
let pingTimeout;

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

export function connectWebSocket(onMessageCallback) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        return ws;
    }

    ws = new WebSocket(wsUrl);

    webSocketColor();

    ws.onopen = function(event) {

        console.log('WebSocket connection opened.');
        
        // Nettoyage du tableau lors de la reconnexion
        //cleanBoard();

        sendWebSocketMessage("HELLO", {} );

        webSocketColor();

        startPing();
    };

    ws.onerror = function(event) {
        console.error('WebSocket error:', event);

        webSocketColor();
    }

    ws.onclose = function(event) {

        console.log('WebSocket connection closed. Attempting to reconnect...');
        setTimeout(connectWebSocket, reconnectInterval); // Tente de se reconnecter après un délai

        webSocketColor();

        stopPing();
    };

    ws.onmessage = onMessageCallback || function(event) {
        console.log('Message reçu:', event.data);
        resetPing();
    };
    return ws;

}

function startPing() {
    stopPing();
    pingTimeout = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
            sendWebSocketMessage("PING", {}); 
            console.log('PING...')
        }
    }, pingInterval);
}


function resetPing() {
    stopPing();
    startPing(); 
}


function stopPing() {
    if (pingTimeout) clearInterval(pingTimeout);
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

connectWebSocket(handleConfigSocketMessage);