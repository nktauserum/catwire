CXX = g++
CXXFLAGS = -std=c++23 -Wall -Wextra -pedantic
LIBS = -luring -lstdc++
SRC = main.cc memory.cc 
OBJ = $(SRC:.cc=.o)
TARGET = srv 

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CXX) -o $@ $^ $(LIBS)

%.o: %.cc
	$(CXX) $(CXXFLAGS) -c $< -o $@

run: all
	./$(TARGET)

clean:
	rm -f $(OBJ) $(TARGET)

.PHONY: all clean run
