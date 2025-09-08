#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "includes.h"
#include "messages_to_send.h"
#include "messages_received.h"
#include <ArduinoJson.h>
#include <AsyncTCP.h>
#include <unordered_map>
#include <Ticker.h>

//#include "jsonManager.h"

static const char* BUMPER_TAG = "BUMPER_SERVER";
String gamePhase = "STOP"; /* STARTED PAUSED */
int gameStartTimeStamp = 0;
// Déclaration du Ticker global
Ticker gameTimer;

// Flag pour suivre l'état du timer
bool isTimerRunning = false;

void timerCallback() {
    if (getGamePhase() == "START") {
        updateTimer(getGameCurrentTime(), -1);
    }
}

void processButtonPress(const String& bumperID, const char* b_team, int64_t b_time, String b_button) {
  ESP_LOGI(BUMPER_TAG, "Button Pressed %s@%s at time %lld", b_button, bumperID.c_str(), b_time);
  if (xSemaphoreTake(questionMutex, pdMS_TO_TICKS(1000)) == pdTRUE) {
    int64_t bTime = getBumperTime(bumperID.c_str());
    int64_t teamTime = getTeamTime(b_team);
    ESP_LOGI(BUMPER_TAG, "Button Pressed %s: existing time %i", b_button, bTime);
    ESP_LOGI(BUMPER_TAG, "Button Pressed %s for team %s: existing Team time %lld", b_button, b_team, teamTime);
    if (bTime == 0)
    {
      setBumperButton(bumperID.c_str(), b_button);
      setBumperTime(bumperID.c_str(), b_time);
      setBumperStatus(bumperID.c_str(), "PAUSE");
    }

    
    ESP_LOGD(BUMPER_TAG, "Actual Team Time %s:%lld/%lld", b_team, teamTime, b_time);
    if (teamTime == 0 || teamTime > b_time) {
      setTeamBumper(b_team, bumperID.c_str());
      setTeamTime(b_team, b_time);
      setTeamStatus(b_team, "PAUSE");
      enqueueOutgoingMessage("UPDATE", getTeamsAndBumpersJSON().c_str(), false, nullptr,"");
    }
    else {
      ESP_LOGD(BUMPER_TAG, "Actual Team Time already setup %s:%lld", b_team, teamTime);
    }
    
    xSemaphoreGive(questionMutex);
  } else {
        // Le mutex n'a pas pu être obtenu après le timeout
        ESP_LOGI(BUMPER_TAG, "Couldn't obtain mutex in processButtonPress");
  }
}


void sendQuestions() {
  enqueueOutgoingMessage("QUESTIONS", getQuestions().c_str(), false, nullptr,"");
}

void sendGame() {
  enqueueOutgoingMessage("UPDATE", getGameJSON().c_str(), false, nullptr,"");
}

void sendTeamsAndBumpers() {
  enqueueOutgoingMessage("UPDATE", getTeamsAndBumpersJSON().c_str(), false, nullptr,"");
}

void resetBumpersTime() {
  std::vector<String> keysToRemove = {"STATUS", "BUTTON", "TIME", "TIMESTAMP", "BUMPER"};

  JsonObject bumpers = teamsAndBumpers["bumpers"];
  JsonObject teams = teamsAndBumpers["teams"];
  
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
      JsonObject bumper = kvp.value().as<JsonObject>();
      for (const String& key : keysToRemove) {
        bumper.remove(key);
      }
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Bumper entry is not a JsonObject");
    }
  }
  
  for (JsonPair team : teams) {
    if (team.value().is<JsonObject>()) {
      JsonObject teamData = team.value().as<JsonObject>();
      for (const String& key : keysToRemove) {
        teamData.remove(key);
      }
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Team entry is not a JsonObject");
    }
  }
  
  timeRef = 0;
  for (const auto& pair : timeRefTeam) {
    timeRefTeam[pair.first] = 0;
  }
}

