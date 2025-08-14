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
// Déclarations anticipées
void cleanupTrueParallelTarRestore();
bool initTarGzFS();

// Task handle pour le traitement en arrière-plan
static TaskHandle_t tarProcessingTask = nullptr;

// Stream qui traite VRAIMENT en parallèle - VERSION SÉCURISÉE
class TrueParallelTarProcessor : public Stream {
private:
    std::vector<String> chunkFiles;
    size_t currentReadChunk;
    File currentReadFile;
    size_t totalReceived;
    size_t totalProcessed;
    size_t currentWriteChunkSize;
    File currentWriteFile;
    String tempDir;
    bool streamComplete;
    bool processingStarted;
    bool processingComplete; // AJOUTÉ : Pour coordination sécurisée
    
    // Synchronisation
    SemaphoreHandle_t readMutex;
    SemaphoreHandle_t writeMutex;
    
    // Configuration
    static const size_t CHUNK_SIZE = 8192*4;         // 8KB par chunk
    static const size_t START_PROCESSING_AFTER = 1; // Démarrer après 2 chunks
    
public:
    TrueParallelTarProcessor() : 
        currentReadChunk(0), totalReceived(0), totalProcessed(0), 
        currentWriteChunkSize(0), streamComplete(false), 
        processingStarted(false), processingComplete(false) { // AJOUTÉ processingComplete
        tempDir = "/temp_parallel";
        
        // Créer les semaphores pour thread-safety
        readMutex = xSemaphoreCreateMutex();
        writeMutex = xSemaphoreCreateMutex();
    }
    
    ~TrueParallelTarProcessor() {
        cleanup();
        if (readMutex) vSemaphoreDelete(readMutex);
        if (writeMutex) vSemaphoreDelete(writeMutex);
    }
    
    bool initialize() {
        cleanup();
        
        if (LittleFS.exists(tempDir)) {
            cleanupTempDirectory();
        }
        
        if (!LittleFS.mkdir(tempDir)) {
            ESP_LOGE(FS_TAG, "Impossible de créer: %s", tempDir.c_str());
            return false;
        }
        
        ESP_LOGI(FS_TAG, "TrueParallelTarProcessor initialisé");
        return true;
    }
    
    // Ajouter des données (thread principal - HTTP)
    size_t addData(const uint8_t* data, size_t len) {
        if (streamComplete) return 0;
        
        // Protéger l'écriture
        if (xSemaphoreTake(writeMutex, pdMS_TO_TICKS(100)) != pdTRUE) {
            ESP_LOGW(FS_TAG, "Timeout écriture mutex");
            return 0;
        }
        
        size_t totalWritten = 0;
        
        while (totalWritten < len) {
            yield(); // Reset watchdog
            
            // Créer nouveau chunk si nécessaire
            if (!currentWriteFile || currentWriteChunkSize >= CHUNK_SIZE) {
                if (!createNewWriteChunk()) {
                    ESP_LOGE(FS_TAG, "Impossible de créer chunk d'écriture");
                    break;
                }
            }
            
            // Écrire dans le chunk courant
            size_t remaining = len - totalWritten;
            size_t space = CHUNK_SIZE - currentWriteChunkSize;
            size_t toWrite = min(remaining, space);
            
            size_t written = currentWriteFile.write(data + totalWritten, toWrite);
            if (written != toWrite) {
                ESP_LOGE(FS_TAG, "Erreur écriture: %zu/%zu", written, toWrite);
                break;
            }
            
            totalWritten += written;
            currentWriteChunkSize += written;
            totalReceived += written;
            
            // Fermer le chunk s'il est plein
            if (currentWriteChunkSize >= CHUNK_SIZE) {
                currentWriteFile.close();
                ESP_LOGD(FS_TAG, "Chunk %d fermé: %zu bytes", chunkFiles.size() - 1, currentWriteChunkSize);
                currentWriteChunkSize = 0;
            }
        }
        
        xSemaphoreGive(writeMutex);
        
        // Démarrer le traitement parallèle si conditions remplies
        if (!processingStarted && chunkFiles.size() >= START_PROCESSING_AFTER) {
            startParallelProcessing();
        }
        
        return totalWritten;
    }
    
