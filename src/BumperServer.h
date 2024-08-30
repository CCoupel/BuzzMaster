#include <ArduinoJson.h>
#include <AsyncTCP.h>
#include <esp_log.h>

static const char* BUMPER_TAG = "BUMPER_SERVER";


void processButtonPress(JsonObject& bumpers, JsonObject& teams, const String& bumperID, const char* b_team, int b_time, int b_button) {
  if (timeRef == 0) {
    timeRef = b_time;
  }
  int b_delay = b_time - timeRef;

  if (timeRefTeam[b_team] == 0) {
    timeRefTeam[b_team] = b_time;
  }

  int b_delayTeam = b_time - timeRefTeam[b_team];
  bumpers[bumperID]["TIME"] = b_time;
  bumpers[bumperID]["BUTTON"] = b_button;
  bumpers[bumperID]["DELAY"] = b_delay;
  bumpers[bumperID]["DELAY_TEAM"] = b_delayTeam;
  bumpers[bumperID]["STATUS"] = "PAUSE";

  teams[b_team]["TIME"] = b_time;
  teams[b_team]["DELAY"] = b_delay;
  teams[b_team]["STATUS"] = "PAUSE";
  teams[b_team]["BUMPER"] = bumperID;
}

void resetBumpersTime() {
  std::vector<String> keysToRemove = {"STATUS", "BUTTON", "TIME", "DELAY", "DELAY_TEAM"};

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
  putMsgToQueue("START","",true);
  
}

void stopGame(){
  GameStarted=false;
  putMsgToQueue("STOP","",true);

}

void pauseGame(AsyncClient* client) {
  putMsgToQueue("PAUSE","",true, client);
}

// Fonction pour fusionner deux documents JSON
void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
  for (JsonPair kvp : srcObj) {
    if (kvp.value().is<JsonObject>()) {
      JsonObject nestedDestObj;
      if (destObj.containsKey(kvp.key()) && destObj[kvp.key()].is<JsonObject>()) {
        nestedDestObj = destObj[kvp.key()].as<JsonObject>();
      } else {
        nestedDestObj = destObj[kvp.key()].to<JsonObject>();
      }
      JsonObject nestedSrcObj = kvp.value().as<JsonObject>();
      mergeJson(nestedDestObj, nestedSrcObj);
    } else {
      destObj[kvp.key()] = kvp.value();
    }
  }
}

void update(String action, JsonObject& obj) {
  JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
  String output;
  serializeJson(obj, output);

  ESP_LOGI(BUMPER_TAG, "Update: received %s", output.c_str());

  mergeJson(jsonObj, obj);
  serializeJson(jsonObj, output);
  ESP_LOGI(BUMPER_TAG, "Update: complete %s", output.c_str());
  
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

void handleHelloAction(JsonObject& bumpers, const String& bumperID, JsonObject& MSG) {
  if (!bumpers[bumperID].containsKey("NAME")) {
    bumpers[bumperID]["NAME"] = MSG["NAME"];
  }
  if (!bumpers[bumperID].containsKey("TEAM")) {
    bumpers[bumperID]["TEAM"] = "";
  }
  bumpers[bumperID]["IP"] = MSG["IP"];
  notifyAll();
}

void handleButtonAction(JsonObject& bumpers, JsonObject& teams, const String& bumperID, JsonObject& MSG, AsyncClient* c) {
  if (bumpers.containsKey(bumperID)) {
    if (bumpers[bumperID].containsKey("IP") && bumpers[bumperID]["IP"] == c->remoteIP().toString()) {
      const char* b_team = bumpers[bumperID]["TEAM"];
      int b_time = MSG["time"];
      int b_button = MSG["button"];
      if (b_team != nullptr) {
        processButtonPress(bumpers, teams, bumperID, b_team, b_time, b_button);
        pauseGame(c);
      }
    }
  }
}

void parseJSON(const String& data, AsyncClient* c)
{
  String action;
  String bumperID;

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
  action = jsonObjData["ACTION"].as<String>();
  MSG = jsonObjData["MSG"];

  ESP_LOGD(BUMPER_TAG, "bumperID=%s ACTION=%s", bumperID.c_str(), action.c_str());
    
  switch(hash(action.c_str())) {
    case hash("HELLO"):
      handleHelloAction(bumpers, bumperID, MSG);
      break;
    case hash("BUTTON"):
      handleButtonAction(bumpers, teams, bumperID, MSG, c);
      break;
    case hash("PING"):
      // Handle PING action if needed
      break;
    default:
      ESP_LOGW(BUMPER_TAG, "Unknown action: %s", action.c_str());
      break;
  }
}
/*
  if (action == "HELLO") {
    if ( !bumpers[bumperID].containsKey("NAME") ) {
      bumpers[bumperID]["NAME"]=MSG["NAME"];
    };
    if ( !bumpers[bumperID].containsKey("TEAM") ) {
      bumpers[bumperID]["TEAM"]="";
    };
    bumpers[bumperID]["IP"]=MSG["IP"];
    notifyAll();
  }
  if (action == "BUTTON") {
    if ( bumpers.containsKey(bumperID) ) {

      if ( bumpers[bumperID].containsKey("IP") && bumpers[bumperID]["IP"] == c->remoteIP().toString()) {
        const char * b_team=bumpers[bumperID]["TEAM"];
        int b_time=MSG["time"];
        int b_button=MSG["button"];
        if ( b_team != nullptr ) {
          
          if (timeRef==0) {
            timeRef=b_time;
          }
          int b_delay=b_time-timeRef;

         if (timeRefTeam[b_team]==0) {
            timeRefTeam[b_team]=b_time;
          }

          int b_delayTeam=b_time-timeRefTeam[b_team];
          bumpers[bumperID]["TIME"]=b_time;
          bumpers[bumperID]["BUTTON"]=b_button;
          bumpers[bumperID]["DELAY"]=b_delay;
          bumpers[bumperID]["DELAY_TEAM"]=b_delayTeam;

          teams[b_team]["TIME"]=b_time;
          teams[b_team]["DELAY"]=b_delay;
          teams[b_team]["STATUS"]="PAUSE";
          teams[b_team]["BUMPER"]=bumperID;

          pauseGame(c);
        };
      };
    };
  };
  if (action == "PING") {/*
    bumpers[bumperID]["lastPingTime"] = millis();  
    if (bumpers[bumperID]["STATUS"] != "online") {
      Serial.println("BUMPER: Bumper is going online");
      bumpers[bumperID]["STATUS"] = "online";
      notifyAll();
    }*/
//  }
//}

