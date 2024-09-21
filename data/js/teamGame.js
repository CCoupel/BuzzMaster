let socket;
let gameState = {
    teams: {},
    bumpers: {},
    timer: 30,
    isRunning: false,
    totalTime: 30,
    gamePhase: 'STOP'
};
let localTimer;

function connectWebSocket() {
    socket = new WebSocket('ws://buzzcontrol.local/ws');

    socket.onopen = function(event) {
        console.log('Connecté au serveur WebSocket');
        sendAction('HELLO');
    };

    socket.onmessage = function(event) {
        console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) {
            handleServerAction(data.ACTION, data.MSG);
        } else if (data.TIME !== undefined) {
            gameState.timer = data.TIME;
            updateTimer();
            updateTimeBar();
        }
    };

    socket.onclose = function(event) {
        console.log('Déconnecté du serveur WebSocket');
        setTimeout(connectWebSocket, 5000);
    };

    socket.onerror = function(error) {
        console.error('Erreur WebSocket:', error);
    };
}

function handleServerAction(action, msg) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':
            gameState.gamePhase = 'START';
            updateTimeBar(true);
            startTimer();
            updateDisplay();
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
                gameState.teams = msg.teams;
                gameState.bumpers = msg.bumpers;
                updateDisplay();
            }
            break;
        default:
            console.log('Action non reconnue:', action);
    }
}

function sendAction(action, msg = '') {
    if (socket.readyState === WebSocket.OPEN) {
        let message;
        switch (action) {
            case 'START':
                const gameTimeInput = document.getElementById('game-time-input');
                const gameTime = parseInt(gameTimeInput.value, 10) || 30;
                gameState.totalTime = gameTime;
                gameState.timer = gameTime;
                updateTimeBar(true);
                message = { ACTION: action, MSG: gameTime.toString() };
                break;
            case 'UPDATE':
                message = { ACTION: action, MSG: msg };
                break;
            case 'HELLO':
                message = { ACTION: action, MSG: "Salut, serveur WebSocket !" };
                break;
            default:
                message = { ACTION: action, MSG: msg };
                break;
        }
        console.log('Envoi de l\'action au serveur:', message);
        socket.send(JSON.stringify(message));
    } else {
        console.error("La connexion WebSocket n'est pas ouverte");
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
            if (gameState.timer === 0) {
                sendAction('STOP');
            }
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

    const sortedTeams = Object.entries(gameState.teams).sort((a, b) => {
        const delayA = a[1].DELAY !== undefined ? a[1].DELAY : Infinity;
        const delayB = b[1].DELAY !== undefined ? b[1].DELAY : Infinity;
        return delayA - delayB;
    });

    sortedTeams.forEach(([teamName, teamData]) => {
        const teamElement = document.createElement('div');
        const isStartPhase = gameState.gamePhase === 'START';
        const isTeamActive = teamData.DELAY !== undefined;
        teamElement.className = `team ${isTeamActive ? 'active' : ''} ${isStartPhase && !isTeamActive ? 'start-phase' : ''}`;
        
        const teamHeader = document.createElement('div');
        teamHeader.className = 'team-header';
        
        const teamColor = document.createElement('div');
        teamColor.className = 'team-color';
        teamColor.style.backgroundColor = `rgb(${teamData.COLOR.join(',')})`;
        
        const teamTitle = document.createElement('h2');
        teamTitle.textContent = teamName;
        
        teamHeader.appendChild(teamColor);
        teamHeader.appendChild(teamTitle);
        teamElement.appendChild(teamHeader);

        const teamBumpers = Object.entries(gameState.bumpers)
            .filter(([_, bumperData]) => bumperData.TEAM === teamName)
            .sort((a, b) => {
                const delayA = a[1].DELAY_TEAM !== undefined ? a[1].DELAY_TEAM : Infinity;
                const delayB = b[1].DELAY_TEAM !== undefined ? b[1].DELAY_TEAM : Infinity;
                return delayA - delayB;
            });

        teamBumpers.forEach(([bumperMac, bumperData]) => {
            const bumperElement = document.createElement('div');
            const isBumperActive = bumperData.BUTTON !== undefined || bumperData.DELAY !== undefined;
            bumperElement.className = `bumper ${isBumperActive ? 'active' : ''} ${isStartPhase && !isBumperActive ? 'start-phase' : ''}`;
            bumperElement.textContent = `${bumperData.NAME} (Score: ${bumperData.SCORE || 0})`;
            
            if (gameState.gamePhase === 'STOP' && isBumperActive) {
                bumperElement.onclick = () => addPointToBumper(teamName, bumperMac);
                bumperElement.style.cursor = 'pointer';
            }
            
            teamElement.appendChild(bumperElement);
        });

        container.appendChild(teamElement);
    });
}

function updateTimer() {
    console.log('Mise à jour du timer:', gameState.timer);
    const timerElement = document.getElementById('timer');
    const minutes = Math.floor(gameState.timer / 60);
    const seconds = gameState.timer % 60;
    timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
}

function addPointToBumper(teamName, bumperMac) {
    if (gameState.gamePhase === 'STOP') {
        if (gameState.bumpers[bumperMac]) {
            gameState.bumpers[bumperMac].SCORE = (gameState.bumpers[bumperMac].SCORE || 0) + 1;
            updateDisplay();
            sendAction('UPDATE', { bumpers: gameState.bumpers });
        }
    }
}

function updateTimeBar(immediate = false) {
    console.log('Mise à jour de la barre de temps');
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

    if (gameState.timer <= 10) {
        timeBar.classList.add('blink');
    } else {
        timeBar.classList.remove('blink');
    }

    if (immediate) {
        setTimeout(() => {
            timeBar.style.transition = 'width 0.2s linear, background-color 0.2s linear';
        }, 50);
    }
}

document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket();
    updateDisplay();
    updateTimer();
    updateTimeBar();

    document.getElementById('startButton').addEventListener('click', () => sendAction('START'));
    document.getElementById('stopButton').addEventListener('click', () => sendAction('STOP'));
    document.getElementById('pauseButton').addEventListener('click', () => sendAction('PAUSE'));
    document.getElementById('continueButton').addEventListener('click', () => sendAction('CONTINUE'));
});