import { createTeamDiv } from './team.js';
import { createBuzzerDiv } from './buzzer.js';
import { initializeDropzones } from './dragAndDrop.js'
import { trySendTeamData, sendWebSocketMessage, connectWebSocket, getTeams, getBumpers, updateTeams, updateBumpers, addNewTeam } from './main.js';


function handleConfigSocketMessage(event) {
    const data = JSON.parse(event.data);
    if (data.ACTION === 'UPDATE' || data.ACTION === 'FULL') {
        if (data.MSG.teams) updateTeams(data.MSG.teams);
        if (data.MSG.bumpers) updateBumpers(data.MSG.bumpers);
        updateDisplay();
    }
}

function updateDisplay() {
    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());
}

/*
function handleAddTeam() {
    const teamName = prompt('Nom de la team :', 'Team ');
    if (teamName && teamName.trim() !== '') {
        const newTeam = { [teamName]: { COLOR: [255, 255, 255] } };
        sendWebSocketMessage('UPDATE', JSON.stringify({ teams: newTeam }));
    } else {
        alert('Le nom de l\'équipe ne peut pas être vide.');
    }
}
*/
// Fonction pour afficher une invite pour le nom de l'équipe et vérifier sa validité
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
        trySendTeamData(teamName);
        //createTeamDiv(teams);
    }
}

function handleReset() {
    if (confirm('Êtes-vous sûr de vouloir réinitialiser la configuration ?')) {
        sendWebSocketMessage('RESET', '');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket(handleConfigSocketMessage);
    initializeDropzones();

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
