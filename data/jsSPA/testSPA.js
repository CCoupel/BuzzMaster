import { routes } from './routes.js';
import { questionsPage } from './questionsSPA.js';
import { configPage } from './configSPA.js';
import { scorePage } from './scoreSPA.js';
import { teamGamePage } from './teamGameSPA.js';

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
};

function navbarColor(location) {
    // Supprime la classe de tous les boutons de navigation
    document.querySelectorAll('.span-navbar').forEach(btn => {
        btn.classList.remove('currentLocation');
    });

    // Ajoute la classe uniquement au bouton correspondant
    const spanNavbar = document.getElementById(location);
    if (!spanNavbar) return;
    spanNavbar.classList.add('currentLocation');
}

// Ajouter des événements spécifiques à chaque page
async function attachEvents(hash) {
    navbarColor(hash.substring(1));

    if (hash === "#questions") {
        questionsPage(); 
    }

    if (hash === "#config") {
        configPage();
    }

    if (hash === "#score") {
        scorePage();
    }

    if (hash === "#teamGame") {
        teamGamePage();
    }
};

// Écouter les changements de hash
window.addEventListener('hashchange', navigate);

// Charger la page initiale
navigate();
