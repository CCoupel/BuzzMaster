import { gameState } from './interface.js';
import { sendWebSocketMessage, ws} from './websocket.js';

export let questions = {};
export let fileStorage = {};

function validateFileSize(event) {
    const fileInput = document.querySelector('input[name="file"]');
    const maxSize = 400 * 1024;

    if (fileInput.files.length > 0 && fileInput.files[0].size > maxSize) {
        alert("Le fichier est trop volumineux. La taille maximale autorisée est de 400 Ko.");
        event.preventDefault();
        return false;
    }
    return true;
}


function sendForm(formId, actionUrl) {
    const form = document.getElementById(formId);
    const progressBar = document.getElementById("progressBarQuestion");
    const submitButton = form?.querySelector('button[type="submit"]');

    if (!form || !progressBar || !submitButton) return;

    form.addEventListener('submit', function(event) {
        event.preventDefault();

        if (!validateFileSize(event)) return;

        // On cherche les ID existants dans questions (pas number)
        const usedIds = Object.values(questions)
            .map(q => parseInt(q.ID))
            .filter(n => !isNaN(n))
            .sort((a, b) => a - b);

        // Trouver le plus petit ID manquant
        let nextId = 1;
        for (let i = 0; i < usedIds.length; i++) {
            if (usedIds[i] !== nextId) break;
            nextId++;
        }

        const formData = new FormData(form);

        const numberField = form.querySelector('[name="number"]');
        const currentNumberValue = numberField ? numberField.value.trim() : "";

        // Valide number dans form
        const isValidNumber = /^[1-9]\d*$/.test(currentNumberValue);

        // Si vide ou invalide, on remplace par nextId trouvé via ID
        if (!isValidNumber) {
            const nextNumberStr = nextId.toString().trim();
            formData.set("number", nextNumberStr);
        }
        console.log("FormData avant envoi :");
            for (const [key, val] of formData.entries()) {
                console.log(`- ${key}: "${val}"`);
            }

        const xhr = new XMLHttpRequest();
        xhr.open("POST", actionUrl, true);

        submitButton.disabled = true;

        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                const percentComplete = (e.loaded / e.total) * 100;
                progressBar.value = percentComplete;
            }
        };

        xhr.onload = function() {
            submitButton.disabled = false;
            progressBar.value = 0;

            if (xhr.status >= 200 && xhr.status < 300) {
                alert("Question créée avec succès !");
                console.log("Réponse du serveur :", xhr.responseText);
                questionList();
                form.reset();
            } else {
                alert("Erreur du serveur: " + xhr.statusText);
                console.error("Erreur lors de l'envoi:", xhr.statusText);
            }
        };

        xhr.onerror = function() {
            submitButton.disabled = false;
            progressBar.value = 0;

            alert("Une erreur est survenue lors de la création de la question.");
            console.error("Erreur lors de l'envoi.");
        };

        xhr.send(formData);
    });
}

function sendFileForm(formId, actionUrl) {
    const form = document.getElementById(formId);
    const progressBar = document.getElementById('progressBarBackground');
    const submitButton = form.querySelector('button[type="submit"]');

    form.addEventListener('submit', function(event) {
        event.preventDefault();

        submitButton.disabled = true;
        submitButton.classList.add('disabled'); // Ajout de la classe pour le style

        const formData = new FormData(form);
        const xhr = new XMLHttpRequest();

        xhr.open('POST', actionUrl);

        xhr.upload.onprogress = function(event) {
            if (event.lengthComputable) {
                const percent = (event.loaded / event.total) * 100;
                progressBar.value = percent;
            }
        };

        xhr.onload = function() {
            submitButton.disabled = false;
            submitButton.classList.remove('disabled');

            if (xhr.status === 200) {
                alert('Image envoyée avec succès !');
                console.log('Réponse du serveur :', xhr.responseText);

            } else {
                alert('Erreur lors de l\'upload.');
                console.error('Erreur :', xhr.statusText);
            }
        };

        xhr.onerror = function() {
            submitButton.disabled = false;
            submitButton.classList.remove('disabled');
            alert('Une erreur réseau est survenue.');
        };

        xhr.send(formData);
    });
}

export function getFileStorage(fileStorageData) {
    fileStorage = fileStorageData;
    console.log(fileStorage)
}

export function getQuestions(questionData) {
    questions = questionData;
    console.log(questions)
}


function showCustomConfirm(message) {
  return new Promise((resolve) => {
    const modal = document.getElementById('custom-confirm');
    const msgElem = document.getElementById('confirm-message');
    const yesBtn = document.getElementById('confirm-yes');
    const noBtn = document.getElementById('confirm-no');

    msgElem.textContent = message;
    modal.classList.remove('modal-hidden');

    function cleanup() {
      modal.classList.add('modal-hidden');
      yesBtn.removeEventListener('click', onYes);
      noBtn.removeEventListener('click', onNo);
    }

    function onYes() {
      cleanup();
      resolve(true);
    }

    function onNo() {
      cleanup();
      resolve(false);
    }

    yesBtn.addEventListener('click', onYes);
    noBtn.addEventListener('click', onNo);
  });
}

