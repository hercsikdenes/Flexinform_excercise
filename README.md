# Autószerviz API

Dockerben futtatható, csak olvasási műveleteket biztosító REST API az ügyfelek, járművek és szervizesemények lekérdezésére. Az alkalmazás induláskor létrehozza az adatbázissémát, majd az első futás során betölti a `data` könyvtárban található forrásadatokat.

## Felhasznált technológiák és verziók

| Technológia | Verzió | Szerepe |
| --- | --- | --- |
| Go | 1.27.1 | Az alkalmazás nyelve és fordítási környezete |
| Gin | 1.10.0 | HTTP router és webes keretrendszer |
| GORM | 1.25.12 | Adatbázis-hozzáférés |
| GORM PostgreSQL driver | 1.5.11 | PostgreSQL illesztőprogram |
| Goose | 3.24.1 | Adatbázis-migrációk kezelése |
| PostgreSQL | 16, Alpine image | Adatbázis |
| Alpine Linux | 3.20 | A futtatási Docker image alapja |

A közvetett Go-függőségek pontos verzióit a `go.mod` és a `go.sum` fájlok rögzítik.

## Előfeltételek

A Dockeres futtatáshoz az alábbiak szükségesek:

- Docker Engine;
- Docker Compose v2, vagyis a `docker compose` parancs;
- szabad host port az API számára.

Helyi Go- vagy PostgreSQL-telepítés nem szükséges, mert a fordítás, a tesztelés és az adatbázis is konténerben fut.

## Beüzemelés

### 1. Környezeti változók beállítása

Hozza létre a helyi `.env` fájlt a mellékelt sablonból:

```sh
cp .env.example .env
```

Fejlesztői környezetben az alapértékek azonnal használhatók. Éles vagy megosztott környezetben legalább a `POSTGRES_PASSWORD` értékét módosítani kell.

A sablon alapértelmezés szerint a `8080`-as porton teszi elérhetővé az API-t. A `httptests` könyvtárban található kézi HTTP-kérések a `42069`-es portot használják; ezek futtatásához állítsa ezt az értéket az `.env` fájlban:

```dotenv
SERVER_PORT=42069
```

### 2. Az alkalmazás felépítése és elindítása

```sh
docker compose up -d --build
```

A Compose először elindítja a PostgreSQL-adatbázist, megvárja annak üzemkész állapotát, majd elindítja az API-t. Az alkalmazás indulásakor automatikusan:

1. kapcsolódik az adatbázishoz;
2. lefuttatja a `migrations` könyvtár Goose-migrációit;
3. üres adatbázis esetén betölti a `data` könyvtár JSON-állományait;
4. elindítja a HTTP-kiszolgálót.

### 3. A működés ellenőrzése

Ha a `SERVER_PORT=8080` beállítást használja:

```sh
curl http://localhost:8080/health
```

Elvárt válasz:

```json
{"message":"ok"}
```

Ha a `SERVER_PORT=42069` értéket állította be, az ellenőrző URL `http://localhost:42069/health`.

Az elindított konténerek állapota és az API naplója az alábbi parancsokkal vizsgálható:

```sh
docker compose ps
docker compose logs -f api
```

### 4. Leállítás

```sh
docker compose down
```

Ez a parancs nem törli a PostgreSQL-adatokat tároló `postgres_data` Docker volume-ot, ezért a következő indításkor az adatok megmaradnak.

Ha kifejezetten tiszta adatbázissal szeretné újrakezdeni, a konténerek és a volume együtt eltávolítható:

```sh
docker compose down --volumes
```

> Figyelem: a `--volumes` kapcsoló véglegesen törli a helyi adatbázis tartalmát.

## Konfiguráció

| Változó | Alapérték | Leírás |
| --- | --- | --- |
| `POSTGRES_DB` | `car_service` | A PostgreSQL-adatbázis neve |
| `POSTGRES_USER` | `car_service` | PostgreSQL-felhasználó |
| `POSTGRES_PASSWORD` | `car_service_dev_password` | PostgreSQL-jelszó; fejlesztői alapérték |
| `POSTGRES_HOST` | `db` | Az adatbázis Compose szolgáltatásneve |
| `POSTGRES_PORT` | `5432` | A PostgreSQL belső portja |
| `SERVER_PORT` | `8080` | Az API belső és host portja |
| `SHUTDOWN_TIMEOUT` | `10s` | A szabályos leállítás maximális ideje |
| `DATA_DIR` | `/app/data` | A forrás JSON-fájlok könyvtára |
| `MIGRATIONS_DIR` | `/app/migrations` | Az SQL-migrációk könyvtára |
| `DATABASE_DSN` | összeállított érték | Opcionális teljes PostgreSQL kapcsolati karakterlánc |

