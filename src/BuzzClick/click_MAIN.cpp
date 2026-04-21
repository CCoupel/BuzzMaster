#include "click_includes.h"
#include "click_nvsConfig.h"
#include "click_WifiManager.h"
#include "click_usbConfig.h"

#include "Common/CustomLogger.h"
#include "Common/led.h"

#include "esp_task_wdt.h"
#include "click_serverConnection.h"
#include "click_otaManager.h"

static const char* MAIN_TAG = "MAIN";
const uint16_t logPort = 8889;  // Port UDP pour les logs

// USB-only mode: skip WiFi/logger/buttons, only run AT commands on serial
bool usbOnlyMode = false;

// Boot button: hold red buzzer button (GPIO 6 = ROUGE, broche 5-6) for 3 seconds at boot
// to enter USB reconfigure mode. Factory reset is available via AT+FACTORY command.
const int FACTORY_RESET_PIN = 6;
const int FACTORY_RESET_HOLD_MS = 3000;


void printPinInfo() {
  #if defined(ESP32)
    ESP_LOGI(MAIN_TAG, "RGB pin: %d", RGB_BUILTIN);
  #endif
  ESP_LOGI(MAIN_TAG, "LED pin: %d", LED_BUILTIN);
  #if defined(ESP32)
    ESP_LOGI(MAIN_TAG, "NEO pin: %d", PIN_NEOPIXEL);
  #endif
}

void watchdogTask(void *pvParameters) {
    // Anti-freeze watchdog: 30s timeout — if the task stops feeding it, the chip reboots
    esp_task_wdt_init(30, true);
    esp_task_wdt_add(NULL);

    for (;;) {
        esp_task_wdt_reset();
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}

// Check if buzzer button is held at boot.
// If held for 3s, CLEAR NVS and enter USB reconfigure mode.
// The user can then reconfigure via AT commands.
// Returns true if USB reconfigure mode was activated.
bool checkBootButton() {
  pinMode(FACTORY_RESET_PIN, INPUT_PULLUP);
  pinMode(pin_gnd, OUTPUT);
  digitalWrite(pin_gnd, LOW);

  // Check if button is pressed at boot (LOW = pressed with pullup)
  if (digitalRead(FACTORY_RESET_PIN) == LOW) {
    ESP_LOGI(MAIN_TAG, "Button detected at boot, hold for %d ms to enter USB reconfigure mode...", FACTORY_RESET_HOLD_MS);

    unsigned long startTime = millis();
    bool blinkState = false;

    // Blue blinking while waiting for 3s hold
    while (digitalRead(FACTORY_RESET_PIN) == LOW) {
      unsigned long elapsed = millis() - startTime;

      // Blink blue LED at 4Hz
      blinkState = !blinkState;
      if (blinkState) {
        setLedColor(0, 0, 255, true);
      } else {
        setLedColor(0, 0, 0, true);
      }

      if (elapsed >= FACTORY_RESET_HOLD_MS) {
        // Button held long enough - CLEAR NVS and enter USB reconfigure mode
        setLedColor(255, 0, 255, true);  // Magenta flash = mode confirmed
        delay(500);

        // FIX: Clear NVS config to persist factory reset
        nvsClearConfig();

        ESP_LOGI(MAIN_TAG, "NVS cleared, rebooting to apply factory reset...");
        delay(500);  // Let log flush before reboot
        ESP.restart();  // Force immediate reboot to re-read empty NVS

        usbOnlyMode = true;
        setLedColor(255, 0, 255, true);  // Magenta = USB config mode
        return true;  // Caller handles WiFi.OFF + log suppression + USB_READY
      }

      delay(125);
    }

    // Button released too early
    ESP_LOGI(MAIN_TAG, "Button released early, normal boot");
    setLedColor(0, 0, 0, true);
  }

  return false;
}

void setup()
{
  // 1. Serial first
  Serial.begin(115200);
  Serial.println("STARTING!!");

  // 2. Init LED early (required for checkBootButton visual feedback)
  initLED();

  // 3. Check boot button IMMEDIATELY after LED init.
  //    No watchdog, no WiFi, no logs enabled yet.
  if (checkBootButton()) {
    // Disable WiFi radio before it auto-starts
    WiFi.mode(WIFI_OFF);
    WiFi.disconnect(true);
    // Suppress ALL logs (no watchdog created yet, so nothing else logs)
    esp_log_level_set("*", ESP_LOG_NONE);
    Serial.println("USB_READY");
    return;  // No watchdog, no WiFi, no logger - only AT commands in loop()
  }

  // 4. Normal boot path: timezone + logs
  setenv("TZ", "UTC-1", 1);
  tzset();
  struct timeval tv = { .tv_sec = 0 };
  settimeofday(&tv, NULL);

  esp_log_level_set("*", ESP_LOG_INFO);
  ESP_LOGI(MAIN_TAG, "Starting up...");

  // === BOOT PHASE 1: RED 1/2 - Boot start ===
  setLedColor(255, 0, 0, true, 0, 2);
  ESP_LOGI(MAIN_TAG, "Boot phase: RED 1/2 (boot start)");

  // 5. Load NVS config BEFORE watchdog - if NVS empty, enter USB mode
  //    without ever creating the watchdog task.
  bool hasNvsConfig = nvsLoadConfig();
  ESP_LOGI(MAIN_TAG, "NVS config: %s", hasNvsConfig ? "FOUND" : "EMPTY (using defaults)");

  if (!hasNvsConfig) {
    usbOnlyMode = true;
    setLedColor(255, 0, 255, true);  // Magenta = USB config mode
    WiFi.mode(WIFI_OFF);
    WiFi.disconnect(true);
    esp_log_level_set("*", ESP_LOG_NONE);
    Serial.println("USB_READY");
    return;  // No watchdog, no WiFi - only AT commands in loop()
  }

  // 6. Valid NVS config: create watchdog and continue normal boot
  xTaskCreate(
      watchdogTask,
      "WatchdogTask",
      2048,
      NULL,
      configMAX_PRIORITIES - 1,
      NULL
  );

  ESP_LOGI(MAIN_TAG, "STARTING:");
  printPinInfo();

  setupWifi();
  yield();

  CustomLogger::init(logPort);
  ESP_LOGI(MAIN_TAG, "BOOTING Version: %s", String(VERSION));

  attachButtons();
}

void loop() {
  // Process USB AT commands (non-blocking)
  usbConfigProcess();

  if (usbOnlyMode) {
    // Magenta blinking LED: slow (1Hz) idle, fast (4Hz) for 2s after AT command
    static unsigned long lastBlink = 0;
    static bool blinkState = false;
    bool atActive = (millis() - lastAtCommandTime < 2000);
    unsigned long interval = atActive ? 125 : 500;

    if (millis() - lastBlink > interval) {
      blinkState = !blinkState;
      if (blinkState) {
        setLedColor(255, 0, 255, true);  // Magenta ON
      } else {
        setLedColor(0, 0, 0, true);      // OFF
      }
      lastBlink = millis();
    }
    return;
  }

  // If no team assigned AND boot is complete, run gray rotation animation
  // During boot, LED patterns are managed by the boot sequence
  if (bootComplete && !hasTeamAssignment) {
    updateGrayRotation();
  }

  // LED blink animation (server-driven LED_SET with EFFECT="BLINK")
  manageLedBlink();

#ifdef USE_WEBSOCKET
  // Poll WebSocket incoming messages and reconnect if disconnected
  pollWebSocket();
  checkWebSocketConnection();
#endif

  // Normal mode: manage button presses
  manageButtonMessages();
}