async function deleteQuestion(questionId) {
  console.log("Suppression demandée pour la question ID:", questionId);

  const confirmed = await showCustomConfirm(`Voulez-vous vraiment supprimer la question ${questionId} ?`);
  if (!confirmed) {
    console.log("Suppression annulée par l'utilisateur.");
    return;
  }

  if (!ws) {
    console.error("WebSocket non initialisée !");
    return;
  }

  console.log("Etat WebSocket :", ws.readyState);

  if (ws.readyState === WebSocket.OPEN) {
    sendWebSocketMessage("DELETE", { "ID": questionId });
    alert(`Suppression effectuée`);
    console.log("Message DELETE envoyé via WebSocket");
  } else {
    console.log('WebSocket not ready. Retrying dans 1s...');
    setTimeout(() => deleteQuestion(questionId), 1000);
  }
}


export function questionList() {
    try {
        const container = document.getElementById('questions-container');
        if (!container) return;
        
        container.innerHTML = '';

        if (!questions || Object.keys(questions).length === 0) {
            container.innerHTML = '<p>Aucune question disponible pour le moment.</p>';
            return;
        }
        // Trier les questions par ID
        const sortedEntries = Object.entries(questions)
            .map(([key, data]) => ({
                key,
                data,
                id: Number(data.ID)
            }))
            .sort((a, b) => a.id - b.id);

        sortedEntries.forEach(({ data }) => {
            const questionDiv = document.createElement('div');
            questionDiv.className = 'question-item';
            questionDiv.innerHTML = `
                <div class="question-header">
                    <button class="edit-button"
                        data-id="${data.ID}"
                        data-number="${data.ID}"
                        data-question="${data.QUESTION}" 
                        data-answer="${data.ANSWER}" 
                        data-points="${data.POINTS}" 
                        data-time="${data.TIME}">
                        ✏️
                    </button>
                    <button class="delete-button" data-id="${data.ID}">
                        ❌
                    </button>
                </div>
                <div class="text">
                    <p><strong>ID:</strong> ${data.ID}</p>
                    <p><strong>Question:</strong> ${data.QUESTION}</p>
                    <p><strong>Réponse:</strong> ${data.ANSWER}</p>
                    <p><strong>Points:</strong> ${data.POINTS}</p>
                    <p><strong>Temps:</strong> ${data.TIME} secondes</p>
                </div>
            `;

            if (data.MEDIA) {
                const imgDiv = document.createElement('div');
                imgDiv.className = 'img';
                imgDiv.innerHTML = `<img src="http://buzzcontrol.local${data.MEDIA}" alt="Question Media">`;
                questionDiv.appendChild(imgDiv);
            }

            container.appendChild(questionDiv);
        });

        // Ajouter l'événement sur les boutons "Modifier"
        document.querySelectorAll('.edit-button').forEach(button => {
            button.addEventListener('click', (event) => {
                const button = event.target;
                document.querySelector('input[name="number"]').value = button.getAttribute('data-number');
                document.querySelector('input[name="question"]').value = button.getAttribute('data-question');
                document.querySelector('input[name="answer"]').value = button.getAttribute('data-answer');
                document.querySelector('input[name="points"]').value = button.getAttribute('data-points');
                document.querySelector('input[name="time"]').value = button.getAttribute('data-time');
            });
        });

        // Ajouter l'événement sur les boutons "Supprimer"
        document.querySelectorAll('.delete-button').forEach(button => {
            button.addEventListener('click', (event) => {
                const questionId = event.target.getAttribute('data-id');
                console.log('Bouton supprimer cliqué, id =', questionId);
                deleteQuestion(questionId);
            });
        });

    } catch (error) {
        console.error("Erreur lors de l'affichage des questions :", error);
    }
}

export function fsInfo() {
    const container = document.getElementById('file-storage-text');
    if (!container) return;

    const numberOfQuestions = Math.round(fileStorage.FREE / 300);
    container.textContent =
        `Espace utilisé: ${fileStorage.USED} / ${fileStorage.TOTAL} Ko, Libre: ${fileStorage.FREE} Ko. Environ ${numberOfQuestions} questions avec image restantes`;

    const progressBar = document.getElementById("file");

    const usedPercent = fileStorage.P_USED;

    if (Number.isFinite(usedPercent)) {
        progressBar.value = usedPercent;
    } else {
        console.warn("fileStorage.P_USED n'est pas un nombre valide :", usedPercent);
        progressBar.value = 0;
    }
}


export function questionsPage() {
    sendForm('question-form', 'http://buzzcontrol.local/questions');
    sendFileForm('background-form', 'http://buzzcontrol.local/background');
    questionList();
    fsInfo();
}
