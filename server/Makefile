CXX = g++
CXXFLAGS_DEBUG = -std=c++23 -Wall -Wextra -pedantic -g
CXXFLAGS_RELEASE = -std=c++23 -O3 -march=native -mtune=native -flto -DNDEBUG
LIBS = -luring -lsodium
BUILD_DIR = build

SRC = $(shell find . -name "*.cpp" | sed 's|^\./||')
OBJ = $(addprefix $(BUILD_DIR)/, $(SRC:.cpp=.o))
TARGET = $(BUILD_DIR)/srv

.PHONY: all clean run release

all: CXXFLAGS = $(CXXFLAGS_DEBUG)
all: $(TARGET)

release: CXXFLAGS = $(CXXFLAGS_RELEASE)
release: $(TARGET)

$(TARGET): $(OBJ)
	@mkdir -p $(dir $@)
	$(CXX) $(CXXFLAGS) -o $@ $^ $(LIBS)

$(BUILD_DIR)/%.o: %.cpp
	@mkdir -p $(dir $@)
	$(CXX) $(CXXFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -rf $(BUILD_DIR)
