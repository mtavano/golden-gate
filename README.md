# Golden Gate

Golden Gate is a reverse proxy to debug and visualize requests/responses to external APIs, with a real-time dashboard.

## Requirements
- Go 1.20+
- [templ](https://templ.guide/) (for generating views)
- Docker (optional)
- Nixpacks (optional)

## Local Installation & Usage

1. Install dependencies:
   ```sh
   go mod download
   go install github.com/a-h/templ/cmd/templ@latest
   ```

2. Generate view files:
   ```sh
   make generate
   ```

3. Build and run:
   ```sh
   make run
   ```

4. Access the dashboard at [http://localhost:8080/dashboard](http://localhost:8080/dashboard)

## Configuration

Edit the `configs/service.json` file to define the services to proxy:

```json
{
  "api_1_name": {
    "base_prefix": "/cloud",
    "target": "http://cloud.mtavano.cc"
  }
}
```

## Docker

1. Build the image:
   ```sh
   docker build -t golden-gate .
   ```

2. Run the container:
   ```sh
   docker run -p 8080:8080 -v $(pwd)/configs:/app/configs golden-gate
   ```

## Nixpacks

1. Build the image:
   ```sh
   nixpacks build . -o golden-gate-nixpacks
   ```

2. Run the image:
   ```sh
   docker run -p 8080:8080 -v $(pwd)/configs:/app/configs golden-gate-nixpacks
   ```

---

## Image Reference

The Docker/Nixpacks image exposes port 8080 and expects the configuration file at `/app/configs/service.json`. 