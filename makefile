build:
	go build .
	./compiler input.toy
	./input

compile:
	./compiler input.toy

run:
	./input
