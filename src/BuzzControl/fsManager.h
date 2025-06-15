#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>
//#include <esp_log.h>
#include "esp_littlefs.h"
#include <LittleFS.h>

#define DEST_FS_USES_LITTLEFS
#include <ESP32-targz.h>
#include <vector>

static const char* FS_TAG = "FS_MANAGER";
String VERSION_FILE="/config/version.txt";
String TEMP_DIR="/temp_update";
//String BACKUP_DIR="/backup";

String baseURL="/config/base.url";
String baseFILE="/config/catalog.url";

bool createDirectories(const String& path);

// BACKUP //
class StreamingTarBuffer : public Stream {
private:
    std::vector<TAR::dir_entity_t> entities;
    size_t currentEntityIndex;
    size_t currentFileOffset;
    File currentFile;
    bool headerSent;
    size_t totalBytesGenerated;
    size_t pendingPadding;
    size_t endBlocksSent; // Pour compter les blocs de fin envoyés
    
    // Buffer temporaire pour les headers TAR
    uint8_t headerBuffer[512];
    size_t headerPos;
    
public:
    StreamingTarBuffer() : currentEntityIndex(0), currentFileOffset(0), 
                          headerSent(false), totalBytesGenerated(0), headerPos(0),
                          pendingPadding(0), endBlocksSent(0) {}
    
    ~StreamingTarBuffer() {
        if (currentFile) currentFile.close();
    }
    
    bool initialize() {
        entities.clear();
        currentEntityIndex = 0;
        currentFileOffset = 0;
        headerSent = false;
        totalBytesGenerated = 0;
        headerPos = 0;
        pendingPadding = 0;
        endBlocksSent = 0;
        
        if (currentFile) {
            currentFile.close();
        }
        
        yield(); // Reset watchdog
        
        // Collecter seulement le CONTENU de /files (pas le dossier lui-même)
        std::vector<TAR::dir_entity_t> allEntities;
        TarPacker::collectDirEntities(&allEntities, &LittleFS, "/files");
        
        // Filtrer pour exclure le dossier /files lui-même
        for (const auto& entity : allEntities) {
            if (entity.path != "/files") {  // Exclure le dossier racine /files
                entities.push_back(entity);
            }
        }
        
        if (entities.empty()) {
            ESP_LOGW(FS_TAG, "Aucun contenu trouvé dans /files");
            return false;
        }
        
        ESP_LOGI(FS_TAG, "Collecté %d entités depuis le contenu de /files", entities.size());
        for (const auto& entity : entities) {
            ESP_LOGD(FS_TAG, "  - %s (%s)", entity.path.c_str(), entity.is_dir ? "DIR" : "FILE");
        }
        return true;
    }
    
    // Méthodes Stream non utilisées mais nécessaires
    size_t write(uint8_t data) override { return 0; }
    size_t write(const uint8_t *data, size_t len) override { return 0; }
    int peek() override { return -1; }
    void flush() override {}
    
    int available() override {
        // Retourne qu'il y a des données disponibles si :
        // - On n'a pas fini tous les fichiers
        // - Ou on a du padding en attente
        // - Ou on n'a pas encore envoyé les 2 blocs de fin (1024 bytes)
        return (currentEntityIndex < entities.size() || pendingPadding > 0 || endBlocksSent < 1024) ? 1 : 0;
    }
    
    int read() override {
        uint8_t byte;
        return (readBytes(&byte, 1) == 1) ? byte : -1;
    }
    
