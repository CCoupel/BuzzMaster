import { getTeams, getBumpers, updateTeams, updateBumpers } from './main.js';

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

    if (draggedElement) {
        if (draggedElement.classList.contains('buzzer')) {
            const bumperMac = draggedElementId.replace('buzzer-', '');
            
            if (onDropCallback) {
                console.log("Appel du callback de drop avec:", bumperMac);
                onDropCallback(bumperMac);
            } else {
                dropzone.appendChild(draggedElement);

                const currentBumpers = getBumpers();
                const updatedBumpers = {
                    ...currentBumpers,
                    [bumperMac]: { ...currentBumpers[bumperMac], TEAM: id }
                };

                const updateMessage = {
                    "ACTION": "FULL",
                    "MSG": {
                        "bumpers": updatedBumpers,
                        "teams": getTeams()
                    }
                };

                ws.send(JSON.stringify(updateMessage));
                updateBumpers(updatedBumpers);
                console.log(`Élément ${draggedElementId} mis à jour avec la team ${id}`);
            }
        } else if (draggedElement.classList.contains('dynamic-div')) {
            const teamId = draggedElementId;
            const buzzers = Array.from(draggedElement.querySelectorAll('.buzzer'));
            
            const currentBumpers = getBumpers();
            const updatedBumpers = { ...currentBumpers };
            
            buzzers.forEach(buzzer => {
                dropzone.appendChild(buzzer);
                const bumperMac = buzzer.id.replace('buzzer-', '');
                updatedBumpers[bumperMac] = { ...updatedBumpers[bumperMac], TEAM: "" };
            });

            const currentTeams = getTeams();
            const updatedTeams = Object.entries(currentTeams).reduce((acc, [key, value]) => {
                if (key !== teamId) {
                    acc[key] = value;
                }
                return acc;
            }, {});

            const updateMessage = {
                "ACTION": "FULL",
                "MSG": {
                    "bumpers": updatedBumpers,
                    "teams": updatedTeams
                }
            };
            ws.send(JSON.stringify(updateMessage));

            updateBumpers(updatedBumpers);
            updateTeams(updatedTeams);

            draggedElement.remove();

            console.log(`Équipe ${teamId} supprimée et buzzers détachés`);
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