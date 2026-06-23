#!/usr/bin/env python3
"""
Embedding-similarity as an edit-time semantic-conflict detector for interlock.

E1 — LATENCY: warm the model once, then measure per-encode wall-time over ~30
realistic code spans. Report p50/p90/p99 for a single encode and for a "pair"
(2 encodes + cosine). Verdict against <50ms / <100ms.

E2 — DISCRIMINATION: a hand-labeled corpus of edit pairs (CONFLICT / NO-CONFLICT),
cosine similarity per pair, sorted, and threshold-separability analysis.

Run: cd intersearch && HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 uv run python <thisfile>
Outputs JSON + CSV to the budget dir.
"""

from __future__ import annotations

import json
import statistics
import sys
import time
from pathlib import Path

sys.path.insert(0, "src")
from intersearch.embeddings import EmbeddingClient  # noqa: E402

OUT = Path("/Users/sma/.claude/jobs/16643546/tmp/embed-exp")
OUT.mkdir(parents=True, exist_ok=True)


def pct(data, p):
    """Nearest-rank-ish percentile via statistics.quantiles for robustness."""
    if len(data) == 1:
        return data[0]
    # use linear interpolation (inclusive) like numpy default
    s = sorted(data)
    k = (len(s) - 1) * (p / 100.0)
    f = int(k)
    c = min(f + 1, len(s) - 1)
    if f == c:
        return s[f]
    return s[f] + (s[c] - s[f]) * (k - f)


