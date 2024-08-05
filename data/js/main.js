  // Connectez-vous au serveur WebSocket
  //const loc = window.location;
  const loc = "buzzcontrol.local";
  //const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
  const wsProtocol = "ws:"
  //const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
  const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

  // Initialiser l'objet WebSocket
  const ws = new WebSocket(wsUrl);


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
    console.log(buzzerData);
    const container = document.querySelector('.buzzer-container');

    // Nettoyer le conteneur avant d'ajouter les nouvelles divs
    container.innerHTML = '';

    // Itérer sur chaque buzzer dans l'objet
    for (const [id, data] of Object.entries(buzzerData.bumpers)) {
        // Créer une nouvelle div pour chaque buzzer
        const buzzerDiv = document.createElement('div');
        buzzerDiv.id = `buzzer-${id}`;
        buzzerDiv.className = 'buzzer';

        // Créer un élément pour afficher l'ID du buzzer
        const idElement = document.createElement('p');
        idElement.className = 'buzzer-id';
        idElement.textContent = `ID: ${id}`;

        // Ajouter l'élément ID à la div du buzzer
        buzzerDiv.appendChild(idElement);

        // Vérifier si le buzzer a déjà un nom non vide
        if (data.NAME) {
            // Afficher le nom du joueur
            const nameElement = document.createElement('p');
            nameElement.className = 'buzzer-name';
            nameElement.textContent = `Nom: ${data.NAME}`;

            // Ajouter un bouton pour supprimer le nom
            const deleteButton = document.createElement('button');
            deleteButton.textContent = 'Supprimer le nom';
            deleteButton.addEventListener('click', () => {
                // Construire le message JSON pour supprimer le nom
                const message = {
                    "bumpers": {
                        [id]: {
                            "NAME": "",
                            "TEAM": data.TEAM,
                            "IP": data.IP,
                            "TIME": data.TIME,
                            "BUTTON": data.BUTTON
                        }
                    }
                };

                // Envoyer les données sous forme de JSON via WebSocket
                ws.send(JSON.stringify(message));

                // Mettre à jour l'interface utilisateur pour afficher le formulaire à la place du nom
                buzzerDiv.removeChild(nameElement);
                buzzerDiv.removeChild(deleteButton);
                buzzerDiv.appendChild(form);
            });

            // Ajouter le nom et le bouton de suppression à la div du buzzer
            buzzerDiv.appendChild(nameElement);
            buzzerDiv.appendChild(deleteButton);
        } else {
            // Créer le formulaire de texte
            const form = document.createElement('form');
            form.className = 'buzzer-form';

            // Créer le champ de texte avec un label pour l'accessibilité
            const label = document.createElement('label');
            label.textContent = 'Nom du joueur';
            label.htmlFor = `buzzer-text-${id}`;

            const input = document.createElement('input');
            input.type = 'text';
            input.name = 'buzzer-text';
            input.id = `buzzer-text-${id}`;
            input.placeholder = 'Nom du joueur';
            input.value = data.NAME || ''; // Pré-remplir le champ avec le nom existant, s'il y en a un

            // Ajouter le label et le champ de texte au formulaire
            form.appendChild(label);
            form.appendChild(input);

            // Empêcher la soumission du formulaire de recharger la page et envoyer les données via WebSocket
            form.addEventListener('submit', (e) => {
                e.preventDefault();

                // Récupérer la valeur du champ de texte
                const playerName = input.value;

                // Construire le message JSON conforme
                const message = {
                    "bumpers": {
                        [id]: {
                            "NAME": playerName,
                            "TEAM": data.TEAM,
                            "IP": data.IP,
                            "TIME": data.TIME,
                            "BUTTON": data.BUTTON
                        }
                    }
                };

                // Envoyer les données sous forme de JSON via WebSocket
                ws.send(JSON.stringify(message));

                // Mettre à jour l'interface utilisateur pour afficher le nom au lieu du formulaire
                const nameElement = document.createElement('p');
                nameElement.className = 'buzzer-name';
                nameElement.textContent = `Nom: ${playerName}`;

                // Ajouter un bouton pour supprimer le nom
                const deleteButton = document.createElement('button');
                deleteButton.textContent = 'Supprimer le nom';
                deleteButton.addEventListener('click', () => {
                    // Construire le message JSON pour supprimer le nom
                    const deleteMessage = {
                        "bumpers": {
                            [id]: {
                                "NAME": "",
                                "TEAM": data.TEAM,
                                "IP": data.IP,
                                "TIME": data.TIME,
                                "BUTTON": data.BUTTON
                            }
                        }
                    };

                    // Envoyer les données sous forme de JSON via WebSocket
                    ws.send(JSON.stringify(deleteMessage));

                    // Mettre à jour l'interface utilisateur pour afficher le formulaire à la place du nom
                    buzzerDiv.removeChild(nameElement);
                    buzzerDiv.removeChild(deleteButton);
                    buzzerDiv.appendChild(form);
                });

                buzzerDiv.replaceChild(nameElement, form);
                buzzerDiv.appendChild(deleteButton);
            });

            // Ajouter le formulaire à la div du buzzer
            buzzerDiv.appendChild(form);
        }

        // Ajouter la div du buzzer au conteneur principal
        container.appendChild(buzzerDiv);
    }
}


