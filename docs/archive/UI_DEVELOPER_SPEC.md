iddyUp UI — Product & Engineering Spec (v1.0)

Goal: ship a first-class, modern UI for a working horse-racing database where punters can search horses and link data across horses/trainers/jockeys/markets to find an edge. This doc is what I’d hand a UI dev to build something scalable, fast, and easy to use from day one.

1) What we’re building (MVP → v1)
Core user jobs (MVP)

Search anything (focus: horses; also trainers/jockeys/courses) with fuzzy match + autocomplete.

Open a horse profile with career summary, recent form, splits (going/distance/course), and rating trends.

See today’s racing (racecards) with quick filters (course, race type).

Market context: a lightweight “Steamers & Drifters” dashboard (refreshed every 60s).

First upgrades (v1)

Advanced search: race finder with date range, field size, class, type.

Edge surfaces: draw bias quick-look on racecards; DSR (recency) hints on horse profile.

Linking: cross-nav chips from horse → trainer → jockey profiles (trainer/jockey endpoints are slower; use on-demand + cache).

All endpoints are already available in the API and documented with typical latencies—use those to drive UI loading strategies.

2) Information architecture & pages
Global layout

Top bar: Search box (debounced), date picker (defaults to today), nav (Home, Search, Insights).

Left rail (lg screens): Filters contextually (course, race type, class).

Content area: Cards/tables/charts; responsive grid.

Pages

Home

“Today’s Racing” racecards list.

“Market Movers” panel (Steamers/Drifters, auto-refresh 60s).

Search

One search box → tabs for Horses / Trainers / Jockeys / Courses (start focused on Horses).

Typeahead (limit 5–10), fuzzy match score.

Horse Profile

Header: name, career stats (runs/wins/places, peak/avg RPR/OR).

Recent Form table (last 10) + RPR trend chart.

Splits: going/distance/course.

Quick links: trainer, jockey, recent courses.

Insights (v1)

Draw Bias mini-explorer (course + distance filters).

Recency (DSR) overview.

3) Feature → API mapping (and performance notes)
UI Feature	Endpoint(s)	Notes
Global search / autocomplete	GET /search?q=	Fuzzy search; show score; limit per type. Typical 100–150 ms.
Today’s racecards	GET /races?date=	1–5 ms; paginate if >50.
Race detail (on demand)	GET /races/{id}	Lazy-load runners; 50–200 ms.
Horse profile	GET /horses/{id}/profile	10–50 ms via MV; cache 1h.
Market movers	GET /market/movers?date&min_move	Refresh every 60 s. ~150 ms.
Draw bias	GET /bias/draw?...	~3–4 s → prefetch on route hover; show skeleton.
Recency analysis	GET /analysis/recency?...	~1–2 s; cache & background refresh.

Data model references (names/fields) live in DB guide (races/runners/horses, MV mv_runner_base)—use to shape TS types.

4) Tech stack & project scaffolding

Framework: Next.js (App Router) + TypeScript for SSR/ISR and route-level data fetching.

State/data: TanStack Query for fetching, caching, revalidation; tiny global store (Zustand) for UI state (filters, date). Patterns align with the Frontend Guide.

UI: TailwindCSS + shadcn/ui components; Recharts for charts (already referenced).

HTTP: small apiGet<T>() wrapper with typed responses.

Env: NEXT_PUBLIC_API_URL pointing at …/api/v1.

Suggested structure
/app
  /(public) home, search, horse/[id]
  /insights/draw-bias, /insights/recency
/components
  SearchBox/, RaceCard/, HorseProfile/, MoversPanel/, Charts/
/lib
  api/client.ts, api/races.ts, api/search.ts, api/profiles.ts
  dates.ts, formatters.ts
/types
  race.ts, runner.ts, horse.ts, search.ts


Types can be lifted from the Frontend Guide samples and expanded with DB fields as needed.

5) UX patterns that keep it fast

Debounced search (300 ms) with autocomplete; cancel inflight requests on new keystrokes.

Optimistic routing + skeletons for racecards and profiles; error boundaries with retry.

Lazy detail: list first, fetch detail when a row/card expands.

Aggressive caching:

Courses list: 5–60 min.

Horse profiles: 1 h (MV makes them stable).

Movers: 60 s refetch interval.

Pagination for any list >50 (offset model on API).

Show stale-while-revalidate badges (“Updating…”) to keep UI snappy.

