export const routes = {
    '#welcome': () => `
    <div id="welcome-page"> 

    </div>
    `,

    '#teamGame': () => `
        <link rel="stylesheet" href="../css/teamGame.css">
        <div id="teamGame-page"> 
            <div id="questions-select-list"></div>
            <div id="admin-container">
                <div id="game-container" class="game-container"></div>
                <div id="question-container-admin"></div>
            </div>
        </div>
    `,

    '#score': () => `
        <link rel="stylesheet" href="../css/score.css">
        <div id="score-container">
            <div class="scoreboard">
                <h2>Classement des Équipes</h2>
                <table id="team-scores">
                    <thead>
                        <tr>
                            <th>Rang</th>
                            <th>Équipe</th>
                            <th>Score Total</th>
                        </tr>
                    </thead>
                    <tbody>
                    </tbody>
                </table>
            </div>

            <div class="scoreboard">
                <h2>Classement des Joueurs</h2>
                <table id="player-scores">
                    <thead>
                        <tr>
                            <th>Rang</th>
                            <th>Joueur</th>
                            <th>Équipe</th>
                            <th>Score</th>
                        </tr>
                    </thead>
                    <tbody>
                    </tbody>
                </table>
            </div>
        </div>
    `,

    '#config': () => `
        <link rel="stylesheet" href="../css/config.css">
        <div class="main-config-container">
            <div class="buzzer-container"></div>
            <div class="team-container"></div>
        </div>
    `,

    '#questions': () => `
        <link rel="stylesheet" href="../css/questions.css">
        <div class="main-container-questions">
            <div id="questions-container">
                <div id="loader" style="display:none;">
                    <p>Chargement...</p>
                </div>
            </div>
            <div id ="flex-container">
                <div id ="form-container">
                    <div id="background-container">
                        <form method="post" enctype="multipart/form-data" id="background-form" action="http://buzzcontrol.local/background">
                            <div>
                                <label for="file">Image de fond :</label>
                                <input type="file" id="background" name="background"/>
                                <button type="submit">Envoyer</button>
                            </div> 
                        </form>
                        <progress id="progressBarBackground" value="0" max="100"></progress>
                    </div> 
                    <div class="question-form-div">
                        <form id="question-form">
                            <label for="number">Numéro de la question :</label>
                            <input name="number" type="number" min="1">

                            <label for="points">Nombre de points :</label>
                            <input name="points" type="number" min="1" value="1" required="required">

                            <label for="time">Temps (en secondes) :</label>
                            <input name="time" type="number" min="1" value="40" required="required">

                            <label for="question">Question :</label>
                            <input name="question" type="text" required="required">

                            <label for="answer">Réponse :</label>
                            <input name="answer" type="text" required="required">

                            <input name="file" type="file">

                            <button type="submit">Valider la question</button>
                        </form>
                        <progress id="progressBarQuestion" value="0" max="100"></progress>
                    </div>
                </div>
                <div id="file-storage-container">
                    <div id="file-storage">
                        <p id="file-storage-text"></p>
                        <progress id="file" max="100" value=""></progress>
                    </div>
                </div>
            </div>
        </div>
        <div id="custom-confirm" class="modal-hidden">
            <div class="modal-content">
                <p id="confirm-message"></p>
                <div class="modal-buttons">
                <button id="confirm-yes">Oui</button>
                <button id="confirm-no">Non</button>
                </div>
            </div>
        </div>
    `,
};
