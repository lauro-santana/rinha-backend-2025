### Ferramentas

- GO
- GOE ORM
- RabbitMQ
- PostgreSQL
- Nginx

### Setup e teste manual

- Na pasta raiz

    ```cmd
    docker compose up --build
    ```

### Testes automatizados

- Na pasta tests, preparar o ambiente

    ```cmd
    docker compose up -d
    ```

- Subi os serviços da rinha

- Rodar os testes do Go

    ```cmd
    PAYMENT_PROCESSOR_URL_DEFAULT=http://localhost:8001 PAYMENT_PROCESSOR_URL_FALLBACK=http://localhost:8002 go test . -v -race -count=1
    ```