    size_t readBytes(uint8_t* buffer, size_t length) {
        size_t bytesRead = 0;
        
        while (bytesRead < length) {
            yield(); // Reset watchdog périodiquement
            
            // Gérer le padding en attente d'abord
            if (pendingPadding > 0) {
                size_t paddingToAdd = min(pendingPadding, length - bytesRead);
                memset(buffer + bytesRead, 0, paddingToAdd);
                bytesRead += paddingToAdd;
                pendingPadding -= paddingToAdd;
                totalBytesGenerated += paddingToAdd;
                continue;
            }
            
            // Si on a fini tous les fichiers, envoyer les blocs de fin
            if (currentEntityIndex >= entities.size()) {
                if (endBlocksSent < 1024) {
                    size_t endBytesToSend = min((size_t)(1024 - endBlocksSent), length - bytesRead);
                    memset(buffer + bytesRead, 0, endBytesToSend);
                    bytesRead += endBytesToSend;
                    endBlocksSent += endBytesToSend;
                    totalBytesGenerated += endBytesToSend;
                    continue;
                } else {
                    // Vraiment fini
                    break;
                }
            }
            
            // Si on n'a pas encore envoyé le header pour le fichier courant
            if (!headerSent) {
                size_t headerBytesAvailable = 512 - headerPos;
                if (headerBytesAvailable > 0) {
                    // Générer le header TAR si pas encore fait
                    if (headerPos == 0) {
                        generateTarHeader(entities[currentEntityIndex]);
                    }
                    
                    size_t toCopy = min(headerBytesAvailable, length - bytesRead);
                    memcpy(buffer + bytesRead, headerBuffer + headerPos, toCopy);
                    headerPos += toCopy;
                    bytesRead += toCopy;
                    totalBytesGenerated += toCopy;
                    
                    if (headerPos >= 512) {
                        headerSent = true;
                        headerPos = 0;
                        
                        // Ouvrir le fichier s'il ne s'agit pas d'un répertoire
                        if (!entities[currentEntityIndex].is_dir) {
                            currentFile = LittleFS.open(entities[currentEntityIndex].path.c_str(), "r");
                            if (!currentFile) {
                                ESP_LOGE(FS_TAG, "Impossible d'ouvrir le fichier: %s", 
                                        entities[currentEntityIndex].path.c_str());
                                // Passer au fichier suivant
                                nextEntity();
                                continue;
                            }
                        } else {
                            // Pour un répertoire, passer directement au suivant
                            nextEntity();
                            continue;
                        }
                    }
                    continue;
                }
            }
            
            // Lire les données du fichier (seulement si ce n'est pas un répertoire)
            if (currentFile && currentFile.available()) {
                size_t maxRead = min((size_t)currentFile.available(), length - bytesRead);
                size_t actualRead = currentFile.read(buffer + bytesRead, maxRead);
                bytesRead += actualRead;
                currentFileOffset += actualRead;
                totalBytesGenerated += actualRead;
                
                // Vérifier si on a fini de lire le fichier
                if (!currentFile.available()) {
                    currentFile.close();
                    
                    // Calculer le padding nécessaire pour aligner sur 512 bytes
                    pendingPadding = (512 - (currentFileOffset % 512)) % 512;
                    
                    // Si on peut ajouter du padding maintenant, le faire
                    if (pendingPadding > 0 && bytesRead < length) {
                        size_t paddingToAdd = min(pendingPadding, length - bytesRead);
                        memset(buffer + bytesRead, 0, paddingToAdd);
                        bytesRead += paddingToAdd;
                        pendingPadding -= paddingToAdd;
                        totalBytesGenerated += paddingToAdd;
                    }
                    
                    // Passer au fichier suivant seulement si on a fini le padding
                    if (pendingPadding == 0) {
                        nextEntity();
                    }
                }
            } else {
                // Pas de fichier ouvert ou plus de données - passer au suivant
                nextEntity();
            }
        }
        
        return bytesRead;
    }
    
private:
    void nextEntity() {
        currentEntityIndex++;
        currentFileOffset = 0;
        headerSent = false;
        headerPos = 0;
        pendingPadding = 0;
        if (currentFile) {
            currentFile.close();
        }
    }
    
    void generateTarHeader(const TAR::dir_entity_t& entity) {
        memset(headerBuffer, 0, 512);
        
        // Nom du fichier - RETIRER le préfixe /files/ pour mettre à la racine du TAR
        String relativePath = entity.path;
        if (relativePath.startsWith("/files/")) {
            relativePath = relativePath.substring(7);  // Enlever "/files/"
        }
        
        ESP_LOGD(FS_TAG, "Génération header TAR: %s -> %s", entity.path.c_str(), relativePath.c_str());
        
        strncpy((char*)headerBuffer, relativePath.c_str(), 100);
        
        // Mode (permissions)
        sprintf((char*)headerBuffer + 100, "%07o", entity.is_dir ? 0755 : 0644);
        
        // UID/GID
        sprintf((char*)headerBuffer + 108, "%07o", 0);
        sprintf((char*)headerBuffer + 116, "%07o", 0);
        
        // Taille
        if (!entity.is_dir) {
            sprintf((char*)headerBuffer + 124, "%011lo", (unsigned long)entity.size);
        } else {
            sprintf((char*)headerBuffer + 124, "%011o", 0);
        }
        
        // Timestamp
        sprintf((char*)headerBuffer + 136, "%011lo", (unsigned long)time(nullptr));
        
        // Type de fichier
        headerBuffer[156] = entity.is_dir ? '5' : '0';
        
        // Magic number
        strcpy((char*)headerBuffer + 257, "ustar");
        headerBuffer[263] = '0';
        headerBuffer[264] = '0';
        
        // Calculer et définir le checksum
        unsigned int checksum = 0;
        for (int i = 0; i < 512; i++) {
            if (i >= 148 && i < 156) {
                checksum += ' '; // Les octets du checksum sont comptés comme des espaces
            } else {
                checksum += headerBuffer[i];
            }
        }
        sprintf((char*)headerBuffer + 148, "%06o", checksum);
        headerBuffer[154] = 0;
        headerBuffer[155] = ' ';
    }
    
public:
    size_t getTotalBytesGenerated() { return totalBytesGenerated; }
    bool isFinished() { 
        return currentEntityIndex >= entities.size() && 
               pendingPadding == 0 && 
               endBlocksSent >= 1024; 
    }
};

