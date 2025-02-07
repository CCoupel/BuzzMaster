import { sendWebSocketMessage } from './websocket.js';
import { gameState } from './interface.js';
//import { setBumperPoint,  updateTeams, updateBumpers, getBumpers, getTeams } from './main.js';
import { updateTeams, updateBumpers, getBumpers, getTeams } from './configSPA.js'

export function handleConfigSocketMessage(event) {
    console.log('Message reçu du serveur:', event.data);
        const data = JSON.parse(event.data);
        if (data.ACTION) {
            handleServerAction(data.ACTION, data.MSG);
        } else {
            console.log("Pas d'action :" , console.log(event.data))
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


export function updateDisplayGame() {
    console.log('Mise à jour de l\'affichage avec l\'état du jeu:', gameState);
    const container = document.getElementById('game-container');
    if (!container) return;
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


function addPointToBumper(bumperMac) {
    if (gameState.gamePhase === 'STOP') {
        const pointsInput = document.getElementById('game-points-input');
        const pointsValue = parseInt(pointsInput.value, 10);
        setBumperPoint(bumperMac, pointsValue || 1);
        updateDisplayGame();
    }
}

function receiveQuestion(data) {
    if (!data || Object.keys(data).length === 0) {
        return;
    }

    const question = data;
    const timeInput = document.getElementById('game-time-input');
    const pointsInput = document.getElementById('game-points-input');
    const questionsSelect = document.getElementById('questions-select');
    timeInput.value = question.TIME;
    pointsInput.value = question.POINTS;
    questionsSelect.selectedOption = question.ID;

    const questionContainer = document.getElementById('question-container-admin');
    questionContainer.innerHTML= '';
    
    if(question.MEDIA) {
        const questionMedia = document.createElement('img')
        questionMedia.src ="http://buzzcontrol.local" + question.MEDIA;
        questionContainer.appendChild(questionMedia);
    }

    const questionDiv = document.createElement('div');
    questionDiv.id = "question-div-admin";

    const questionP = document.createElement('p');
    questionP.innerHTML = question.QUESTION;

    const answerP = document.createElement('p');
    answerP.innerHTML = `Réponse :   <span class="hidden-answer">${question.ANSWER}</span>`;

    questionDiv.appendChild(questionP);
    questionDiv.appendChild(answerP);
    questionContainer.appendChild(questionDiv);
}

function showAnswer() {
    const questionContainer = document.getElementById('question-div-admin');
    if (questionContainer) {
        questionContainer.style.border = '5px solid red'; 
    } else {
        console.warn('Element with ID "question-container-admin" not found.');
    }
}

/*document.addEventListener('DOMContentLoaded', function() {
    updateDisplayGame();
    questionsSelect();
});*/