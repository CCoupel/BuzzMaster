#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>

String baseURL="/base.url";
String baseFILE="/catalog.url";

void downloadFiles();
bool downloadFile(const String& url, const String& localPath);
bool createDirectories(const String& path);

void downloadFiles() {
// Initialiser LittleFS
    if (!LittleFS.begin()) {
        Serial.println("Failed to mount LittleFS");
        return;
    }

    // Lire le fichier base.url pour obtenir la base de l'URL
    File baseFile = LittleFS.open(baseURL, "r");
    if (!baseFile) {
        Serial.println("Failed to open base.url");
        return;
    }

    String baseUrl = baseFile.readString();  // Lire la chaîne
    baseUrl.trim();                          // Appliquer trim() sur la chaîne
    baseFile.close();

    // Lire le fichier catalog.url pour obtenir la liste des fichiers à télécharger
    File catalogFile = LittleFS.open(baseFILE, "r");
    if (!catalogFile) {
        Serial.println("Failed to open catalog.url");
        return;
    }

    while (catalogFile.available()) {
        String filePath = catalogFile.readStringUntil('\n');
        filePath.trim();
        if (filePath.length() == 0) continue;

        // Construire l'URL complète
        String fileUrl = baseUrl + "/" + filePath;

        // Télécharger le fichier et le sauvegarder localement dans le même chemin
        if (downloadFile(fileUrl, "/" + filePath)) {
            Serial.print("Downloaded and saved ");
            Serial.println(filePath);
        } else {
            Serial.print("Failed to download ");
            Serial.println(fileUrl);
        }
    }

    catalogFile.close();
}

bool downloadFile(const String& fileUrl, const String& localPath) {
    HTTPClient http;
    http.begin(fileUrl); // Spécifie l'URL
    int httpCode = http.GET(); // Envoie la requête HTTP GET

    if (httpCode == HTTP_CODE_OK) {
        WiFiClient * stream = http.getStreamPtr(); // Obtient le flux de données

        File file = LittleFS.open(localPath, "w"); // Ouvre le fichier en mode écriture
        if (!file) {
            Serial.println("Failed to open file for writing");
            http.end();
            return false;
        }

        // Crée un buffer pour lire les données en petites portions
        const size_t bufferSize = 512; // Vous pouvez ajuster la taille du buffer
        uint8_t buffer[bufferSize];

        // Lit le flux de données et écrit dans le fichier en morceaux
        int bytesRead;
        while ((bytesRead = stream->read(buffer, bufferSize)) > 0) {
            file.write(buffer, bytesRead); // Écrit les données lues dans le fichier
        }

        file.close(); // Ferme le fichier après l'écriture
        Serial.println("File downloaded successfully");
    } else {
        Serial.print("HTTP GET failed with code ");
        Serial.println(httpCode);
        http.end();
        return false;
    }

    http.end(); // Termine la connexion HTTP
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
                    Serial.print("Failed to create directory: ");
                    Serial.println(subPath);
                    return false;
                }
            }
        }
        subPath += path[i];
    }

    return true;
}