void startGame(const int delay) {
  ESP_LOGD(BUMPER_TAG, "STARTING GAME: %i ", delay);
  int newDelay = delay;
  
  resetBumpersTime();
  setGameDelay(newDelay);
  setGameCurrentTime(newDelay);
  setGameTime();
  setGamePhase("START");
  setQuestionStatus("STARTED");
  enqueueOutgoingMessage("START", getGameJSON().c_str(), true, nullptr,"");
  
  // Démarrer le timer s'il n'est pas déjà en cours
  if (!isTimerRunning) {
      gameTimer.attach(1.0, timerCallback);
      isTimerRunning = true;
  }
}

void updateTimer(const int Time, const int delta) {
    int newTime = Time + delta;
    if (newTime < 0) { 
        newTime = 0;
    }
    setGameCurrentTime(newTime);
    enqueueOutgoingMessage("UPDATE_TIMER", getGameJSON().c_str(), false, nullptr,"");
    if (newTime == 0) { 
        stopGame();
    }
}

void stopGame() {
  setGameCurrentTime(0);
  setGamePhase("STOP");
  setQuestionStatus("STOPPED");
  enqueueOutgoingMessage("STOP", getGameJSON().c_str(), true, nullptr,"");
  
  // Arrêter le timer
  if (isTimerRunning) {
      gameTimer.detach();
      isTimerRunning = false;
  }
}

void pauseGame(AsyncClient* client) {
  enqueueOutgoingMessage("PAUSE", getGameJSON().c_str(), true, client,"");
}

void pauseAllGame() {
  setGamePhase("PAUSE");
  setQuestionStatus("PAUSED");
  enqueueOutgoingMessage("PAUSE", getGameJSON().c_str(), true, nullptr,"");
}

void continueGame() {
  setGamePhase("START");
  setQuestionStatus("STARTED");
  enqueueOutgoingMessage("CONTINUE", getGameJSON().c_str(), true, nullptr,"");
}

void revealGame() {
    if (isGameStopped()) {
      String msg = "\"" + getQuestionResponse() + "\"";
      setQuestionStatus("REVEALED");
      enqueueOutgoingMessage("REVEAL", msg.c_str(), true, nullptr,"");
    }
}

void updateScore(const String bumperID, const int points) {
  ESP_LOGD(BUMPER_TAG, "Bumper update %s %i", bumperID.c_str(), points);
  int bscore=updateBumperScore(bumperID.c_str(), points);
  int tscore=updateBumperTeamScore(bumperID.c_str(), points);
  String update="\"POINTS\": {";
  update+="\"bumperId\": \""+bumperID+"\"";
  update+=", \"teamId\": \""+String(getBumperTeam(bumperID.c_str()))+"\"";
  update+=", \"points\": "+String(points);
  update+=", \"scoreBumper\": "+String(bscore);
  update+=", \"scoreTeam\": "+String(tscore);
  
  update+="}";
  enqueueOutgoingMessage("UPDATE", getTeamsAndBumpersJSON().c_str(), false, nullptr, update.c_str());
}

void readyGame(const String question) {
  ESP_LOGI(BUMPER_TAG, "Preparing game with question: %s", question.c_str());


  if (isGameStopped() || isGamePrepare() || isGameReady()) {
 
    setGamePhase("PREPARE");
    if (question.toInt() > 0) {
      setCurrentQuestion(question);
      setQuestionStatus("AVAILABLE");
    } else {
      setCurrentQuestion("");
    }
    resetBumpersTime();
    resetBumpersReady();
    updateTeamsReady();
    sendTeamsAndBumpers();

    enqueueOutgoingMessage("PING", "{}", false, nullptr,"");
  }
}

