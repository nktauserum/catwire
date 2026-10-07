#include "config.hpp"

#include <fstream>
#include <string.h>
#include <algorithm>
#include <arpa/inet.h>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>

#include "../../common/macro.h"

enum State {
    BEGIN,
    EXPECT_MAIN_FIELD,
    EXPECT_CLIENT_FIELD,
};

static std::pair<std::string, std::string> parse_field(const char* field) {
    std::string s;
    size_t len = strlen(field);
    for (size_t i = 0; i < len; ++i) {
        if (field[i] == '=') return std::pair<std::string, std::string>(s.assign(field, i), std::string(&field[i+1]));
    }

    panic("bad field");
}

static bool compareClientsByAddr(const Client& a, const Client& b) {
    return ntohl(a.local_addr) < ntohl(b.local_addr);
}

#define handle_string(s) s.erase(s.size()-1).erase(0, 1)
#define handle_int(s) std::stoi(s)

Config::Config(const char* filename) {
    std::ifstream f(filename);

    std::string s;
    State state = BEGIN;
    Client current_client = {0};

    while (std::getline(f, s)) {
        if (s == "") continue;
        if (s[0] == '[') {
            if (s == "[main]") state = EXPECT_MAIN_FIELD;
            else {
                if (state == EXPECT_CLIENT_FIELD) {
                    clients.push_back(current_client);
                    current_client = {0};
                }
                state = EXPECT_CLIENT_FIELD;
            }
            continue;
        }

        auto val = parse_field(s.c_str());

        switch (state) {
            case BEGIN:
                panic("panic: config should start with [main]");

            case EXPECT_MAIN_FIELD:
                if (val.first == "seed") { 
                    auto key_s = handle_string(val.second);
                    boost::beast::detail::base64::decode(&seed, key_s.c_str(), key_s.size());

                } else if (val.first == "port") {
                    port = handle_int(val.second);
                } else continue;

                break;

            case EXPECT_CLIENT_FIELD:
                if (val.first == "publicKey") {
                    auto publicKey = handle_string(val.second);
                    boost::beast::detail::base64::decode(&current_client.publicKey, publicKey.c_str(), publicKey.size());

                } else if (val.first == "address") {
                    auto addr = handle_string(val.second);
                    if (inet_pton(AF_INET, addr.c_str(), &current_client.local_addr) < 0) panic("error parsing client address"); // big endian
                } else continue;

                break;
        }
    }

    clients.push_back(current_client);

    std::sort(clients.begin(), clients.end(), compareClientsByAddr);

    std::cout << "[INFO]: Parse " << filename << ": " << clients.size() << " client(s)" << std::endl;
}

