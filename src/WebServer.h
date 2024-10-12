#include <WiFi.h>
#include <ESPAsyncWebServer.h>
#include <esp_log.h>

static const char* WEB_TAG = "WEBSERVER";

void wifiConnect() {
    ESP_LOGI(WEB_TAG, "Connecting to %s", ssid);
    
    WiFi.begin(ssid, password);

    while (WiFi.status() != WL_CONNECTED) {
        delay(500);
        ESP_LOGI(WEB_TAG, ".");
    }

    ESP_LOGI(WEB_TAG, "WiFi connected. IP address: %s", WiFi.localIP().toString().c_str());
    ESP_LOGI(WEB_TAG, "MAC address: %s", WiFi.macAddress().c_str());
}

void w_handleRoot(AsyncWebServerRequest *request) {
  ESP_LOGI(WEB_TAG, "Handling root request");
  request->send(200, "text/plain", "hello from BuzzControl!");
  digitalWrite(ledPin, LOW);
}

void w_handleNotFound(AsyncWebServerRequest *request) {
  ESP_LOGW(WEB_TAG, "Handling 404 for: %s", request->url().c_str());
  String message = "File Not Found\n\n";
  message += "URI: " + request->url() + "\n";
  message += "Method: " + String((request->method() == HTTP_GET) ? "GET" : "POST") + "\n";
  message += "Arguments: " + String(request->args()) + "\n";
  
  for (uint8_t i = 0; i < request->args(); i++) {
      message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
  }
  
  request->send(404, "text/plain", message);
}

void w_handleReboot(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reboot request");
    rebootServer();
}

void w_handleReset(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reset request");
    resetServer();
}

void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }

    server.serveStatic("/", LittleFS, ROOT.c_str());
    server.on("/", HTTP_GET, [](AsyncWebServerRequest *request){
        request->redirect("/html/config.html");
    });

    server.on("/index.html", HTTP_GET, [](AsyncWebServerRequest *request){
        request->redirect("/html/config.html");
    });

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
    server.onNotFound(w_handleNotFound);
    
    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}