# ----------------------------------------------------------------------------
# E1 corpus: ~30 realistic code spans, 10-40 lines, Rust / Python / TS mix.
# ----------------------------------------------------------------------------
E1_SPANS = [
    # --- Rust ---
    """pub fn apply_mood(elf: &mut Elf, delta: i32) {
    elf.mood = (elf.mood + delta).clamp(-100, 100);
    if elf.mood < -50 {
        elf.tantrum_timer = Some(TICKS_PER_DAY);
    }
}""",
    """impl World {
    pub fn tick(&mut self) {
        self.advance_clock();
        self.run_needs_system();
        self.run_social_system();
        self.run_art_system();
        self.gc_dead_entities();
    }
}""",
    """#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Selection {
    None,
    Single(EntityId),
    Multi(Vec<EntityId>),
    Region { x0: u16, y0: u16, x1: u16, y1: u16 },
}""",
    """fn pathfind(grid: &Grid, start: Coord, goal: Coord) -> Option<Vec<Coord>> {
    let mut open = BinaryHeap::new();
    open.push(Node { cost: 0, pos: start });
    let mut came_from = HashMap::new();
    while let Some(node) = open.pop() {
        if node.pos == goal {
            return Some(reconstruct(came_from, goal));
        }
        for n in grid.neighbors(node.pos) {
            came_from.entry(n).or_insert(node.pos);
            open.push(Node { cost: node.cost + 1, pos: n });
        }
    }
    None
}""",
    """pub struct Relationship {
    pub other: EntityId,
    pub affinity: i16,
    pub trust: i16,
    pub last_interaction: u64,
    pub shared_memories: Vec<MemoryId>,
}""",
    """fn render_tile(buf: &mut Buffer, tile: Tile, x: u16, y: u16) {
    let glyph = match tile.kind {
        TileKind::Grass => '.',
        TileKind::Tree => 'T',
        TileKind::Water => '~',
        TileKind::Stone => '#',
    };
    buf.set(x, y, glyph, tile.color());
}""",
    """impl Storyteller {
    fn maybe_spawn_event(&mut self, world: &World) -> Option<Event> {
        let pressure = self.drama_curve.sample(world.tick);
        if pressure > self.threshold && self.cooldown == 0 {
            self.cooldown = EVENT_COOLDOWN;
            return Some(self.pick_event(world));
        }
        None
    }
}""",
    """pub fn decay_needs(elf: &mut Elf, dt: f32) {
    elf.hunger += HUNGER_RATE * dt;
    elf.fatigue += FATIGUE_RATE * dt;
    elf.social_need += SOCIAL_RATE * dt;
    elf.hunger = elf.hunger.min(MAX_NEED);
}""",
    """#[test]
fn test_mood_clamps() {
    let mut elf = Elf::default();
    apply_mood(&mut elf, 500);
    assert_eq!(elf.mood, 100);
    apply_mood(&mut elf, -500);
    assert_eq!(elf.mood, -100);
}""",
    """fn reconstruct(came: HashMap<Coord, Coord>, goal: Coord) -> Vec<Coord> {
    let mut path = vec![goal];
    let mut cur = goal;
    while let Some(&prev) = came.get(&cur) {
        path.push(prev);
        cur = prev;
    }
    path.reverse();
    path
}""",
    # --- Python ---
    """def embed_batch(self, texts: list[str]) -> np.ndarray:
    self._ensure_model()
    embeddings = self._model.encode(
        texts, normalize_embeddings=True, show_progress_bar=False
    )
    return np.array(embeddings, dtype=np.float32)""",
    """class EmbeddingStore:
    def __init__(self, project_hash: str):
        self.path = INDEX_ROOT / project_hash / "embeddings.db"
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.conn = sqlite3.connect(self.path)
        self.conn.execute("PRAGMA journal_mode=WAL")
        self._ensure_schema()""",
    """def cosine_similarity(a: np.ndarray, b: np.ndarray) -> float:
    return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))""",
    """async def search(self, query: str, top_k: int = 10) -> list[Hit]:
    qvec = self.client.embed(query)
    rows = self.store.all_vectors()
    scored = [(cosine(qvec, v), doc) for doc, v in rows]
    scored.sort(key=lambda x: x[0], reverse=True)
    return [Hit(doc, score) for score, doc in scored[:top_k]]""",
    """def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="intersearch CLI")
    p.add_argument("--index", action="store_true")
    p.add_argument("--query", type=str)
    p.add_argument("--top-k", type=int, default=10)
    return p.parse_args()""",
    """@dataclass
class Hit:
    doc_id: str
    score: float
    snippet: str = ""
    line_start: int = 0
    line_end: int = 0""",
    """def chunk_file(path: Path, max_lines: int = 40) -> list[Chunk]:
    lines = path.read_text().splitlines()
    chunks = []
    for i in range(0, len(lines), max_lines):
        body = "\\n".join(lines[i : i + max_lines])
        chunks.append(Chunk(path, i, i + max_lines, body))
    return chunks""",
    """def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for block in iter(lambda: f.read(8192), b""):
            h.update(block)
    return h.hexdigest()""",
    """def test_embed_dim():
    client = EmbeddingClient()
    vec = client.embed("hello world")
    assert vec.shape == (768,)
    assert abs(np.linalg.norm(vec) - 1.0) < 1e-3""",
    """def retry(fn, attempts=3, delay=0.5):
    for i in range(attempts):
        try:
            return fn()
        except Exception:
            if i == attempts - 1:
                raise
            time.sleep(delay * (2 ** i))""",
    # --- TypeScript ---
    """export function reserveFiles(agent: string, paths: string[]): Reservation {
  const now = Date.now();
  const res: Reservation = { agent, paths, createdAt: now, ttl: DEFAULT_TTL };
  for (const p of paths) {
    locks.set(p, res);
  }
  return res;
}""",
    """export interface AgentMessage {
  from: string;
  to: string | "broadcast";
  body: string;
  topic?: string;
  timestamp: number;
}""",
    """async function checkConflicts(file: string, region: Region): Promise<Conflict[]> {
  const others = await listReservations(file);
  return others
    .filter((r) => overlaps(r.region, region))
    .map((r) => ({ agent: r.agent, region: r.region }));
}""",
    """export const useAgents = () => {
  const [agents, setAgents] = useState<Agent[]>([]);
  useEffect(() => {
    const sub = subscribe("agents", setAgents);
    return () => sub.unsubscribe();
  }, []);
  return agents;
};""",
    """function overlaps(a: Region, b: Region): boolean {
  return a.start < b.end && b.start < a.end;
}""",
    """export class InboxStore {
  private messages: AgentMessage[] = [];
  push(msg: AgentMessage): void {
    this.messages.push(msg);
    this.messages.sort((x, y) => x.timestamp - y.timestamp);
  }
  drain(agent: string): AgentMessage[] {
    const mine = this.messages.filter((m) => m.to === agent);
    this.messages = this.messages.filter((m) => m.to !== agent);
    return mine;
  }
}""",
    """router.post("/reserve", async (req, res) => {
  const { agent, paths } = req.body;
  if (!agent || !Array.isArray(paths)) {
    return res.status(400).json({ error: "bad request" });
  }
  const reservation = reserveFiles(agent, paths);
  res.json(reservation);
});""",
    """export function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}""",
    """type ReleasePolicy = "auto" | "negotiate" | "deny";
const POLICY_DEFAULTS: Record<string, ReleasePolicy> = {
  test: "auto",
  src: "negotiate",
  config: "deny",
};""",
    """export async function negotiateRelease(
  file: string,
  requester: string
): Promise<boolean> {
  const holder = locks.get(file);
  if (!holder) return true;
  const ok = await ask(holder.agent, `release ${file}?`);
  if (ok) locks.delete(file);
  return ok;
}""",
]


