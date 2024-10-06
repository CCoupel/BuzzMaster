#pragma once

#include <Arduino.h>
#include <WiFiUdp.h>
#include <esp_log.h>
#include <time.h>

class CustomLogger {
private:
    static WiFiUDP udp;
    static uint16_t logPort;
    static bool initialized;
    static IPAddress broadcastIP;

    static const char* getLevelColor(esp_log_level_t level) {
        switch (level) {
            case ESP_LOG_ERROR:   return "\033[1;31m"; // Rouge gras
            case ESP_LOG_WARN:    return "\033[1;33m"; // Jaune gras
            case ESP_LOG_INFO:    return "\033[1;32m"; // Vert gras
            case ESP_LOG_DEBUG:   return "\033[1;36m"; // Cyan gras
            case ESP_LOG_VERBOSE: return "\033[1;35m"; // Magenta gras
            default:              return "\033[0m";    // Réinitialisation
        }
    }

    static const char* getLevelString(esp_log_level_t level) {
        switch (level) {
            case ESP_LOG_ERROR:   return "E";
            case ESP_LOG_WARN:    return "W";
            case ESP_LOG_INFO:    return "I";
            case ESP_LOG_DEBUG:   return "D";
            case ESP_LOG_VERBOSE: return "V";
            default:              return "U";
        }
    }

    static void getTimestamp(char* buffer, size_t bufferSize) {
        struct tm timeinfo;
        if(!getLocalTime(&timeinfo)){
            snprintf(buffer, bufferSize, "Time not set");
            return;
        }
        strftime(buffer, bufferSize, "%Y-%m-%d %H:%M:%S", &timeinfo);
    }

public:
    static void init(uint16_t port) {
        logPort = port;
        initialized = true;
        broadcastIP = IPAddress(255, 255, 255, 255);  // Adresse de broadcast
        configTime(0, 0, "pool.ntp.org");  // Configurez le fuseau horaire et le serveur NTP si nécessaire
//        esp_log_set_vprintf(customLogFunction);
    }

    static int customLogFunction(esp_log_level_t level, const char* tag, const char* format, va_list args)  {
        char timestampBuffer[20];
        getTimestamp(timestampBuffer, sizeof(timestampBuffer));

        char logBuffer[1024];
        int prefixLen = snprintf(logBuffer, sizeof(logBuffer), 
                                "%s%s \033[1;37m%s\033[0m %s(\033[1m%s\033[0m) ", 
                                getLevelColor(level), getLevelString(level),
                                timestampBuffer,
                                getLevelColor(level), tag);

        // Formater le message de log
        int messageLen = vsnprintf(logBuffer + prefixLen, sizeof(logBuffer) - prefixLen, format, args);
        
        // Ajouter le code de réinitialisation de couleur et le saut de ligne
        strcat(logBuffer, "\033[0m\n");

        // Calculer la longueur totale
        int totalLen = prefixLen + messageLen + 5; // 5 pour "\033[0m\n"

        // Log to console
        Serial.print(logBuffer);

        // Log to UDP broadcast
        if (initialized) {
            udp.beginPacket(broadcastIP, logPort);
            udp.write((const uint8_t*)logBuffer, totalLen);
            udp.endPacket();
        }

        return totalLen - 1; // -1 pour ne pas compter le '\n' final

    }

    static void log(esp_log_level_t level, const char* tag, const char* format, ...) {
        va_list args;
        va_start(args, format);
        customLogFunction(level, tag, format, args);
        va_end(args);
    }
};

WiFiUDP CustomLogger::udp;
uint16_t CustomLogger::logPort;
bool CustomLogger::initialized = false;
IPAddress CustomLogger::broadcastIP;

#define CUSTOM_LOGE(tag, format, ...) CustomLogger::log(ESP_LOG_ERROR, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGW(tag, format, ...) CustomLogger::log(ESP_LOG_WARN, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGI(tag, format, ...) CustomLogger::log(ESP_LOG_INFO, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGD(tag, format, ...) CustomLogger::log(ESP_LOG_DEBUG, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGV(tag, format, ...) CustomLogger::log(ESP_LOG_VERBOSE, tag, format, ##__VA_ARGS__)

// Surcharge des macros ESP_LOGx
#undef ESP_LOGE
#undef ESP_LOGW
#undef ESP_LOGI
#undef ESP_LOGD
#undef ESP_LOGV

#define ESP_LOGE(tag, format, ...) CUSTOM_LOGE(tag, format, ##__VA_ARGS__)
#define ESP_LOGW(tag, format, ...) CUSTOM_LOGW(tag, format, ##__VA_ARGS__)
#define ESP_LOGI(tag, format, ...) CUSTOM_LOGI(tag, format, ##__VA_ARGS__)
#define ESP_LOGD(tag, format, ...) CUSTOM_LOGD(tag, format, ##__VA_ARGS__)
#define ESP_LOGV(tag, format, ...) CUSTOM_LOGV(tag, format, ##__VA_ARGS__)