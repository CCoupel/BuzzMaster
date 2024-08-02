  // Connectez-vous au serveur WebSocket
  //const loc = window.location;
  const loc = "buzzcontrol.local";
  //const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
  const wsProtocol = "ws:"
  //const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
  const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

  // Initialiser l'objet WebSocket
  const ws = new WebSocket(wsUrl);

  // Écouter les messages du serveur
  ws.onmessage = function(event) {
      console.log('Message du serveur:', event.data);
  };

  // Envoyer un message au serveur
  ws.onopen = function() {
      console.log('Connecté au serveur WebSocket');
      ws.send('Salut, serveur WebSocket !');
  };

  // Événement de fermeture de la connexion
  ws.onclose = function() {
      console.log('Connexion fermée');
  };


  function createBuzzerDiv(buzzerData) {
    // Récupérer le conteneur principal
    const container = document.querySelector('.buzzer-container');
    
    // Nettoyer le conteneur avant d'ajouter les nouvelles divs
    container.innerHTML = '';

    // Itérer sur chaque buzzer dans l'objet
    for (const [id, data] of Object.entries(buzzerData.bumpers)) {
        // Créer une nouvelle div pour chaque buzzer
        const buzzerDiv = document.createElement('div');
        buzzerDiv.id = id;
        buzzerDiv.className = 'buzzer';

        // Créer un élément pour afficher l'ID du buzzer
        const idElement = document.createElement('p');
        idElement.className = 'buzzer-id';
        idElement.textContent = `ID: ${id}   Nom: `;

        // Créer le formulaire de texte
        const form = document.createElement('form');
        form.className = 'buzzer-form';

        // Créer le champ de texte
        const input = document.createElement('input');
        input.type = 'text';
        input.name = 'buzzer-text';
        input.placeholder = 'Entrez un texte';

        // Ajouter le champ de texte au formulaire
        form.appendChild(input);

        // Ajouter l'élément ID et le formulaire à la div du buzzer
        buzzerDiv.appendChild(idElement);
        buzzerDiv.appendChild(form);

        // Ajouter la div du buzzer au conteneur principal
        container.appendChild(buzzerDiv);
    }
}

// Gestion des messages reçus
ws.onmessage = function(event) {
    try {
        // Parse le JSON reçu
        const data = JSON.parse(event.data);

        // Appeler la fonction pour créer les divs
        createBuzzerDiv(data);
    } catch (error) {
        console.error('Erreur lors du traitement des données JSON:', error);
    }
};