// Variables globales pour le backup streaming
static StreamingTarBuffer* streamingTarBuffer = nullptr;
static bool tarStreamReady = false;

// Initialise le backup TAR streaming
bool initStreamingTarBackup() {
    // Nettoyer toute session précédente
    if (streamingTarBuffer) {
        delete streamingTarBuffer;
        streamingTarBuffer = nullptr;
    }
    
    tarStreamReady = false;
    
    // Vérifier que le répertoire /files existe
    if (!LittleFS.exists("/files")) {
        ESP_LOGW(FS_TAG, "Répertoire /files non trouvé");
        return false;
    }
    
    yield(); // Reset watchdog
    
    // Créer le buffer streaming
    streamingTarBuffer = new StreamingTarBuffer();
    if (!streamingTarBuffer) {
        ESP_LOGE(FS_TAG, "Échec allocation StreamingTarBuffer");
        return false;
    }
    
    // Initialiser le streaming
    if (!streamingTarBuffer->initialize()) {
        ESP_LOGE(FS_TAG, "Échec initialisation StreamingTarBuffer");
        delete streamingTarBuffer;
        streamingTarBuffer = nullptr;
        return false;
    }
    
    tarStreamReady = true;
    ESP_LOGI(FS_TAG, "Backup TAR streaming initialisé avec succès");
    return true;
}

// Récupère le prochain chunk du TAR (streaming)
size_t getStreamingTarChunk(uint8_t* output, size_t maxLen) {
    if (!streamingTarBuffer || !tarStreamReady) {
        return 0;
    }
    
    yield(); // Reset watchdog
    
    // Logging pour debug (seulement périodiquement)
    static size_t lastLoggedBytes = 0;
    size_t currentBytes = streamingTarBuffer->getTotalBytesGenerated();
    if (currentBytes - lastLoggedBytes > 10240) { // Log tous les 10KB
        ESP_LOGI(FS_TAG, "Backup progress: %zu bytes générés", currentBytes);
        lastLoggedBytes = currentBytes;
    }
    
    size_t bytesRead = streamingTarBuffer->readBytes(output, maxLen);
    
    if (bytesRead == 0 && streamingTarBuffer->isFinished()) {
        ESP_LOGI(FS_TAG, "Backup TAR streaming terminé: %zu bytes total", 
                streamingTarBuffer->getTotalBytesGenerated());
        lastLoggedBytes = 0; // Reset pour le prochain backup
    }
    
    return bytesRead;
}

// Nettoie le backup TAR streaming
void cleanupStreamingTarBackup() {
    if (streamingTarBuffer) {
        delete streamingTarBuffer;
        streamingTarBuffer = nullptr;
    }
    tarStreamReady = false;
    ESP_LOGI(FS_TAG, "Backup TAR streaming nettoyé");
}

// Fonction utilitaire pour obtenir la taille estimée du backup
size_t getEstimatedBackupSize() {
    if (!LittleFS.exists("/files")) {
        return 0;
    }
    
    std::vector<TAR::dir_entity_t> entities;
    TarPacker::collectDirEntities(&entities, &LittleFS, "/files");
    
    size_t estimatedSize = 0;
    for (const auto& entity : entities) {
        estimatedSize += 512; // Header TAR
        if (!entity.is_dir) {
            // Taille du fichier + padding pour aligner sur 512
            size_t fileSize = entity.size;
            size_t paddedSize = ((fileSize + 511) / 512) * 512;
            estimatedSize += paddedSize;
        }
    }
    estimatedSize += 1024; // Blocs de fin TAR
    
    return estimatedSize;
}
// GÃ©nÃ¨re le nom de fichier de backup
String generateBackupFilename() {
    time_t now = time(nullptr);
    struct tm* timeinfo = localtime(&now);
    
    char filename[64];
    strftime(filename, sizeof(filename), "buzzcontrol_backup_%Y%m%d_%H%M%S.tar", timeinfo);
    
    return String(filename);
}


// RESTAURE //

// ========== PARTIE RESTORE TAR ==========
// À ajouter dans fsManager.h après les fonctions de backup

