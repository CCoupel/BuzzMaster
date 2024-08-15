// Connectez-vous au serveur WebSocket
  //const loc = window.location;
  const loc = "buzzcontrol.local";
  //const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'; // Utilisez 'wss' si la page est chargée via HTTPS
  const wsProtocol = "ws:"
  //const wsUrl = `${wsProtocol}//${loc.host}/ws`; // Utilise le même hôte et protocole que la page
  const wsUrl = `${wsProtocol}//${loc}/ws`; // Utilise le même hôte et protocole que la page

  // Initialiser l'objet WebSocket
  export const ws = new WebSocket(wsUrl);


  // Envoyer un message au serveur
  ws.onopen = function() {
      console.log('Connecté au serveur WebSocket');
      ws.send('{ "ACTION": "HELLO", "MSG": "Salut, serveur WebSocket !"}');
  };

  // Événement de fermeture de la connexion
  ws.onclose = function() {
      console.log('Connexion fermée');
  };
