# Frontend Integration Guide

**Guide for UI developers building on the GiddyUp API**

## Quick Start

### 1. API Connection

```typescript
// config.ts
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000/api/v1';

// api/client.ts
export async function apiGet<T>(endpoint: string): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`);
  
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || 'API request failed');
  }
  
  return response.json();
}
```

### 2. Common UI Patterns

**Racecards (Today's Races)**:
```typescript
const races = await apiGet<Race[]>(`/races?date=${today}`);
```

**Horse Search**:
```typescript
const results = await apiGet<SearchResults>(`/search?q=${query}&limit=10`);
const horses = results.horses;
```

**Horse Profile Page**:
```typescript
const profile = await apiGet<HorseProfile>(`/horses/${id}/profile`);
```

---

## UI Feature Patterns

### Feature: Racecards

**Endpoint**: `GET /races?date={date}`

**UI Components Needed**:
1. Date picker
2. Course filter dropdown
3. Race type tabs (All, Flat, Jumps)
4. Race cards list

**Example Query**:
```typescript
async function getRacecards(date: string, courseId?: number) {
  let url = `/races?date=${date}`;
  if (courseId) {
    url = `/races/search?date_from=${date}&date_to=${date}&course_id=${courseId}`;
  }
  return apiGet<Race[]>(url);
}
```

**Sample UI**:
```
┌────────────────────────────────────┐
│ Date: [2024-01-01 ▼]  Course: [All▼]│
├────────────────────────────────────┤
│ 🏇 Ascot - 14:30                   │
│ Clarence House Chase (Grade 1)     │
│ 8 runners • Class 1 • 2m           │
│                           [View →] │
├────────────────────────────────────┤
│ 🏇 Kempton - 15:00                 │
│ ... │
└────────────────────────────────────┘
```

### Feature: Search

**Endpoint**: `GET /search?q={query}`

**UI Components**:
1. Search input with autocomplete
2. Result type tabs (Horses, Trainers, Jockeys)
3. Result cards with relevance score

**Example**:
```typescript
// Debounced search
const [query, setQuery] = useState('');
const [results, setResults] = useState<SearchResults | null>(null);

useEffect(() => {
  const timer = setTimeout(async () => {
    if (query.length > 0) {
      const data = await apiGet<SearchResults>(`/search?q=${query}&limit=10`);
      setResults(data);
    }
  }, 300);
  
  return () => clearTimeout(timer);
}, [query]);
```

### Feature: Horse Profile

**Endpoints**: 
- `GET /horses/{id}/profile` - Complete profile
- `GET /races?horse_id={id}` - Race history

**UI Sections**:
1. **Header**: Name, career summary
2. **Recent Form**: Last 10 runs table
3. **Statistics**: Going/distance/course splits (charts)
4. **Performance Trends**: RPR over time (line chart)

**Example**:
```typescript
interface HorseProfileProps {
  horseId: number;
}

