# Sportz - Datos de partidos en tiempo real con WebSockets

Una plataforma integral para partidos deportivos en tiempo real que demuestra la arquitectura WebSocket en múltiples frameworks de backend (Express, FastAPI y Go). El sistema transmite comentarios de partidos en vivo y actualizaciones de marcadores a los clientes conectados mediante WebSockets para la comunicación bidireccional.

---

## Fundamentos de WebSockets

### ¿Qué son los WebSockets?

Los WebSockets permiten una comunicación bidireccional y persistente entre el cliente y el servidor a través de una única conexión TCP. A diferencia del HTTP tradicional (solicitud-respuesta), los WebSockets mantienen una conexión abierta, lo que permite a ambas partes enviar datos en cualquier momento sin necesidad de sondeo.

**Diferencias clave con HTTP:**
- **Conexión persistente:** Conexión única y abierta, que no se cierra después de cada mensaje.
- **Bidireccional:** Servidor y cliente pueden iniciar la comunicación.
- **Baja latencia:** Sin sobrecarga de establecimiento de conexión para cada mensaje.
- **Orientado a eventos:** Los mensajes llegan como eventos, lo que permite actualizaciones en tiempo real.
### Cómo funcionan

1. El cliente inicia una solicitud de actualización HTTP a `/websocket`.
2. El servidor acepta y actualiza la conexión al protocolo WebSocket.
3. La conexión permanece abierta para el intercambio de mensajes.
4. Cliente y servidor pueden enviar mensajes en cualquier momento.
5. La conexión se cierra cuando el cliente se desconecta o el servidor finaliza.
---

## Arquitectura del proyecto: Actualizaciones de partidos en tiempo real

### Sistema Descripción general

```
┌────────────────────┐
│ Frontend con React │
│    (Vite + TS)     │
└──────────┬─────────┘
           │
           ├─── API REST ──┐      ┌───────────────┐
           │               ├─────→│ Base de datos │
           └── WebSocket ──┤      └───────────────┘
                           │
           ┌───────────────┴────────────────┐
           │  Servidor backend (Elija uno)  │
           ├─ Express.js (Node.js)          │
           ├─ FastAPI (Python)              │
           └─ Go + Gorilla                  │
```

### Flujo de comentarios de partidos en tiempo real

```
1. El frontend se conecta a WebSocket
   └─ El cliente envía: { type: "subscribe", matchId: 1 }

2. El usuario hace clic en "Ver en vivo" en un partido
   └─ El frontend se suscribe al canal del partido

3. El script inicial inserta nuevos comentarios
   └─ API REST POST /matches/1/commentary

4. El backend transmite a los suscriptores
   └─ El servidor envía: { type: "commentary", data: { ... } }

5. El frontend recibe la actualización en tiempo real
   └─ El componente LiveFeed se actualiza instantáneamente

6. Las actualizaciones de puntuación siguen el mismo flujo
   └─ El servidor envía: { type: "score_update", matchId: 1, data: { ... } }
```
### Diagrama de flujo de mensajes

```
Frontend                                 Backend                        Base de datos
    │                                       │                                 │
    ├─ REST: GET /matches ────────────────→ │                                 │
    │                                       ├───── SELECT * from matches ────→│
    │                                       │←────────────────────────────────│
    │ ← [Array de coincidencias]────────────│                                 │
    │                                       │                                 │
    ├─ WS: subscribe(matchId: 1) ──────────→│                                 │
    │                                       ├─    Suscripción a la tienda     │
    │                                       │                                 │
    │                        [Ejecución del proceso semilla]                  │
    │                                       │                                 │
    │                                       ├──── REST: POST /comentario ────→│
    │                                       │                                 │
    │                                       ├──── INSERTAR comentario ───────→│
    │                                       │←────────────────────────────────│
    │                                       │                                 │
    │ ← WS: { evento comentario }   ────────┤    Transmisión a suscriptores   │
    │                                       │                                 │
    └─ Actualizaciones en directo           │                                 │
```

---

## Primeros pasos

### Requisitos previos

- **Node.js** (para Express) o **Python 3.12+** (para FastAPI) o **Go 1.25+** (para Gorilla)
- Base de datos **PostgreSQL**
- **Frontend**: Node.js + npm/pnpm

### Configuración del entorno

Crea un archivo `.env` en el backend que hayas elegido Directorio:

