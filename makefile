build:
	go build .
	./compiler input2.toy
	./input2

clang:
	clang input.ll -o output
