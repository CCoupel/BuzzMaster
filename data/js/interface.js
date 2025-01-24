import {sendWebSocketMessage} from './websocket.js';

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
}

export function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) {
            handleServerAction(data.ACTION, data.MSG);
        } else {
            console.log("Pas d'action :" , console.log(event.data))
        };
};

function handleServerAction(action, msg) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':          
            updateGameState(msg.GAME);
            updateTimeBar(true);
            //updateDisplay();
            break;
        case 'STOP':
            gameState.gamePhase = 'STOP';
            updateTimeBar(true);
            //updateDisplay();
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
                //updateTeams(msg.teams);
                //updateBumpers(msg.bumpers);
                updateGameState(msg.GAME);
                //updateDisplay();
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
            cleanUp('question-container-admin')
            receiveQuestion(msg.QUESTION)
            break;
        default:
            console.log('Action non reconnue:', action);
    }
};

function sendAction(action, msg = {}) {
        let message;
        switch (action) {
            case 'START':
                const gameTimeInput = document.getElementById('game-time-input');
                const gamePointsInput = document.getElementById('game-points-input');
                const gameTime = parseInt(gameTimeInput.value, 10) || 30;
                const gamePoints = parseInt(gamePointsInput.value, 10) || 30;
                message = {'DELAY':  gameTime, 'POINTS': gamePoints};
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
            case 'READY':
                console.log("oui j'envoie")
                const questionId = parseInt(sendQuestionId());
                message = {'QUESTION': questionId};
                sendWebSocketMessage( action, message);
                break;
            default:
                message = msg;
                sendWebSocketMessage( action, message);
                break;
        }
    console.log('Envoi de l\'action au serveur:', action, "Message", message);
};

export function updateTimer() {
    const timerElement = document.getElementById('timer');
    const minutes = Math.floor(gameState.timer / 60);
    const seconds = gameState.timer % 60;
    timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
}

export function updateTimeBar(immediate = false) {
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
};

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
};

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
};


document.addEventListener('DOMContentLoaded', function() {
    updateTimer();
    updateTimeBar();
    questionsSelect();

    const playersButton = document.getElementById('players');
    const preparation = document.getElementById('preparation');
    const answerButton = document.getElementById('answer');

    
    answerButton.addEventListener('click', function() {
        sendAction('REVEAL');
    });

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