```env
DATABASE_URL=postgresql://usuario:contraseña@localhost:5432/sportz
PUERTO=8000
HOST=0.0.0.0
VITE_FRONTEND_URL=http://localhost:5173
ARCJET_KEY=tu_clave_arcjet_opcional
ARCJET_MODE=DRY_RUN
```

### Inicialización de la base de datos

Todos los backends utilizan el mismo esquema de PostgreSQL:

```sql
-- Enumeración de estado de coincidencia
CREATE TYPE match_status AS ENUM ('scheduled', 'live', 'finished');

-- Tabla de Partidos
CREATE TABLE matches (
id SERIAL PRIMARY KEY,
sport TEXT NOT NULL,
home_team TEXT NOT NULL,
away_team TEXT NOT NULL,
status match_status DEFAULT 'scheduled',
start_time TIMESTAMP,
end_time TIMESTAMP,
home_score INTEGER DEFAULT 0,
away_score INTEGER DEFAULT 0,
created_at TIMESTAMP DEFAULT now()
);

- Tabla de Comentarios
CREATE TABLE commentary (
id SERIAL PRIMARY KEY,
match_id INTEGER NOT NULL REFERENCES matches(id),
minute INTEGER
,
secuencia ENTERO,
periodo TEXTO,
tipo_evento TEXTO,
actor TEXTO,
equipo TEXTO,
mensaje TEXTO NO NULO,
metadatos JSONB,
etiquetas TEXTO[],
creado_en MARCA_DE_TIEMPO PREDETERMINADO ahora()
);
```

---

## Configuración y ejecución del backend

### Express.js (Node.js)

**Ubicación:** `sportz-express/`

#### Instalación

```bash
cd sportz-express
npm install
```

#### Migraciones de la base de datos

```bash
# Generar archivos de migración (si hay cambios en el esquema)
npm run db:generate

# Aplicar migraciones
npm run db:migrate

# Ver la base de datos en Studio (opcional)
npm run db:studio
```

#### Iniciar el servidor

```bash
# Desarrollo (con recarga automática)
npm run dev

# Producción
npm start
```

**Salida:**

```
El servidor se está ejecutando en http://localhost:8000
El servidor WebSocket se está ejecutando en ws://localhost:8000/websocket
```

#### Datos de semilla

```bash
# En otra terminal
API_URL=http://localhost:8000 npm run seed

# Con opciones personalizadas
DELAY_MS=500 SEED_FORCE_LIVE=1 npm run seed
```

---

### FastAPI (Python)

**Ubicación:** `sportz-fastapi/`

#### Instalación

```bash
cd sportz-fastapi

# Crear entorno virtual
python -m venv .venv
source .venv/bin/activate # En Windows: .venv\Scripts\activate

# Instalar dependencias
pip install -r requirements.txt
```

#### Migraciones de base de datos

```bash
# Aplicar migraciones
alembic upgrade Encabezado

# Crear nueva migración (si cambia el esquema)
alembic revision --autogenerate -m "Descripción"

# Ver estado actual
alembic current
```

#### Iniciar servidor

```bash
# Desarrollo (con recarga automática)
uvicorn src.main:app --reload --host 0.0.0.0 --port 8000

# Producción
python -m src.main
```

**Salida:**
```
INFO: Uvicorn ejecutándose en http://0.0.0.0:8000
```

#### Datos de inicialización

```bash
# En otra terminal (con .venv activado)
API_URL=http://localhost:8000 python -m src.seed.seed

# Con opciones personalizadas
DELAY_MS=500 SEED_FORCE_LIVE=1 python -m src.seed.seed
```

---

### Go + Gorilla

**Ubicación:** `sportz-gorilla/`

#### Instalación

```bash
cd sportz-gorilla

# Descargar dependencias
go mod download
```

#### Migraciones de base de datos

```bash
# Ejecutar la migración SQL manualmente
psql "$DATABASE_URL" -f db/migrations/0001_initial_schema.sql
```

#### Iniciar servidor

```bash
# Desarrollo/Producción
go run cmd/server/main.go

# Compilar binario (opcional)
go build -o bin/sportz-server cmd/server/main.go
./bin/sportz-server
```

**Salida:**
```
El servidor se está ejecutando en http://localhost:8000
El servidor WebSocket se está ejecutando en ws://localhost:8000/websocket
```

#### Datos de semilla

```bash
# En otra terminal
API_URL=http://localhost:8000 go run seed/main.go

# Con opciones personalizadas
DELAY_MS=500 SEED_FORCE_LIVE=1 go run seed/main.go
```

---

## Configuración del Frontend

**Ubicación:** `sportz-frontend/`

