import {sendWebSocketMessage} from './websocket.js';
import { updateBumpers, updateTeams, updateDisplayConfig, configPage } from './configSPA.js';
import { scorePage } from './scoreSPA.js';
import { getQuestions, questionList, getFileStorage, fsInfo } from './questionsSPA.js';
import { teamGamePage,receiveQuestion } from './teamGameSPA.js';

export let gameState = {
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
export let webSocketMessage = {};

export function createElement(tag, className, attributes = {}) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, value));
    return element;
};

export function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
    webSocketMessage = JSON.parse(event.data);
        if (webSocketMessage.ACTION) {
            handleServerAction(webSocketMessage.ACTION, webSocketMessage.MSG, webSocketMessage.FSINFO);
        } else {
            console.log("Pas d'action :" , console.log(event.data))
        };
};

function handleServerAction(action, msg, fsinfo) {
    console.log('Action reçue du serveur:', action);
    switch (action) {
        case 'START':          
            updateGameState(msg.GAME);
            updateTimeBar(true);
            //updateDisplayConfig();
            break;
        case 'STOP':
            gameState.gamePhase = 'STOP';
            updateTimeBar(true);
            //updateDisplayConfig();
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
            }
            if (msg.GAME.REMOTE) {
                remoteDisplay(msg.GAME.REMOTE)
            }
            switch (window.location.hash || "#config") {
                case '#config':
                    console.log("testconfig")
                    configPage();
                    break;
                case '#score':
                    scorePage(); 
                    break;
                case '#teamGame':
                    teamGamePage();
                    break;
            }
            updateGameState(msg.GAME);
            updateTimeBar(true);
            updateTimer();
            handlePhase(msg.GAME.PHASE);
            receiveQuestion(msg.GAME.QUESTION);
            getFileStorage(fsinfo);
            fsInfo();
            break;
        case 'UPDATE_TIMER':
            updateGameState(msg.GAME);
            updateTimer();
            updateTimeBar();
            break;
        case 'REVEAL':
            break;
        case 'READY':
            cleanUp('question-container-admin')
            receiveQuestion(msg.QUESTION)
            break;
        case 'QUESTIONS':
            getQuestions(msg);
            getFileStorage(fsinfo);
            fsInfo();
            questionList();
            if (window.location.hash === "#teamGame") {
                teamGamePage();
            }
            break;
        default:
            console.log('Action non reconnue:', action);
            break;
    }
};

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

export function sendAction(action, msg = {}) {
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
                message = {'QUESTION': msg};
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
    if (!timerElement) return;
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

let isGame = true;

function remoteDisplay(msg) {
    const toggleButton = document.getElementById('toggleButton');
    if (msg === "GAME") {
        toggleButton.classList.remove('active');
        isGame = true;
    } else if (msg === "SCORE") {
        toggleButton.classList.add('active');
        isGame = false;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    updateTimer();
    updateTimeBar();

    const playersButton = document.getElementById('players');
    const answerButton = document.getElementById('answer');

    if (answerButton) {
        answerButton.addEventListener('click', function() {
            sendAction('REVEAL');
        });
    }

    if (playersButton) {
        playersButton.onclick = () => {
            window.open('http://buzzcontrol.local/html/players.html')
        }
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

    document.getElementById('toggleButton').addEventListener('click', function() {
        isGame = !isGame;
        const newState = isGame ? 'GAME' : 'SCORE';

        remoteDisplay(newState);  // Met à jour l'affichage
        sendAction('REMOTE', {'REMOTE': newState});  // Envoie l'action
    });
});