#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "includes.h"
#include <ArduinoJson.h>
#include <AsyncTCP.h>
#include <unordered_map>
#include <Ticker.h>

static const char* BUMPER_TAG = "BUMPER_SERVER";
String gamePhase = "STOP"; /* STARTED PAUSED */
int gameStartTimeStamp = 0;
// Déclaration du Ticker global
Ticker gameTimer;

// Flag pour suivre l'état du timer
bool isTimerRunning = false;




void timerCallback() {
    if (getGamePhase() =="START") {
        updateTimer(getGameCurrentTime(), -1);
    }
}

void processButtonPress(const String& bumperID, const char* b_team, int64_t b_time, const char* s_button) {
  ESP_LOGI(BUMPER_TAG, "Button Pressed %s@%s at time %i", s_button, bumperID.c_str(),b_team,b_time);
  if (xSemaphoreTake(questionMutex, pdMS_TO_TICKS(1000)) == pdTRUE) {

    setBumperButton(bumperID.c_str(), s_button);
    setBumperTime(bumperID.c_str(), b_time);
    setBumperStatus(bumperID.c_str(), "PAUSE");

    int64_t teamTime=getTeamTime(b_team);
    ESP_LOGD(BUMPER_TAG, "Actual Team Time %s:%i",b_team,teamTime);
    if (teamTime == 0 or teamTime > b_time) {
      setTeamBumper(b_team,bumperID.c_str());
      setTeamTime(b_team, b_time);
      setTeamStatus(b_team, "PAUSE");
    }
    xSemaphoreGive(questionMutex);
  } else {
        // Le mutex n'a pas pu être obtenu après le timeout
        ESP_LOGI(BUMPER_TAG, "Couldn't obtain mutex in processButtonPress");
    }
    
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

void startGame(const int delay){
  ESP_LOGD(BUMPER_TAG, "STARTING GAME: %i ",delay);
  int newDelay=delay;
  
  resetBumpersTime();
  setGameDelay(newDelay);
  setGameCurrentTime(newDelay);
  setGameTime();
  setGamePhase( "START");
    putMsgToQueue("START",getTeamsAndBumpersJSON().c_str(),true);
  // Démarrer le timer s'il n'est pas déjà en cours
  if (!isTimerRunning) {
      gameTimer.attach(1.0, timerCallback);
      isTimerRunning = true;
  }
}

void updateTimer(const int Time, const int delta ) {
    int newTime = Time + delta;
    if (newTime < 0) { 
        newTime = 0;
    }
    setGameCurrentTime(newTime);
    putMsgToQueue("UPDATE_TIMER", getTeamsAndBumpersJSON().c_str(), false);
    if (newTime == 0) { 
        stopGame();
    }
}


void stopGame(){
  setGameCurrentTime(0);
  setGamePhase( "STOP" );
  putMsgToQueue("UPDATE_TIMER", getTeamsAndBumpersJSON().c_str(), false);
  putMsgToQueue("STOP",getTeamsAndBumpersJSON().c_str(),true);
  // Arrêter le timer
  if (isTimerRunning) {
      gameTimer.detach();
      isTimerRunning = false;
  }
}

void pauseGame(AsyncClient* client) {
  putMsgToQueue("PAUSE",getTeamsAndBumpersJSON().c_str(),true, client);
}

void pauseAllGame(){
  setGamePhase( "PAUSE" );
  putMsgToQueue("UPDATE_TIMER", getTeamsAndBumpersJSON().c_str(), false);
  putMsgToQueue("PAUSE",getTeamsAndBumpersJSON().c_str(),true);
}

void continueGame(){
  setGamePhase( "START");
  putMsgToQueue("UPDATE_TIMER", getTeamsAndBumpersJSON().c_str(), false);
  putMsgToQueue("CONTINUE",getTeamsAndBumpersJSON().c_str(),true);
}

void revealGame() {
    if (isGameStoped()) {
      String msg="\""+getQuestionResponse()+"\"";
      
      putMsgToQueue("REVEAL",msg.c_str(),true);
    }
}

void readyGame(const String question) {
  ESP_LOGI(BUMPER_TAG, "Preparing game with question: %s", question.c_str());

  if (isGameStoped() || isGamePrepare() || isGameReady()) {
    setGamePhase("PREPARE");
    if (question.toInt()>0) {
      setQuestion(question);
    } else {
      setQuestion("");
    }
    resetBumpersTime();
    resetBumpersReady();
    updateTeamsReady();
    putMsgToQueue("UPDATE",getTeamsAndBumpersJSON().c_str(),false);
    putMsgToQueue("PING","{}",false);
  }
}


void RAZscores() {
  ESP_LOGI(BUMPER_TAG, "Resetting Bumpers Scores");
  JsonObject bumpers=getBumpers();
  if (!bumpers.isNull()) {
        for (JsonPair kvp : bumpers) {
            const char* key = kvp.key().c_str();
            ESP_LOGI(BUMPER_TAG, "Resetting Score for %s", key);
            setBumperScore(key, 0);
            setBumperTime(key, -1);
        }
    } else {
        ESP_LOGW(TAG, "Bumpers object is null or invalid");
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
        ESP_LOGW(TAG, "Teams object is null or invalid");
    }
  ESP_LOGI(BUMPER_TAG, "Resetted Scores");
  notifyAll();
  
}

void sendHelloToAll() {
  putMsgToQueue("HELLO", "{  }",true);
}

void sendResetToAll() {
  putMsgToQueue("RESET", "{  }",true);
}

void clearGame(bool notify=true) {
  ESP_LOGI(BUMPER_TAG, "Clearing Game");
  if (LittleFS.exists(saveGameFile)) {
    if (LittleFS.remove(saveGameFile)) {
      ESP_LOGI(BUMPER_TAG, "Save file deleted successfully");
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Unable to delete save file");
    }
  } 
  String dirToRemove="/files";
  deleteDirectory(dirToRemove.c_str());
  ensureDirectoryExists(dirToRemove);
  
  loadJson(GameFile);
  if (notify) {
    sendResetToAll();
    sleep(2);
    sendHelloToAll();
  }
}

void resetServer(bool notify=true) {
  ESP_LOGI(BUMPER_TAG, "Resetting server");
  clearGame(false);

  dirToRemove="/CURRENT";
  deleteDirectory(dirToRemove.c_str());

  loadJson(GameFile);
  if (notify) {
    sendResetToAll();
    sleep(2);
    sendHelloToAll();
  }
}

void rebootServer() {
  ESP_LOGI(BUMPER_TAG, "Rebooting server");
  ESP.restart();
}

void setRemotePage(const String remotePage) {
  if (remotePage==nullptr or remotePage.isEmpty() or remotePage == "null") {
    setGamePage("GAME");
  } else {
    setGamePage(remotePage);
  }
  putMsgToQueue("REMOTE",getTeamsAndBumpersJSON().c_str());
}


void handleHelloAction(const char* bumperID, JsonObject& MSG) {
  updateBumper(bumperID, MSG);
  notifyAll();
}


void handleButtonAction(const char* bumperID, JsonObject& MSG, AsyncClient* c) {
  ESP_LOGD(BUMPER_TAG, "Button pressed: %s", bumperID);
  JsonObject bumper=getBumper(bumperID);
  const char* teamID=bumper["TEAM"];
  const char* s_button = MSG["button"];
  if (teamID != nullptr) {
    processButtonPress(bumperID, teamID, micros(), s_button);
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
            putMsgToQueue("READY", getTeamsAndBumpersJSON().c_str(), true);
        }
    }
}