A `DATABASE_DSN` megadása felülírja a különálló PostgreSQL-beállításokból összeállított kapcsolati karakterláncot. A `SERVER_PORT` értékének 1 és 65535 közötti egész számnak, a `SHUTDOWN_TIMEOUT` értékének pedig pozitív Go időtartamnak, például `10s` vagy `1m` formátumúnak kell lennie.

## API-végpontok

Az alábbi példákban a `BASE_URL` az `.env` fájlban beállított porttól függően például `http://localhost:8080` vagy `http://localhost:42069`.

| Metódus és útvonal | Leírás |
| --- | --- |
| `GET /health` | Az alkalmazás és az adatbázis állapotának ellenőrzése |
| `GET /api/clients?page=1&per_page=50` | Aktív ügyfelek lapozott listája |
| `GET /api/clients/search?name=...` | Ügyfél keresése részleges, kis- és nagybetűtől független név alapján |
| `GET /api/clients/search?personal_id=...` | Ügyfél keresése pontos személyi azonosító alapján |
| `GET /api/clients/{clientId}/cars` | Egy aktív ügyfél aktív járművei és azok legutóbbi eseménye |
| `GET /api/clients/{clientId}/cars/{carId}/services` | Egy jármű időrendbe rendezett szerviztörténete |

A név és a személyi azonosító közül pontosan egy keresési paraméter adható meg. A név szerinti keresésnek pontosan egy ügyfelet kell eredményeznie. A lapméret legnagyobb engedélyezett értéke 100.

Minden hibaválasz azonos formátumú:

```json
{
  "message": "request input is invalid"
}
```

Az API a következő fontos HTTP-státuszkódokat használja:

- `200 OK`: sikeres lekérdezés;
- `404 Not Found`: nem létező erőforrás vagy útvonal;
- `405 Method Not Allowed`: nem támogatott HTTP-metódus;
- `422 Unprocessable Entity`: hibás vagy nem egyértelmű bemenet;
- `500 Internal Server Error`: váratlan belső hiba;
- `503 Service Unavailable`: sikertelen adatbázis-állapotellenőrzés.

## Tesztelés és minőség-ellenőrzés

A Go-tesztek Dockerben az alábbi paranccsal futtathatók. A `--build` kapcsoló gondoskodik arról, hogy a teszt image mindig az aktuális forrás- és tesztfájlokat tartalmazza:

```sh
docker compose run --rm --build tests
```

Az egyes tesztek és altesztek nevének megjelenítéséhez használja a részletes kimenetet:

```sh
docker compose run --rm --build tests go test -v ./...
```

Ha a teszt image a legutóbbi forrásmódosítás óta már újra lett építve, a `--build` elhagyható:

```sh
docker compose run --rm tests
```

A formázás és a statikus elemzés ellenőrzése:

```sh
docker compose run --rm --build tests sh -c 'test -z "$(gofmt -l .)" && go vet ./...'
```

A szolgáltatásréteg tesztjei többek között a lapozást, a keresési szabályokat, a találati eseteket, a járműlistákat és az eseményidők kezelését fedik le. A HTTP-tesztek a Go `net/http/httptest` csomagjával és tesztpéldányokkal ellenőrzik a routert, ezért futó adatbázis nélkül is végrehajthatók.

A `httptests` könyvtár minden végponthoz tartalmaz kézzel futtatható `.http` kéréseket. Ezek használata előtt az API-nak futnia kell a `42069`-es porton. A fájlok IntelliJ IDEA, GoLand vagy megfelelő REST Client bővítménnyel rendelkező szerkesztőből indíthatók.

## Architektúra

```text
HTTP / Gin
    ↓ context.Context
Szolgáltatásréteg / üzleti szabályok
    ↓ repository interfész
GORM repository
    ↓ paraméterezett SQL
PostgreSQL
```

A főbb könyvtárak szerepe:

| Útvonal | Tartalom |
| --- | --- |
| `cmd/api` | Az alkalmazás belépési pontja |
| `internal/application` | Függőségek összeállítása, indítás és szabályos leállítás |
| `internal/config` | Környezeti konfiguráció betöltése és ellenőrzése |
| `internal/database` | PostgreSQL-kapcsolat és migrációk |
| `internal/http` | Router, végpontok és HTTP-hibakezelés |
| `internal/service` | Üzleti szabályok |
| `internal/repository` | GORM-alapú adatelérés |
| `internal/seed` | A JSON-forrásadatok ellenőrzése és betöltése |
| `migrations` | Verziózott adatbázis-migrációk |
| `data` | Kiinduló ügyfél-, jármű- és szervizadatok |
| `httptests` | Kézzel futtatható HTTP-kérések |

## Adatbázis és kezdeti adatok