### Instalación

```bash
cd sportz-frontend
npm install
```

### Configurar la URL del Backend

Editar `src/constants/constants.ts`:

```typescript
const DEFAULT_API_BASE_URL = "http://localhost:8000";
const DEFAULT_WS_BASE_URL = "ws://localhost:8000/websocket";

// O usar variables de entorno
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || DEFAULT_API_BASE_URL;

export const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL || DEFAULT_WS_BASE_URL;
```

### Iniciar el servidor de desarrollo

```bash
# Terminal 1: Interfaz (puerto 5173)
npm run dev

# Terminal 2: Servidor backend (puerto 8000)
cd ../sportz-express && npm run dev
# O
cd ../sportz-fastapi && uvicorn src.main:app --reload
# O
cd ../sportz-gorilla && go run cmd/server/main.go

# Terminal 3: Cargar datos
API_URL=http://localhost:8000 npm run seed # o equivalentes en Python/Go
```

### Compilación para producción

```bash
npm run build
npm run preview
```

---

## Interfaz: Interacción con el usuario y flujo de datos

### Componente Arquitectura

```
App.tsx (Contenedor principal)
├── Indicador de estado
│   └─ Muestra el estado de la conexión WebSocket (conectado/conectando/error)
│
├── Tarjeta de partido (Elemento de la cuadrícula)
│   ├─ Muestra: Nombres de los equipos, puntuaciones, estado del partido
│   ├─ Acciones: Botones "Ver en directo" / "Ver resumen"
│   └─ Efecto: Los resúmenes de las puntuaciones se animan al actualizarse
│
└── Noticias en directo (Barra lateral)
    ├─ Muestra los comentarios del partido seleccionado
    ├─ Actualizaciones en tiempo real a medida que llegan los mensajes
    └─ Visualización de la línea de tiempo con marcas de tiempo
```

### Flujo de interacción del usuario

#### 1. **Página Cargar**

```
Se monta App.tsx
    ↓
El hook useMatchData se inicializa
    ↓
API REST: GET /matches?limit=50
    ↓
Partidos mostrados en cuadrícula (6 por página)
    ↓
Se establece la conexión WebSocket (hook useWebSocket)
    ↓
El indicador de estado se pone VERDE
```

**Archivos involucrados:**
- `src/hooks/useMatchData.ts` - Gestiona la obtención y el sondeo de los partidos
- `src/hooks/useWebSocket.ts` - Gestiona la conexión WebSocket
- ​​`src/services/api.ts` - Llamadas a la API REST

#### 2. **Ver partido (Seleccionar comentarios)**

```
El usuario hace clic en el botón "Ver en vivo" en la tarjeta del partido
    ↓
Se llama a watchMatch(matchId)
    ↓
WebSocket: { type: "subscribe", matchId: 123 }
    ↓ 
API REST: GET /matches/123/commentary?limit=100
    ↓ 
Comentarios cargados y mostrados en LiveFeed
    ↓ 
Resaltados de MatchCard (r amarilla)
```

(inicio) muestra que está activo

**Archivos relacionados:**
- `src/components/MatchCard.tsx` - Visualización del partido y acciones de los botones
- `src/components/LiveFeed.tsx` - Visualización de comentarios con línea de tiempo
- `src/hooks/useMatchData.ts` - Implementación de watchMatch

#### 3. **Actualización de comentarios en tiempo real**

```
El script inicial inserta los comentarios
    ↓
Backend: INSERT INTO commentary
    ↓
El backend transmite mediante WebSocket
    ↓
El frontend recibe: { type: "commentary", data: {...} }
    ↓
useMatchData actualiza el estado
    ↓
LiveFeed se vuelve a renderizar con la nueva entrada
    ↓
El mensaje aparece con animación de desvanecimiento y deslizamiento
```

**Gestor de mensajes en tiempo real:**
```typescript
// src/hooks/useMatchData.ts
const handleWSMessage = useCallback((msg: WSMessage) => {
    switch (msg.type) {
        case "commentary":
            if (latestMatchIdRef.current == msg.data.matchId) {
                setCommentary((prev) => [msg.data, ...prev]);
            }
            break;
        case "score_update":
            setMatches((prevMatches) =>
                prevMatches.map((m) => 
                    m.id == msg.matchId 
                        ? { ...m, homeScore: msg.data.homeScore, awayScore: msg.data.awayScore }
                        : m
                )
            );
            break;
    }
}, []);
```

#### 4. **Animación de actualización de puntuación**

