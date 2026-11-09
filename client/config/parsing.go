package config

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
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

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		str := []rune(scanner.Text())
		if str[0] == '[' {
			continue
		}

		key, value, err := parseField(str)
		if err != nil {
			return config, err
		}

		switch key {
		case "seed":
			seed, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return config, err
			}
			copy(config.Seed[:], seed)

		case "server_address":
			addr, err := net.ResolveUDPAddr("udp", value)
			if err != nil {
				return config, err
			}
			config.ServerAddr = *addr
		}
	}
	if err := scanner.Err(); err != nil {
		return config, err
	}

	return config, nil
}
