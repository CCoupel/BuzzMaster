document.addEventListener('DOMContentLoaded', function() {
    fetch('navbar.html')
        .then(response => response.text())
        .then(data => {
            // Cibler l'élément 'header' dans ton document
            const header = document.querySelector('header');
            if (header) {
                header.insertAdjacentHTML('afterbegin', data);
            } else {
                document.body.insertAdjacentHTML('afterbegin', data);
            }
        })
        .catch(error => console.error('Error loading navbar:', error));
});