import { getTeams, getBumpers, addBumperToTeam, addNewTeam, removeBumperFromTeam, deleteTeam } from './configSPA.js';
import { createTeamDiv } from './teamSPA.js';
import { createBuzzerDiv } from './buzzerSPA.js';

let autoScrollInterval = null;
let lastMouseY = 0;

function trackMouseY(e) {
    lastMouseY = e.clientY;
}

function setupDraggableElement(element) {
    element.draggable = true;

    element.addEventListener('dragstart', (e) => {
        handleDragStart(e);
        window.addEventListener('dragover', trackMouseY);
        startAutoScroll();
    });

    element.addEventListener('dragend', () => {
        window.removeEventListener('dragover', trackMouseY);
        stopAutoScroll();
    });
}

function handleDragStart(e) {
    e.dataTransfer.setData('text', e.target.id);
};


function handleDragOver(e) {
    const dropzone = e.currentTarget;
    e.preventDefault();
    e.stopPropagation();
};

function handleDragEnter(e) {
    const dropzone = e.currentTarget;
    e.stopPropagation();

    // Désactiver toutes les zones de drop
    document.querySelectorAll('.dropzone-hover').forEach(el => {
        el.classList.remove('dropzone-hover');
    });
    
    // Activer la zone actuelle
    dropzone.classList.add('dropzone-hover');

    // Si c'est une équipe individuelle, désactiver la zone générale des teams
    if (dropzone.closest('.dynamic-div')) {
        const teamContainer = document.querySelector('.team-container');
        if (teamContainer) {
            teamContainer.classList.remove('dropzone-hover');
        }
    }
};

function handleDragLeave(e) {
    const dropzone = e.currentTarget;
    const relatedTarget = e.relatedTarget;
    
    // Vérifier si on quitte réellement la zone de drop (et pas juste un enfant)
    if (!dropzone.contains(relatedTarget)) {
        dropzone.classList.remove('dropzone-hover');
        
        // Si on quitte une équipe individuelle, réactiver la zone générale des teams
        if (dropzone.closest('.dynamic-div')) {
            const teamContainer = document.querySelector('.team-container');
            if (teamContainer && !teamContainer.contains(relatedTarget)) {
                teamContainer.classList.add('dropzone-hover');
            }
        }
    }
};

function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();

    const draggedElementId = e.dataTransfer.getData('text');
    const draggedElement = document.getElementById(draggedElementId);
    const dropzone = e.currentTarget;

    if (draggedElement.classList.contains('buzzer')) {
        if (dropzone.classList.contains('dropzone') && dropzone.closest('.dynamic-div')) {
            buzzerDropInTeam(draggedElement, dropzone);
        } else if (dropzone.classList.contains('team-container')) {
            buzzerDropInTeams(draggedElement, dropzone);
        } else if (dropzone.classList.contains('buzzer-container')) {
            buzzerDropInBuzzers(draggedElement, dropzone);
        }
    } else if (draggedElement.classList.contains('dynamic-div')) {
        if (dropzone.classList.contains('buzzer-container')) {
            teamDropInBuzzers(draggedElement, dropzone);
        } else {
            console.log('Dropping a team into another team is not allowed');
            showInvalidDropFeedback(dropzone);
        }
    }

    // Retirer toutes les classes actives après le drop
    document.querySelectorAll('.dropzone-hover').forEach(el => {
        el.classList.remove('dropzone-hover');
    });
};

function startAutoScroll() {
    if (autoScrollInterval) return;

    autoScrollInterval = setInterval(() => {
        const threshold = 100; // marge haute/basse
        const scrollSpeed = 25;

        if (lastMouseY < threshold) {
            window.scrollBy(0, -scrollSpeed);
        } else if (lastMouseY > window.innerHeight - threshold) {
            window.scrollBy(0, scrollSpeed);
        }
    }, 20);
}

function stopAutoScroll() {
    clearInterval(autoScrollInterval);
    autoScrollInterval = null;
}