Az alkalmazás a sémát a `migrations/00001_initial_schema.sql` migrációval hozza létre; GORM `AutoMigrate` nincs használatban. A Goose nyilvántartja a már alkalmazott migrációkat, ezért azok ismételt futtatása biztonságos.

A kezdeti adatbetöltés egy tranzakcióban ellenőrzi a `clients`, `vehicles`, `car_ownerships` és `event_log` táblákat:

- ha mindegyik üres, betölti a JSON-forrásadatokat;
- ha mindegyik tartalmaz adatot, kihagyja az importot;
- részlegesen feltöltött adatbázis esetén hibával leáll, hogy ne hozzon létre bizonytalan állapotot.

A rendszer külön kezeli a fizikai járművet és az ügyfélhez tartozó tulajdonosi kapcsolatot. Az API-ban a `car_id` az ügyfélen belüli járműsorszámot, a `vehicle_id` pedig a jármű globális forrásazonosítóját jelenti.

## Leállítási viselkedés

A folyamat kezeli a `SIGINT` és `SIGTERM` jelzéseket. Leállításkor a HTTP-kiszolgáló a `SHUTDOWN_TIMEOUT` időtartamán belül megpróbálja befejezni a folyamatban lévő kéréseket, majd lezárja az adatbázis-kapcsolatot.

## Gondolkodtató kérdések
### Hogyan dokumentálnád ezt az API-t úgy, hogy egy másik fejlesztő gyorsan eligazodjon rajta? Milyen eszközt használnál hozzá?

Swagger / OpenAPI ha arról van szó, hogy csak használni és nem fejleszteni. Másfelől README.md elég jól összefoglal dolgokat amiket az AI könnyen és amúgy struktúráltan le tud dokumentálni.

### Milyen eszközöket, csomagokat, gyakorlatokat ismersz, amiket egy ilyen Go projektben szívesen alkalmaznál, ha nem lett volna időkorlát?
Gin és a GORM de ez alapból benne volt a feladatban, még különböző security package-k is vannak amiket lehetne használni. Ha egy kicsit távolabbról nézzük a dolgot és nem csak Go-related kérdésekbe akarunk belemenni akkor a logolást ki lehetett volna szervezni egy saját service-be és ha ( feltételezünk egy micro service struktúrát és ) azt használja az összes többi micro-service akkor még RabbitMQ-t a kommunikációhoz, de ha pusztán Go-related dolgokba akarunk belemenni akkor részben emiatt is jelentkeztem, mert a Go-val tisztában vagyok és egyszerü volt felkapni és játszani vele, de magát a Go ökonómiát annyira nem ismerem. ¯\_(ツ)_/¯

Ha van olyan package ami a teszteket szexibbé tudja tenni az menő lenne.

### Ha ennek az API-nak minden hajnalban le kellene futtatnia egy időigényes karbantartó feladatot (pl. régi naplóbejegyzések archiválása), hogyan valósítanád meg? Mire figyelnél oda (pl. mi történjen, ha a feladat elhúzódik vagy elszáll)?

Na igen ez egy jó kérdés, mármint attól függ. Mert egy systemd.timer+systemd.service vagy cron jobbal be lehet időziteni a dolgokat, de azok nem docker specifikusak, viszont szerintem a dockernek is megvan ehhez a saját beépitett megoldása. Másfelől dockerben lévő scripteket is tuti meg lehet hivni külső scripttel amit a fentiek inditanak el viszont a logolás akkor nem feltétlen lesz annyira kielégitő ( gondolok főleg a systemd naplózására, mert a cron az cron és kész ), másfelől viszont a script is valószinüleg logolna közben.

Ami az adatfeldolgozást illeti azt batch-transaction ha egyáltalán létezik ez a megnevezés vagy transaction chunking. Ahol egy nagy transactiont sok kisebbre darabolunk igy ha egy része elhal se hal el az összes és le lenne logolva, hogy melyiknél volt probléma és mi. Plusz igy a timeout is elosztott lesz és nem egy taskra lesz beállitva.

## Mivel mennyi idő telt el
- Adatbázis megtervezése: ~2 óra
- Infra létrehozása: 5 perc (AI)
- Feladat: ~8 óra
    - két régebbi Gin és GORM játszadozásom gyúrtam össze és irtam át a dolgokat
    - plusz AI, hogy egy ellenőrzést nyomjon rá és ajánljon ezt azt
    - annak leellenőrzése
    - graceful shutdown példa keresés + copypasta
- Tesztek:
    - unit tesztek: 5 perc ( AI )
    - http tesztek: 1 óra
- Fogalmazás: egész este a plafont nézve gondolkoztam, hogy hogy lenne érdemes megfogalmazni a gondolataim
