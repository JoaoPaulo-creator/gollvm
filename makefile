build:
	go build .

compile:
	./compiler -input input.xyz -output input.ll -run

run:
	./input
