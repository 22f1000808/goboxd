#!/usr/bin/env python3
"""
docs/loadtest/plot.py
Generate breaking-point.png and latency.png from results.csv.

Usage:
    python3 plot.py <results.csv> <output_dir>
"""

import csv
import sys
import os

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker

def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <results.csv> <output_dir>")
        sys.exit(1)

    csv_path = sys.argv[1]
    out_dir = sys.argv[2]

    rows = list(csv.DictReader(open(csv_path)))
    if not rows:
        print("ERROR: No data in CSV")
        sys.exit(1)

    rps = [float(r["target_rps"]) for r in rows]
    error_pct = [float(r["error_pct"]) for r in rows]
    p50 = [float(r["p50_ms"]) for r in rows]
    p95 = [float(r["p95_ms"]) for r in rows]
    p99 = [float(r["p99_ms"]) for r in rows]

    # ── Find breaking point ─────────────────────────────────────────
    breaking_rps = None
    for i, e in enumerate(error_pct):
        if e > 0:
            breaking_rps = rps[i]
            break

    # ── Style ────────────────────────────────────────────────────────
    plt.rcParams.update({
        "font.family": "sans-serif",
        "font.size": 11,
        "axes.grid": True,
        "grid.alpha": 0.3,
        "figure.facecolor": "#f8f9fa",
        "axes.facecolor": "#ffffff",
    })

    # ── Breaking-point graph ─────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(10, 6))
    ax.plot(rps, error_pct, marker="o", color="#e63946", linewidth=2,
            markersize=8, label="Error rate")
    ax.fill_between(rps, error_pct, alpha=0.15, color="#e63946")

    if breaking_rps is not None:
        ax.axvline(x=breaking_rps, color="#457b9d", linestyle="--",
                   linewidth=2, label=f"Breaking point: {breaking_rps:.0f} RPS")

    ax.set_xlabel("Offered RPS", fontsize=13, fontweight="bold")
    ax.set_ylabel("Error Rate (%)", fontsize=13, fontweight="bold")
    ax.set_title("GoBoxd Load Test — Breaking Point", fontsize=15, fontweight="bold")
    ax.legend(fontsize=11)
    ax.yaxis.set_major_formatter(ticker.FormatStrFormatter("%.1f%%"))

    bp_path = os.path.join(out_dir, "breaking-point.png")
    fig.savefig(bp_path, dpi=150, bbox_inches="tight")
    plt.close(fig)
    print(f"  → {bp_path}")

    # ── Latency graph ────────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(10, 6))
    ax.plot(rps, p50, marker="o", color="#2a9d8f", linewidth=2,
            markersize=7, label="p50")
    ax.plot(rps, p95, marker="s", color="#e9c46a", linewidth=2,
            markersize=7, label="p95")
    ax.plot(rps, p99, marker="^", color="#e76f51", linewidth=2,
            markersize=7, label="p99")

    if breaking_rps is not None:
        ax.axvline(x=breaking_rps, color="#457b9d", linestyle="--",
                   linewidth=1.5, alpha=0.6, label=f"Breaking point: {breaking_rps:.0f} RPS")

    ax.set_xlabel("Offered RPS", fontsize=13, fontweight="bold")
    ax.set_ylabel("Latency (ms)", fontsize=13, fontweight="bold")
    ax.set_title("GoBoxd Load Test — RPS vs Latency", fontsize=15, fontweight="bold")
    ax.legend(fontsize=11)

    lat_path = os.path.join(out_dir, "latency.png")
    fig.savefig(lat_path, dpi=150, bbox_inches="tight")
    plt.close(fig)
    print(f"  → {lat_path}")

    # ── Summary ──────────────────────────────────────────────────────
    print()
    if breaking_rps is not None:
        print(f"  Breaking point: {breaking_rps:.0f} RPS")
    else:
        print("  No failures detected across all rates tested.")


if __name__ == "__main__":
    main()
