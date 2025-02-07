import { createTeamDiv } from './teamSPA.js';
import { createBuzzerDiv } from './buzzerSPA.js';
import { initializeDropzones } from './dragAndDropSPA.js'
//import { getWebVersion, getCoreVersion } from './version.js';
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

/*function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
    const data = JSON.parse(event.data);
    if (data.ACTION === 'UPDATE' || data.ACTION === 'FULL') {
        if (data.MSG.bumpers) updateBumpers(data.MSG.bumpers);
        if (data.MSG.teams) updateTeams(data.MSG.teams);
        //if (data.MSG.VERSION) getCoreVersion(data.MSG.VERSION);
        updateDisplayConfig();
    }
        //if (data.VERSION) getCoreVersion(data.VERSION);
};*/

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

async function sendFileForm(formId, actionUrl) {
    const form = document.getElementById(formId);

    form.addEventListener('submit', async function(event) {
        event.preventDefault();

        const formData = new FormData(form);

        try {
            const response = await fetch(actionUrl, {
                method: 'POST',
                body: formData 
            });

            if (!response.ok) {
                throw new Error("Erreur du serveur: " + response.statusText);
            }

            const responseData = await response.text();

            alert("Image envoyée avec succès !");
            console.log("Réponse du serveur :", responseData);

        } catch (error) {
            console.error("Erreur lors de l'envoi:", error);
            alert("Une erreur est survenue lors de l'envoi de l'image.");
        }
    });
};

document.addEventListener('DOMContentLoaded', () => {
    //sendFileForm('background-form', 'http://buzzcontrol.local/background');
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
    //sendFileForm('background-form', 'http://buzzcontrol.local/background');
    initializeDropzones();
    //getWebVersion();
    updateDisplayConfig();
};

// Nettoyage lors du déchargement de la page
/*
window.addEventListener('unload', () => {
    removeWebSocketCallback(handleConfigSocketMessage);
});
*/
