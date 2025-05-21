#pragma once

#include <DNSServer.h>
#include <ESPmDNS.h>

static const char* DNS_TAG = "DNS";
DNSServer dnsServer;
void setupDNS() {
    dnsServer.setErrorReplyCode(DNSReplyCode::NoError);
    dnsServer.start(53, "dns.msftncsi.com", IPAddress(131,107,255,255));
    dnsServer.start(53, "*", apIP);
    ESP_LOGI(DNS_TAG, "Serveur DNS démarré");

}

void setupmDNS() {
    if(MDNS.begin("buzzcontrol")) {
        ESP_LOGI(WIFI_TAG, "MDNS responder started");
        
        // Enregistrer les services
        MDNS.addService("buzzcontrol", "tcp", configManager.getControllerPort());
        MDNS.addService("http", "tcp", 80);  // HTTP is always on port 80
        MDNS.addService("sock", "tcp", configManager.getControllerPort());
        
        ESP_LOGI(DNS_TAG, "Serveur mDNS démarré");

    }
}


void setupDNSServer() {

    // Configuration du mDNS
        setupmDNS();
        setupDNS();
        // Show IPs for debug
        ESP_LOGI(DNS_TAG, "AP IP: %s", apIP.toString().c_str());
        ESP_LOGI(DNS_TAG, "STA IP: %s", WiFi.localIP().toString().c_str());
}