    void markComplete() {
        streamComplete = true;
        
        // Protéger l'écriture
        if (xSemaphoreTake(writeMutex, pdMS_TO_TICKS(1000)) == pdTRUE) {
            if (currentWriteFile) {
                currentWriteFile.close();
                ESP_LOGI(FS_TAG, "Dernier chunk fermé: %zu bytes", currentWriteChunkSize);
            }
            xSemaphoreGive(writeMutex);
        }
        
        // Démarrer le traitement même avec moins de chunks
        if (!processingStarted) {
            startParallelProcessing();
        }
        
        ESP_LOGI(FS_TAG, "Stream marqué complet: %zu bytes en %d chunks", 
                totalReceived, chunkFiles.size());
    }
    
    // Interface Stream pour TarUnpacker (thread de traitement)
    int available() override {
        if (!processingStarted) return 0;
        
        // Protéger la lecture
        if (xSemaphoreTake(readMutex, pdMS_TO_TICKS(10)) != pdTRUE) {
            return 0; // Timeout court pour éviter de bloquer
        }
        
        int result = 0;
        
        // Vérifier le chunk courant
        if (currentReadFile && currentReadFile.available()) {
            result = currentReadFile.available();
        } else {
            // Essayer d'ouvrir le chunk suivant
            if (openNextReadChunk()) {
                result = currentReadFile ? currentReadFile.available() : 0;
            } else {
                // Plus de chunks disponibles
                if (streamComplete) {
                    ESP_LOGD(FS_TAG, "🏁 Stream terminé - available() retourne 0");
                    result = 0;
                } else {
                    // Décompression plus rapide - on attend
                    ESP_LOGD(FS_TAG, "🏃‍♂️ Décompression plus rapide que réception - attente");
                    result = 0; // Le TarUnpacker va réessayer
                }
            }
        }
        
        xSemaphoreGive(readMutex);
        return result;
    }
    
    int read() override {
        if (!processingStarted) return -1;
        
        // Protéger la lecture
        if (xSemaphoreTake(readMutex, pdMS_TO_TICKS(100)) != pdTRUE) {
            return -1;
        }
        
        int result = -1;
        
        // Lire depuis le chunk courant
        if (currentReadFile && currentReadFile.available()) {
            result = currentReadFile.read();
            if (result != -1) {
                totalProcessed++;
            }
        } else {
            // Chunk courant épuisé - passer au suivant
            if (openNextReadChunk()) {
                result = read(); // Récursion pour lire depuis le nouveau chunk
            }
        }
        
        xSemaphoreGive(readMutex);
        return result;
    }
    
    size_t readBytes(uint8_t* buffer, size_t length) override {
        if (!processingStarted) return 0;
        
        // Protéger la lecture
        if (xSemaphoreTake(readMutex, pdMS_TO_TICKS(200)) != pdTRUE) {
            return 0;
        }
        
        size_t totalRead = 0;
        
        while (totalRead < length) {
            // Lire depuis le chunk courant
            if (currentReadFile && currentReadFile.available()) {
                size_t toRead = min(length - totalRead, (size_t)currentReadFile.available());
                size_t actualRead = currentReadFile.read(buffer + totalRead, toRead);
                totalRead += actualRead;
                totalProcessed += actualRead;
                
                if (actualRead < toRead) break; // Erreur de lecture
            } else {
                // Passer au chunk suivant
                if (!openNextReadChunk()) {
                    break; // Plus de chunks
                }
            }
        }
        
        xSemaphoreGive(readMutex);
        return totalRead;
    }
    
    int peek() override {
        if (!processingStarted) return -1;
        
        if (xSemaphoreTake(readMutex, pdMS_TO_TICKS(50)) != pdTRUE) {
            return -1;
        }
        
        int result = -1;
        if (currentReadFile && currentReadFile.available()) {
            size_t pos = currentReadFile.position();
            result = currentReadFile.read();
            currentReadFile.seek(pos);
        }
        
        xSemaphoreGive(readMutex);
        return result;
    }
    
    void flush() override {
        // Pas d'action nécessaire
    }
    
    // Interface Stream pour écriture (pas utilisé)
    size_t write(uint8_t) override { return 0; }
    size_t write(const uint8_t*, size_t) override { return 0; }
    
