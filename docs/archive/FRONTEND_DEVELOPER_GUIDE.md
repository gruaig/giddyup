# Frontend Developer Guide - Racing Analytics UI

## 🎯 Overview

Build a modern, responsive web application for **30 racing analytics features** consuming the Racing Data API. Framework choice is flexible (React, Vue, Svelte, etc.).

---

## 🏗️ Tech Stack Recommendations

### Core Framework Options

#### Option A: React + TypeScript (Recommended)
```bash
- React 18+ with TypeScript
- TanStack Query (React Query) for data fetching
- TailwindCSS + shadcn/ui for components
- Recharts/Victory for data visualization
- Zustand or Redux Toolkit for state management
```

#### Option B: Next.js (Full-Stack)
```bash
- Next.js 14+ App Router
- Server Components for initial load
- API routes for BFF pattern
- Built-in caching and optimization
```

#### Option C: Vue 3 + Vite
```bash
- Vue 3 Composition API
- Pinia for state management
- Vuetify or PrimeVue for UI components
- Chart.js for visualizations
```

### Essential Libraries
- **Data Tables**: TanStack Table, AG Grid, or MUI DataGrid
- **Charts**: Recharts, Chart.js, or D3.js
- **Forms**: React Hook Form or Formik
- **Date Handling**: date-fns or dayjs
- **HTTP Client**: Axios or Fetch API with React Query
- **Notifications**: React Hot Toast or Sonner

---

## 📊 API Integration

### Base Configuration

```typescript
// src/config/api.ts
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000/api/v1';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth interceptor
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});
```

### React Query Setup

```typescript
// src/lib/queryClient.ts
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes
      cacheTime: 10 * 60 * 1000, // 10 minutes
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
```

---

## 🎨 Component Architecture

### Feature Structure
```
src/
├── features/
│   ├── search/
│   │   ├── components/
│   │   │   ├── GlobalSearchBar.tsx
│   │   │   ├── SearchResults.tsx
│   │   │   └── CommentSearch.tsx
│   │   ├── hooks/
│   │   │   └── useGlobalSearch.ts
│   │   └── api/
│   │       └── searchApi.ts
│   ├── profiles/
│   │   ├── components/
│   │   │   ├── HorseProfile.tsx
│   │   │   ├── TrainerProfile.tsx
│   │   │   └── JockeyProfile.tsx
│   │   └── hooks/
│   ├── races/
│   │   ├── components/
│   │   │   ├── RaceExplorer.tsx
│   │   │   ├── RaceDashboard.tsx
│   │   │   └── HeadToHead.tsx
│   │   └── hooks/
│   ├── market/
│   │   ├── components/
│   │   │   ├── MarketMovers.tsx
│   │   │   ├── Calibration.tsx
│   │   │   └── InPlayAnalysis.tsx
│   │   └── hooks/
│   └── bias/
│       ├── components/
│       │   ├── DrawBias.tsx
│       │   └── FormSplits.tsx
│       └── hooks/
├── components/
│   ├── ui/         # Reusable UI components
│   ├── charts/     # Chart components
│   └── layout/     # Layout components
├── hooks/          # Shared hooks
├── types/          # TypeScript types
└── utils/          # Utility functions
```

---

## 🔑 TypeScript Types

### Core Data Types

```typescript
// src/types/racing.ts

export interface Horse {
  id: number;
  name: string;
  score?: number; // For search results
}

export interface Trainer {
  id: number;
  name: string;
}

export interface Jockey {
  id: number;
  name: string;
}

export interface Course {
  id: number;
  name: string;
  region: 'GB' | 'IRE';
}

export interface Race {
  id: number;
  race_key: string;
  date: string;
  off_time: string;
  course: Course;
  race_name: string;
  race_type: 'flat' | 'jumps';
  class?: string;
  pattern?: string;
  dist_f?: number;
  dist_raw?: string;
  going?: string;
  surface?: string;
  ran: number;
}

export interface Runner {
  id: number;
  race_id: number;
  num?: number;
  draw?: number;
  horse: Horse;
  age?: number;
  sex?: string;
  lbs?: number;
  or?: number;
  rpr?: number;
  trainer?: Trainer;
  jockey?: Jockey;
  pos_raw?: string;
  pos_num?: number;
  btn?: number;
  comment?: string;
  win_bsp?: number;
  win_ppwap?: number;
  win_ppmax?: number;
  win_ppmin?: number;
  place_bsp?: number;
  dec?: number;
}

export interface HorseProfile {
  horse: Horse;
  career_summary: {
    runs: number;
    wins: number;
    places: number;
    total_prize: number;
    avg_rpr: number;
    peak_rpr: number;
  };
  recent_form: Runner[];
  going_splits: StatsSplit[];
  distance_splits: StatsSplit[];
  course_splits: StatsSplit[];
  rpr_trend: TrendPoint[];
}

export interface StatsSplit {
  category: string;
  runs: number;
  wins: number;
  sr: number;
  roi?: number;
  avg_rpr?: number;
}

export interface TrendPoint {
  date: string;
  value: number;
  label?: string;
}

export interface SearchResults {
  horses: Horse[];
  trainers: Trainer[];
  jockeys: Jockey[];
  owners: { id: number; name: string }[];
  courses: Course[];
  total_results: number;
}
```

