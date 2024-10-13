#include <WiFi.h>
#include <ESPAsyncWebServer.h>
#include <esp_log.h>
#include <DNSServer.h>
#include <ESPmDNS.h>

static const char* WIFI_TAG = "WIFI";
static const char* WEB_TAG = "WEBSERVER";
const char* ssid     = "CC-Home";
const char* password = "GenericPassword";

const char* apSSID = "buzzmaster";
const char* apPASSWORD = "BuzzMaster";

/*
IPAddress apIP(192, 168, 10, 1);
IPAddress gateway(192, 168, 10, 1);
*/
IPAddress apIP(8,8,8,8);
IPAddress gateway(8,8,8,8);
IPAddress subnet(255, 255, 255, 0);


DNSServer dnsServer;
TaskHandle_t wifiConnectTaskHandle = NULL;

void wifiConnectTask(void * parameter) {
    for (;;) {
        if (WiFi.status() != WL_CONNECTED) {
            ESP_LOGI(WIFI_TAG, "Attempting to connect to %s", ssid);
            WiFi.begin(ssid, password);
            
            int attempts = 0;
            while (WiFi.status() != WL_CONNECTED && attempts < 20) {
                vTaskDelay(500 / portTICK_PERIOD_MS);
                attempts++;
                //ESP_LOGI(WIFI_TAG, ".");
            }
            
            if (WiFi.status() == WL_CONNECTED) {
                ESP_LOGI(WIFI_TAG, "WiFi connected. IP address: %s", WiFi.localIP().toString().c_str());
                ESP_LOGI(WIFI_TAG, "MAC address: %s", WiFi.macAddress().c_str());
            } else {
                ESP_LOGW(WIFI_TAG, "Failed to connect to WiFi. Will retry...");
            }
        }
        vTaskDelay(1000 / portTICK_PERIOD_MS);  // Wait for 30 seconds before checking again
    }
}

void wifiConnect() {
    ESP_LOGI(WIFI_TAG, "Connecting to %s", ssid);
    WiFi.mode(WIFI_AP_STA);
    xTaskCreate(
        wifiConnectTask,
        "WiFiConnectTask",
        4096,
        NULL,
        1,
        &wifiConnectTaskHandle
    );
}

void setupAP() {
//    WiFi.mode(WIFI_AP_STA);
    WiFi.softAPConfig(apIP, gateway, subnet);
    WiFi.softAP(apSSID, apPASSWORD);
    
    // Attendez un peu que l'AP soit complètement configuré
    delay(100);
    
    IPAddress IP = WiFi.softAPIP();
    ESP_LOGI(WIFI_TAG, "WiFi AP: %s started with IP: %s", apSSID, IP.toString().c_str());
}

void setupDNSServer() {
  dnsServer.start(53, "*", apIP);
  ESP_LOGI(WIFI_TAG,"Serveur DNS démarré");

  if (MDNS.begin("buzzcontrol")) {
    ESP_LOGI(WIFI_TAG, "MDNS responder started");
  }
  MDNS.addService("buzzcontrol", "tcp", localWWWpPort);
  MDNS.addService("http", "tcp", localWWWpPort);
  MDNS.addService("sock", "tcp", CONTROLER_PORT);
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


void w_handleRedirect(AsyncWebServerRequest *request) {
    ESP_LOGD(WEB_TAG, "REdirecting for: %s", request->url().c_str());
    request->redirect("http://buzzcontrol.local/html/config.html");
}

/*
void handleCaptivePortal(AsyncWebServerRequest *request) {
        ESP_LOGW(WEB_TAG, "Captive Redirect for: %s", request->url().c_str());

    AsyncResponseStream *response = request->beginResponseStream("text/html");
    response->print("<!DOCTYPE html><html><head><title>Redirection</title>");
    response->print("<meta http-equiv=\"refresh\" content=\"0;url=http://");
    response->print(apIP.toString());
    response->print("/html/config.html\">");
    response->print("</head><body>Redirection vers le portail captif...</body></html>");
    request->send(response);
}
*/

void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }

    server.serveStatic("/", LittleFS, ROOT.c_str());
    //server.on("/", HTTP_GET, handleCaptivePortal);

    server.on("/index.html", w_handleRedirect);

    server.on("/generate_204", w_handleRedirect);  // Android captive portal
    server.on("/gen_204", w_handleRedirect);       // Android captive portal
    server.on("/hotspot-detect.html", w_handleRedirect);  // iOS captive portal
    server.on("/canonical.html", w_handleRedirect);       // Windows captive portal
    server.on("/success.txt", w_handleRedirect);          // macOS captive portal
    server.on("/favicon.ico", w_handleRedirect);   // Browser icon

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
//    server.onNotFound(w_handleRedirect);
    
    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}