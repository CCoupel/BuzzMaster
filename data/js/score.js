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
    const sortedTeams = Object.entries(teams).sort((a, b) => b[1].SCORE - a[1].SCORE);

    sortedTeams.forEach((team, index) => {
        const [teamName, teamData] = team;
        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = teamName;
        row.insertCell(2).textContent = teamData.SCORE || 0;
    });
}

function renderPlayerScores() {
    const tbody = document.querySelector('#player-scores tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    const bumpers = getBumpers();
    const sortedPlayers = Object.entries(bumpers)
        .map(([id, data]) => ({id, ...data}))
        .sort((a, b) => b.SCORE - a.SCORE);

    sortedPlayers.forEach((player, index) => {
        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = player.NAME || `Joueur ${player.id}`;
        row.insertCell(2).textContent = player.TEAM || 'Sans équipe';
        row.insertCell(3).textContent = player.SCORE || 0;
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