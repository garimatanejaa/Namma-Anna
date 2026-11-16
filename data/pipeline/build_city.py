#!/usr/bin/env python3
"""Build the Bengaluru city model embedded in the Go backend.

Inputs (data/raw):
  bbmp_wards_2022.geojson   243 BBMP wards (2022 delimitation), KSRSAC via DataMeet. CC licence.
  education_points.csv      ~2.8k geocoded schools and pre-schools, Open Bangalore (ODbL).
Plus a small hand-curated list of landmark anchors (tech parks, transit hubs, retail districts)
with approximate public coordinates; they are a *prior* for where jobs and transit trips originate.

Outputs (backend/internal/city/):
  wards.geojson   simplified ward polygons for the map
  city.json       per-ward features + adjacency + anchors + provenance
"""
from __future__ import annotations

import csv
import json
import math
from pathlib import Path

import numpy as np
from shapely.geometry import Point, shape, mapping
from shapely.strtree import STRtree

ROOT = Path(__file__).resolve().parents[2]
RAW = ROOT / "data" / "raw"
OUT = ROOT / "backend" / "internal" / "city"

LAT0, LNG0 = 12.9716, 77.5946          # Vidhana Soudha, used as the projection origin and "core"
KX = 111.32 * math.cos(math.radians(LAT0))
KY = 110.57


def xy(lng, lat):
    return ((lng - LNG0) * KX, (lat - LAT0) * KY)


# kind: employ | transit | retail ; weight is a relative strength (1.0 = a major hub)
ANCHORS = [
    # tech / office clusters
    ("Whitefield (ITPL)", 12.9863, 77.7371, "employ", 1.0),
    ("Marathahalli - Outer Ring Road", 12.9569, 77.7011, "employ", 0.9),
    ("Bellandur - Sarjapur ORR", 12.9330, 77.6830, "employ", 1.0),
    ("Electronic City", 12.8452, 77.6602, "employ", 1.0),
    ("Manyata Tech Park", 13.0451, 77.6200, "employ", 0.9),
    ("Koramangala", 12.9345, 77.6112, "employ", 0.8),
    ("MG Road - CBD", 12.9756, 77.6068, "employ", 1.0),
    ("Indiranagar", 12.9784, 77.6408, "employ", 0.6),
    ("HSR Layout", 12.9116, 77.6474, "employ", 0.55),
    ("Bannerghatta Road", 12.8950, 77.6000, "employ", 0.5),
    ("Peenya Industrial Area", 13.0285, 77.5197, "employ", 0.45),
    ("Hebbal - Bellary Road", 13.0358, 77.5970, "employ", 0.4),
    # transit hubs
    ("Majestic (KSR / Kempegowda bus stn)", 12.9777, 77.5724, "transit", 1.0),
    ("Cantonment Railway", 12.9937, 77.5995, "transit", 0.6),
    ("Yeshwanthpur", 13.0285, 77.5400, "transit", 0.7),
    ("KR Puram", 13.0010, 77.6960, "transit", 0.8),
    ("Silk Board Junction", 12.9170, 77.6230, "transit", 0.7),
    ("Kengeri", 12.9177, 77.4830, "transit", 0.4),
    ("Jayanagar", 12.9250, 77.5830, "transit", 0.5),
    ("Banashankari", 12.9255, 77.5468, "transit", 0.45),
    ("Rajajinagar", 12.9916, 77.5520, "transit", 0.4),
    ("Yelahanka", 13.1007, 77.5963, "transit", 0.4),
    ("Hennur - Kalyan Nagar", 13.0390, 77.6430, "transit", 0.35),
    # retail / leisure
    ("Brigade Road - Commercial St", 12.9718, 77.6080, "retail", 1.0),
    ("Forum Koramangala", 12.9345, 77.6112, "retail", 0.7),
    ("Phoenix Marketcity", 12.9965, 77.6960, "retail", 0.7),
    ("Malleshwaram", 13.0035, 77.5700, "retail", 0.6),
    ("Jayanagar 4th Block", 12.9250, 77.5830, "retail", 0.6),
    ("Indiranagar 100ft Rd", 12.9784, 77.6408, "retail", 0.7),
]


