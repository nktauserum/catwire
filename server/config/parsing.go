package config

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
)

const (
	BEGIN_STATE = iota
	MAIN_STATE
	CLIENT_STATE
)

func parseField(line []rune) (string, string, error) {
	for i, c := range line {
		if c == '=' {
			return string(line[:i]), string(line[i+2 : len(line)-1]), nil
		}
	}

	return "", "", fmt.Errorf("malformed string")
}

func ParseConfig(path string) (Config, error) {
	var config Config

	file, err := os.Open(path)
	if err != nil {
		return config, err
	}

	state := BEGIN_STATE
	client := Client{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		str := []rune(scanner.Text())
		if str[0] == '[' {
			if scanner.Text() == "[main]" {
				state = MAIN_STATE
				continue
			}

			if state == CLIENT_STATE {
				config.Clients = append(config.Clients, client)
				client = Client{}
			}

			state = CLIENT_STATE
			client.Name = string(str[1:len(str)-1])

			continue
		}

		key, value, err := parseField(str)
		if err != nil {
			return config, err
		}

		switch state {
		case MAIN_STATE:
			switch key {
			case "seed":
				seed, err := base64.StdEncoding.DecodeString(value)
				if err != nil {
					return config, err
				}
				copy(config.Seed[:], seed)

			case "address":
				addr, err := net.ResolveUDPAddr("udp", value)
				if err != nil {
					return config, err
				}
				config.Address = *addr
			}

		case CLIENT_STATE:
			switch key {
			case "address":
				client.LocalAddr = net.ParseIP(value)
			case "public_key":
				key, err := base64.StdEncoding.DecodeString(value)
				if err != nil {
					return config, err
				}
				copy(client.PublicKey[:], key)
			}
		case BEGIN_STATE:
			return config, fmt.Errorf("Config file should start with [main]")
		}
	}
	if err := scanner.Err(); err != nil {
		return config, err
	}

	config.Clients = append(config.Clients, client)

	return config, nil
}
