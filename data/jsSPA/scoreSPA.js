import { sendWebSocketMessage } from './websocket.js';
import { updateBumpers, updateTeams, getTeams, getBumpers, setBumperPoint, } from './configSPA.js';

function updateScores(data) {
    if (data.teams) updateTeams(data.teams);
    if (data.bumpers) updateBumpers(data.bumpers);
    renderTeamScores();
    renderPlayerScores();
}

let previousTeamPositions = {};
let previousPlayerPositions = {};
let editingPlayerId = null;

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

        const teamNameCell = row.insertCell(1);
        const teamNameDiv = document.createElement('div');
        teamNameDiv.className = 'color-cell';
        teamNameCell.appendChild(teamNameDiv);

        if (teamData.COLOR) {
            const teamColor = document.createElement('div');
            teamColor.className = 'team-color';
            teamColor.style.backgroundColor = `rgb(${teamData.COLOR.join(',')})`;
            teamNameDiv.appendChild(teamColor);
        }

        const teamNameP = document.createElement('p');
        teamNameP.textContent = teamName;
        teamNameDiv.appendChild(teamNameP);

        row.insertCell(2).textContent = teamData.SCORE || 0;

        // Ajout de la classe highlight si la position de l'équipe change
        if (previousTeamPositions[teamName] !== undefined && previousTeamPositions[teamName] !== index + 1) {
            row.classList.add('highlight');
        }

        // Mettre à jour la position précédente de l'équipe
        previousTeamPositions[teamName] = index + 1;
    });

    // Nettoyage de l'animation après un certain temps
    setTimeout(() => {
        document.querySelectorAll('.highlight').forEach(row => row.classList.remove('highlight'));
    }, 1000);
}

function renderPlayerScores() {
    const tbody = document.querySelector('#player-scores tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    const bumpers = getBumpers();
    const teams = getTeams();

    const sortedPlayers = Object.entries(bumpers)
        .map(([id, data]) => ({
            id,
            ...data,
            SCORE: parseInt(data.SCORE) || 0
        }))
        .sort((a, b) => b.SCORE - a.SCORE);

    sortedPlayers.forEach((player, index) => {
        const previousPosition = previousPlayerPositions[player.id];

        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = player.NAME || `Joueur ${player.id}`;

        const teamNameCell = row.insertCell(2);
        const teamNameDiv = document.createElement('div');
        teamNameDiv.className = 'color-cell';
        teamNameCell.appendChild(teamNameDiv);

        if (teams[player.TEAM]) {
            const teamColor = document.createElement('div');
            teamColor.className = 'team-color';
            teamColor.style.backgroundColor = `rgb(${teams[player.TEAM].COLOR.join(',')})`;
            teamNameDiv.appendChild(teamColor);
        }

        const teamNameP = document.createElement('p');
        teamNameP.textContent = player.TEAM || 'Sans équipe';
        teamNameDiv.appendChild(teamNameP);

        const scoreCell = row.insertCell(3);

        const modalPlayerName = document.getElementById('modal-player-name');
        const scoreInput = document.getElementById('score-input');
        const modal = document.getElementById('score-modal');

        // Score cliquable
        const scoreText = document.createElement('span');
        scoreText.textContent = player.SCORE || 0;
        scoreText.style.cursor = 'pointer';
        scoreText.addEventListener('click', () => {
            editingPlayerId = player.id;
            modalPlayerName.textContent = player.NAME || `Joueur ${player.id}`;
            scoreInput.value = player.SCORE || 0;
            modal.classList.remove('hidden');
        });
        scoreCell.appendChild(scoreText);

        // Animation si changement de position
        if (previousPosition !== undefined && previousPosition !== index + 1) {
            row.classList.add('highlight');
        }

        previousPlayerPositions[player.id] = index + 1;
    });

    setTimeout(() => {
        document.querySelectorAll('.highlight').forEach(row => row.classList.remove('highlight'));
    }, 1000);
}

function handleScoreWebSocketMessage(event) {
    const data = JSON.parse(event.data);
    if (data.ACTION === 'UPDATE' || data.ACTION === 'FULL') {
        updateScores(data.MSG);
    }
}

function handleResetScore() {
    if (confirm('Êtes-vous sûr de vouloir réinitialiser les scores ?')) {
        sendWebSocketMessage('RAZ', {});
    }
}

function modalEvents() {
    const modal = document.getElementById('score-modal');
    const scoreInput = document.getElementById('score-input');
    const closeModalBtn = document.getElementById('close-modal');
    const saveScoreBtn = document.getElementById('save-score');
    closeModalBtn.onclick = () => modal.classList.add('hidden');

    saveScoreBtn.onclick = () => {
        const bumpers = getBumpers();
        if (editingPlayerId && bumpers[editingPlayerId]) {
            const currentScore = bumpers[editingPlayerId].SCORE || 0;
            const newScore = parseInt(scoreInput.value) || 0;
            const diff = newScore - currentScore;

            if (diff !== 0) {
                setBumperPoint(editingPlayerId, diff);
            }

            renderPlayerScores();
            modal.classList.add('hidden');
        }
    };
}

export function scorePage() {
    renderTeamScores();
    renderPlayerScores();
    modalEvents()
}

document.addEventListener('DOMContentLoaded', () => {
    //connectWebSocket(handleScoreWebSocketMessage);
    //renderTeamScores();
    renderPlayerScores();

    const resetButton = document.getElementById('reset-score');
    if (resetButton) resetButton.addEventListener('click', handleResetScore);
});