// Classe pour traiter les données TAR en streaming lors du restore
// Classe Stream pour recevoir les données TAR depuis HTTP en streaming
class HttpTarStream : public Stream {
private:
    uint8_t* buffer;
    size_t bufferSize;
    size_t writePos;
    size_t readPos;
    size_t totalReceived; // Nouvelle variable pour tracer la taille totale
    bool streamComplete;
    
public:
    HttpTarStream(size_t size = 8192) : 
        bufferSize(size), writePos(0), readPos(0), totalReceived(0), streamComplete(false) {
        buffer = (uint8_t*)malloc(size);
    }
    
    ~HttpTarStream() {
        if (buffer) free(buffer);
    }
    
    bool isValid() { return buffer != nullptr; }
    
    // Méthodes pour recevoir les données HTTP
    size_t receiveData(const uint8_t* data, size_t len) {
        if (!buffer || streamComplete) return 0;
        
        size_t space = bufferSize - writePos;
        size_t toWrite = min(len, space);
        
        if (toWrite > 0) {
            memcpy(buffer + writePos, data, toWrite);
            writePos += toWrite;
            totalReceived += toWrite; // Mettre à jour la taille totale
        }
        
        yield(); // Céder la main pendant la réception
        return toWrite;
    }
    
    void markComplete() {
        streamComplete = true;
    }
    
    bool isComplete() {
        return streamComplete && (readPos >= writePos);
    }
    
    // Nouvelle méthode pour obtenir la taille totale reçue
    size_t getTotalSize() {
        return totalReceived;
    }
    
    // Interface Stream pour ESP32-targz (lecture)
    int available() override {
        return writePos - readPos;
    }
    
    int read() override {
        if (readPos < writePos) {
            return buffer[readPos++];
        }
        return -1;
    }
    
    int peek() override {
        if (readPos < writePos) {
            return buffer[readPos];
        }
        return -1;
    }
    
    void flush() override {
        // Compacter le buffer : déplacer les données non lues au début
        if (readPos > 0 && readPos < writePos) {
            memmove(buffer, buffer + readPos, writePos - readPos);
            writePos -= readPos;
            readPos = 0;
        }
    }
    
    // Interface Stream pour écriture (pas utilisé)
    size_t write(uint8_t) override { return 0; }
    size_t write(const uint8_t*, size_t) override { return 0; }
    
    // Méthodes utilitaires
    bool hasSpace() {
        return writePos < bufferSize;
    }
    
    size_t getFreeSpace() {
        return bufferSize - writePos;
    }
    
    void reset() {
        writePos = 0;
        readPos = 0;
        totalReceived = 0;
        streamComplete = false;
    }
};

// Variables globales pour le restore
static HttpTarStream* tarStream = nullptr;
static TarUnpacker* tarUnpacker = nullptr;
static bool restoreInProgress = false;
static bool restoreSuccess = false;
static bool tarGzFSInitialized = false;

// Déclaration anticipée
void cleanupTarRestore();

// Initialise tarGzFS (requis par ESP32-targz)
bool initTarGzFS() {
    if (tarGzFSInitialized) {
        return true;
    }
    
    // tarGzFS est un alias automatique créé par ESP32-targz quand DEST_FS_USES_LITTLEFS est défini
    // Il pointe vers LittleFS
    if (!tarGzFS.begin()) {
        ESP_LOGE(FS_TAG, "Échec initialisation tarGzFS");
        return false;
    }
    
    tarGzFSInitialized = true;
    ESP_LOGI(FS_TAG, "tarGzFS initialisé");
    return true;
}