void parseJSON(const String& data, AsyncClient* c)
{
  String action;
  String bumperID;
  String versionBuzzer;

  JsonDocument receivedData;
  JsonObject MSG;
  JsonObject bumpers = teamsAndBumpers["bumpers"];
  JsonObject teams = teamsAndBumpers["teams"];
  ESP_LOGD(BUMPER_TAG, "parseJSON=%s", data.c_str());
  DeserializationError error = deserializeJson(receivedData, data);
  if (error) {
      ESP_LOGE(BUMPER_TAG, "Failed to parse JSON: %s", error.c_str());
      return;
  }
  JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID = jsonObjData["ID"].as<String>();
  versionBuzzer = jsonObjData["VERSION"].as<String>();
  action = jsonObjData["ACTION"].as<String>();
  MSG = jsonObjData["MSG"];

  ESP_LOGD(BUMPER_TAG, "bumperID=%s version= %s ACTION=%s", bumperID.c_str(), versionBuzzer.c_str(), action.c_str());
    
  switch(hash(action.c_str())) {
    case hash("HELLO"):
      handleHelloAction(bumperID.c_str(), MSG);
      break;
    case hash("BUTTON"):
      handleButtonAction(bumperID.c_str(), MSG, c);
      break;
    case hash("PONG"):
      handlePingResponseAction(bumperID.c_str());
      break;
    default:
      ESP_LOGW(BUMPER_TAG, "Unknown action: %s", action.c_str());
      break;
  }
}

void deleteQuestion(const String ID)
{
  if ( ID.toInt()>0)
  {
    deleteDirectory((questionsPath+"/"+ID).c_str());
    putMsgToQueue("QUESTIONS",getQuestions().c_str());
  }
}