import { sendWebSocketMessage, ws} from './websocket.js';

export let questions = {};

async function sendForm(formId, actionUrl) {
    const form = document.getElementById(formId);

    if (!form) return;

    form.addEventListener('submit', async function(event) {
        event.preventDefault();

        const formData = new FormData(form);

        try {
            const response = await fetch(actionUrl, {
                method: 'POST',
                body: formData 
            });

            if (!response.ok) {
                throw new Error("Erreur du serveur: " + response.statusText);
            }

            const responseData = await response.text();

            alert("Question crée avec succès !");
            console.log("Réponse du serveur :", responseData);
            questionList();

            form.reset();
        } catch (error) {
            console.error("Erreur lors de l'envoi:", error);
            alert("Une erreur est survenue lors de la création de la question.");
        }
    });
};


export function getQuestions(questionData) {
    questions = questionData;
    console.log(questions)
}


/*
export async function fetchQuestions() {
    try {
        // Vérifier si l'élément loader existe avant d'essayer de manipuler son style
        const loader = document.getElementById('loader');
        if (loader) {
            loader.style.display = 'block'; // Afficher le loader
        }

        const response = await fetch('http://buzzcontrol.local/questions');

        if (!response.ok) {
            throw new Error('Erreur réseau : ' + response.statusText);
        }

        const newQuestions = await response.json();

        // Vérifier si les nouvelles données sont différentes des précédentes
        if (JSON.stringify(questions) !== JSON.stringify(newQuestions)) {
            questions = newQuestions;
            localStorage.setItem('questions', JSON.stringify(questions)); // Sauvegarder dans le localStorage
            questionList(); // Rafraîchir l'affichage si les données ont changé
        } else {
            questionList(); // Afficher la liste même sans changement
        }
    } catch (error) {
        console.error('Erreur lors de la récupération des questions :', error);
        const savedQuestions = localStorage.getItem('questions');
        if (savedQuestions) {
            questions = JSON.parse(savedQuestions); // Récupérer les questions depuis localStorage si l'API échoue
        }
        questionList(); // Toujours afficher la liste, même en cas d'erreur
    } finally {
        // Vérifier à nouveau si le loader existe avant de le masquer
        const loader = document.getElementById('loader');
        if (loader) {
            loader.style.display = 'none'; // Masquer le loader une fois le chargement terminé
        }
    }
};
*/
function deleteQuestion(questionId) {
    if (!confirm(`Voulez-vous vraiment supprimer la question ${questionId} ?`)) {
        return;
    }

    // Envoyer la demande de suppression au serveur via WebSocket
    if (ws.readyState === WebSocket.OPEN) {
        sendWebSocketMessage("DELETE", { "ID": questionId });
        alert(`Suppression effectuée`);
    } else {
        console.log('WebSocket not ready. Retrying...');
        setTimeout(() => deleteQuestion(questionId), 1000); // Réessaie après 1 seconde
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
                deleteQuestion(questionId);
            });
        });

    } catch (error) {
        console.error("Erreur lors de l'affichage des questions :", error);
    }
}


export function questionsPage() {
    sendForm('question-form', 'http://buzzcontrol.local/questions');
    questionList();
};
