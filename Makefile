all: compose

compose:
	@docker compose up --build --force-recreate

build:
	@docker build -t img-backend-rinha-2025 .

run: 
	@docker run -p 9999:9999 -e PORT=9999 -e HOST=0.0.0.0 img-backend-rinha-2025