//const char* WIFI_SSID     = "buzzmaster";
//const char* WIFI_PASSWORD = "BuzzMaster";

const char* ssid     = "CC-Home2";
const char* password = "GenericPassword";

const char* apSSID = "buzzmaster";
const char* apPASSWORD = "BuzzMaster";


/* **** UTILITY FUNCTIONS *** */
constexpr unsigned int hash(const char* str, int h = 0) {
    return !str[h] ? 5381 : (hash(str, h+1) * 33) ^ str[h];
}