#include <WiFi.h>
#include <NTPServer.h>

NTPServer ntpServer(WiFi.localIP());
static const char *ntpTAG="NTP_SERVER";

void setupNTPserver() {
    ntpServer.begin();
  ESP_LOGI(ntpTAG,"Serveur NTP démarré");
}

void handleNTPserver() {
  ntpServer.handle();
}