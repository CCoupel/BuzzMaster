import { connectWebSocket, setBumperPoint, sendWebSocketMessage, updateTeams, updateBumpers, getBumpers, getTeams } from './main.js';


let gameState = {
    timer: 30,
    isRunning: false,
    totalTime: 30,
    gamePhase: 'STOP',
    gameTime: 0
};

function updateGameState (msg) {
    gameState.gameTime = msg.TIME;
    gameState.gamePhase = msg.PHASE;
    if (msg.CURRENT_TIME) {
        gameState.timer = msg.CURRENT_TIME;
    } else {
        gameState.timer = msg.DELAY
    }
    gameState.totalTime = msg.DELAY;
};

let localTimer;

function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) {
            handleServerAction(data.ACTION, data.MSG);
        } else {
            console.log("Pas d'action :" , console.log(event.data))
        }
}

function toggleDisplay(target) {
    // Cache l'un avant de montrer l'autre
    const scoreDiv = document.getElementById("score-container")
    const gameDiv = document.getElementById("game-container")

    scoreDiv.classList.add("hidden");
    gameDiv.classList.add("hidden");

    setTimeout(() => {
        if (target === 'SCORE') {
            scoreDiv.classList.remove("hidden");
        } else if (target === 'GAME') {
            gameDiv.classList.remove("hidden");
        }
    }, 500); // Délai pour permettre au CSS de cacher les éléments
}

function handleServerAction(action, msg) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':          
            updateGameState(msg.GAME);
            updateTimeBar(true);
            startTimer();
            updateDisplay();
            console.log(msg);
            break;
        case 'STOP':
            gameState.gamePhase = 'STOP';
            updateTimeBar(true);
            stopTimer();
            updateDisplay();
            break;
        case 'PAUSE':
            gameState.gamePhase = 'PAUSE';
            pauseTimer();
            updateTimeBar(true);
            break;
        case 'CONTINUE':
            gameState.gamePhase = 'START';
            continueTimer();
            updateTimeBar(true);
            break;
        case 'UPDATE':
            if (msg.teams && msg.bumpers) {
                updateTeams(msg.teams);
                updateBumpers(msg.bumpers);
                updateGameState(msg.GAME);
                updateDisplay();
                updateTimeBar(true);
                updateTimer();
                updateScores(msg);
            }
            break;
        case 'REMOTE':
            if (msg.GAME.REMOTE === 'SCORE') {
                toggleDisplay('SCORE');
            } else if (msg.GAME.REMOTE === 'GAME') {
                toggleDisplay('GAME');
            };
            break;
        default:
            console.log('Action non reconnue:', action);
    }
}


function startTimer() {
    if (localTimer) clearInterval(localTimer);
    gameState.isRunning = true;
    localTimer = setInterval(() => {
        if (gameState.timer > 0) {
            gameState.timer--;
            updateTimer();
            updateTimeBar();
        } else {
            stopTimer();
        }
    }, 1000);
}

function stopTimer() {
    clearInterval(localTimer);
    gameState.isRunning = false;
    gameState.timer = 0;
    updateTimer();
    updateTimeBar(true);
}

function pauseTimer() {
    clearInterval(localTimer);
    gameState.isRunning = false;
}

function continueTimer() {
    if (!gameState.isRunning) {
        startTimer();
    }
}