// Initialise le restore TAR streaming
bool initTarRestore() {
    if (restoreInProgress) {
        ESP_LOGW(FS_TAG, "Un restore est déjà en cours");
        return false;
    }
    
    // Nettoyer toute session précédente
    cleanupTarRestore();
    
    yield();
    
    // Initialiser tarGzFS (OBLIGATOIRE)
    if (!initTarGzFS()) {
        ESP_LOGE(FS_TAG, "Impossible d'initialiser tarGzFS");
        return false;
    }
    
    // Créer le stream pour recevoir les données HTTP
    tarStream = new HttpTarStream();
    if (!tarStream || !tarStream->isValid()) {
        ESP_LOGE(FS_TAG, "Échec allocation HttpTarStream");
        cleanupTarRestore();
        return false;
    }
    
    // Créer le TarUnpacker d'ESP32-targz
    tarUnpacker = new TarUnpacker();
    if (!tarUnpacker) {
        ESP_LOGE(FS_TAG, "Échec création TarUnpacker");
        cleanupTarRestore();
        return false;
    }
    
    // Configuration du unpacker (basée sur l'exemple)
    tarUnpacker->haltOnError(false); // Continuer en cas d'erreur mineure
    tarUnpacker->setTarVerify(false); // Pas de vérification pour économiser la RAM
    
    // Setup callbacks comme dans l'exemple
    tarUnpacker->setupFSCallbacks(targzTotalBytesFn, targzFreeBytesFn);
    
    // Callbacks de progression
    tarUnpacker->setTarProgressCallback([](uint8_t progress) {
        static uint8_t lastProgress = 255;
        if (progress != lastProgress && progress % 20 == 0) {
            ESP_LOGI(FS_TAG, "Restore progress: %d%%", progress);
            lastProgress = progress;
        }
    });
    
    tarUnpacker->setTarStatusProgressCallback([](const char* name, size_t size, size_t total_unpacked) {
        ESP_LOGI(FS_TAG, "Extrait: %s (%zu bytes)", name, size);
    });
    
    tarUnpacker->setTarMessageCallback(BaseUnpacker::targzPrintLoggerCallback);
    
    // S'assurer que le répertoire de destination existe
    ensureDirectoryExists("/files");
    
    restoreInProgress = true;
    restoreSuccess = false;
    
    ESP_LOGI(FS_TAG, "Restore TAR streaming initialisé");
    return true;
}

// Traite un chunk de données TAR reçues depuis HTTP
size_t processTarChunk(const uint8_t* data, size_t len) {
    if (!tarStream || !restoreInProgress) {
        ESP_LOGE(FS_TAG, "Restore non initialisé");
        return 0;
    }
    
    yield();
    
    // Recevoir les données dans le stream
    size_t received = tarStream->receiveData(data, len);
    
    if (received != len) {
        ESP_LOGW(FS_TAG, "Chunk partiellement reçu: %zu/%zu bytes", received, len);
    }
    
    return received;
}

// Finalise le restore en traitant toutes les données
bool finalizeTarRestore() {
    if (!tarStream || !tarUnpacker || !restoreInProgress) {
        ESP_LOGW(FS_TAG, "Restore non initialisé pour finalisation");
        return false;
    }
    
    yield();
    
    // Marquer le stream comme complet
    tarStream->markComplete();
    
    ESP_LOGI(FS_TAG, "Début extraction TAR streaming...");
    
    // Obtenir la taille totale du TAR reçu
    size_t streamSize = tarStream->getTotalSize();
    if (streamSize == 0) {
        ESP_LOGW(FS_TAG, "Aucune donnée TAR reçue");
        restoreInProgress = false;
        return false;
    }
    
    ESP_LOGI(FS_TAG, "Taille totale du TAR reçu: %zu bytes", streamSize);
    
    // Utiliser la signature correcte de l'exemple Unpack_tar_stream.ino
    // Format : tarStreamExpander(Stream* stream, size_t streamSize, fs::FS& destinationFS, const char* destinationPath)
    bool success = tarUnpacker->tarStreamExpander(tarStream, streamSize, tarGzFS, "/files");
    
    if (success) {
        ESP_LOGI(FS_TAG, "Restore TAR terminé avec succès");
        restoreSuccess = true;
    } else {
        int errorCode = tarUnpacker->tarGzGetError();
        ESP_LOGE(FS_TAG, "Erreur restore TAR, code: %d", errorCode);
        restoreSuccess = false;
    }
    
    // Nettoyer
    restoreInProgress = false;
    
    return success;
}

// Nettoie le restore TAR
void cleanupTarRestore() {
    if (tarUnpacker) {
        delete tarUnpacker;
        tarUnpacker = nullptr;
    }
    
    if (tarStream) {
        delete tarStream;
        tarStream = nullptr;
    }
    
    restoreInProgress = false;
    ESP_LOGI(FS_TAG, "Restore TAR nettoyé");
}

// Vérifie si un restore est en cours
bool isRestoreInProgress() {
    return restoreInProgress;
}

// Obtient le status du restore
String getRestoreStatus() {
    if (!restoreInProgress) {
        if (restoreSuccess) {
            return "Restore terminé avec succès";
        } else {
            return "Aucun restore en cours";
        }
    }
    
    return "Restore en cours...";
}






/***** FILES ********/
String readFile(const String& path, const String& defaultValue) {
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Échec du montage de LittleFS");
        return defaultValue;
    }

    File file = LittleFS.open(path, "r");
    if (!file) {
        ESP_LOGE(FS_TAG, "Échec de l'ouverture du fichier %s", path.c_str());
        return defaultValue;
    }

    String content = file.readString();
    file.close();
    content.trim();
    return content;
}