# ----------------------------------------------------------------------------
# E2 corpus: hand-labeled edit pairs. Each pair = two edit regions to the SAME
# file. label = CONFLICT (overlapping/contradictory changes to same region or
# logic) or NO-CONFLICT (different functions/regions, independent).
#
# An "edit region" here is the code span an agent touched (the resulting text of
# the edited block). This is what an edit-time detector would have available.
# ----------------------------------------------------------------------------
E2_PAIRS = [
    # 1. CONFLICT: two edits to the SAME function body (mood calc), contradictory clamp.
    {
        "id": "same-fn-mood-A",
        "label": "CONFLICT",
        "desc": "Two agents edit apply_mood body; different clamp bounds",
        "a": """pub fn apply_mood(elf: &mut Elf, delta: i32) {
    elf.mood = (elf.mood + delta).clamp(-100, 100);
    if elf.mood < -50 { elf.tantrum_timer = Some(TICKS_PER_DAY); }
}""",
        "b": """pub fn apply_mood(elf: &mut Elf, delta: i32) {
    elf.mood = (elf.mood + delta * 2).clamp(-128, 127);
    if elf.mood < -60 { elf.tantrum_timer = Some(TICKS_PER_DAY / 2); }
}""",
    },
    # 2. CONFLICT: same function, one renames the symbol the other still uses.
    {
        "id": "rename-shared-symbol",
        "label": "CONFLICT",
        "desc": "decay_needs: rename field hunger->satiety vs edit using hunger",
        "a": """pub fn decay_needs(elf: &mut Elf, dt: f32) {
    elf.hunger += HUNGER_RATE * dt;
    elf.hunger = elf.hunger.min(MAX_NEED);
}""",
        "b": """pub fn decay_needs(elf: &mut Elf, dt: f32) {
    elf.satiety -= HUNGER_RATE * dt;
    elf.satiety = elf.satiety.max(0.0);
}""",
    },
    # 3. CONFLICT: same tick pipeline, both reorder/insert system calls.
    {
        "id": "tick-pipeline-reorder",
        "label": "CONFLICT",
        "desc": "World::tick: both edit the same system-call ordering",
        "a": """pub fn tick(&mut self) {
    self.advance_clock();
    self.run_needs_system();
    self.run_social_system();
    self.gc_dead_entities();
}""",
        "b": """pub fn tick(&mut self) {
    self.advance_clock();
    self.run_social_system();
    self.run_needs_system();
    self.run_art_system();
    self.gc_dead_entities();
}""",
    },
    # 4. CONFLICT: same TS function checkConflicts, contradictory filter logic.
    {
        "id": "ts-checkconflicts",
        "label": "CONFLICT",
        "desc": "checkConflicts: two edits to overlap filter",
        "a": """async function checkConflicts(file: string, region: Region): Promise<Conflict[]> {
  const others = await listReservations(file);
  return others.filter((r) => overlaps(r.region, region))
    .map((r) => ({ agent: r.agent, region: r.region }));
}""",
        "b": """async function checkConflicts(file: string, region: Region): Promise<Conflict[]> {
  const others = await listReservations(file);
  return others.filter((r) => overlaps(r.region, region) && r.agent !== self)
    .map((r) => ({ agent: r.agent, region: r.region, severity: "hard" }));
}""",
    },
    # 5. CONFLICT: same pathfind loop, both change cost model.
    {
        "id": "pathfind-cost",
        "label": "CONFLICT",
        "desc": "pathfind inner loop: both edit cost computation",
        "a": """while let Some(node) = open.pop() {
    if node.pos == goal { return Some(reconstruct(came_from, goal)); }
    for n in grid.neighbors(node.pos) {
        open.push(Node { cost: node.cost + 1, pos: n });
    }
}""",
        "b": """while let Some(node) = open.pop() {
    if node.pos == goal { return Some(reconstruct(came_from, goal)); }
    for n in grid.neighbors(node.pos) {
        let h = heuristic(n, goal);
        open.push(Node { cost: node.cost + grid.weight(n) + h, pos: n });
    }
}""",
    },
    # 6. CONFLICT: same Python embed_batch, contradictory normalize flag.
    {
        "id": "py-embed-normalize",
        "label": "CONFLICT",
        "desc": "embed_batch: one disables normalize, other keeps it",
        "a": """def embed_batch(self, texts: list[str]) -> np.ndarray:
    self._ensure_model()
    embeddings = self._model.encode(texts, normalize_embeddings=True, show_progress_bar=False)
    return np.array(embeddings, dtype=np.float32)""",
        "b": """def embed_batch(self, texts: list[str]) -> np.ndarray:
    self._ensure_model()
    embeddings = self._model.encode(texts, normalize_embeddings=False, batch_size=64)
    return np.array(embeddings, dtype=np.float64)""",
    },
    # 7. CONFLICT: same struct definition, both add overlapping fields.
    {
        "id": "struct-relationship-fields",
        "label": "CONFLICT",
        "desc": "Relationship struct: both add fields, layout clash",
        "a": """pub struct Relationship {
    pub other: EntityId,
    pub affinity: i16,
    pub trust: i16,
    pub last_interaction: u64,
}""",
        "b": """pub struct Relationship {
    pub other: EntityId,
    pub affinity: i32,
    pub rivalry: i16,
    pub last_interaction: u64,
}""",
    },
    # 8. CONFLICT: same enum, both add/rename variants on the same line region.
    {
        "id": "enum-selection-variants",
        "label": "CONFLICT",
        "desc": "Selection enum: both edit variant set",
        "a": """pub enum Selection {
    None,
    Single(EntityId),
    Multi(Vec<EntityId>),
}""",
        "b": """pub enum Selection {
    Empty,
    One(EntityId),
    Many(Vec<EntityId>),
    Box { x0: u16, y0: u16, x1: u16, y1: u16 },
}""",
    },
    # --- NO-CONFLICT cases ---
    # 9. NO-CONFLICT: helper at top vs test at bottom (different regions/files-logic).
    {
        "id": "helper-vs-test",
        "label": "NO-CONFLICT",
        "desc": "edit reconstruct() helper vs edit a test fn",
        "a": """fn reconstruct(came: HashMap<Coord, Coord>, goal: Coord) -> Vec<Coord> {
    let mut path = vec![goal];
    let mut cur = goal;
    while let Some(&prev) = came.get(&cur) { path.push(prev); cur = prev; }
    path.reverse();
    path
}""",
        "b": """#[test]
fn test_mood_clamps() {
    let mut elf = Elf::default();
    apply_mood(&mut elf, 500);
    assert_eq!(elf.mood, 100);
}""",
    },
    # 10. NO-CONFLICT: additive imports vs unrelated render function.
    {
        "id": "imports-vs-render",
        "label": "NO-CONFLICT",
        "desc": "additive imports block vs render_tile body",
        "a": """use std::collections::HashMap;
use std::collections::BinaryHeap;
use crate::grid::{Grid, Coord};
use crate::storyteller::Storyteller;""",
        "b": """fn render_tile(buf: &mut Buffer, tile: Tile, x: u16, y: u16) {
    let glyph = match tile.kind {
        TileKind::Grass => '.',
        TileKind::Water => '~',
    };
    buf.set(x, y, glyph, tile.color());
}""",
    },
    # 11. NO-CONFLICT: two entirely different functions in same TS file.
    {
        "id": "ts-formatduration-vs-overlaps",
        "label": "NO-CONFLICT",
        "desc": "formatDuration vs overlaps — unrelated utilities",
        "a": """export function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}""",
        "b": """function overlaps(a: Region, b: Region): boolean {
  return a.start < b.end && b.start < a.end;
}""",
    },
    # 12. NO-CONFLICT: storyteller event logic vs needs decay — different subsystems.
    {
        "id": "storyteller-vs-needs",
        "label": "NO-CONFLICT",
        "desc": "Storyteller::maybe_spawn_event vs decay_needs",
        "a": """fn maybe_spawn_event(&mut self, world: &World) -> Option<Event> {
    let pressure = self.drama_curve.sample(world.tick);
    if pressure > self.threshold && self.cooldown == 0 {
        self.cooldown = EVENT_COOLDOWN;
        return Some(self.pick_event(world));
    }
    None
}""",
        "b": """pub fn decay_needs(elf: &mut Elf, dt: f32) {
    elf.hunger += HUNGER_RATE * dt;
    elf.fatigue += FATIGUE_RATE * dt;
    elf.social_need += SOCIAL_RATE * dt;
}""",
    },
    # 13. NO-CONFLICT: python CLI arg parser vs sha256 helper.
    {
        "id": "py-args-vs-sha256",
        "label": "NO-CONFLICT",
        "desc": "parse_args vs sha256_file — unrelated",
        "a": """def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="intersearch CLI")
    p.add_argument("--index", action="store_true")
    p.add_argument("--query", type=str)
    return p.parse_args()""",
        "b": """def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for block in iter(lambda: f.read(8192), b""):
            h.update(block)
    return h.hexdigest()""",
    },
    # 14. NO-CONFLICT: TS interface (additive) vs unrelated React hook.
    {
        "id": "ts-interface-vs-hook",
        "label": "NO-CONFLICT",
        "desc": "AgentMessage interface vs useAgents hook",
        "a": """export interface AgentMessage {
  from: string;
  to: string | "broadcast";
  body: string;
  timestamp: number;
}""",
        "b": """export const useAgents = () => {
  const [agents, setAgents] = useState<Agent[]>([]);
  useEffect(() => {
    const sub = subscribe("agents", setAgents);
    return () => sub.unsubscribe();
  }, []);
  return agents;
};""",
    },
    # 15. NO-CONFLICT: two adjacent but independent functions in store.py.
    {
        "id": "py-chunk-vs-retry",
        "label": "NO-CONFLICT",
        "desc": "chunk_file vs retry — independent utilities",
        "a": """def chunk_file(path: Path, max_lines: int = 40) -> list[Chunk]:
    lines = path.read_text().splitlines()
    chunks = []
    for i in range(0, len(lines), max_lines):
        body = "\\n".join(lines[i : i + max_lines])
        chunks.append(Chunk(path, i, i + max_lines, body))
    return chunks""",
        "b": """def retry(fn, attempts=3, delay=0.5):
    for i in range(attempts):
        try:
            return fn()
        except Exception:
            if i == attempts - 1:
                raise
            time.sleep(delay * (2 ** i))""",
    },
    # 16. NO-CONFLICT: TS router endpoint vs InboxStore class — different concerns.
    {
        "id": "ts-router-vs-inbox",
        "label": "NO-CONFLICT",
        "desc": "POST /reserve handler vs InboxStore.drain",
        "a": """router.post("/reserve", async (req, res) => {
  const { agent, paths } = req.body;
  if (!agent || !Array.isArray(paths)) {
    return res.status(400).json({ error: "bad request" });
  }
  res.json(reserveFiles(agent, paths));
});""",
        "b": """drain(agent: string): AgentMessage[] {
    const mine = this.messages.filter((m) => m.to === agent);
    this.messages = this.messages.filter((m) => m.to !== agent);
    return mine;
}""",
    },
]


