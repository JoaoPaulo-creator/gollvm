build:
	go build .
	./compiler input.toy
	./input

compile:
	./compiler input.toy

clang:
	clang input.ll -o output
