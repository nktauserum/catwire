CC = gcc
CFLAGS = -Wall -Wextra -pedantic 
LIBS = -luring -lstdc++
SRC = main.cc 
OBJ = $(SRC:.c=.o)
TARGET = srv 

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CC) -o $@ $^ $(LIBS)

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -f $(OBJ) $(TARGET)

.PHONY: all clean
