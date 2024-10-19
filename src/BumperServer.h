#pragma once

#include <ArduinoJson.h>
#include <AsyncTCP.h>
#include <esp_log.h>
#include <unordered_map>

static const char* BUMPER_TAG = "BUMPER_SERVER";
String gamePhase = "STOPPED"; /* STARTED PAUSED */
int gameStartTimeStamp = 0;

void processButtonPress(const String& bumperID, const char* b_team, int64_t b_time, int b_button) {
  ESP_LOGI(BUMPER_TAG, "Button Pressed %i", b_button);
  /*static int64_t timeRef = 0;
  static std::unordered_map<String, int64_t> timeRefTeam;

  if (timeRef == 0) {
    timeRef = b_time;
  }
  int64_t b_delay = b_time - timeRef;

  if (timeRefTeam[b_team] == 0) {
    timeRefTeam[b_team] = b_time;
  }

  int64_t b_delayTeam = b_time - timeRefTeam[b_team];
  */
  //JsonObject b = getBumper(bumperID.c_str());
  //b["TIME"] = b_time;
  //b["BUTTON"] = b_button;
  setBumperButton(bumperID.c_str(), b_button);
  setBumperTime(bumperID.c_str(), b_time);
  setTeamBumper(b_team,bumperID.c_str());
  setTeamTime(b_team, b_time);
  //b["DELAY"] = b_delay;
  //b["DELAY_TEAM"] = b_delayTeam;
  //b["STATUS"] = "PAUSE";
  setBumperStatus(bumperID.c_str(), "PAUSE");
  //setBumper(bumperID.c_str(), b);

  //JsonObject t = getTeam(b_team);
  //t["TIME"] = b_time;
  //t["DELAY"] = b_delay;
  //t["STATUS"] = "PAUSE";
  setTeamStatus(b_team, "PAUSE");
  //t["BUMPER"] = bumperID;
  
  //setTeam(b_team, t);
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

void startGame(){
  resetBumpersTime();
  GameStarted=true;
  setGamePhase( "STARTED");
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
    if (serializeJson(tb, output)) {
      putMsgToQueue("START",output.c_str(),true);
    } else {
      ESP_LOGE(BUMPER_TAG, "Failed to serialize JSON");
    }
}

void stopGame(){
  GameStarted=false;
  setGamePhase( "STOPPED" );
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
  if (serializeJson(tb, output)) {
    putMsgToQueue("STOP",output.c_str(),true);
  } else {
     ESP_LOGE(BUMPER_TAG, "Failed to serialize JSON");
  }
}

void pauseGame(AsyncClient* client) {
  setGamePhase( "PAUSED" );
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
  if (serializeJson(tb, output)) {
    putMsgToQueue("PAUSE",output.c_str(),true, client);
  } else {
     ESP_LOGE(BUMPER_TAG, "Failed to serialize JSON");
  }
}

void pauseAllGame(){
  setGamePhase( "PAUSED" );
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
  if (serializeJson(tb, output)) {
    putMsgToQueue("PAUSE",output.c_str(),true);
  } else {
     ESP_LOGE(BUMPER_TAG, "Failed to serialize JSON");
  }
}

void continueGame(){
  startGame();
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
  //notifyAll();
  
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
  notifyAll();
}

void rebootServer() {
  ESP_LOGI(BUMPER_TAG, "Rebooting server");
  if (LittleFS.exists(saveGameFile)) {
    if (LittleFS.remove(saveGameFile)) {
      ESP_LOGI(BUMPER_TAG, "Save file deleted successfully");
    } else {
      ESP_LOGE(BUMPER_TAG, "Error: Unable to delete save file");
    }
  } 
  ESP.restart();
}

void handleHelloAction(const char* bumperID, JsonObject& MSG) {
  updateBumper(bumperID, MSG);
  notifyAll();
}

void handleButtonAction(const char* bumperID, JsonObject& MSG, AsyncClient* c) {
  ESP_LOGE(BUMPER_TAG, "Button pressed: %s", bumperID);
  JsonObject bumper=getBumper(bumperID);
  //if(bumper["IP"]==c->remoteIP().toString()) {
    const char* teamID=bumper["TEAM"];
//    int64_t b_time = MSG["time"].as<int64_t>();  // Changé en int64_t
    int b_button = MSG["button"];
    if (teamID != nullptr) {
      processButtonPress(bumperID, teamID, micros(), b_button);
      pauseGame(c);
    }
  //}
}

/*
void handleFile(JsonObject& MSG) {
  String fName = MSG["NAME"].as<String>();
  String question = MSG["QUESTION"].as<String>();
  size_t fSize = MSG["SIZE"].as<size_t>();
  JsonArray fContent = MSG["CONTENT"].as<JsonArray>();

  if (!LittleFS.begin()) {
    ESP_LOGE(BUMPER_TAG,"Erreur lors du montage de LittleFS");
    return;
  }

  String filePath = "/files/" + fName;
  File file = LittleFS.open(filePath, "w");
  if (!file) {
    ESP_LOGE(BUMPER_TAG,"Erreur lors de l'ouverture du fichier en écriture");
    return;
  }

  for (JsonVariant v : fContent) {
    uint8_t byte = v.as<uint8_t>();
    file.write(byte);
  }

  file.close();

  if (file.size() == fSize) {
    ESP_LOGI(BUMPER_TAG,"Fichier sauvegardé avec succès : " + filePath);
  } else {
    ESP_LOGE(BUMPER_TAG,"Erreur : La taille du fichier sauvegardé ne correspond pas à la taille attendue");
  }

  LittleFS.end();
}

*/
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