---

## 📋 Feature Implementation Guide

### **1. Search & Navigation**

#### Feature 1: Global Search Bar

**Component**: `src/features/search/components/GlobalSearchBar.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { searchApi } from '../api/searchApi';

export function GlobalSearchBar() {
  const [query, setQuery] = useState('');
  const [isOpen, setIsOpen] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ['globalSearch', query],
    queryFn: () => searchApi.search(query),
    enabled: query.length >= 2,
  });

  return (
    <div className="relative">
      <input
        type="text"
        placeholder="Search horses, trainers, jockeys..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onFocus={() => setIsOpen(true)}
        className="w-full px-4 py-2 border rounded-lg"
      />
      
      {isOpen && data && (
        <div className="absolute top-full mt-2 w-full bg-white border rounded-lg shadow-lg z-50">
          {/* Horses */}
          {data.horses.length > 0 && (
            <div className="p-2">
              <h3 className="text-sm font-semibold text-gray-500">Horses</h3>
              {data.horses.map((horse) => (
                <a
                  key={horse.id}
                  href={`/horses/${horse.id}`}
                  className="block px-2 py-1 hover:bg-gray-100 rounded"
                >
                  {horse.name}
                </a>
              ))}
            </div>
          )}
          
          {/* Trainers */}
          {data.trainers.length > 0 && (
            <div className="p-2">
              <h3 className="text-sm font-semibold text-gray-500">Trainers</h3>
              {data.trainers.map((trainer) => (
                <a
                  key={trainer.id}
                  href={`/trainers/${trainer.id}`}
                  className="block px-2 py-1 hover:bg-gray-100 rounded"
                >
                  {trainer.name}
                </a>
              ))}
            </div>
          )}
          
          {/* Similar for jockeys, courses */}
        </div>
      )}
    </div>
  );
}
```

**API Hook**: `src/features/search/hooks/useGlobalSearch.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';
import type { SearchResults } from '@/types/racing';

export function useGlobalSearch(query: string) {
  return useQuery<SearchResults>({
    queryKey: ['search', query],
    queryFn: async () => {
      const { data } = await apiClient.get('/search', {
        params: { q: query, limit: 10 },
      });
      return data;
    },
    enabled: query.length >= 2,
    staleTime: 60000, // 1 minute
  });
}
```

---

#### Feature 2: Advanced Comment Search

**Component**: `src/features/search/components/CommentSearch.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';

interface CommentSearchFilters {
  query: string;
  dateFrom?: string;
  dateTo?: string;
  region?: string;
  courseId?: number;
}

export function CommentSearch() {
  const [filters, setFilters] = useState<CommentSearchFilters>({
    query: '',
  });

  const { data, isLoading } = useQuery({
    queryKey: ['commentSearch', filters],
    queryFn: async () => {
      const { data } = await apiClient.get('/search/comments', {
        params: filters,
      });
      return data;
    },
    enabled: filters.query.length >= 3,
  });

  return (
    <div className="space-y-4">
      {/* Search Input */}
      <input
        type="text"
        placeholder="Search comments... (e.g., 'led throughout')"
        value={filters.query}
        onChange={(e) => setFilters({ ...filters, query: e.target.value })}
        className="w-full px-4 py-2 border rounded-lg"
      />

      {/* Filters */}
      <div className="flex gap-4">
        <input
          type="date"
          value={filters.dateFrom || ''}
          onChange={(e) => setFilters({ ...filters, dateFrom: e.target.value })}
          className="px-3 py-2 border rounded"
        />
        <input
          type="date"
          value={filters.dateTo || ''}
          onChange={(e) => setFilters({ ...filters, dateTo: e.target.value })}
          className="px-3 py-2 border rounded"
        />
        <select
          value={filters.region || ''}
          onChange={(e) => setFilters({ ...filters, region: e.target.value })}
          className="px-3 py-2 border rounded"
        >
          <option value="">All Regions</option>
          <option value="GB">Great Britain</option>
          <option value="IRE">Ireland</option>
        </select>
      </div>

      {/* Results */}
      {isLoading && <div>Loading...</div>}
      {data && (
        <div className="space-y-2">
          {data.map((result: any) => (
            <div key={result.runner_id} className="p-4 border rounded-lg">
              <div className="flex justify-between">
                <div>
                  <a href={`/races/${result.race_id}`} className="font-semibold text-blue-600">
                    {result.course_name} - {result.race_date}
                  </a>
                  <p className="text-sm text-gray-600">{result.horse_name}</p>
                </div>
              </div>
              <p className="mt-2 text-sm italic">&ldquo;{result.comment}&rdquo;</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

---

### **2. Profiles**

#### Feature 4: Horse Profile

**Component**: `src/features/profiles/components/HorseProfile.tsx`

```typescript
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { apiClient } from '@/config/api';
import { StatsSplitTable } from './StatsSplitTable';
import { RPRTrendChart } from './RPRTrendChart';
import { RecentForm } from './RecentForm';

