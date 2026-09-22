CXX = g++
CXXFLAGS = -std=c++23 -Wall -Wextra -pedantic
LIBS = -lsodium -lstdc++

BUILD_DIR = build
SRC = main.cpp
OBJ = $(addprefix $(BUILD_DIR)/, $(SRC:.cpp=.o))
TARGET = $(BUILD_DIR)/cl

all: $(TARGET)

$(BUILD_DIR):
	mkdir -p $@

$(TARGET): $(OBJ) | $(BUILD_DIR)
	$(CXX) -o $@ $^ $(LIBS)

$(BUILD_DIR)/%.o: %.cpp | $(BUILD_DIR)
	$(CXX) $(CXXFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -rf $(BUILD_DIR)

.PHONY: all clean run