def hav(lat1, lng1, lat2, lng2):
    (x1, y1), (x2, y2) = xy(lng1, lat1), xy(lng2, lat2)
    return math.hypot(x1 - x2, y1 - y2)


def norm(a):
    a = np.asarray(a, dtype=float)
    return a / max(a.sum(), 1e-12)


def main():
    gj = json.loads((RAW / "bbmp_wards_2022.geojson").read_text())
    feats = gj["features"]
    polys = [shape(f["geometry"]) for f in feats]
    n = len(polys)
    print("wards:", n)

    # ---- geometry-derived features
    cent, area = [], []
    for p in polys:
        c = p.representative_point() if not p.is_valid else p.centroid
        cent.append((c.x, c.y))
        # area in km2 using the local equirectangular projection
        pts = [xy(*pt) for geom in (p.geoms if p.geom_type == "MultiPolygon" else [p]) for pt in geom.exterior.coords]
        a = 0.0
        for geom in (p.geoms if p.geom_type == "MultiPolygon" else [p]):
            ring = [xy(*pt) for pt in geom.exterior.coords]
            a += abs(sum(ring[i][0] * ring[i + 1][1] - ring[i + 1][0] * ring[i][1] for i in range(len(ring) - 1))) / 2
        area.append(a)
    area = np.array(area)

    # ---- adjacency (shared boundary)
    tree = STRtree(polys)
    nbrs = [set() for _ in range(n)]
    for i, p in enumerate(polys):
        for j in tree.query(p.buffer(2e-5)):
            j = int(j)
            if j != i and p.buffer(2e-5).intersection(polys[j]).length > 3e-4:
                nbrs[i].add(j)
                nbrs[j].add(i)
    # guarantee a connected graph: attach islands to their nearest wards
    d = lambda a, b: hav(cent[a][1], cent[a][0], cent[b][1], cent[b][0])
    comp = list(range(n))

    def find(x):
        while comp[x] != x:
            comp[x] = comp[comp[x]]
            x = comp[x]
        return x
    for i in range(n):
        for j in nbrs[i]:
            comp[find(i)] = find(j)
    while len({find(i) for i in range(n)}) > 1:
        roots = {}
        for i in range(n):
            roots.setdefault(find(i), []).append(i)
        smallest = min(roots.values(), key=len)
        others = [i for i in range(n) if find(i) != find(smallest[0])]
        a, b = min(((a, b) for a in smallest for b in others), key=lambda ab: d(*ab))
        nbrs[a].add(b); nbrs[b].add(a); comp[find(a)] = find(b)
    print("avg degree:", np.mean([len(s) for s in nbrs]).round(2))

    # ---- education points per ward (real data; proxy for residential families / catchments)
    edu = np.zeros(n)
    pts_in = 0
    with open(RAW / "education_points.csv") as f:
        for r in csv.DictReader(f):
            pt = Point(float(r["lng"]), float(r["lat"]))
            for j in tree.query(pt):
                if polys[int(j)].contains(pt):
                    edu[int(j)] += 1
                    pts_in += 1
                    break
    print("education points inside BBMP:", pts_in)

    # ---- anchors -> kernel density per component
    sigma = {"employ": 1.8, "transit": 1.3, "retail": 1.2}
    comps = {k: np.zeros(n) for k in sigma}
    for name, lat, lng, kind, w in ANCHORS:
        for i in range(n):
            dd = hav(lat, lng, cent[i][1], cent[i][0])
            comps[kind][i] += w * math.exp(-0.5 * (dd / sigma[kind]) ** 2)

    core_d = np.array([hav(LAT0, LNG0, c[1], c[0]) for c in cent])
    # residential: smoothed school density (real) blended with a core-decay population prior
    edu_s = np.array([edu[i] + 0.5 * np.mean([edu[j] for j in nbrs[i]]) for i in range(n)])
    prior = np.sqrt(area) / (1 + (core_d / 6.0) ** 2.4)
    prior = np.minimum(prior, np.percentile(prior, 97))       # damp giant outer wards
    resid = 0.55 * norm(edu_s) + 0.45 * norm(prior)
    resid = norm(resid + 0.15 / n)
    employ = norm(comps["employ"] + 0.02)
    transit = norm(comps["transit"] + 0.01)
    retail = norm(comps["retail"] + 0.01)

    # ---- label: highest percentile-rank component if it is clearly dominant, else Mixed
    def prank(v):
        return np.argsort(np.argsort(v)) / (len(v) - 1)
    ranks = {"Residential": prank(resid), "Tech & offices": prank(employ),
             "Transit hub": prank(transit), "Retail & leisure": prank(retail)}
    labels = []
    for i in range(n):
        top = max(ranks, key=lambda k: ranks[k][i])
        labels.append(top if ranks[top][i] >= 0.87 else "Mixed")

    wards = []
    for i, f in enumerate(feats):
        p = f["properties"]
        wards.append({
            "id": i, "no": p["KGISWardNo"], "name": p["KGISWardName"].replace(" Ward", "").strip(),
            "lng": round(cent[i][0], 5), "lat": round(cent[i][1], 5), "area_km2": round(float(area[i]), 2),
            "core_km": round(float(core_d[i]), 2), "nbrs": sorted(nbrs[i]), "schools": int(edu[i]),
            "resid": round(float(resid[i]), 6), "employ": round(float(employ[i]), 6),
            "transit": round(float(transit[i]), 6), "retail": round(float(retail[i]), 6), "type": labels[i],
        })

    meta = {
        "city": "Bengaluru", "wards": n, "origin": [LNG0, LAT0],
        "sources": [
            {"name": "BBMP ward boundaries (2022 delimitation, 243 wards)", "by": "KSRSAC / Karnataka GIS, collated by DataMeet",
             "url": "https://github.com/datameet/Municipal_Spatial_Data/tree/master/Bangalore", "licence": "CC BY / CC BY-SA 2.5 India",
             "used_for": "ward polygons, areas, centroids, adjacency graph"},
            {"name": "Schools and pre-schools (geocoded)", "by": "Open Bangalore repository",
             "url": "https://github.com/openbangalore/bangalore", "licence": "ODbL",
             "used_for": "residential-demand proxy (smoothed point density per ward)"},
            {"name": "Landmark anchors (tech parks, transit hubs, retail districts)", "by": "hand-curated, approximate public coordinates",
             "url": "", "licence": "n/a", "used_for": "employment / transit / retail demand prior"},
        ],
        "caveats": [
            "No real Rapido trip data is used; demand is a modelled proxy built from the sources above.",
            "School density is a coarse stand-in for resident population; the 2011/2023 ward populations were not reachable.",
            "Anchor coordinates are approximate and their weights are judgement calls, not measurements.",
        ],
        "anchors": [{"name": a[0], "lat": a[1], "lng": a[2], "kind": a[3], "weight": a[4]} for a in ANCHORS],
    }
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "city.json").write_text(json.dumps({"meta": meta, "wards": wards}, separators=(",", ":")))

    # ---- simplified polygons for the web map
    out_feats = []
    for i, p in enumerate(polys):
        s = p.simplify(0.00025, preserve_topology=True)
        g = mapping(s)

        def rnd(c):
            return [round(c[0], 5), round(c[1], 5)] if isinstance(c[0], float) else [rnd(x) for x in c]
        g["coordinates"] = rnd(g["coordinates"])
        out_feats.append({"type": "Feature", "id": i, "properties": {"id": i, "name": wards[i]["name"], "no": wards[i]["no"]}, "geometry": g})
    (OUT / "wards.geojson").write_text(json.dumps({"type": "FeatureCollection", "features": out_feats}, separators=(",", ":")))
    for fn in ("city.json", "wards.geojson"):
        print(fn, round((OUT / fn).stat().st_size / 1024), "KB")
    top = sorted(wards, key=lambda w: -w["employ"])[:5]
    print("top employ wards:", [w["name"] for w in top])
    print("type counts:", {t: labels.count(t) for t in set(labels)})
    print("core-nearest resid top:", [w["name"] for w in sorted(wards, key=lambda w: -w["resid"])[:5]])


if __name__ == "__main__":
    main()
