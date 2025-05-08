build:
	go build .
	./compiler input.toy
	./input

clang:
	clang input.ll -o output