def run_e1(client: EmbeddingClient) -> dict:
    # Warmup: one throwaway encode (model already loaded by smoke earlier, but
    # _ensure_model is idempotent; do an explicit throwaway anyway).
    _ = client.embed("warmup throwaway encode to stabilize timing")

    single_ms = []
    for span in E1_SPANS:
        t0 = time.perf_counter()
        _ = client.embed(span)
        single_ms.append((time.perf_counter() - t0) * 1000.0)

    # Pair timing: 2 encodes + cosine, over consecutive span pairs (30 pairs by wraparound).
    pair_ms = []
    n = len(E1_SPANS)
    for i in range(n):
        a_txt = E1_SPANS[i]
        b_txt = E1_SPANS[(i + 1) % n]
        t0 = time.perf_counter()
        va = client.embed(a_txt)
        vb = client.embed(b_txt)
        _ = client.cosine_similarity(va, vb)
        pair_ms.append((time.perf_counter() - t0) * 1000.0)

    def summ(d):
        return {
            "n": len(d),
            "min": round(min(d), 3),
            "p50": round(pct(d, 50), 3),
            "p90": round(pct(d, 90), 3),
            "p99": round(pct(d, 99), 3),
            "max": round(max(d), 3),
            "mean": round(statistics.fmean(d), 3),
        }

    return {
        "single_encode_ms": summ(single_ms),
        "pair_2enc_plus_cosine_ms": summ(pair_ms),
        "raw_single_ms": [round(x, 3) for x in single_ms],
        "raw_pair_ms": [round(x, 3) for x in pair_ms],
        "verdict_pair_under_50ms": pct(pair_ms, 90) < 50.0,
        "verdict_pair_under_100ms": pct(pair_ms, 90) < 100.0,
    }


