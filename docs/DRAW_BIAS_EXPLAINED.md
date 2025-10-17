# Draw Bias Explained

**Complete guide to understanding and using draw bias analysis**

Last Updated: October 16, 2025

---

## 📊 What is Draw Bias?

**Draw bias** is the statistical advantage or disadvantage certain starting positions (stalls) have in horse racing, particularly on flat tracks.

**Key Concept:** In a perfectly fair race, each draw should win proportionally to field size. If draw 1 wins more than expected, it has a **positive bias**. If it wins less, it has a **negative bias**.

---

## 🧮 The Formula

### Win Rate Calculation

```
Win Rate (%) = (Total Wins / Total Runs) × 100
```

**Example:**
- Draw 1: 54 wins from 617 runs
- Win Rate = (54 / 617) × 100 = **8.75%**

### Bias Detection

```
Bias = Actual Win Rate - Expected Win Rate

Expected Win Rate = Total Wins / Total Runs (across all draws)
```

**Example:**
- Average win rate across all draws: 7.23%
- Draw 1 win rate: 8.75%
- **Bias: +1.52%** (positive - advantageous!)

---

## 📈 When is a Draw an Advantage?

### Simple Rule

A draw is considered **advantageous** if:
```
Win Rate > Average Win Rate + 1.5%
```

A draw is considered **disadvantageous** if:
```
Win Rate < Average Win Rate - 1.5%
```

### Statistical Significance

The **1.5% threshold** is based on:
- Typical variance in racing data
- Sample sizes (usually 500+ runs per draw)
- Confidence that bias is real, not random noise

**Stricter thresholds:**
- **+2%**: Strong positive bias
- **+3%**: Very strong positive bias
- **-2%**: Strong negative bias

---

## 🎯 Real Example: Ascot (5-7f)

**Data: 6,171 races, 10+ runners**

| Draw | Runs | Wins | Win % | Bias | Interpretation |
|------|------|------|-------|------|----------------|
| 1 | 617 | 54 | 8.75% | +1.52% | ✅ **GOOD** - inside draw advantage |
| 2 | 622 | 48 | 7.72% | +0.49% | ⚪ Neutral |
| 3 | 608 | 40 | 6.58% | -0.65% | ⚪ Neutral |
| 4 | 613 | 39 | 6.36% | -0.87% | ⚪ Neutral |
| 5 | 613 | 30 | 4.89% | -2.34% | 🔴 **BAD** - middle draw disadvantage |
| 6 | 614 | 40 | 6.51% | -0.72% | ⚪ Neutral |
| 7 | 627 | 46 | 7.34% | +0.11% | ⚪ Neutral |

**Average Win Rate: 7.23%**

**Findings:**
- ✅ **Draw 1 (inside):** +1.52% advantage - horses save ground on turns
- 🔴 **Draw 5 (middle):** -2.34% disadvantage - worst position
- 📊 **Pattern:** Low draws (1-2) and high draws (7+) perform better

---

## 🏁 Why Does Draw Bias Exist?

### 1. Track Configuration

**Oval tracks (like Ascot):**
- Inside draws (1-3) save ground on turns
- Can "hug the rail" for shorter distance
- Less traffic to navigate

**Straight tracks:**
- Usually less bias (no turns)
- High draws might have fresher ground

### 2. Going Conditions

**Firm going:**
- Often favors high draws (fresher ground away from rail)

**Soft/Heavy going:**
- Can favor low draws (less kickback)
- Or reverse bias if rail is waterlogged

### 3. Distance

**Sprints (5-7f):**
- Significant bias (less time to overcome poor draw)
- Inside/outside advantage more pronounced

**Middle distance (8-12f):**
- Moderate bias (more time to position)

**Long distance (13f+):**
- Minimal bias (plenty of time to recover)

### 4. Field Size

**Small fields (<10):**
- Less bias (more space)

**Large fields (12+):**
- More bias (traffic, positioning crucial)

---

## 📊 API Response Fields

```json
{
  "draw": 1,
  "total_runs": 617,
  "wins": 54,
  "top3": 125,
  "win_rate": 8.75,
  "top3_rate": 20.26,
  "avg_position": 10.26
}
```

### Field Explanations

| Field | Description | How to Use |
|-------|-------------|------------|
| `draw` | Stall number | The starting position |
| `total_runs` | Sample size | Higher = more reliable |
| `wins` | Actual wins | Raw count for transparency |
| `top3` | Top 3 finishes | Broader success measure |
| `win_rate` | Win % | Main metric for bias |
| `top3_rate` | Top 3 % | Confirms bias pattern |
| `avg_position` | Avg finish position | Lower = better |

**Key Metrics:**
1. **`win_rate`** - Primary indicator of bias
2. **`total_runs`** - Ensure sample size > 500 for reliability
3. **`top3_rate`** - Confirms if bias extends beyond wins
4. **`avg_position`** - Shows overall performance

---

## 🎲 How to Use in Betting

### 1. Identify Strong Bias

```bash
# Get draw bias for Chester (tight turns)
curl "http://localhost:8000/api/v1/bias/draw?course_id=XXX&dist_min=5&dist_max=7"
```

