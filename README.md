# Golden Gate

Golden Gate es un proxy reverso para debuggear y visualizar requests/responses a APIs externas, con dashboard en tiempo real.

## Requisitos
- Go 1.20+
- [templ](https://templ.guide/) (para generar vistas)
- Docker (opcional)
- Nixpacks (opcional)

## Instalación y uso local

1. Instala las dependencias:
   ```sh
   go mod download
   go install github.com/a-h/templ/cmd/templ@latest
   ```

2. Genera los archivos de vistas:
   ```sh
   make generate
   ```

3. Compila y ejecuta:
   ```sh
   make run
   ```

4. Accede al dashboard en [http://localhost:8080/dashboard](http://localhost:8080/dashboard)

## Configuración

Edita el archivo `configs/service.json` para definir los servicios a proxificar:

```json
{
  "api_1_name": {
    "base_prefix": "/cloud",
    "target": "http://cloud.mtavano.cc"
  }
}
```

## Docker

1. Crea la imagen:
   ```sh
   docker build -t golden-gate .
   ```

2. Ejecuta el contenedor:
   ```sh
   docker run -p 8080:8080 -v $(pwd)/configs:/app/configs golden-gate
   ```

## Nixpacks

1. Construye la imagen:
   ```sh
   nixpacks build . -o golden-gate-nixpacks
   ```

2. Ejecuta la imagen:
   ```sh
   docker run -p 8080:8080 -v $(pwd)/configs:/app/configs golden-gate-nixpacks
   ```

---

## Referencia de imagen

La imagen de Docker/Nixpacks expone el puerto 8080 y espera el archivo de configuración en `/app/configs/service.json`. 