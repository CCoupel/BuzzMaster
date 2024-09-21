#include <esp_wifi.h>
#include <WiFi.h>
#include <esp_now.h>
#include <esp_timer.h>

static const char* NTP_TAG = "NTP";

// Structure for sync message
typedef struct sync_message {
    uint64_t epoch_us;     // Epoch time in microseconds
    uint64_t controller_time; // Controller's time when sending the message
    uint32_t sequence;
} sync_message;


uint32_t sequence = 0;

// Broadcast MAC address
uint8_t broadcastAddress[] = {0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF};

// Timer for periodic sync
esp_timer_handle_t periodicTimer;

// Function to send sync message
void sendSyncMessage(void* arg) {
    sync_message msg;
    struct timeval tv_now;
    gettimeofday(&tv_now, nullptr);
    
    msg.epoch_us = (uint64_t)tv_now.tv_sec * 1000000ULL + (uint64_t)tv_now.tv_usec;
    msg.controller_time = esp_timer_get_time();
    msg.sequence = sequence++;
    
    esp_err_t result = esp_now_send(broadcastAddress, (uint8_t *) &msg, sizeof(sync_message));
    if (result != ESP_OK) {
        ESP_LOGE(NTP_TAG,"Error sending the data");
    }
}

// Callback when data is sent
void OnDataSent(const uint8_t *mac_addr, esp_now_send_status_t status) {
    if (status != ESP_NOW_SEND_SUCCESS) {
        ESP_LOGW(NTP_TAG,"Last Packet Send Status: FAIL, Sequence: %u\n", sequence - 1);
    }
}

void setupNTPserver() {
    esp_wifi_start();
    esp_wifi_set_promiscuous(true);
    esp_wifi_set_channel(1, WIFI_SECOND_CHAN_NONE);

    // Init ESP-NOW
    if (esp_now_init() != ESP_OK) {
        ESP_LOGE(NTP_TAG,"Error initializing ESP-NOW");
        return;
    }

    // Register peer
    esp_now_peer_info_t peerInfo = {};

    memcpy(peerInfo.peer_addr, broadcastAddress, 6);
    peerInfo.channel = 0;  
    peerInfo.encrypt = false;
    
    if (esp_now_add_peer(&peerInfo) != ESP_OK) {
        ESP_LOGI(NTP_TAG,"Failed to add peer");
        return;
    }
    
    // Register callback function
    esp_now_register_send_cb(OnDataSent);

    // Create a periodic timer for sync
    const esp_timer_create_args_t periodic_timer_args = {
        .callback = &sendSyncMessage,
        .name = "periodic_sync"
    };
    ESP_ERROR_CHECK(esp_timer_create(&periodic_timer_args, &periodicTimer));
    
    // Start periodic timer (100 ms interval for higher precision)
    ESP_ERROR_CHECK(esp_timer_start_periodic(periodicTimer, 100000));
}

void handleNTPserver() {
}