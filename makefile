build:
	go build .
	./compiler -input input.xyz -output input.ll -run

compile:
	./compiler -input input.xyz -output input.ll -run

run:
	./input
