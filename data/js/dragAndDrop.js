// Fonction pour configurer le drag and drop pour une dropzone
export function configureDropzone(dropzone, ws, id) {
    dropzone.addEventListener('dragover', (e) => {
        e.preventDefault(); 
        dropzone.classList.add('dropzone-hover');
    });

    dropzone.addEventListener('dragleave', () => {
        dropzone.classList.remove('dropzone-hover');
    });

    dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('dropzone-hover');

        const draggedElementId = e.dataTransfer.getData('text');
        const draggedElement = document.getElementById(draggedElementId);

        if (draggedElement && draggedElement.classList.contains('buzzer')) {
            dropzone.appendChild(draggedElement);

            const updatedBuzzerMessage = {
                "bumpers": {
                    [draggedElementId.replace('buzzer-', '')]: { // Remplace 'buzzer-' dans l'ID pour obtenir l'ID original du bumper
                        "TEAM": id,
                    }
                }
            };

            ws.send(JSON.stringify(updatedBuzzerMessage));

            console.log(`Élément ${draggedElementId} mis à jour avec la team ${id}`);
        }
    });
}

// Fonction pour configurer le drag sur un élément
export function configureDragElement(element) {
    element.draggable = true;
    element.addEventListener('dragstart', (e) => {
        e.dataTransfer.setData('text', e.target.id);
    });
}
