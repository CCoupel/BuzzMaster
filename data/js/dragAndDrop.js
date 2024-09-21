function setupDraggableElement(element) {
    element.draggable = true;
    element.addEventListener('dragstart', handleDragStart);
}

function handleDragStart(e) {
    e.dataTransfer.setData('text', e.target.id);
}

function handleDragOver(e, dropzone) {
    e.preventDefault(); 
    dropzone.classList.add('dropzone-hover');
}

function handleDragLeave(dropzone) {
    dropzone.classList.remove('dropzone-hover');
}

function handleDrop(e, dropzone, ws, id, onDropCallback) {
    e.preventDefault();
    dropzone.classList.remove('dropzone-hover');

    const draggedElementId = e.dataTransfer.getData('text');
    const draggedElement = document.getElementById(draggedElementId);

    if (draggedElement && draggedElement.classList.contains('buzzer')) {
        const bumperMac = draggedElementId.replace('buzzer-', '');
        
        if (onDropCallback) {
            console.log("Appel du callback de drop avec:", bumperMac);
            onDropCallback(bumperMac);
        } else {
            dropzone.appendChild(draggedElement);

            const updatedBumperMessage = {
                "ACTION": "UPDATE",
                "MSG": {
                    "bumpers": {
                        [bumperMac]: { 
                            "TEAM": id,
                        }
                    }
                }
            };

            ws.send(JSON.stringify(updatedBumperMessage));
            console.log(`Élément ${draggedElementId} mis à jour avec la team ${id}`);
        }
    }
}

export function configureDropzone(dropzone, ws, id, onDropCallback) {
    dropzone.addEventListener('dragover', (e) => handleDragOver(e, dropzone));
    dropzone.addEventListener('dragleave', () => handleDragLeave(dropzone));
    dropzone.addEventListener('drop', (e) => handleDrop(e, dropzone, ws, id, onDropCallback));
}

export function configureDragElement(element) {
    setupDraggableElement(element);
}

export function initializeDropzones(ws) {
    const buzzerContainer = document.querySelector('.buzzer-container');
    configureDropzone(buzzerContainer, ws, '');
}