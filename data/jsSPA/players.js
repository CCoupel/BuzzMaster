let gameState = {
    timer: 30,
    isRunning: false,
    totalTime: 30,
    gamePhase: 'STOP',
    gameTime: 0
};

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
function updateGameState (msg) {
    console.log(msg)
    if(msg.TIME){
        gameState.gameTime = msg.TIME;
    }
    if(msg.PHASE){
        gameState.gamePhase = msg.PHASE;
    }
    gameState.timer = msg.CURRENT_TIME !== undefined ? msg.CURRENT_TIME : msg.DELAY;
    
    if(msg.DELAY){
        gameState.totalTime = msg.DELAY;
    }
};

let localTimer;

function handleConfigSocketMessagePlayers(event) {
    console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) { 
            if (data.POINTS) {
                handleServerAction(data.ACTION, data.MSG, data.POINTS);
            } else {
                handleServerAction(data.ACTION, data.MSG);
            }
        } else {
            console.log("Pas d'action :" , console.log(event.data))
        }
}

function toggleDisplay(target) {
    // Cache l'un avant de montrer l'autre
    const scoreDiv = document.getElementById("score-container")
    const gameDiv = document.getElementById("question-container-players")
    
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

function handleServerAction(action, msg, msgpoints) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':          
            updateGameState(msg.GAME);
            updateTimeBar(true);
            receiveQuestion(msg.GAME.QUESTION)
            break;
        case 'STOP':
            gameState.gamePhase = 'STOP';
            updateTimeBar(true);
            break;
        case 'PAUSE':
            gameState.gamePhase = 'PAUSE';
            updateTimeBar(true);
            break;
        case 'CONTINUE':
            gameState.gamePhase = 'START';
            updateTimeBar(true);
            break;
        case 'UPDATE':
            getBackgroundUrl(msg.GAME);
            applyBackground();
            if (msg.teams && msg.bumpers) {
                updateGameState(msg.GAME);
                updateTimeBar(true);
                updateTimer();
                updateScores(msg);
            }
            if (msgpoints) {
                console.log("Je suis là")
                const { teamId, points } = msgpoints;
                triggerPointsAnimation(teamId, points);
            }
            break;
        case 'UPDATE_TIMER':
            updateGameState(msg.GAME);
            updateTimer();
            updateTimeBar();
            break;
        case 'REVEAL':
            showAnswer(msg)
            break;
        case 'READY':
            cleanUp('question-container')
            cleanUp('answer-container')
            cleanUp('image-container')
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

function cleanUp(container) {
    if (typeof container !== 'string') {
        console.error('Container ID must be a string.');
        return;
    }
    const cleanContainer = document.getElementById(container);
    if (cleanContainer) {
        cleanContainer.innerHTML = '';
    } else {
        console.warn(`Element with ID "${container}" not found.`);
    }
}

let backgroundUrl = '';

function getBackgroundUrl(MSG) {
    backgroundUrl = MSG.background || null;
    console.log(backgroundUrl)
};

function applyBackground() {
    console.log(backgroundUrl)
    document.body.style.backgroundImage =
    `linear-gradient(rgba(255, 255, 255, 0.5)), url('http://buzzcontrol.local${backgroundUrl}')`;
};


function updateTimer() {
    const timerElement = document.getElementById('timer-players');
    const minutes = Math.floor(gameState.timer / 60);
    const seconds = gameState.timer % 60;
    timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
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
  
        // Ajouter le score à la même cellule
        const scoreText = document.createTextNode(player.SCORE || 0);
        scoreButtonCell.appendChild(scoreText);
    
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

function receiveQuestion(data) {
    if (!data || Object.keys(data).length === 0) {
        return;
    }
    
    const question = data;
    const questionContainer = document.getElementById('question-container');
    const answerContainer = document.getElementById('answer-container');
    const imageContainer = document.getElementById('image-container');

    questionContainer.innerHTML= '';
    answerContainer.innerHTML= '';
    imageContainer.innerHTML= '';

    if(question.MEDIA) {
        const questionMedia = document.createElement('img')
        questionMedia.src ="http://buzzcontrol.local" + question.MEDIA;
        imageContainer.appendChild(questionMedia);
    }
    
    const questionDiv = document.createElement('div');
    questionDiv.id = "question-div";

    const questionP = document.createElement('p');
    questionP.innerHTML = question.QUESTION;

    questionDiv.appendChild(questionP);
    questionContainer.appendChild(questionDiv);
}

function showAnswer(data) {
    const answerContainer = document.getElementById('answer-container');
    answerContainer.innerHTML = '';

    if (!data || Object.keys(data).length === 0) {
        return;
    }
    const answer = data;

    if (!document.querySelector('.answer-div')) {
        const answerDiv = document.createElement('div');
        answerDiv.id = "answer-div";

        const answerP = document.createElement('p');
        answerP.innerHTML = answer;

        answerDiv.appendChild(answerP);
        answerContainer.appendChild(answerDiv);
    }
};

const loc = "buzzcontrol.local";
const wsProtocol = "ws:"
const wsUrl = `${wsProtocol}//${loc}/ws`;
let ws;
let reconnectInterval = 5000;

export function connectWebSocketPlayers(onMessageCallback) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        return ws;
    }

    ws = new WebSocket(wsUrl);

    ws.onopen = function(event) {

        console.log('WebSocket connection opened.');
        
        // Nettoyage du tableau lors de la reconnexion
        //cleanBoard();

        sendWebSocketMessage("HELLO", {} );

    };

    ws.onerror = function(event) {
        console.error('WebSocket error:', event);
    }

    ws.onclose = function(event) {

        console.log('WebSocket connection closed. Attempting to reconnect...');
        setTimeout(() => connectWebSocketPlayers(onMessageCallback), reconnectInterval); // Tente de se reconnecter après un délai

    };

    ws.onmessage = onMessageCallback || function(event) {
        console.log('Message reçu:', event.data);
    };
    return ws;

}


export function sendWebSocketMessage (action, MSG= "{}")  {

    if (ws.readyState === WebSocket.OPEN) {
        const message = {
            "ACTION": action,
            "MSG": MSG
        };

    ws.send(JSON.stringify(message));

    } else {
        console.error("WebSocket is not open. Current state: ", ws.readyState);
    }
};

function triggerPointsAnimation(teamName, points) {
    const container = document.createElement('div');
    container.classList.add('points-animation');
    container.textContent = `${teamName} +${points} points !`;

    document.body.appendChild(container);

    // Lancer l'animation d'apparition
    requestAnimationFrame(() => {
        container.classList.add('show');
    });

    // Après 1s (durée de l'animation d'apparition), lancer la disparition
    setTimeout(() => {
        container.classList.remove('show');
        container.classList.add('hide');

        // Attendre la fin de l'animation de disparition avant suppression
        container.addEventListener('animationend', () => {
            container.remove();
        }, { once: true });

    }, 1500); // délai avant disparition (peut être ajusté)
}

document.addEventListener('DOMContentLoaded', function() {
    connectWebSocketPlayers(handleConfigSocketMessagePlayers);
    updateTimer();
    updateTimeBar();
    renderTeamScores();
    renderPlayerScores();
});