document.addEventListener('DOMContentLoaded', () => {
    const button = document.getElementById('addDivButton');
    const container = document.querySelector('.team-container');

    button.addEventListener('click', () => {
        // Demander à l'utilisateur de saisir le titre de la nouvelle div
        const titleText = prompt('Nom de la team :', 'Team ');

        if (titleText !== null && titleText.trim() !== '') {
            // Créer l'objet team avec un identifiant unique et le nom
            const teamData = {
                teams: {}
            };
            teamData.teams[titleText] = {};

            // Envoyer les informations de la nouvelle équipe au WebSocket
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify(teamData));
                console.log('Team data sent:', teamData);
            } else {
                console.log('WebSocket is not open. Cannot send team data.');
            }

            // Créer la nouvelle div pour l'équipe et l'ajouter au conteneur
            createTeamDiv(teamData.teams);
        } else {
            alert('Le titre ne peut pas être vide.');
        }
    });
});

function createTeamDiv(teams) {
    const container = document.querySelector('.team-container');

    for (const id in teams) {
        console.log(`ID : ${id}`);
        // Vérifier si une div avec le même ID existe déjà
        if (document.getElementById(id)) {
            console.log(`La div avec l'ID ${id} existe déjà.`);
            continue;
        }

        // Créer la nouvelle div
        const newDiv = document.createElement('div');
        newDiv.className = 'dynamic-div';
        newDiv.id = id;

        // Créer et ajouter le titre <h2>
        const title = document.createElement('h2');
        title.className = 'team-name';
        title.textContent = id;
        newDiv.appendChild(title);

        const text = document.createElement('p');
        text.className = 'team-member-p';
        text.textContent = 'Membres de la team :';
        newDiv.appendChild(text);

        // Créer et ajouter la zone de dépôt
        const dropzone = document.createElement('div');
        dropzone.className = 'dropzone';
        newDiv.appendChild(dropzone);

        // Ajouter la nouvelle div au conteneur
        container.appendChild(newDiv);
    }

    // Ajouter le bouton après la nouvelle div
    const button = document.getElementById('addDivButton');
    container.appendChild(button);
}

// Gestion des messages WebSocket
ws.onmessage = function(event) {
    try {
        // Parse le JSON reçu
        console.log(event.data)
        const data = JSON.parse(event.data);

        // Vérifier que la structure des données est correcte
        if (data.teams) {
            // Appeler la fonction pour créer les divs
            createTeamDiv(data.teams);
        } else {
            console.error('Données reçues invalides:', data);
        }
        if (data.bumpers) {
            // Appeler la fonction pour créer les divs
            createBuzzerDiv(data);
        } else {
            console.error('Données reçues invalides:', data);
        }
    } catch (error) {
        console.error('Erreur lors du traitement des données JSON:', error);
    }
};