    // Informations de diagnostic
    size_t getTotalReceived() const { return totalReceived; }
    size_t getTotalProcessed() const { return totalProcessed; }
    int getChunkCount() const { return chunkFiles.size(); }
    int getCurrentReadChunk() const { return currentReadChunk; }
    
    // AJOUTÉ : Méthodes pour coordination sécurisée
    bool isProcessingComplete() const { return processingComplete; }
    void setProcessingComplete(bool complete) { processingComplete = complete; }
    
    // CORRIGÉ : Cleanup sécurisé contre les crashes
    void cleanup() {
        if (currentWriteFile) currentWriteFile.close();
        if (currentReadFile) currentReadFile.close();
        
        // CORRECTION CRITIQUE : Cleanup sécurisé de la tâche
        if (tarProcessingTask) {
            ESP_LOGI(FS_TAG, "🛑 Arrêt sécurisé de la tâche");
            
            // Signal d'arrêt
            processingComplete = true;
            streamComplete = true;
            
            // Attendre que la tâche se termine naturellement
            int timeout = 3000; // 3 secondes max
            while (tarProcessingTask && timeout > 0) {
                vTaskDelay(pdMS_TO_TICKS(100));
                timeout -= 100;
                yield();
            }
            
            // Si la tâche existe encore, vérifier son état avant suppression
            if (tarProcessingTask) {
                eTaskState taskState = eTaskGetState(tarProcessingTask);
                if (taskState != eDeleted && taskState != eInvalid) {
                    ESP_LOGW(FS_TAG, "⚠️ Force suppression tâche");
                    vTaskDelete(tarProcessingTask);
                } else {
                    ESP_LOGI(FS_TAG, "✅ Tâche déjà supprimée naturellement");
                }
            }
            tarProcessingTask = nullptr;
        }
        
        cleanupTempDirectory();
        
        chunkFiles.clear();
        currentReadChunk = 0;
        totalReceived = 0;
        totalProcessed = 0;
        currentWriteChunkSize = 0;
        streamComplete = false;
        processingStarted = false;
        processingComplete = false;
        
        ESP_LOGI(FS_TAG, "✅ Cleanup sécurisé terminé");
    }
    
private:
    // CORRIGÉ : Démarrage sécurisé avec pile plus grande
    void startParallelProcessing() {
        if (processingStarted) return;
        
        processingStarted = true;
        ESP_LOGI(FS_TAG, "🚀 Démarrage traitement parallèle sécurisé");
        
        // CRITIQUE : Pile plus grande pour éviter stack overflow
        const size_t STACK_SIZE = 16384; // 16KB au lieu de 8KB
        
        BaseType_t result = xTaskCreate(
            parallelProcessingTask,         // Fonction sécurisée
            "SafeTarProcessor",             // Nom
            STACK_SIZE,                     // PILE PLUS GRANDE
            this,                           // Paramètre
            1,                              // Priorité normale
            &tarProcessingTask              // Handle
        );
        
        if (result != pdPASS) {
            ESP_LOGE(FS_TAG, "❌ Échec création tâche sécurisée");
            processingStarted = false;
        } else {
            ESP_LOGI(FS_TAG, "✅ Tâche sécurisée créée avec pile de %d bytes", STACK_SIZE);
        }
    }
    