def run_e2(client: EmbeddingClient) -> dict:
    results = []
    for pair in E2_PAIRS:
        va = client.embed(pair["a"])
        vb = client.embed(pair["b"])
        cos = client.cosine_similarity(va, vb)
        results.append(
            {
                "id": pair["id"],
                "label": pair["label"],
                "desc": pair["desc"],
                "cosine": round(cos, 4),
            }
        )

    results.sort(key=lambda r: r["cosine"])

    conflict = [r["cosine"] for r in results if r["label"] == "CONFLICT"]
    noconf = [r["cosine"] for r in results if r["label"] == "NO-CONFLICT"]

    # Separability: is there a threshold T such that CONFLICT cos >= T and
    # NO-CONFLICT cos < T (i.e. conflicts are MORE similar — same region/logic)?
    clean = min(conflict) > max(noconf)
    overlap_lo = max(noconf)
    overlap_hi = min(conflict)
    suggested_threshold = round((overlap_lo + overlap_hi) / 2.0, 4) if clean else None

    # Best achievable accuracy if we sweep a threshold (conflicts >= T).
    all_cos = sorted(set([r["cosine"] for r in results]))
    candidate_ts = [(all_cos[i] + all_cos[i + 1]) / 2 for i in range(len(all_cos) - 1)]
    candidate_ts += [min(all_cos) - 0.01, max(all_cos) + 0.01]
    best_acc, best_t = 0.0, None
    for t in candidate_ts:
        correct = 0
        for r in results:
            pred = "CONFLICT" if r["cosine"] >= t else "NO-CONFLICT"
            if pred == r["label"]:
                correct += 1
        acc = correct / len(results)
        if acc > best_acc:
            best_acc, best_t = acc, round(t, 4)

    return {
        "pairs_sorted_by_cosine": results,
        "conflict_cos_range": [round(min(conflict), 4), round(max(conflict), 4)],
        "noconflict_cos_range": [round(min(noconf), 4), round(max(noconf), 4)],
        "clean_separation": clean,
        "suggested_threshold": suggested_threshold,
        "overlap_band": [round(overlap_lo, 4), round(overlap_hi, 4)],
        "best_sweep_threshold": best_t,
        "best_sweep_accuracy": round(best_acc, 4),
        "n_conflict": len(conflict),
        "n_noconflict": len(noconf),
    }


def main():
    client = EmbeddingClient()
    print(f"model: {client.model_name} dim={client.dim}")

    e1 = run_e1(client)
    print("E1 single:", e1["single_encode_ms"])
    print("E1 pair  :", e1["pair_2enc_plus_cosine_ms"])

    e2 = run_e2(client)
    print(
        "E2 clean_separation:",
        e2["clean_separation"],
        "best_sweep_acc:",
        e2["best_sweep_accuracy"],
        "@T",
        e2["best_sweep_threshold"],
    )

    out = {"model": client.model_name, "dim": client.dim, "E1": e1, "E2": e2}
    (OUT / "results.json").write_text(json.dumps(out, indent=2))

    # CSV for E2
    lines = ["cosine,label,id,desc"]
    for r in e2["pairs_sorted_by_cosine"]:
        lines.append(f'{r["cosine"]},{r["label"]},{r["id"]},"{r["desc"]}"')
    (OUT / "e2_pairs.csv").write_text("\n".join(lines) + "\n")

    print(f"\nWrote {OUT/'results.json'} and {OUT/'e2_pairs.csv'}")


if __name__ == "__main__":
    main()