```
El backend envía: { type: "score_update", matchId: 1, data: { homeScore: 2, awayScore: 1 } }
    ↓
El frontend recibe el mensaje de actualización de puntuación
    ↓
MatchCard detecta el cambio de puntuación mediante useEffect
    ↓
El cuadro de puntuación se anima con un fondo amarillo (animación de pulso)
    ↓
La animación finaliza después de 900 ms
```

**Código de animación de puntuación:**
```typescript
// src/components/MatchCard.tsx
useEffect(() => {
    if (prevScore.home !== match.homeScore) {
        setHomePulse(true);
        setTimeout(() => setHomePulse(false), 900);
    }
}, [match.homeScore, match.awayScore]);
```

---

## Referencia de la API

### Puntos finales REST

#### Partidos

```
GET /matches?limit=50
Response: { data: Match[] }

POST /matches
Body: {
    sport: "Football",
    homeTeam: "Arsenal",
    awayTeam: "Liverpool",
    startTime: "2025-05-21T20:00:00Z",
    endTime: "2025-05-21T21:45:00Z",
    homeScore: 0,
    awayScore: 0
}
Response: { data: Match }
```

#### Comentarios

```
GET /matches/{id}/commentary?limit=100
Response: { data: Commentary[] }

POST /matches/{id}/commentary
Body: {
    minute: 45,
    message: "Goal! Arsenal scores",
    team: "Arsenal",
    eventType: "goal",
    actor: "Saka",
    metadata: { assist: "Odegaard" },
    tags: ["goal", "counter-attack"]
}
Response: { data: Commentary }
```

### Eventos WebSocket
#### Cliente → Servidor

```json
{ "tipo": "suscribir", "id del partido": 1 }
{ "tipo": "cancelar suscripción", "id del partido": 1 }
```
#### Servidor → Cliente

```json
{ "tipo": "bienvenido" }
{ "tipo": "suscrito", "id del partido": 1 }
{ "tipo": "cancelar suscripción", "id del partido": 1 } }
{ "type": "match_created", "data": { ... } }
{ "type": "commentary", "data": { ... } }
{ "type": "score_update", "matchId": 1, "data": { "homeScore": 2, "awayScore": 1 } }
{ "type": "ping" }
{ "type": "error", "message": "JSON no válido" }
```

---

## Ejemplos de formato de datos

### Objeto de partido

```json
{
    "id": 1,
    "sport": "Football",
    "homeTeam": "Arsenal FC",
    "awayTeam": "Liverpool FC",
    "status": "live",
    "startTime": "2025-05-21T20:00:00Z",
    "endTime": "2025-05-21T21:45:00Z",
    "homeScore": 2,
    "awayScore": 1,
    "createdAt": "2025-05-21T18:30:00Z"
}
```

### Objeto de comentarios

```json
{
    "id": 42,
    "matchId": 1,
    "minute": 34,
    "sequence": 2,
    "period": "1st Half",
    "eventType": "goal",
    "actor": "Bukayo Saka",
    "team": "Arsenal FC",
    "message": "GOALLLLLL! Saka breaks the deadlock with a brilliant finish!",
    "metadata": { "assist": "Martin Odegaard", "difficulty": "high" },
    "tags": ["goal", "counter-attack"],
    "createdAt": "2025-05-21T20:34:15Z"
}
```

---

## Probando el sistema

### Tutorial completo

1. **Iniciar el backend** (elige uno):

   ```bash
   # Express
   cd sportz-express && npm run dev
   
   # FastAPI
   cd sportz-fastapi && uvicorn src.main:app --reload
   
   # Go
   cd sportz-gorilla && go run cmd/server/main.go
   ```

2. **Iniciar el frontend**:

   ```bash
   cd sportz-frontend && npm run dev
   # Open http://localhost:5173
   ```

3. **Introducir datos** (en una nueva terminal):

   ```bash
   API_URL=http://localhost:8000 npm run seed
   ```

4. **Observar actualizaciones en tiempo real**:

- La interfaz carga los partidos.
- Haz clic en "Ver en vivo" en cualquier partido.
- El panel de LiveFeed se llena con comentarios.
- Los nuevos comentarios aparecen en tiempo real.
- Las actualizaciones de puntuación se animan con resúmenes.

### Verificar la conexión WebSocket

**Herramientas para desarrolladores del navegador → Red → WS**