    // CORRIGÉ : Tâche avec auto-suppression sécurisée
    static void parallelProcessingTask(void* parameter) {
        TrueParallelTarProcessor* processor = static_cast<TrueParallelTarProcessor*>(parameter);
        
        ESP_LOGI(FS_TAG, "🔄 Tâche sécurisée démarrée");
        
        TarUnpacker* unpacker = nullptr;
        bool success = false;
        
        try {
            unpacker = new TarUnpacker();
            if (!unpacker) {
                ESP_LOGE(FS_TAG, "❌ Échec allocation TarUnpacker");
                goto cleanup_and_exit;
            }
            
            unpacker->haltOnError(true); // ARRÊTER sur erreur
            unpacker->setTarVerify(false);
            unpacker->setupFSCallbacks(targzTotalBytesFn, targzFreeBytesFn);
            
            // CORRIGÉ : Callbacks sans capture (pointeurs de fonction simples)
            unpacker->setTarProgressCallback([](uint8_t progress) {
                static uint8_t lastProgress = 255;
                if (progress != lastProgress && progress % 20 == 0) {
                    ESP_LOGI(FS_TAG, "🔄 Extraction: %d%%", progress);
                    lastProgress = progress;
                    yield();
                }
            });
            
            unpacker->setTarStatusProgressCallback([](const char* name, size_t size, size_t total_unpacked) {
                ESP_LOGI(FS_TAG, "📁 Extrait: %s (%zu bytes)", name, size);
                yield();
            });
            
            // Attendre des données
            int waitCount = 0;
            while (processor->available() == 0 && waitCount < 100 && !processor->processingComplete) {
                vTaskDelay(pdMS_TO_TICKS(100));
                waitCount++;
                yield();
            }
            
            if (processor->processingComplete) {
                ESP_LOGI(FS_TAG, "🛑 Arrêt demandé avant extraction");
                goto cleanup_and_exit;
            }
            
            ESP_LOGI(FS_TAG, "🎯 Début extraction");
            success = unpacker->tarStreamExpander(processor, 0, tarGzFS, "/files");
            
            if (success) {
                ESP_LOGI(FS_TAG, "✅ Extraction terminée avec succès");
            } else {
                int errorCode = unpacker->tarGzGetError();
                ESP_LOGE(FS_TAG, "❌ Erreur extraction, code: %d", errorCode);
            }
            
        } catch (...) {
            ESP_LOGE(FS_TAG, "💥 EXCEPTION dans la tâche!");
        }
        
    cleanup_and_exit:
        if (unpacker) {
            delete unpacker;
            unpacker = nullptr;
        }
        
        processor->processingComplete = true;
        
        ESP_LOGI(FS_TAG, "🏁 Tâche terminée - auto-suppression");
        
        // CRITIQUE : Délai avant auto-suppression pour éviter race condition
        vTaskDelay(pdMS_TO_TICKS(200));
        
        // CRITIQUE : Marquer que la tâche va s'auto-supprimer
        tarProcessingTask = nullptr; // Important: faire ça AVANT vTaskDelete
        
        // Auto-suppression sécurisée
        vTaskDelete(nullptr);
    }
    
    bool createNewWriteChunk() {
        if (currentWriteFile) {
            currentWriteFile.close();
        }
        
        String chunkPath = tempDir + "/chunk_" + String(chunkFiles.size()) + ".bin";
        currentWriteFile = LittleFS.open(chunkPath, "w");
        
        if (!currentWriteFile) {
            ESP_LOGE(FS_TAG, "Impossible de créer: %s", chunkPath.c_str());
            return false;
        }
        
        chunkFiles.push_back(chunkPath);
        currentWriteChunkSize = 0;
        
        ESP_LOGD(FS_TAG, "Nouveau chunk créé: %s", chunkPath.c_str());
        return true;
    }
    