export function HorseProfile() {
  const { horseId } = useParams();

  const { data: profile, isLoading } = useQuery({
    queryKey: ['horseProfile', horseId],
    queryFn: async () => {
      const { data } = await apiClient.get(`/horses/${horseId}/profile`);
      return data;
    },
  });

  if (isLoading) return <div>Loading...</div>;
  if (!profile) return <div>Horse not found</div>;

  return (
    <div className="max-w-7xl mx-auto p-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">{profile.horse.name}</h1>
      </div>

      {/* Career Summary */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="p-4 bg-white border rounded-lg">
          <div className="text-2xl font-bold">{profile.career_summary.runs}</div>
          <div className="text-sm text-gray-600">Total Runs</div>
        </div>
        <div className="p-4 bg-white border rounded-lg">
          <div className="text-2xl font-bold">{profile.career_summary.wins}</div>
          <div className="text-sm text-gray-600">Wins</div>
        </div>
        <div className="p-4 bg-white border rounded-lg">
          <div className="text-2xl font-bold">
            {((profile.career_summary.wins / profile.career_summary.runs) * 100).toFixed(1)}%
          </div>
          <div className="text-sm text-gray-600">Strike Rate</div>
        </div>
        <div className="p-4 bg-white border rounded-lg">
          <div className="text-2xl font-bold">{profile.career_summary.peak_rpr}</div>
          <div className="text-sm text-gray-600">Peak RPR</div>
        </div>
      </div>

      {/* Recent Form */}
      <div>
        <h2 className="text-xl font-semibold mb-4">Recent Form</h2>
        <RecentForm runs={profile.recent_form} />
      </div>

      {/* RPR Trend */}
      <div>
        <h2 className="text-xl font-semibold mb-4">RPR/OR Trend</h2>
        <RPRTrendChart data={profile.rpr_trend} />
      </div>

      {/* Splits */}
      <div className="grid md:grid-cols-2 gap-6">
        <div>
          <h2 className="text-xl font-semibold mb-4">Going Splits</h2>
          <StatsSplitTable splits={profile.going_splits} />
        </div>
        <div>
          <h2 className="text-xl font-semibold mb-4">Distance Splits</h2>
          <StatsSplitTable splits={profile.distance_splits} />
        </div>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-4">Course Splits</h2>
        <StatsSplitTable splits={profile.course_splits} />
      </div>
    </div>
  );
}
```

**Chart Component**: `src/features/profiles/components/RPRTrendChart.tsx`

```typescript
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

interface RPRTrendChartProps {
  data: Array<{
    date: string;
    rpr?: number;
    or?: number;
  }>;
}

export function RPRTrendChart({ data }: RPRTrendChartProps) {
  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis 
          dataKey="date" 
          tickFormatter={(date) => new Date(date).toLocaleDateString()}
        />
        <YAxis />
        <Tooltip 
          labelFormatter={(date) => new Date(date).toLocaleDateString()}
        />
        <Legend />
        <Line 
          type="monotone" 
          dataKey="rpr" 
          stroke="#8884d8" 
          name="RPR" 
          connectNulls
        />
        <Line 
          type="monotone" 
          dataKey="or" 
          stroke="#82ca9d" 
          name="OR" 
          connectNulls
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
```

**Stats Table**: `src/features/profiles/components/StatsSplitTable.tsx`

```typescript
interface StatsSplitTableProps {
  splits: Array<{
    category: string;
    runs: number;
    wins: number;
    sr: number;
    roi?: number;
    avg_rpr?: number;
  }>;
}

export function StatsSplitTable({ splits }: StatsSplitTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-4 py-2 text-left">Category</th>
            <th className="px-4 py-2 text-right">Runs</th>
            <th className="px-4 py-2 text-right">Wins</th>
            <th className="px-4 py-2 text-right">SR%</th>
            {splits[0]?.roi !== undefined && (
              <th className="px-4 py-2 text-right">ROI%</th>
            )}
            {splits[0]?.avg_rpr !== undefined && (
              <th className="px-4 py-2 text-right">Avg RPR</th>
            )}
          </tr>
        </thead>
        <tbody>
          {splits.map((split) => (
            <tr key={split.category} className="border-t">
              <td className="px-4 py-2">{split.category}</td>
              <td className="px-4 py-2 text-right">{split.runs}</td>
              <td className="px-4 py-2 text-right">{split.wins}</td>
              <td className="px-4 py-2 text-right">{split.sr.toFixed(1)}%</td>
              {split.roi !== undefined && (
                <td className={`px-4 py-2 text-right ${split.roi > 0 ? 'text-green-600' : 'text-red-600'}`}>
                  {split.roi > 0 ? '+' : ''}{split.roi.toFixed(1)}%
                </td>
              )}
              {split.avg_rpr !== undefined && (
                <td className="px-4 py-2 text-right">{split.avg_rpr.toFixed(0)}</td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

---

### **3. Race Exploration**

#### Feature 8: Race Explorer

**Component**: `src/features/races/components/RaceExplorer.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';

interface RaceFilters {
  dateFrom: string;
  dateTo: string;
  region?: string;
  courseId?: number;
  type?: string;
  class?: string;
  distMin?: number;
  distMax?: number;
  going?: string;
}

export function RaceExplorer() {
  const today = new Date().toISOString().split('T')[0];
  const weekAgo = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];

  const [filters, setFilters] = useState<RaceFilters>({
    dateFrom: weekAgo,
    dateTo: today,
  });

  const { data, isLoading } = useQuery({
    queryKey: ['raceExplorer', filters],
    queryFn: async () => {
      const { data } = await apiClient.get('/races/search', { params: filters });
      return data;
    },
  });

  return (
    <div className="max-w-7xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Race Explorer</h1>

      {/* Filters */}
      <div className="bg-white p-6 rounded-lg border mb-6">
        <div className="grid md:grid-cols-4 gap-4">
          {/* Date Range */}
          <div>
            <label className="block text-sm font-medium mb-1">Date From</label>
            <input
              type="date"
              value={filters.dateFrom}
              onChange={(e) => setFilters({ ...filters, dateFrom: e.target.value })}
              className="w-full px-3 py-2 border rounded"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Date To</label>
            <input
              type="date"
              value={filters.dateTo}
              onChange={(e) => setFilters({ ...filters, dateTo: e.target.value })}
              className="w-full px-3 py-2 border rounded"
            />
          </div>

          {/* Region */}
          <div>
            <label className="block text-sm font-medium mb-1">Region</label>
            <select
              value={filters.region || ''}
              onChange={(e) => setFilters({ ...filters, region: e.target.value || undefined })}
              className="w-full px-3 py-2 border rounded"
            >
              <option value="">All</option>
              <option value="GB">Great Britain</option>
              <option value="IRE">Ireland</option>
            </select>
          </div>

          {/* Type */}
          <div>
            <label className="block text-sm font-medium mb-1">Type</label>
            <select
              value={filters.type || ''}
              onChange={(e) => setFilters({ ...filters, type: e.target.value || undefined })}
              className="w-full px-3 py-2 border rounded"
            >
              <option value="">All</option>
              <option value="flat">Flat</option>
              <option value="jumps">Jumps</option>
            </select>
          </div>

          {/* Class */}
          <div>
            <label className="block text-sm font-medium mb-1">Class</label>
            <select
              value={filters.class || ''}
              onChange={(e) => setFilters({ ...filters, class: e.target.value || undefined })}
              className="w-full px-3 py-2 border rounded"
            >
              <option value="">All</option>
              {[1, 2, 3, 4, 5, 6, 7].map((c) => (
                <option key={c} value={c}>Class {c}</option>
              ))}
            </select>
          </div>

          {/* Distance */}
          <div>
            <label className="block text-sm font-medium mb-1">Min Distance (f)</label>
            <input
              type="number"
              value={filters.distMin || ''}
              onChange={(e) => setFilters({ ...filters, distMin: e.target.value ? Number(e.target.value) : undefined })}
              className="w-full px-3 py-2 border rounded"
              placeholder="5"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Max Distance (f)</label>
            <input
              type="number"
              value={filters.distMax || ''}
              onChange={(e) => setFilters({ ...filters, distMax: e.target.value ? Number(e.target.value) : undefined })}
              className="w-full px-3 py-2 border rounded"
              placeholder="20"
            />
          </div>

          {/* Going */}
          <div>
            <label className="block text-sm font-medium mb-1">Going</label>
            <input
              type="text"
              value={filters.going || ''}
              onChange={(e) => setFilters({ ...filters, going: e.target.value || undefined })}
              className="w-full px-3 py-2 border rounded"
              placeholder="e.g., Good, Soft"
            />
          </div>
        </div>
      </div>

      {/* Results */}
      {isLoading && <div>Loading races...</div>}
      {data && (
        <div className="space-y-4">
          <div className="text-sm text-gray-600">
            Found {data.length} races
          </div>
          {data.map((race: any) => (
            <a
              key={race.race_id}
              href={`/races/${race.race_id}`}
              className="block p-4 bg-white border rounded-lg hover:shadow-md transition"
            >
              <div className="flex justify-between items-start">
                <div>
                  <h3 className="font-semibold text-lg">{race.race_name}</h3>
                  <p className="text-sm text-gray-600">
                    {race.course_name} • {new Date(race.race_date).toLocaleDateString()} {race.off_time}
                  </p>
                  <div className="mt-2 flex gap-2 text-xs">
                    <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded">
                      {race.race_type}
                    </span>
                    {race.class && (
                      <span className="px-2 py-1 bg-gray-100 text-gray-700 rounded">
                        Class {race.class}
                      </span>
                    )}
                    {race.pattern && (
                      <span className="px-2 py-1 bg-purple-100 text-purple-700 rounded">
                        {race.pattern}
                      </span>
                    )}
                    <span className="px-2 py-1 bg-gray-100 text-gray-700 rounded">
                      {race.dist_raw || `${race.dist_f}f`}
                    </span>
                    {race.going && (
                      <span className="px-2 py-1 bg-green-100 text-green-700 rounded">
                        {race.going}
                      </span>
                    )}
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-2xl font-bold">{race.ran}</div>
                  <div className="text-xs text-gray-600">Runners</div>
                </div>
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
```

---

#### Feature 9: Per-Race Dashboard

**Component**: `src/features/races/components/RaceDashboard.tsx`

```typescript
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { apiClient } from '@/config/api';

export function RaceDashboard() {
  const { raceId } = useParams();

  const { data: race, isLoading } = useQuery({
    queryKey: ['race', raceId],
    queryFn: async () => {
      const { data } = await apiClient.get(`/races/${raceId}`);
      return data;
    },
  });

  if (isLoading) return <div>Loading...</div>;
  if (!race) return <div>Race not found</div>;

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Race Header */}
      <div className="bg-white p-6 rounded-lg border mb-6">
        <h1 className="text-3xl font-bold">{race.race_name}</h1>
        <p className="text-gray-600 mt-2">
          {race.course_name} • {new Date(race.race_date).toLocaleDateString()} {race.off_time}
        </p>
        <div className="mt-4 flex gap-2">
          <span className="px-3 py-1 bg-blue-100 text-blue-700 rounded">
            {race.race_type}
          </span>
          {race.class && (
            <span className="px-3 py-1 bg-gray-100 text-gray-700 rounded">
              Class {race.class}
            </span>
          )}
          <span className="px-3 py-1 bg-gray-100 text-gray-700 rounded">
            {race.dist_raw} • {race.going}
          </span>
          <span className="px-3 py-1 bg-gray-100 text-gray-700 rounded">
            {race.ran} runners
          </span>
        </div>
      </div>

      {/* Runners Table */}
      <div className="bg-white rounded-lg border overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">Pos</th>
              <th className="px-4 py-3 text-left">Draw</th>
              <th className="px-4 py-3 text-left">Horse</th>
              <th className="px-4 py-3 text-left">Jockey</th>
              <th className="px-4 py-3 text-right">Age</th>
              <th className="px-4 py-3 text-right">Wgt</th>
              <th className="px-4 py-3 text-right">OR</th>
              <th className="px-4 py-3 text-right">RPR</th>
              <th className="px-4 py-3 text-right">BSP</th>
              <th className="px-4 py-3 text-right">SP</th>
              <th className="px-4 py-3 text-left">Comment</th>
            </tr>
          </thead>
          <tbody>
            {race.runners.map((runner: any) => (
              <tr 
                key={runner.runner_id} 
                className={`border-t ${runner.pos_num === 1 ? 'bg-yellow-50' : ''}`}
              >
                <td className="px-4 py-3 font-semibold">{runner.pos_raw || '-'}</td>
                <td className="px-4 py-3">{runner.draw || '-'}</td>
                <td className="px-4 py-3">
                  <a href={`/horses/${runner.horse.id}`} className="text-blue-600 hover:underline">
                    {runner.horse.name}
                  </a>
                  <div className="text-xs text-gray-500">{runner.trainer?.name}</div>
                </td>
                <td className="px-4 py-3">{runner.jockey?.name || '-'}</td>
                <td className="px-4 py-3 text-right">{runner.age || '-'}</td>
                <td className="px-4 py-3 text-right">{runner.lbs || '-'}</td>
                <td className="px-4 py-3 text-right">{runner.or || '-'}</td>
                <td className="px-4 py-3 text-right">{runner.rpr || '-'}</td>
                <td className="px-4 py-3 text-right">
                  {runner.win_bsp ? runner.win_bsp.toFixed(2) : '-'}
                </td>
                <td className="px-4 py-3 text-right">
                  {runner.dec ? runner.dec.toFixed(1) : '-'}
                </td>
                <td className="px-4 py-3 text-sm italic max-w-xs truncate">
                  {runner.comment || '-'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

---

### **4. Market Analytics**

#### Feature 11: Steamers & Drifters

**Component**: `src/features/market/components/MarketMovers.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';

export function MarketMovers() {
  const [date, setDate] = useState(new Date().toISOString().split('T')[0]);
  const [type, setType] = useState<'steamer' | 'drifter' | 'all'>('all');
  const [minMove, setMinMove] = useState(20);

  const { data, isLoading } = useQuery({
    queryKey: ['marketMovers', date, type, minMove],
    queryFn: async () => {
      const { data } = await apiClient.get('/market/movers', {
        params: { date, type: type === 'all' ? undefined : type, min_move: minMove },
      });
      return data;
    },
  });

  return (
    <div className="max-w-7xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Market Movers</h1>

      {/* Filters */}
      <div className="bg-white p-4 rounded-lg border mb-6 flex gap-4 items-end">
        <div>
          <label className="block text-sm font-medium mb-1">Date</label>
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="px-3 py-2 border rounded"
          />
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">Type</label>
          <select
            value={type}
            onChange={(e) => setType(e.target.value as any)}
            className="px-3 py-2 border rounded"
          >
            <option value="all">All Movers</option>
            <option value="steamer">Steamers</option>
            <option value="drifter">Drifters</option>
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">Min Move %</label>
          <input
            type="number"
            value={minMove}
            onChange={(e) => setMinMove(Number(e.target.value))}
            className="px-3 py-2 border rounded w-24"
          />
        </div>
      </div>

      {/* Results */}
      {isLoading && <div>Loading...</div>}
      {data && (
        <div className="bg-white rounded-lg border overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left">Time</th>
                <th className="px-4 py-3 text-left">Course</th>
                <th className="px-4 py-3 text-left">Race</th>
                <th className="px-4 py-3 text-left">Horse</th>
                <th className="px-4 py-3 text-right">Morning</th>
                <th className="px-4 py-3 text-right">BSP</th>
                <th className="px-4 py-3 text-right">Move %</th>
                <th className="px-4 py-3 text-left">Result</th>
              </tr>
            </thead>
            <tbody>
              {data.map((mover: any) => {
                const isSteamer = mover.bsp < mover.morning_price;
                return (
                  <tr key={`${mover.race_id}-${mover.horse_name}`} className="border-t">
                    <td className="px-4 py-3">{mover.off_time}</td>
                    <td className="px-4 py-3">{mover.course_name}</td>
                    <td className="px-4 py-3">
                      <a href={`/races/${mover.race_id}`} className="text-blue-600 hover:underline">
                        {mover.race_name}
                      </a>
                    </td>
                    <td className="px-4 py-3 font-semibold">{mover.horse_name}</td>
                    <td className="px-4 py-3 text-right">{mover.morning_price.toFixed(2)}</td>
                    <td className="px-4 py-3 text-right">{mover.bsp.toFixed(2)}</td>
                    <td className={`px-4 py-3 text-right font-bold ${isSteamer ? 'text-green-600' : 'text-red-600'}`}>
                      {isSteamer ? '↓' : '↑'} {Math.abs(mover.move_pct).toFixed(1)}%
                    </td>
                    <td className="px-4 py-3">
                      {mover.win_flag ? (
                        <span className="px-2 py-1 bg-green-100 text-green-700 rounded text-xs font-semibold">
                          WON
                        </span>
                      ) : (
                        <span className="text-gray-500">{mover.pos_num || 'Lost'}</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
```

---

#### Feature 12: Market Calibration

**Component**: `src/features/market/components/Calibration.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

export function MarketCalibration() {
  const [dateFrom, setDateFrom] = useState('2024-01-01');
  const [dateTo, setDateTo] = useState(new Date().toISOString().split('T')[0]);

  const { data, isLoading } = useQuery({
    queryKey: ['calibration', dateFrom, dateTo],
    queryFn: async () => {
      const { data } = await apiClient.get('/market/calibration/win', {
        params: { date_from: dateFrom, date_to: dateTo },
      });
      return data;
    },
  });

  return (
    <div className="max-w-7xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Win Market Calibration</h1>

      {/* Date Range */}
      <div className="bg-white p-4 rounded-lg border mb-6 flex gap-4">
        <div>
          <label className="block text-sm font-medium mb-1">From</label>
          <input
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className="px-3 py-2 border rounded"
          />
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">To</label>
          <input
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className="px-3 py-2 border rounded"
          />
        </div>
      </div>

      {isLoading && <div>Loading...</div>}
      {data && (
        <>
          {/* Chart */}
          <div className="bg-white p-6 rounded-lg border mb-6">
            <h2 className="text-lg font-semibold mb-4">Implied vs Actual Strike Rate</h2>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={data}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="price_bin" />
                <YAxis label={{ value: 'Strike Rate %', angle: -90, position: 'insideLeft' }} />
                <Tooltip />
                <Legend />
                <Bar dataKey="implied_sr" fill="#8884d8" name="Implied SR %" />
                <Bar dataKey="actual_sr" fill="#82ca9d" name="Actual SR %" />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Table */}
          <div className="bg-white rounded-lg border overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left">Price Range</th>
                  <th className="px-4 py-3 text-right">Runners</th>
                  <th className="px-4 py-3 text-right">Winners</th>
                  <th className="px-4 py-3 text-right">Actual SR %</th>
                  <th className="px-4 py-3 text-right">Implied SR %</th>
                  <th className="px-4 py-3 text-right">Edge %</th>
                </tr>
              </thead>
              <tbody>
                {data.map((row: any) => (
                  <tr key={row.price_bin} className="border-t">
                    <td className="px-4 py-3 font-medium">{row.price_bin}</td>
                    <td className="px-4 py-3 text-right">{row.runners.toLocaleString()}</td>
                    <td className="px-4 py-3 text-right">{row.wins.toLocaleString()}</td>
                    <td className="px-4 py-3 text-right">{row.actual_sr.toFixed(2)}%</td>
                    <td className="px-4 py-3 text-right">{row.implied_sr.toFixed(2)}%</td>
                    <td className={`px-4 py-3 text-right font-semibold ${row.edge > 0 ? 'text-green-600' : 'text-red-600'}`}>
                      {row.edge > 0 ? '+' : ''}{row.edge.toFixed(2)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
```

---

### **5. Bias Analysis**

#### Feature 16: Draw Bias

**Component**: `src/features/bias/components/DrawBias.tsx`

```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/config/api';

export function DrawBias() {
  const [courseId, setCourseId] = useState<number>();
  const [distMin, setDistMin] = useState<number>();
  const [distMax, setDistMax] = useState<number>();
  const [going, setGoing] = useState<string>();

  const { data, isLoading } = useQuery({
    queryKey: ['drawBias', courseId, distMin, distMax, going],
    queryFn: async () => {
      if (!courseId) return null;
      const { data } = await apiClient.get('/bias/draw', {
        params: { course_id: courseId, dist_min: distMin, dist_max: distMax, going },
      });
      return data;
    },
    enabled: !!courseId,
  });

  return (
    <div className="max-w-7xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Draw Bias Analyzer</h1>

      {/* Filters */}
      <div className="bg-white p-4 rounded-lg border mb-6">
        <div className="grid md:grid-cols-4 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">Course *</label>
            <select
              value={courseId || ''}
              onChange={(e) => setCourseId(Number(e.target.value) || undefined)}
              className="w-full px-3 py-2 border rounded"
            >
              <option value="">Select course...</option>
              {/* Populate from courses API */}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Min Distance (f)</label>
            <input
              type="number"
              value={distMin || ''}
              onChange={(e) => setDistMin(Number(e.target.value) || undefined)}
              className="w-full px-3 py-2 border rounded"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Max Distance (f)</label>
            <input
              type="number"
              value={distMax || ''}
              onChange={(e) => setDistMax(Number(e.target.value) || undefined)}
              className="w-full px-3 py-2 border rounded"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Going</label>
            <input
              type="text"
              value={going || ''}
              onChange={(e) => setGoing(e.target.value || undefined)}
              placeholder="e.g., Good"
              className="w-full px-3 py-2 border rounded"
            />
          </div>
        </div>
      </div>

      {/* Heatmap/Results */}
      {isLoading && <div>Loading...</div>}
      {data && (
        <div className="bg-white rounded-lg border overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left">Draw</th>
                <th className="px-4 py-3 text-right">Runs</th>
                <th className="px-4 py-3 text-right">Win Rate %</th>
                <th className="px-4 py-3 text-right">Top 3 Rate %</th>
                <th className="px-4 py-3 text-right">Avg Position</th>
              </tr>
            </thead>
            <tbody>
              {data.map((row: any) => {
                const winRate = row.win_rate;
                const heatColor = winRate > 15 ? 'bg-green-100' : winRate > 10 ? 'bg-yellow-100' : 'bg-red-100';
                
                return (
                  <tr key={row.draw} className={`border-t ${heatColor}`}>
                    <td className="px-4 py-3 font-bold">{row.draw}</td>
                    <td className="px-4 py-3 text-right">{row.total_runs}</td>
                    <td className="px-4 py-3 text-right font-semibold">{row.win_rate.toFixed(1)}%</td>
                    <td className="px-4 py-3 text-right">{row.top3_rate.toFixed(1)}%</td>
                    <td className="px-4 py-3 text-right">{row.avg_position.toFixed(1)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
```

---

### **6. Workflow Features**

#### Feature 21: Watchlists

**Component**: `src/features/watchlists/components/WatchlistManager.tsx`

```typescript
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/config/api';

export function WatchlistManager() {
  const queryClient = useQueryClient();
  const [newListName, setNewListName] = useState('');

  const { data: watchlists } = useQuery({
    queryKey: ['watchlists'],
    queryFn: async () => {
      const { data } = await apiClient.get('/watchlists');
      return data;
    },
  });

  const createMutation = useMutation({
    mutationFn: async (name: string) => {
      const { data } = await apiClient.post('/watchlists', { name });
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['watchlists'] });
      setNewListName('');
    },
  });

  return (
    <div className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Watchlists</h1>

      {/* Create New */}
      <div className="bg-white p-4 rounded-lg border mb-6">
        <h2 className="text-lg font-semibold mb-3">Create New Watchlist</h2>
        <div className="flex gap-2">
          <input
            type="text"
            value={newListName}
            onChange={(e) => setNewListName(e.target.value)}
            placeholder="e.g., My Top Horses"
            className="flex-1 px-3 py-2 border rounded"
          />
          <button
            onClick={() => createMutation.mutate(newListName)}
            disabled={!newListName.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
          >
            Create
          </button>
        </div>
      </div>

      {/* Existing Lists */}
      <div className="space-y-4">
        {watchlists?.map((list: any) => (
          <div key={list.watchlist_id} className="bg-white p-4 rounded-lg border">
            <div className="flex justify-between items-start">
              <div>
                <h3 className="text-lg font-semibold">{list.name}</h3>
                <p className="text-sm text-gray-600">
                  {list.item_count} items
                </p>
              </div>
              <a
                href={`/watchlists/${list.watchlist_id}`}
                className="px-3 py-1 text-sm bg-gray-100 rounded hover:bg-gray-200"
              >
                View →
              </a>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

## 🎨 Design Guidelines

### Color Palette
```css
/* Primary */
--primary-blue: #2563eb;
--primary-blue-dark: #1d4ed8;

/* Status Colors */
--success: #10b981;
--warning: #f59e0b;
--error: #ef4444;
--info: #3b82f6;

/* Neutrals */
--gray-50: #f9fafb;
--gray-100: #f3f4f6;
--gray-600: #4b5563;
--gray-900: #111827;

/* Market Colors */
--steamer: #10b981; /* Green for price shortening */
--drifter: #ef4444; /* Red for price drifting */
```

### Typography
```css
/* Headings */
--font-heading: 'Inter', -apple-system, sans-serif;
--font-body: 'Inter', -apple-system, sans-serif;
--font-mono: 'JetBrains Mono', 'Courier New', monospace;

/* Sizes */
--text-xs: 0.75rem;
--text-sm: 0.875rem;
--text-base: 1rem;
--text-lg: 1.125rem;
--text-xl: 1.25rem;
--text-2xl: 1.5rem;
--text-3xl: 1.875rem;
```

### Responsive Breakpoints
```typescript
const breakpoints = {
  sm: '640px',
  md: '768px',
  lg: '1024px',
  xl: '1280px',
  '2xl': '1536px',
};
```

---

## 📱 Mobile Considerations

### Touch-Friendly
- Minimum tap target: 44×44px
- Increased spacing on mobile
- Bottom sheet modals for filters
- Swipe gestures for navigation

### Performance
- Lazy load tables (virtual scrolling)
- Image optimization
- Code splitting by route
- Service worker for offline capability

### Mobile-Specific Components
```typescript
// src/components/mobile/BottomSheet.tsx
export function BottomSheet({ isOpen, onClose, children }) {
  return (
    <div className={`fixed inset-0 z-50 ${isOpen ? 'block' : 'hidden'}`}>
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="fixed bottom-0 left-0 right-0 bg-white rounded-t-2xl p-6 max-h-[80vh] overflow-y-auto">
        {children}
      </div>
    </div>
  );
}
```

---

## ⚡ Performance Optimization

### Data Fetching Strategy

```typescript
// Prefetch on hover
function RaceLink({ raceId, children }) {
  const queryClient = useQueryClient();

  const prefetch = () => {
    queryClient.prefetchQuery({
      queryKey: ['race', raceId],
      queryFn: () => apiClient.get(`/races/${raceId}`),
    });
  };

  return (
    <a 
      href={`/races/${raceId}`} 
      onMouseEnter={prefetch}
    >
      {children}
    </a>
  );
}
```

### Virtual Scrolling for Large Tables

```typescript
import { useVirtualizer } from '@tanstack/react-virtual';

function VirtualTable({ data }) {
  const parentRef = useRef(null);

  const virtualizer = useVirtualizer({
    count: data.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 50,
  });

  return (
    <div ref={parentRef} className="h-[600px] overflow-auto">
      <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative' }}>
        {virtualizer.getVirtualItems().map((virtualRow) => (
          <div
            key={virtualRow.index}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: `${virtualRow.size}px`,
              transform: `translateY(${virtualRow.start}px)`,
            }}
          >
            <TableRow data={data[virtualRow.index]} />
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

## 🧪 Testing

### Unit Tests (Vitest)
```typescript
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { HorseProfile } from './HorseProfile';

describe('HorseProfile', () => {
  it('renders horse name', () => {
    render(<HorseProfile horseId={123} />);
    expect(screen.getByText('Frankel')).toBeInTheDocument();
  });
});
```

### E2E Tests (Playwright)
```typescript
import { test, expect } from '@playwright/test';

test('search for horse', async ({ page }) => {
  await page.goto('/');
  await page.fill('[placeholder*="Search"]', 'Frankel');
  await page.click('text=Frankel');
  await expect(page).toHaveURL(/\/horses\/\d+/);
});
```

---

## 🚀 Deployment Checklist

### Pre-Production
- [ ] Environment variables configured
- [ ] API endpoints tested
- [ ] Error boundaries implemented
- [ ] Loading states on all async operations
- [ ] Mobile responsiveness verified
- [ ] Accessibility audit (WCAG 2.1 AA)
- [ ] Performance audit (Lighthouse > 90)
- [ ] SEO meta tags
- [ ] Analytics integration

### Production
- [ ] CDN for static assets
- [ ] Gzip/Brotli compression
- [ ] Security headers
- [ ] HTTPS enforced
- [ ] Error tracking (Sentry)
- [ ] Monitoring (Datadog/New Relic)

---

## 📚 Resources

- **API Documentation**: See `BACKEND_DEVELOPER_GUIDE.md`
- **Database Schema**: `postgres/API_DOCUMENTATION.md`
- **Design System**: TailwindCSS + shadcn/ui
- **State Management**: TanStack Query docs

**Estimated Timeline**: 10-12 weeks for full implementation

**Questions?** Review the backend API guide and coordinate with the backend team for any endpoint clarifications.

