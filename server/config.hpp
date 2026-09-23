#include <fstream>
#include <string.h>
#include <vector>
#include <algorithm>
#include <arpa/inet.h>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>

#include "../common/types.h"
#include "../common/macro.h"
#include "../common/models.h"

enum State {
    EXPECT_MAIN_FIELD,
    EXPECT_CLIENT_FIELD,
};

static std::pair<std::string, std::string> parse_field(const char* field) {
    std::string s;
    size_t len = strlen(field);
    for (int i = 0; i < len; ++i) {
        if (field[i] == '=') return std::pair<std::string, std::string>(s.assign(field, i), std::string(&field[i+1]));
    }

    throw panic("bad field");
}

bool compareSessionsByAddr(const Session& a, const Session& b) {
    return ntohl(a.local_addr) < ntohl(b.local_addr);
}

#define handle_string(s) s.erase(s.size()-1).erase(0, 1)
#define handle_int(s) std::stoi(s)

struct Config {
    u8 seed[32];
    u16 port;

    std::vector<Session> clients;

    static Config load_from_file(const char* filename) {
        Config config;
        std::ifstream f(filename);

        std::string s;
        State state;
        Session current_session;

        while (std::getline(f, s)) {
            if (s == "") continue;
            if (s[0] == '[') {
                if (s == "[main]") state = EXPECT_MAIN_FIELD;
                else {
                    if (state == EXPECT_CLIENT_FIELD) {
                        config.clients.push_back(current_session);
                        current_session = {0};
                    }
                    state = EXPECT_CLIENT_FIELD;
                }
                continue;
            }

            auto val = parse_field(s.c_str());

            switch (state) {
                case EXPECT_MAIN_FIELD:
                    if (val.first == "seed") { 
                        auto key_s = handle_string(val.second);
                        boost::beast::detail::base64::decode(&config.seed, key_s.c_str(), key_s.size());

                    } else if (val.first == "port") {
                        config.port = handle_int(val.second);
                    } else continue;

                    break;

                case EXPECT_CLIENT_FIELD:
                    if (val.first == "publicKey") {
                        auto publicKey = handle_string(val.second);
                        boost::beast::detail::base64::decode(&current_session.publicKey, publicKey.c_str(), publicKey.size());

                    } else if (val.first == "address") {
                        auto addr = handle_string(val.second);
                        if (inet_pton(AF_INET, addr.c_str(), &current_session.local_addr) < 0) throw panic("error parsing client address"); // big endian
                    } else continue;

                    break;
            }
        }
    
        config.clients.push_back(current_session);

        std::sort(config.clients.begin(), config.clients.end(), compareSessionsByAddr);
        return config;
    }
};