**Look for:**
- Win rate > Average + 2% (**strong bias**)
- Sample size > 500 runs (**reliable**)
- Top3 rate also elevated (**consistent**)

### 2. Filter by Conditions

```bash
# Draw bias on Good going
curl "http://localhost:8000/api/v1/bias/draw?course_id=XXX&going=Good"

# Draw bias for 6f races only
curl "http://localhost:8000/api/v1/bias/draw?course_id=XXX&dist_min=6&dist_max=6"
```

### 3. Apply to Today's Races

**Example Strategy:**
1. Find courses with strong low draw bias (e.g., Chester, Brighton)
2. For 5-7f races at these courses:
   - Upgrade horses in draws 1-3
   - Downgrade horses in draws 8+
3. Adjust prices mentally:
   - Draw 1 at Chester: +20% win probability boost
   - Draw 10 at Chester: -30% win probability

**Warning:** Don't use draw bias in isolation! Consider:
- Horse ability (most important)
- Jockey/trainer form
- Recent performances
- Price value

---

## 📐 Advanced: Impact Ratio

**Impact Ratio** = (Draw Win %) / (Overall Win %)

```
Impact > 1.15 → Positive bias
Impact < 0.85 → Negative bias
Impact ≈ 1.00 → Neutral
```

**Example:**
- Draw 1 win rate: 8.75%
- Overall win rate: 7.23%
- **Impact = 8.75 / 7.23 = 1.21** (21% more wins than expected!)

---

## 🔍 Sample Size Requirements

**Minimum sample sizes for reliable bias:**

| Min Runs per Draw | Reliability | Use Case |
|-------------------|-------------|----------|
| 100 | Low | Exploratory only |
| 300 | Medium | General trends |
| 500 | High | Betting decisions |
| 1000+ | Very High | Strong conclusions |

**Our Ascot data: 617 runs per draw** → ✅ **Highly reliable**

---

## 🏇 Course-Specific Patterns

### Known Strong Biases

**Chester (tight oval):**
- Strong low draw bias (1-3)
- 5f straight: draw 1 wins 12%+ (vs ~7% average)

**Brighton (downhill, left-hand):**
- High draw bias (fresher ground on stands' side)

**Newmarket Rowley Mile (straight):**
- High draw bias (wide draws get better ground)

**York (long straight):**
- Minimal bias (straight finish, wide track)

---

## 📊 Confidence Intervals

For statistical rigor, calculate **95% confidence interval:**

```
Standard Error = √(p × (1-p) / n)

Where:
  p = win rate (as decimal, e.g., 0.0875)
  n = sample size (e.g., 617)

95% CI = win_rate ± (1.96 × SE)
```

**Example (Draw 1):**
- Win rate: 0.0875 (8.75%)
- Sample: 617
- SE = √(0.0875 × 0.9125 / 617) = 0.0114
- 95% CI = 8.75% ± 2.24% → **6.51% to 11.0%**

**If CI doesn't overlap with average (7.23%), bias is statistically significant!**

---

## 🎯 Quick Reference

### When to Trust Draw Bias

✅ **Yes:**
- Sample size > 500 per draw
- Consistent across win%, top3%, avg_position
- Matches track configuration logic
- Bias > ±1.5%

❌ **No:**
- Sample size < 200
- Only one metric shows bias
- Contradicts track layout
- Bias < ±0.5%

### Best Use Cases

1. **Tight, turning tracks** (Chester, Epsom, Goodwood)
2. **Sprints** (5-7f)
3. **Large fields** (12+ runners)
4. **Consistent going** (filter by Good/Firm)

### Weak Bias Scenarios

1. **Straight tracks** (Newmarket, Ascot straight mile)
2. **Long distance** (13f+, plenty of time to overcome)
3. **Small fields** (<10 runners, space to maneuver)

---

## 🔧 Implementation in UI

### Display Example

```tsx
function DrawBiasIndicator({ draw, winRate, avgWinRate }: Props) {
  const bias = winRate - avgWinRate;
  
  if (bias > 1.5) {
    return <span className="badge badge-success">+{bias.toFixed(1)}%</span>;
  } else if (bias < -1.5) {
    return <span className="badge badge-danger">{bias.toFixed(1)}%</span>;
  }
  return <span className="badge badge-secondary">Neutral</span>;
}
```

### Tooltip

```tsx
<Tooltip>
  <p>Draw {draw} win rate: {winRate}%</p>
  <p>Course average: {avgWinRate}%</p>
  <p>Bias: {bias > 0 ? '+' : ''}{bias}%</p>
  <p>Sample: {totalRuns} runs</p>
</Tooltip>
```

---

## 📚 Related Documentation

- [02_API_DOCUMENTATION.md](02_API_DOCUMENTATION.md) - `/api/v1/bias/draw` endpoint
- [03_DATABASE_GUIDE.md](03_DATABASE_GUIDE.md) - `racing.runners.draw` field

---

**Last Updated:** October 16, 2025  
**Status:** ✅ Complete with win counts  
**Formula:** (Wins / Runs) × 100  
**Advantage:** Win % > Average + 1.5%