bool deleteFile(const char* filePath) {
    if (!LittleFS.exists(filePath)) {
        ESP_LOGW(FS_TAG, "File does not exist: %s", filePath);
        return false;
    }
    
    if (LittleFS.remove(filePath)) {
        ESP_LOGI(FS_TAG, "File deleted: %s", filePath);
        return true;
    } else {
        ESP_LOGE(FS_TAG, "Failed to delete file: %s", filePath);
        return false;
    }
}

bool downloadFile(const String& fileUrl, const String& localPath) {
    HTTPClient http;
    http.begin(fileUrl);
    int httpCode = http.GET();

    createDirectories(localPath);

    if (httpCode == HTTP_CODE_OK) {
        WiFiClient * stream = http.getStreamPtr();

        File file = LittleFS.open(localPath, "w");
        if (!file) {
            ESP_LOGE(FS_TAG, "Failed to open file for writing %s", localPath.c_str());
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
        ESP_LOGI(FS_TAG, "File downloaded successfully %s to %s", fileUrl.c_str(), localPath.c_str());
    } else {
        ESP_LOGE(FS_TAG, "HTTP GET failed with code %d", httpCode);
        http.end();
        return false;
    }

    http.end();
    return true;
}

bool copyFile(const char* sourcePath, const char* destPath) {
    File sourceFile = LittleFS.open(sourcePath, "r");
    if (!sourceFile) {
        ESP_LOGE(FS_TAG, "Failed to open source file for reading %s", sourceFile);

        return false;
    }

    File destFile = LittleFS.open(destPath, "w");
    if (!destFile) {
        ESP_LOGE(FS_TAG, "Failed to open destination file for writing %s", destPath);
        sourceFile.close();
        return false;
    }

    static uint8_t buf[512];
    size_t len = 0;
    while ((len = sourceFile.read(buf, sizeof(buf))) > 0) {
        destFile.write(buf, len);
    }

    sourceFile.close();
    destFile.close();
    return true;
}

bool moveFile(const char* sourcePath, const char* destPath) {
    if (!LittleFS.exists(sourcePath)) {
        ESP_LOGE(FS_TAG, "Source file does not exist: %s", sourcePath);
        return false;
    }

    if (LittleFS.exists(destPath)) {
        ESP_LOGW(FS_TAG, "Destination file already exists, overwriting: %s", destPath);
        if (!deleteFile(destPath)) {
            ESP_LOGE(FS_TAG, "Failed to delete existing destination file: %s", destPath);
            return false;
        }
    }

    if (!copyFile(sourcePath, destPath)) {
        ESP_LOGE(FS_TAG, "Failed to copy file from %s to %s", sourcePath, destPath);
        return false;
    }

    if (!deleteFile(sourcePath)) {
        ESP_LOGE(FS_TAG, "Failed to delete source file after copy: %s", sourcePath);
        // Consider if you want to delete the destination file here in case of failure
        return false;
    }

    ESP_LOGI(FS_TAG, "File moved successfully from %s to %s", sourcePath, destPath);
    return true;
}

/***** DIR ********/

bool ensureDirectoryExists(const String& path) {
    if (LittleFS.exists(path)) {
        return true;  // Le répertoire existe déjà
    }
    ESP_LOGD(FS_TAG, "Creating Directory %s", path.c_str());
    return LittleFS.mkdir(path);
}

bool createDirectories(const String& path) {
    // Extraire le chemin du répertoire
    int lastSlash = path.lastIndexOf('/');
    if (lastSlash == -1) return true; // Pas de répertoire à créer

    String directoryPath = path.substring(0, lastSlash);
    ensureDirectoryExists(directoryPath);
    return true;
}

bool deleteDirectory(const char* dirPath) {
    std::vector<String> dirStack;
    dirStack.push_back(String(dirPath));

    while (!dirStack.empty()) {
        // Reset watchdog timer périodiquement
        esp_task_wdt_reset();
        
        // Ajouter un delay pour céder du temps CPU
        vTaskDelay(pdMS_TO_TICKS(1));
        String currentPath = dirStack.back();
        
        File dir = LittleFS.open(currentPath.c_str());
        if (!dir || !dir.isDirectory()) {
            ESP_LOGE(FS_TAG, "Failed to open directory: %s", currentPath.c_str());
            dirStack.pop_back();
            continue;
        }

        File file = dir.openNextFile();
        if (file) {
            char filePath[64];
            snprintf(filePath, sizeof(filePath), "%s/%s", currentPath.c_str(), file.name());

            if (file.isDirectory()) {
                dirStack.push_back(String(filePath));
            } else {
                file.close(); // Close the file before attempting to delete it
                if (LittleFS.remove(filePath)) {
                    ESP_LOGI(FS_TAG, "Deleted file: %s", filePath);
                } else {
                    ESP_LOGE(FS_TAG, "Failed to delete file: %s", filePath);
                    // Additional diagnostics
                    File testFile = LittleFS.open(filePath, "r");
                    if (testFile) {
                        ESP_LOGI(FS_TAG, "File can be opened for reading. Size: %d bytes", testFile.size());
                        testFile.close();
                    } else {
                        ESP_LOGE(FS_TAG, "File cannot be opened for reading");
                    }
                }
            }
        } else {
            dir.close();
            if (LittleFS.rmdir(currentPath.c_str())) {
                ESP_LOGI(FS_TAG, "Removed empty directory: %s", currentPath.c_str());
            } else {
                ESP_LOGE(FS_TAG, "Failed to remove directory: %s", currentPath.c_str());
            }
            dirStack.pop_back();
        }

        dir.close();
    }

    bool success = !LittleFS.exists(dirPath);
    ESP_LOGI(FS_TAG, "Directory deletion %s: %s", success ? "succeeded" : "failed", dirPath);
    return success;
}

bool moveDirectory(const char* sourceDir, const char* destDir) {
    std::vector<std::pair<String, String>> dirStack;
    dirStack.push_back({String(sourceDir), String(destDir)});

    while (!dirStack.empty()) {
        esp_task_wdt_reset();
        auto [currentSource, currentDest] = dirStack.back();
        dirStack.pop_back();

        if (!LittleFS.exists(currentSource.c_str())) {
            ESP_LOGE(FS_TAG, "Source directory does not exist: %s", currentSource.c_str());
            return false;
        }

        if (!ensureDirectoryExists(currentDest.c_str())) {
            ESP_LOGE(FS_TAG, "Failed to create destination directory: %s", currentDest.c_str());
            return false;
        }

        File source = LittleFS.open(currentSource.c_str());
        if (!source || !source.isDirectory()) {
            ESP_LOGE(FS_TAG, "Failed to open source directory: %s", currentSource.c_str());
            return false;
        }

        File file = source.openNextFile();
        while (file) {
            String sourceFilePath = currentSource + "/" + file.name();
            String destFilePath = currentDest + "/" + file.name();

            if (file.isDirectory()) {
                dirStack.push_back({sourceFilePath, destFilePath});
            } else {
                if (!copyFile(sourceFilePath.c_str(), destFilePath.c_str())) {
                    ESP_LOGE(FS_TAG, "Failed to move file: %s to %s", sourceFilePath.c_str(), destFilePath.c_str());
                    source.close();
                    return false;
                }
            }
            file = source.openNextFile();
        }

        source.close();
    }

    // Delete the original source directory
    if (!deleteDirectory(sourceDir)) {
        ESP_LOGE(FS_TAG, "Failed to remove source directory after move: %s", sourceDir);
        return false;
    }

    ESP_LOGI(FS_TAG, "Directory moved successfully from %s to %s", sourceDir, destDir);
    return true;
}

/******* TOOLS *******/

String printLittleFSInfo(bool isShort) {
    String result="";
    String line="";
    String s_result="{";

    if (!LittleFS.begin(true)) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return result;
    }

    unsigned long totalBytes = LittleFS.totalBytes();
    unsigned long usedBytes = LittleFS.usedBytes();
    unsigned long freeBytes = totalBytes - usedBytes;

    float usedPercentage = (float)usedBytes / totalBytes * 100;
    float freePercentage = (float)freeBytes / totalBytes * 100;

    line="LittleFS Usage Information:";
    result += line+"\n";
    ESP_LOGI(FS_TAG, line.c_str());

    line="Total size: "+String(totalBytes/1024)+" Kbytes";
    
    result += line+"\n";
    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Total size: %lu bytes", totalBytes);

    line="Used space: "+String(usedBytes/1024)+" Kbytes ("+usedPercentage+"%)";
    result += line+"\n";
    
    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Used space: %lu bytes (%.2f%%)", usedBytes, usedPercentage);

    line="Free space: "+String(freeBytes/1024)+" Kbytes ("+String(freePercentage)+"%)";
    result += line+"\n";

    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Free space: %lu bytes (%.2f%%)", freeBytes, freePercentage);

    s_result+="\"TOTAL\": "+String(totalBytes/1024)+", ";
    s_result+="\"USED\": "+String(usedBytes/1024)+", ";
    s_result+="\"FREE\": "+String(freeBytes/1024)+", ";
    s_result+="\"P_USED\": "+String(usedPercentage)+", ";
    s_result+="\"P_FREE\": "+String(freePercentage)+"}";

    if (isShort) {return s_result;}
    else {return result;}

}

