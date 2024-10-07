import { connectWebSocket, getTeams, getBumpers, updateTeams, updateBumpers, setBumperPoint, sendWebSocketMessage} from './main.js';

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
        const teamNameCell = row.insertCell(1)
        teamNameCell.className = 'color-cell';
        
        if (teamData.COLOR) {
            const teamColor = document.createElement('div');
            teamColor.className = 'team-color';
            teamColor.style.backgroundColor = `rgb(${teamData.COLOR.join(',')})`;
            teamNameCell.appendChild(teamColor);
        }

        const teamNameP = document.createElement('p');
        teamNameP.textContent = teamName;
        teamNameCell.appendChild(teamNameP);

        console.log(teamData.COLOR)
        row.insertCell(2).textContent = teamData.SCORE || 0;
        console.log('team :', team)
        console.log('teamdata:', teamData)
    });
}

let previousPositions = {};

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
        const previousPosition = previousPositions[player.id];
        
        const row = tbody.insertRow();
        row.insertCell(0).textContent = index + 1;
        row.insertCell(1).textContent = player.NAME || `Joueur ${player.id}`;
        row.insertCell(2).textContent = player.TEAM || 'Sans équipe';
        const scoreButtonCell = row.insertCell(3);

        // Créer le bouton -
        const buttonMinus = document.createElement('button');
        buttonMinus.className = "button-score";
        buttonMinus.textContent = '-';
        buttonMinus.style = "background-color: #2196F3; margin-right: 5px;"; // Ajouter un espacement
        buttonMinus.onclick = () => {
            console.log(`Bouton - cliqué pour le joueur : ${player.NAME || `Joueur ${player.id}`}`);
            setBumperPoint(player.id, -1);
        };
    
        // Ajouter le bouton - à la cellule
        scoreButtonCell.appendChild(buttonMinus);
    
        // Ajouter le score à la même cellule
        const scoreText = document.createTextNode(player.SCORE || 0);
        scoreButtonCell.appendChild(scoreText);
    
        // Créer le bouton +
        const buttonPlus = document.createElement('button');
        buttonPlus.className = "button-score";
        buttonPlus.textContent = '+';
        buttonPlus.style = "margin-left: 5px;";
        buttonPlus.onclick = () => {
            console.log(`Bouton + cliqué pour le joueur : ${player.NAME || `Joueur ${player.id}`}`);
            setBumperPoint(player.id, 1);
        };
    
        // Ajouter le bouton + à la cellule
        scoreButtonCell.appendChild(buttonPlus);
    
        // Vérification si le joueur a changé de position
        if (previousPosition !== undefined && previousPosition !== index + 1) {
            // Ajouter une classe d'animation si le joueur a changé de position
            row.classList.add('highlight');
        }

        // Mettre à jour la position précédente du joueur
        previousPositions[player.id] = index + 1;
    });

    // Nettoyage de l'animation après un certain temps
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


document.addEventListener('DOMContentLoaded', () => {
    connectWebSocket(handleScoreWebSocketMessage);
    renderTeamScores();
    renderPlayerScores();

    const resetButton = document.getElementById('reset-score');
    if (resetButton) resetButton.addEventListener('click', handleResetScore);
});