#include <esp_log.h>

static const char* PING_TAG = "PING";

void checkPingForAllClients() {
    unsigned long currentTime = millis();
    JsonObject bumpers = teamsAndBumpers["bumpers"];
    
    for (JsonPair bumperPair : bumpers) {
        JsonObject bumper = bumperPair.value().as<JsonObject>();
        if (currentTime - bumper["lastPingTime"].as<unsigned long>() > 3000) {
            if (bumper["STATUS"] != "offline") {
                ESP_LOGW(PING_TAG, "Bumper %s is going offline", bumper["IP"].as<String>().c_str());
                bumper["STATUS"] = "offline";
                notifyAll();
            }
        }
    }
}

void IRAM_ATTR onTimerISR() {
    sendMessageToAllClients("PING", "'Are you alive?'");
    checkPingForAllClients();
}