String listLittleFSFilesRecursive(File &dir, const String &indent = "") {
    String result="";
    String line="";
    File file = dir.openNextFile();
    while (file) {
        // Reset watchdog timer périodiquement
        esp_task_wdt_reset();
        
        // Ajouter un delay pour céder du temps CPU
        vTaskDelay(pdMS_TO_TICKS(1));
        if (file.isDirectory()) {
            line=String(indent)+"L__"+String(file.name())+"/";
            line+=listLittleFSFilesRecursive(file, indent + "    ");
        } else {
            line=String(indent)+"L__"+String(file.name())+" ("+String(file.size())+")";
        }
        
        result+="\n"+line;
        file = dir.openNextFile();
    }
    return result;
}

String listLittleFSFiles(String path) {
    String result="";
    ESP_LOGI(FS_TAG, "Listing files in LittleFS:");
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return result;
    }

    File root = LittleFS.open(path);
    if (!root) {
        ESP_LOGE(FS_TAG, "Failed to open root directory");
        return result;
    }
    if (!root.isDirectory()) {
        ESP_LOGE(FS_TAG, "Root is not a directory");
        return result;
    }

    ESP_LOGI(FS_TAG, path.c_str());
    result=listLittleFSFilesRecursive(root);

    return result;
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
    // Lire l'URL de base
    String baseUrl = readFile(baseURL,"https://bitbucket.org/ccoupel/buzzcontrol/raw/main/data");
    if (baseUrl.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire l'URL de base");
        return;
    }

    // Lire la version locale
    float localVersion =readFile("/CURRENT"+VERSION_FILE,"-1").toFloat();
    ESP_LOGI(FS_TAG, "CURRENT Version=%f", localVersion);
    if (localVersion<0) {
        localVersion = readFile(VERSION_FILE, "-1").toFloat();
        ESP_LOGW(FS_TAG, "Local Version=%f", localVersion);
    }
    

    // Télécharger et lire la version distante
    String remoteVersionUrl = baseUrl + VERSION_FILE;
    String tempVersionPath = "/remote_version.txt";
    deleteDirectory(TEMP_DIR.c_str());

    ensureDirectoryExists(TEMP_DIR);
    if (!downloadFile(remoteVersionUrl, tempVersionPath)) {
        ESP_LOGE(FS_TAG, "Échec du téléchargement du fichier version distant");
        return;
    }

    float remoteVersion = atof(readFile(tempVersionPath,"-1").c_str());
    if (remoteVersion<0) {
        ESP_LOGE(FS_TAG, "Impossible de lire la version distante");
        return;
    }

    // Comparer les versions
    if (localVersion >= remoteVersion) {
        ESP_LOGI(FS_TAG, "La version locale est à jour %f / %f", localVersion, remoteVersion);
        LittleFS.remove(tempVersionPath);
        return;
    }

    ESP_LOGI(FS_TAG, "La version locale est a remplacer %f / %f", localVersion, remoteVersion);
    setLedColor(255, 128, 0, true);
    // Télécharger le fichier catalogue distant
    String remoteCatalogUrl = baseUrl + baseFILE;
    String tempCatalogPath = baseFILE+"_remote";
    if (!downloadFile(remoteCatalogUrl, tempCatalogPath)) {
        ESP_LOGE(FS_TAG, "Échec du téléchargement du fichier catalogue distant");
        return;
    }

    String catalogContent = readFile(tempCatalogPath);
    if (catalogContent.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire le fichier catalogue");
        return;
    }

    bool updateSuccess = true;
    int pos = 0;
    while (pos < catalogContent.length()) {
        esp_task_wdt_reset();
        setLedColor(0,128*(pos)/catalogContent.length(),255*(pos)/catalogContent.length(),true);
        int endPos = catalogContent.indexOf('\n', pos);
        if (endPos == -1) endPos = catalogContent.length();
        
        String filePath = catalogContent.substring(pos, endPos);
        filePath.trim();
        pos = endPos + 1;

        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;
        String tempFilePath = TEMP_DIR + "/" + filePath;
        setLedColor(0,0,0,true);

        if (!downloadFile(fileUrl, tempFilePath)) {
            ESP_LOGE(FS_TAG, "Échec du téléchargement ou de la création du répertoire pour %s", fileUrl.c_str());
            updateSuccess = false;
            break;
        }
    }
    setLedColor(128,255,0,true);

    if (updateSuccess) { 
        deleteDirectory("/CURRENT");
        moveDirectory(TEMP_DIR.c_str(),"/CURRENT" );
    }
}

bool isFileExists(String Path) {
    return LittleFS.exists(Path);
}
