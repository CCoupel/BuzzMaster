import { createTeamDiv } from './teamSPA.js';
import { createBuzzerDiv } from './buzzerSPA.js';
import { initializeDropzones } from './dragAndDropSPA.js'
import { sendWebSocketMessage, ws} from './websocket.js';
 
let teams = {};
let bumpers = {};

export function getTeams() {
    return teams;
};

export function getBumpers() {
    return bumpers;
};

export function updateTeams(newTeams) {
    teams = newTeams;
};

export function updateBumpers(newBumpers) {
    bumpers = newBumpers;
};

export function addNewTeam(id) {
  teams[id]={"COLOR": [64, 64, 64]};
  sendTeamsAndBumpers();
};

export function setBumperName(id, name) {
    for (const bumperId in bumpers) {
        if (bumpers[bumperId]["NAME"] === name && bumperId !== id) {
            console.error(`Le nom "${name}" est déjà utilisé par l'ID ${bumperId}`);
            return; // On stoppe la fonction pour éviter de définir un nom dupliqué
        }
    }
    bumpers[id]["NAME"]=name;
    sendTeamsAndBumpers();
};

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
};

export function deleteTeam(id) {
    delete teams[id];
    sendTeamsAndBumpers();
};

export function addBumperToTeam(bId, tId) {
    bumpers[bId]["TEAM"]=tId;
    sendTeamsAndBumpers()
};

export function removeBumperFromTeam(bId) {
    bumpers[bId]["TEAM"]='';
    sendTeamsAndBumpers()
};

export function setTeamColor(id, color) {
    console.log("MAIN: set team color id=%s",id)
    teams[id]["COLOR"]=color;
    sendTeamsAndBumpers();
};

export function sendTeamsAndBumpers() {
    if (ws.readyState === WebSocket.OPEN) {
        sendWebSocketMessage("FULL", {"bumpers": bumpers, "teams": teams});
    } else {
        console.log('WebSocket not ready. Retrying...');
        setTimeout(() => sendTeamsAndBumpers(), 1000); // Réessaie après 1 seconde
    }
};


export function updateDisplayConfig() {
    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());
};

function promptForTeamName() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '' && !getTeams()[teamName]) {
        return teamName;
    }
    alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
    return null;
};

function handleAddTeam() {
    const teamName = promptForTeamName();
    if (teamName) {
        addNewTeam(teamName);
        //trySendTeamData(teamName);
        //createTeamDiv(teams);
    }
};

function handleReset() {
    if (confirm('Êtes-vous sûr de vouloir réinitialiser la configuration ?')) {
        sendWebSocketMessage('RESET', '');
    }
};

document.addEventListener('DOMContentLoaded', () => {
    //connectWebSocket(handleConfigSocketMessage);
    initializeDropzones();
    //getWebVersion();

    const addButton = document.getElementById('addDivButton');
    const resetButton = document.getElementById('resetButton');
    
    if (addButton) addButton.addEventListener('click', handleAddTeam);
    if (resetButton) resetButton.addEventListener('click', handleReset);

    updateDisplayConfig();
});

export function configPage() {
    initializeDropzones();
    updateDisplayConfig();
};

// Nettoyage lors du déchargement de la page
/*
window.addEventListener('unload', () => {
    removeWebSocketCallback(handleConfigSocketMessage);
});
*/