Busca la conexión `/websocket`. Deberías ver:
- Mensaje inicial de bienvenida.
- Mensaje de suscripción al ver un partido.
- Mensajes de comentarios durante la ejecución de la semilla.
- Mensajes periódicos de ping (cada 30 s).
---

## Estructura del proyecto

```
.
├── README.md (Este archivo)
├── sportz-express/ (Node.js + Express)
│   ├── src/
│   │   ├── index.js (Entrada del servidor)
│   │   ├── routes/ (Puntos finales REST)
│   │   ├── websocket/ (Servidor WS)
│   │   ├── database/ (Drizzle ORM)
│   │   └── seed/
│   ├── package.json
│   └── drizzle.config.js
│
├── sportz-fastapi/ (Python + FastAPI)
│   ├── src/
│   │   ├── main.py (Entrada del servidor)
│   │   ├── routes/ (Puntos finales REST)
│   │   ├── websocket/ (Servidor WS)
│   │   ├── database/ (SQLAlchemy)
│   │   └── seed/
│   ├── requirements.txt
│   └── alembic/ (Migraciones)
│
├── sportz-gorilla/ (Go + Gorilla WebSocket)
│   ├── cmd/server/
│   │   └── main.go (Entrada del servidor)
│   ├── internal/
│   │   ├── routes/ (Puntos finales REST)
│   │   ├── websocket/ (Servidor WS)
│   │   └── database/ (Controlador pgx)
│   ├── seed/
│   │   └── main.go
│   └── db/migrations/
│
└── sportz-frontend/ (React + TypeScript + Vite)
    ├── src/
    │   ├── App.tsx (Componente principal)
    │   ├── components/ (Componentes de la interfaz de usuario)
    │   ├── hooks/ (useMatchData, useWebSocket)
    │   ├── services/ (Llamadas a la API)
    │   └── constants/ (URLs del backend)
    ├── package.json
    └── tsconfig.json
```

---

## Seguridad y rendimiento

### Integración con Arcjet (opcional)

Todos los backends son compatibles con Arcjet para la protección contra ataques DDoS y la limitación de velocidad:

```env
ARCJET_KEY=tu_clave_aquí
ARCJET_MODE=LIVE # o DRY_RUN para pruebas
```

- **HTTP**: 50 solicitudes cada 10 segundos
- **WebSocket**: 5 conexiones cada 2 segundos
### Conexión Límites

- Tamaño máximo de mensaje WebSocket: **1 MB**
- Límite máximo de consultas a la base de datos: **100 filas**
- Intervalo de ping: **30 segundos**
- Tiempo de espera de pong: **60 segundos**
### Manejo de errores

Las desconexiones de WebSocket activan la reconexión automática con retroceso exponencial:

```typescript
// src/constants/constants.ts
INITIAL_RECONNECT_DELAY = 1000ms    // 1 segundo
MAX_RECONNECT_DELAY = 30000ms       // 30 segundos
```

---

## Solución de problemas

### Problemas de conexión

**Problema**: El frontend muestra "Actualizaciones en vivo no disponibles"

**Soluciones**:
1. Compruebe que el backend esté en funcionamiento: `curl http://localhost:8000/`
2. Verifique el punto final de WebSocket: `ws://localhost:8000/websocket`
3. Verificar CORS: `VITE_FRONTEND_URL` en `.env`
4. Consultar la consola del navegador para ver si hay errores: F12 → Consola

### Conexión a la base de datos

**Problema**: "DATABASE_URL no está definido"

**Solución**:
```bash
# Verificar que el archivo .env exista en el directorio del backend
cat .env | grep DATABASE_URL
# Probar conexión
psql "$DATABASE_URL" -c "SELECT 1"
```

### No se carga la semilla

**Problema**: Los comentarios no aparecen en tiempo real

**Soluciones**:
1. Verificar que existan coincidencias: `curl http://localhost:8000/matches`
2. Comprobar que la semilla se esté ejecutando: `API_URL=http://localhost:8000 npm run seed`
3. El frontend debe estar "observando" una coincidencia (suscribirse)
4. Comprobar la base de datos directamente: `psql "$DATABASE_URL" -c "SELECT * FROM commentary LIMIT 5"`
---

## Recursos adicionales

- [Documentación de WebSocket MDN](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
- [Express.js](https://expressjs.com/)
- [FastAPI](https://fastapi.tiangolo.com/)
- [Go Gorilla WebSocket](https://github.com/gorilla/websocket)
- [React Hooks](https://react.dev/reference/react)
---

## Licencia

Este proyecto se proporciona tal cual, con fines educativos.