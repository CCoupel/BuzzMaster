import { connectWebSocket, setBumperPoint, sendWebSocketMessage, updateTeams, updateBumpers, getBumpers, getTeams } from './main.js';

let gameState = {
    timer: 30,
    isRunning: false,
    totalTime: 30,
    gamePhase: 'STOP',
    gameTime: 0
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

export function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) {
            handleServerAction(data.ACTION, data.MSG);
        } else {
            console.log("Pas d'action :" , console.log(event.data))
        }
}

function handleServerAction(action, msg) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':          
            updateGameState(msg.GAME);
            updateTimeBar(true);
            updateDisplay();
            receiveQuestion(msg.GAME.QUESTION)
            break;
        case 'STOP':
            gameState.gamePhase = 'STOP';
            updateTimeBar(true);
            updateDisplay();
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
            if (msg.teams && msg.bumpers) {
                updateTeams(msg.teams);
                updateBumpers(msg.bumpers);
                updateGameState(msg.GAME);
                updateDisplay();
                updateTimeBar(true);
                updateTimer();
                handlePhase(msg.GAME.PHASE);
            }
            break;
        case 'UPDATE_TIMER':
            updateGameState(msg.GAME);
            updateTimer();
            updateTimeBar();
            break;
        case 'REVEAL':
            showAnswer()
            break;
        case 'READY':
            cleanUp('question-container')
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

function sendAction(action, msg = {}) {
        let message;
        switch (action) {
            case 'START':
                const gameTimeInput = document.getElementById('game-time-input');
                const questionId = parseInt(sendQuestionId());
                const gameTime = parseInt(gameTimeInput.value, 10) || 30;
                message = {'DELAY':  gameTime, 'QUESTION': questionId};
                sendWebSocketMessage( action, message);
                break;
            case 'UPDATE':
                message = msg;
                sendWebSocketMessage( action, message);
                break;
            case 'PAUSE':
                message =  msg;
                sendWebSocketMessage( action, message);
                break;
            default:
                message = msg;
                sendWebSocketMessage( action, message);
                break;
        }
        console.log('Envoi de l\'action au serveur:', action, "Message", message);
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
    const timerElement = document.getElementById('timer');
    const minutes = Math.floor(gameState.timer / 60);
    const seconds = gameState.timer % 60;
    timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
}

function addPointToBumper(bumperMac) {
    if (gameState.gamePhase === 'STOP') {
        const pointsInput = document.getElementById('game-points-input');
        const pointsValue = parseInt(pointsInput.value, 10);
        setBumperPoint(bumperMac, pointsValue || 1);
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

function handlePhase(state) {
    const startStopButton = document.getElementById('startStopButton');
    const pauseContinueButton = document.getElementById('pauseContinueButton');
    switch (state) {
        case 'START' :
            startStopButton.textContent = "STOP";
            pauseContinueButton.textContent = "PAUSE";
            break;
        case 'STOP' :
            startStopButton.textContent = "START";
            pauseContinueButton.textContent = "PAUSE";
            break;
        case 'PAUSE' :
            pauseContinueButton.textContent = "CONTINUE";
            startStopButton.textContent = "STOP";
            break;
        case 'CONTINUE' :
            pauseContinueButton.textContent = "PAUSE";
            startStopButton.textContent = "STOP";
            break;
    }
}

async function questionsSelect() {
    try {
        const response = await fetch('http://buzzcontrol.local/questions');
        
        if (!response.ok) {
            throw new Error('Erreur réseau : ' + response.statusText);
        }
        
        const questions = await response.json();
        
        const container = document.getElementById('questions-select');
        const timeInput = document.getElementById('game-time-input');
        const pointsInput = document.getElementById('game-points-input');
        
        Object.keys(questions).forEach(key => {
            const questionData = questions[key];
            const questionOption = document.createElement('option');
            questionOption.value=`${questionData.ID}`;
            questionOption.innerHTML = `Question : ${questionData.ID}`;

            questionOption.setAttribute('data-question', questionData.QUESTION);
            questionOption.setAttribute('data-time', questionData.TIME);
            questionOption.setAttribute('data-points', questionData.POINTS);
            questionOption.setAttribute('data-answer', questionData.ANSWER);
                 
            container.appendChild(questionOption);
        });

        container.addEventListener('change', (event) => {
            const selectedOption = event.target.selectedOptions[0];
            
            const question = selectedOption.getAttribute('data-question')
            const time = selectedOption.getAttribute('data-time');
            const points = selectedOption.getAttribute('data-points');
            const answer = selectedOption.getAttribute('data-answer');
            
            console.log(`Temps : ${time}, Points : ${points},Question : ${question}, Réponse : ${answer}`);
            
            timeInput.value = time || '35';
            pointsInput.value = points || '1';
        });

    } catch (error) {
        console.error('Erreur lors de la récupération des questions :', error);
    }
}


function sendQuestionId() {
    const select = document.getElementById('questions-select');
    const selectedOption = select.selectedOptions[0];
    const questionId = selectedOption.value;
    return questionId;
}

function receiveQuestion(data) {
    if (!data || Object.keys(data).length === 0) {
        return;
    }

    const question = data;
    const timeInput = document.getElementById('game-time-input');
    const pointsInput = document.getElementById('game-points-input');
    const questionsSelect = document.getElementById('questions-select');
    const answerButton = document.getElementById('answer');
    timeInput.value = question.TIME;
    pointsInput.value = question.POINTS;
    questionsSelect.selectedOption = question.ID;

    const questionContainer = document.getElementById('question-container');
    questionContainer.innerHTML= '';

    const questionDiv = document.createElement('div');
    questionDiv.id = "question-div";

    const questionP = document.createElement('p');
    questionP.innerHTML = question.QUESTION;

    const answerP = document.createElement('p');
    answerP.innerHTML = `Réponse :   <span class="hidden-answer">${question.ANSWER}</span>`;

    questionDiv.appendChild(questionP);
    questionDiv.appendChild(answerP);
    questionContainer.appendChild(questionDiv);

    answerButton.addEventListener('click', function() {
        sendAction('REVEAL');
    });
}

function showAnswer() {
    const questionContainer = document.getElementById('question-div');
    if (questionContainer) {
        questionContainer.style.border = '5px solid red'; 
    } else {
        console.warn('Element with ID "question-container" not found.');
    }
}

document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket(handleConfigSocketMessage);
    updateDisplay();
    updateTimer();
    updateTimeBar();
    questionsSelect();

    const playersButton = document.getElementById('players');
    const preparation = document.getElementById('preparation');

    preparation.addEventListener('click', function() {
        sendAction('READY')
    })

    playersButton.onclick = () => {
        window.open('http://buzzcontrol.local/html/players.html')
    }

    startStopButton.addEventListener('click', function() {
        if (gameState.gamePhase ==='STOP') {
            sendAction('START');
        } else {
            startStopButton.textContent = "STOP";
            sendAction('STOP');
        }
    });

    // Gestion du bouton Pause/Continue
    pauseContinueButton.addEventListener('click', function() {
        if (gameState.gamePhase !== 'PAUSE') {
            sendAction('PAUSE');
        } else {
            console.log(gameState.gamePhase)
            sendAction('CONTINUE');
        }
    });

    playerGame.addEventListener('click', function() {
        sendAction('REMOTE', {'REMOTE': 'GAME'});
    });

    playerScores.addEventListener('click', function() {
        sendAction('REMOTE', {'REMOTE' :'SCORE'});
    });
});