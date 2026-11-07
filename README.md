# tour-track

A web app that shows bands and artists from the Groupie Trackers API, with
their concerts and an **Artist Activity Classifier** that rates every artist
from *Emerging* to *Legendary*.

Written in Go with the standard library only. No frameworks, no external
packages.

## Run

```bash
go run .                         # http://localhost:8084
go run . -addr :9000             # another port
go run . -api http://localhost:1 # unreachable API: pages answer 503
```

| Flag    | Default                                 | Meaning                 |
|---------|-----------------------------------------|-------------------------|
| `-addr` | `:8084`                                 | HTTP listen address     |
| `-api`  | `https://groupietrackers.01edu.ai/api`  | base URL of the API     |

Templates and static files are embedded into the binary, so `go build`
produces a single file that runs from any directory.

## Tests

```bash
go test ./... -race -cover
```

Every package except `main` is covered at 99-100%.

## Project structure

```
main.go                  wiring: flags, embedded files, background reload, server, shutdown
internal/api             HTTP client for the Groupie Trackers API
internal/activity        Activity Classifier and Rising Star rule (pure functions)
internal/store           parsing, artist views, search, thread-safe in-memory store
internal/handlers        routes, HTML pages, JSON API, middleware
templates/               HTML templates (layout.html holds the shared parts)
static/                  style.css, app.js, favicon.svg
```

Dependencies point one way only: `handlers → store → activity, api`.

## How the four API parts are used

All four endpoints are loaded at start, in parallel, and linked by artist
`id` in `store.Build`. Each one is used for what it describes:

| Endpoint     | Used for                                                              |
|--------------|-----------------------------------------------------------------------|
| `/artists`   | name, image, members, creation year, first album                      |
| `/locations` | `tour_spread`: unique countries of the artist's concert locations     |
| `/dates`     | `concert_count` and the Rising Star recent rate: every listed date    |
| `/relation`  | the concerts table on the artist page: each location with its dates   |

Why the counts come from `/dates` and not `/relation`: `/relation` is a JSON
object keyed by location, so when an artist played in the same place on two
tours, only one group of dates survives there. This affects 3 artists in the
current data (Maroon 5: Florida, The Rolling Stones: California, Foo
Fighters: Massachusetts). `/dates` keeps every date: 441 in total against
438 in `/relation`. In `/dates`, a date starting with `*` opens the group
that belongs to the next entry of `/locations`.

## Artist Activity Classifier

A rule-based classifier: a weighted sum of touring features compared with
fixed thresholds. There is no machine learning and no external service.

```
score = tour_spread × 2 + concert_count × 3 + min(10, career_years / 5)
```

| Feature         | Meaning                                                        |
|-----------------|----------------------------------------------------------------|
| `tour_spread`   | number of different countries in `/locations`                  |
| `concert_count` | number of concert dates in `/dates`                            |
| `career_years`  | current year − creation year; integer division, so one point per full 5 years, at most 10 points |

| Label       | Score  |
|-------------|--------|
| Emerging    | 0–10   |
| Active      | 11–25  |
| Established | 26–40  |
| Legendary   | 41+    |

Example, Queen: 3 countries, 8 concerts, created 1970 →
`3×2 + 8×3 + min(10, 56/5) = 6 + 24 + 10 = 40` → **Established**.

The label is shown as a badge on every card and on the artist page. The
artist page also shows how each feature adds to the score.

`career_years` uses the current year at request time, so labels never go
stale while the server is running.

The code lives in `internal/activity`: `Score` is the formula,
`LabelForScore` maps a score to a label, and `Classify(tourSpread,
concertCount, careerYears)` returns the label. The audit example is a test
case: `Score(5, 8, 30) = 10 + 24 + 6 = 40` → Established.

With the current data the labels are: Active 5, Established 25, Legendary
22, and **no Emerging artist**. Even the artist with the fewest concerts,
Mamonas Assassinas, scores above 10: 1 country, 2 concerts, created 1995 →
`1×2 + 2×3 + min(10, 31/5) = 2 + 6 + 6 = 14` → Active. Every artist has at
least 2 concerts and a long enough career, so the Emerging range (0–10) is
never reached. The boundaries themselves (10/11, 25/26, 40/41) are covered
by unit tests.

## Bonus: Rising Star

As in the subject: an artist is a **Rising Star** when their concert rate
over the latest 2 years is more than 50% above their career average.

```
recent rate    = concerts in the latest 2 years / 2
career average = concert_count / career_years
rising         = recent rate > 1.5 × career average
```

The subject's example is a unit test: with a career average of 4 per year,
7 per year is rising (7 > 6) and 5 is not. Exactly 6 is not rising either,
because the rule says "more than".

* "Latest 2 years" means the 2 years before **today**: from the same date
  two years ago up to the moment of the request. Like `career_years`, it
  uses the time of the request, so the badge never goes stale.
* Upcoming concerts (after today) have not happened yet and are not counted.
* `concert_count` and `career_years` are the same values the classifier
  uses. A career shorter than a year counts as one year.
* The artist page shows both rates next to the badge, so the result can be
  checked by hand.