function showInvalidDropFeedback(element) {
    element.classList.add('invalid-drop');
    setTimeout(() => {
        element.classList.remove('invalid-drop');
    }, 500);
};


function removeAllDropzoneHoverClasses() {
    document.querySelectorAll('.dropzone-hover').forEach(element => {
        element.classList.remove('dropzone-hover');
    });
};

function buzzerDropInTeams(buzzerElement, teamsContainer) {
    console.log('buzzerDropInTeams called:', {
        buzzerId: buzzerElement.id,
        teamsContainerId: teamsContainer.id
    });

    const bumperMac = buzzerElement.id.replace('buzzer-', '');
    const playerName = buzzerElement.querySelector('.buzzer-name')?.textContent || 'Nouvelle équipe';
    const newTeamId = playerName.toUpperCase();

    console.log('Creating new team:', {
        bumperMac,
        playerName,
        newTeamId
    });

    addNewTeam(newTeamId);
    addBumperToTeam(bumperMac, newTeamId);

    console.log('Team created and buzzer added:', {
        newTeamId,
        bumperMac
    });

    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());
};

function buzzerDropInTeam(buzzerElement, teamDropzone) {
    console.log('buzzerDropInTeam called:', {
        buzzerId: buzzerElement.id,
        teamDropzoneId: teamDropzone.id
    });

    const bumperMac = buzzerElement.id.replace('buzzer-', '');
    const teamId = teamDropzone.closest('.dynamic-div').id;

    console.log('Adding buzzer to team:', {
        bumperMac,
        teamId
    });

    addBumperToTeam(bumperMac, teamId);
    teamDropzone.appendChild(buzzerElement);

    console.log('Buzzer added to team');
    
    // Mettre à jour l'affichage
    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());
};

function buzzerDropInBuzzers(buzzerElement, buzzerContainer) {
    console.log('buzzerDropInBuzzers called:', {
        buzzerId: buzzerElement.id,
        buzzerContainerId: buzzerContainer.id
    });

    const bumperMac = buzzerElement.id.replace('buzzer-', '');

    console.log('Removing buzzer from team:', bumperMac);

    removeBumperFromTeam(bumperMac);
    buzzerContainer.appendChild(buzzerElement);

    console.log('Buzzer removed from team and added to buzzer container');
};

function teamDropInBuzzers(teamElement, buzzerContainer) {
    console.log('teamDropInBuzzers called:', {
        teamId: teamElement.id,
        buzzerContainerId: buzzerContainer.id
    });

    const teamId = teamElement.id;
    const buzzers = Array.from(teamElement.querySelectorAll('.buzzer'));

    console.log('Removing buzzers from team:', {
        teamId,
        buzzerCount: buzzers.length
    });

    buzzers.forEach(buzzer => {
        const bumperMac = buzzer.id.replace('buzzer-', '');
        removeBumperFromTeam(bumperMac);
        buzzerContainer.appendChild(buzzer);
        console.log('Buzzer removed from team:', bumperMac);
    });

    deleteTeam(teamId);
    teamElement.remove();

    console.log('Team deleted:', teamId);

    createTeamDiv(getTeams());
    createBuzzerDiv(getBumpers());

    console.log('Team and buzzer views updated');
};

export function configureDropzone(dropzone) {
    dropzone.addEventListener('dragover', handleDragOver);
    dropzone.addEventListener('dragenter', handleDragEnter);
    dropzone.addEventListener('dragleave', handleDragLeave);
    dropzone.addEventListener('drop', handleDrop);
};

export function configureDragElement(element) {
    setupDraggableElement(element);
};

export function initializeDropzones() {
    const buzzerContainer = document.querySelector('.buzzer-container');
    const teamContainer = document.querySelector('.team-container');

    if (buzzerContainer) configureDropzone(buzzerContainer);
    if (teamContainer) configureDropzone(teamContainer);
};