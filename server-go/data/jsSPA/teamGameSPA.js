import { gameState } from './interface.js';
import { getBumpers, getTeams, setBumperPoint } from './configSPA.js'
import { getQuestions, questions } from './questionsSPA.js';
import { sendAction } from './interface.js';

let selectedQuestion = {};

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
        teamTitle.className = 'team-name';
        teamTitle.textContent = teamName;
        
        if (gameState.gamePhase === "PREPARE" && teamData.READY === "FALSE") {
            teamElement.classList.remove('ready');
            teamElement.classList.add('not-ready');
        } else if (gameState.gamePhase === "PREPARE" && teamData.READY === "TRUE") {
            teamElement.classList.remove('not-ready');
            teamElement.classList.add('ready');
        } else {
            teamElement.classList.remove('not-ready');
            teamElement.classList.remove('ready');
        }

        teamHeader.appendChild(teamColor);
        teamHeader.appendChild(teamTitle);
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
            if (bumperData.READY === "FALSE") {
                bumperElement.classList.add('not-ready');
            }
            if (bumperData.READY === "TRUE") {
                bumperElement.classList.remove('not-ready');
                bumperElement.classList.add('ready');
            }

            if (gameState.gamePhase != "PREPARE") {
                bumperElement.classList.remove('ready');
                bumperElement.classList.remove('not-ready');
            }
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


export function receiveQuestion(data) {
    if (!data || Object.keys(data).length === 0) {
        return;
    }

    // Stocker les informations dans selectedQuestion
    selectedQuestion = { ...data };

    // Mettre à jour les inputs du formulaire
    const timeInput = document.getElementById('game-time-input');
    const pointsInput = document.getElementById('game-points-input');

    if (timeInput) timeInput.value = selectedQuestion.TIME;
    if (pointsInput) pointsInput.value = selectedQuestion.POINTS;
    displayQuestion();
};

export function displayQuestion() {
    const questionContainer = document.getElementById('question-container-admin');
    if (!questionContainer) return;
    console.log("j'affiche les questions")
    // Vider le conteneur
    questionContainer.innerHTML = '';

    if (!selectedQuestion || Object.keys(selectedQuestion).length === 0) {
        return;
    }

    // Ajouter un média s'il existe
    if (selectedQuestion.MEDIA) {
        const questionMedia = document.createElement('img');
        questionMedia.src = selectedQuestion.MEDIA;
        questionContainer.appendChild(questionMedia);
    }

    // Création des éléments HTML pour la question et la réponse
    const questionDiv = document.createElement('div');
    questionDiv.id = "question-div-admin";

    const questionP = document.createElement('p');
    questionP.innerHTML = selectedQuestion.QUESTION;

    const answerP = document.createElement('p');
    answerP.innerHTML = `Réponse : ${selectedQuestion.ANSWER}`;

    questionDiv.appendChild(questionP);
    questionDiv.appendChild(answerP);
    questionContainer.appendChild(questionDiv);
}

function updateQuestionClass(element, status) {
  const statusClasses = ["STARTED", "STOPPED", "AVAILABLE", "REVEALED", "PAUSED"];
  element.classList.remove(...statusClasses);
  if (statusClasses.includes(status)) {
    element.classList.add(status);
  } else {
    element.classList.add("AVAILABLE")
  }
}

export function questionsSelectList() {
    const container = document.getElementById('questions-select-list');
    if (!container) return;
    container.innerHTML = '';

    if (!questions || Object.keys(questions).length === 0) {
        container.innerHTML = '<p>Aucune question disponible pour le moment.</p>';
        return;
    }

    console.log("questions values:", Object.values(questions));
    const sortedQuestions = Object.values(questions)
    .filter(q => q && q.ID !== undefined) // remove null/invalid
    .sort((a, b) => parseInt(a.ID) - parseInt(b.ID));
    
    sortedQuestions.forEach(questionData => {
        const questionDiv = document.createElement('div');
        questionDiv.className = 'question-item';
        questionDiv.id = `question-${questionData.ID}`;

        updateQuestionClass(questionDiv, questionData.STATUS);

        if (selectedQuestion && selectedQuestion.ID === questionData.ID) {
            if (gameState.gamePhase === "PREPARE") {
                questionDiv.style.backgroundColor = '#fff176'; 
            } else {
                questionDiv.style.backgroundColor = '#aed581';
            }
        }

        questionDiv.innerHTML = `
            <p><strong>ID:</strong> ${questionData.ID} <strong>Question:</strong> ${questionData.QUESTION}</p>
        `;

        let popup = document.getElementById(`popup-question-${questionData.ID}`);
        if (!popup) {
            popup = document.createElement('div');
            popup.className = 'popup';
            popup.id = `popup-question-${questionData.ID}`;

            popup.innerHTML = `
                <p><strong>Question:</strong> ${questionData.QUESTION}</p>
                <p><strong>Réponse:</strong> ${questionData.ANSWER}</p>
                <p><strong>Points:</strong> ${questionData.POINTS}</p>
                <p><strong>Temps:</strong> ${questionData.TIME} sec</p>
                ${questionData.MEDIA ? `<img src="${questionData.MEDIA}" alt="Question Media">` : ''}
            `;
            popup.style.display = 'none';
            popup.style.position = 'absolute';
            popup.style.zIndex = '1000';

            document.body.appendChild(popup);

            popup.addEventListener('mouseleave', () => {
                if (!questionDiv.matches(':hover')) {
                    popup.style.display = 'none';
                }
            });
        }

        let popupTimeout;

        questionDiv.addEventListener('mouseenter', (event) => {
            popupTimeout = setTimeout(() => {
                popup.style.display = 'block';
                const rect = questionDiv.getBoundingClientRect(); 

                let offsetX = 15; 
                let offsetY = -popup.offsetHeight - 10; 

                popup.style.left = `${rect.right + offsetX}px`; 
                popup.style.top = `${rect.top + offsetY}px`;
            }, 500);
        });

        questionDiv.addEventListener('mouseleave', () => {
            clearTimeout(popupTimeout);
            hidePopup();
        });

        popup.addEventListener('mouseleave', () => {
            if (!questionDiv.matches(':hover')) {
                hidePopup();
            }
        });

        const hidePopup = () => {
            popup.style.display = 'none';
        };

        if (["STOP", "PREPARE", "READY"].includes(gameState.gamePhase)) {
            questionDiv.addEventListener('click', () => {
                document.querySelectorAll('.question-item').forEach(item => {
                    item.style.backgroundColor = '';
                });
    
                questionDiv.style.backgroundColor = 'yellow';
    
                const allPopups = document.querySelectorAll('.popup');
                allPopups.forEach(popup => {
                    popup.style.display = 'none';
                });
    
                selectedQuestion = { ...questionData };
    
                sendAction("READY", questionData.ID);
            });
        } else {
            questionDiv.style.opacity = '0.5'; // Visuellement indiquer que ce n'est pas cliquable
            questionDiv.style.cursor = 'not-allowed';
        }

        container.appendChild(questionDiv);
    });
}

function showAnswer() {
    const questionContainer = document.getElementById('question-div-admin');
    if (questionContainer) {
        questionContainer.style.border = '5px solid red'; 
    } else {
        console.warn('Element with ID "question-container-admin" not found.');
    }
}


export function teamGamePage() {
    updateDisplayGame();
    questionsSelectList();
    displayQuestion();
}

/*document.addEventListener('DOMContentLoaded', function() {
    updateDisplayGame();
    questionsSelect();
});*/