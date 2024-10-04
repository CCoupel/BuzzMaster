#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <esp_log.h>

static const char* FS_TAG = "FS_MANAGER";

String baseURL="/base.url";
String baseFILE="/catalog.url";

void downloadFiles();
bool downloadFile(const String& url, const String& localPath);
bool createDirectories(const String& path);

void listLittleFSFiles() {
    ESP_LOGI(FS_TAG, "Listing files in LittleFS:");
    File root = LittleFS.open("/");
    if (!root) {
        ESP_LOGE(FS_TAG, "Failed to open directory");
        return;
    }
    if (!root.isDirectory()) {
        ESP_LOGE(FS_TAG, "Not a directory");
        return;
    }

    File file = root.openNextFile();
    while (file) {
        if (file.isDirectory()) {
            ESP_LOGI(FS_TAG, "DIR : %s", file.name());
        } else {
            ESP_LOGI(FS_TAG, "FILE: %s\tSIZE: %d", file.name(), file.size());
        }
        file = root.openNextFile();
    }
}


void loadJson(String path) {
    File file;
    String output;
    ESP_LOGI(FS_TAG, "Loading game file");
    
    if (LittleFS.exists(saveGameFile)) {
        file = LittleFS.open(saveGameFile, "r");
        ESP_LOGI(FS_TAG, "Loading from save file: %s", saveGameFile);
    } else if (LittleFS.exists(GameFile)) {
        file = LittleFS.open(GameFile, "r");
        ESP_LOGI(FS_TAG, "Loading from game file: %s", GameFile);
    }

    if (!file) {
        ESP_LOGE(FS_TAG, "Failed to open file for reading. Initializing with default values.");
        setBumpers(JsonObject());
        setTeams(JsonObject());
        return;
    }

    DeserializationError error = deserializeJson(teamsAndBumpers, file);
    if (error) {
        ESP_LOGE(FS_TAG, "deserializeJson() failed: %s", error.c_str());
        setBumpers( JsonObject());
        setTeams( JsonObject());
    } else {
        ESP_LOGI(FS_TAG, "JSON loaded successfully");
    }

    file.close();
    serializeJson(teamsAndBumpers, output);
    ESP_LOGI(FS_TAG, "JSON loaded: %s", output.c_str());
}

void saveJson() {
    File file = LittleFS.open(saveGameFile, "w");
    if (!file) {
        ESP_LOGE(FS_TAG, "Failed to open file for writing");
        return;
    }

    if (serializeJson(teamsAndBumpers, file) == 0) {
        ESP_LOGE(FS_TAG, "Failed to write to file");
    }

    file.close();
    ESP_LOGI(FS_TAG, "JSON saved successfully");
}

void downloadFiles() {
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return;
    }

    File baseFile = LittleFS.open(baseURL, "r");
    if (!baseFile) {
        ESP_LOGE(FS_TAG, "Failed to open base.url");
        return;
    }

    String baseUrl = baseFile.readString();
    baseUrl.trim();
    baseFile.close();

    File catalogFile = LittleFS.open(baseFILE, "r");
    if (!catalogFile) {
        ESP_LOGE(FS_TAG, "Failed to open catalog.url");
        return;
    }

    while (catalogFile.available()) {
        String filePath = catalogFile.readStringUntil('\n');
        filePath.trim();
        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;

        if (downloadFile(fileUrl, "/" + filePath)) {
            ESP_LOGI(FS_TAG, "Downloaded and saved %s", filePath.c_str());
        } else {
            ESP_LOGE(FS_TAG, "Failed to download %s", fileUrl.c_str());
        }
    }

    catalogFile.close();
}

bool downloadFile(const String& fileUrl, const String& localPath) {
    HTTPClient http;
    http.begin(fileUrl);
    int httpCode = http.GET();

    if (httpCode == HTTP_CODE_OK) {
        WiFiClient * stream = http.getStreamPtr();

        File file = LittleFS.open(localPath, "w");
        if (!file) {
            ESP_LOGE(FS_TAG, "Failed to open file for writing");
            http.end();
            return false;
        }

        const size_t bufferSize = 512;
        uint8_t buffer[bufferSize];

        int bytesRead;
        while ((bytesRead = stream->read(buffer, bufferSize)) > 0) {
            file.write(buffer, bytesRead);
        }

        file.close();
        ESP_LOGI(FS_TAG, "File downloaded successfully");
    } else {
        ESP_LOGE(FS_TAG, "HTTP GET failed with code %d", httpCode);
        http.end();
        return false;
    }

    http.end();
    return true;
}

bool createDirectories(const String& path) {
    // Extraire le chemin du répertoire
    int lastSlash = path.lastIndexOf('/');
    if (lastSlash == -1) return true; // Pas de répertoire à créer

    String directoryPath = path.substring(0, lastSlash);
    
    // Si le répertoire existe déjà, rien à faire
    if (LittleFS.exists(directoryPath)) {
        return true;
    }

    String subPath = "";
    for (int i = 1; i < lastSlash; i++) {
        if (path[i] == '/') {
            if (!LittleFS.exists(subPath)) {
                if (!LittleFS.mkdir(subPath)) {
                    ESP_LOGE(FS_TAG, "HTTP GET failed with code %s", subPath);
                    return false;
                }
            }
        }
        subPath += path[i];
    }

    return true;
}