6) Component sketches (functional contracts)
<SearchBox />

Props: onSelect(result); local state for query; debounced fetch GET /search?q=&limit=5.

Renders a results menu with type chips & score.

<HorseProfile />

Fetch: GET /horses/{id}/profile via Query with staleTime: 3600_000.

Sections: Header (name + KPIs), FormTable, SplitsCharts, RPRTrendChart.

<RaceCardsList />

Fetch: GET /races?date=; filter by course/type locally; expand card → GET /races/{id}.

<MoversPanel />

Fetch: GET /market/movers?date=&min_move=20, refetchInterval: 60000; two columns (Steamers/Drifters).

7) Visual design language

Tone: calm, analytical; emphasize readability of tables and clear deltas (e.g., +65% in green).

Cards with subtle shadows, rounded corners; responsive grid (2–3 columns desktop, 1 on mobile).

Iconography: course, distance (f), going badges; small emojis can help in MVP, later swap to icon set.

Charts: single focus line (RPR) with win markers; keep legends simple. (Recharts baseline from guide.)

Dark mode from the start (Tailwind + data-theme).

8) Performance budget & targets

LCP < 2.5 s on Home (warm cache); TTI < 3.5 s.

Route transitions < 200 ms for cached data; slow endpoints must stream skeletons immediately and settle within their documented times (Draw Bias 3–4 s, Trainer/Jockey profiles 3–9 s—fetch only on explicit user action).

Bundle: code-split charts & heavy tables; keep initial JS < 180 KB gz.

9) Data correctness & shaping

Prefer the API’s typed shape over client derivations; enrich only for presentation. See example TS types (Race, Runner, HorseProfile) in the Frontend Guide, and cross-check DB columns (runners/races, MV fields).

Convert distances (furlongs ↔ m) and times only for display; keep raw values for compute.

Normalize going labels and display as chips; rely on backend’s normalized text fields for matching.

10) Error handling, loading, empty states

Global API error component that parses {"error": "…"} and renders helpful action (retry / go back).

Empty search: show suggestions (recent winners, popular horses).

No races today: display next available date with 1-click jump.

Movers empty: explain “No movers ≥ X% yet.”

11) Accessibility & i18n

Keyboard nav in search/autocomplete; ARIA roles for menus/tables.

Color contrast AA minimum; avoid color-only indicators (use +/– and icons).

Copy all labels to a dictionary; date formats YYYY-MM-DD (consistency with API).

12) Security & platform considerations

In dev, API auth is open; in prod plan for API key header (Authorization: Bearer {key}) with rate-limit messaging. Build header injection via env.

CORS origins: read from API config; local dev: http://localhost:3000.

Avoid exposing internal IDs in URLs beyond what endpoints already require.

13) Telemetry & quality

Web Vitals to console (Next.js) + simple analytics eventing (search performed, profile opened, bias viewed).

Hook into API health (ping on app start; log failures), optional status pill in footer.

14) Delivery plan

Week 1

Scaffold Next.js + Tailwind + shadcn + TanStack Query; wire apiGet; build Search page with autocomplete.

Week 2

Home racecards + filters; lazy race detail; Movers panel w/ auto-refresh.

Week 3

Horse Profile (form table, splits, trend chart) + caching; link chips to trainer/jockey (click → toast “slow—loading” + fetch).

Week 4

Insights (Draw Bias + Recency) with skeletons and result caching; polish, a11y, dark mode.

15) Nice-to-haves (backlog)

Trainer/Jockey profile pages (heavier aggregations—gate behind click).

Comment search explorer (/search/comments) for pattern hunting.

Book vs Exchange quick panel.

16) Quick endpoint cheatsheet (dev)

Courses: GET /courses (cache long).

Races (date): GET /races?date=YYYY-MM-DD (fast).

Race detail: GET /races/{id} (lazy).

Search: GET /search?q=...&limit=10 (debounced).

Horse profile: GET /horses/{id}/profile (cache 1h).

Movers: GET /market/movers?date=&min_move=20 (refetch 60s).

Insights: GET /bias/draw?..., GET /analysis/recency?... (show skeleton).

17) References

Frontend Integration Guide — patterns, types, examples used above.

API Documentation — endpoints, parameters, response times.

Database Guide — schema and MV powering fast profiles.

Start Here / README — overall scope & dataset scale.