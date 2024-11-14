async function sendForm(formId, actionUrl) {
    const form = document.getElementById(formId);

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

        } catch (error) {
            console.error("Erreur lors de l'envoi:", error);
            alert("Une erreur est survenue lors de la création de la question.");
        }
    });
}

async function questionList() {
    try {
        const response = await fetch('http://buzzcontrol.local/questions');
        
        if (!response.ok) {
            throw new Error('Erreur réseau : ' + response.statusText);
        }
        
        const questions = await response.json();
        
        const container = document.getElementById('questions-container');
        
        container.innerHTML='';

        Object.keys(questions).forEach(key => {
            const questionData = questions[key];
            
            const questionDiv = document.createElement('div');
            questionDiv.className = 'question-item';

            questionDiv.innerHTML = `
                <p><strong>ID:</strong> ${questionData.ID}</p>
                <p><strong>Question:</strong> ${questionData.QUESTION}</p>
                <p><strong>Réponse:</strong> ${questionData.ANSWER}</p>
                <p><strong>Points:</strong> ${questionData.POINTS}</p>
                <p><strong>Temps:</strong> ${questionData.TIME} secondes</p>
            `;
            
            container.appendChild(questionDiv);
        });
    } catch (error) {
        console.error('Erreur lors de la récupération des questions :', error);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    sendForm('question-form', 'http://buzzcontrol.local/questions');
    questionList();
});