    bool openNextReadChunk() {
        // Fermer et supprimer le chunk courant s'il existe
        if (currentReadFile) {
            currentReadFile.close();
            
            // Supprimer le chunk traité (avec protection)
            if (currentReadChunk > 0) {
                String oldChunkPath = chunkFiles[currentReadChunk - 1];
                if (LittleFS.remove(oldChunkPath)) {
                    ESP_LOGD(FS_TAG, "🗑️ Chunk supprimé: %s", oldChunkPath.c_str());
                } else {
                    ESP_LOGW(FS_TAG, "Impossible de supprimer: %s", oldChunkPath.c_str());
                }
            }
        }
        
        // Vérifier s'il y a un chunk suivant disponible
        if (currentReadChunk >= chunkFiles.size()) {
            if (streamComplete) {
                ESP_LOGI(FS_TAG, "🏁 Fin de stream atteinte - plus de chunks à attendre");
                return false; // Vraiment fini
            } else {
                // Attendre que plus de chunks arrivent
                ESP_LOGD(FS_TAG, "⏳ Attente de nouveaux chunks... (chunk %d/%d)", 
                        currentReadChunk, chunkFiles.size());
                
                // Attente intelligente avec timeout
                int waitCount = 0;
                const int maxWaitCycles = 100; // 5 secondes max (100 * 50ms)
                
                while (currentReadChunk >= chunkFiles.size() && !streamComplete && waitCount < maxWaitCycles) {
                    vTaskDelay(pdMS_TO_TICKS(50));
                    waitCount++;
                    
                    // Logger périodiquement pour debug
                    if (waitCount % 20 == 0) {
                        ESP_LOGI(FS_TAG, "⏱️ Attente chunks depuis %d secondes (reçu: %zu, traité: %zu)", 
                                waitCount / 20, totalReceived, totalProcessed);
                    }
                }
                
                if (waitCount >= maxWaitCycles) {
                    ESP_LOGW(FS_TAG, "⚠️ Timeout attente chunks - arrêt du traitement");
                    return false;
                }
                
                // Re-vérifier après l'attente
                if (currentReadChunk >= chunkFiles.size()) {
                    return false;
                }
            }
        }
        
        // Vérifier si le chunk est prêt (pas en cours d'écriture)
        if (currentReadChunk == chunkFiles.size() - 1) {
            // C'est le dernier chunk - il pourrait être en cours d'écriture
            if (!streamComplete) {
                ESP_LOGD(FS_TAG, "📝 Chunk en cours d'écriture - attente...");
                
                // Attendre que l'écriture soit terminée
                int writeWaitCount = 0;
                const int maxWriteWait = 20; // 1 seconde max
                
                while (!streamComplete && writeWaitCount < maxWriteWait) {
                    vTaskDelay(pdMS_TO_TICKS(50));
                    writeWaitCount++;
                }
                
                if (!streamComplete && writeWaitCount >= maxWriteWait) {
                    ESP_LOGD(FS_TAG, "📝 Lecture du chunk en cours d'écriture (risqué)");
                }
            }
        }
        
        // Ouvrir le chunk suivant
        String chunkPath = chunkFiles[currentReadChunk];
        currentReadFile = LittleFS.open(chunkPath, "r");
        
        if (!currentReadFile) {
            ESP_LOGE(FS_TAG, "❌ Impossible d'ouvrir: %s", chunkPath.c_str());
            return false;
        }
        
        ESP_LOGD(FS_TAG, "📖 Chunk ouvert: %s (%d bytes)", 
                chunkPath.c_str(), currentReadFile.size());
        
        currentReadChunk++;
        return true;
    }
    
    void cleanupTempDirectory() {
        if (!LittleFS.exists(tempDir)) return;
        
        File dir = LittleFS.open(tempDir);
        if (dir && dir.isDirectory()) {
            File file = dir.openNextFile();
            while (file) {
                String filePath = tempDir + "/" + String(file.name());
                file.close();
                
                LittleFS.remove(filePath);
                file = dir.openNextFile();
                yield();
            }
            dir.close();
        }
        
        LittleFS.rmdir(tempDir);
        ESP_LOGI(FS_TAG, "Répertoire temporaire nettoyé: %s", tempDir.c_str());
    }
};

// Variables globales
static TrueParallelTarProcessor* parallelProcessor = nullptr;
static bool parallelRestoreInProgress = false;
static bool parallelRestoreSuccess = false;
static bool tarGzFSInitialized = false;

// Initialise tarGzFS (requis par ESP32-targz)
bool initTarGzFS() {
    if (tarGzFSInitialized) {
        return true;
    }
    
    if (!tarGzFS.begin()) {
        ESP_LOGE(FS_TAG, "Échec initialisation tarGzFS");
        return false;
    }
    
    tarGzFSInitialized = true;
    ESP_LOGI(FS_TAG, "tarGzFS initialisé");
    return true;
}

// CORRIGÉ : Cleanup sécurisé
void cleanupTrueParallelTarRestore() {
    ESP_LOGI(FS_TAG, "🧹 Début cleanup sécurisé");
    
    if (parallelProcessor) {
        parallelProcessor->cleanup(); // Cette méthode est maintenant sécurisée
        
        // Délai de sécurité avant suppression de l'objet
        vTaskDelay(pdMS_TO_TICKS(500));
        
        delete parallelProcessor;
        parallelProcessor = nullptr;
    }
    
    parallelRestoreInProgress = false;
    ESP_LOGI(FS_TAG, "✅ Cleanup sécurisé terminé");
}

