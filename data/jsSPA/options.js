let configData = {}; // pour conserver les données JSON

async function fetchConfig() {
  try {
    const response = await fetch('http://buzzcontrol.local/config.json');
    if (!response.ok) throw new Error(`Erreur HTTP : ${response.status}`);
    configData = await response.json();
    displayConfig(configData);
  } catch (error) {
    console.error('Erreur de chargement :', error);
    document.getElementById('configContent').innerHTML = "<p>Erreur de chargement.</p>";
  }
}

function displayConfig(data) {
  const container = document.getElementById('configContent');
  container.innerHTML = '';

  for (const section in data) {
    const h2 = document.createElement('h2');
    h2.textContent = section;
    container.appendChild(h2);

    for (const key in data[section]) {
      const p = document.createElement('p');
      p.dataset.section = section;
      p.dataset.key = key;
      p.textContent = `${key}: ${data[section][key]}`;
      container.appendChild(p);
    }
  }
}

function enableEditMode() {
  const container = document.getElementById('configContent');
  const form = document.createElement('form');
  form.id = "editForm";

  for (const section in configData) {
    const h2 = document.createElement('h2');
    h2.textContent = section;
    form.appendChild(h2);

    for (const key in configData[section]) {
      const label = document.createElement('label');
      label.textContent = key + ': ';
      label.style.display = 'block';

      const input = document.createElement('input');
      input.type = 'text';
      input.name = `${section}.${key}`;
      input.value = configData[section][key];
      input.style.marginBottom = '8px';
      input.style.width = '100%';

      label.appendChild(input);
      form.appendChild(label);
    }
  }

  const submitBtn = document.createElement('button');
  submitBtn.textContent = 'Enregistrer';
  submitBtn.type = 'submit';
  submitBtn.style.marginTop = '20px';

  form.appendChild(submitBtn);
  container.innerHTML = '';
  container.appendChild(form);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();

    // Reconstruire les données JSON
    const formData = new FormData(form);
    const updatedConfig = {};

    for (const [fullKey, value] of formData.entries()) {
      const [section, key] = fullKey.split('.');
      if (!updatedConfig[section]) updatedConfig[section] = {};
      updatedConfig[section][key] = isNaN(value) ? value : Number(value);
    }

    try {
      const response = await fetch('http://buzzcontrol.local/config.json', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(updatedConfig)
      });

      if (!response.ok) throw new Error('Erreur lors de l’envoi');
      configData = updatedConfig;
      displayConfig(updatedConfig);
    } catch (error) {
      alert('Erreur lors de l’enregistrement : ' + error.message);
    }
  });
}

function callApi(url) {
  fetch(url, { method: "GET" })
    .then(response => {
      if (!response.ok) throw new Error("HTTP error " + response.status);
      console.log(`Call to ${url} successful`);
    })
    .catch(error => console.error(`Error calling ${url}:`, error));
}



document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('editButton').addEventListener('click', enableEditMode);
  fetchConfig();

 document.getElementById("rebootButton").addEventListener("click", () => {
    callApi("http://buzzcontrol.local/reboot");
  });

  document.getElementById("resetButton").addEventListener("click", () => {
    callApi("http://buzzcontrol.local/reset");
  });

  document.getElementById("updateButton").addEventListener("click", () => {
    callApi("http://buzzcontrol.local/update");
  });

  document.getElementById("clearGameButton").addEventListener("click", () => {
    callApi("http://buzzcontrol.local/clearGame");
  });

    document.getElementById("clearBuzzersButton").addEventListener("click", () => {
    callApi("http://buzzcontrol.local/cleaBuzzers");
  });

  document.getElementById('listFilesButton').addEventListener('click', () => {
    window.open('http://buzzcontrol.local/listFiles', '_blank');
  });

  document.getElementById('listGameButton').addEventListener('click', () => {
    window.open('http://buzzcontrol.local/listGame', '_blank');
  });

  document.getElementById('backupButton').addEventListener('click', () => {
    window.open('http://buzzcontrol.local/fs-backup', '_blank');
  });

  document.getElementById('gameBackupButtonButton').addEventListener('click', () => {
    window.open('http://buzzcontrol.local/game-backup', '_blank');
  });  
});