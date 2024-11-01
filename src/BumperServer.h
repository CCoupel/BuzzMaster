#pragma once

#include <ArduinoJson.h>
#include <AsyncTCP.h>
#include <esp_log.h>
#include <unordered_map>

static const char* BUMPER_TAG = "BUMPER_SERVER";
String gamePhase = "STOP"; /* STARTED PAUSED */
int gameStartTimeStamp = 0;

void processButtonPress(const String& bumperID, const char* b_team, int64_t b_time, int b_button) {
  ESP_LOGI(BUMPER_TAG, "Button Pressed %i at time %i", b_button, b_time);

  setBumperButton(bumperID.c_str(), b_button);
  setBumperTime(bumperID.c_str(), b_time);
  setBumperStatus(bumperID.c_str(), "PAUSE");

  int64_t teamTime=getTeamTime(b_team);
  ESP_LOGD(BUMPER_TAG, "Actual Team Time %s:%i",b_team,teamTime);
  if (teamTime == 0 or teamTime > b_time) {
    setTeamBumper(b_team,bumperID.c_str());
    setTeamTime(b_team, b_time);
    setTeamStatus(b_team, "PAUSE");
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
  resetBumpersTime();
  setGameDelay(delay);
  setGameCurrentTime(delay);
  setGameTime();
  GameStarted=true;
  setGamePhase( "START");
    putMsgToQueue("START",getTeamsAndBumpersJSON().c_str(),true);
}

void stopGame(){
  GameStarted=false;
  setGameCurrentTime(0);
  setGamePhase( "STOP" );
    putMsgToQueue("STOP",getTeamsAndBumpersJSON().c_str(),true);
}

void pauseGame(AsyncClient* client) {
  putMsgToQueue("PAUSE",getTeamsAndBumpersJSON().c_str(),true, client);
}

void pauseAllGame(const int currentTime=0){
  setGamePhase( "PAUSE" );
  setGameCurrentTime(currentTime);
    putMsgToQueue("PAUSE",getTeamsAndBumpersJSON().c_str(),true);
}

void continueGame(){
  GameStarted=true;
  setGamePhase( "START");
      putMsgToQueue("CONTINUE",getTeamsAndBumpersJSON().c_str(),true);
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

void resetServer() {
  ESP_LOGI(BUMPER_TAG, "Resetting server");
  if (LittleFS.exists(saveGameFile)) {
    if (LittleFS.remove(saveGameFile)) {
      ESP_LOGI(BUMPER_TAG, "Save file deleted successfully");
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Unable to delete save file");
    }
  } 
  loadJson(GameFile);
  sendResetToAll();
  sleep(2);
  sendHelloToAll();
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
  ESP_LOGE(BUMPER_TAG, "Button pressed: %s", bumperID);
  JsonObject bumper=getBumper(bumperID);
  const char* teamID=bumper["TEAM"];
  int b_button = MSG["button"];
  if (teamID != nullptr) {
    processButtonPress(bumperID, teamID, micros(), b_button);
    pauseGame(c);
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
    case hash("PING"):
      // Handle PING action if needed
      break;
    default:
      ESP_LOGW(BUMPER_TAG, "Unknown action: %s", action.c_str());
      break;
  }
}

