### Setup for development

#### Build docker image
```cmd
docker build -t lauro-santana/rinha-backend-2025 .
```

#### Run docker image
```cmd
docker run -p 9999:9999 -e PORT=9999 -e HOST=0.0.0.0 lauro-santana/rinha-backend-2025
```

### Setup compose

```cmd
docker compose up --build
```