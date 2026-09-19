CC = gcc
CFLAGS = -Wall -Wextra -pedantic  
SRC = main.c 
OBJ = $(SRC:.c=.o)
TARGET = srv 

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CC) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -f $(OBJ) $(TARGET)

.PHONY: all clean
