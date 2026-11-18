"""Summarize Go benchmark output: median, min, max per benchmark and metric.

usage: stats.py FILE [FILE2]   (two files: side-by-side medians and change)
"""
import re, sys, statistics as st
from collections import OrderedDict

def load(path):
    out = OrderedDict()
    for line in open(path):
        m = re.match(r"^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+(.*)$", line)
        if not m:
            continue
        name, rest = m.group(1), m.group(2).split()
        rec = out.setdefault(name, OrderedDict())
        for value, unit in zip(rest[0::2], rest[1::2]):
            rec.setdefault(unit, []).append(float(value))
    return out

def fmt(v):
    return f"{v:.3g}" if v < 100 else f"{v:.0f}"

def one(path):
    for name, rec in load(path).items():
        ns = rec["ns/op"]
        extra = " ".join(f"{fmt(st.median(v))} {u}" for u, v in rec.items() if u != "ns/op")
        print(f"{name[9:]:62s} n={len(ns):2d} med {fmt(st.median(ns)):>6s}  min {fmt(min(ns)):>6s}  max {fmt(max(ns)):>6s}  {extra}")

def two(a, b):
    A, B = load(a), load(b)
    for name, rec in A.items():
        if name not in B:
            continue
        x, y = rec["ns/op"], B[name]["ns/op"]
        mx, my = st.median(x), st.median(y)
        extra = ""
        for u in rec:
            if u != "ns/op" and u in B[name]:
                extra += f"  {fmt(st.median(rec[u]))}->{fmt(st.median(B[name][u]))} {u}"
        print(f"{name[9:]:62s} {fmt(mx):>6s} [{fmt(min(x))}-{fmt(max(x))}] -> {fmt(my):>6s} [{fmt(min(y))}-{fmt(max(y))}]  {100*(my-mx)/mx:+6.1f}%{extra}")

if len(sys.argv) == 2:
    one(sys.argv[1])
else:
    two(sys.argv[1], sys.argv[2])
