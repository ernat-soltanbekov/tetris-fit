#!/usr/bin/env python3
"""Black-box audit of built executables; Python standard library only.

Checks shapes independently of the Go parser/solver, including legal rotations.
Each subprocess has a 10-second external deadline, including process startup.
"""

import concurrent.futures
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / "bin/tetris-optimizer"
VISUALIZER = ROOT / "bin/tetris-visualizer"
EXPECTED = {
    "goodexample00": 0,
    "goodexample01": 9,
    "goodexample02": 4,
    "goodexample03": 5,
    "hardexam": 1,
}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def run(*args, binary=BINARY):
    return subprocess.run(
        [str(binary), *map(str, args)], cwd=ROOT, capture_output=True,
        text=True, timeout=10, check=False,
    )


def normalized(cells):
    min_x = min(x for x, _ in cells)
    min_y = min(y for _, y in cells)
    return frozenset((x - min_x, y - min_y) for x, y in cells)


def variants(cells, fixed=False):
    result = set()
    for _ in range(1 if fixed else 4):
        result.add(normalized(cells))
        cells = [(y, -x) for x, y in cells]
    return result


def validate(name, holes, fixed=False):
    path = ROOT / "testdata" / (name + ".txt")
    shapes = [
        [(x, y) for y, row in enumerate(block.splitlines())
         for x, cell in enumerate(row) if cell == "#"]
        for block in path.read_text().strip().split("\n\n")
    ]
    started = time.perf_counter()
    result = run(path, "--analyze", *(["--fixed"] if fixed else []))
    wall_ms = (time.perf_counter() - started) * 1000
    require(result.returncode == 0 and result.stderr == "", result.stderr + result.stdout)
    board_text, report = result.stdout.split("\n\n=== Solution Analysis ===\n", 1)
    grid = board_text.splitlines()
    side = len(grid)
    require(all(len(row) == side for row in grid), "non-square output")
    allowed = "." + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"[:len(shapes)]
    require(all(c in allowed for row in grid for c in row), "unexpected character")
    actual_holes = sum(row.count(".") for row in grid)
    require(actual_holes == holes, f"{name}: holes {actual_holes}, expected {holes}")
    for index, shape in enumerate(shapes):
        label = chr(ord("A") + index)
        cells = [(x, y) for y, row in enumerate(grid) for x, c in enumerate(row) if c == label]
        require(len(cells) == 4, f"{name}: {label} lost cells")
        require(normalized(cells) in variants(shape, fixed), f"{name}: {label} changed shape")
    filled = 4 * len(shapes)
    minimum = math.isqrt(filled)
    minimum += minimum * minimum < filled
    for text in [
        f"Tetrominoes: {len(shapes)}", f"Grid size: {side}x{side} ({side*side} cells)",
        f"Filled cells: {filled}", f"Empty cells: {holes}",
        f"Compactness: {filled/(side*side)*100:.2f}%",
        f"Theoretical minimum: {minimum}x{minimum} ({'matches' if minimum == side else 'does not match'}",
    ]:
        require(text in report, f"inconsistent metric: {text}")
    attempts = int(re.search(r"Placement attempts: (\d+)", report).group(1))
    backtracks = int(re.search(r"Backtracks: (\d+)", report).group(1))
    elapsed = re.search(r"Execution time: ([^\n]+)", report).group(1)
    require(attempts > 0 and 0 <= backtracks <= attempts, "invalid counters")
    require(elapsed != "0s" and not elapsed.startswith("-"), "invalid duration")
    return {"file": name, "side": side, "holes": holes, "attempts": attempts,
            "backtracks": backtracks, "solver_time": elapsed, "wall_ms": round(wall_ms, 3)}


def main():
    manifest = json.loads((ROOT / "testdata/SOURCES.json").read_text())
    for item in manifest:
        data = (ROOT / "testdata" / item["file"]).read_bytes()
        require(hashlib.sha256(data).hexdigest() == item["sha256"], "fixture changed: " + item["file"])

    results = [validate(name, holes) for name, holes in EXPECTED.items()]
    results += [validate("max-squares", 40), validate("max-bars", 40, fixed=True)]
    results += [validate("rotation-demo", 1), validate("rotation-demo", 8, fixed=True)]
    bad_files = sorted((ROOT / "testdata").glob("bad*.txt"))
    require(len(bad_files) == 6, "missing official invalid fixtures")
    for path in bad_files:
        for flags in [[], ["--analyze"], ["--visualize"]]:
            result = run(path, *flags)
            require(result.returncode == 1 and result.stdout == "ERROR\n" and not result.stderr, str(path))

    with tempfile.TemporaryDirectory(prefix="tetris-fit-audit-") as directory:
        directory = Path(directory)
        for path in [directory, directory / "missing"]:
            result = run(path, "--analyze")
            require(result.returncode == 1 and result.stdout == "ERROR\n", "invalid path accepted")
        huge = directory / "oversized"
        huge.write_bytes(b"." * 1_000_000)
        require(run(huge).stdout == "ERROR\n", "oversized input accepted")
        if hasattr(os, "mkfifo"):
            fifo = directory / "fifo"
            os.mkfifo(fifo)
            result = run(fifo)
            require(result.returncode == 1 and result.stdout == "ERROR\n", "FIFO must not block")

    for arguments in [[], ["x", "y"], ["--timeout=6m", "x"], ["--rotate", "--fixed", "x"]]:
        result = run(*arguments)
        require(result.returncode == 2 and result.stdout == "ERROR\n", "bad arguments accepted")
    result = run("testdata/goodexample01.txt", "--max-attempts=1", "--analyze")
    require(result.returncode == 124 and result.stdout == "ERROR\n", "budget not enforced")
    require("no minimum solution is claimed" in result.stderr, "missing interruption explanation")

    optimized = run("testdata/goodexample01.txt", "--visualize", "--analyze")
    visualized = run("testdata/goodexample01.txt", "--analyze", binary=VISUALIZER)
    for result in [optimized, visualized]:
        require(result.returncode == 0 and "backtrack piece" in result.stdout, "missing real backtrack")
        require("Final solution reached." in result.stdout, "missing visualizer finish")
    # Timing is expected to differ, while all board events and counters must match.
    remove_time = lambda text: re.sub(r"Execution time: [^\n]+", "Execution time: <measured>", text)
    require(remove_time(optimized.stdout) == remove_time(visualized.stdout), "visualizer entry points differ")

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        concurrent_results = list(pool.map(lambda _: validate("hardexam", 1), range(32)))
    require(all(r["attempts"] == results[4]["attempts"] for r in concurrent_results), "nondeterministic search")
    print(json.dumps({"status": "PASS", "official_sha256_verified": len(manifest),
                      "invalid_fixtures": len(bad_files), "concurrent_processes": 32,
                      "cases": results}, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