function updateDisplay() {
    console.log('Mise à jour de l\'affichage avec l\'état du jeu:', gameState);
    const container = document.getElementById('game-container');
    container.innerHTML = '';

    const sortedTeams = Object.entries(getTeams()).sort((a, b) => {
        const delayA = a[1].TIMESTAMP !== undefined ? a[1].TIMESTAMP : Infinity;
        const delayB = b[1].TIMESTAMP !== undefined ? b[1].TIMESTAMP : Infinity;

        return delayA - delayB;
    });

    sortedTeams.forEach(([teamName, teamData]) => {
        const teamElement = document.createElement('div');
        const isStartPhase = gameState.gamePhase === 'START';

        const isTeamActive = teamData.TIMESTAMP !== undefined;

        teamElement.className = `team ${isTeamActive ? 'active' : ''} ${isStartPhase && !isTeamActive ? 'start-phase' : ''}`;
        
        const teamHeader = document.createElement('div');
        teamHeader.className = 'team-header';
        const teamColor = document.createElement('div');
        teamColor.className = 'team-color';
        if (teamData.COLOR) {
            teamColor.style.backgroundColor = `rgb(${teamData.COLOR.join(',')})`;
        }       
        const teamTitle = document.createElement('h2');
        teamTitle.textContent = teamName;
        const teamScore = document.createElement('p');
        teamScore.className = 'team-score';
        teamScore.textContent = `Score: ${teamData.SCORE ?? 0}`;
        
        teamHeader.appendChild(teamColor);
        teamHeader.appendChild(teamTitle);
        teamHeader.appendChild(teamScore);
        teamElement.appendChild(teamHeader);

        const teamBumpers = Object.entries(getBumpers())
            .filter(([_, bumperData]) => bumperData.TEAM === teamName)
            .sort((a, b) => {
                const delayA = a[1].TIMESTAMP !== undefined ? a[1].TIMESTAMP : Infinity;
                const delayB = b[1].TIMESTAMP !== undefined ? b[1].TIMESTAMP : Infinity;

                return delayA - delayB;
            });

        teamBumpers.forEach(([bumperMac, bumperData]) => {
            const bumperElement = document.createElement('div');
            console.log(gameState.gameTime)
            const bumperText = document.createElement ('p');
            const bumperTime = document.createElement('p');
            const isBumperActive = bumperData.BUTTON !== undefined || bumperData.TIMESTAMP !== undefined;
            bumperElement.className = `bumper ${isBumperActive ? 'active' : ''} ${isStartPhase && !isBumperActive ? 'start-phase' : ''}`;
            bumperText.textContent = `${bumperData.NAME}`;
            bumperTime.textContent = `${bumperData.TIMESTAMP !== undefined ? ' Temps : ' + ((bumperData.TIMESTAMP - gameState.gameTime) / 1000000) + ' s' : ''}`;
            
            
            if (gameState.gamePhase === 'STOP' && isBumperActive) {
                bumperElement.onclick = () => addPointToBumper(bumperMac);
                bumperElement.style.cursor = 'pointer';
            }
            bumperElement.appendChild(bumperText)
            bumperElement.appendChild(bumperTime)
            teamElement.appendChild(bumperElement);
        });

        container.appendChild(teamElement);
    });
}

function updateTimer() {
    const timerElement = document.getElementById('timer-players');
    const minutes = Math.floor(gameState.timer / 60);
    const seconds = gameState.timer % 60;
    timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
}

function addPointToBumper(bumperMac) {
    if (gameState.gamePhase === 'STOP') {
        setBumperPoint(bumperMac, 1);
        updateDisplay();
    }
}

function updateTimeBar(immediate = false) {
    const timeBar = document.getElementById('time-bar');
    const percentageRemaining = (gameState.timer / gameState.totalTime) * 100;
    
    if (immediate) {
        timeBar.style.transition = 'none';
        timeBar.offsetHeight; // Force a reflow
    }
    
    timeBar.style.width = `${percentageRemaining}%`;

    if (percentageRemaining > 50) {
        timeBar.style.backgroundColor = '#4CAF50';
    } else if (percentageRemaining > 25) {
        timeBar.style.backgroundColor = '#FFA500';
    } else {
        timeBar.style.backgroundColor = '#FF0000';
    }

    if (gameState.gamePhase === 'PAUSE') {
        timeBar.classList.add('pause-blink');
    } else {
        timeBar.classList.remove('pause-blink');
    }

    if (gameState.timer <= 5) {
        timeBar.classList.remove('blink');
        timeBar.classList.add('blink-fast');
    } else if (gameState.timer <= 10){
        timeBar.classList.remove('blink-fast');
        timeBar.classList.add('blink');
    } else {
        timeBar.classList.remove('blink');
        timeBar.classList.remove('blink-fast');
    }

    if (gameState.gamePhase === 'STOP') {
        timeBar.classList.remove('blink');
        timeBar.classList.remove('blink-fast');
    }

    if (immediate) {
        setTimeout(() => {
            timeBar.style.transition = 'width 0.2s linear, background-color 0.2s linear';
        }, 50);
    }
}

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
        const teamNameDiv = document.createElement('div')
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

document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket(handleConfigSocketMessage);
    updateDisplay();
    updateTimer();
    updateTimeBar();
    renderTeamScores();
    renderPlayerScores();
});