**Result with the current data: no artist is a Rising Star.** The API only
lists concerts up to 2020, so nobody has a concert in the latest 2 years
and every recent rate is 0. Example, Queen: 0 per year recently, against
8 / 56 = 0.14 per year over the career. The rule is implemented and tested
as written (`internal/activity/rising_test.go`, `TestStoreRisingStar`). An
artist with new concerts in the API would get the badge, and the store test
shows that Queen was a Rising Star when asked in 2021.

## Client-server event: live search

The home page has a search box, a label filter and a "Rising Stars"
checkbox. The menu shows how many artists each label has, for example
`Emerging (0)`. While the user types,
`static/app.js` waits 250 ms (debounce) and then calls the JSON API. It
rebuilds the cards without reloading the page and keeps the filters in the
address bar.

```
GET /api/artists?q=queen&label=Established&rising=1
```

```json
{
  "total": 52,
  "artists": [
    {"id": 1, "name": "Queen", "image": "...", "creationDate": 1970,
     "concertCount": 8, "score": 40, "label": "Established", "risingStar": false}
  ]
}
```

* `q` matches the name, members, concert cities and countries, ignoring case.
* `label` is one of the four labels, ignoring case. An empty value means any label.
* `rising=1` keeps only Rising Stars. Any other value is ignored.

Filtering runs on the server. The same function serves the HTML page
(`/?q=...&label=...`), so search also works with JavaScript turned off.

## Routes and errors

| Route              | Answer                                                    |
|--------------------|-----------------------------------------------------------|
| `GET /`            | artist cards, optionally filtered by `q`, `label`, `rising` |
| `GET /artist/{id}` | artist page                                               |
| `GET /api/artists` | JSON search API                                           |
| `GET /static/...`  | CSS, JS, icon                                             |

| Situation                             | Status | Page            |
|---------------------------------------|--------|-----------------|
| ID is not a positive number           | 400    | error page      |
| unknown label in a filter             | 400    | error page / JSON |
| unknown artist or page                | 404    | error page      |
| method other than GET or HEAD         | 405    | error page / JSON, with `Allow` header |
| template or handler failure, panic    | 500    | error page      |
| API data not loaded yet               | 503    | error page / JSON |

How the site stays up:

* Pages are rendered into a buffer first, so a template error never sends
  half a page with status 200.
* A middleware recovers from panics.
* API requests have a timeout and a 10 MB body limit.
* The server starts even when the API is down. It retries every 30 seconds
  and refreshes the data every hour. A failed reload keeps the old data.
* The server has read, write and idle timeouts.
* On Ctrl+C or SIGTERM the server shuts down gracefully and lets running
  requests finish.
* Every response carries `Content-Security-Policy`,
  `X-Content-Type-Options` and `Referrer-Policy` headers. Templates use
  `html/template` (auto-escaping) and `app.js` builds the DOM with
  `textContent`, so data from the API is never run as code.

## Bonus: goroutines and channels

* `store.Load` fetches the four endpoints **in parallel**. Each request runs
  in its own goroutine (`fetchAsync`) and sends its answer through a
  buffered channel, so no goroutine leaks, even when `Load` returns early on
  an error.
* `store.KeepFresh` runs in the background. A `select` waits on a timer and
  on context cancellation: it retries quickly while the API is down and
  refreshes hourly after that.
* `main` runs `ListenAndServe` in a goroutine and waits on a channel for
  either a server error or a shutdown signal.
* The store is protected by a `sync.RWMutex`. The tests run with `-race`.

## Bonus: public DNS

The binary needs nothing but itself, so deployment is:

```bash
GOOS=linux GOARCH=amd64 go build -o tour-track .   # build for the server
scp tour-track user@server:                        # copy one file
./tour-track -addr :8084                           # run (systemd or tmux in practice)
```

Then point a DNS `A` record (for example `tour-track.example.com`) at the
server's IP. Put a reverse proxy such as Caddy or nginx in front for HTTPS
on port 443.

## Data assumptions

* An artist missing from `/locations`, `/dates` or `/relation` gets no data
  from that endpoint. It is not an error.
* A location is `city-country`, split on the **last** `-`, and underscores
  become spaces (`new_york-usa` → New York, USA). The API never has a `-`
  inside a city or country name. A location without `-` is shown, but it
  does not count towards `tour_spread`.
* Alternative country spellings in the API are merged: `us` → USA,
  `brasil` → Brazil, `philippine` → Philippines, `czech_republic` → Czechia,
  `korea` → South Korea.
* `glasgow-australia` is a mistake in the API data (Glasgow is in the UK).
  It is shown as given.
* Dates use the `DD-MM-YYYY` format. A leading `*` is ignored. Invalid dates
  are skipped and not counted, and a location left without any valid date is
  not shown in the concerts table.
* Locations are shown formatted: `sao_paulo-brasil` becomes "Sao Paulo,
  Brazil".
* Concerts are sorted by first date, then by city, so the output does not
  depend on Go's random map order.
* Formula weights and label ranges are shown as text on the artist page.
  They come from the subject and do not change, so they are not passed in
  from Go.
