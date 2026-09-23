#include <fstream>
#include <cstring>
#include <stdexcept>
#include <arpa/inet.h>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>

#include "../common/types.h"

static std::pair<std::string, std::string> parse_field(const char* field) {
    std::string s;
    int len = strlen(field);
    for (int i = 0; i < len; ++i) {
        if (field[i] == '=') return std::pair<std::string, std::string>(s.assign(field, i), std::string(&field[i+1]));
    }

    throw std::runtime_error("bad field");
}

#define handle_string(s) s.erase(s.size()-1).erase(0, 1)
#define handle_int(s) std::stoi(s)

struct Config {
    u8 privateKey[32];
    char server_addr[16];
    u16 server_port = 0;

    static Config load_from_file(const char* filename) {
        Config config;
        std::ifstream f(filename);

        std::string s;
        while (std::getline(f, s)) {
            if (s == "") continue;
            if (s[0] == '[') {
                continue;
            }

            auto val = parse_field(s.c_str());

            if (val.first == "privateKey") {         
                auto key_s = handle_string(val.second);
                boost::beast::detail::base64::decode(&config.privateKey, key_s.c_str(), key_s.size());
            } else if (val.first == "server_port") {
                config.server_port = handle_int(val.second);
            } else if (val.first == "server_addr") {
                auto str = handle_string(val.second);
                strncpy(config.server_addr, str.c_str(), str.size());
            } else continue;
        }
        return config;
    }
};

