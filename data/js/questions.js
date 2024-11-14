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

        } catch (error) {
            console.error("Erreur lors de l'envoi:", error);
            alert("Une erreur est survenue lors de la création de la question.");
        }
    });
}

document.addEventListener('DOMContentLoaded', () => {
    sendForm('question-form', 'http://buzzcontrol.local/questions');
});