import { routes } from './routes.js';

// Fonction pour mettre à jour le contenu de la page
function navigate() {
    const appDiv = document.getElementById('main');
    const hash = window.location.hash || '#config'; // Par défaut, aller à #home
    const route = routes[hash];

    if (route) {
        appDiv.innerHTML = route(); // Appeler la fonction associée à la route
        attachEvents(hash); // Ajouter des événements spécifiques à la page
    } else {
        appDiv.innerHTML = '<h1>404 - Page non trouvée</h1>';
    }
}

// Ajouter des événements spécifiques à chaque page
function attachEvents(hash) {
  
}

// Écouter les changements de hash
window.addEventListener('hashchange', navigate);

// Charger la page initiale
navigate();
