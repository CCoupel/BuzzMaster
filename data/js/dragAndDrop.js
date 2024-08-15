// Fonction pour configurer un élément comme draggable
function setupDraggableElement(element) {
    element.draggable = true;
    element.addEventListener('dragstart', handleDragStart);
}

// Fonction pour gérer le début du drag d'un élément
function handleDragStart(e) {
    e.dataTransfer.setData('text', e.target.id);
}

// Fonction pour gérer l'événement 'dragover' sur une dropzone
function handleDragOver(e, dropzone) {
    e.preventDefault(); 
    dropzone.classList.add('dropzone-hover');
}

// Fonction pour gérer l'événement 'dragleave' sur une dropzone
function handleDragLeave(dropzone) {
    dropzone.classList.remove('dropzone-hover');
}

// Fonction pour gérer l'événement 'drop' sur une dropzone
function handleDrop(e, dropzone, ws, id) {
    e.preventDefault();
    dropzone.classList.remove('dropzone-hover');

    const draggedElementId = e.dataTransfer.getData('text');
    const draggedElement = document.getElementById(draggedElementId);

    if (draggedElement && draggedElement.classList.contains('buzzer')) {
        dropzone.appendChild(draggedElement);

        const updatedBumperMessage = {
            "ACTION": "UPDATE",
            "MSG": {
                "bumpers": {
                    [draggedElementId.replace('buzzer-', '')]: { 
                        "TEAM": id.toString(),
                    }
                }
            }
        };

        ws.send(JSON.stringify(updatedBumperMessage));
        console.log(`Élément ${draggedElementId} mis à jour avec la team ${id}`);
    }
}

// Fonction pour configurer le drag and drop pour une dropzone
export function configureDropzone(dropzone, ws, id) {
    dropzone.addEventListener('dragover', (e) => handleDragOver(e, dropzone));
    dropzone.addEventListener('dragleave', () => handleDragLeave(dropzone));
    dropzone.addEventListener('drop', (e) => handleDrop(e, dropzone, ws, id));
}

// Fonction pour configurer le drag sur un élément
export function configureDragElement(element) {
    setupDraggableElement(element);
}

// Fonction pour initialiser les dropzones
export function initializeDropzones(ws) {
    const buzzerContainer = document.querySelector('.buzzer-container');
    configureDropzone(buzzerContainer, ws, '0');
}