// Initialise le restore streaming
bool initTrueParallelTarRestore() {
    if (parallelRestoreInProgress) {
        ESP_LOGW(FS_TAG, "Un restore parallèle est déjà en cours");
        return false;
    }
    
    cleanupTrueParallelTarRestore();
    yield();
    
    if (!initTarGzFS()) {
        ESP_LOGE(FS_TAG, "Impossible d'initialiser tarGzFS");
        return false;
    }
    
    // Créer le processeur parallèle
    parallelProcessor = new TrueParallelTarProcessor();
    if (!parallelProcessor || !parallelProcessor->initialize()) {
        ESP_LOGE(FS_TAG, "Échec initialisation TrueParallelTarProcessor");
        cleanupTrueParallelTarRestore();
        return false;
    }
    
    ensureDirectoryExists("/files");
    
    parallelRestoreInProgress = true;
    parallelRestoreSuccess = false;
    
    ESP_LOGI(FS_TAG, "🚀 Restore TAR parallèle RÉEL initialisé");
    return true;
}

size_t processTrueParallelTarChunk(const uint8_t* data, size_t len) {
    if (!parallelProcessor || !parallelRestoreInProgress) {
        ESP_LOGE(FS_TAG, "Restore parallèle non initialisé");
        return 0;
    }
    
    yield();
    return parallelProcessor->addData(data, len);
}

// CORRIGÉ : Finalisation avec délais de sécurité
bool finalizeTrueParallelTarRestore() {
    if (!parallelProcessor || !parallelRestoreInProgress) {
        ESP_LOGW(FS_TAG, "Restore parallèle non initialisé pour finalisation");
        return false;
    }
    
    yield();
    
    parallelProcessor->markComplete();
    
    ESP_LOGI(FS_TAG, "⏳ Attente fin traitement...");
    
    // Attente avec timeout
    int timeout = 120000; // 60 secondes max
    while (!parallelProcessor->isProcessingComplete() && timeout > 0) {
        vTaskDelay(pdMS_TO_TICKS(250));
        timeout -= 250;
        yield();
        
        if (timeout % 5000 == 0) {
            ESP_LOGI(FS_TAG, "⏱️ Attente... (timeout: %d ms)", timeout);
        }
    }
    
    bool success = parallelProcessor->isProcessingComplete();
    
    if (success) {
        ESP_LOGI(FS_TAG, "✅ Restore parallèle terminé avec succès");
        ESP_LOGI(FS_TAG, "📊 Statistiques: %zu reçus, %zu traités", 
                parallelProcessor->getTotalReceived(), parallelProcessor->getTotalProcessed());
        parallelRestoreSuccess = true;
    } else {
        ESP_LOGE(FS_TAG, "❌ Timeout ou erreur restore parallèle");
        parallelRestoreSuccess = false;
    }
    
    parallelRestoreInProgress = false;
    
    // CRITIQUE : Délai avant nettoyage pour stabiliser
    vTaskDelay(pdMS_TO_TICKS(1000)); // 1 seconde de délai
    
    return success;
}

bool isTrueParallelRestoreInProgress() {
    return parallelRestoreInProgress;
}

String getTrueParallelRestoreStatus() {
    if (!parallelRestoreInProgress) {
        if (parallelRestoreSuccess) {
            return "Restore parallèle terminé avec succès";
        } else {
            return "Aucun restore en cours";
        }
    }
    
    if (parallelProcessor) {
        return "Restore parallèle en cours... (" + 
               String(parallelProcessor->getTotalProcessed()) + 
               "/" + String(parallelProcessor->getTotalReceived()) + " bytes, " +
               String(parallelProcessor->getChunkCount()) + " chunks)";
    }
    
    return "Restore parallèle en cours...";
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
    String baseUrl = configManager.getUpdateBaseURL(); 
    String VersionFile= configManager.getVersionFile();
    //readFile(baseURL,"https://bitbucket.org/ccoupel/buzzcontrol/raw/main/data");
    if (baseUrl.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire l'URL de base");
        return;
    }

    // Lire la version locale
    float localVersion =readFile("/CURRENT"+VersionFile,"-1").toFloat();
    ESP_LOGI(FS_TAG, "CURRENT Version=%f", localVersion);
    if (localVersion<0) {
        localVersion = readFile(VersionFile, "-1").toFloat();
        ESP_LOGW(FS_TAG, "Local Version=%f", localVersion);
    }
    

    // Télécharger et lire la version distante
    String remoteVersionUrl = baseUrl + VersionFile;
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
