```cmd
PAYMENT_PROCESSOR_URL_DEFAULT=http://localhost:8001 PAYMENT_PROCESSOR_URL_FALLBACK=http://localhost:8002 go test . -v -race -count=1
```