export function HorseProfile({ horseId }: HorseProfileProps) {
  const { data: profile, isLoading } = useQuery(
    ['horse', horseId],
    () => apiGet<HorseProfile>(`/horses/${horseId}/profile`)
  );
  
  if (isLoading) return <Skeleton />;
  if (!profile) return <NotFound />;
  
  return (
    <div>
      <ProfileHeader 
        name={profile.horse.horse_name}
        runs={profile.career_summary.runs}
        wins={profile.career_summary.wins}
        peakRPR={profile.career_summary.peak_rpr}
      />
      
      <FormTable form={profile.recent_form} />
      
      <SplitsCharts 
        going={profile.going_splits}
        distance={profile.distance_splits}
      />
      
      <TrendChart data={profile.rpr_trend} />
    </div>
  );
}
```

### Feature: Market Movers

**Endpoint**: `GET /market/movers?date={date}&min_move={pct}`

**UI Pattern**: Real-time dashboard showing price movements

**Example**:
```typescript
function MarketMovers({ date }: { date: string }) {
  const { data } = useQuery(
    ['movers', date],
    () => apiGet<Mover[]>(`/market/movers?date=${date}&min_move=20`),
    { refetchInterval: 60000 } // Refresh every minute
  );
  
  const steamers = data?.filter(m => m.direction === 'steamer');
  const drifters = data?.filter(m => m.direction === 'drifter');
  
  return (
    <div className="grid grid-cols-2 gap-4">
      <div>
        <h3>💹 Steamers</h3>
        {steamers?.map(s => (
          <div key={s.horse_name}>
            {s.horse_name}: {s.morning_price} → {s.bsp}
            <span className="text-green-600">
              ({s.move_pct.toFixed(0)}%)
            </span>
          </div>
        ))}
      </div>
      
      <div>
        <h3>📉 Drifters</h3>
        {drifters?.map(d => (
          <div key={d.horse_name}>
            {d.horse_name}: {d.morning_price} → {d.bsp}
            <span className="text-red-600">
              (+{d.move_pct.toFixed(0)}%)
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

## TypeScript Definitions

### Core Types

```typescript
// types/race.ts
export interface Race {
  race_id: number;
  race_date: string;  // YYYY-MM-DD
  region: 'GB' | 'IRE';
  course_id?: number;
  course_name?: string;
  off_time?: string;  // HH:MM
  race_name: string;
  race_type: 'Flat' | 'Hurdle' | 'Chase' | 'NH Flat';
  class?: string;
  distance_f?: number;
  going?: string;
  surface?: string;
  ran: number;
}

export interface Runner {
  runner_id: number;
  horse_id?: number;
  horse_name?: string;
  trainer_id?: number;
  trainer_name?: string;
  jockey_id?: number;
  jockey_name?: string;
  num?: number;
  pos_num?: number;
  pos_raw?: string;
  draw?: number;
  age?: number;
  lbs?: number;
  or?: number;
  rpr?: number;
  win_bsp?: number;
  win_ppwap?: number;
  place_bsp?: number;
  dec?: number;
  win_flag?: boolean;
  comment?: string;
}

export interface RaceWithRunners extends Race {
  runners: Runner[];
}

export interface HorseProfile {
  horse: {
    horse_id: number;
    horse_name: string;
  };
  career_summary: {
    runs: number;
    wins: number;
    places: number;
    total_prize: number;
    avg_rpr?: number;
    peak_rpr?: number;
    avg_or?: number;
    peak_or?: number;
  };
  recent_form: FormEntry[];
  going_splits: StatsSplit[];
  distance_splits: StatsSplit[];
  course_splits: StatsSplit[];
  rpr_trend: TrendPoint[];
}

export interface SearchResults {
  horses: SearchResult[];
  trainers: SearchResult[];
  jockeys: SearchResult[];
  owners: SearchResult[];
  courses: SearchResult[];
  total_results: number;
}

export interface SearchResult {
  id: number;
  name: string;
  score: number;  // 0.0-1.0
  type: 'horse' | 'trainer' | 'jockey' | 'owner' | 'course';
}
```

---

## Performance Best Practices

### 1. Caching Strategy

```typescript
// Use React Query for automatic caching
import { useQuery } from '@tanstack/react-query';

// Cache for 5 minutes
const { data: courses } = useQuery(
  ['courses'],
  () => apiGet<Course[]>('/courses'),
  { staleTime: 5 * 60 * 1000 }
);

// Cache profile for 1 hour
const { data: profile } = useQuery(
  ['horse', horseId],
  () => apiGet<HorseProfile>(`/horses/${horseId}/profile`),
  { staleTime: 60 * 60 * 1000 }
);
```

### 2. Avoid Over-Fetching

```typescript
// ❌ Don't fetch full race if you only need the list
const races = await apiGet<RaceWithRunners[]>('/races?date=2024-01-01');

// ✅ Fetch race list first, then details on-demand
const races = await apiGet<Race[]>('/races?date=2024-01-01');
// User clicks a race:
const raceDetail = await apiGet<RaceWithRunners>(`/races/${raceId}`);
```

### 3. Loading States

```typescript
function RaceCard({ date }: { date: string }) {
  const { data: races, isLoading, error } = useQuery(
    ['races', date],
    () => apiGet<Race[]>(`/races?date=${date}`)
  );
  
  if (isLoading) return <Skeleton count={5} />;
  if (error) return <ErrorMessage error={error} />;
  if (!races || races.length === 0) return <NoRaces date={date} />;
  
  return <RaceList races={races} />;
}
```

### 4. Pagination

```typescript
function useRacesPaginated(filters: RaceFilters, page: number = 1) {
  const limit = 50;
  const offset = (page - 1) * limit;
  
  return useQuery(
    ['races', 'search', filters, page],
    () => apiGet<Race[]>(
      `/races/search?${new URLSearchParams({
        ...filters,
        limit: limit.toString(),
        offset: offset.toString()
      })}`
    )
  );
}
```

---

## Common UI Patterns

### Pattern 1: Autocomplete Search

```typescript
function HorseSearchAutocomplete() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  
  // Debounced search
  useEffect(() => {
    if (query.length === 0) {
      setResults([]);
      return;
    }
    
    const timer = setTimeout(async () => {
      const data = await apiGet<SearchResults>(`/search?q=${query}&limit=5`);
      setResults(data.horses);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [query]);
  
  return (
    <Combobox value={selected} onChange={setSelected}>
      <ComboboxInput onChange={(e) => setQuery(e.target.value)} />
      <ComboboxOptions>
        {results.map((horse) => (
          <ComboboxOption key={horse.id} value={horse}>
            {horse.name} <span className="text-gray-500">({horse.score.toFixed(2)})</span>
          </ComboboxOption>
        ))}
      </ComboboxOptions>
    </Combobox>
  );
}
```

### Pattern 2: Form Table

```typescript
function FormTable({ form }: { form: FormEntry[] }) {
  return (
    <table>
      <thead>
        <tr>
          <th>Date</th>
          <th>Course</th>
          <th>Dist</th>
          <th>Going</th>
          <th>Pos</th>
          <th>BTN</th>
          <th>RPR</th>
          <th>OR</th>
          <th>BSP</th>
          <th>Trainer</th>
          <th>Jockey</th>
        </tr>
      </thead>
      <tbody>
        {form.map((run, idx) => (
          <tr key={idx} className={run.pos_num === 1 ? 'bg-green-50' : ''}>
            <td>{run.race_date}</td>
            <td>{run.course_name}</td>
            <td>{run.dist_f}f</td>
            <td>{run.going}</td>
            <td className="font-bold">{run.pos_num}</td>
            <td>{run.btn?.toFixed(2)}</td>
            <td>{run.rpr}</td>
            <td>{run.or}</td>
            <td>{run.win_bsp?.toFixed(2)}</td>
            <td>{run.trainer_name}</td>
            <td>{run.jockey_name}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
```

### Pattern 3: Performance Charts

```typescript
import { LineChart, Line, XAxis, YAxis, Tooltip } from 'recharts';

function RPRTrendChart({ data }: { data: TrendPoint[] }) {
  return (
    <LineChart width={600} height={300} data={data}>
      <XAxis dataKey="date" />
      <YAxis />
      <Tooltip />
      <Line 
        type="monotone" 
        dataKey="rpr" 
        stroke="#8884d8" 
        strokeWidth={2}
        dot={{ fill: (point) => point.win_flag ? 'green' : 'blue' }}
      />
    </LineChart>
  );
}
```

---

## Sample Pages

### Home Page

```typescript
export default function HomePage() {
  const today = new Date().toISOString().split('T')[0];
  const { data: races } = useQuery(['races', today], 
    () => apiGet<Race[]>(`/races?date=${today}`));
  
  return (
    <div>
      <h1>Today's Racing</h1>
      <RaceCardsList races={races} />
      
      <h2>Market Movers</h2>
      <MarketMovers date={today} />
    </div>
  );
}
```

### Search Page

```typescript
export default function SearchPage() {
  const [query, setQuery] = useState('');
  const { data } = useQuery(
    ['search', query],
    () => apiGet<SearchResults>(`/search?q=${query}`),
    { enabled: query.length > 0 }
  );
  
  return (
    <div>
      <SearchInput value={query} onChange={setQuery} />
      {data && <SearchResults results={data} />}
    </div>
  );
}
```

### Horse Profile Page

```typescript
export default function HorseProfilePage({ params }: { params: { id: string } }) {
  const { data: profile, isLoading } = useQuery(
    ['horse', params.id],
    () => apiGet<HorseProfile>(`/horses/${params.id}/profile`)
  );
  
  if (isLoading) return <ProfileSkeleton />;
  if (!profile) return <NotFound />;
  
  return (
    <div>
      <ProfileHeader profile={profile} />
      <CareerStats stats={profile.career_summary} />
      <FormTable form={profile.recent_form} />
      <SplitsSection splits={profile} />
      <TrendCharts trend={profile.rpr_trend} />
    </div>
  );
}
```

---

## Performance Tips

### 1. Lazy Load Heavy Data

```typescript
// Load profile header first (fast)
const { data: horse } = useQuery(['horse', id], 
  () => apiGet(`/horses/${id}`));

// Load form in separate query (can be slow)
const { data: form } = useQuery(['horse-form', id], 
  () => apiGet(`/horses/${id}/form`),
  { enabled: !!horse } // Only load after horse loads
);
```

### 2. Prefetch Common Data

```typescript
// Prefetch courses on app load (89 courses, <1ms)
useEffect(() => {
  queryClient.prefetchQuery(['courses'], 
    () => apiGet<Course[]>('/courses'));
}, []);
```

### 3. Show Stale Data While Revalidating

```typescript
const { data, isStale } = useQuery(
  ['races', date],
  () => apiGet<Race[]>(`/races?date=${date}`),
  {
    staleTime: 60000, // 1 minute
    cacheTime: 300000, // 5 minutes
  }
);

// Show stale data with indicator
{isStale && <Badge>Updating...</Badge>}
```

---

## Error Handling

```typescript
// Global error handler
function APIError({ error }: { error: Error }) {
  // Parse API error
  const message = error.message || 'Something went wrong';
  
  // Check for specific errors
  if (message.includes('not found')) {
    return <NotFoundPage />;
  }
  
  if (message.includes('network')) {
    return <OfflineBanner />;
  }
  
  return (
    <ErrorCard 
      title="Error"
      message={message}
      retry={() => window.location.reload()}
    />
  );
}

// Use in components
const { data, error } = useQuery(...);
if (error) return <APIError error={error} />;
```

---

## Quick Reference

### Most Used Endpoints

| Use Case | Endpoint | Response Time |
|----------|----------|---------------|
| Today's races | `GET /races?date={today}` | 1-5ms |
| Search horse | `GET /search?q={query}` | 100ms |
| Horse profile | `GET /horses/{id}/profile` | 10-50ms |
| Course list | `GET /courses` | <1ms |
| Market movers | `GET /market/movers` | 150ms |

### Sample API Calls

```bash
# Get all courses (cache this!)
curl "http://localhost:8000/api/v1/courses"

# Today's races
curl "http://localhost:8000/api/v1/races?date=$(date +%Y-%m-%d)"

# Search
curl "http://localhost:8000/api/v1/search?q=Enable&limit=5"

# Horse profile
curl "http://localhost:8000/api/v1/horses/1/profile"

# Race detail with runners
curl "http://localhost:8000/api/v1/races/123"
```

---

## Resources

- **Full API Reference**: `02_API_DOCUMENTATION.md`
- **Developer Guide**: `01_DEVELOPER_GUIDE.md`
- **Database Schema**: `03_DATABASE_GUIDE.md`

---

**Last Updated**: October 15, 2025  
**API Version**: 1.0.0  
**Status**: ✅ Ready for Frontend Development

