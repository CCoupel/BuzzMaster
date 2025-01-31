export const routes = {
    '#teamGame': () => `
        <link rel="stylesheet" href="../css/teamGame.css">
        <div id="game-container" class="game-container"></div>
        <div id="question-container-admin"></div>
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
            <div class="question-form-div">
                <form id="question-form">
                    <label for="number">Numéro de la question :</label>
                    <input name="number" type="number" min="1">

                    <label for="points">Nombre de points :</label>
                    <input name="points" type="number" min="1" value="1">

                    <label for="time">Temps (en secondes) :</label>
                    <input name="time" type="number" min="1" value="40">

                    <label for="question">Question :</label>
                    <input name="question" type="text">

                    <label for="answer">Réponse :</label>
                    <input name="answer" type="text">

                    <input name="file" type="file">

                    <button type="submit">Valider la question</button>
                </form>
            </div>
            <div id="questions-container">
                <div id="loader" style="display:none;">
                    <p>Chargement...</p>
                </div>
            </div>
        </div>
    `,
};
