build:
	go build -o scout
cp:
	cp scout ~/.local/bin/
	
install: build cp