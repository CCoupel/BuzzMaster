import { connectWebSocket, getTeams, getBumpers, updateTeams, updateBumpers } from './main.js';

function updateScores(data) {
    if (data.teams) updateTeams(data.teams);
    if (data.bumpers) updateBumpers(data.bumpers);
    renderTeamScores();
    renderPlayerScores();
}

function renderTeamScores() {
    const tbody = document.querySelector('#team-scores tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    const teams = getTeams();
    console.log(teams)
    const sortedTeams = Object.entries(teams).sort((a, b) => b[1].SCORE - a[1].SCORE);

    sortedTeams.forEach((team, index) => {
        const [teamName, teamData] = team;
        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = teamName;
        row.insertCell(2).textContent = teamData.SCORE || 0;
        console.log('team :', team)
        console.log('teamdate:', teamData)
    });
}

function renderPlayerScores() {
    const tbody = document.querySelector('#player-scores tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    const bumpers = getBumpers();
    const sortedPlayers = Object.entries(bumpers)
        .map(([id, data]) => ({
            id,
            ...data,
            SCORE: parseInt(data.SCORE) || 0
        }))
        .sort((a, b) => b.SCORE - a.SCORE);

    sortedPlayers.forEach((player, index) => {
        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = player.NAME || `Joueur ${player.id}`;
        row.insertCell(2).textContent = player.TEAM || 'Sans équipe';
        const scoreButtonCell = row.insertCell(3);

        // Créer le bouton -
        const buttonMinus = document.createElement('button');
        buttonMinus.textContent = '-';
        buttonMinus.style = "background-color: #2196F3; margin-right: 5px;"; // Ajouter un espacement
        buttonMinus.onclick = () => {
            console.log(`Bouton - cliqué pour le joueur : ${player.NAME || `Joueur ${player.id}`}`);
        };
    
        // Ajouter le bouton - à la cellule
        scoreButtonCell.appendChild(buttonMinus);
    
        // Ajouter le score à la même cellule
        const scoreText = document.createTextNode(player.SCORE || 0);
        scoreButtonCell.appendChild(scoreText);
    
        // Créer le bouton +
        const buttonPlus = document.createElement('button');
        buttonPlus.textContent = '+';
        buttonPlus.style = "margin-left: 5px;"; // Ajouter un espacement
        buttonPlus.onclick = () => {
            console.log(`Bouton + cliqué pour le joueur : ${player.NAME || `Joueur ${player.id}`}`);
        };
    
        // Ajouter le bouton + à la cellule
        scoreButtonCell.appendChild(buttonPlus);
    });
}

function handleScoreWebSocketMessage(event) {
    const data = JSON.parse(event.data);
    if (data.ACTION === 'UPDATE' || data.ACTION === 'FULL') {
        updateScores(data.MSG);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket(handleScoreWebSocketMessage);
    renderTeamScores();
    renderPlayerScores();
});