void RAZscores() {
  ESP_LOGI(BUMPER_TAG, "Resetting Bumpers Scores");
  JsonObject bumpers = getBumpers();
  if (!bumpers.isNull()) {
        for (JsonPair kvp : bumpers) {
            const char* key = kvp.key().c_str();
            ESP_LOGI(BUMPER_TAG, "Resetting Score for %s", key);
            setBumperScore(key, 0);
            setBumperTime(key, -1);
        }
    } else {
        ESP_LOGW(BUMPER_TAG, "Bumpers object is null or invalid");
    }
  
  ESP_LOGI(BUMPER_TAG, "Resetting Teams Scores");
  JsonObject teams = getTeams();
    if (!teams.isNull()) {
        for (JsonPair kvp : teams) {
            const char* key = kvp.key().c_str();
            ESP_LOGI(BUMPER_TAG, "Resetting Score for %s", key);
            setTeamScore(key, 0);
            setTeamTime(key, -1);
        }
    } else {
        ESP_LOGW(BUMPER_TAG, "Teams object is null or invalid");
    }
  ESP_LOGI(BUMPER_TAG, "Resetted Scores");
  notifyAll();
}

void sendHelloToAll() {
  enqueueOutgoingMessage("HELLO", "{  }", true, nullptr,"");
}

void sendResetToAll() {
  enqueueOutgoingMessage("RESET", "{  }", true, nullptr,"");
}

void clearGame(bool notify=true) {
  ESP_LOGI(BUMPER_TAG, "clear Game");
  if (LittleFS.exists(saveGameFile)) {
    if (LittleFS.remove(saveGameFile)) {
      ESP_LOGI(BUMPER_TAG, "Save file deleted successfully");
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Unable to delete save file");
    }
  } 
  String dirToRemove = "/files";
  deleteDirectory(dirToRemove.c_str());
  ensureDirectoryExists(dirToRemove);
  dirToRemove = "/temp_chunks";
  deleteDirectory(dirToRemove.c_str());
  dirToRemove = "/temp_stream";
  deleteDirectory(dirToRemove.c_str());
  loadJson(GameFile);
  if (notify) {
    sendResetToAll();
    sleep(2);
    sendHelloToAll();
    sendQuestions();
  }
}

void resetServer() {
  String dirToRemove = "/CURRENT";
  deleteDirectory(dirToRemove.c_str());

  clearGame();
}

void rebootServer() {
  ESP_LOGI(BUMPER_TAG, "Rebooting server");
  setLedColor(255,16,16,true);
  ESP.restart();
}

void setRemotePage(const String remotePage) {
  if (remotePage == nullptr || remotePage.isEmpty() || remotePage == "null") {
    setGamePage("GAME");
  } else {
    setGamePage(remotePage);
  }
  enqueueOutgoingMessage("REMOTE", getTeamsAndBumpersJSON().c_str(), false, nullptr,"");
}

void handleHelloAction(const char* bumperID, JsonObject& MSG) {
  updateBumper(bumperID, MSG);
  notifyAll();
}

void handleButtonAction(const char* bumperID, JsonObject& MSG, AsyncClient* c) {
  ESP_LOGE(BUMPER_TAG, "Button pressed: %s", bumperID);
  JsonObject bumper = getBumper(bumperID);
  const char* teamID = bumper["TEAM"];
  String b_button = MSG["button"];
  if (teamID != nullptr) {
    processButtonPress(bumperID, teamID, micros(), b_button);
    pauseGame(c);
  }
}

void handlePingResponseAction(const char* bumperID) {
    ESP_LOGI(BUMPER_TAG, "Bumper PONG received from: %s", bumperID);
    if (isGamePrepare()) {
        setBumperReady(bumperID);
        updateTeamsReady();
        notifyAll();
        
        // Check if all teams are ready to potentially transition to READY state
        if (areAllTeamsReady()) {
            ESP_LOGI(BUMPER_TAG, "All teams are ready to start");
            setGamePhase("READY");
            enqueueOutgoingMessage("READY", getTeamsAndBumpersJSON().c_str(), true, nullptr,"");
        }
    }
}

void deleteQuestion(const String ID) {
  if (ID.toInt() > 0) {
    deleteDirectory((questionsPath + "/" + ID).c_str());
    sendQuestions();
  }
}