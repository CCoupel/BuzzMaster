import { createTeamDiv } from './team.js';
import { createBuzzerDiv } from './buzzer.js';
import { initializeDropzones } from './dragAndDrop.js'
import { getWebVersion, getCoreVersion } from './version.js';
import { sendWebSocketMessage, connectWebSocket, getTeams, getBumpers, updateTeams, updateBumpers, addNewTeam } from './main.js';


function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
    const data = JSON.parse(event.data);
    if (data.ACTION === 'UPDATE' || data.ACTION === 'FULL') {
        if (data.MSG.bumpers) updateBumpers(data.MSG.bumpers);
        if (data.MSG.teams) updateTeams(data.MSG.teams);
        //if (data.MSG.VERSION) getCoreVersion(data.MSG.VERSION);
        updateDisplay();
    }
        if (data.VERSION) getCoreVersion(data.VERSION);
    }

function updateDisplay() {
    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());
}

function promptForTeamName() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '' && !getTeams()[teamName]) {
        return teamName;
    }
    alert('Le titre ne peut pas être vide ou l\'équipe existe déjà.');
    return null;
}
function handleAddTeam() {
    const teamName = promptForTeamName();
    if (teamName) {
        addNewTeam(teamName);
        //trySendTeamData(teamName);
        //createTeamDiv(teams);
    }
}

function handleReset() {
    if (confirm('Êtes-vous sûr de vouloir réinitialiser la configuration ?')) {
        sendWebSocketMessage('RESET', '');
    }
}

async function sendFileForm(formId, actionUrl) {
    
    const form = document.getElementById(formId);
    form.addEventListener('submit', async function(event) {
        event.preventDefault();

        const formData = new FormData(form);

        try {
            console.log("upload en cours")
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
}

document.addEventListener('DOMContentLoaded', () => {
    sendFileForm('background-form', 'http://buzzcontrol.local/background');
    connectWebSocket(handleConfigSocketMessage);
    initializeDropzones();
    getWebVersion();

    const addButton = document.getElementById('addDivButton');
    const resetButton = document.getElementById('resetButton');
    
    if (addButton) addButton.addEventListener('click', handleAddTeam);
    if (resetButton) resetButton.addEventListener('click', handleReset);

    updateDisplay();
});

// Nettoyage lors du déchargement de la page
window.addEventListener('unload', () => {
    removeWebSocketCallback(handleConfigSocketMessage);
});
