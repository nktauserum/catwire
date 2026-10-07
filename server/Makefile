CXX = g++
CXXFLAGS = -std=c++23 -Wall -Wextra -pedantic -fno-rtti
LIBS = -luring -lsodium -lstdc++
BUILD_DIR = build

SRC = $(shell find . -name "*.cpp" | sed 's|^\./||')
OBJ = $(addprefix $(BUILD_DIR)/, $(SRC:.cpp=.o))
TARGET = $(BUILD_DIR)/srv

.PHONY: all clean run

all: $(TARGET)

$(TARGET): $(OBJ)
	@mkdir -p $(dir $@)
	$(CXX) -o $@ $^ $(LIBS)

$(BUILD_DIR)/%.o: %.cpp
	@mkdir -p $(dir $@)
	$(CXX) $(CXXFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -rf $(BUILD_DIR)
