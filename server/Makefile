CXX = g++
CXXFLAGS = -std=c++23 -Wall -Wextra -pedantic
LIBS = -luring -lstdc++

BUILD_DIR = build
SRC = main.cc memory.cc networking.cc
OBJ = $(addprefix $(BUILD_DIR)/, $(SRC:.cc=.o))
TARGET = $(BUILD_DIR)/srv

all: $(TARGET)

$(BUILD_DIR):
	mkdir -p $@

$(TARGET): $(OBJ) | $(BUILD_DIR)
	$(CXX) -o $@ $^ $(LIBS)

$(BUILD_DIR)/%.o: %.cc | $(BUILD_DIR)
	$(CXX) $(CXXFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -rf $(BUILD_DIR)

.PHONY: all clean run
