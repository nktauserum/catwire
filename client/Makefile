CC = gcc
CFLAGS_DEBUG = -Wall -Wextra -pedantic -g
CFLAGS_RELEASE = -O3 -march=native -mtune=native -flto -DNDEBUG
LIBS = -lsodium
BUILD_DIR = build

SRC = $(shell find . -name "*.c" | sed 's|^\./||')
OBJ = $(addprefix $(BUILD_DIR)/, $(SRC:.c=.o))
TARGET = $(BUILD_DIR)/cl

.PHONY: all clean run release

all: CFLAGS = $(CFLAGS_DEBUG)
all: $(TARGET)

release: CFLAGS = $(CFLAGS_RELEASE)
release: $(TARGET)

$(TARGET): $(OBJ)
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -o $@ $^ $(LIBS)

$(BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -rf $(BUILD_DIR)
